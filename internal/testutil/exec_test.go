//go:build unit

package testutil

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestRunCommand_Success(t *testing.T) {
	result := RunCommand(t, "echo", "test output")

	if result.Err != nil {
		t.Fatalf("expected no error, got: %v", result.Err)
	}

	if result.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got: %d", result.ExitCode)
	}

	if !strings.Contains(result.Stdout, "test output") {
		t.Fatalf("expected stdout to contain 'test output', got: %s", result.Stdout)
	}
}

func TestRunCommand_Failure(t *testing.T) {
	result := RunCommand(t, "sh", "-c", "echo error message >&2 && exit 1")

	if result.Err == nil {
		t.Fatal("expected error, got nil")
	}

	if result.ExitCode != 1 {
		t.Fatalf("expected exit code 1, got: %d", result.ExitCode)
	}

	if !strings.Contains(result.Stderr, "error message") {
		t.Fatalf("expected stderr to contain 'error message', got: %s", result.Stderr)
	}
}

func TestRunCommand_CommandNotFound(t *testing.T) {
	result := RunCommand(t, "nonexistent-command-12345")

	if result.Err == nil {
		t.Fatal("expected error for nonexistent command, got nil")
	}

	if result.ExitCode == 0 {
		t.Fatal("expected non-zero exit code for nonexistent command")
	}
}

func TestRunCommand_Timeout(t *testing.T) {
	// Set a very short timeout for this test
	originalTimeout := os.Getenv("TEST_TIMEOUT")
	os.Setenv("TEST_TIMEOUT", "100ms")
	defer func() {
		if originalTimeout == "" {
			os.Unsetenv("TEST_TIMEOUT")
		} else {
			os.Setenv("TEST_TIMEOUT", originalTimeout)
		}
	}()

	result := RunCommand(t, "sleep", "10")

	if result.Err == nil {
		t.Fatal("expected timeout error, got nil")
	}

	// Check for either "timed out" or "killed" as the command may be killed before timeout message is set
	errStr := result.Err.Error()
	if !strings.Contains(errStr, "timed out") && !strings.Contains(errStr, "killed") {
		t.Fatalf("expected 'timed out' or 'killed' in error, got: %v", result.Err)
	}

	if result.ExitCode != -1 {
		t.Fatalf("expected exit code -1 for timeout, got: %d", result.ExitCode)
	}
}

func TestRunCommandInDir_Success(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file in the temp directory
	testFile := "testfile.txt"
	if err := os.WriteFile(tmpDir+"/"+testFile, []byte("content"), 0o644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	result := RunCommandInDir(t, tmpDir, "ls")

	if result.Err != nil {
		t.Fatalf("expected no error, got: %v", result.Err)
	}

	if !strings.Contains(result.Stdout, testFile) {
		t.Fatalf("expected stdout to contain %q, got: %s", testFile, result.Stdout)
	}
}

func TestRunCommandInDir_EmptyDir(t *testing.T) {
	// We can't easily test this without causing the test to fail,
	// so we just verify the behavior in integration
	// This is a limitation of testing Fatal() calls
	t.Skip("Skipping test for Fatal() call - covered by integration tests")
}

func TestExpectOutput_Success(t *testing.T) {
	result := ExecResult{
		Stdout: "hello world",
		Stderr: "warning message",
	}

	// Should not fail
	ExpectOutput(t, result, "hello", "world")
	ExpectOutput(t, result, "warning")
}

func TestExpectOutput_Failure(t *testing.T) {
	result := ExecResult{
		Stdout: "hello world",
		Stderr: "warning message",
	}

	// Create a mock test that we expect to fail
	mockT := &mockTestingT{t: t}
	ExpectOutput(mockT, result, "missing string")

	if !mockT.failed {
		t.Fatal("expected ExpectOutput to fail, but it didn't")
	}
}

func TestExpectSuccess_Success(t *testing.T) {
	result := ExecResult{
		Stdout:   "output",
		Stderr:   "",
		ExitCode: 0,
		Err:      nil,
	}

	// Should not fail
	ExpectSuccess(t, result)
}

func TestExpectSuccess_Failure_WithError(t *testing.T) {
	result := ExecResult{
		Stdout:   "output",
		Stderr:   "error",
		ExitCode: 1,
		Err:      os.ErrNotExist,
	}

	mockT := &mockTestingT{t: t}
	ExpectSuccess(mockT, result)

	if !mockT.failed {
		t.Fatal("expected ExpectSuccess to fail, but it didn't")
	}
}

func TestExpectSuccess_Failure_NonZeroExitCode(t *testing.T) {
	result := ExecResult{
		Stdout:   "output",
		Stderr:   "error",
		ExitCode: 1,
		Err:      nil,
	}

	mockT := &mockTestingT{t: t}
	ExpectSuccess(mockT, result)

	if !mockT.failed {
		t.Fatal("expected ExpectSuccess to fail, but it didn't")
	}
}

func TestExpectFailure_Success(t *testing.T) {
	result := ExecResult{
		Stdout:   "",
		Stderr:   "file not found",
		ExitCode: 1,
		Err:      os.ErrNotExist,
	}

	// Should not fail
	ExpectFailure(t, result, "not found")
}

func TestExpectFailure_UnexpectedSuccess(t *testing.T) {
	result := ExecResult{
		Stdout:   "output",
		Stderr:   "",
		ExitCode: 0,
		Err:      nil,
	}

	mockT := &mockTestingT{t: t}
	ExpectFailure(mockT, result, "some error")

	if !mockT.failed {
		t.Fatal("expected ExpectFailure to fail when command succeeded, but it didn't")
	}
}

func TestGetTestTimeout_Default(t *testing.T) {
	// Ensure TEST_TIMEOUT is not set
	originalTimeout := os.Getenv("TEST_TIMEOUT")
	os.Unsetenv("TEST_TIMEOUT")
	defer func() {
		if originalTimeout != "" {
			os.Setenv("TEST_TIMEOUT", originalTimeout)
		}
	}()

	timeout := getTestTimeout()
	if timeout != 2*time.Second {
		t.Fatalf("expected default timeout of 2s, got: %v", timeout)
	}
}

func TestGetTestTimeout_CustomValid(t *testing.T) {
	originalTimeout := os.Getenv("TEST_TIMEOUT")
	os.Setenv("TEST_TIMEOUT", "5s")
	defer func() {
		if originalTimeout == "" {
			os.Unsetenv("TEST_TIMEOUT")
		} else {
			os.Setenv("TEST_TIMEOUT", originalTimeout)
		}
	}()

	timeout := getTestTimeout()
	if timeout != 5*time.Second {
		t.Fatalf("expected timeout of 5s, got: %v", timeout)
	}
}

func TestGetTestTimeout_InvalidFallsBackToDefault(t *testing.T) {
	originalTimeout := os.Getenv("TEST_TIMEOUT")
	os.Setenv("TEST_TIMEOUT", "invalid")
	defer func() {
		if originalTimeout == "" {
			os.Unsetenv("TEST_TIMEOUT")
		} else {
			os.Setenv("TEST_TIMEOUT", originalTimeout)
		}
	}()

	timeout := getTestTimeout()
	if timeout != 2*time.Second {
		t.Fatalf("expected fallback to default timeout of 2s, got: %v", timeout)
	}
}

// mockTestingT is a minimal mock of testing.T for testing helper functions
type mockTestingT struct {
	t      *testing.T
	failed bool
}

func (m *mockTestingT) Helper() {}

func (m *mockTestingT) Fatalf(format string, args ...interface{}) {
	m.failed = true
	// Don't actually fail the test, just mark that we would have
}

func (m *mockTestingT) Fatal(args ...interface{}) {
	m.failed = true
	// Don't actually fail the test, just mark that we would have
}
