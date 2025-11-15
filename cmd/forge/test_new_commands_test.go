//go:build unit

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge/pkg/forge"
)

// TestIsTestReportStage tests the helper function that identifies test-report stages
func TestIsTestReportStage(t *testing.T) {
	tests := []struct {
		name     string
		testSpec *forge.TestSpec
		want     bool
	}{
		{
			name: "test-report stage",
			testSpec: &forge.TestSpec{
				Name:    "unit",
				Testenv: "go://test-report",
			},
			want: true,
		},
		{
			name: "non test-report stage",
			testSpec: &forge.TestSpec{
				Name:    "integration",
				Testenv: "go://testenv",
			},
			want: false,
		},
		{
			name: "empty testenv",
			testSpec: &forge.TestSpec{
				Name:    "custom",
				Testenv: "",
			},
			want: false,
		},
		{
			name: "noop testenv",
			testSpec: &forge.TestSpec{
				Name:    "noop",
				Testenv: "noop",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsTestReportStage(tt.testSpec)
			if got != tt.want {
				t.Errorf("IsTestReportStage() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestStringSliceContains tests the helper function
func TestStringSliceContains(t *testing.T) {
	tests := []struct {
		name  string
		slice []string
		s     string
		want  bool
	}{
		{
			name:  "contains element",
			slice: []string{"run", "list", "get"},
			s:     "list",
			want:  true,
		},
		{
			name:  "does not contain element",
			slice: []string{"run", "list", "get"},
			s:     "delete",
			want:  false,
		},
		{
			name:  "empty slice",
			slice: []string{},
			s:     "test",
			want:  false,
		},
		{
			name:  "nil slice",
			slice: nil,
			s:     "test",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stringSliceContains(tt.slice, tt.s)
			if got != tt.want {
				t.Errorf("stringSliceContains() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestParseOutputFormat tests the output format parser
func TestParseOutputFormat(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		wantFormat    outputFormat
		wantRemaining int
	}{
		{
			name:          "no format flag",
			args:          []string{"arg1", "arg2"},
			wantFormat:    outputFormatTable,
			wantRemaining: 2,
		},
		{
			name:          "json format with -o",
			args:          []string{"-o", "json", "arg1"},
			wantFormat:    outputFormatJSON,
			wantRemaining: 1,
		},
		{
			name:          "yaml format with -o",
			args:          []string{"-o", "yaml", "arg1"},
			wantFormat:    outputFormatYAML,
			wantRemaining: 1,
		},
		{
			name:          "json format with -ojson",
			args:          []string{"-ojson", "arg1"},
			wantFormat:    outputFormatJSON,
			wantRemaining: 1,
		},
		{
			name:          "yaml format with -oyaml",
			args:          []string{"-oyaml", "arg1"},
			wantFormat:    outputFormatYAML,
			wantRemaining: 1,
		},
		{
			name:          "format at end",
			args:          []string{"arg1", "arg2", "-o", "json"},
			wantFormat:    outputFormatJSON,
			wantRemaining: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotFormat, gotRemaining := parseOutputFormat(tt.args)
			if gotFormat != tt.wantFormat {
				t.Errorf("parseOutputFormat() format = %v, want %v", gotFormat, tt.wantFormat)
			}
			if len(gotRemaining) != tt.wantRemaining {
				t.Errorf("parseOutputFormat() remaining = %d args, want %d", len(gotRemaining), tt.wantRemaining)
			}
		})
	}
}

// TestFindTestSpec tests finding test specs by name
func TestFindTestSpec(t *testing.T) {
	specs := []forge.TestSpec{
		{Name: "unit", Testenv: "go://test-report"},
		{Name: "integration", Testenv: "go://testenv"},
		{Name: "e2e", Testenv: "go://test-report"},
	}

	tests := []struct {
		name      string
		specs     []forge.TestSpec
		stageName string
		wantNil   bool
		wantName  string
	}{
		{
			name:      "find existing spec",
			specs:     specs,
			stageName: "unit",
			wantNil:   false,
			wantName:  "unit",
		},
		{
			name:      "find integration spec",
			specs:     specs,
			stageName: "integration",
			wantNil:   false,
			wantName:  "integration",
		},
		{
			name:      "spec not found",
			specs:     specs,
			stageName: "nonexistent",
			wantNil:   true,
		},
		{
			name:      "empty specs",
			specs:     []forge.TestSpec{},
			stageName: "unit",
			wantNil:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findTestSpec(tt.specs, tt.stageName)
			if tt.wantNil {
				if got != nil {
					t.Errorf("findTestSpec() = %v, want nil", got)
				}
			} else {
				if got == nil {
					t.Errorf("findTestSpec() = nil, want non-nil")
				} else if got.Name != tt.wantName {
					t.Errorf("findTestSpec().Name = %v, want %v", got.Name, tt.wantName)
				}
			}
		})
	}
}

// TestCommandParserValidation tests command parser validation
func TestCommandParserValidation(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		wantError     bool
		errorContains string
	}{
		{
			name:          "no arguments",
			args:          []string{},
			wantError:     true,
			errorContains: "usage",
		},
		{
			name:          "invalid subcommand",
			args:          []string{"invalid", "unit"},
			wantError:     true,
			errorContains: "unknown subcommand",
		},
		{
			name:      "valid run command",
			args:      []string{"run", "unit"},
			wantError: false,
		},
		{
			name:      "valid list command",
			args:      []string{"list", "unit"},
			wantError: false,
		},
		{
			name:      "valid get command with ID",
			args:      []string{"get", "unit", "test-id-123"},
			wantError: false,
		},
		{
			name:      "valid delete command",
			args:      []string{"delete", "unit", "test-id-123"},
			wantError: false,
		},
		{
			name:      "valid list-env command",
			args:      []string{"list-env", "unit"},
			wantError: false,
		},
		{
			name:      "valid get-env command",
			args:      []string{"get-env", "unit", "default"},
			wantError: false,
		},
		{
			name:      "valid create-env command",
			args:      []string{"create-env", "integration"},
			wantError: false,
		},
		{
			name:      "valid delete-env command",
			args:      []string{"delete-env", "integration", "env-id-123"},
			wantError: false,
		},
		{
			name:          "subcommand without stage",
			args:          []string{"run"},
			wantError:     true,
			errorContains: "usage",
		},
	}

	// Create a temporary forge.yaml for testing
	tmpDir := t.TempDir()
	forgeYaml := filepath.Join(tmpDir, "forge.yaml")
	content := `name: test-project
artifactStorePath: .ignore.artifact-store.yaml

test:
  - name: unit
    runner: go://go-test
    testenv: go://test-report
  - name: integration
    runner: go://go-test
    testenv: go://testenv
`
	if err := os.WriteFile(forgeYaml, []byte(content), 0o644); err != nil {
		t.Fatalf("Failed to create test forge.yaml: %v", err)
	}

	// Change to temp directory
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(oldWd)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := runTest(tt.args)

			if tt.wantError {
				if err == nil {
					t.Errorf("runTest() error = nil, want error containing %q", tt.errorContains)
				} else if !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("runTest() error = %q, want error containing %q", err.Error(), tt.errorContains)
				}
			} else {
				// For valid commands without actual execution, we might get other errors
				// but not validation errors about command format
				if err != nil {
					errMsg := err.Error()
					if strings.Contains(errMsg, "unknown subcommand") ||
						strings.Contains(errMsg, "usage: forge test") {
						t.Errorf("runTest() unexpected validation error = %v", err)
					}
				}
			}
		})
	}
}

// TestTestCreateEnvRejectsTestReport tests that create-env rejects test-report stages
func TestTestCreateEnvRejectsTestReport(t *testing.T) {
	testSpec := &forge.TestSpec{
		Name:    "unit",
		Testenv: "go://test-report",
	}

	err := testCreateEnv(testSpec)
	if err == nil {
		t.Error("testCreateEnv() should return error for test-report stage")
	}

	errMsg := err.Error()
	expectedPhrases := []string{
		"uses test-report",
		"no environment creation",
		"forge test run",
		"forge test list",
	}

	for _, phrase := range expectedPhrases {
		if !strings.Contains(strings.ToLower(errMsg), strings.ToLower(phrase)) {
			t.Errorf("testCreateEnv() error should contain %q, got: %v", phrase, errMsg)
		}
	}
}

// TestTestDeleteEnvRejectsTestReport tests that delete-env rejects test-report stages
func TestTestDeleteEnvRejectsTestReport(t *testing.T) {
	testSpec := &forge.TestSpec{
		Name:    "unit",
		Testenv: "go://test-report",
	}

	err := testDeleteEnv(testSpec, []string{"default"})
	if err == nil {
		t.Error("testDeleteEnv() should return error for test-report stage")
	}

	errMsg := err.Error()
	expectedPhrases := []string{
		"uses test-report",
		"no environment exists",
		"forge test delete",
	}

	for _, phrase := range expectedPhrases {
		if !strings.Contains(strings.ToLower(errMsg), strings.ToLower(phrase)) {
			t.Errorf("testDeleteEnv() error should contain %q, got: %v", phrase, errMsg)
		}
	}
}

// TestTestDeleteEnvRequiresEnvID tests that delete-env requires an ENV_ID
func TestTestDeleteEnvRequiresEnvID(t *testing.T) {
	testSpec := &forge.TestSpec{
		Name:    "integration",
		Testenv: "go://testenv",
	}

	err := testDeleteEnv(testSpec, []string{})
	if err == nil {
		t.Error("testDeleteEnv() should return error when ENV_ID missing")
	}

	if !strings.Contains(err.Error(), "usage") {
		t.Errorf("testDeleteEnv() error should contain 'usage', got: %v", err)
	}
}
