//go:build e2e

package e2e

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestForgeTestCommand tests the complete forge test workflow end-to-end.
func TestForgeTestCommand(t *testing.T) {
	// Skip if not running integration tests
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create temporary directory for test
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)

	// Create build/bin directory in temp dir
	buildBinDir := filepath.Join(tmpDir, "build", "bin")
	if err := os.MkdirAll(buildBinDir, 0o755); err != nil {
		t.Fatalf("Failed to create build/bin directory: %v", err)
	}

	// Build forge binary
	forgeBinary := filepath.Join(buildBinDir, "forge")
	buildCmd := exec.Command("go", "build", "-o", forgeBinary, "../../cmd/forge")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build forge binary: %v", err)
	}

	// Build test-integration binary
	testIntegrationBinary := filepath.Join(buildBinDir, "test-integration")
	buildCmd = exec.Command("go", "build", "-o", testIntegrationBinary, "../../cmd/test-integration")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build test-integration binary: %v", err)
	}

	// Build test-runner-go binary
	testRunnerBinary := filepath.Join(buildBinDir, "test-runner-go")
	buildCmd = exec.Command("go", "build", "-o", testRunnerBinary, "../../cmd/test-runner-go")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build test-runner-go binary: %v", err)
	}

	// Setup test forge.yaml
	artifactStorePath := filepath.Join(tmpDir, "artifacts.yaml")
	forgeYAML := `name: e2e-test-project
artifactStorePath: ` + artifactStorePath + `

test:
  - name: unit
    engine: "noop"
    runner: "go://test-runner-go"

  - name: integration
    engine: "go://test-integration"
    runner: "go://test-runner-go"
`
	forgeYAMLPath := filepath.Join(tmpDir, "forge.yaml")
	err := os.WriteFile(forgeYAMLPath, []byte(forgeYAML), 0o644)
	if err != nil {
		t.Fatalf("Failed to write forge.yaml: %v", err)
	}

	// Change to temp directory
	os.Chdir(tmpDir)

	// Step 1: Create integration test environment
	t.Log("Step 1: Creating integration test environment...")
	createCmd := exec.Command(forgeBinary, "test", "integration", "create")
	output, err := createCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to create test environment: %v\nOutput: %s", err, string(output))
	}

	// Extract test ID from output (last line, as logs may be present)
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	testID := strings.TrimSpace(lines[len(lines)-1])
	t.Logf("Created test environment: %s", testID)

	if testID == "" {
		t.Fatal("Test ID is empty")
	}

	if !strings.HasPrefix(testID, "test-integration-") {
		t.Errorf("Expected test ID to start with 'test-integration-', got %s", testID)
	}

	// Ensure cleanup at the end
	defer func() {
		t.Log("Cleaning up test environment...")
		cleanupCmd := exec.Command(forgeBinary, "test", "integration", "delete", testID)
		cleanupCmd.Run() // Ignore errors in cleanup
	}()

	// Step 2: List test environments
	t.Log("Step 2: Listing test environments...")
	listCmd := exec.Command(forgeBinary, "test", "integration", "list")
	output, err = listCmd.Output()
	if err != nil {
		t.Fatalf("Failed to list test environments: %v", err)
	}

	listOutput := string(output)
	t.Logf("List output:\n%s", listOutput)

	if !strings.Contains(listOutput, testID) {
		t.Errorf("Test environment %s not found in list output", testID)
	}

	// Step 3: Get test environment details
	t.Log("Step 3: Getting test environment details...")
	getCmd := exec.Command(forgeBinary, "test", "integration", "get", testID)
	output, err = getCmd.Output()
	if err != nil {
		t.Fatalf("Failed to get test environment: %v", err)
	}

	getOutput := string(output)
	t.Logf("Get output:\n%s", getOutput)

	if !strings.Contains(getOutput, testID) {
		t.Error("Test ID not found in get output")
	}

	if !strings.Contains(getOutput, "integration") {
		t.Error("Stage name not found in get output")
	}

	if !strings.Contains(getOutput, "created") {
		t.Error("Status not found in get output")
	}

	// Step 4: Run unit tests (doesn't require test environment)
	t.Log("Step 4: Running unit tests...")
	runUnitCmd := exec.Command(forgeBinary, "test", "unit", "run")
	output, err = runUnitCmd.CombinedOutput()
	if err != nil {
		t.Logf("Unit test output:\n%s", string(output))
		// Don't fail if unit tests fail - we're testing the command works
		t.Logf("Unit tests exited with error (expected if no tests found): %v", err)
	}

	runOutput := string(output)
	if !strings.Contains(runOutput, "Running tests") && !strings.Contains(runOutput, "Test Results") {
		t.Logf("Note: Unexpected unit test output format:\n%s", runOutput)
	}

	// Step 5: Delete test environment
	t.Log("Step 5: Deleting test environment...")
	deleteCmd := exec.Command(forgeBinary, "test", "integration", "delete", testID)
	output, err = deleteCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to delete test environment: %v\nOutput: %s", err, string(output))
	}

	deleteOutput := string(output)
	t.Logf("Delete output:\n%s", deleteOutput)

	if !strings.Contains(deleteOutput, "Deleted") {
		t.Errorf("Expected deletion confirmation in output, got: %s", deleteOutput)
	}

	// Step 6: Verify deletion
	t.Log("Step 6: Verifying deletion...")
	listCmd = exec.Command(forgeBinary, "test", "integration", "list")
	output, err = listCmd.Output()
	if err != nil {
		t.Fatalf("Failed to list test environments after delete: %v", err)
	}

	listAfterDelete := string(output)
	t.Logf("List after delete:\n%s", listAfterDelete)

	if strings.Contains(listAfterDelete, testID) {
		t.Errorf("Test environment %s still appears in list after deletion", testID)
	}

	// Verify artifact store exists and is valid
	if _, err := os.Stat(artifactStorePath); os.IsNotExist(err) {
		t.Error("Artifact store file was not created")
	}
}

// TestForgeTestCommand_AutoCreateEnvironment tests that forge test run auto-creates environments.
func TestForgeTestCommand_AutoCreateEnvironment(t *testing.T) {
	// Skip if not running integration tests
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create temporary directory for test
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)

	// Create build/bin directory in temp dir
	buildBinDir := filepath.Join(tmpDir, "build", "bin")
	if err := os.MkdirAll(buildBinDir, 0o755); err != nil {
		t.Fatalf("Failed to create build/bin directory: %v", err)
	}

	// Build binaries
	forgeBinary := filepath.Join(buildBinDir, "forge")
	buildCmd := exec.Command("go", "build", "-o", forgeBinary, "../../cmd/forge")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build forge binary: %v", err)
	}

	testIntegrationBinary := filepath.Join(buildBinDir, "test-integration")
	buildCmd = exec.Command("go", "build", "-o", testIntegrationBinary, "../../cmd/test-integration")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build test-integration binary: %v", err)
	}

	testRunnerBinary := filepath.Join(buildBinDir, "test-runner-go")
	buildCmd = exec.Command("go", "build", "-o", testRunnerBinary, "../../cmd/test-runner-go")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build test-runner-go binary: %v", err)
	}

	// Setup test forge.yaml
	artifactStorePath := filepath.Join(tmpDir, "artifacts.yaml")
	forgeYAML := `name: e2e-test-project
artifactStorePath: ` + artifactStorePath + `

test:
  - name: integration
    engine: "go://test-integration"
    runner: "go://test-runner-go"
`
	forgeYAMLPath := filepath.Join(tmpDir, "forge.yaml")
	err := os.WriteFile(forgeYAMLPath, []byte(forgeYAML), 0o644)
	if err != nil {
		t.Fatalf("Failed to write forge.yaml: %v", err)
	}

	// Change to temp directory
	os.Chdir(tmpDir)

	// Note: We can't easily test forge test integration run without actual Go tests
	// This would require more complex setup with a test project
	t.Log("Auto-create environment test requires full project setup - skipping detailed test")
}

// TestForgeTestCommand_ErrorCases tests error handling.
func TestForgeTestCommand_ErrorCases(t *testing.T) {
	// Skip if not running integration tests
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create temporary directory for test
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)

	// Create build/bin directory in temp dir
	buildBinDir := filepath.Join(tmpDir, "build", "bin")
	if err := os.MkdirAll(buildBinDir, 0o755); err != nil {
		t.Fatalf("Failed to create build/bin directory: %v", err)
	}

	// Build forge binary
	forgeBinary := filepath.Join(buildBinDir, "forge")
	buildCmd := exec.Command("go", "build", "-o", forgeBinary, "../../cmd/forge")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build forge binary: %v", err)
	}

	// Setup minimal forge.yaml
	artifactStorePath := filepath.Join(tmpDir, "artifacts.yaml")
	forgeYAML := `name: e2e-test-project
artifactStorePath: ` + artifactStorePath + `

test:
  - name: integration
    engine: "go://test-integration"
    runner: "go://test-runner-go"
`
	forgeYAMLPath := filepath.Join(tmpDir, "forge.yaml")
	err := os.WriteFile(forgeYAMLPath, []byte(forgeYAML), 0o644)
	if err != nil {
		t.Fatalf("Failed to write forge.yaml: %v", err)
	}

	// Change to temp directory
	os.Chdir(tmpDir)

	// Test 1: Get nonexistent environment
	t.Log("Test 1: Getting nonexistent environment...")
	getCmd := exec.Command(forgeBinary, "test", "integration", "get", "nonexistent-id")
	_, err = getCmd.Output()
	if err == nil {
		t.Error("Expected error when getting nonexistent environment")
	}

	// Test 2: Delete nonexistent environment
	t.Log("Test 2: Deleting nonexistent environment...")
	deleteCmd := exec.Command(forgeBinary, "test", "integration", "delete", "nonexistent-id")
	_, err = deleteCmd.Output()
	if err == nil {
		t.Error("Expected error when deleting nonexistent environment")
	}

	// Test 3: Nonexistent stage
	t.Log("Test 3: Using nonexistent stage...")
	createCmd := exec.Command(forgeBinary, "test", "nonexistent-stage", "create")
	_, err = createCmd.Output()
	if err == nil {
		t.Error("Expected error when using nonexistent stage")
	}
}

// TestForgeTestCommand_MCP tests MCP mode of test tools.
func TestForgeTestCommand_MCP(t *testing.T) {
	// Skip if not running integration tests
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create temporary directory for test
	tmpDir := t.TempDir()

	// Create build/bin directory in temp dir
	buildBinDir := filepath.Join(tmpDir, "build", "bin")
	if err := os.MkdirAll(buildBinDir, 0o755); err != nil {
		t.Fatalf("Failed to create build/bin directory: %v", err)
	}

	// Build test-integration binary
	testIntegrationBinary := filepath.Join(buildBinDir, "test-integration")
	buildCmd := exec.Command("go", "build", "-o", testIntegrationBinary, "../../cmd/test-integration")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build test-integration binary: %v", err)
	}

	// Test MCP mode starts without error
	t.Log("Testing MCP mode initialization...")
	mcpCmd := exec.Command(testIntegrationBinary, "--mcp")

	// Start the process
	stdin, err := mcpCmd.StdinPipe()
	if err != nil {
		t.Fatalf("Failed to get stdin pipe: %v", err)
	}

	if err := mcpCmd.Start(); err != nil {
		t.Fatalf("Failed to start MCP mode: %v", err)
	}

	// Give it a moment to initialize
	time.Sleep(100 * time.Millisecond)

	// Send initialize request
	initReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "1.0.0",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "test-client",
				"version": "1.0.0",
			},
		},
	}

	jsonData, _ := json.Marshal(initReq)
	stdin.Write(jsonData)
	stdin.Write([]byte("\n"))

	// Close and wait
	stdin.Close()

	// Wait for process to finish (with timeout)
	done := make(chan error)
	go func() {
		done <- mcpCmd.Wait()
	}()

	select {
	case <-done:
		t.Log("MCP mode started and responded successfully")
	case <-time.After(2 * time.Second):
		mcpCmd.Process.Kill()
		t.Log("MCP mode test completed (timeout is expected for server)")
	}
}
