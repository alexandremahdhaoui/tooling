//go:build unit

package cmdutil

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestExecuteCommand_SimpleSuccess(t *testing.T) {
	var cmd string
	var args []string

	if runtime.GOOS == "windows" {
		cmd = "cmd"
		args = []string{"/C", "echo", "hello"}
	} else {
		cmd = "echo"
		args = []string{"hello"}
	}

	output := ExecuteCommand(ExecuteInput{
		Command: cmd,
		Args:    args,
	})

	if output.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", output.ExitCode)
	}
	if !strings.Contains(output.Stdout, "hello") {
		t.Errorf("Expected stdout to contain 'hello', got: %q", output.Stdout)
	}
	if output.Error != "" {
		t.Errorf("Expected no error, got: %s", output.Error)
	}
}

func TestExecuteCommand_NonZeroExit(t *testing.T) {
	var cmd string
	var args []string

	if runtime.GOOS == "windows" {
		cmd = "cmd"
		args = []string{"/C", "exit", "42"}
	} else {
		cmd = "sh"
		args = []string{"-c", "exit 42"}
	}

	output := ExecuteCommand(ExecuteInput{
		Command: cmd,
		Args:    args,
	})

	if output.ExitCode != 42 {
		t.Errorf("Expected exit code 42, got %d", output.ExitCode)
	}
}

func TestExecuteCommand_InvalidCommand(t *testing.T) {
	output := ExecuteCommand(ExecuteInput{
		Command: "nonexistentcommandthatdoesnotexist12345",
		Args:    []string{},
	})

	if output.ExitCode == 0 {
		t.Error("Expected non-zero exit code for invalid command")
	}
	if output.Error == "" {
		t.Error("Expected error message for invalid command")
	}
}

func TestExecuteCommand_WithWorkDir(t *testing.T) {
	tmpDir := t.TempDir()

	var cmd string
	var args []string

	if runtime.GOOS == "windows" {
		cmd = "cmd"
		args = []string{"/C", "cd"}
	} else {
		cmd = "pwd"
		args = []string{}
	}

	output := ExecuteCommand(ExecuteInput{
		Command: cmd,
		Args:    args,
		WorkDir: tmpDir,
	})

	if output.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d (error: %s)", output.ExitCode, output.Error)
	}

	// Output should contain the temp directory path
	if !strings.Contains(output.Stdout, filepath.Base(tmpDir)) {
		t.Errorf("Expected stdout to contain temp dir, got: %q", output.Stdout)
	}
}

func TestExecuteCommand_WithEnvVars(t *testing.T) {
	var cmd string
	var args []string

	if runtime.GOOS == "windows" {
		cmd = "cmd"
		args = []string{"/C", "echo", "%TEST_VAR%"}
	} else {
		cmd = "sh"
		args = []string{"-c", "echo $TEST_VAR"}
	}

	output := ExecuteCommand(ExecuteInput{
		Command: cmd,
		Args:    args,
		Env: map[string]string{
			"TEST_VAR": "test_value",
		},
	})

	if output.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", output.ExitCode)
	}
	if !strings.Contains(output.Stdout, "test_value") {
		t.Errorf("Expected stdout to contain 'test_value', got: %q", output.Stdout)
	}
}

func TestExecuteCommand_WithEnvFile(t *testing.T) {
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, "test.env")

	envContent := `TEST_VAR1=value1
TEST_VAR2=value2
`
	if err := os.WriteFile(envFile, []byte(envContent), 0o644); err != nil {
		t.Fatalf("Failed to create env file: %v", err)
	}

	var cmd string
	var args []string

	if runtime.GOOS == "windows" {
		cmd = "cmd"
		args = []string{"/C", "echo", "%TEST_VAR1%"}
	} else {
		cmd = "sh"
		args = []string{"-c", "echo $TEST_VAR1"}
	}

	output := ExecuteCommand(ExecuteInput{
		Command: cmd,
		Args:    args,
		EnvFile: envFile,
	})

	if output.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", output.ExitCode)
	}
	if !strings.Contains(output.Stdout, "value1") {
		t.Errorf("Expected stdout to contain 'value1', got: %q", output.Stdout)
	}
}

func TestExecuteCommand_EnvFileTakesPrecedenceOverSystem(t *testing.T) {
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, "test.env")

	// Set a system env var
	os.Setenv("CMDUTIL_TEST_VAR", "system_value")
	defer os.Unsetenv("CMDUTIL_TEST_VAR")

	// Override in env file
	envContent := `CMDUTIL_TEST_VAR=file_value
`
	if err := os.WriteFile(envFile, []byte(envContent), 0o644); err != nil {
		t.Fatalf("Failed to create env file: %v", err)
	}

	var cmd string
	var args []string

	if runtime.GOOS == "windows" {
		cmd = "cmd"
		args = []string{"/C", "echo", "%CMDUTIL_TEST_VAR%"}
	} else {
		cmd = "sh"
		args = []string{"-c", "echo $CMDUTIL_TEST_VAR"}
	}

	output := ExecuteCommand(ExecuteInput{
		Command: cmd,
		Args:    args,
		EnvFile: envFile,
	})

	if output.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", output.ExitCode)
	}
	// Should see file_value, not system_value
	if !strings.Contains(output.Stdout, "file_value") {
		t.Errorf("Expected stdout to contain 'file_value', got: %q", output.Stdout)
	}
}

func TestExecuteCommand_InlineEnvTakesPrecedence(t *testing.T) {
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, "test.env")

	envContent := `CMDUTIL_TEST_VAR2=file_value
`
	if err := os.WriteFile(envFile, []byte(envContent), 0o644); err != nil {
		t.Fatalf("Failed to create env file: %v", err)
	}

	var cmd string
	var args []string

	if runtime.GOOS == "windows" {
		cmd = "cmd"
		args = []string{"/C", "echo", "%CMDUTIL_TEST_VAR2%"}
	} else {
		cmd = "sh"
		args = []string{"-c", "echo $CMDUTIL_TEST_VAR2"}
	}

	output := ExecuteCommand(ExecuteInput{
		Command: cmd,
		Args:    args,
		EnvFile: envFile,
		Env: map[string]string{
			"CMDUTIL_TEST_VAR2": "inline_value",
		},
	})

	if output.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", output.ExitCode)
	}
	// Should see inline_value (highest precedence)
	if !strings.Contains(output.Stdout, "inline_value") {
		t.Errorf("Expected stdout to contain 'inline_value', got: %q", output.Stdout)
	}
}

func TestExecuteCommand_InvalidEnvFile(t *testing.T) {
	output := ExecuteCommand(ExecuteInput{
		Command: "echo",
		Args:    []string{"test"},
		EnvFile: "/invalid/path/to/nonexistent.env",
	})

	// Non-existent env file should not cause error (returns empty map)
	if output.ExitCode != 0 {
		t.Errorf("Expected exit code 0 for non-existent env file, got %d", output.ExitCode)
	}
}
