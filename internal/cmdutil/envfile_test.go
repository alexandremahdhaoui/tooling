//go:build unit

package cmdutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEnvFile_NonExistentFile(t *testing.T) {
	envVars, err := LoadEnvFile("/non/existent/file.env")
	if err != nil {
		t.Errorf("Expected no error for non-existent file, got: %v", err)
	}
	if len(envVars) != 0 {
		t.Errorf("Expected empty map, got %d entries", len(envVars))
	}
}

func TestLoadEnvFile_ValidFile(t *testing.T) {
	// Create temporary test file
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, "test.env")

	content := `# This is a comment
KEY1=value1
KEY2="value with spaces"
export KEY3=value3
KEY4='single quoted'

# Another comment
KEY5=value5
`
	if err := os.WriteFile(envFile, []byte(content), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	envVars, err := LoadEnvFile(envFile)
	if err != nil {
		t.Fatalf("LoadEnvFile failed: %v", err)
	}

	expected := map[string]string{
		"KEY1": "value1",
		"KEY2": "value with spaces",
		"KEY3": "value3",
		"KEY4": "single quoted",
		"KEY5": "value5",
	}

	if len(envVars) != len(expected) {
		t.Errorf("Expected %d vars, got %d", len(expected), len(envVars))
	}

	for key, expectedValue := range expected {
		if actualValue, ok := envVars[key]; !ok {
			t.Errorf("Missing key: %s", key)
		} else if actualValue != expectedValue {
			t.Errorf("Key %s: expected %q, got %q", key, expectedValue, actualValue)
		}
	}
}

func TestLoadEnvFile_InvalidFormat(t *testing.T) {
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, "invalid.env")

	content := `KEY1=value1
INVALID_LINE_WITHOUT_EQUALS
KEY2=value2
`
	if err := os.WriteFile(envFile, []byte(content), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	_, err := LoadEnvFile(envFile)
	if err == nil {
		t.Error("Expected error for invalid format, got nil")
	}
}

func TestLoadEnvFile_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, "empty.env")

	if err := os.WriteFile(envFile, []byte(""), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	envVars, err := LoadEnvFile(envFile)
	if err != nil {
		t.Errorf("Expected no error for empty file, got: %v", err)
	}
	if len(envVars) != 0 {
		t.Errorf("Expected empty map, got %d entries", len(envVars))
	}
}

func TestLoadEnvFile_OnlyComments(t *testing.T) {
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, "comments.env")

	content := `# Comment 1
# Comment 2
# Comment 3
`
	if err := os.WriteFile(envFile, []byte(content), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	envVars, err := LoadEnvFile(envFile)
	if err != nil {
		t.Errorf("Expected no error for comments-only file, got: %v", err)
	}
	if len(envVars) != 0 {
		t.Errorf("Expected empty map, got %d entries", len(envVars))
	}
}

func TestLoadEnvFile_WhitespaceHandling(t *testing.T) {
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, "whitespace.env")

	content := `  KEY1  =  value1
KEY2="  spaces inside  "
  export   KEY3  =  value3
`
	if err := os.WriteFile(envFile, []byte(content), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	envVars, err := LoadEnvFile(envFile)
	if err != nil {
		t.Fatalf("LoadEnvFile failed: %v", err)
	}

	expected := map[string]string{
		"KEY1": "value1",
		"KEY2": "  spaces inside  ",
		"KEY3": "value3",
	}

	for key, expectedValue := range expected {
		if actualValue, ok := envVars[key]; !ok {
			t.Errorf("Missing key: %s", key)
		} else if actualValue != expectedValue {
			t.Errorf("Key %s: expected %q, got %q", key, expectedValue, actualValue)
		}
	}
}
