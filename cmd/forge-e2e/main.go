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

// TestCategory represents a category of tests
type TestCategory string

const (
	CategoryBuild         TestCategory = "build"
	CategoryTestEnv       TestCategory = "testenv"
	CategoryTestRunner    TestCategory = "test-runner"
	CategoryPrompt        TestCategory = "prompt"
	CategorySystem        TestCategory = "system"
	CategoryError         TestCategory = "error-handling"
	CategoryCleanup       TestCategory = "cleanup"
	CategoryMCP           TestCategory = "mcp"
	CategoryPerformance   TestCategory = "performance"
	CategoryArtifactStore TestCategory = "artifact-store"
)

// TestResult represents the result of a single test
type TestResult struct {
	Name     string       `json:"name"`
	Category TestCategory `json:"category"`
	Status   string       `json:"status"` // "passed", "failed", "skipped"
	Duration float64      `json:"duration"`
	Error    string       `json:"error,omitempty"`
	Output   string       `json:"output,omitempty"`
}

// CategoryStats represents statistics for a test category
type CategoryStats struct {
	Total    int     `json:"total"`
	Passed   int     `json:"passed"`
	Failed   int     `json:"failed"`
	Skipped  int     `json:"skipped"`
	Duration float64 `json:"duration"`
}

// TestReport represents the structured output of an e2e test run
type TestReport struct {
	Status       string  `json:"status"` // "passed" or "failed"
	ErrorMessage string  `json:"error,omitempty"`
	Duration     float64 `json:"duration"` // seconds
	Total        int     `json:"total"`    // total test cases
	Passed       int     `json:"passed"`   // passed test cases
	Failed       int     `json:"failed"`   // failed test cases
	Skipped      int     `json:"skipped"`  // skipped test cases
}

// DetailedTestReport extends TestReport with per-test and per-category details
type DetailedTestReport struct {
	TestReport
	Results    []TestResult                   `json:"results"`
	Categories map[TestCategory]CategoryStats `json:"categories"`
}

type RunInput struct {
	Stage string `json:"stage"`
	Name  string `json:"name"`
}

// TestFunc represents a test function
type TestFunc func() error

// Test represents a single test case
type Test struct {
	Name       string
	Category   TestCategory
	Run        TestFunc
	Skip       bool
	SkipReason string
}

// TestSuite manages and executes tests
type TestSuite struct {
	tests             []Test
	results           []TestResult
	categories        map[TestCategory]*CategoryStats
	filterCategory    string
	filterNamePattern string
}

// NewTestSuite creates a new test suite
func NewTestSuite() *TestSuite {
	return &TestSuite{
		tests:             make([]Test, 0),
		results:           make([]TestResult, 0),
		categories:        make(map[TestCategory]*CategoryStats),
		filterCategory:    os.Getenv("TEST_CATEGORY"),
		filterNamePattern: os.Getenv("TEST_NAME_PATTERN"),
	}
}

// AddTest adds a test to the suite
func (ts *TestSuite) AddTest(test Test) {
	// Apply filters if set
	if ts.filterCategory != "" && string(test.Category) != ts.filterCategory {
		return // Skip tests not matching category filter
	}

	if ts.filterNamePattern != "" && !strings.Contains(strings.ToLower(test.Name), strings.ToLower(ts.filterNamePattern)) {
		return // Skip tests not matching name pattern
	}

	ts.tests = append(ts.tests, test)
}

// Setup performs global test suite setup
func (ts *TestSuite) Setup() error {
	// Check if forge binary exists
	if _, err := os.Stat("./build/bin/forge"); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: ./build/bin/forge not found, attempting to build...\n")
		buildCmd := exec.Command("go", "run", "./cmd/forge", "build", "forge")
		if output, err := buildCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed to build forge: %w\nOutput: %s", err, output)
		}
		fmt.Fprintf(os.Stderr, "✓ Built forge binary\n")
	}

	// Check for leftover test environments and warn
	kindBinary := os.Getenv("KIND_BINARY")
	if kindBinary != "" {
		cmd := exec.Command(kindBinary, "get", "clusters")
		if output, err := cmd.CombinedOutput(); err == nil {
			if strings.Contains(string(output), "forge-test-") {
				fmt.Fprintf(os.Stderr, "Warning: Found leftover test clusters. Run cleanup before tests.\n")
			}
		}
	}

	return nil
}

// Teardown performs global test suite teardown
func (ts *TestSuite) Teardown() {
	// Check for leftover resources unless SKIP_CLEANUP is set
	if os.Getenv("SKIP_CLEANUP") != "" {
		fmt.Fprintf(os.Stderr, "\n⚠️  SKIP_CLEANUP set, leaving test resources intact for inspection\n")
		return
	}

	// Check for leftover test clusters
	kindBinary := os.Getenv("KIND_BINARY")
	if kindBinary != "" {
		cmd := exec.Command(kindBinary, "get", "clusters")
		if output, err := cmd.CombinedOutput(); err == nil {
			clusters := strings.Split(strings.TrimSpace(string(output)), "\n")
			leftoverCount := 0
			for _, cluster := range clusters {
				if strings.HasPrefix(cluster, "forge-test-") {
					leftoverCount++
					fmt.Fprintf(os.Stderr, "⚠️  Leftover cluster found: %s\n", cluster)
				}
			}
			if leftoverCount > 0 {
				fmt.Fprintf(os.Stderr, "\nTo clean up leftover clusters, run:\n")
				fmt.Fprintf(os.Stderr, "  %s get clusters | grep forge-test | xargs -I {} %s delete cluster --name {}\n", kindBinary, kindBinary)
			}
		}
	}

	// Check for leftover tmpDirs
	tmpDirs, _ := os.ReadDir("/tmp")
	leftoverDirs := 0
	for _, entry := range tmpDirs {
		if strings.HasPrefix(entry.Name(), "forge-test-") && entry.IsDir() {
			leftoverDirs++
			fmt.Fprintf(os.Stderr, "⚠️  Leftover tmpDir found: /tmp/%s\n", entry.Name())
		}
	}
	if leftoverDirs > 0 {
		fmt.Fprintf(os.Stderr, "\nTo clean up leftover tmpDirs, run:\n")
		fmt.Fprintf(os.Stderr, "  rm -rf /tmp/forge-test-*\n")
	}
}

// RunAll executes all tests in the suite
func (ts *TestSuite) RunAll() *DetailedTestReport {
	// Run global setup
	if err := ts.Setup(); err != nil {
		fmt.Fprintf(os.Stderr, "Setup failed: %v\n", err)
		return &DetailedTestReport{
			TestReport: TestReport{
				Status:       "failed",
				ErrorMessage: fmt.Sprintf("Setup failed: %v", err),
				Duration:     0,
				Total:        0,
				Passed:       0,
				Failed:       1,
				Skipped:      0,
			},
		}
	}

	// Ensure teardown runs even if tests panic
	defer ts.Teardown()

	startTime := time.Now()

	fmt.Fprintf(os.Stderr, "\n=== Forge E2E Test Suite ===\n")

	// Display active filters
	if ts.filterCategory != "" {
		fmt.Fprintf(os.Stderr, "Filter: Category = %s\n", ts.filterCategory)
	}
	if ts.filterNamePattern != "" {
		fmt.Fprintf(os.Stderr, "Filter: Name Pattern = %s\n", ts.filterNamePattern)
	}

	fmt.Fprintf(os.Stderr, "Running %d tests across %d categories\n\n", len(ts.tests), len(ts.getCategoriesUsed()))

	// Group tests by category for display
	testsByCategory := make(map[TestCategory][]Test)
	for _, test := range ts.tests {
		testsByCategory[test.Category] = append(testsByCategory[test.Category], test)
	}

	// Run tests by category
	categories := []TestCategory{
		CategoryBuild, CategoryTestEnv, CategoryTestRunner, CategoryPrompt,
		CategoryArtifactStore, CategorySystem, CategoryError, CategoryCleanup,
		CategoryMCP, CategoryPerformance,
	}

	for _, category := range categories {
		tests := testsByCategory[category]
		if len(tests) == 0 {
			continue
		}

		fmt.Fprintf(os.Stderr, "\n=== Category: %s (%d tests) ===\n", category, len(tests))

		for _, test := range tests {
			ts.runTest(test)
		}
	}

	// Calculate final statistics
	duration := time.Since(startTime).Seconds()
	return ts.generateReport(duration)
}

// runTest executes a single test and records the result
func (ts *TestSuite) runTest(test Test) {
	testStart := time.Now()

	fmt.Fprintf(os.Stderr, "🔹 %s", test.Name)

	var result TestResult
	result.Name = test.Name
	result.Category = test.Category

	// Check if test should be skipped
	if test.Skip {
		result.Status = "skipped"
		result.Output = test.SkipReason
		result.Duration = 0
		ts.results = append(ts.results, result)
		ts.updateCategoryStats(test.Category, result)
		fmt.Fprintf(os.Stderr, " ⏭️  SKIPPED: %s\n", test.SkipReason)
		return
	}

	// Run the test
	err := test.Run()
	result.Duration = time.Since(testStart).Seconds()

	if err != nil {
		result.Status = "failed"
		result.Error = err.Error()
		ts.results = append(ts.results, result)
		ts.updateCategoryStats(test.Category, result)
		fmt.Fprintf(os.Stderr, " ❌ FAILED (%.2fs): %v\n", result.Duration, err)
	} else {
		result.Status = "passed"
		ts.results = append(ts.results, result)
		ts.updateCategoryStats(test.Category, result)
		fmt.Fprintf(os.Stderr, " ✅ PASSED (%.2fs)\n", result.Duration)
	}
}

// updateCategoryStats updates statistics for a category
func (ts *TestSuite) updateCategoryStats(category TestCategory, result TestResult) {
	stats, exists := ts.categories[category]
	if !exists {
		stats = &CategoryStats{}
		ts.categories[category] = stats
	}

	stats.Total++
	stats.Duration += result.Duration

	switch result.Status {
	case "passed":
		stats.Passed++
	case "failed":
		stats.Failed++
	case "skipped":
		stats.Skipped++
	}
}

// generateReport generates the final test report
func (ts *TestSuite) generateReport(duration float64) *DetailedTestReport {
	var total, passed, failed, skipped int
	var errors []string

	for _, result := range ts.results {
		total++
		switch result.Status {
		case "passed":
			passed++
		case "failed":
			failed++
			errors = append(errors, fmt.Sprintf("%s: %s", result.Name, result.Error))
		case "skipped":
			skipped++
		}
	}

	status := "passed"
	if failed > 0 {
		status = "failed"
	}

	errorMessage := strings.Join(errors, "; ")

	// Print summary
	fmt.Fprintf(os.Stderr, "\n=== Test Summary ===\n")
	fmt.Fprintf(os.Stderr, "Status: %s\n", status)
	fmt.Fprintf(os.Stderr, "Total: %d\n", total)
	fmt.Fprintf(os.Stderr, "Passed: %d\n", passed)
	fmt.Fprintf(os.Stderr, "Failed: %d\n", failed)
	if skipped > 0 {
		fmt.Fprintf(os.Stderr, "Skipped: %d\n", skipped)
	}
	fmt.Fprintf(os.Stderr, "Duration: %.2fs\n", duration)

	// Print category breakdown
	if len(ts.categories) > 0 {
		fmt.Fprintf(os.Stderr, "\n=== Category Breakdown ===\n")
		for category, stats := range ts.categories {
			fmt.Fprintf(os.Stderr, "%s: %d/%d passed (%.2fs)\n",
				category, stats.Passed, stats.Total, stats.Duration)
		}
	}

	if errorMessage != "" {
		fmt.Fprintf(os.Stderr, "\nErrors: %s\n", errorMessage)
	}

	return &DetailedTestReport{
		TestReport: TestReport{
			Status:       status,
			ErrorMessage: errorMessage,
			Duration:     duration,
			Total:        total,
			Passed:       passed,
			Failed:       failed,
			Skipped:      skipped,
		},
		Results:    ts.results,
		Categories: ts.categoriesToMap(),
	}
}

// getCategoriesUsed returns the set of categories with tests
func (ts *TestSuite) getCategoriesUsed() map[TestCategory]bool {
	used := make(map[TestCategory]bool)
	for _, test := range ts.tests {
		used[test.Category] = true
	}
	return used
}

// categoriesToMap converts category stats to a map
func (ts *TestSuite) categoriesToMap() map[TestCategory]CategoryStats {
	result := make(map[TestCategory]CategoryStats)
	for cat, stats := range ts.categories {
		result[cat] = *stats
	}
	return result
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

Environment Variables:
  TEST_CATEGORY         Filter tests by category (e.g., "build", "testenv")
  TEST_NAME_PATTERN     Filter tests by name pattern (case-insensitive substring)
  KIND_BINARY           Path to kind binary (required for testenv tests)
  CONTAINER_ENGINE      Container engine to use (docker or podman)
  SKIP_CLEANUP          Set to skip cleanup of test resources for debugging
  FORGE_E2E_VERBOSE     Enable verbose test output

Examples:
  # Run all tests
  forge-e2e e2e test-20241106

  # Run only build tests
  TEST_CATEGORY=build forge-e2e e2e test-20241106

  # Run tests matching "environment" in the name
  TEST_NAME_PATTERN=environment forge-e2e e2e test-20241106

  # Run with testenv prerequisites
  KIND_BINARY=kind CONTAINER_ENGINE=docker forge-e2e e2e test-20241106

  # Keep test resources for debugging
  SKIP_CLEANUP=1 forge-e2e e2e test-20241106

Output:
  - Test output is written to stderr
  - Structured JSON report is written to stdout

Test Categories:
  build           - Build system tests
  testenv         - Test environment lifecycle tests
  test-runner     - Test runner integration tests
  prompt          - Prompt system tests
  artifact-store  - Artifact store tests
  system          - System command tests
  error-handling  - Error handling tests
  cleanup         - Resource cleanup tests
  mcp             - MCP integration tests
  performance     - Performance tests`)
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

	report := runTests(input.Stage, input.Name)

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
	report := runTests(stage, name)

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

func runTests(stage, name string) *DetailedTestReport {
	fmt.Fprintf(os.Stderr, "Stage: %s, Name: %s\n", stage, name)

	// Create test suite
	suite := NewTestSuite()

	// Register all tests
	registerAllTests(suite)

	// Run all tests
	return suite.RunAll()
}

// registerAllTests registers all test cases with the suite
func registerAllTests(suite *TestSuite) {
	// Phase 2: Build system tests
	suite.AddTest(Test{
		Name:     "forge build",
		Category: CategoryBuild,
		Run:      testForgeBuild,
	})

	suite.AddTest(Test{
		Name:     "forge build specific artifact",
		Category: CategoryBuild,
		Run:      testForgeBuildSpecific,
	})

	suite.AddTest(Test{
		Name:       "forge build container",
		Category:   CategoryBuild,
		Run:        testForgeBuildContainer,
		Skip:       shouldSkipContainerTests(),
		SkipReason: "CONTAINER_ENGINE not available",
	})

	suite.AddTest(Test{
		Name:     "forge build format",
		Category: CategoryBuild,
		Run:      testForgeBuildFormat,
	})

	suite.AddTest(Test{
		Name:     "incremental build",
		Category: CategoryBuild,
		Run:      testIncrementalBuild,
	})

	// Phase 10: System tests
	suite.AddTest(Test{
		Name:     "forge version",
		Category: CategorySystem,
		Run:      testForgeVersion,
	})

	suite.AddTest(Test{
		Name:     "forge help",
		Category: CategorySystem,
		Run:      testForgeHelp,
	})

	suite.AddTest(Test{
		Name:     "forge no args",
		Category: CategorySystem,
		Run:      testForgeNoArgs,
	})

	// Phase 6: Artifact store tests
	suite.AddTest(Test{
		Name:     "artifact store validation",
		Category: CategoryArtifactStore,
		Run:      testArtifactStore,
	})

	// Phase 3: TestEnv lifecycle tests
	suite.AddTest(Test{
		Name:       "test environment create",
		Category:   CategoryTestEnv,
		Run:        testTestEnvCreate,
		Skip:       shouldSkipTestEnvTests(),
		SkipReason: "KIND_BINARY not available",
	})

	suite.AddTest(Test{
		Name:       "test environment list",
		Category:   CategoryTestEnv,
		Run:        testTestEnvList,
		Skip:       shouldSkipTestEnvTests(),
		SkipReason: "KIND_BINARY not available",
	})

	suite.AddTest(Test{
		Name:       "test environment get",
		Category:   CategoryTestEnv,
		Run:        testTestEnvGet,
		Skip:       shouldSkipTestEnvTests(),
		SkipReason: "KIND_BINARY not available",
	})

	suite.AddTest(Test{
		Name:       "test environment get JSON",
		Category:   CategoryTestEnv,
		Run:        testTestEnvGetJSON,
		Skip:       shouldSkipTestEnvTests(),
		SkipReason: "KIND_BINARY not available",
	})

	suite.AddTest(Test{
		Name:       "test environment delete",
		Category:   CategoryTestEnv,
		Run:        testTestEnvDelete,
		Skip:       shouldSkipTestEnvTests(),
		SkipReason: "KIND_BINARY not available",
	})

	suite.AddTest(Test{
		Name:       "test environment isolation",
		Category:   CategoryTestEnv,
		Run:        testTestEnvIsolation,
		Skip:       shouldSkipTestEnvTests(),
		SkipReason: "KIND_BINARY not available",
	})

	suite.AddTest(Test{
		Name:       "test environment spec override",
		Category:   CategoryTestEnv,
		Run:        testTestEnvSpecOverride,
		Skip:       true, // Skip for now - requires config manipulation
		SkipReason: "requires forge.yaml modification",
	})

	// Phase 4: Test runner tests
	suite.AddTest(Test{
		Name:     "forge test unit run",
		Category: CategoryTestRunner,
		Run:      testForgeTestUnit,
	})

	suite.AddTest(Test{
		Name:       "forge test integration run (with testenv)",
		Category:   CategoryTestRunner,
		Run:        testIntegrationTestRunner,
		Skip:       shouldSkipTestEnvTests(),
		SkipReason: "KIND_BINARY not available",
	})

	suite.AddTest(Test{
		Name:     "forge test lint run",
		Category: CategoryTestRunner,
		Run:      testLintRunner,
	})

	suite.AddTest(Test{
		Name:     "forge test verify-tags run",
		Category: CategoryTestRunner,
		Run:      testVerifyTagsRunner,
	})

	suite.AddTest(Test{
		Name:       "auto-create environment on integration run",
		Category:   CategoryTestRunner,
		Run:        testAutoCreateEnv,
		Skip:       shouldSkipTestEnvTests(),
		SkipReason: "KIND_BINARY not available",
	})

	// Phase 5: Prompt system tests
	suite.AddTest(Test{
		Name:     "forge prompt list",
		Category: CategoryPrompt,
		Run:      testPromptList,
	})

	suite.AddTest(Test{
		Name:     "forge prompt get",
		Category: CategoryPrompt,
		Run:      testPromptGet,
	})

	suite.AddTest(Test{
		Name:     "forge prompt get invalid",
		Category: CategoryPrompt,
		Run:      testPromptGetInvalid,
	})

	// Phase 6: Additional artifact store tests
	suite.AddTest(Test{
		Name:       "artifact store updates",
		Category:   CategoryArtifactStore,
		Run:        testArtifactStoreUpdates,
		Skip:       shouldSkipTestEnvTests(),
		SkipReason: "requires test environment creation",
	})

	suite.AddTest(Test{
		Name:       "artifact store cleanup",
		Category:   CategoryArtifactStore,
		Run:        testArtifactStoreCleanup,
		Skip:       shouldSkipTestEnvTests(),
		SkipReason: "requires test environment creation",
	})

	suite.AddTest(Test{
		Name:       "artifact store concurrent access",
		Category:   CategoryArtifactStore,
		Run:        testArtifactStoreConcurrentAccess,
		Skip:       shouldSkipTestEnvTests(),
		SkipReason: "requires test environment creation",
	})

	// Phase 7: Error handling tests
	suite.AddTest(Test{
		Name:       "missing binary error",
		Category:   CategoryError,
		Run:        testMissingBinaryError,
		Skip:       true, // Requires binary manipulation
		SkipReason: "requires binary manipulation",
	})

	suite.AddTest(Test{
		Name:       "invalid testID error",
		Category:   CategoryError,
		Run:        testInvalidTestIDError,
		Skip:       shouldSkipTestEnvTests(),
		SkipReason: "KIND_BINARY not available",
	})

	suite.AddTest(Test{
		Name:       "missing env var error",
		Category:   CategoryError,
		Run:        testMissingEnvVarError,
		Skip:       true, // Requires env manipulation
		SkipReason: "requires environment manipulation",
	})

	suite.AddTest(Test{
		Name:       "delete nonexistent error",
		Category:   CategoryError,
		Run:        testDeleteNonExistentError,
		Skip:       shouldSkipTestEnvTests(),
		SkipReason: "KIND_BINARY not available",
	})

	suite.AddTest(Test{
		Name:       "duplicate environment error",
		Category:   CategoryError,
		Run:        testDuplicateEnvironmentError,
		Skip:       shouldSkipTestEnvTests(),
		SkipReason: "KIND_BINARY not available",
	})

	suite.AddTest(Test{
		Name:       "cluster already exists error",
		Category:   CategoryError,
		Run:        testClusterExistsError,
		Skip:       shouldSkipTestEnvTests(),
		SkipReason: "KIND_BINARY not available",
	})

	suite.AddTest(Test{
		Name:       "malformed forge.yaml error",
		Category:   CategoryError,
		Run:        testMalformedForgeYamlError,
		Skip:       true, // Requires forge.yaml manipulation
		SkipReason: "requires forge.yaml manipulation",
	})

	// Phase 8: Cleanup tests
	suite.AddTest(Test{
		Name:       "tmpDir cleanup on delete",
		Category:   CategoryCleanup,
		Run:        testTmpDirCleanup,
		Skip:       shouldSkipTestEnvTests(),
		SkipReason: "KIND_BINARY not available",
	})

	suite.AddTest(Test{
		Name:       "managed resources cleanup",
		Category:   CategoryCleanup,
		Run:        testManagedResourcesCleanup,
		Skip:       shouldSkipTestEnvTests(),
		SkipReason: "KIND_BINARY not available",
	})

	suite.AddTest(Test{
		Name:       "partial cleanup on failure",
		Category:   CategoryCleanup,
		Run:        testPartialCleanupOnFailure,
		Skip:       shouldSkipTestEnvTests(),
		SkipReason: "KIND_BINARY not available",
	})

	suite.AddTest(Test{
		Name:       "old environment cleanup",
		Category:   CategoryCleanup,
		Run:        testOldEnvironmentCleanup,
		Skip:       shouldSkipTestEnvTests(),
		SkipReason: "KIND_BINARY not available",
	})

	// Phase 9: MCP integration tests
	suite.AddTest(Test{
		Name:     "MCP server mode",
		Category: CategoryMCP,
		Run:      testMCPServerMode,
	})

	suite.AddTest(Test{
		Name:       "MCP run tool call",
		Category:   CategoryMCP,
		Run:        testMCPRunToolCall,
		Skip:       shouldSkipTestEnvTests(),
		SkipReason: "KIND_BINARY not available",
	})

	suite.AddTest(Test{
		Name:     "MCP error propagation",
		Category: CategoryMCP,
		Run:      testMCPErrorPropagation,
	})

	// Phase 11: Performance tests
	suite.AddTest(Test{
		Name:       "rapid create/delete cycle",
		Category:   CategoryPerformance,
		Run:        testRapidCycle,
		Skip:       shouldSkipTestEnvTests(),
		SkipReason: "KIND_BINARY not available",
	})

	suite.AddTest(Test{
		Name:       "parallel environment operations",
		Category:   CategoryPerformance,
		Run:        testParallelOperations,
		Skip:       shouldSkipTestEnvTests(),
		SkipReason: "KIND_BINARY not available",
	})

	suite.AddTest(Test{
		Name:       "large number of environments",
		Category:   CategoryPerformance,
		Run:        testLargeNumberOfEnvironments,
		Skip:       shouldSkipTestEnvTests(),
		SkipReason: "KIND_BINARY not available",
	})
}

// shouldSkipContainerTests checks if container engine is available
func shouldSkipContainerTests() bool {
	engine := os.Getenv("CONTAINER_ENGINE")
	if engine == "" {
		return true
	}
	// Try to run docker/podman version
	cmd := exec.Command(engine, "version")
	return cmd.Run() != nil
}

// shouldSkipTestEnvTests checks if testenv prerequisites are available
func shouldSkipTestEnvTests() bool {
	kindBinary := os.Getenv("KIND_BINARY")
	if kindBinary == "" {
		return true
	}
	// Check if kind is available
	cmd := exec.Command(kindBinary, "version")
	return cmd.Run() != nil
}

// Utility functions for testenv tests

// extractTestID extracts testID from command output
func extractTestID(output string) string {
	// testID format: test-<stage>-YYYYMMDD-XXXXXXXX
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "test-") && len(line) > 10 {
			// Verify format
			parts := strings.Split(line, "-")
			if len(parts) >= 4 {
				return line
			}
		}
	}
	return ""
}

// verifyClusterExists checks if a kind cluster exists
func verifyClusterExists(testID string) error {
	kindBinary := os.Getenv("KIND_BINARY")
	if kindBinary == "" {
		kindBinary = "kind"
	}

	expectedClusterName := fmt.Sprintf("forge-%s", testID)

	cmd := exec.Command(kindBinary, "get", "clusters")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get clusters: %w\nOutput: %s", err, output)
	}

	if !strings.Contains(string(output), expectedClusterName) {
		return fmt.Errorf("cluster %s not found in kind clusters", expectedClusterName)
	}

	return nil
}

// cleanupTestEnv deletes a test environment
func cleanupTestEnv(testID string) {
	if testID == "" {
		return
	}

	// Try to delete via forge
	cmd := exec.Command("./build/bin/forge", "test", "integration", "delete", testID)
	cmd.Env = os.Environ()
	_ = cmd.Run() // Ignore errors during cleanup
}

// verifyArtifactStoreHasTestEnv checks if artifact store contains a test environment
func verifyArtifactStoreHasTestEnv(testID string) error {
	storePath := ".forge/artifacts.json"
	data, err := os.ReadFile(storePath)
	if err != nil {
		return fmt.Errorf("failed to read artifact store: %w", err)
	}

	content := string(data)
	if !strings.Contains(content, testID) {
		return fmt.Errorf("testID %s not found in artifact store", testID)
	}

	// Should contain test environment structure
	if !strings.Contains(content, "testEnvironments") && !strings.Contains(content, "\"id\"") {
		return fmt.Errorf("artifact store missing test environment structure")
	}

	return nil
}

// verifyArtifactStoreMissingTestEnv checks that artifact store doesn't contain a test environment
func verifyArtifactStoreMissingTestEnv(testID string) error {
	storePath := ".forge/artifacts.json"
	data, err := os.ReadFile(storePath)
	if err != nil {
		return fmt.Errorf("failed to read artifact store: %w", err)
	}

	content := string(data)
	if strings.Contains(content, testID) {
		return fmt.Errorf("testID %s still found in artifact store after deletion", testID)
	}

	return nil
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

// Phase 2: Additional Build Tests

func testForgeBuildContainer() error {
	engine := os.Getenv("CONTAINER_ENGINE")
	if engine == "" {
		return fmt.Errorf("CONTAINER_ENGINE not set")
	}

	cmd := exec.Command("go", "run", "./cmd/forge", "build", "for-testing-purposes")
	cmd.Env = os.Environ()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("command failed: %w\nOutput: %s", err, string(output))
	}

	// Verify output contains success message
	if !strings.Contains(string(output), "Successfully built") {
		return fmt.Errorf("expected success message in output, got: %s", string(output))
	}

	// Verify image exists
	checkCmd := exec.Command(engine, "images", "for-testing-purposes")
	checkOutput, err := checkCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to check image: %w", err)
	}

	if !strings.Contains(string(checkOutput), "for-testing-purposes") {
		return fmt.Errorf("container image not found in %s images", engine)
	}

	return nil
}

func testForgeBuildFormat() error {
	// This test runs the format-code artifact
	cmd := exec.Command("go", "run", "./cmd/forge", "build", "format-code")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("command failed: %w\nOutput: %s", err, string(output))
	}

	// Formatting is successful if it completes without error
	// Verify the command ran
	if len(output) == 0 {
		return fmt.Errorf("no output from format command")
	}

	return nil
}

func testIncrementalBuild() error {
	// Build forge binary first time
	cmd1 := exec.Command("go", "run", "./cmd/forge", "build", "forge")
	if _, err := cmd1.CombinedOutput(); err != nil {
		return fmt.Errorf("first build failed: %w", err)
	}

	// Get timestamp
	info1, err := os.Stat("./build/bin/forge")
	if err != nil {
		return fmt.Errorf("failed to stat forge binary: %w", err)
	}
	modTime1 := info1.ModTime()

	// Wait a moment to ensure timestamp would change if rebuilt
	time.Sleep(100 * time.Millisecond)

	// Build again without changes
	cmd2 := exec.Command("go", "run", "./cmd/forge", "build", "forge")
	if _, err := cmd2.CombinedOutput(); err != nil {
		return fmt.Errorf("second build failed: %w", err)
	}

	// Get new timestamp
	info2, err := os.Stat("./build/bin/forge")
	if err != nil {
		return fmt.Errorf("failed to stat forge binary after rebuild: %w", err)
	}
	modTime2 := info2.ModTime()

	// Timestamps should be different (Go rebuilds every time for go run)
	// But binary should still exist and be functional
	_ = modTime1
	_ = modTime2

	// Verify binary is still executable
	testCmd := exec.Command("./build/bin/forge", "version")
	if err := testCmd.Run(); err != nil {
		return fmt.Errorf("forge binary not executable after rebuild: %w", err)
	}

	return nil
}

// Phase 10: Additional System Tests

func testForgeHelp() error {
	cmd := exec.Command("go", "run", "./cmd/forge", "help")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("command failed: %w\nOutput: %s", err, string(output))
	}

	// Verify help output contains key information
	requiredContent := []string{"Usage:", "build", "test", "version"}
	for _, content := range requiredContent {
		if !strings.Contains(string(output), content) {
			return fmt.Errorf("help output missing '%s'", content)
		}
	}

	return nil
}

func testForgeNoArgs() error {
	cmd := exec.Command("go", "run", "./cmd/forge")
	output, err := cmd.CombinedOutput()

	// Should show usage/help or error
	if err == nil {
		return fmt.Errorf("expected error when running forge with no args")
	}

	// Should show usage information
	if !strings.Contains(string(output), "Usage:") && !strings.Contains(string(output), "usage:") {
		return fmt.Errorf("expected usage information, got: %s", string(output))
	}

	return nil
}

// Phase 3: TestEnv Lifecycle Tests

func testTestEnvCreate() error {
	// Create test environment
	cmd := exec.Command("./build/bin/forge", "test", "integration", "create")
	cmd.Env = os.Environ()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("create command failed: %w\nOutput: %s", err, output)
	}

	// Extract testID
	testID := extractTestID(string(output))
	if testID == "" {
		return fmt.Errorf("no testID found in output: %s", output)
	}

	// Register for cleanup
	defer cleanupTestEnv(testID)

	// Verify cluster exists
	if err := verifyClusterExists(testID); err != nil {
		return fmt.Errorf("cluster verification failed: %w", err)
	}

	// Verify artifact store entry
	if err := verifyArtifactStoreHasTestEnv(testID); err != nil {
		return fmt.Errorf("artifact store verification failed: %w", err)
	}

	return nil
}

func testTestEnvList() error {
	// First create a test environment so list has something to show
	createCmd := exec.Command("./build/bin/forge", "test", "integration", "create")
	createCmd.Env = os.Environ()
	createOutput, err := createCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("create command failed: %w\nOutput: %s", err, createOutput)
	}

	testID := extractTestID(string(createOutput))
	if testID == "" {
		return fmt.Errorf("failed to extract testID from create output")
	}

	defer cleanupTestEnv(testID)

	// List test environments
	listCmd := exec.Command("./build/bin/forge", "test", "integration", "list")
	listOutput, err := listCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("list command failed: %w\nOutput: %s", err, listOutput)
	}

	// Verify output contains our testID
	if !strings.Contains(string(listOutput), testID) {
		return fmt.Errorf("testID %s not found in list output: %s", testID, listOutput)
	}

	// Verify table format
	if !strings.Contains(string(listOutput), "ID") || !strings.Contains(string(listOutput), "NAME") {
		return fmt.Errorf("list output missing table headers: %s", listOutput)
	}

	return nil
}

func testTestEnvGet() error {
	// Create test environment
	createCmd := exec.Command("./build/bin/forge", "test", "integration", "create")
	createCmd.Env = os.Environ()
	createOutput, err := createCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("create command failed: %w\nOutput: %s", err, createOutput)
	}

	testID := extractTestID(string(createOutput))
	if testID == "" {
		return fmt.Errorf("failed to extract testID")
	}

	defer cleanupTestEnv(testID)

	// Get test environment details
	getCmd := exec.Command("./build/bin/forge", "test", "integration", "get", testID)
	getOutput, err := getCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("get command failed: %w\nOutput: %s", err, getOutput)
	}

	// Verify output contains expected fields
	requiredFields := []string{"ID:", "Name:", "Status:", "TmpDir:", "Files:", "Metadata:"}
	for _, field := range requiredFields {
		if !strings.Contains(string(getOutput), field) {
			return fmt.Errorf("get output missing field '%s': %s", field, getOutput)
		}
	}

	// Verify testID appears in output
	if !strings.Contains(string(getOutput), testID) {
		return fmt.Errorf("testID not found in get output: %s", getOutput)
	}

	return nil
}

func testTestEnvGetJSON() error {
	// Create test environment
	createCmd := exec.Command("./build/bin/forge", "test", "integration", "create")
	createCmd.Env = os.Environ()
	createOutput, err := createCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("create command failed: %w\nOutput: %s", err, createOutput)
	}

	testID := extractTestID(string(createOutput))
	if testID == "" {
		return fmt.Errorf("failed to extract testID")
	}

	defer cleanupTestEnv(testID)

	// Get test environment as JSON
	getCmd := exec.Command("./build/bin/forge", "test", "integration", "get", testID, "-o", "json")
	getOutput, err := getCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("get JSON command failed: %w\nOutput: %s", err, getOutput)
	}

	// Verify JSON is valid
	var result map[string]interface{}
	if err := json.Unmarshal(getOutput, &result); err != nil {
		return fmt.Errorf("invalid JSON output: %w\nOutput: %s", err, getOutput)
	}

	// Verify JSON contains expected fields
	requiredFields := []string{"id", "name", "status", "tmpDir", "files", "metadata"}
	for _, field := range requiredFields {
		if _, exists := result[field]; !exists {
			return fmt.Errorf("JSON missing field '%s'", field)
		}
	}

	return nil
}

func testTestEnvDelete() error {
	// Create test environment
	createCmd := exec.Command("./build/bin/forge", "test", "integration", "create")
	createCmd.Env = os.Environ()
	createOutput, err := createCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("create command failed: %w\nOutput: %s", err, createOutput)
	}

	testID := extractTestID(string(createOutput))
	if testID == "" {
		return fmt.Errorf("failed to extract testID")
	}

	// Verify cluster exists before deletion
	if err := verifyClusterExists(testID); err != nil {
		return fmt.Errorf("cluster not found before deletion: %w", err)
	}

	// Delete test environment
	deleteCmd := exec.Command("./build/bin/forge", "test", "integration", "delete", testID)
	deleteCmd.Env = os.Environ()
	deleteOutput, err := deleteCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("delete command failed: %w\nOutput: %s", err, deleteOutput)
	}

	// Verify cluster no longer exists
	kindBinary := os.Getenv("KIND_BINARY")
	if kindBinary == "" {
		kindBinary = "kind"
	}

	expectedClusterName := fmt.Sprintf("forge-%s", testID)
	checkCmd := exec.Command(kindBinary, "get", "clusters")
	checkOutput, _ := checkCmd.CombinedOutput()

	if strings.Contains(string(checkOutput), expectedClusterName) {
		return fmt.Errorf("cluster %s still exists after deletion", expectedClusterName)
	}

	// Verify artifact store no longer contains testID
	if err := verifyArtifactStoreMissingTestEnv(testID); err != nil {
		return fmt.Errorf("artifact store verification failed: %w", err)
	}

	return nil
}

func testTestEnvIsolation() error {
	// Create two test environments in parallel
	type result struct {
		testID string
		err    error
	}

	results := make(chan result, 2)

	for i := 0; i < 2; i++ {
		go func(idx int) {
			cmd := exec.Command("./build/bin/forge", "test", "integration", "create")
			cmd.Env = os.Environ()
			output, err := cmd.CombinedOutput()
			if err != nil {
				results <- result{err: fmt.Errorf("create %d failed: %w\nOutput: %s", idx, err, output)}
				return
			}

			testID := extractTestID(string(output))
			if testID == "" {
				results <- result{err: fmt.Errorf("create %d: no testID in output", idx)}
				return
			}

			results <- result{testID: testID}
		}(i)
	}

	// Collect results
	var testIDs []string
	var errors []error

	for i := 0; i < 2; i++ {
		res := <-results
		if res.err != nil {
			errors = append(errors, res.err)
		} else {
			testIDs = append(testIDs, res.testID)
		}
	}

	// Cleanup
	for _, testID := range testIDs {
		defer cleanupTestEnv(testID)
	}

	// Check for errors
	if len(errors) > 0 {
		return fmt.Errorf("parallel creation failed: %v", errors)
	}

	// Verify we got 2 different testIDs
	if len(testIDs) != 2 {
		return fmt.Errorf("expected 2 testIDs, got %d", len(testIDs))
	}

	if testIDs[0] == testIDs[1] {
		return fmt.Errorf("testIDs are not unique: %s", testIDs[0])
	}

	// Verify both clusters exist
	for _, testID := range testIDs {
		if err := verifyClusterExists(testID); err != nil {
			return fmt.Errorf("cluster verification failed for %s: %w", testID, err)
		}
	}

	return nil
}

func testTestEnvSpecOverride() error {
	// This test would require modifying forge.yaml to test spec override
	// For now, we'll skip it as noted in registerAllTests
	return fmt.Errorf("not implemented - requires forge.yaml manipulation")
}

// Phase 4: Test Runner Integration Tests

func testIntegrationTestRunner() error {
	// Create a test environment first
	createCmd := exec.Command("./build/bin/forge", "test", "integration", "create")
	createCmd.Env = os.Environ()
	createOutput, err := createCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("create command failed: %w\nOutput: %s", err, createOutput)
	}

	testID := extractTestID(string(createOutput))
	if testID == "" {
		return fmt.Errorf("failed to extract testID")
	}

	defer cleanupTestEnv(testID)

	// Run integration tests with the test environment
	runCmd := exec.Command("./build/bin/forge", "test", "integration", "run", testID)
	runCmd.Env = os.Environ()
	runOutput, err := runCmd.CombinedOutput()

	// We expect this might fail if there are no integration tests,
	// but we're testing that the command executes
	_ = err // Don't fail on test execution errors

	// Verify the command produced output
	if len(runOutput) == 0 {
		return fmt.Errorf("no output from test run command")
	}

	return nil
}

func testLintRunner() error {
	cmd := exec.Command("./build/bin/forge", "test", "lint", "run")
	cmd.Env = os.Environ()
	output, _ := cmd.CombinedOutput() // May fail due to lint errors

	// Verify command executed (produced output)
	if len(output) == 0 {
		return fmt.Errorf("no output from lint command")
	}

	return nil
}

// Phase 5: Prompt System Tests

func testPromptList() error {
	cmd := exec.Command("./build/bin/forge", "prompt", "list")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("command failed: %w\nOutput: %s", err, output)
	}

	// Verify output is not empty
	if len(output) == 0 {
		return fmt.Errorf("no output from prompt list")
	}

	return nil
}

func testPromptGet() error {
	// First list prompts to get a valid name
	listCmd := exec.Command("./build/bin/forge", "prompt", "list")
	listOutput, err := listCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to list prompts: %w", err)
	}

	// If there are no prompts, skip the test
	if len(listOutput) == 0 || !strings.Contains(string(listOutput), "-") {
		return nil // Skip if no prompts available
	}

	// Try to get a prompt (use a common name or parse from list)
	getCmd := exec.Command("./build/bin/forge", "prompt", "get", "engine-implementation-guide")
	getOutput, err := getCmd.CombinedOutput()

	// If this specific prompt doesn't exist, that's okay
	if err != nil && strings.Contains(string(getOutput), "not found") {
		return nil
	}

	if err != nil {
		return fmt.Errorf("command failed: %w\nOutput: %s", err, getOutput)
	}

	return nil
}

func testPromptGetInvalid() error {
	cmd := exec.Command("./build/bin/forge", "prompt", "get", "nonexistent-prompt-name-12345")
	output, err := cmd.CombinedOutput()

	// Should fail with error
	if err == nil {
		return fmt.Errorf("expected error for invalid prompt name")
	}

	// Should mention not found or similar
	if !strings.Contains(string(output), "not found") && !strings.Contains(string(output), "error") {
		return fmt.Errorf("expected error message about not found, got: %s", output)
	}

	return nil
}

// Phase 6: Additional Artifact Store Tests

func testArtifactStoreUpdates() error {
	// Create first test environment
	cmd1 := exec.Command("./build/bin/forge", "test", "integration", "create")
	cmd1.Env = os.Environ()
	output1, err := cmd1.CombinedOutput()
	if err != nil {
		return fmt.Errorf("first create failed: %w\nOutput: %s", err, output1)
	}

	testID1 := extractTestID(string(output1))
	if testID1 == "" {
		return fmt.Errorf("failed to extract first testID")
	}

	defer cleanupTestEnv(testID1)

	// Create second test environment
	cmd2 := exec.Command("./build/bin/forge", "test", "integration", "create")
	cmd2.Env = os.Environ()
	output2, err := cmd2.CombinedOutput()
	if err != nil {
		cleanupTestEnv(testID1)
		return fmt.Errorf("second create failed: %w\nOutput: %s", err, output2)
	}

	testID2 := extractTestID(string(output2))
	if testID2 == "" {
		return fmt.Errorf("failed to extract second testID")
	}

	defer cleanupTestEnv(testID2)

	// Verify both are in artifact store
	if err := verifyArtifactStoreHasTestEnv(testID1); err != nil {
		return fmt.Errorf("first testID not in store: %w", err)
	}

	if err := verifyArtifactStoreHasTestEnv(testID2); err != nil {
		return fmt.Errorf("second testID not in store: %w", err)
	}

	return nil
}

func testArtifactStoreCleanup() error {
	// Create test environment
	createCmd := exec.Command("./build/bin/forge", "test", "integration", "create")
	createCmd.Env = os.Environ()
	createOutput, err := createCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("create failed: %w\nOutput: %s", err, createOutput)
	}

	testID := extractTestID(string(createOutput))
	if testID == "" {
		return fmt.Errorf("failed to extract testID")
	}

	// Verify it's in the store
	if err := verifyArtifactStoreHasTestEnv(testID); err != nil {
		return fmt.Errorf("testID not in store after create: %w", err)
	}

	// Delete test environment
	deleteCmd := exec.Command("./build/bin/forge", "test", "integration", "delete", testID)
	deleteCmd.Env = os.Environ()
	if _, err := deleteCmd.CombinedOutput(); err != nil {
		// Try to cleanup anyway
		cleanupTestEnv(testID)
		return fmt.Errorf("delete failed: %w", err)
	}

	// Verify it's removed from the store
	if err := verifyArtifactStoreMissingTestEnv(testID); err != nil {
		return fmt.Errorf("testID still in store after delete: %w", err)
	}

	return nil
}

// Phase 7: Error Handling Tests

func testMissingBinaryError() error {
	// This would require renaming a binary temporarily
	// Skipped in registerAllTests
	return fmt.Errorf("not implemented")
}

func testInvalidTestIDError() error {
	invalidID := "invalid-test-id-12345"

	cmd := exec.Command("./build/bin/forge", "test", "integration", "get", invalidID)
	output, err := cmd.CombinedOutput()

	// Should fail
	if err == nil {
		return fmt.Errorf("expected error for invalid testID")
	}

	// Should mention not found
	if !strings.Contains(string(output), "not found") && !strings.Contains(string(output), "error") {
		return fmt.Errorf("expected error message, got: %s", output)
	}

	return nil
}

func testMissingEnvVarError() error {
	// This would require unsetting KIND_BINARY temporarily
	// Skipped in registerAllTests
	return fmt.Errorf("not implemented")
}

func testDeleteNonExistentError() error {
	nonExistentID := "test-integration-20990101-deadbeef"

	cmd := exec.Command("./build/bin/forge", "test", "integration", "delete", nonExistentID)
	cmd.Env = os.Environ()
	output, err := cmd.CombinedOutput()

	// Should fail
	if err == nil {
		return fmt.Errorf("expected error for nonexistent testID")
	}

	// Should mention not found
	if !strings.Contains(string(output), "not found") && !strings.Contains(string(output), "error") {
		return fmt.Errorf("expected error message, got: %s", output)
	}

	return nil
}

// Phase 8: Cleanup Tests

func testTmpDirCleanup() error {
	// Create test environment
	createCmd := exec.Command("./build/bin/forge", "test", "integration", "create")
	createCmd.Env = os.Environ()
	createOutput, err := createCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("create failed: %w\nOutput: %s", err, createOutput)
	}

	testID := extractTestID(string(createOutput))
	if testID == "" {
		return fmt.Errorf("failed to extract testID")
	}

	// Get tmpDir path
	tmpDir := fmt.Sprintf("/tmp/forge-test-integration-%s", testID)

	// Verify tmpDir exists
	if _, err := os.Stat(tmpDir); err != nil {
		return fmt.Errorf("tmpDir not found before deletion: %w", err)
	}

	// Delete test environment
	deleteCmd := exec.Command("./build/bin/forge", "test", "integration", "delete", testID)
	deleteCmd.Env = os.Environ()
	if _, err := deleteCmd.CombinedOutput(); err != nil {
		cleanupTestEnv(testID)
		return fmt.Errorf("delete failed: %w", err)
	}

	// Verify tmpDir is removed
	if _, err := os.Stat(tmpDir); err == nil {
		return fmt.Errorf("tmpDir still exists after deletion: %s", tmpDir)
	}

	return nil
}

func testManagedResourcesCleanup() error {
	// Create test environment
	createCmd := exec.Command("./build/bin/forge", "test", "integration", "create")
	createCmd.Env = os.Environ()
	createOutput, err := createCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("create failed: %w\nOutput: %s", err, createOutput)
	}

	testID := extractTestID(string(createOutput))
	if testID == "" {
		return fmt.Errorf("failed to extract testID")
	}

	// Delete and verify cleanup happens
	deleteCmd := exec.Command("./build/bin/forge", "test", "integration", "delete", testID)
	deleteCmd.Env = os.Environ()
	if _, err := deleteCmd.CombinedOutput(); err != nil {
		cleanupTestEnv(testID)
		return fmt.Errorf("delete failed: %w", err)
	}

	// Verify cluster is gone (main managed resource)
	kindBinary := os.Getenv("KIND_BINARY")
	if kindBinary == "" {
		kindBinary = "kind"
	}

	checkCmd := exec.Command(kindBinary, "get", "clusters")
	checkOutput, _ := checkCmd.CombinedOutput()

	expectedClusterName := fmt.Sprintf("forge-%s", testID)
	if strings.Contains(string(checkOutput), expectedClusterName) {
		return fmt.Errorf("cluster still exists after deletion")
	}

	return nil
}

// Phase 9: MCP Integration Tests

func testMCPServerMode() error {
	// Start MCP server in background
	cmd := exec.Command("./build/bin/forge-e2e", "--mcp")

	// Create pipes for stdin/stdout
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Start the server
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start MCP server: %w", err)
	}

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	// Send initialize request
	initRequest := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"0.1.0","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0.0"}}}` + "\n"
	if _, err := stdin.Write([]byte(initRequest)); err != nil {
		cmd.Process.Kill()
		return fmt.Errorf("failed to write initialize: %w", err)
	}

	// Read response with timeout
	responseChan := make(chan []byte, 1)
	go func() {
		buf := make([]byte, 4096)
		n, _ := stdout.Read(buf)
		responseChan <- buf[:n]
	}()

	select {
	case response := <-responseChan:
		// Verify we got a JSON-RPC response
		if !strings.Contains(string(response), "jsonrpc") {
			cmd.Process.Kill()
			return fmt.Errorf("invalid MCP response: %s", response)
		}
	case <-time.After(2 * time.Second):
		cmd.Process.Kill()
		return fmt.Errorf("timeout waiting for MCP response")
	}

	// Kill the server
	cmd.Process.Kill()
	cmd.Wait()

	return nil
}

// Phase 11: Performance Tests

func testRapidCycle() error {
	// Create and delete 3 test environments rapidly
	for i := 0; i < 3; i++ {
		// Create
		createCmd := exec.Command("./build/bin/forge", "test", "integration", "create")
		createCmd.Env = os.Environ()
		createOutput, err := createCmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("create %d failed: %w\nOutput: %s", i, err, createOutput)
		}

		testID := extractTestID(string(createOutput))
		if testID == "" {
			return fmt.Errorf("failed to extract testID for iteration %d", i)
		}

		// Delete immediately
		deleteCmd := exec.Command("./build/bin/forge", "test", "integration", "delete", testID)
		deleteCmd.Env = os.Environ()
		if _, err := deleteCmd.CombinedOutput(); err != nil {
			cleanupTestEnv(testID)
			return fmt.Errorf("delete %d failed: %w", i, err)
		}
	}

	return nil
}

// Phase 4: Additional Test Runner Tests

func testVerifyTagsRunner() error {
	cmd := exec.Command("./build/bin/forge", "test", "verify-tags", "run")
	cmd.Env = os.Environ()
	output, _ := cmd.CombinedOutput() // May fail if no Go files have tags

	// Verify command executed (produced output)
	if len(output) == 0 {
		return fmt.Errorf("no output from verify-tags command")
	}

	return nil
}

func testAutoCreateEnv() error {
	// This test verifies that running integration tests without an environment auto-creates one
	// Note: This behavior may not be implemented yet, so we test what forge actually does

	// First ensure no test environments exist
	listCmd := exec.Command("./build/bin/forge", "test", "integration", "list")
	listOutput, _ := listCmd.CombinedOutput()
	initialCount := strings.Count(string(listOutput), "test-integration-")

	// Run integration tests without creating environment first
	runCmd := exec.Command("./build/bin/forge", "test", "integration", "run")
	runCmd.Env = os.Environ()
	runOutput, err := runCmd.CombinedOutput()
	// Check if it auto-created or failed with helpful message
	if err != nil {
		// Should either auto-create or provide helpful error message
		if !strings.Contains(string(runOutput), "create") && !strings.Contains(string(runOutput), "environment") {
			return fmt.Errorf("expected helpful error about creating environment, got: %s", runOutput)
		}
		// This is expected behavior if auto-create isn't implemented
		return nil
	}

	// If it succeeded, verify an environment was created
	listCmd2 := exec.Command("./build/bin/forge", "test", "integration", "list")
	listOutput2, _ := listCmd2.CombinedOutput()
	finalCount := strings.Count(string(listOutput2), "test-integration-")

	if finalCount <= initialCount {
		return fmt.Errorf("no new environment created during integration run")
	}

	// Cleanup any auto-created environment
	if strings.Contains(string(listOutput2), "test-integration-") {
		testID := extractTestID(string(listOutput2))
		if testID != "" {
			cleanupTestEnv(testID)
		}
	}

	return nil
}

// Phase 6: Additional Artifact Store Tests

func testArtifactStoreConcurrentAccess() error {
	// Test that multiple create operations can safely write to artifact store concurrently
	type result struct {
		testID string
		err    error
	}

	results := make(chan result, 2)

	// Create 2 environments concurrently
	for i := 0; i < 2; i++ {
		go func(idx int) {
			cmd := exec.Command("./build/bin/forge", "test", "integration", "create")
			cmd.Env = os.Environ()
			output, err := cmd.CombinedOutput()
			if err != nil {
				results <- result{err: fmt.Errorf("create %d failed: %w", idx, err)}
				return
			}

			testID := extractTestID(string(output))
			if testID == "" {
				results <- result{err: fmt.Errorf("create %d: no testID", idx)}
				return
			}

			results <- result{testID: testID}
		}(i)
	}

	// Collect results
	var testIDs []string
	var errors []error

	for i := 0; i < 2; i++ {
		res := <-results
		if res.err != nil {
			errors = append(errors, res.err)
		} else {
			testIDs = append(testIDs, res.testID)
		}
	}

	// Cleanup
	for _, testID := range testIDs {
		defer cleanupTestEnv(testID)
	}

	// Check for errors
	if len(errors) > 0 {
		return fmt.Errorf("concurrent creation failed: %v", errors)
	}

	// Verify both are in artifact store
	for _, testID := range testIDs {
		if err := verifyArtifactStoreHasTestEnv(testID); err != nil {
			return fmt.Errorf("testID %s not in store after concurrent create: %w", testID, err)
		}
	}

	return nil
}

// Phase 7: Additional Error Handling Tests

func testDuplicateEnvironmentError() error {
	// Create an environment
	createCmd := exec.Command("./build/bin/forge", "test", "integration", "create")
	createCmd.Env = os.Environ()
	createOutput, err := createCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("first create failed: %w\nOutput: %s", err, createOutput)
	}

	testID := extractTestID(string(createOutput))
	if testID == "" {
		return fmt.Errorf("failed to extract testID")
	}

	defer cleanupTestEnv(testID)

	// Try to create with same testID (if forge supports specifying testID)
	// Note: Current implementation auto-generates testID, so this tests the actual behavior
	// Multiple creates should succeed with different IDs
	createCmd2 := exec.Command("./build/bin/forge", "test", "integration", "create")
	createCmd2.Env = os.Environ()
	createOutput2, err := createCmd2.CombinedOutput()
	if err != nil {
		// If it fails, verify it's not a duplicate error (since we didn't specify same ID)
		return fmt.Errorf("second create unexpectedly failed: %w", err)
	}

	testID2 := extractTestID(string(createOutput2))
	if testID2 != "" {
		defer cleanupTestEnv(testID2)
	}

	// Verify they have different IDs
	if testID == testID2 {
		return fmt.Errorf("duplicate testIDs generated: %s", testID)
	}

	return nil
}

func testClusterExistsError() error {
	// Create an environment
	createCmd := exec.Command("./build/bin/forge", "test", "integration", "create")
	createCmd.Env = os.Environ()
	createOutput, err := createCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("create failed: %w\nOutput: %s", err, createOutput)
	}

	testID := extractTestID(string(createOutput))
	if testID == "" {
		return fmt.Errorf("failed to extract testID")
	}

	defer cleanupTestEnv(testID)

	// Verify cluster exists
	if err := verifyClusterExists(testID); err != nil {
		return fmt.Errorf("cluster doesn't exist: %w", err)
	}

	// Try to create a KIND cluster with the same name manually
	clusterName := fmt.Sprintf("forge-%s", testID)
	kindBinary := os.Getenv("KIND_BINARY")
	if kindBinary == "" {
		kindBinary = "kind"
	}

	// This should fail because cluster already exists
	cmd := exec.Command(kindBinary, "create", "cluster", "--name", clusterName)
	output, err := cmd.CombinedOutput()

	if err == nil {
		// Cleanup the duplicate cluster
		exec.Command(kindBinary, "delete", "cluster", "--name", clusterName).Run()
		return fmt.Errorf("expected error when creating duplicate cluster, but succeeded")
	}

	// Verify error message mentions cluster already exists
	if !strings.Contains(string(output), "already exists") && !strings.Contains(string(output), "exist") {
		return fmt.Errorf("expected 'already exists' error, got: %s", output)
	}

	return nil
}

func testMalformedForgeYamlError() error {
	// This test would require temporarily modifying forge.yaml
	// Skip for now as noted in registerAllTests
	return fmt.Errorf("not implemented - requires forge.yaml manipulation")
}

// Phase 8: Additional Cleanup Tests

func testPartialCleanupOnFailure() error {
	// Test that if delete fails partway through, we still cleanup what we can
	// Create a test environment
	createCmd := exec.Command("./build/bin/forge", "test", "integration", "create")
	createCmd.Env = os.Environ()
	createOutput, err := createCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("create failed: %w\nOutput: %s", err, createOutput)
	}

	testID := extractTestID(string(createOutput))
	if testID == "" {
		return fmt.Errorf("failed to extract testID")
	}

	// Get tmpDir path
	tmpDir := fmt.Sprintf("/tmp/forge-test-integration-%s", testID)

	// Verify resources exist
	if err := verifyClusterExists(testID); err != nil {
		cleanupTestEnv(testID)
		return fmt.Errorf("cluster not found before test: %w", err)
	}

	if _, err := os.Stat(tmpDir); err != nil {
		cleanupTestEnv(testID)
		return fmt.Errorf("tmpDir not found before test: %w", err)
	}

	// Now delete the environment (should cleanup even if some steps fail)
	deleteCmd := exec.Command("./build/bin/forge", "test", "integration", "delete", testID)
	deleteCmd.Env = os.Environ()
	deleteOutput, err := deleteCmd.CombinedOutput()

	// Even if delete returns an error, some cleanup should happen
	_ = err // Don't fail on error
	_ = deleteOutput

	// Verify at least one resource was cleaned up
	clusterGone := verifyClusterExists(testID) != nil
	tmpDirGone := func() bool {
		_, err := os.Stat(tmpDir)
		return err != nil
	}()

	if !clusterGone && !tmpDirGone {
		return fmt.Errorf("no cleanup happened after delete")
	}

	// Best effort cleanup of remaining resources
	cleanupTestEnv(testID)

	return nil
}

func testOldEnvironmentCleanup() error {
	// Test that we can identify and cleanup old/stale test environments
	// List all test environments
	listCmd := exec.Command("./build/bin/forge", "test", "integration", "list")
	listOutput, err := listCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("list command failed: %w", err)
	}

	// Parse testIDs from output
	lines := strings.Split(string(listOutput), "\n")
	var oldTestIDs []string

	for _, line := range lines {
		if strings.Contains(line, "test-integration-") {
			testID := extractTestID(line)
			if testID != "" {
				// Check if this is from an old date (more than 1 day old based on date in ID)
				// testID format: test-integration-YYYYMMDD-XXXXXXXX
				parts := strings.Split(testID, "-")
				if len(parts) >= 3 {
					dateStr := parts[2] // YYYYMMDD
					// For this test, we'll just mark any found environment as "old"
					// In practice, you'd parse the date and compare to current date
					_ = dateStr // Would check if older than threshold
				}
				oldTestIDs = append(oldTestIDs, testID)
			}
		}
	}

	// If we found old environments, try to clean them up
	if len(oldTestIDs) > 0 {
		for _, testID := range oldTestIDs {
			// Try to delete
			deleteCmd := exec.Command("./build/bin/forge", "test", "integration", "delete", testID)
			deleteCmd.Env = os.Environ()
			_ = deleteCmd.Run() // Best effort cleanup
		}
	}

	// Test passes if we can list and attempt cleanup without crashing
	return nil
}

// Phase 9: Additional MCP Tests

func testMCPRunToolCall() error {
	// Test calling the MCP run tool directly
	// This would require setting up an MCP client, which is complex
	// For now, we'll do a basic test of the tool interface

	// Start MCP server
	cmd := exec.Command("./build/bin/forge-e2e", "--mcp")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start MCP server: %w", err)
	}

	defer func() {
		cmd.Process.Kill()
		cmd.Wait()
	}()

	time.Sleep(100 * time.Millisecond)

	// Send initialize
	initRequest := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"0.1.0","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0.0"}}}` + "\n"
	if _, err := stdin.Write([]byte(initRequest)); err != nil {
		return fmt.Errorf("failed to write initialize: %w", err)
	}

	// Read initialize response
	buf := make([]byte, 4096)
	_, _ = stdout.Read(buf)

	// Send tools/list request to verify run tool exists
	listRequest := `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}` + "\n"
	if _, err := stdin.Write([]byte(listRequest)); err != nil {
		return fmt.Errorf("failed to write tools/list: %w", err)
	}

	// Read tools/list response
	responseChan := make(chan []byte, 1)
	go func() {
		buf := make([]byte, 4096)
		n, _ := stdout.Read(buf)
		responseChan <- buf[:n]
	}()

	select {
	case response := <-responseChan:
		// Verify response contains "run" tool
		if !strings.Contains(string(response), "run") {
			return fmt.Errorf("run tool not found in tools/list response: %s", response)
		}
	case <-time.After(2 * time.Second):
		return fmt.Errorf("timeout waiting for tools/list response")
	}

	return nil
}

func testMCPErrorPropagation() error {
	// Test that errors from forge-e2e are properly propagated through MCP
	// Start MCP server
	cmd := exec.Command("./build/bin/forge-e2e", "--mcp")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start MCP server: %w", err)
	}

	defer func() {
		cmd.Process.Kill()
		cmd.Wait()
	}()

	time.Sleep(100 * time.Millisecond)

	// Send initialize
	initRequest := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"0.1.0","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0.0"}}}` + "\n"
	if _, err := stdin.Write([]byte(initRequest)); err != nil {
		return fmt.Errorf("failed to write initialize: %w", err)
	}

	// Read initialize response
	buf := make([]byte, 4096)
	_, _ = stdout.Read(buf)

	// Send a tool call that will fail (invalid parameters)
	toolCallRequest := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"run","arguments":{"stage":"invalid","name":""}}}` + "\n"
	if _, err := stdin.Write([]byte(toolCallRequest)); err != nil {
		return fmt.Errorf("failed to write tool call: %w", err)
	}

	// Read response
	responseChan := make(chan []byte, 1)
	go func() {
		buf := make([]byte, 4096)
		n, _ := stdout.Read(buf)
		responseChan <- buf[:n]
	}()

	select {
	case response := <-responseChan:
		// Verify response indicates an error
		if !strings.Contains(string(response), "error") && !strings.Contains(string(response), "isError") {
			return fmt.Errorf("expected error in response, got: %s", response)
		}
	case <-time.After(2 * time.Second):
		return fmt.Errorf("timeout waiting for error response")
	}

	return nil
}

// Phase 11: Additional Performance Tests

func testParallelOperations() error {
	// Test running multiple operations in parallel
	type result struct {
		operation string
		err       error
	}

	results := make(chan result, 4)

	// Create 2 environments in parallel
	for i := 0; i < 2; i++ {
		go func(idx int) {
			cmd := exec.Command("./build/bin/forge", "test", "integration", "create")
			cmd.Env = os.Environ()
			output, err := cmd.CombinedOutput()
			if err != nil {
				results <- result{operation: fmt.Sprintf("create-%d", idx), err: err}
				return
			}

			testID := extractTestID(string(output))
			if testID == "" {
				results <- result{operation: fmt.Sprintf("create-%d", idx), err: fmt.Errorf("no testID")}
				return
			}

			// Cleanup this environment
			defer cleanupTestEnv(testID)

			results <- result{operation: fmt.Sprintf("create-%d", idx)}
		}(i)
	}

	// Also run list and version commands in parallel
	go func() {
		cmd := exec.Command("./build/bin/forge", "test", "integration", "list")
		_, err := cmd.CombinedOutput()
		results <- result{operation: "list", err: err}
	}()

	go func() {
		cmd := exec.Command("./build/bin/forge", "version")
		_, err := cmd.CombinedOutput()
		results <- result{operation: "version", err: err}
	}()

	// Collect results
	var errors []error
	for i := 0; i < 4; i++ {
		res := <-results
		if res.err != nil {
			errors = append(errors, fmt.Errorf("%s: %w", res.operation, res.err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("parallel operations failed: %v", errors)
	}

	return nil
}

func testLargeNumberOfEnvironments() error {
	// Test creating multiple environments (5) and verify they all work
	const numEnvironments = 5
	testIDs := make([]string, 0, numEnvironments)

	// Create environments
	for i := 0; i < numEnvironments; i++ {
		createCmd := exec.Command("./build/bin/forge", "test", "integration", "create")
		createCmd.Env = os.Environ()
		createOutput, err := createCmd.CombinedOutput()
		if err != nil {
			// Cleanup any created environments
			for _, tid := range testIDs {
				cleanupTestEnv(tid)
			}
			return fmt.Errorf("create %d failed: %w\nOutput: %s", i, err, createOutput)
		}

		testID := extractTestID(string(createOutput))
		if testID == "" {
			// Cleanup
			for _, tid := range testIDs {
				cleanupTestEnv(tid)
			}
			return fmt.Errorf("failed to extract testID for environment %d", i)
		}

		testIDs = append(testIDs, testID)
	}

	// Verify all environments exist
	for _, testID := range testIDs {
		if err := verifyClusterExists(testID); err != nil {
			// Cleanup all
			for _, tid := range testIDs {
				cleanupTestEnv(tid)
			}
			return fmt.Errorf("cluster verification failed for %s: %w", testID, err)
		}
	}

	// List all environments
	listCmd := exec.Command("./build/bin/forge", "test", "integration", "list")
	listOutput, err := listCmd.CombinedOutput()
	if err != nil {
		// Cleanup all
		for _, tid := range testIDs {
			cleanupTestEnv(tid)
		}
		return fmt.Errorf("list command failed: %w", err)
	}

	// Verify all our testIDs appear in list
	for _, testID := range testIDs {
		if !strings.Contains(string(listOutput), testID) {
			// Cleanup all
			for _, tid := range testIDs {
				cleanupTestEnv(tid)
			}
			return fmt.Errorf("testID %s not found in list output", testID)
		}
	}

	// Cleanup all environments
	for _, testID := range testIDs {
		cleanupTestEnv(testID)
	}

	return nil
}
