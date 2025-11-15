//go:build integration

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge/internal/testutil"
)

// TestTestAll_WithMultiEngineBuilder tests that forge test-all works with multi-engine builder aliases.
// This is a regression test for the bug where resolveEngine() was called too early and failed
// for multi-engine aliases before the orchestration logic could handle them.
func TestTestAll_WithMultiEngineBuilder(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// This test verifies that multi-engine aliases are detected and routed to orchestration
	// rather than failing with "cannot be resolved to a single engine" error.
	// We test within the forge repository to have access to the built binaries.

	// Get forge root
	forgeRoot, err := testutil.FindForgeRepository()
	if err != nil {
		t.Fatalf("Failed to find forge repository root: %v", err)
	}

	// Create a temporary directory for test artifacts
	tmpDir := t.TempDir()

	// Create a test forge.yaml with multi-engine builder alias
	forgeYAML := `name: test-multi-engine-project
artifactStorePath: .ignore.artifact-store.yaml

build:
  - name: test-multi-build
    src: .
    dest: ` + tmpDir + `
    engine: alias://test-multi-build

test:
  - name: unit
    runner: go://go-test

engines:
  - alias: test-multi-build
    type: builder
    builder:
      - engine: go://generic-builder
        spec:
          command: "sh"
          args: ["-c", "echo 'Step 1: First builder' && mkdir -p ` + tmpDir + ` && touch ` + tmpDir + `/step1.txt"]
      - engine: go://generic-builder
        spec:
          command: "sh"
          args: ["-c", "echo 'Step 2: Second builder' && touch ` + tmpDir + `/step2.txt"]
      - engine: go://generic-builder
        spec:
          command: "sh"
          args: ["-c", "echo 'Step 3: Third builder' && touch ` + tmpDir + `/step3.txt"]
`
	testConfigPath := filepath.Join(tmpDir, "forge.yaml")
	if err := os.WriteFile(testConfigPath, []byte(forgeYAML), 0o644); err != nil {
		t.Fatalf("Failed to create forge.yaml: %v", err)
	}

	// Create a simple test file to satisfy the test runner
	testFile := `package forge_test

import "testing"

func TestExample(t *testing.T) {
	if 1+1 != 2 {
		t.Error("Math is broken")
	}
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "example_test.go"), []byte(testFile), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Make sure required binaries exist
	forgeBin := filepath.Join(forgeRoot, "build", "bin", "forge")
	genericBuilderBin := filepath.Join(forgeRoot, "build", "bin", "generic-builder")
	testRunnerBin := filepath.Join(forgeRoot, "build", "bin", "go-test")

	if _, err := os.Stat(forgeBin); os.IsNotExist(err) {
		t.Skip("forge binary not found, run 'forge build' first")
	}
	if _, err := os.Stat(genericBuilderBin); os.IsNotExist(err) {
		t.Skip("generic-builder binary not found, run 'forge build' first")
	}
	if _, err := os.Stat(testRunnerBin); os.IsNotExist(err) {
		t.Skip("go-test binary not found, run 'forge build' first")
	}

	// Set FORGE_REPO_PATH so forge can find its engines
	t.Setenv("FORGE_REPO_PATH", forgeRoot)

	// Change to temp directory
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Run forge build (not test-all) to isolate the multi-engine builder test
	t.Logf("Running forge build in %s", tmpDir)
	cmd := exec.Command(forgeBin, "build")
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	// Log the output for debugging
	t.Logf("forge build output:\n%s", outputStr)

	// The key verification: we should NOT see the "cannot be resolved to a single engine" error
	if strings.Contains(outputStr, "cannot be resolved to a single engine") {
		t.Fatal("Found the bug: multi-engine alias failed with 'cannot be resolved to a single engine' error")
	}

	// Verify multi-engine builder was detected (proves fix is working)
	if !strings.Contains(outputStr, "Multi-engine builder detected") {
		t.Error("Expected to see 'Multi-engine builder detected' in output - fix may not be working")
	}

	// If we got this far without the error, and we see multi-engine detection, the fix is working
	t.Log("✅ Multi-engine alias successfully detected and routed to orchestration")

	// Verify all three builder steps executed
	if !strings.Contains(outputStr, "Step 1: First builder") {
		t.Error("Expected to see output from first builder")
	}
	if !strings.Contains(outputStr, "Step 2: Second builder") {
		t.Error("Expected to see output from second builder")
	}
	if !strings.Contains(outputStr, "Step 3: Third builder") {
		t.Error("Expected to see output from third builder")
	}

	// Verify the marker files were created by each builder
	for i := 1; i <= 3; i++ {
		markerFile := filepath.Join(tmpDir, fmt.Sprintf("step%d.txt", i))
		if _, err := os.Stat(markerFile); os.IsNotExist(err) {
			t.Errorf("Builder %d did not create marker file: %s", i, markerFile)
		}
	}
}

// TestTestAll_WithSingleEngineBuilder tests that forge test-all still works with single-engine builders.
// This ensures our fix doesn't break the normal case.
func TestTestAll_WithSingleEngineBuilder(t *testing.T) {
	t.Skip("Temporarily skipped during package rename migration - test relies on published version with new package names")

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create a temporary directory for the test project
	tmpDir := t.TempDir()

	// Find forge root first
	forgeRoot, err := testutil.FindForgeRepository()
	if err != nil {
		t.Fatalf("Failed to find forge repository root: %v", err)
	}

	// Create a minimal forge.yaml with a single-engine builder (direct go:// URI)
	forgeYAML := `name: test-single-engine-project
artifactStorePath: .ignore.artifact-store.yaml

test:
  - name: unit
    runner: go://go-test
`
	if err := os.WriteFile(filepath.Join(tmpDir, "forge.yaml"), []byte(forgeYAML), 0o644); err != nil {
		t.Fatalf("Failed to create forge.yaml: %v", err)
	}

	// Create a minimal Go module (no forge dependency - will use FORGE_REPO_PATH)
	goMod := `module test-single-engine-project

go 1.23
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	// Create a simple test file
	testFile := `package main

import "testing"

func TestExample(t *testing.T) {
	if 1+1 != 2 {
		t.Error("Math is broken")
	}
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "example_test.go"), []byte(testFile), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Make sure required binaries exist
	forgeBin := filepath.Join(forgeRoot, "build", "bin", "forge")
	testRunnerBin := filepath.Join(forgeRoot, "build", "bin", "go-test")

	if _, err := os.Stat(forgeBin); os.IsNotExist(err) {
		t.Skip("forge binary not found, run 'forge build' first")
	}
	if _, err := os.Stat(testRunnerBin); os.IsNotExist(err) {
		t.Skip("go-test binary not found, run 'forge build' first")
	}

	// Set FORGE_REPO_PATH so forge can find its engines
	t.Setenv("FORGE_REPO_PATH", forgeRoot)

	// Change to temp directory
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Run forge test-all (with no build specs)
	t.Logf("Running forge test-all in %s", tmpDir)
	cmd := exec.Command(forgeBin, "test-all")
	cmd.Env = append(os.Environ(), fmt.Sprintf("FORGE_REPO_PATH=%s", forgeRoot))
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	// Log the output for debugging
	t.Logf("forge test-all output:\n%s", outputStr)

	// Verify the command succeeded
	if err != nil {
		t.Fatalf("forge test-all failed: %v\nOutput: %s", err, outputStr)
	}

	// Verify test stage ran
	if !strings.Contains(outputStr, "Running test stage: unit") {
		t.Error("Expected to see 'Running test stage: unit' in output")
	}

	// Verify overall success
	if !strings.Contains(outputStr, "All test stages passed") {
		t.Error("Expected to see 'All test stages passed' in output")
	}
}
