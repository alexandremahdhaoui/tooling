package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/alexandremahdhaoui/tooling/internal/util"
	"github.com/alexandremahdhaoui/tooling/pkg/flaterrors"
	"github.com/alexandremahdhaoui/tooling/pkg/forge"
	"sigs.k8s.io/yaml"
)

var (
	errLoggingInToRegistry    = errors.New("logging in to registry")
	errPushingSingleImage     = errors.New("pushing image")
	errPushingImages          = errors.New("pushing images from artifact store")
	errReadingCredentials     = errors.New("reading credentials")
	errSettingUpDockerCerts   = errors.New("setting up docker certificates")
	errTearingDownDockerCerts = errors.New("tearing down docker certificates")
)

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
		mkdirCmd = exec.Command(prependCmd, "mkdir", "-p", certsDir)
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
		cpCmd = exec.Command(prependCmd, "cp", caCrtPath, destCertPath)
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
		rmCmd = exec.Command(prependCmd, "rm", "-rf", certsDir)
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
	stdin.Close()

	// Wait for command to finish
	if err := loginCmd.Wait(); err != nil {
		return flaterrors.Join(err, errLoggingInToRegistry)
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

// withRegistryAccess handles all the setup (port-forward, certs, login) and teardown for registry access.
// It calls the provided function with the registry FQDN:PORT.
func withRegistryAccess(
	ctx context.Context,
	config forge.Spec,
	envs Envs,
	fn func(registryFQDNWithPort string) error,
) error {
	// I. Establish port-forward to registry
	pf := NewPortForwarder(config, config.LocalContainerRegistry.Namespace)
	if err := pf.Start(ctx); err != nil {
		return flaterrors.Join(err, errors.New("establishing port-forward"))
	}
	defer pf.Stop()

	// II. Create FQDN:PORT for image tags, certs, and login
	containerRegistry := NewContainerRegistry(nil, config.LocalContainerRegistry.Namespace, nil)
	registryFQDNWithPort := fmt.Sprintf("%s:%d", containerRegistry.FQDN(), pf.LocalPort())

	// III. Set up Docker certificates if using Docker
	var certsDir string
	var err error
	if envs.ContainerEngineExecutable == "docker" {
		certsDir, err = setupDockerCerts(
			registryFQDNWithPort,
			config.LocalContainerRegistry.CaCrtPath,
			envs.PrependCmd,
		)
		if err != nil {
			return flaterrors.Join(err, errSettingUpDockerCerts)
		}
		defer func() {
			_ = teardownDockerCerts(certsDir, envs.PrependCmd)
		}()
	}

	// IV. Login to registry using FQDN:PORT (Docker stores credentials per registry hostname)
	if err := loginToRegistry(envs.ContainerEngineExecutable, registryFQDNWithPort, config.LocalContainerRegistry.CredentialPath); err != nil {
		return flaterrors.Join(err, errLoggingInToRegistry)
	}

	// V. Execute the provided function
	return fn(registryFQDNWithPort)
}

// pushImagesFromArtifactStore reads the artifact store and pushes all container images
// defined in the project configuration to the local container registry.
func pushImagesFromArtifactStore(ctx context.Context, config forge.Spec, envs Envs) error {
	_, _ = fmt.Fprintln(os.Stdout, "⏳ Pushing images from artifact store")

	return withRegistryAccess(ctx, config, envs, func(registryFQDNWithPort string) error {
		// I. Read artifact store
		store, err := forge.ReadArtifactStore(config.Build.ArtifactStorePath)
		if err != nil {
			return flaterrors.Join(err, errPushingImages)
		}

		// II. For each container spec in the config, find the latest artifact and push it
		for _, spec := range config.Build.Specs {
			// Skip if not a container spec (check engine field)
			if spec.Engine != "go://build-container" {
				continue
			}

			// Get latest artifact for this container
			artifact, err := forge.GetLatestArtifact(store, spec.Name)
			if err != nil {
				// If no artifact found, skip with warning
				_, _ = fmt.Fprintf(
					os.Stderr,
					"⚠️  Warning: %s - skipping %s\n",
					err.Error(),
					spec.Name,
				)
				continue
			}

			// Push the image using FQDN:PORT (Location contains "build-container:abc123")
			if err := pushImage(envs.ContainerEngineExecutable, artifact.Location, registryFQDNWithPort); err != nil {
				return flaterrors.Join(err, errPushingImages)
			}
		}

		_, _ = fmt.Fprintln(os.Stdout, "✅ All images pushed successfully")
		return nil
	})
}

// pushSingleImage pushes a single image to the local container registry.
func pushSingleImage(
	ctx context.Context,
	config forge.Spec,
	envs Envs,
	imageName string,
) error {
	_, _ = fmt.Fprintf(os.Stdout, "⏳ Pushing image: %s\n", imageName)

	return withRegistryAccess(ctx, config, envs, func(registryFQDNWithPort string) error {
		if err := pushImage(envs.ContainerEngineExecutable, imageName, registryFQDNWithPort); err != nil {
			return flaterrors.Join(err, errPushingSingleImage)
		}

		_, _ = fmt.Fprintln(os.Stdout, "✅ Image pushed successfully")
		return nil
	})
}
