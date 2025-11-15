# testutil - Shared Test Utilities Package

## Overview

The `internal/testutil` package provides a comprehensive set of utilities for writing consistent, maintainable tests across the forge codebase. It consolidates duplicate test code and provides standard patterns for subprocess execution, test lifecycle management, and common assertions.

**Package Path**: `github.com/alexandremahdhaoui/forge/internal/testutil`

## Quick Start

```go
import (
    "testing"
    "github.com/alexandremahdhaoui/forge/internal/testutil"
)

func TestMyIntegrationTest(t *testing.T) {
    // Create test environment with automatic cleanup
    env := testutil.NewTestEnvironment(t)

    // Run a command
    result := testutil.RunCommand(t, "forge", "build")
    testutil.ExpectSuccess(t, result)
    testutil.AssertContains(t, result.Stdout, "Successfully built")

    // Create a test environment (KIND cluster, registry, etc.)
    testID, err := env.CreateTestEnv("integration")
    if err != nil {
        t.Fatalf("Failed to create test env: %v", err)
    }

    // Test environment is automatically cleaned up via t.Cleanup()
}
```

## Package Components

### 1. Subprocess Execution (exec.go)

Unified subprocess execution with consistent timeout handling and output capture.

#### Types

```go
type ExecResult struct {
    Stdout   string
    Stderr   string
    ExitCode int
    Err      error
}
```

#### Functions

```go
// Run a command in current directory
func RunCommand(t *testing.T, command string, args ...string) ExecResult

// Run a command in specific directory
func RunCommandInDir(t *testing.T, dir, command string, args ...string) ExecResult

// Verify command output contains expected strings
func ExpectOutput(t *testing.T, result ExecResult, expectedStrings ...string)

// Assert command succeeded (exit code 0)
func ExpectSuccess(t *testing.T, result ExecResult)

// Assert command failed with specific error message
func ExpectFailure(t *testing.T, result ExecResult, expectedErrMsg string)
```

#### Example

```go
// Before (old pattern - 6 different variants)
cmd := exec.Command("forge", "build")
output, err := cmd.CombinedOutput()
if err != nil {
    t.Fatalf("Command failed: %v\nOutput: %s", err, output)
}
if !strings.Contains(string(output), "Successfully built") {
    t.Errorf("Expected 'Successfully built' in output")
}

// After (testutil pattern)
result := testutil.RunCommand(t, "forge", "build")
testutil.ExpectSuccess(t, result)
testutil.AssertContains(t, result.Stdout, "Successfully built")
```

### 2. Test Lifecycle Management (lifecycle.go)

Standardized test environment lifecycle with automatic cleanup.

#### Types

```go
type TestEnvironment struct {
    T              *testing.T
    TempDir        string
    ForgeBinary    string
    CleanupFuncs   []func() error
    // internal fields
}
```

#### Functions

```go
// Create new test environment with automatic cleanup
func NewTestEnvironment(t *testing.T) *TestEnvironment

// Create a forge test environment (KIND cluster, registry, etc.)
func (te *TestEnvironment) CreateTestEnv(stage string) (string, error)

// Register custom cleanup function
func (te *TestEnvironment) RegisterCleanup(fn func() error)

// Check if cleanup should be skipped (SKIP_CLEANUP env var)
func (te *TestEnvironment) SkipCleanup() bool

// Manual cleanup (usually automatic via t.Cleanup)
func (te *TestEnvironment) Cleanup()
```

#### Example

```go
// Before (old pattern - inconsistent cleanup)
tmpDir := t.TempDir()
oldWd, _ := os.Getwd()
defer os.Chdir(oldWd)
os.Chdir(tmpDir)

// Create test env
cmd := exec.Command("./build/bin/forge", "test", "create-env", "integration")
output, _ := cmd.CombinedOutput()
testID := extractTestID(string(output))

// Manual cleanup
defer func() {
    cleanupCmd := exec.Command("./build/bin/forge", "test", "delete-env", "integration", testID)
    cleanupCmd.Run()
}()

// After (testutil pattern)
env := testutil.NewTestEnvironment(t)
testID, err := env.CreateTestEnv("integration")
if err != nil {
    t.Fatalf("Failed to create test env: %v", err)
}
// Cleanup happens automatically via t.Cleanup()
```

### 3. Common Helper Functions (helpers.go)

Forge-specific test helpers for working with test environments and artifacts.

#### Functions

```go
// Extract test ID from command output
func ExtractTestID(output string) string

// Verify KIND cluster exists
func VerifyClusterExists(testID string) error

// Verify artifact store has test environment entry
func VerifyArtifactStoreHasTestEnv(testID string) error

// Verify artifact store doesn't have test environment entry
func VerifyArtifactStoreMissingTestEnv(testID string) error

// Force cleanup of test environment
func ForceCleanupTestEnv(testID string) error

// Force cleanup of all leftover resources
func ForceCleanupLeftovers() error

// Find forge binary in build/bin/
func FindForgeBinary() (string, error)

// Find forge repository root
func FindForgeRepository() (string, error)
```

#### Example

```go
// Before (old pattern - duplicate helpers in multiple files)
func extractTestID(output string) string {
    // ... 15 lines of implementation
}

// After (testutil pattern)
testID := testutil.ExtractTestID(output)
```

### 4. Assertion Helpers (assertions.go)

Consistent assertion functions for common test validations.

#### Functions

```go
// Assert string contains substring
func AssertContains(t *testing.T, actual, expected string)

// Assert string doesn't contain substring
func AssertNotContains(t *testing.T, actual, unexpected string)

// Assert values are equal
func AssertEqual(t *testing.T, expected, actual interface{})

// Assert values are not equal
func AssertNotEqual(t *testing.T, unexpected, actual interface{})

// Assert error occurred
func AssertError(t *testing.T, err error, msgAndArgs ...interface{})

// Assert no error occurred
func AssertNoError(t *testing.T, err error, msgAndArgs ...interface{})

// Assert file exists
func AssertFileExists(t *testing.T, path string)

// Assert file doesn't exist
func AssertFileNotExists(t *testing.T, path string)
```

#### Example

```go
// Before (old pattern - manual assertions)
if !strings.Contains(output, "expected") {
    t.Errorf("Expected to find %q in output: %s", "expected", output)
}

// After (testutil pattern)
testutil.AssertContains(t, output, "expected")
```

## Migration Guide

### Migrating from exec.Command

**Before:**
```go
cmd := exec.Command("forge", "build")
cmd.Dir = "/some/dir"
output, err := cmd.CombinedOutput()
if err != nil {
    t.Fatalf("Command failed: %v", err)
}
```

**After:**
```go
result := testutil.RunCommandInDir(t, "/some/dir", "forge", "build")
testutil.ExpectSuccess(t, result)
```

### Migrating Cleanup Code

**Before:**
```go
tmpDir := t.TempDir()
defer func() {
    // Manual cleanup
    os.RemoveAll(tmpDir)
}()
```

**After:**
```go
env := testutil.NewTestEnvironment(t)
// env.TempDir is available
// Cleanup is automatic via t.Cleanup()
```

### Migrating String Assertions

**Before:**
```go
if !strings.Contains(output, "SUCCESS") {
    t.Errorf("Expected SUCCESS in output")
}
```

**After:**
```go
testutil.AssertContains(t, output, "SUCCESS")
```

## Best Practices

### 1. Always Use testutil for Subprocess Execution

✅ **DO**: Use `testutil.RunCommand()` for all subprocess execution in tests
```go
result := testutil.RunCommand(t, "forge", "build")
```

❌ **DON'T**: Use `exec.Command` directly unless testing subprocess functionality itself
```go
cmd := exec.Command("forge", "build")
```

### 2. Use TestEnvironment for Integration Tests

✅ **DO**: Create `TestEnvironment` for consistent lifecycle
```go
env := testutil.NewTestEnvironment(t)
testID, _ := env.CreateTestEnv("integration")
```

❌ **DON'T**: Manually manage temp directories and cleanup
```go
tmpDir := t.TempDir()
defer cleanup() // Error-prone
```

### 3. Use Assertion Helpers

✅ **DO**: Use testutil assertions for clarity
```go
testutil.AssertContains(t, output, "expected")
testutil.AssertNoError(t, err)
```

❌ **DON'T**: Write manual assertion code
```go
if !strings.Contains(output, "expected") {
    t.Errorf("...")
}
```

### 4. Respect SKIP_CLEANUP for Debugging

When debugging test failures, set `SKIP_CLEANUP=1` to preserve test resources:

```bash
SKIP_CLEANUP=1 go test -run TestMyFailingTest
```

This leaves KIND clusters, temp directories, and other resources intact for inspection.

## Environment Variables

### Subprocess Execution
- `TEST_TIMEOUT` - Override default command timeout (default: 2 seconds)

### Test Environment
- `SKIP_CLEANUP` - Skip cleanup of test resources (default: false)
- `KIND_BINARY` - Path to KIND binary (default: "kind")
- `CONTAINER_ENGINE` - Container engine to use (default: "docker")

### Forge-Specific
- `FORGE_BINARY` - Path to forge binary (default: "./build/bin/forge")

## Package Architecture

```
internal/testutil/
├── exec.go           - Subprocess execution utilities
├── lifecycle.go      - Test environment lifecycle management
├── helpers.go        - Forge-specific helper functions
├── assertions.go     - Assertion helpers
├── exec_test.go      - Unit tests for exec utilities
├── lifecycle_test.go - Unit tests for lifecycle management
├── helpers_test.go   - Unit tests for helpers
├── assertions_test.go- Unit tests for assertions
└── README.md         - This file
```

## Testing the testutil Package

The testutil package itself is thoroughly tested:

```bash
# Run testutil unit tests
go test ./internal/testutil/

# Run testutil integration tests
go test ./internal/testutil/ -tags=integration
```

## Related Documentation

- [Forge Test Usage Guide](../../docs/forge-test-usage.md) - How to use `forge test` commands
- [Test Environment Architecture](../../docs/testenv-architecture.md) - How test environments work
- [CLAUDE.md](../../CLAUDE.md) - End-to-end test-driven workflow

## Support

For questions or issues with testutil:
1. Check examples in this README
2. Look at existing test files that use testutil
3. Review testutil source code (well-commented)
4. Check test files in `internal/testutil/*_test.go`

## Changelog

### v1.0.0 (2025-11-15)
- Initial release
- Consolidated 6 subprocess execution patterns into unified API
- Standardized test lifecycle management
- Added common assertion helpers
- Migrated 8+ test files to use testutil
- Eliminated ~230 lines of duplicate code
