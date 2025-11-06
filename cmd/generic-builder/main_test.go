//go:build unit

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge/internal/cmdutil"
)

func TestExecuteCommand(t *testing.T) {
	output := cmdutil.ExecuteCommand(cmdutil.ExecuteInput{
		Command: "echo",
		Args:    []string{"hello", "world"},
	})

	if output.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", output.ExitCode)
	}

	if !strings.Contains(output.Stdout, "hello world") {
		t.Errorf("Expected stdout to contain 'hello world', got: %s", output.Stdout)
	}
}

func TestExecuteCommandWithEnv(t *testing.T) {
	output := cmdutil.ExecuteCommand(cmdutil.ExecuteInput{
		Command: "sh",
		Args:    []string{"-c", "echo $TEST_VAR"},
		Env: map[string]string{
			"TEST_VAR": "test_value",
		},
	})

	if output.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", output.ExitCode)
	}

	if !strings.Contains(output.Stdout, "test_value") {
		t.Errorf("Expected stdout to contain 'test_value', got: %s", output.Stdout)
	}
}

func TestExecuteCommandWithWorkDir(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	output := cmdutil.ExecuteCommand(cmdutil.ExecuteInput{
		Command: "pwd",
		WorkDir: tmpDir,
	})

	if output.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", output.ExitCode)
	}

	if !strings.Contains(output.Stdout, tmpDir) {
		t.Errorf("Expected stdout to contain '%s', got: %s", tmpDir, output.Stdout)
	}
}

func TestExecuteCommandFailure(t *testing.T) {
	output := cmdutil.ExecuteCommand(cmdutil.ExecuteInput{
		Command: "command-that-does-not-exist-xyz123",
	})

	if output.ExitCode == 0 {
		t.Error("Expected non-zero exit code for non-existent command")
	}

	if output.Error == "" {
		t.Error("Expected error message for non-existent command")
	}
}

func TestLoadEnvFile(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".envrc")

	content := `# Comment line
TEST_KEY1=value1
export TEST_KEY2=value2
TEST_KEY3="value with spaces"

# Another comment
TEST_KEY4='single quoted'
`
	err := os.WriteFile(envFile, []byte(content), 0o644)
	if err != nil {
		t.Fatalf("Failed to create test .envrc: %v", err)
	}

	// Load env file
	envVars, err := cmdutil.LoadEnvFile(envFile)
	if err != nil {
		t.Fatalf("LoadEnvFile failed: %v", err)
	}

	// Verify values
	expected := map[string]string{
		"TEST_KEY1": "value1",
		"TEST_KEY2": "value2",
		"TEST_KEY3": "value with spaces",
		"TEST_KEY4": "single quoted",
	}

	for key, expectedValue := range expected {
		actualValue, ok := envVars[key]
		if !ok {
			t.Errorf("Expected key '%s' not found in env vars", key)
			continue
		}
		if actualValue != expectedValue {
			t.Errorf("For key '%s': expected '%s', got '%s'", key, expectedValue, actualValue)
		}
	}
}

func TestLoadEnvFileNonExistent(t *testing.T) {
	// Loading non-existent file should return empty map, not error
	envVars, err := cmdutil.LoadEnvFile("/path/that/does/not/exist/.envrc")
	if err != nil {
		t.Errorf("Expected no error for non-existent file, got: %v", err)
	}

	if len(envVars) != 0 {
		t.Errorf("Expected empty map for non-existent file, got %d entries", len(envVars))
	}
}

func TestLoadEnvFileWithComments(t *testing.T) {
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".envrc")

	content := `
# This is a comment
  # Indented comment

KEY1=value1


KEY2=value2
`
	err := os.WriteFile(envFile, []byte(content), 0o644)
	if err != nil {
		t.Fatalf("Failed to create test .envrc: %v", err)
	}

	envVars, err := cmdutil.LoadEnvFile(envFile)
	if err != nil {
		t.Fatalf("LoadEnvFile failed: %v", err)
	}

	if len(envVars) != 2 {
		t.Errorf("Expected 2 env vars, got %d", len(envVars))
	}

	if envVars["KEY1"] != "value1" {
		t.Errorf("Expected KEY1=value1, got KEY1=%s", envVars["KEY1"])
	}

	if envVars["KEY2"] != "value2" {
		t.Errorf("Expected KEY2=value2, got KEY2=%s", envVars["KEY2"])
	}
}
