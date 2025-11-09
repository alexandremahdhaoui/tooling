//go:build unit

package orchestrate

import (
	"fmt"
	"testing"
	"time"

	"github.com/alexandremahdhaoui/forge/pkg/forge"
)

// Mock MCP caller that returns predefined responses
type mockMCPCaller struct {
	calls   []mockCall
	results []interface{}
	errors  []error
	index   int
}

type mockCall struct {
	command  string
	args     []string
	toolName string
	params   interface{}
}

func newMockMCPCaller(results []interface{}, errors []error) *mockMCPCaller {
	return &mockMCPCaller{
		calls:   make([]mockCall, 0),
		results: results,
		errors:  errors,
		index:   0,
	}
}

func (m *mockMCPCaller) call(command string, args []string, toolName string, params interface{}) (interface{}, error) {
	m.calls = append(m.calls, mockCall{command, args, toolName, params})
	if m.index >= len(m.results) {
		return nil, fmt.Errorf("mock: no more results")
	}
	result := m.results[m.index]
	err := m.errors[m.index]
	m.index++
	return result, err
}

// Mock engine resolver that returns fake command and args
type mockEngineResolver struct {
	engines map[string]struct {
		command string
		args    []string
	}
}

func newMockEngineResolver(engines map[string]struct {
	command string
	args    []string
},
) *mockEngineResolver {
	return &mockEngineResolver{engines: engines}
}

func (m *mockEngineResolver) resolve(engineURI string) (string, []string, error) {
	if engine, ok := m.engines[engineURI]; ok {
		return engine.command, engine.args, nil
	}
	return "", nil, fmt.Errorf("mock: engine not found: %s", engineURI)
}

// Helper to create artifact response
func createArtifactResponse(name string) map[string]any {
	return map[string]any{
		"name":      name,
		"type":      "binary",
		"location":  "/build/bin/" + name,
		"timestamp": time.Now().Format(time.RFC3339),
		"version":   "test-version",
	}
}

// Helper to create test report response
func createTestReportResponse(total, passed, failed int, status string) map[string]any {
	return map[string]any{
		"id":        "test-report-id",
		"stage":     "unit",
		"status":    status,
		"startTime": time.Now().Format(time.RFC3339),
		"duration":  1.23,
		"testStats": map[string]any{
			"total":   total,
			"passed":  passed,
			"failed":  failed,
			"skipped": 0,
		},
		"coverage": map[string]any{
			"percentage": 85.5,
		},
		"artifactFiles": []string{},
		"outputPath":    "/tmp/test-output.txt",
	}
}

// TestBuilderOrchestrator_SingleEngine tests orchestration with a single builder
func TestBuilderOrchestrator_SingleEngine(t *testing.T) {
	// Setup mocks
	mockMCP := newMockMCPCaller(
		[]interface{}{createArtifactResponse("test-app")},
		[]error{nil},
	)
	mockResolver := newMockEngineResolver(map[string]struct {
		command string
		args    []string
	}{
		"go://build-go": {command: "go", args: []string{"run", "github.com/alexandremahdhaoui/forge/cmd/build-go"}},
	})

	// Create orchestrator
	orchestrator := NewBuilderOrchestrator(mockMCP.call, mockResolver.resolve)

	// Prepare specs
	builderSpecs := []forge.BuilderEngineSpec{
		{
			Engine: "go://build-go",
			Spec:   forge.EngineSpec{},
		},
	}
	buildSpecs := []map[string]any{
		{"name": "test-app", "src": "./cmd/test-app"},
	}
	dirs := map[string]any{
		"tmpDir":   "/tmp/forge",
		"buildDir": "/build",
		"rootDir":  "/project",
	}

	// Execute
	artifacts, err := orchestrator.Orchestrate(builderSpecs, buildSpecs, dirs)
	// Verify
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if len(artifacts) != 1 {
		t.Fatalf("Expected 1 artifact, got %d", len(artifacts))
	}
	if artifacts[0].Name != "test-app" {
		t.Errorf("Expected artifact name 'test-app', got '%s'", artifacts[0].Name)
	}
	if len(mockMCP.calls) != 1 {
		t.Errorf("Expected 1 MCP call, got %d", len(mockMCP.calls))
	}
	if mockMCP.calls[0].toolName != "build" {
		t.Errorf("Expected tool name 'build', got '%s'", mockMCP.calls[0].toolName)
	}
}

// TestBuilderOrchestrator_MultipleEngines tests sequential execution of multiple builders
func TestBuilderOrchestrator_MultipleEngines(t *testing.T) {
	// Setup mocks - 3 builders, each returns one artifact
	mockMCP := newMockMCPCaller(
		[]interface{}{
			createArtifactResponse("artifact-1"),
			createArtifactResponse("artifact-2"),
			createArtifactResponse("artifact-3"),
		},
		[]error{nil, nil, nil},
	)
	mockResolver := newMockEngineResolver(map[string]struct {
		command string
		args    []string
	}{
		"go://build-go":        {command: "go", args: []string{"run", "github.com/alexandremahdhaoui/forge/cmd/build-go"}},
		"go://generic-builder": {command: "go", args: []string{"run", "github.com/alexandremahdhaoui/forge/cmd/generic-builder"}},
	})

	// Create orchestrator
	orchestrator := NewBuilderOrchestrator(mockMCP.call, mockResolver.resolve)

	// Prepare specs - 3 builders
	builderSpecs := []forge.BuilderEngineSpec{
		{Engine: "go://build-go", Spec: forge.EngineSpec{}},
		{Engine: "go://generic-builder", Spec: forge.EngineSpec{Command: "echo"}},
		{Engine: "go://build-go", Spec: forge.EngineSpec{}},
	}
	buildSpecs := []map[string]any{
		{"name": "test-app", "src": "./cmd/test-app"},
	}
	dirs := map[string]any{"tmpDir": "/tmp"}

	// Execute
	artifacts, err := orchestrator.Orchestrate(builderSpecs, buildSpecs, dirs)
	// Verify
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if len(artifacts) != 3 {
		t.Fatalf("Expected 3 artifacts, got %d", len(artifacts))
	}
	if len(mockMCP.calls) != 3 {
		t.Errorf("Expected 3 MCP calls (sequential), got %d", len(mockMCP.calls))
	}
	// Verify sequential execution order
	if mockMCP.calls[0].command != "go" {
		t.Errorf("Expected first call command 'go', got %s", mockMCP.calls[0].command)
	}
	if len(mockMCP.calls[0].args) < 2 || mockMCP.calls[0].args[1] != "github.com/alexandremahdhaoui/forge/cmd/build-go" {
		t.Errorf("Expected first call to build-go package, got %v", mockMCP.calls[0].args)
	}
	if mockMCP.calls[1].command != "go" {
		t.Errorf("Expected second call command 'go', got %s", mockMCP.calls[1].command)
	}
	if len(mockMCP.calls[1].args) < 2 || mockMCP.calls[1].args[1] != "github.com/alexandremahdhaoui/forge/cmd/generic-builder" {
		t.Errorf("Expected second call to generic-builder package, got %v", mockMCP.calls[1].args)
	}
}

// TestBuilderOrchestrator_EngineFailure tests fail-fast behavior
func TestBuilderOrchestrator_EngineFailure(t *testing.T) {
	// Setup mocks - second builder fails
	mockMCP := newMockMCPCaller(
		[]interface{}{
			createArtifactResponse("artifact-1"),
			nil, // Second call fails
		},
		[]error{
			nil,
			fmt.Errorf("build failed"),
		},
	)
	mockResolver := newMockEngineResolver(map[string]struct {
		command string
		args    []string
	}{
		"go://build-go":        {command: "go", args: []string{"run", "github.com/alexandremahdhaoui/forge/cmd/build-go"}},
		"go://generic-builder": {command: "go", args: []string{"run", "github.com/alexandremahdhaoui/forge/cmd/generic-builder"}},
	})

	// Create orchestrator
	orchestrator := NewBuilderOrchestrator(mockMCP.call, mockResolver.resolve)

	// Prepare specs - 3 builders (should stop at 2nd)
	builderSpecs := []forge.BuilderEngineSpec{
		{Engine: "go://build-go", Spec: forge.EngineSpec{}},
		{Engine: "go://generic-builder", Spec: forge.EngineSpec{}},
		{Engine: "go://build-go", Spec: forge.EngineSpec{}}, // Should not be called
	}
	buildSpecs := []map[string]any{
		{"name": "test-app", "src": "./cmd/test-app"},
	}
	dirs := map[string]any{"tmpDir": "/tmp"}

	// Execute
	_, err := orchestrator.Orchestrate(builderSpecs, buildSpecs, dirs)

	// Verify
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !contains(err.Error(), "build failed") {
		t.Errorf("Expected error to contain 'build failed', got: %v", err)
	}
	if len(mockMCP.calls) != 2 {
		t.Errorf("Expected 2 MCP calls (fail-fast), got %d", len(mockMCP.calls))
	}
}

// TestBuilderOrchestrator_ConfigInjection tests that builder config is injected
func TestBuilderOrchestrator_ConfigInjection(t *testing.T) {
	// Setup mocks
	mockMCP := newMockMCPCaller(
		[]interface{}{createArtifactResponse("test-app")},
		[]error{nil},
	)
	mockResolver := newMockEngineResolver(map[string]struct {
		command string
		args    []string
	}{
		"go://generic-builder": {command: "go", args: []string{"run", "github.com/alexandremahdhaoui/forge/cmd/generic-builder"}},
	})

	// Create orchestrator
	orchestrator := NewBuilderOrchestrator(mockMCP.call, mockResolver.resolve)

	// Prepare specs with config
	builderSpecs := []forge.BuilderEngineSpec{
		{
			Engine: "go://generic-builder",
			Spec: forge.EngineSpec{
				Command: "custom-command",
				Args:    []string{"arg1", "arg2"},
				Env:     map[string]string{"KEY": "value"},
			},
		},
	}
	buildSpecs := []map[string]any{
		{"name": "test-app", "src": "./cmd/test-app"},
	}
	dirs := map[string]any{"tmpDir": "/tmp"}

	// Execute
	_, err := orchestrator.Orchestrate(builderSpecs, buildSpecs, dirs)
	// Verify
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify config was injected into params
	if len(mockMCP.calls) != 1 {
		t.Fatalf("Expected 1 call, got %d", len(mockMCP.calls))
	}
	params, ok := mockMCP.calls[0].params.(map[string]any)
	if !ok {
		t.Fatal("Expected params to be map[string]any")
	}
	if params["command"] != "custom-command" {
		t.Errorf("Expected command to be injected, got: %v", params["command"])
	}
	if params["tmpDir"] != "/tmp" {
		t.Errorf("Expected tmpDir to be injected, got: %v", params["tmpDir"])
	}
}

// TestTestRunnerOrchestrator_SingleRunner tests orchestration with a single runner
func TestTestRunnerOrchestrator_SingleRunner(t *testing.T) {
	// Setup mocks
	mockMCP := newMockMCPCaller(
		[]interface{}{createTestReportResponse(10, 10, 0, "passed")},
		[]error{nil},
	)
	mockResolver := newMockEngineResolver(map[string]struct {
		command string
		args    []string
	}{
		"go://test-runner-go": {command: "go", args: []string{"run", "github.com/alexandremahdhaoui/forge/cmd/test-runner-go"}},
	})

	// Create orchestrator
	orchestrator := NewTestRunnerOrchestrator(mockMCP.call, mockResolver.resolve)

	// Prepare specs
	runnerSpecs := []forge.TestRunnerSpec{
		{
			Engine: "go://test-runner-go",
			Spec:   forge.EngineSpec{},
		},
	}
	params := map[string]any{
		"stage": "unit",
		"name":  "unit-tests",
	}

	// Execute
	report, err := orchestrator.Orchestrate(runnerSpecs, params)
	// Verify
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if report == nil {
		t.Fatal("Expected report, got nil")
	}
	if report.Status != "passed" {
		t.Errorf("Expected status 'passed', got '%s'", report.Status)
	}
	if report.TestStats.Total != 10 {
		t.Errorf("Expected 10 total tests, got %d", report.TestStats.Total)
	}
	if len(mockMCP.calls) != 1 {
		t.Errorf("Expected 1 MCP call, got %d", len(mockMCP.calls))
	}
}

// TestTestRunnerOrchestrator_MultipleRunners tests report merging
func TestTestRunnerOrchestrator_MultipleRunners(t *testing.T) {
	// Setup mocks - 2 runners with different test results
	mockMCP := newMockMCPCaller(
		[]interface{}{
			createTestReportResponse(10, 8, 2, "failed"), // Runner 1: 8 passed, 2 failed
			createTestReportResponse(5, 5, 0, "passed"),  // Runner 2: 5 passed, 0 failed
		},
		[]error{nil, nil},
	)
	mockResolver := newMockEngineResolver(map[string]struct {
		command string
		args    []string
	}{
		"go://test-runner-go": {command: "go", args: []string{"run", "github.com/alexandremahdhaoui/forge/cmd/test-runner-go"}},
		"go://lint-go":        {command: "go", args: []string{"run", "github.com/alexandremahdhaoui/forge/cmd/lint-go"}},
	})

	// Create orchestrator
	orchestrator := NewTestRunnerOrchestrator(mockMCP.call, mockResolver.resolve)

	// Prepare specs
	runnerSpecs := []forge.TestRunnerSpec{
		{Engine: "go://test-runner-go", Spec: forge.EngineSpec{}},
		{Engine: "go://lint-go", Spec: forge.EngineSpec{}},
	}
	params := map[string]any{
		"stage": "unit",
		"name":  "comprehensive-tests",
	}

	// Execute
	report, err := orchestrator.Orchestrate(runnerSpecs, params)
	// Verify
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify merged stats
	if report.TestStats.Total != 15 {
		t.Errorf("Expected total 15 tests (10+5), got %d", report.TestStats.Total)
	}
	if report.TestStats.Passed != 13 {
		t.Errorf("Expected 13 passed (8+5), got %d", report.TestStats.Passed)
	}
	if report.TestStats.Failed != 2 {
		t.Errorf("Expected 2 failed (2+0), got %d", report.TestStats.Failed)
	}

	// Verify status (should be failed if any runner failed)
	if report.Status != "failed" {
		t.Errorf("Expected status 'failed' (any runner failed), got '%s'", report.Status)
	}

	// Verify sequential execution
	if len(mockMCP.calls) != 2 {
		t.Errorf("Expected 2 MCP calls (sequential), got %d", len(mockMCP.calls))
	}
}

// TestTestRunnerOrchestrator_RunnerFailure tests fail-fast behavior
func TestTestRunnerOrchestrator_RunnerFailure(t *testing.T) {
	// Setup mocks - first runner fails
	mockMCP := newMockMCPCaller(
		[]interface{}{nil},
		[]error{fmt.Errorf("runner failed")},
	)
	mockResolver := newMockEngineResolver(map[string]struct {
		command string
		args    []string
	}{
		"go://test-runner-go": {command: "go", args: []string{"run", "github.com/alexandremahdhaoui/forge/cmd/test-runner-go"}},
		"go://lint-go":        {command: "go", args: []string{"run", "github.com/alexandremahdhaoui/forge/cmd/lint-go"}},
	})

	// Create orchestrator
	orchestrator := NewTestRunnerOrchestrator(mockMCP.call, mockResolver.resolve)

	// Prepare specs
	runnerSpecs := []forge.TestRunnerSpec{
		{Engine: "go://test-runner-go", Spec: forge.EngineSpec{}},
		{Engine: "go://lint-go", Spec: forge.EngineSpec{}}, // Should not be called
	}
	params := map[string]any{"stage": "unit"}

	// Execute
	_, err := orchestrator.Orchestrate(runnerSpecs, params)

	// Verify
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !contains(err.Error(), "runner failed") {
		t.Errorf("Expected error to contain 'runner failed', got: %v", err)
	}
	if len(mockMCP.calls) != 1 {
		t.Errorf("Expected 1 MCP call (fail-fast), got %d", len(mockMCP.calls))
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
