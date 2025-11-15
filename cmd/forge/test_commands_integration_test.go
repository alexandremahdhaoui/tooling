//go:build integration

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alexandremahdhaoui/forge/pkg/forge"
)

// TestTestReportWorkflow tests the complete test report workflow:
// run -> list -> get -> delete
func TestTestReportWorkflow(t *testing.T) {
	// Setup: Create temporary directory with forge.yaml
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(oldWd)

	forgeYaml := filepath.Join(tmpDir, "forge.yaml")
	content := `name: test-project
artifactStorePath: .ignore.artifact-store.yaml

test:
  - name: unit
    runner: go://go-test
    testenv: go://test-report
`
	if err := os.WriteFile(forgeYaml, []byte(content), 0o644); err != nil {
		t.Fatalf("Failed to create test forge.yaml: %v", err)
	}

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Step 1: Run tests to create a test report
	t.Log("Step 1: Running tests...")
	config, err := loadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	testSpec := findTestSpec(config.Test, "unit")
	if testSpec == nil {
		t.Fatal("Test spec 'unit' not found")
	}

	// We can't actually run the tests in this test environment,
	// but we can simulate by creating a test report directly
	artifactStorePath, err := forge.GetArtifactStorePath(config.ArtifactStorePath)
	if err != nil {
		t.Fatalf("Failed to get artifact store path: %v", err)
	}

	store, err := forge.ReadOrCreateArtifactStore(artifactStorePath)
	if err != nil {
		t.Fatalf("Failed to read/create artifact store: %v", err)
	}

	// Create a test report
	testReport := &forge.TestReport{
		ID:        "test-report-integration-" + time.Now().Format("20060102-150405"),
		Stage:     "unit",
		Status:    "passed",
		StartTime: time.Now(),
		Duration:  1.5,
		TestStats: forge.TestStats{
			Total:   10,
			Passed:  10,
			Failed:  0,
			Skipped: 0,
		},
		Coverage: forge.Coverage{
			Percentage: 85.5,
			FilePath:   "/tmp/coverage.out",
		},
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	forge.AddOrUpdateTestReport(&store, testReport)
	if err := forge.WriteArtifactStore(artifactStorePath, store); err != nil {
		t.Fatalf("Failed to write artifact store: %v", err)
	}

	t.Logf("Created test report: %s", testReport.ID)

	// Step 2: List test reports
	t.Log("Step 2: Listing test reports...")
	err = testListReports(testSpec, []string{})
	if err != nil {
		t.Errorf("Failed to list test reports: %v", err)
	}

	// Verify report exists in list
	store, _ = forge.ReadArtifactStore(artifactStorePath)
	reports := forge.ListTestReports(&store, "unit")
	if len(reports) == 0 {
		t.Error("Expected at least one test report")
	}

	foundReport := false
	for _, r := range reports {
		if r.ID == testReport.ID {
			foundReport = true
			break
		}
	}
	if !foundReport {
		t.Errorf("Test report %s not found in list", testReport.ID)
	}

	// Step 3: Get test report details
	t.Log("Step 3: Getting test report details...")
	err = testGetReport(testSpec, []string{testReport.ID})
	if err != nil {
		t.Errorf("Failed to get test report: %v", err)
	}

	// Step 4: Delete test report
	t.Log("Step 4: Deleting test report...")
	err = testDeleteReport(testSpec, []string{testReport.ID})
	if err != nil {
		t.Errorf("Failed to delete test report: %v", err)
	}

	// Step 5: Verify report is gone
	t.Log("Step 5: Verifying report deletion...")
	store, _ = forge.ReadArtifactStore(artifactStorePath)
	reports = forge.ListTestReports(&store, "unit")
	for _, r := range reports {
		if r.ID == testReport.ID {
			t.Errorf("Test report %s still exists after deletion", testReport.ID)
		}
	}

	t.Log("✓ Test report workflow completed successfully")
}

// TestListEnvShowsDefaultForTestReport verifies list-env shows synthetic "default"
func TestListEnvShowsDefaultForTestReport(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(oldWd)

	forgeYaml := filepath.Join(tmpDir, "forge.yaml")
	content := `name: test-project
artifactStorePath: .ignore.artifact-store.yaml

test:
  - name: unit
    runner: go://go-test
    testenv: go://test-report
`
	if err := os.WriteFile(forgeYaml, []byte(content), 0o644); err != nil {
		t.Fatalf("Failed to create test forge.yaml: %v", err)
	}

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	config, err := loadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	testSpec := findTestSpec(config.Test, "unit")
	if testSpec == nil {
		t.Fatal("Test spec 'unit' not found")
	}

	// Test list-env shows synthetic default
	t.Log("Testing list-env for test-report stage...")
	err = testListEnvironments(testSpec, []string{})
	if err != nil {
		t.Errorf("Failed to list environments: %v", err)
	}

	// Verify it's recognized as test-report stage
	if !IsTestReportStage(testSpec) {
		t.Error("Expected unit stage to be test-report stage")
	}

	t.Log("✓ list-env correctly shows default for test-report stage")
}

// TestGetEnvShowsDetailsForTestReport verifies get-env shows synthetic details
func TestGetEnvShowsDetailsForTestReport(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(oldWd)

	forgeYaml := filepath.Join(tmpDir, "forge.yaml")
	content := `name: test-project
artifactStorePath: .ignore.artifact-store.yaml

test:
  - name: unit
    runner: go://go-test
    testenv: go://test-report
`
	if err := os.WriteFile(forgeYaml, []byte(content), 0o644); err != nil {
		t.Fatalf("Failed to create test forge.yaml: %v", err)
	}

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	config, err := loadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	testSpec := findTestSpec(config.Test, "unit")
	if testSpec == nil {
		t.Fatal("Test spec 'unit' not found")
	}

	// Test get-env with "default" ID
	t.Log("Testing get-env unit default...")
	err = testGetEnvironment(testSpec, []string{"default"})
	if err != nil {
		t.Errorf("Failed to get environment: %v", err)
	}

	t.Log("✓ get-env correctly shows details for default test-report environment")
}

// TestCreateEnvFailsForTestReport verifies create-env rejects test-report stages
func TestCreateEnvFailsForTestReport(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(oldWd)

	forgeYaml := filepath.Join(tmpDir, "forge.yaml")
	content := `name: test-project
artifactStorePath: .ignore.artifact-store.yaml

test:
  - name: unit
    runner: go://go-test
    testenv: go://test-report
`
	if err := os.WriteFile(forgeYaml, []byte(content), 0o644); err != nil {
		t.Fatalf("Failed to create test forge.yaml: %v", err)
	}

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	config, err := loadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	testSpec := findTestSpec(config.Test, "unit")
	if testSpec == nil {
		t.Fatal("Test spec 'unit' not found")
	}

	// Test create-env fails with helpful error
	t.Log("Testing create-env unit (should fail)...")
	err = testCreateEnv(testSpec)
	if err == nil {
		t.Error("Expected create-env to fail for test-report stage")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "test-report") {
		t.Errorf("Error should mention test-report: %v", errMsg)
	}
	if !strings.Contains(errMsg, "forge test run") {
		t.Errorf("Error should suggest 'forge test run': %v", errMsg)
	}

	t.Log("✓ create-env correctly rejects test-report stage with helpful message")
}

// TestDeleteEnvFailsForTestReport verifies delete-env rejects test-report stages
func TestDeleteEnvFailsForTestReport(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(oldWd)

	forgeYaml := filepath.Join(tmpDir, "forge.yaml")
	content := `name: test-project
artifactStorePath: .ignore.artifact-store.yaml

test:
  - name: unit
    runner: go://go-test
    testenv: go://test-report
`
	if err := os.WriteFile(forgeYaml, []byte(content), 0o644); err != nil {
		t.Fatalf("Failed to create test forge.yaml: %v", err)
	}

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	config, err := loadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	testSpec := findTestSpec(config.Test, "unit")
	if testSpec == nil {
		t.Fatal("Test spec 'unit' not found")
	}

	// Test delete-env fails with helpful error
	t.Log("Testing delete-env unit default (should fail)...")
	err = testDeleteEnv(testSpec, []string{"default"})
	if err == nil {
		t.Error("Expected delete-env to fail for test-report stage")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "test-report") {
		t.Errorf("Error should mention test-report: %v", errMsg)
	}
	if !strings.Contains(errMsg, "forge test delete") {
		t.Errorf("Error should suggest 'forge test delete': %v", errMsg)
	}

	t.Log("✓ delete-env correctly rejects test-report stage with helpful message")
}

// TestOutputFormats tests all output formats work correctly
func TestOutputFormats(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(oldWd)

	forgeYaml := filepath.Join(tmpDir, "forge.yaml")
	content := `name: test-project
artifactStorePath: .ignore.artifact-store.yaml

test:
  - name: unit
    runner: go://go-test
    testenv: go://test-report
`
	if err := os.WriteFile(forgeYaml, []byte(content), 0o644); err != nil {
		t.Fatalf("Failed to create test forge.yaml: %v", err)
	}

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	config, err := loadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	testSpec := findTestSpec(config.Test, "unit")
	if testSpec == nil {
		t.Fatal("Test spec 'unit' not found")
	}

	// Create a test report
	artifactStorePath, _ := forge.GetArtifactStorePath(config.ArtifactStorePath)
	store, _ := forge.ReadOrCreateArtifactStore(artifactStorePath)

	testReport := &forge.TestReport{
		ID:        "test-format-check",
		Stage:     "unit",
		Status:    "passed",
		StartTime: time.Now(),
		Duration:  1.0,
		TestStats: forge.TestStats{Total: 5, Passed: 5},
		Coverage:  forge.Coverage{Percentage: 90.0},
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	forge.AddOrUpdateTestReport(&store, testReport)
	forge.WriteArtifactStore(artifactStorePath, store)

	// Test JSON format
	t.Log("Testing JSON output format...")
	err = testListReports(testSpec, []string{"-o", "json"})
	if err != nil {
		t.Errorf("Failed to list reports in JSON: %v", err)
	}

	// Test YAML format
	t.Log("Testing YAML output format...")
	err = testListReports(testSpec, []string{"-o", "yaml"})
	if err != nil {
		t.Errorf("Failed to list reports in YAML: %v", err)
	}

	// Test table format (default)
	t.Log("Testing table output format...")
	err = testListReports(testSpec, []string{})
	if err != nil {
		t.Errorf("Failed to list reports in table: %v", err)
	}

	t.Log("✓ All output formats work correctly")
}

// TestListReportsWithNoReports tests list behavior when no reports exist
func TestListReportsWithNoReports(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(oldWd)

	forgeYaml := filepath.Join(tmpDir, "forge.yaml")
	content := `name: test-project
artifactStorePath: .ignore.artifact-store.yaml

test:
  - name: unit
    runner: go://go-test
    testenv: go://test-report
`
	if err := os.WriteFile(forgeYaml, []byte(content), 0o644); err != nil {
		t.Fatalf("Failed to create test forge.yaml: %v", err)
	}

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	config, err := loadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	testSpec := findTestSpec(config.Test, "unit")
	if testSpec == nil {
		t.Fatal("Test spec 'unit' not found")
	}

	// Create empty artifact store so list doesn't fail
	artifactStorePath, _ := forge.GetArtifactStorePath(config.ArtifactStorePath)
	store, _ := forge.ReadOrCreateArtifactStore(artifactStorePath)
	forge.WriteArtifactStore(artifactStorePath, store)

	// List should work even with no reports
	t.Log("Testing list with no reports...")
	err = testListReports(testSpec, []string{})
	if err != nil {
		t.Errorf("Failed to list reports (empty): %v", err)
	}

	t.Log("✓ list handles empty reports gracefully")
}

// TestGetNonExistentReport tests error handling for missing reports
func TestGetNonExistentReport(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(oldWd)

	forgeYaml := filepath.Join(tmpDir, "forge.yaml")
	content := `name: test-project
artifactStorePath: .ignore.artifact-store.yaml

test:
  - name: unit
    runner: go://go-test
    testenv: go://test-report
`
	if err := os.WriteFile(forgeYaml, []byte(content), 0o644); err != nil {
		t.Fatalf("Failed to create test forge.yaml: %v", err)
	}

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	config, err := loadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	testSpec := findTestSpec(config.Test, "unit")
	if testSpec == nil {
		t.Fatal("Test spec 'unit' not found")
	}

	// Create empty artifact store so get can check for non-existent report
	artifactStorePath, _ := forge.GetArtifactStorePath(config.ArtifactStorePath)
	store, _ := forge.ReadOrCreateArtifactStore(artifactStorePath)
	forge.WriteArtifactStore(artifactStorePath, store)

	// Get non-existent report should fail
	t.Log("Testing get with non-existent report...")
	err = testGetReport(testSpec, []string{"non-existent-id"})
	if err == nil {
		t.Error("Expected error when getting non-existent report")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Error should mention 'not found': %v", err)
	}

	t.Log("✓ get handles missing reports correctly")
}

// TestMCPHandlerListReturnsReports tests that MCP handler returns test reports
func TestMCPHandlerListReturnsReports(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(oldWd)

	forgeYaml := filepath.Join(tmpDir, "forge.yaml")
	content := `name: test-project
artifactStorePath: .ignore.artifact-store.yaml

test:
  - name: unit
    runner: go://go-test
    testenv: go://test-report
`
	if err := os.WriteFile(forgeYaml, []byte(content), 0o644); err != nil {
		t.Fatalf("Failed to create test forge.yaml: %v", err)
	}

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	config, err := loadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Create a test report
	artifactStorePath, _ := forge.GetArtifactStorePath(config.ArtifactStorePath)
	store, _ := forge.ReadOrCreateArtifactStore(artifactStorePath)

	testReport := &forge.TestReport{
		ID:        "test-mcp-handler",
		Stage:     "unit",
		Status:    "passed",
		StartTime: time.Now(),
		Duration:  1.0,
		TestStats: forge.TestStats{Total: 5, Passed: 5},
		Coverage:  forge.Coverage{Percentage: 90.0},
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	forge.AddOrUpdateTestReport(&store, testReport)
	forge.WriteArtifactStore(artifactStorePath, store)

	// Test MCP handler behavior
	t.Log("Testing MCP handler returns test reports...")

	input := TestListInput{Stage: "unit"}
	result, artifact, err := handleTestListTool(nil, nil, input)
	if err != nil {
		t.Errorf("MCP handler failed: %v", err)
	}

	if result.IsError {
		t.Error("MCP handler returned error result")
	}

	// Verify artifact contains TestReport objects, not TestEnvironment
	if artifact != nil {
		artifactJSON, _ := json.Marshal(artifact)
		if !strings.Contains(string(artifactJSON), "testStats") {
			t.Error("MCP handler should return test reports (with testStats field)")
		}
	}

	t.Log("✓ MCP handler correctly returns test reports")
}
