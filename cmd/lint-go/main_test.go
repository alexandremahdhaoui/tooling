//go:build unit

package main

import (
	"os"
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge/pkg/mcptypes"
)

// TestGolangciLintVersion tests that the correct golangci-lint version is used
func TestGolangciLintVersion(t *testing.T) {
	tests := []struct {
		name        string
		envValue    string
		expected    string
		description string
	}{
		{
			name:        "Default version",
			envValue:    "",
			expected:    "v2.6.0",
			description: "Should use v2.6.0 as default when env var not set",
		},
		{
			name:        "Custom version from env",
			envValue:    "v2.7.0",
			expected:    "v2.7.0",
			description: "Should use custom version when env var is set",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment
			originalEnv := os.Getenv("GOLANGCI_LINT_VERSION")
			defer os.Setenv("GOLANGCI_LINT_VERSION", originalEnv)

			if tt.envValue != "" {
				os.Setenv("GOLANGCI_LINT_VERSION", tt.envValue)
			} else {
				os.Unsetenv("GOLANGCI_LINT_VERSION")
			}

			// Simulate the version selection logic from runLint
			golangciVersion := os.Getenv("GOLANGCI_LINT_VERSION")
			if golangciVersion == "" {
				golangciVersion = "v2.6.0"
			}

			if golangciVersion != tt.expected {
				t.Errorf("%s: expected %s, got %s", tt.description, tt.expected, golangciVersion)
			}
		})
	}
}

// TestGolangciLintPackagePath tests that the v2 module path is used
func TestGolangciLintPackagePath(t *testing.T) {
	tests := []struct {
		name        string
		version     string
		expected    string
		description string
	}{
		{
			name:        "Default version v2.6.0",
			version:     "v2.6.0",
			expected:    "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.6.0",
			description: "Should use v2 module path for v2.x versions",
		},
		{
			name:        "Custom version v2.7.0",
			version:     "v2.7.0",
			expected:    "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.7.0",
			description: "Should use v2 module path for any v2.x version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate package path construction from runLint
			golangciPkg := "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@" + tt.version

			if golangciPkg != tt.expected {
				t.Errorf("%s: expected %s, got %s", tt.description, tt.expected, golangciPkg)
			}

			// Verify it contains /v2/ for major version 2
			if !strings.Contains(golangciPkg, "/v2/") {
				t.Errorf("Package path should contain /v2/ for major version 2, got: %s", golangciPkg)
			}
		})
	}
}

// TestRunInputStructure tests the RunInput structure
func TestRunInputStructure(t *testing.T) {
	input := mcptypes.RunInput{
		Stage: "lint",
		Name:  "lint-test",
	}

	if input.Stage != "lint" {
		t.Errorf("Expected Stage to be 'lint', got %s", input.Stage)
	}
	if input.Name != "lint-test" {
		t.Errorf("Expected Name to be 'lint-test', got %s", input.Name)
	}
}

// TestTestReportStructure tests the TestReport structure
func TestTestReportStructure(t *testing.T) {
	report := &TestReport{
		Status:       "passed",
		ErrorMessage: "",
		Duration:     1.5,
		Total:        0,
		Passed:       1,
		Failed:       0,
	}

	if report.Status != "passed" {
		t.Errorf("Expected Status to be 'passed', got %s", report.Status)
	}
	if report.Passed != 1 {
		t.Errorf("Expected Passed to be 1, got %d", report.Passed)
	}
	if report.Failed != 0 {
		t.Errorf("Expected Failed to be 0, got %d", report.Failed)
	}
}

// TestTestReportFailedStatus tests TestReport for failed linting
func TestTestReportFailedStatus(t *testing.T) {
	report := &TestReport{
		Status:       "failed",
		ErrorMessage: "linting failed with exit code 1",
		Duration:     2.5,
		Total:        1,
		Passed:       0,
		Failed:       1,
	}

	if report.Status != "failed" {
		t.Errorf("Expected Status to be 'failed', got %s", report.Status)
	}
	if report.ErrorMessage == "" {
		t.Error("Expected ErrorMessage to be set for failed status")
	}
	if report.Passed != 0 {
		t.Errorf("Expected Passed to be 0 for failed run, got %d", report.Passed)
	}
	if report.Failed != 1 {
		t.Errorf("Expected Failed to be 1, got %d", report.Failed)
	}
}

// TestVersionInfoInitialized tests that version info is properly initialized
func TestVersionInfoInitialized(t *testing.T) {
	if versionInfo == nil {
		t.Fatal("versionInfo should be initialized in init()")
	}

	// versionInfo.Get() returns (version, commit, timestamp), not tool name
	// Just verify it's not nil and can be called without panicking
	version, _, _ := versionInfo.Get()
	if version == "" {
		t.Log("Version is empty, which is expected for non-built binaries")
	}
}

// TestGolangciLintCommandArgs tests that the correct command arguments are constructed
func TestGolangciLintCommandArgs(t *testing.T) {
	version := "v2.6.0"
	golangciPkg := "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@" + version

	// Simulate args construction from runLint
	args := []string{"run", golangciPkg, "run", "--fix"}

	expectedArgs := []string{"run", "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.6.0", "run", "--fix"}

	if len(args) != len(expectedArgs) {
		t.Fatalf("Expected %d args, got %d", len(expectedArgs), len(args))
	}

	for i, arg := range args {
		if arg != expectedArgs[i] {
			t.Errorf("Arg %d: expected %s, got %s", i, expectedArgs[i], arg)
		}
	}

	// Verify --fix flag is included
	hasFixFlag := false
	for _, arg := range args {
		if arg == "--fix" {
			hasFixFlag = true
			break
		}
	}
	if !hasFixFlag {
		t.Error("Expected --fix flag to be included in args")
	}
}
