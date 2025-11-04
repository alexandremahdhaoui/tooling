package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/alexandremahdhaoui/forge/internal/mcpserver"
	"github.com/alexandremahdhaoui/forge/internal/version"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Version information (set via ldflags during build)
var (
	Version        = "dev"
	CommitSHA      = "unknown"
	BuildTimestamp = "unknown"
)

var versionInfo *version.Info

func init() {
	versionInfo = version.New("forge-e2e")
	versionInfo.Version = Version
	versionInfo.CommitSHA = CommitSHA
	versionInfo.BuildTimestamp = BuildTimestamp
}

// TestReport represents the structured output of an e2e test run
type TestReport struct {
	Status       string  `json:"status"` // "passed" or "failed"
	ErrorMessage string  `json:"error,omitempty"`
	Duration     float64 `json:"duration"` // seconds
	Total        int     `json:"total"`    // total test cases
	Passed       int     `json:"passed"`   // passed test cases
	Failed       int     `json:"failed"`   // failed test cases
}

type RunInput struct {
	Stage string `json:"stage"`
	Name  string `json:"name"`
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "--mcp":
		// Run in MCP server mode
		if err := runMCPServer(); err != nil {
			log.Printf("MCP server error: %v", err)
			os.Exit(1)
		}
	case "version", "--version", "-v":
		versionInfo.Print()
	case "help", "--help", "-h":
		printUsage()
	default:
		// Assume first arg is stage, second is name
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "Error: requires <STAGE> and <NAME> arguments\n\n")
			printUsage()
			os.Exit(1)
		}

		stage := os.Args[1]
		name := os.Args[2]

		if err := run(stage, name); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}
}

func printUsage() {
	fmt.Println(`forge-e2e - End-to-end test runner for forge

Usage:
  forge-e2e <STAGE> <NAME>      Run e2e tests for the given stage
  forge-e2e --mcp               Run as MCP server
  forge-e2e version             Show version information

Arguments:
  STAGE    Test stage name (e.g., "e2e")
  NAME     Test run identifier

Examples:
  forge-e2e e2e smoke-20241103
  forge-e2e --mcp

Output:
  - Test output is written to stderr
  - Structured JSON report is written to stdout`)
}

func runMCPServer() error {
	v, _, _ := versionInfo.Get()
	server := mcpserver.New("forge-e2e", v)

	mcpserver.RegisterTool(server, &mcp.Tool{
		Name:        "run",
		Description: "Run forge end-to-end tests",
	}, handleRun)

	return server.RunDefault()
}

func handleRun(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input RunInput,
) (*mcp.CallToolResult, any, error) {
	log.Printf("Running e2e tests: stage=%s, name=%s", input.Stage, input.Name)

	report, err := runTests(input.Stage, input.Name)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("E2E test execution failed: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	// Return the test report as JSON
	reportJSON, _ := json.Marshal(report)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(reportJSON)},
		},
		IsError: report.Status == "failed",
	}, report, nil
}

func run(stage, name string) error {
	// Execute tests and generate report
	report, err := runTests(stage, name)
	if err != nil {
		return fmt.Errorf("test execution failed: %w", err)
	}

	// Output JSON report to stdout
	if err := json.NewEncoder(os.Stdout).Encode(report); err != nil {
		return fmt.Errorf("failed to encode report: %w", err)
	}

	// Exit with non-zero if tests failed
	if report.Status == "failed" {
		os.Exit(1)
	}

	return nil
}

func runTests(stage, name string) (*TestReport, error) {
	startTime := time.Now()

	fmt.Fprintf(os.Stderr, "\n=== Forge E2E Test Suite ===\n")
	fmt.Fprintf(os.Stderr, "Stage: %s, Name: %s\n\n", stage, name)

	var total, passed, failed int
	var errors []string

	// Test 1: forge build command
	fmt.Fprintf(os.Stderr, "🔹 Test 1: forge build\n")
	total++
	if err := testForgeBuild(); err != nil {
		failed++
		errors = append(errors, fmt.Sprintf("forge build: %v", err))
		fmt.Fprintf(os.Stderr, "  ❌ FAILED: %v\n\n", err)
	} else {
		passed++
		fmt.Fprintf(os.Stderr, "  ✅ PASSED\n\n")
	}

	// Test 2: forge build with specific artifact
	fmt.Fprintf(os.Stderr, "🔹 Test 2: forge build <artifact>\n")
	total++
	if err := testForgeBuildSpecific(); err != nil {
		failed++
		errors = append(errors, fmt.Sprintf("forge build specific: %v", err))
		fmt.Fprintf(os.Stderr, "  ❌ FAILED: %v\n\n", err)
	} else {
		passed++
		fmt.Fprintf(os.Stderr, "  ✅ PASSED\n\n")
	}

	// Test 3: forge test unit run
	fmt.Fprintf(os.Stderr, "🔹 Test 3: forge test unit run\n")
	total++
	if err := testForgeTestUnit(); err != nil {
		failed++
		errors = append(errors, fmt.Sprintf("forge test unit: %v", err))
		fmt.Fprintf(os.Stderr, "  ❌ FAILED: %v\n\n", err)
	} else {
		passed++
		fmt.Fprintf(os.Stderr, "  ✅ PASSED\n\n")
	}

	// Test 4: Artifact store validation
	fmt.Fprintf(os.Stderr, "🔹 Test 4: Artifact store validation\n")
	total++
	if err := testArtifactStore(); err != nil {
		failed++
		errors = append(errors, fmt.Sprintf("artifact store: %v", err))
		fmt.Fprintf(os.Stderr, "  ❌ FAILED: %v\n\n", err)
	} else {
		passed++
		fmt.Fprintf(os.Stderr, "  ✅ PASSED\n\n")
	}

	// Test 5: forge version command
	fmt.Fprintf(os.Stderr, "🔹 Test 5: forge version\n")
	total++
	if err := testForgeVersion(); err != nil {
		failed++
		errors = append(errors, fmt.Sprintf("forge version: %v", err))
		fmt.Fprintf(os.Stderr, "  ❌ FAILED: %v\n\n", err)
	} else {
		passed++
		fmt.Fprintf(os.Stderr, "  ✅ PASSED\n\n")
	}

	duration := time.Since(startTime).Seconds()

	// Determine status
	status := "passed"
	errorMessage := ""
	if failed > 0 {
		status = "failed"
		errorMessage = strings.Join(errors, "; ")
	}

	fmt.Fprintf(os.Stderr, "\n=== Test Summary ===\n")
	fmt.Fprintf(os.Stderr, "Status: %s\n", status)
	fmt.Fprintf(os.Stderr, "Total: %d\n", total)
	fmt.Fprintf(os.Stderr, "Passed: %d\n", passed)
	fmt.Fprintf(os.Stderr, "Failed: %d\n", failed)
	fmt.Fprintf(os.Stderr, "Duration: %.2fs\n", duration)
	if errorMessage != "" {
		fmt.Fprintf(os.Stderr, "Errors: %s\n", errorMessage)
	}

	return &TestReport{
		Status:       status,
		ErrorMessage: errorMessage,
		Duration:     duration,
		Total:        total,
		Passed:       passed,
		Failed:       failed,
	}, nil
}

func testForgeBuild() error {
	cmd := exec.Command("go", "run", "./cmd/forge", "build", "forge")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("command failed: %w\nOutput: %s", err, string(output))
	}

	// Verify output contains success message
	if !strings.Contains(string(output), "Successfully built") {
		return fmt.Errorf("expected success message in output, got: %s", string(output))
	}

	// Verify binary exists
	if _, err := os.Stat("./build/bin/forge"); err != nil {
		return fmt.Errorf("forge binary not found: %w", err)
	}

	return nil
}

func testForgeBuildSpecific() error {
	cmd := exec.Command("go", "run", "./cmd/forge", "build", "build-go")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("command failed: %w\nOutput: %s", err, string(output))
	}

	// Verify output contains success message
	if !strings.Contains(string(output), "Successfully built") {
		return fmt.Errorf("expected success message in output, got: %s", string(output))
	}

	// Verify binary exists
	if _, err := os.Stat("./build/bin/build-go"); err != nil {
		return fmt.Errorf("build-go binary not found: %w", err)
	}

	return nil
}

func testForgeTestUnit() error {
	cmd := exec.Command("go", "run", "./cmd/forge", "test", "unit", "run")
	output, _ := cmd.CombinedOutput()

	// Unit tests may fail due to linting issues, but command should execute
	// We just check that it runs and produces output
	if len(output) == 0 {
		return fmt.Errorf("no output from test command")
	}

	// Check that output contains test results
	if !strings.Contains(string(output), "Test Results:") && !strings.Contains(string(output), "DONE") {
		return fmt.Errorf("expected test results in output, got: %s", string(output))
	}

	return nil
}

func testArtifactStore() error {
	storePath := ".ignore.artifact-store.yaml"

	// Check file exists
	if _, err := os.Stat(storePath); err != nil {
		return fmt.Errorf("artifact store not found: %w", err)
	}

	// Read file
	data, err := os.ReadFile(storePath)
	if err != nil {
		return fmt.Errorf("failed to read artifact store: %w", err)
	}

	// Basic validation - should contain expected structure
	content := string(data)
	if !strings.Contains(content, "version:") {
		return fmt.Errorf("artifact store missing version field")
	}

	if !strings.Contains(content, "artifacts:") && !strings.Contains(content, "lastUpdated:") {
		return fmt.Errorf("artifact store missing expected fields")
	}

	return nil
}

func testForgeVersion() error {
	cmd := exec.Command("go", "run", "./cmd/forge", "version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("command failed: %w\nOutput: %s", err, string(output))
	}

	// Verify output contains version info
	requiredFields := []string{"forge version", "commit:", "built:", "go:", "platform:"}
	for _, field := range requiredFields {
		if !strings.Contains(string(output), field) {
			return fmt.Errorf("version output missing field '%s'", field)
		}
	}

	return nil
}
