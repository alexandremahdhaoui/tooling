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
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/alexandremahdhaoui/forge/internal/util"
	"github.com/alexandremahdhaoui/forge/pkg/flaterrors"
	"sigs.k8s.io/yaml"
)

var (
	errHTTPSHealthCheck       = errors.New("HTTPS health check failed")
	errLoggingInToRegistry    = errors.New("logging in to registry")
	errPushingSingleImage     = errors.New("pushing image")
	errReadingCredentials     = errors.New("reading credentials")
	errSettingUpDockerCerts   = errors.New("setting up docker certificates")
	errTearingDownDockerCerts = errors.New("tearing down docker certificates")
)

// httpsHealthCheck waits for the registry to respond to HTTPS requests on /v2/.
// Any HTTP response (200, 401, etc.) indicates the TLS listener is ready.
// serverName is the registry FQDN used for TLS hostname verification (the cert SAN
// matches the FQDN, not 127.0.0.1 which is the port-forward address).
func httpsHealthCheck(ctx context.Context, port int32, caCrtPath, serverName string) error {
	_, _ = fmt.Fprintf(os.Stdout, "Waiting for registry HTTPS at 127.0.0.1:%d\n", port)

	caCertPEM, err := os.ReadFile(caCrtPath)
	if err != nil {
		return fmt.Errorf("reading CA certificate: %w", err)
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caCertPEM) {
		return errors.New("failed to parse CA certificate")
	}

	httpClient := &http.Client{ //nolint:exhaustruct
		Transport: &http.Transport{ //nolint:exhaustruct
			TLSClientConfig: &tls.Config{ //nolint:exhaustruct
				RootCAs:    pool,
				ServerName: serverName,
				MinVersion: tls.VersionTLS12,
			},
		},
		Timeout: 2 * time.Second,
	}

	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	url := fmt.Sprintf("https://127.0.0.1:%d/v2/", port)

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for HTTPS health check at %s: %w", url, errHTTPSHealthCheck)
		case <-ctx.Done():
			return fmt.Errorf("context cancelled waiting for HTTPS health check: %w", ctx.Err())
		case <-ticker.C:
			resp, err := httpClient.Get(url) //nolint:noctx
			if err != nil {
				continue
			}
			_, _ = fmt.Fprintf(os.Stdout, "HTTPS health check passed (status %d)\n", resp.StatusCode)
			_ = resp.Body.Close()

			return nil
		}
	}
}

// readCredentials reads the credentials from the specified file.
func readCredentials(credPath string) (Credentials, error) {
	b, err := os.ReadFile(credPath)
	if err != nil {
		return Credentials{}, flaterrors.Join(err, errReadingCredentials)
	}

	var creds Credentials
	if err := yaml.Unmarshal(b, &creds); err != nil {
		return Credentials{}, flaterrors.Join(err, errReadingCredentials)
	}

	return creds, nil
}

// setupDockerCerts sets up the Docker certificate directory for the given registry FQDN.
// This is required for Docker to trust the self-signed certificate when pushing to the registry.
// Returns the path to the certificate directory that was created.
func setupDockerCerts(registryFQDN, caCrtPath, prependCmd string) (string, error) {
	// Create the certificate directory for the registry FQDN (not the IP!)
	// Docker will look for certs based on the hostname in the image tag
	certsDir := filepath.Join("/etc/docker/certs.d", registryFQDN)

	// Create directory with sudo if needed
	var mkdirCmd *exec.Cmd
	if prependCmd != "" {
		// Split prependCmd into parts (e.g., "sudo -E" -> ["sudo", "-E"])
		prependParts := strings.Fields(prependCmd)
		args := append(prependParts[1:], "mkdir", "-p", certsDir)
		mkdirCmd = exec.Command(prependParts[0], args...)
	} else {
		mkdirCmd = exec.Command("mkdir", "-p", certsDir)
	}

	if err := util.RunCmdWithStdPipes(mkdirCmd); err != nil {
		return "", flaterrors.Join(err, errSettingUpDockerCerts)
	}

	// Copy CA certificate to the directory
	destCertPath := filepath.Join(certsDir, "ca.crt")
	var cpCmd *exec.Cmd
	if prependCmd != "" {
		// Split prependCmd into parts (e.g., "sudo -E" -> ["sudo", "-E"])
		prependParts := strings.Fields(prependCmd)
		args := append(prependParts[1:], "cp", caCrtPath, destCertPath)
		cpCmd = exec.Command(prependParts[0], args...)
	} else {
		cpCmd = exec.Command("cp", caCrtPath, destCertPath)
	}

	if err := util.RunCmdWithStdPipes(cpCmd); err != nil {
		return "", flaterrors.Join(err, errSettingUpDockerCerts)
	}

	_, _ = fmt.Fprintf(os.Stdout, "✅ Set up Docker certificates for %s\n", registryFQDN)

	return certsDir, nil
}

// teardownDockerCerts removes the Docker certificate directory that was created for the registry.
func teardownDockerCerts(certsDir, prependCmd string) error {
	if certsDir == "" {
		return nil
	}

	// Remove the certificate directory
	var rmCmd *exec.Cmd
	if prependCmd != "" {
		// Split prependCmd into parts (e.g., "sudo -E" -> ["sudo", "-E"])
		prependParts := strings.Fields(prependCmd)
		args := append(prependParts[1:], "rm", "-rf", certsDir)
		rmCmd = exec.Command(prependParts[0], args...)
	} else {
		rmCmd = exec.Command("rm", "-rf", certsDir)
	}

	if err := util.RunCmdWithStdPipes(rmCmd); err != nil {
		return flaterrors.Join(err, errTearingDownDockerCerts)
	}

	_, _ = fmt.Fprintf(os.Stdout, "✅ Cleaned up Docker certificates\n")

	return nil
}

// loginToRegistry logs into the container registry using the provided credentials.
func loginToRegistry(containerEngine, registryEndpoint, credPath string) error {
	creds, err := readCredentials(credPath)
	if err != nil {
		return flaterrors.Join(err, errLoggingInToRegistry)
	}

	// Create login command: echo password | docker login -u username --password-stdin endpoint
	loginCmd := exec.Command(
		containerEngine,
		"login",
		registryEndpoint,
		"-u", creds.Username,
		"--password-stdin",
	)

	// Capture stdout and stderr for debugging
	var stdout, stderr bytes.Buffer
	loginCmd.Stdout = &stdout
	loginCmd.Stderr = &stderr

	// Set password as stdin using a pipe
	stdin, err := loginCmd.StdinPipe()
	if err != nil {
		return flaterrors.Join(err, errLoggingInToRegistry)
	}

	// Start the command
	if err := loginCmd.Start(); err != nil {
		return flaterrors.Join(err, errLoggingInToRegistry)
	}

	// Write password to stdin
	if _, err := stdin.Write([]byte(creds.Password)); err != nil {
		return flaterrors.Join(err, errLoggingInToRegistry)
	}
	_ = stdin.Close()

	// Wait for command to finish
	if err := loginCmd.Wait(); err != nil {
		// Include stdout and stderr in error message for debugging
		errMsg := fmt.Sprintf("docker login failed: %v\nstdout: %s\nstderr: %s",
			err, stdout.String(), stderr.String())
		return flaterrors.Join(errors.New(errMsg), errLoggingInToRegistry)
	}

	_, _ = fmt.Fprintf(os.Stdout, "✅ Logged in to registry: %s\n", registryEndpoint)

	return nil
}

// pushImage tags and pushes a single image to the registry.
// sourceImage is the local image reference (e.g., "build-container:abc123")
// registryFQDN is the registry FQDN with port (e.g., "local-container-registry.local-container-registry.svc.cluster.local:5000")
func pushImage(containerEngine, sourceImage, registryFQDN string) error {
	// Build destination image name using FQDN (not IP!)
	// This is important because Docker looks for certificates based on the hostname in the tag
	destImage := fmt.Sprintf("%s/%s", registryFQDN, sourceImage)

	_, _ = fmt.Fprintf(os.Stdout, "⏳ Pushing image: %s -> %s\n", sourceImage, destImage)

	// Tag the image
	tagCmd := exec.Command(containerEngine, "tag", sourceImage, destImage)
	if err := util.RunCmdWithStdPipes(tagCmd); err != nil {
		return flaterrors.Join(err, errPushingSingleImage)
	}

	// Push the image
	// Note: For Docker, certificates should be set up beforehand using setupDockerCerts().
	// For Podman, use --tls-verify=false flag.
	pushCmd := exec.Command(containerEngine, "push", destImage)
	if containerEngine == "podman" {
		pushCmd = exec.Command(containerEngine, "push", "--tls-verify=false", destImage)
	}

	if err := util.RunCmdWithStdPipes(pushCmd); err != nil {
		return flaterrors.Join(err, errPushingSingleImage)
	}

	_, _ = fmt.Fprintf(os.Stdout, "✅ Pushed image: %s\n", destImage)

	return nil
}

// withRegistryAccess handles all the setup (certs, login) and uses the already-running port-forward for registry access.
// It calls the provided function with the registry FQDN:PORT.
// The dynamicPort parameter is the port that was acquired by the port lease manager and used for the NodePort service.
// A port-forward should already be running from createLocalContainerRegistry, so this function does NOT start a new one.
func withRegistryAccess(
	ctx context.Context,
	config runConfig,
	envs Envs,
	dynamicPort int32,
	fn func(registryFQDNWithPort string) error,
) error {
	// I. Create container registry with the dynamic port
	containerRegistry := NewContainerRegistry(nil, config.LocalContainerRegistry.Namespace, nil)
	containerRegistry.SetDynamicPort(dynamicPort)

	// II. Port-forward is already running from createLocalContainerRegistry - no need to start a new one.
	// The port-forward maps dynamicPort:dynamicPort (same port on both ends).

	// III. Create FQDN:PORT for image tags, certs, and login
	registryFQDNWithPort := fmt.Sprintf("%s:%d", containerRegistry.FQDN(), dynamicPort)

	// III. Set up Docker certificates if using Docker
	// Check for "docker" in the executable name (handles both "docker" and "/usr/bin/docker")
	var certsDir string
	var err error
	isDocker := strings.Contains(filepath.Base(envs.ContainerEngineExecutable), "docker")
	if isDocker {
		// Use ElevatedPrependCmd for writing to /etc/docker/certs.d/ (requires root)
		certsDir, err = setupDockerCerts(
			registryFQDNWithPort,
			config.LocalContainerRegistry.CaCrtPath,
			envs.ElevatedPrependCmd,
		)
		if err != nil {
			return flaterrors.Join(err, errSettingUpDockerCerts)
		}
		defer func() {
			_ = teardownDockerCerts(certsDir, envs.ElevatedPrependCmd)
		}()
	}

	// IV. Wait for registry HTTPS to be accessible before logging in
	if err := httpsHealthCheck(ctx, dynamicPort, config.LocalContainerRegistry.CaCrtPath, containerRegistry.FQDN()); err != nil {
		return flaterrors.Join(err, errHTTPSHealthCheck)
	}

	// V. Login to registry with retry (transient TLS errors may occur)
	var loginErr error
	for attempt := 1; attempt <= 3; attempt++ {
		loginErr = loginToRegistry(envs.ContainerEngineExecutable, registryFQDNWithPort, config.LocalContainerRegistry.CredentialPath)
		if loginErr == nil {
			break
		}
		if attempt < 3 {
			_, _ = fmt.Fprintf(os.Stdout, "Login attempt %d/3 failed, retrying in 2s: %v\n", attempt, loginErr)
			time.Sleep(2 * time.Second)
		}
	}
	if loginErr != nil {
		return flaterrors.Join(loginErr, errLoggingInToRegistry)
	}

	// VI. Execute the provided function
	return fn(registryFQDNWithPort)
}
