//go:build integration

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestBuildWithSpec_CustomArgs tests building with custom args from spec field.
func TestBuildWithSpec_CustomArgs(t *testing.T) {
	t.Skip("Temporarily skipped during package rename migration - test relies on published version with new package names")
	// Create a temporary directory for the test
	tmpDir := t.TempDir()

	// Create a simple Go program
	mainGo := `package main

import "fmt"

func main() {
	fmt.Println("Hello from test binary")
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainGo), 0o644); err != nil {
		t.Fatalf("Failed to write main.go: %v", err)
	}

	// Create go.mod
	goMod := `module testbinary

go 1.23
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	// Create forge.yaml with spec containing custom args
	forgeYAML := `name: test-project
artifactStorePath: .ignore.artifact-store.yaml

build:
  - name: testbinary
    src: .
    dest: ./build/bin
    engine: go://go-build
    spec:
      args:
        - "-ldflags=-w -s"
      env:
        CGO_ENABLED: "0"
`
	if err := os.WriteFile(filepath.Join(tmpDir, "forge.yaml"), []byte(forgeYAML), 0o644); err != nil {
		t.Fatalf("Failed to write forge.yaml: %v", err)
	}

	// Change to temp directory
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Initialize git repo (required for versioning)
	if err := exec.Command("git", "init").Run(); err != nil {
		t.Fatalf("Failed to init git: %v", err)
	}
	if err := exec.Command("git", "config", "user.email", "test@test.com").Run(); err != nil {
		t.Fatalf("Failed to config git email: %v", err)
	}
	if err := exec.Command("git", "config", "user.name", "Test User").Run(); err != nil {
		t.Fatalf("Failed to config git name: %v", err)
	}
	if err := exec.Command("git", "add", ".").Run(); err != nil {
		t.Fatalf("Failed to git add: %v", err)
	}
	if err := exec.Command("git", "commit", "-m", "initial").Run(); err != nil {
		t.Fatalf("Failed to git commit: %v", err)
	}

	// Get forge repo root (go up from cmd/go-build to repo root)
	forgeRepoRoot, err := filepath.Abs(filepath.Join(originalDir, "../.."))
	if err != nil {
		t.Fatalf("Failed to get forge repo root: %v", err)
	}

	// Run forge build
	forgeExe := filepath.Join(forgeRepoRoot, "build/bin/forge")
	cmd := exec.Command(forgeExe, "build")
	cmd.Env = append(os.Environ(), "FORGE_REPO_PATH="+forgeRepoRoot)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("forge build failed: %v\nOutput: %s", err, string(output))
	}

	t.Logf("forge build output:\n%s", string(output))

	// Verify binary was created
	binaryPath := filepath.Join(tmpDir, "build/bin/testbinary")
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Fatalf("Binary not created at %s", binaryPath)
	}

	// Verify binary runs
	runCmd := exec.Command(binaryPath)
	runOutput, err := runCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Binary execution failed: %v\nOutput: %s", err, string(runOutput))
	}

	expectedOutput := "Hello from test binary\n"
	if string(runOutput) != expectedOutput {
		t.Errorf("Binary output = %q, want %q", string(runOutput), expectedOutput)
	}

	t.Log("✅ Successfully built and ran binary with custom args from spec")
}

// TestBuildWithSpec_CustomEnv tests building with custom environment variables from spec field.
func TestBuildWithSpec_CustomEnv(t *testing.T) {
	t.Skip("Temporarily skipped during package rename migration - test relies on published version with new package names")
	// Create a temporary directory for the test
	tmpDir := t.TempDir()

	// Create a simple Go program
	mainGo := `package main

import "fmt"

func main() {
	fmt.Println("Hello from cross-compiled binary")
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainGo), 0o644); err != nil {
		t.Fatalf("Failed to write main.go: %v", err)
	}

	// Create go.mod
	goMod := `module testbinary

go 1.23
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	// Create forge.yaml with spec containing custom env
	// Note: We'll build for the current platform to ensure it can run
	forgeYAML := `name: test-project
artifactStorePath: .ignore.artifact-store.yaml

build:
  - name: testbinary
    src: .
    dest: ./build/bin
    engine: go://go-build
    spec:
      env:
        CGO_ENABLED: "0"
`
	if err := os.WriteFile(filepath.Join(tmpDir, "forge.yaml"), []byte(forgeYAML), 0o644); err != nil {
		t.Fatalf("Failed to write forge.yaml: %v", err)
	}

	// Change to temp directory
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Initialize git repo (required for versioning)
	if err := exec.Command("git", "init").Run(); err != nil {
		t.Fatalf("Failed to init git: %v", err)
	}
	if err := exec.Command("git", "config", "user.email", "test@test.com").Run(); err != nil {
		t.Fatalf("Failed to config git email: %v", err)
	}
	if err := exec.Command("git", "config", "user.name", "Test User").Run(); err != nil {
		t.Fatalf("Failed to config git name: %v", err)
	}
	if err := exec.Command("git", "add", ".").Run(); err != nil {
		t.Fatalf("Failed to git add: %v", err)
	}
	if err := exec.Command("git", "commit", "-m", "initial").Run(); err != nil {
		t.Fatalf("Failed to git commit: %v", err)
	}

	// Get forge repo root (go up from cmd/go-build to repo root)
	forgeRepoRoot, err := filepath.Abs(filepath.Join(originalDir, "../.."))
	if err != nil {
		t.Fatalf("Failed to get forge repo root: %v", err)
	}

	// Run forge build
	forgeExe := filepath.Join(forgeRepoRoot, "build/bin/forge")
	cmd := exec.Command(forgeExe, "build")
	cmd.Env = append(os.Environ(), "FORGE_REPO_PATH="+forgeRepoRoot)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("forge build failed: %v\nOutput: %s", err, string(output))
	}

	t.Logf("forge build output:\n%s", string(output))

	// Verify binary was created
	binaryPath := filepath.Join(tmpDir, "build/bin/testbinary")
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Fatalf("Binary not created at %s", binaryPath)
	}

	// Verify binary runs (it should run since we built for current platform)
	runCmd := exec.Command(binaryPath)
	runOutput, err := runCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Binary execution failed: %v\nOutput: %s", err, string(runOutput))
	}

	expectedOutput := "Hello from cross-compiled binary\n"
	if string(runOutput) != expectedOutput {
		t.Errorf("Binary output = %q, want %q", string(runOutput), expectedOutput)
	}

	t.Log("✅ Successfully built and ran binary with custom env from spec")
}
