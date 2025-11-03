package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/alexandremahdhaoui/forge/pkg/forge"
)

// TestBuildIntegration tests the forge build command end-to-end
func TestBuildIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Change to repository root
	repoRoot := "../.."
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Failed to change to repo root: %v", err)
	}

	// Build forge binary first if not exists
	forgeBin := "./build/bin/forge"
	if _, err := os.Stat(forgeBin); os.IsNotExist(err) {
		cmd := exec.Command("go", "build", "-o", forgeBin, "./cmd/forge")
		if err := cmd.Run(); err != nil {
			t.Fatalf("Failed to build forge: %v", err)
		}
	}

	// Clean up artifact store before test
	artifactStorePath := ".ignore.artifact-store.yaml"
	_ = os.Remove(artifactStorePath)

	// Run forge build
	cmd := exec.Command(forgeBin, "build")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("forge build failed: %v\nOutput: %s", err, string(output))
	}

	t.Logf("Build output:\n%s", string(output))

	// Verify artifact store was created
	if _, err := os.Stat(artifactStorePath); os.IsNotExist(err) {
		t.Fatal("Artifact store was not created")
	}

	// Read and verify artifact store
	store, err := forge.ReadArtifactStore(artifactStorePath)
	if err != nil {
		t.Fatalf("Failed to read artifact store: %v", err)
	}

	if len(store.Artifacts) == 0 {
		t.Fatal("No artifacts in store")
	}

	t.Logf("Found %d artifacts in store", len(store.Artifacts))

	// Verify expected binaries exist
	expectedBinaries := []string{"forge", "build-go", "build-container", "kindenv", "local-container-registry", "test-go"}
	for _, name := range expectedBinaries {
		found := false
		for _, artifact := range store.Artifacts {
			if artifact.Name == name && artifact.Type == "binary" {
				found = true
				// Verify the binary file exists
				binaryPath := filepath.Join("./build/bin", name)
				if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
					t.Errorf("Binary not found: %s", binaryPath)
				}
				break
			}
		}
		if !found {
			t.Errorf("Artifact not found in store: %s", name)
		}
	}
}

func TestBuildSingleArtifact(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Change to repository root
	repoRoot := "../.."
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Failed to change to repo root: %v", err)
	}

	forgeBin := "./build/bin/forge"
	if _, err := os.Stat(forgeBin); os.IsNotExist(err) {
		t.Skip("forge binary not found, run full build test first")
	}

	// Clean up test binary
	testBinPath := "./build/bin/test-go"
	_ = os.Remove(testBinPath)

	// Build only test-go
	cmd := exec.Command(forgeBin, "build", "test-go")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("forge build test-go failed: %v\nOutput: %s", err, string(output))
	}

	// Verify test-go binary exists
	if _, err := os.Stat(testBinPath); os.IsNotExist(err) {
		t.Fatal("test-go binary was not built")
	}

	t.Log("Successfully built single artifact: test-go")
}

func TestBuildNonexistentArtifact(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Change to repository root
	repoRoot := "../.."
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Failed to change to repo root: %v", err)
	}

	forgeBin := "./build/bin/forge"
	if _, err := os.Stat(forgeBin); os.IsNotExist(err) {
		t.Skip("forge binary not found, run full build test first")
	}

	// Try to build nonexistent artifact
	cmd := exec.Command(forgeBin, "build", "nonexistent-artifact")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("Expected error for nonexistent artifact, but got none")
	}

	outputStr := string(output)
	if len(outputStr) == 0 {
		t.Fatal("Expected error message, but got empty output")
	}

	t.Logf("Got expected error: %s", outputStr)
}
