//go:build integration

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestIntegration_DockerMode tests building a container with docker mode.
func TestIntegration_DockerMode(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Ensure docker is available
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("Docker not available, skipping test")
	}

	// Setup test environment
	tmpDir := t.TempDir()

	// Create test Containerfile
	containerfile := filepath.Join(tmpDir, "Containerfile")
	content := `FROM alpine:3.20
CMD ["echo", "test-docker-mode"]`
	if err := os.WriteFile(containerfile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create minimal forge.yaml
	forgeYaml := filepath.Join(tmpDir, "forge.yaml")
	forgeContent := `name: test-docker-mode
artifactStorePath: .ignore.artifact-store.yaml

build:
  - name: test-docker-image
    src: ./Containerfile
    engine: go://container-build
`
	if err := os.WriteFile(forgeYaml, []byte(forgeContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Initialize a git repo (required for versioning)
	gitInit := exec.Command("git", "init")
	gitInit.Dir = tmpDir
	if err := gitInit.Run(); err != nil {
		t.Fatalf("Failed to init git: %v", err)
	}

	gitConfig1 := exec.Command("git", "config", "user.email", "test@example.com")
	gitConfig1.Dir = tmpDir
	_ = gitConfig1.Run()

	gitConfig2 := exec.Command("git", "config", "user.name", "Test User")
	gitConfig2.Dir = tmpDir
	_ = gitConfig2.Run()

	gitAdd := exec.Command("git", "add", ".")
	gitAdd.Dir = tmpDir
	if err := gitAdd.Run(); err != nil {
		t.Fatalf("Failed to add files: %v", err)
	}

	gitCommit := exec.Command("git", "commit", "-m", "Initial commit")
	gitCommit.Dir = tmpDir
	if err := gitCommit.Run(); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Build the binary first
	buildBinary := exec.Command("go", "build", "-o", "container-build", ".")
	buildBinary.Dir = filepath.Join("/home/alexandremahdhaoui/go/src/github.com/alexandremahdhaoui/forge/cmd/container-build")
	if err := buildBinary.Run(); err != nil {
		t.Fatalf("Failed to build binary: %v", err)
	}
	defer os.Remove(filepath.Join("/home/alexandremahdhaoui/go/src/github.com/alexandremahdhaoui/forge/cmd/container-build", "container-build"))

	// Run container-build with CONTAINER_BUILD_ENGINE=docker
	cmd := exec.Command(
		filepath.Join("/home/alexandremahdhaoui/go/src/github.com/alexandremahdhaoui/forge/cmd/container-build", "container-build"),
	)
	cmd.Dir = tmpDir
	cmd.Env = append(os.Environ(),
		"CONTAINER_BUILD_ENGINE=docker",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to build with docker: %v\nOutput: %s", err, output)
	}

	// Verify output indicates success
	if !strings.Contains(string(output), "✅") {
		t.Errorf("Expected success message in output, got: %s", output)
	}

	// Verify image was created
	checkImage := exec.Command("docker", "images", "-q", "test-docker-image:latest")
	imageOutput, err := checkImage.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to check image: %v", err)
	}

	imageID := strings.TrimSpace(string(imageOutput))
	if imageID == "" {
		t.Error("Image test-docker-image:latest was not created")
	}

	// Cleanup: remove the test image
	cleanup := exec.Command("docker", "rmi", "-f", "test-docker-image:latest")
	_ = cleanup.Run()
}

// TestIntegration_KanikoMode tests building a container with kaniko mode.
func TestIntegration_KanikoMode(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Ensure docker is available (kaniko runs in a docker container)
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("Docker not available, skipping test (kaniko requires docker to run)")
	}

	// Setup test environment
	tmpDir := t.TempDir()

	// Create test Containerfile
	containerfile := filepath.Join(tmpDir, "Containerfile")
	content := `FROM alpine:3.20
CMD ["echo", "test-kaniko-mode"]`
	if err := os.WriteFile(containerfile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create minimal forge.yaml
	forgeYaml := filepath.Join(tmpDir, "forge.yaml")
	forgeContent := `name: test-kaniko-mode
artifactStorePath: .ignore.artifact-store.yaml

build:
  - name: test-kaniko-image
    src: ./Containerfile
    engine: go://container-build
`
	if err := os.WriteFile(forgeYaml, []byte(forgeContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Initialize a git repo (required for versioning)
	gitInit := exec.Command("git", "init")
	gitInit.Dir = tmpDir
	if err := gitInit.Run(); err != nil {
		t.Fatalf("Failed to init git: %v", err)
	}

	gitConfig1 := exec.Command("git", "config", "user.email", "test@example.com")
	gitConfig1.Dir = tmpDir
	_ = gitConfig1.Run()

	gitConfig2 := exec.Command("git", "config", "user.name", "Test User")
	gitConfig2.Dir = tmpDir
	_ = gitConfig2.Run()

	gitAdd := exec.Command("git", "add", ".")
	gitAdd.Dir = tmpDir
	if err := gitAdd.Run(); err != nil {
		t.Fatalf("Failed to add files: %v", err)
	}

	gitCommit := exec.Command("git", "commit", "-m", "Initial commit")
	gitCommit.Dir = tmpDir
	if err := gitCommit.Run(); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Build the binary first
	buildBinary := exec.Command("go", "build", "-o", "container-build", ".")
	buildBinary.Dir = filepath.Join("/home/alexandremahdhaoui/go/src/github.com/alexandremahdhaoui/forge/cmd/container-build")
	if err := buildBinary.Run(); err != nil {
		t.Fatalf("Failed to build binary: %v", err)
	}
	defer os.Remove(filepath.Join("/home/alexandremahdhaoui/go/src/github.com/alexandremahdhaoui/forge/cmd/container-build", "container-build"))

	// Run container-build with CONTAINER_BUILD_ENGINE=kaniko
	cmd := exec.Command(
		filepath.Join("/home/alexandremahdhaoui/go/src/github.com/alexandremahdhaoui/forge/cmd/container-build", "container-build"),
	)
	cmd.Dir = tmpDir
	cmd.Env = append(os.Environ(),
		"CONTAINER_BUILD_ENGINE=kaniko",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to build with kaniko: %v\nOutput: %s", err, output)
	}

	// Verify output indicates success
	if !strings.Contains(string(output), "✅") {
		t.Errorf("Expected success message in output, got: %s", output)
	}

	// Verify image was created
	checkImage := exec.Command("docker", "images", "-q", "test-kaniko-image:latest")
	imageOutput, err := checkImage.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to check image: %v", err)
	}

	imageID := strings.TrimSpace(string(imageOutput))
	if imageID == "" {
		t.Error("Image test-kaniko-image:latest was not created")
	}

	// Verify tar file was cleaned up
	tarPath := filepath.Join(tmpDir, ".ignore.test-kaniko-image.tar")
	if _, err := os.Stat(tarPath); err == nil {
		t.Error("Tar file was not cleaned up")
	}

	// Cleanup: remove the test image
	cleanup := exec.Command("docker", "rmi", "-f", "test-kaniko-image:latest")
	_ = cleanup.Run()
}

// TestIntegration_DockerVsKanikoEquivalence verifies that docker and kaniko modes
// produce functionally equivalent images.
func TestIntegration_DockerVsKanikoEquivalence(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Ensure docker is available
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("Docker not available, skipping test")
	}

	// Setup test environment
	tmpDir := t.TempDir()

	// Create test Containerfile with a simple layer
	containerfile := filepath.Join(tmpDir, "Containerfile")
	content := `FROM alpine:3.20
RUN echo "test-layer" > /test.txt
CMD ["cat", "/test.txt"]`
	if err := os.WriteFile(containerfile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create minimal forge.yaml with single image
	forgeYaml := filepath.Join(tmpDir, "forge.yaml")
	forgeContent := `name: test-comparison
artifactStorePath: .ignore.artifact-store.yaml

build:
  - name: test-comparison
    src: ./Containerfile
    engine: go://container-build
`
	if err := os.WriteFile(forgeYaml, []byte(forgeContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Initialize a git repo (required for versioning)
	gitInit := exec.Command("git", "init")
	gitInit.Dir = tmpDir
	if err := gitInit.Run(); err != nil {
		t.Fatalf("Failed to init git: %v", err)
	}

	gitConfig1 := exec.Command("git", "config", "user.email", "test@example.com")
	gitConfig1.Dir = tmpDir
	_ = gitConfig1.Run()

	gitConfig2 := exec.Command("git", "config", "user.name", "Test User")
	gitConfig2.Dir = tmpDir
	_ = gitConfig2.Run()

	gitAdd := exec.Command("git", "add", ".")
	gitAdd.Dir = tmpDir
	if err := gitAdd.Run(); err != nil {
		t.Fatalf("Failed to add files: %v", err)
	}

	gitCommit := exec.Command("git", "commit", "-m", "Initial commit")
	gitCommit.Dir = tmpDir
	if err := gitCommit.Run(); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Build the binary first
	buildBinary := exec.Command("go", "build", "-o", "container-build", ".")
	buildBinary.Dir = filepath.Join("/home/alexandremahdhaoui/go/src/github.com/alexandremahdhaoui/forge/cmd/container-build")
	if err := buildBinary.Run(); err != nil {
		t.Fatalf("Failed to build binary: %v", err)
	}
	defer os.Remove(filepath.Join("/home/alexandremahdhaoui/go/src/github.com/alexandremahdhaoui/forge/cmd/container-build", "container-build"))

	binaryPath := filepath.Join("/home/alexandremahdhaoui/go/src/github.com/alexandremahdhaoui/forge/cmd/container-build", "container-build")

	// Build with docker mode
	dockerCmd := exec.Command(binaryPath)
	dockerCmd.Dir = tmpDir
	dockerCmd.Env = append(os.Environ(),
		"CONTAINER_BUILD_ENGINE=docker",
	)
	dockerOutput, err := dockerCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to build with docker: %v\nOutput: %s", err, dockerOutput)
	}

	// Run docker-built image and save output
	dockerImageOutput := runContainer(t, "docker", "test-comparison:latest")

	// Clean up docker image to avoid conflicts
	cleanupImage(t, "test-comparison:latest")

	// Build with kaniko mode (rebuilds the same image)
	kanikoCmd := exec.Command(binaryPath)
	kanikoCmd.Dir = tmpDir
	kanikoCmd.Env = append(os.Environ(),
		"CONTAINER_BUILD_ENGINE=kaniko",
	)
	kanikoOutput, err := kanikoCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to build with kaniko: %v\nOutput: %s", err, kanikoOutput)
	}

	// Run kaniko-built image and save output
	kanikoImageOutput := runContainer(t, "docker", "test-comparison:latest")

	// Verify both contain expected content
	expectedOutput := "test-layer\n"
	if dockerImageOutput != expectedOutput {
		t.Errorf("Docker image produced unexpected output: got %q, want %q", dockerImageOutput, expectedOutput)
	}
	if kanikoImageOutput != expectedOutput {
		t.Errorf("Kaniko image produced unexpected output: got %q, want %q", kanikoImageOutput, expectedOutput)
	}

	// Compare outputs
	if dockerImageOutput != kanikoImageOutput {
		t.Errorf("Docker and Kaniko produced different outputs:\nDocker: %s\nKaniko: %s",
			dockerImageOutput, kanikoImageOutput)
	}

	// Cleanup
	cleanupImage(t, "test-comparison:latest")
}

// runContainer runs a container and returns its output.
func runContainer(t *testing.T, engine, image string) string {
	t.Helper()
	cmd := exec.Command(engine, "run", "--rm", image)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to run container %s: %v\nOutput: %s", image, err, output)
	}
	return string(output)
}

// cleanupImage removes a container image (best effort).
func cleanupImage(t *testing.T, image string) {
	t.Helper()
	cmd := exec.Command("docker", "rmi", "-f", image)
	_ = cmd.Run() // Best effort cleanup
}
