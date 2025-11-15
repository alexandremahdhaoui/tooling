//go:build unit

package testutil

import (
	"os"
	"testing"
)

func TestNewTestEnvironment(t *testing.T) {
	env := NewTestEnvironment(t)

	if env.T == nil {
		t.Fatal("expected T to be set")
	}

	if env.TempDir == "" {
		t.Fatal("expected TempDir to be set")
	}

	// Verify temp dir exists
	if _, err := os.Stat(env.TempDir); os.IsNotExist(err) {
		t.Fatalf("temp dir doesn't exist: %s", env.TempDir)
	}

	if env.CleanupFuncs == nil {
		t.Fatal("expected CleanupFuncs to be initialized")
	}

	if env.testEnvIDs == nil {
		t.Fatal("expected testEnvIDs to be initialized")
	}
}

func TestTestEnvironment_RegisterCleanup(t *testing.T) {
	env := NewTestEnvironment(t)

	called := false
	env.RegisterCleanup(func() error {
		called = true
		return nil
	})

	if len(env.CleanupFuncs) != 1 {
		t.Fatalf("expected 1 cleanup function, got %d", len(env.CleanupFuncs))
	}

	// Call cleanup manually to test
	env.CleanupFuncs[0]()

	if !called {
		t.Fatal("cleanup function was not called")
	}
}

func TestTestEnvironment_SkipCleanup_NotSet(t *testing.T) {
	// Ensure SKIP_CLEANUP is not set
	originalSkip := os.Getenv("SKIP_CLEANUP")
	os.Unsetenv("SKIP_CLEANUP")
	defer func() {
		if originalSkip != "" {
			os.Setenv("SKIP_CLEANUP", originalSkip)
		}
	}()

	env := NewTestEnvironment(t)

	if env.SkipCleanup() {
		t.Fatal("expected SkipCleanup to return false when SKIP_CLEANUP is not set")
	}
}

func TestTestEnvironment_SkipCleanup_Set(t *testing.T) {
	// Set SKIP_CLEANUP
	originalSkip := os.Getenv("SKIP_CLEANUP")
	os.Setenv("SKIP_CLEANUP", "1")
	defer func() {
		if originalSkip == "" {
			os.Unsetenv("SKIP_CLEANUP")
		} else {
			os.Setenv("SKIP_CLEANUP", originalSkip)
		}
	}()

	env := NewTestEnvironment(t)

	if !env.SkipCleanup() {
		t.Fatal("expected SkipCleanup to return true when SKIP_CLEANUP is set")
	}
}

func TestTestEnvironment_Cleanup_LIFO_Order(t *testing.T) {
	env := NewTestEnvironment(t)

	// Track call order
	var callOrder []int

	env.RegisterCleanup(func() error {
		callOrder = append(callOrder, 1)
		return nil
	})

	env.RegisterCleanup(func() error {
		callOrder = append(callOrder, 2)
		return nil
	})

	env.RegisterCleanup(func() error {
		callOrder = append(callOrder, 3)
		return nil
	})

	// Manually call cleanup to test order
	// (automatic cleanup is already registered via t.Cleanup())
	// We need to temporarily disable SKIP_CLEANUP
	originalSkip := os.Getenv("SKIP_CLEANUP")
	os.Unsetenv("SKIP_CLEANUP")
	defer func() {
		if originalSkip != "" {
			os.Setenv("SKIP_CLEANUP", originalSkip)
		}
	}()

	// Call cleanup
	env.Cleanup()

	// Verify LIFO order (3, 2, 1)
	if len(callOrder) != 3 {
		t.Fatalf("expected 3 cleanup calls, got %d", len(callOrder))
	}

	if callOrder[0] != 3 {
		t.Fatalf("expected first call to be 3, got %d", callOrder[0])
	}

	if callOrder[1] != 2 {
		t.Fatalf("expected second call to be 2, got %d", callOrder[1])
	}

	if callOrder[2] != 1 {
		t.Fatalf("expected third call to be 1, got %d", callOrder[2])
	}
}

func TestTestEnvironment_Cleanup_RespectsSkipCleanup(t *testing.T) {
	// Set SKIP_CLEANUP
	originalSkip := os.Getenv("SKIP_CLEANUP")
	os.Setenv("SKIP_CLEANUP", "1")
	defer func() {
		if originalSkip == "" {
			os.Unsetenv("SKIP_CLEANUP")
		} else {
			os.Setenv("SKIP_CLEANUP", originalSkip)
		}
	}()

	env := NewTestEnvironment(t)

	called := false
	env.RegisterCleanup(func() error {
		called = true
		return nil
	})

	// Call cleanup - should skip because SKIP_CLEANUP is set
	env.Cleanup()

	if called {
		t.Fatal("cleanup should not have been called when SKIP_CLEANUP is set")
	}
}

// Note: CreateTestEnv is not unit tested here because it requires
// a real forge binary and creates actual resources. It should be
// tested in integration tests.
