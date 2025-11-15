//go:build unit

package testutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractTestID_ValidOutput(t *testing.T) {
	output := `Creating test environment...
test-integration-20251115-12345678
Successfully created environment`

	testID := ExtractTestID(output)
	if testID != "test-integration-20251115-12345678" {
		t.Fatalf("expected 'test-integration-20251115-12345678', got '%s'", testID)
	}
}

func TestExtractTestID_MultipleLines(t *testing.T) {
	output := `Line 1
Line 2
test-unit-20251115-abcd1234
Line 4`

	testID := ExtractTestID(output)
	if testID != "test-unit-20251115-abcd1234" {
		t.Fatalf("expected 'test-unit-20251115-abcd1234', got '%s'", testID)
	}
}

func TestExtractTestID_NoTestID(t *testing.T) {
	output := `No test ID in this output
Just some random text`

	testID := ExtractTestID(output)
	if testID != "" {
		t.Fatalf("expected empty string, got '%s'", testID)
	}
}

func TestExtractTestID_InvalidFormat(t *testing.T) {
	output := `test-invalid
test-two-parts`

	testID := ExtractTestID(output)
	if testID != "" {
		t.Fatalf("expected empty string for invalid format, got '%s'", testID)
	}
}

func TestFindForgeBinary(t *testing.T) {
	// This test will try to find the forge binary
	// It should find it since we're running in the forge repository
	binary, err := FindForgeBinary()
	if err != nil {
		t.Fatalf("FindForgeBinary failed: %v", err)
	}

	if binary == "" {
		t.Fatal("FindForgeBinary returned empty string")
	}

	// Verify the binary exists
	if _, err := os.Stat(binary); err != nil {
		t.Fatalf("Binary path returned by FindForgeBinary doesn't exist: %s", binary)
	}

	// Verify it's in build/bin or PATH
	if !strings.Contains(binary, "build/bin/forge") && !strings.Contains(binary, "forge") {
		t.Fatalf("Binary path doesn't look correct: %s", binary)
	}
}

func TestFindForgeRepository(t *testing.T) {
	// This test should find the forge repository
	repoPath, err := FindForgeRepository()
	if err != nil {
		t.Fatalf("FindForgeRepository failed: %v", err)
	}

	if repoPath == "" {
		t.Fatal("FindForgeRepository returned empty string")
	}

	// Verify go.mod exists
	goModPath := filepath.Join(repoPath, "go.mod")
	if _, err := os.Stat(goModPath); err != nil {
		t.Fatalf("go.mod doesn't exist in repository root: %s", repoPath)
	}

	// Verify cmd/forge/main.go exists
	mainPath := filepath.Join(repoPath, "cmd", "forge", "main.go")
	if _, err := os.Stat(mainPath); err != nil {
		t.Fatalf("cmd/forge/main.go doesn't exist in repository root: %s", repoPath)
	}

	// Verify go.mod contains forge module
	goModData, err := os.ReadFile(goModPath)
	if err != nil {
		t.Fatalf("Failed to read go.mod: %v", err)
	}

	if !strings.Contains(string(goModData), "github.com/alexandremahdhaoui/forge") {
		t.Fatalf("go.mod doesn't contain forge module path")
	}
}

func TestVerifyClusterExists_NoKind(t *testing.T) {
	// Save original KIND_BINARY
	originalKind := os.Getenv("KIND_BINARY")
	defer func() {
		if originalKind == "" {
			os.Unsetenv("KIND_BINARY")
		} else {
			os.Setenv("KIND_BINARY", originalKind)
		}
	}()

	// Set KIND_BINARY to a non-existent command
	os.Setenv("KIND_BINARY", "nonexistent-kind-binary-12345")

	err := VerifyClusterExists("test-integration-20251115-12345678")
	if err == nil {
		t.Fatal("Expected error when KIND_BINARY doesn't exist")
	}
}

func TestExtractTestID_WithWhitespace(t *testing.T) {
	output := `

  test-integration-20251115-12345678

	`

	testID := ExtractTestID(output)
	if testID != "test-integration-20251115-12345678" {
		t.Fatalf("expected 'test-integration-20251115-12345678', got '%s'", testID)
	}
}

func TestForceCleanupTestEnv_EmptyTestID(t *testing.T) {
	// Should return nil without doing anything
	err := ForceCleanupTestEnv("")
	if err != nil {
		t.Fatalf("Expected nil error for empty testID, got: %v", err)
	}
}

// Note: We cannot easily test the actual cleanup functions without creating real resources
// Those are better covered by integration tests
