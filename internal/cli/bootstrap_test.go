//go:build unit

package cli

import (
	"errors"
	"testing"
)

// TestConfigValidation tests that Config struct accepts all required fields.
func TestConfigValidation(t *testing.T) {
	cfg := Config{
		Name:           "test-cmd",
		Version:        "1.0.0",
		CommitSHA:      "abc123",
		BuildTimestamp: "2024-01-01",
		RunCLI:         func() error { return nil },
		RunMCP:         func() error { return nil },
		SuccessHandler: func() {},
		FailureHandler: func(error) {},
	}

	// Verify all fields are set correctly
	if cfg.Name != "test-cmd" {
		t.Errorf("Expected Name 'test-cmd', got %s", cfg.Name)
	}
	if cfg.Version != "1.0.0" {
		t.Errorf("Expected Version '1.0.0', got %s", cfg.Version)
	}
	if cfg.CommitSHA != "abc123" {
		t.Errorf("Expected CommitSHA 'abc123', got %s", cfg.CommitSHA)
	}
	if cfg.BuildTimestamp != "2024-01-01" {
		t.Errorf("Expected BuildTimestamp '2024-01-01', got %s", cfg.BuildTimestamp)
	}
	if cfg.RunCLI == nil {
		t.Error("RunCLI should not be nil")
	}
	if cfg.RunMCP == nil {
		t.Error("RunMCP should not be nil")
	}
	if cfg.SuccessHandler == nil {
		t.Error("SuccessHandler should not be nil")
	}
	if cfg.FailureHandler == nil {
		t.Error("FailureHandler should not be nil")
	}
}

// TestConfigOptionalFields tests that optional fields can be nil.
func TestConfigOptionalFields(t *testing.T) {
	cfg := Config{
		Name:    "test-cmd",
		Version: "1.0.0",
		RunCLI:  func() error { return nil },
		// RunMCP, SuccessHandler, FailureHandler are nil
	}

	if cfg.RunMCP != nil {
		t.Error("RunMCP should be nil when not provided")
	}
	if cfg.SuccessHandler != nil {
		t.Error("SuccessHandler should be nil when not provided")
	}
	if cfg.FailureHandler != nil {
		t.Error("FailureHandler should be nil when not provided")
	}
}

// TestRunCLIExecution tests that RunCLI functions are callable.
func TestRunCLIExecution(t *testing.T) {
	called := false
	runCLI := func() error {
		called = true
		return nil
	}

	cfg := Config{
		Name:    "test-cmd",
		Version: "1.0.0",
		RunCLI:  runCLI,
	}

	err := cfg.RunCLI()
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if !called {
		t.Error("RunCLI was not called")
	}
}

// TestRunCLIError tests that RunCLI can return errors.
func TestRunCLIError(t *testing.T) {
	expectedErr := errors.New("test error")
	runCLI := func() error {
		return expectedErr
	}

	cfg := Config{
		Name:    "test-cmd",
		Version: "1.0.0",
		RunCLI:  runCLI,
	}

	err := cfg.RunCLI()
	if err == nil {
		t.Error("Expected error, got nil")
	}
	if err.Error() != "test error" {
		t.Errorf("Expected 'test error', got: %v", err)
	}
}

// TestRunMCPExecution tests that RunMCP functions are callable.
func TestRunMCPExecution(t *testing.T) {
	called := false
	runMCP := func() error {
		called = true
		return nil
	}

	cfg := Config{
		Name:    "test-cmd",
		Version: "1.0.0",
		RunCLI:  func() error { return nil },
		RunMCP:  runMCP,
	}

	err := cfg.RunMCP()
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if !called {
		t.Error("RunMCP was not called")
	}
}

// TestSuccessHandler tests that SuccessHandler is callable.
func TestSuccessHandler(t *testing.T) {
	called := false
	handler := func() {
		called = true
	}

	cfg := Config{
		Name:           "test-cmd",
		Version:        "1.0.0",
		RunCLI:         func() error { return nil },
		SuccessHandler: handler,
	}

	cfg.SuccessHandler()
	if !called {
		t.Error("SuccessHandler was not called")
	}
}

// TestFailureHandler tests that FailureHandler is callable.
func TestFailureHandler(t *testing.T) {
	called := false
	var receivedErr error
	handler := func(err error) {
		called = true
		receivedErr = err
	}

	testErr := errors.New("test error")
	cfg := Config{
		Name:           "test-cmd",
		Version:        "1.0.0",
		RunCLI:         func() error { return nil },
		FailureHandler: handler,
	}

	cfg.FailureHandler(testErr)
	if !called {
		t.Error("FailureHandler was not called")
	}
	if receivedErr == nil {
		t.Error("Expected error to be passed to FailureHandler")
	}
	if receivedErr.Error() != "test error" {
		t.Errorf("Expected 'test error', got: %v", receivedErr)
	}
}

// TestBootstrapIntegration provides basic integration test structure.
// Note: Full integration testing with os.Exit() is done via manual testing
// or end-to-end tests, as unit testing os.Exit() is complex.
func TestBootstrapIntegration(t *testing.T) {
	t.Skip("Integration test for Bootstrap requires manual testing due to os.Exit() calls")

	// This test documents expected Bootstrap behavior:
	// 1. Check version flags -> versionInfo.Print() + os.Exit(0)
	// 2. Check --mcp flag -> RunMCP() + os.Exit based on error
	// 3. Run CLI mode -> RunCLI() + handlers + os.Exit based on error
}
