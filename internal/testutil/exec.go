package testutil

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// TestingT is the subset of testing.T methods that we use.
// This allows for easier testing of the testutil package itself.
type TestingT interface {
	Helper()
	Fatalf(format string, args ...interface{})
	Fatal(args ...interface{})
}

// ExecResult contains the results of a command execution.
type ExecResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Err      error
}

// getTestTimeout returns the test timeout duration.
// Default: 2 seconds. Override with TEST_TIMEOUT env var (e.g., "5s").
func getTestTimeout() time.Duration {
	if timeout := os.Getenv("TEST_TIMEOUT"); timeout != "" {
		if d, err := time.ParseDuration(timeout); err == nil {
			return d
		}
	}
	return 2 * time.Second
}

// RunCommand executes a command with standard timeout and output capture.
// It marks the calling test as a helper and captures both stdout and stderr.
// The command is executed with a default 2-second timeout (configurable via TEST_TIMEOUT env var).
func RunCommand(t TestingT, command string, args ...string) ExecResult {
	t.Helper()
	return runCommandImpl(t, "", command, args...)
}

// RunCommandInDir executes a command in a specific directory with standard timeout and output capture.
// It marks the calling test as a helper and captures both stdout and stderr.
// The command is executed with a default 2-second timeout (configurable via TEST_TIMEOUT env var).
func RunCommandInDir(t TestingT, dir, command string, args ...string) ExecResult {
	t.Helper()
	if dir == "" {
		t.Fatal("RunCommandInDir: dir parameter cannot be empty")
	}
	return runCommandImpl(t, dir, command, args...)
}

// runCommandImpl is the internal implementation for running commands.
func runCommandImpl(t TestingT, dir, command string, args ...string) ExecResult {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), getTestTimeout())
	defer cancel()

	cmd := exec.CommandContext(ctx, command, args...)
	if dir != "" {
		cmd.Dir = dir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0

	if err != nil {
		// Handle different error types
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else if ctx.Err() == context.DeadlineExceeded {
			exitCode = -1
			err = fmt.Errorf("command timed out after %v: %w", getTestTimeout(), err)
		} else {
			exitCode = -1
		}
	}

	return ExecResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
		Err:      err,
	}
}

// ExpectOutput asserts that the command output contains all expected strings.
// It marks the calling test as a helper and fails the test if any expected string is not found.
func ExpectOutput(t TestingT, result ExecResult, expectedStrings ...string) {
	t.Helper()

	// Combine stdout and stderr for checking
	combinedOutput := result.Stdout + result.Stderr

	for _, expected := range expectedStrings {
		if !strings.Contains(combinedOutput, expected) {
			t.Fatalf("expected output to contain %q\nStdout: %s\nStderr: %s",
				expected, result.Stdout, result.Stderr)
		}
	}
}

// ExpectSuccess asserts that the command succeeded (exit code 0, no error).
// It marks the calling test as a helper and fails the test if the command failed.
func ExpectSuccess(t TestingT, result ExecResult) {
	t.Helper()

	if result.Err != nil {
		t.Fatalf("expected command to succeed, but it failed: %v\nStdout: %s\nStderr: %s",
			result.Err, result.Stdout, result.Stderr)
	}

	if result.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d\nStdout: %s\nStderr: %s",
			result.ExitCode, result.Stdout, result.Stderr)
	}
}

// ExpectFailure asserts that the command failed with a specific error message.
// It marks the calling test as a helper and fails the test if the command succeeded
// or if the error message doesn't match.
func ExpectFailure(t TestingT, result ExecResult, expectedErrMsg string) {
	t.Helper()

	if result.Err == nil {
		t.Fatalf("expected command to fail with %q, but it succeeded\nStdout: %s\nStderr: %s",
			expectedErrMsg, result.Stdout, result.Stderr)
		return
	}

	if result.ExitCode == 0 {
		t.Fatalf("expected non-zero exit code, but got 0")
		return
	}

	// Check if error message or stderr contains the expected message
	combinedOutput := result.Err.Error() + result.Stderr
	if !strings.Contains(combinedOutput, expectedErrMsg) {
		t.Fatalf("expected error/stderr to contain %q\nError: %v\nStderr: %s",
			expectedErrMsg, result.Err, result.Stderr)
	}
}
