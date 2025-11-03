package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge/pkg/forge"
)

// TestIntegrationLifecycle tests the complete lifecycle: create, list, get, delete
func TestIntegrationLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Check if required binaries exist (KIND_BINARY env var must be set for actual kindenv setup)
	if os.Getenv("KIND_BINARY") == "" {
		t.Skip("KIND_BINARY not set, skipping integration lifecycle test")
	}

	// Change to repository root
	repoRoot := "../.."
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Failed to change to repo root: %v", err)
	}

	forgeBin := "./build/bin/forge"
	if _, err := os.Stat(forgeBin); os.IsNotExist(err) {
		t.Skip("forge binary not found, run build test first")
	}

	storePath := ".ignore.integration-envs.yaml"

	// Clean up any existing environments
	defer func() {
		// Try to delete any test environments
		store, _ := forge.ReadIntegrationEnvStore(storePath)
		for _, env := range store.Environments {
			if strings.HasPrefix(env.Name, "test-") {
				_ = exec.Command(forgeBin, "integration", "delete", env.ID).Run()
			}
		}
		_ = os.Remove(storePath)
	}()

	// Test 1: Create environment
	t.Log("Creating test environment...")
	createCmd := exec.Command(forgeBin, "integration", "create", "test-lifecycle")
	createOutput, err := createCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to create environment: %v\nOutput: %s", err, string(createOutput))
	}
	t.Logf("Create output:\n%s", string(createOutput))

	// Extract environment ID from output
	var envID string
	for _, line := range strings.Split(string(createOutput), "\n") {
		if strings.Contains(line, "ID:") {
			parts := strings.Split(line, "ID:")
			if len(parts) == 2 {
				envID = strings.TrimSpace(strings.TrimSuffix(parts[1], ")"))
			}
		}
	}

	if envID == "" {
		t.Fatal("Could not extract environment ID from create output")
	}
	t.Logf("Created environment ID: %s", envID)

	// Test 2: List environments
	t.Log("Listing environments...")
	listCmd := exec.Command(forgeBin, "integration", "list")
	listOutput, err := listCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to list environments: %v\nOutput: %s", err, string(listOutput))
	}
	t.Logf("List output:\n%s", string(listOutput))

	if !strings.Contains(string(listOutput), envID) {
		t.Errorf("Environment ID %s not found in list output", envID)
	}
	if !strings.Contains(string(listOutput), "test-lifecycle") {
		t.Error("Environment name 'test-lifecycle' not found in list output")
	}

	// Test 3: Get environment details
	t.Log("Getting environment details...")
	getCmd := exec.Command(forgeBin, "integration", "get", envID)
	getOutput, err := getCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to get environment: %v\nOutput: %s", err, string(getOutput))
	}
	t.Logf("Get output:\n%s", string(getOutput))

	if !strings.Contains(string(getOutput), envID) {
		t.Errorf("Environment ID %s not found in get output", envID)
	}
	if !strings.Contains(string(getOutput), "test-lifecycle") {
		t.Error("Environment name not found in get output")
	}

	// Test 4: Delete environment
	t.Log("Deleting environment...")
	deleteCmd := exec.Command(forgeBin, "integration", "delete", envID)
	deleteOutput, err := deleteCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to delete environment: %v\nOutput: %s", err, string(deleteOutput))
	}
	t.Logf("Delete output:\n%s", string(deleteOutput))

	// Test 5: Verify deletion
	t.Log("Verifying deletion...")
	listCmd2 := exec.Command(forgeBin, "integration", "list")
	listOutput2, err := listCmd2.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to list environments after deletion: %v\nOutput: %s", err, string(listOutput2))
	}

	if strings.Contains(string(listOutput2), envID) {
		t.Errorf("Environment ID %s still found after deletion", envID)
	}
}

// TestIntegrationCreateWithoutKind tests environment tracking without actual kind setup
func TestIntegrationCreateWithoutKind(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Change to repository root
	repoRoot := "../.."
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Failed to change to repo root: %v", err)
	}

	forgeBin := "./build/bin/forge"
	if _, err := os.Stat(forgeBin); os.IsNotExist(err) {
		t.Skip("forge binary not found, run build test first")
	}

	storePath := ".ignore.integration-envs.yaml"

	// Clean up
	defer os.Remove(storePath)

	// Temporarily disable kindenv by modifying config (we'll just test the store operations)
	// Note: This test will fail if kindenv is actually configured, which is acceptable
	// We're testing that the basic store operations work

	// Test store operations directly
	t.Log("Testing integration environment store operations...")

	// Read store (should be empty or create new)
	store, err := forge.ReadIntegrationEnvStore(storePath)
	if err != nil {
		t.Fatalf("Failed to read integration env store: %v", err)
	}

	initialCount := len(store.Environments)
	t.Logf("Initial environment count: %d", initialCount)

	// Add test environment
	testEnv := forge.IntegrationEnvironment{
		ID:      "test-env-123",
		Name:    "test-without-kind",
		Created: "2025-01-01T00:00:00Z",
		Components: map[string]forge.Component{
			"test": {
				Enabled: true,
				Ready:   false,
				ConnectionInfo: map[string]string{
					"test": "value",
				},
			},
		},
	}

	forge.AddEnvironment(&store, testEnv)

	// Write store
	if err := forge.WriteIntegrationEnvStore(storePath, store); err != nil {
		t.Fatalf("Failed to write integration env store: %v", err)
	}

	// Read back and verify
	store2, err := forge.ReadIntegrationEnvStore(storePath)
	if err != nil {
		t.Fatalf("Failed to read integration env store after write: %v", err)
	}

	if len(store2.Environments) != initialCount+1 {
		t.Errorf("Expected %d environments, got %d", initialCount+1, len(store2.Environments))
	}

	// Get environment
	env, err := forge.GetEnvironment(store2, "test-env-123")
	if err != nil {
		t.Fatalf("Failed to get environment: %v", err)
	}

	if env.Name != "test-without-kind" {
		t.Errorf("Expected name 'test-without-kind', got %s", env.Name)
	}

	// Delete environment
	if err := forge.DeleteEnvironment(&store2, "test-env-123"); err != nil {
		t.Fatalf("Failed to delete environment: %v", err)
	}

	if len(store2.Environments) != initialCount {
		t.Errorf("Expected %d environments after deletion, got %d", initialCount, len(store2.Environments))
	}

	t.Log("Integration environment store operations working correctly")
}
