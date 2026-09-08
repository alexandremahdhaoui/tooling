// Copyright 2024 Alexandre Mahdhaoui
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/alexandremahdhaoui/forge/internal/util"
	"github.com/alexandremahdhaoui/forge/pkg/flaterrors"
)

var errProcessingImages = errors.New("processing images")

// checkLocalImageExists verifies a local image exists in the Docker daemon.
// imageName should be without the local:// prefix (e.g., "myapp:v1").
func checkLocalImageExists(containerEngine, imageName string) error {
	cmd := exec.Command(containerEngine, "inspect", "--type=image", imageName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("local image %q not found in local Docker daemon", imageName)
	}
	return nil
}

// pullRemoteImage pulls a remote image, optionally authenticating first.
func pullRemoteImage(containerEngine string, img ImageSource, parsed ParsedImage) error {
	// Authenticate if basicAuth provided
	if hasBasicAuth(img) {
		username, err := ResolveValueFrom(img.BasicAuth.Username, "username")
		if err != nil {
			return fmt.Errorf("failed to resolve username: %w", err)
		}

		password, err := ResolveValueFrom(img.BasicAuth.Password, "password")
		if err != nil {
			return fmt.Errorf("failed to resolve password: %w", err)
		}

		// Login to registry
		loginCmd := exec.Command(containerEngine, "login", parsed.Registry, "-u", username, "--password-stdin")
		stdin, err := loginCmd.StdinPipe()
		if err != nil {
			return fmt.Errorf("failed to create stdin pipe: %w", err)
		}

		if err := loginCmd.Start(); err != nil {
			return fmt.Errorf("failed to start login command: %w", err)
		}

		if _, err := stdin.Write([]byte(password)); err != nil {
			return fmt.Errorf("failed to write password: %w", err)
		}
		_ = stdin.Close()

		if err := loginCmd.Wait(); err != nil {
			return fmt.Errorf("failed to login to %s: authentication failed", parsed.Registry)
		}

		_, _ = fmt.Fprintf(os.Stdout, "✅ Logged in to registry: %s\n", parsed.Registry)
	}

	// Pull image
	_, _ = fmt.Fprintf(os.Stdout, "⏳ Pulling image: %s\n", img.Name)
	pullCmd := exec.Command(containerEngine, "pull", img.Name)
	if err := util.RunCmdWithStdPipes(pullCmd); err != nil {
		return fmt.Errorf("failed to pull image %q: %w", img.Name, err)
	}

	_, _ = fmt.Fprintf(os.Stdout, "✅ Pulled image: %s\n", img.Name)
	return nil
}

// processImages handles all image processing: pre-flight validation, pull, tag, push.
// The dynamicPort parameter is the port that was acquired by the port lease manager and used for the NodePort service.
func processImages(ctx context.Context, images []ImageSource, config runConfig, envs Envs, dynamicPort int32) error {
	if len(images) == 0 {
		return nil // No images to process
	}

	_, _ = fmt.Fprintln(os.Stdout, "⏳ Processing images")

	// Phase 1: Pre-flight validation - check all local images exist
	for _, img := range images {
		parsed := ParseImageName(img.Name)
		if parsed.Type == ImageTypeLocal {
			if err := checkLocalImageExists(envs.ContainerEngineExecutable, parsed.ImageName); err != nil {
				return flaterrors.Join(err, errProcessingImages)
			}
		}

		// Resolve all env vars to fail fast
		if hasBasicAuth(img) {
			if _, err := ResolveValueFrom(img.BasicAuth.Username, "username"); err != nil {
				return flaterrors.Join(fmt.Errorf("image %q: %w", img.Name, err), errProcessingImages)
			}
			if _, err := ResolveValueFrom(img.BasicAuth.Password, "password"); err != nil {
				return flaterrors.Join(fmt.Errorf("image %q: %w", img.Name, err), errProcessingImages)
			}
		}
	}

	// Phase 2: Process images with registry access
	return withRegistryAccess(ctx, config, envs, dynamicPort, func(registryFQDNWithPort string) error {
		for _, img := range images {
			parsed := ParseImageName(img.Name)

			// Pull if remote
			if parsed.Type == ImageTypeRemote {
				if err := pullRemoteImage(envs.ContainerEngineExecutable, img, parsed); err != nil {
					return err
				}
			}

			// Tag and push
			if err := pushImage(envs.ContainerEngineExecutable, parsed.ImageName, registryFQDNWithPort); err != nil {
				return fmt.Errorf("failed to push image %q: %w", img.Name, err)
			}
		}

		_, _ = fmt.Fprintln(os.Stdout, "✅ All images processed successfully")
		return nil
	})
}
