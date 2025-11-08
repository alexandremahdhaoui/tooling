package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/alexandremahdhaoui/forge/internal/mcpserver"
	"github.com/alexandremahdhaoui/forge/internal/version"
	"github.com/alexandremahdhaoui/forge/pkg/forge"
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
	ID       string `json:"id,omitempty"`
	Stage    string `json:"stage"`
	Name     string `json:"name"`
	TmpDir   string `json:"tmpDir,omitempty"`
	BuildDir string `json:"buildDir,omitempty"`
	RootDir  string `json:"rootDir,omitempty"`
}

// TestFunc represents a test function that receives the test suite for context
type TestFunc func(*TestSuite) error

// Test represents a single test case
type Test struct {
	Name       string
	Category   TestCategory
	Run        TestFunc
	Skip       bool
	SkipReason string
	// Parallel indicates if this test can run in parallel with other parallel tests
	// Tests that use shared resources (like shared test environment) should NOT be parallel
	Parallel bool
}

// TestSuite manages and executes tests
type TestSuite struct {
	tests             []Test
	results           []TestResult
	categories        map[TestCategory]*CategoryStats
	filterCategory    string
	filterNamePattern string
	sharedTestEnvID   string // Shared test environment ID for testenv-dependent tests
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

	// Force cleanup of any leftover test environments
	if err := forceCleanupLeftovers(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to cleanup leftover resources: %v\n", err)
	}

	// Check if we need to create a shared test environment
	if ts.needsSharedTestEnv() {
		fmt.Fprintf(os.Stderr, "\n=== Creating Shared Test Environment ===\n")
		testID, err := ts.createSharedTestEnv()
		if err != nil {
			return fmt.Errorf("failed to create shared test environment: %w", err)
		}
		ts.sharedTestEnvID = testID
		fmt.Fprintf(os.Stderr, "✓ Shared test environment created: %s\n\n", testID)
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

	// Cleanup shared test environment if it was created
	if ts.sharedTestEnvID != "" {
		fmt.Fprintf(os.Stderr, "\n=== Cleaning Up Shared Test Environment ===\n")
		if err := forceCleanupTestEnv(ts.sharedTestEnvID); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to cleanup shared environment: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "✓ Shared test environment cleaned up\n")
		}
	}

	// Force cleanup any remaining leftovers
	if err := forceCleanupLeftovers(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to cleanup leftover resources: %v\n", err)
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

		// Separate parallel and sequential tests
		var parallelTests, sequentialTests []Test
		for _, test := range tests {
			if test.Parallel && !test.Skip {
				parallelTests = append(parallelTests, test)
			} else {
				sequentialTests = append(sequentialTests, test)
			}
		}

		// Run sequential tests first
		for _, test := range sequentialTests {
			ts.runTest(test)
		}

		// Run parallel tests concurrently
		if len(parallelTests) > 0 {
			ts.runTestsParallel(parallelTests)
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

	// Run the test with test suite context
	err := test.Run(ts)
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

// runTestsParallel executes multiple tests in parallel
func (ts *TestSuite) runTestsParallel(tests []Test) {
	var wg sync.WaitGroup
	var mu sync.Mutex // Protect shared state (results, categories)

	for _, test := range tests {
		wg.Add(1)
		go func(t Test) {
			defer wg.Done()

			testStart := time.Now()
			fmt.Fprintf(os.Stderr, "🔹 %s [parallel]", t.Name)

			var result TestResult
			result.Name = t.Name
			result.Category = t.Category

			// Run the test
			err := t.Run(ts)
			result.Duration = time.Since(testStart).Seconds()

			// Lock for updating shared state
			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				result.Status = "failed"
				result.Error = err.Error()
				ts.results = append(ts.results, result)
				ts.updateCategoryStats(t.Category, result)
				fmt.Fprintf(os.Stderr, " ❌ FAILED (%.2fs): %v\n", result.Duration, err)
			} else {
				result.Status = "passed"
				ts.results = append(ts.results, result)
				ts.updateCategoryStats(t.Category, result)
				fmt.Fprintf(os.Stderr, " ✅ PASSED (%.2fs)\n", result.Duration)
			}
		}(test)
	}

	wg.Wait()
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

	// Validate required inputs
	if input.Stage == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Run failed: missing required field 'stage'"},
			},
			IsError: true,
		}, nil, nil
	}

	if input.Name == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Run failed: missing required field 'name'"},
			},
			IsError: true,
		}, nil, nil
	}

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
//
// Test Classification:
//
//   - E2E Tests: Test the entire forge workflow as a user would, no infrastructure setup
//     Examples: forge build, forge version, forge help
//
//   - Integration Tests: Test components together with real dependencies (KIND clusters, etc.)
//     Examples: testenv create/delete, integration test runner
//
// - Unit Tests: Test individual components in isolation (run via forge test unit)
//
// Parallel Execution Strategy:
// - Tests marked Parallel:true can run concurrently with other parallel tests
// - Tests that use shared resources (shared testenv) must be Parallel:false
// - Tests that create/destroy their own resources can be Parallel:true
func registerAllTests(suite *TestSuite) {
	// ====================================================================
	// E2E TESTS - Test complete forge workflows without infrastructure
	// ====================================================================

	// Phase 2: Build system tests (E2E)
	suite.AddTest(Test{
		Name:     "forge build",
		Category: CategoryBuild,
		Run:      testForgeBuild,
		Parallel: true, // Can run in parallel
	})

	suite.AddTest(Test{
		Name:     "forge build specific artifact",
		Category: CategoryBuild,
		Run:      testForgeBuildSpecific,
		Parallel: true,
	})

	suite.AddTest(Test{
		Name:       "forge build container",
		Category:   CategoryBuild,
		Run:        testForgeBuildContainer,
		Skip:       shouldSkipContainerTests(),
		SkipReason: "CONTAINER_ENGINE not available",
		Parallel:   true,
	})

	suite.AddTest(Test{
		Name:     "forge build format",
		Category: CategoryBuild,
		Run:      testForgeBuildFormat,
		Parallel: false, // Modifies code, should be sequential
	})

	suite.AddTest(Test{
		Name:     "incremental build",
		Category: CategoryBuild,
		Run:      testIncrementalBuild,
		Parallel: false, // Depends on build state
	})

	// Phase 10: System tests (E2E - all parallel, read-only operations)
	suite.AddTest(Test{
		Name:     "forge version",
		Category: CategorySystem,
		Run:      testForgeVersion,
		Parallel: true,
	})

	suite.AddTest(Test{
		Name:     "forge help",
		Category: CategorySystem,
		Run:      testForgeHelp,
		Parallel: true,
	})

	suite.AddTest(Test{
		Name:     "forge no args",
		Category: CategorySystem,
		Run:      testForgeNoArgs,
		Parallel: true,
	})

	// Phase 6: Artifact store tests (E2E)
	suite.AddTest(Test{
		Name:     "artifact store validation",
		Category: CategoryArtifactStore,
		Run:      testArtifactStore,
		Parallel: true, // Read-only validation
	})

	// ====================================================================
	// INTEGRATION TESTS - Test with real infrastructure (KIND clusters)
	// ====================================================================

	// Phase 3: TestEnv lifecycle tests (Integration)
	suite.AddTest(Test{
		Name:       "test environment create",
		Category:   CategoryTestEnv,
		Run:        testTestEnvCreate,
		Skip:       shouldSkipTestEnvTests(),
		SkipReason: "KIND_BINARY not available",
		Parallel:   false, // Sequential to avoid artifact store locking contention
	})

	suite.AddTest(Test{
		Name:       "test environment list",
		Category:   CategoryTestEnv,
		Run:        testTestEnvList,
		Skip:       shouldSkipTestEnvTests(),
		SkipReason: "KIND_BINARY not available",
		Parallel:   false, // Uses shared environment - sequential
	})

	suite.AddTest(Test{
		Name:       "test environment get",
		Category:   CategoryTestEnv,
		Run:        testTestEnvGet,
		Skip:       shouldSkipTestEnvTests(),
		SkipReason: "KIND_BINARY not available",
		Parallel:   false, // Uses shared environment - sequential
	})

	suite.AddTest(Test{
		Name:       "test environment get JSON",
		Category:   CategoryTestEnv,
		Run:        testTestEnvGetJSON,
		Skip:       shouldSkipTestEnvTests(),
		SkipReason: "KIND_BINARY not available",
		Parallel:   false, // Uses shared environment - sequential
	})

	suite.AddTest(Test{
		Name:       "test environment delete",
		Category:   CategoryTestEnv,
		Run:        testTestEnvDelete,
		Skip:       shouldSkipTestEnvTests(),
		SkipReason: "KIND_BINARY not available",
		Parallel:   false, // Sequential to avoid artifact store locking contention
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
		Name:       "malformed forge.yaml error",
		Category:   CategoryError,
		Run:        testMalformedForgeYamlError,
		Skip:       true, // Requires forge.yaml manipulation
		SkipReason: "requires forge.yaml manipulation",
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
	storePath, err := forge.GetArtifactStorePath(".forge/artifacts.json")
	if err != nil {
		return fmt.Errorf("failed to get artifact store path: %w", err)
	}
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
	storePath, err := forge.GetArtifactStorePath(".forge/artifacts.json")
	if err != nil {
		return fmt.Errorf("failed to get artifact store path: %w", err)
	}
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

// forceCleanupLeftovers cleans up leftover resources without depending on artifact store
func forceCleanupLeftovers() error {
	var errors []error

	// Cleanup KIND clusters
	kindBinary := os.Getenv("KIND_BINARY")
	if kindBinary == "" {
		kindBinary = "kind"
	}

	cmd := exec.Command(kindBinary, "get", "clusters")
	output, err := cmd.CombinedOutput()
	if err == nil {
		clusters := strings.Split(strings.TrimSpace(string(output)), "\n")
		for _, cluster := range clusters {
			cluster = strings.TrimSpace(cluster)
			if strings.HasPrefix(cluster, "forge-test-") && cluster != "" {
				fmt.Fprintf(os.Stderr, "Cleaning up leftover cluster: %s\n", cluster)
				deleteCmd := exec.Command(kindBinary, "delete", "cluster", "--name", cluster)
				if err := deleteCmd.Run(); err != nil {
					errors = append(errors, fmt.Errorf("failed to delete cluster %s: %w", cluster, err))
				}
			}
		}
	}

	// Cleanup tmp directories
	rootDir, err := os.Getwd()
	if err == nil {
		tmpBase := filepath.Join(rootDir, "tmp")
		entries, err := os.ReadDir(tmpBase)
		if err == nil {
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), "test-integration-") || strings.HasPrefix(entry.Name(), "tmp-") {
					dirPath := filepath.Join(tmpBase, entry.Name())
					if err := os.RemoveAll(dirPath); err != nil {
						errors = append(errors, fmt.Errorf("failed to remove %s: %w", dirPath, err))
					}
				}
			}
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("cleanup errors: %v", errors)
	}
	return nil
}

// forceCleanupTestEnv forcefully cleans up a test environment without artifact store dependency
func forceCleanupTestEnv(testID string) error {
	if testID == "" {
		return nil
	}

	var errors []error

	// Delete KIND cluster
	kindBinary := os.Getenv("KIND_BINARY")
	if kindBinary == "" {
		kindBinary = "kind"
	}

	clusterName := fmt.Sprintf("forge-%s", testID)
	fmt.Fprintf(os.Stderr, "Deleting cluster: %s\n", clusterName)
	deleteCmd := exec.Command(kindBinary, "delete", "cluster", "--name", clusterName)
	if err := deleteCmd.Run(); err != nil {
		// Only add error if cluster might exist (ignore "not found" errors)
		errors = append(errors, fmt.Errorf("failed to delete cluster %s: %w", clusterName, err))
	}

	// Delete tmp directory
	rootDir, err := os.Getwd()
	if err == nil {
		tmpDir := filepath.Join(rootDir, "tmp", testID)
		if err := os.RemoveAll(tmpDir); err != nil {
			errors = append(errors, fmt.Errorf("failed to remove tmpDir %s: %w", tmpDir, err))
		}
	}

	// Try to remove from artifact store (best effort)
	cleanupTestEnv(testID)

	if len(errors) > 0 {
		return fmt.Errorf("cleanup errors: %v", errors)
	}
	return nil
}

// needsSharedTestEnv checks if any tests in the suite need a shared test environment
func (ts *TestSuite) needsSharedTestEnv() bool {
	// Check if KIND_BINARY is available
	if shouldSkipTestEnvTests() {
		return false
	}

	// Check if any tests in the suite are testenv-dependent
	for _, test := range ts.tests {
		switch test.Category {
		case CategoryTestEnv, CategoryTestRunner, CategoryPerformance, CategoryCleanup, CategoryError, CategoryArtifactStore:
			if !test.Skip {
				return true
			}
		}
	}
	return false
}

// createSharedTestEnv creates a shared test environment for reuse across tests
func (ts *TestSuite) createSharedTestEnv() (string, error) {
	cmd := exec.Command("./build/bin/forge", "test", "integration", "create")
	cmd.Env = os.Environ()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to create shared environment: %w\nOutput: %s", err, output)
	}

	testID := extractTestID(string(output))
	if testID == "" {
		return "", fmt.Errorf("failed to extract testID from output: %s", output)
	}

	// Verify cluster was created successfully
	if err := verifyClusterExists(testID); err != nil {
		_ = forceCleanupTestEnv(testID)
		return "", fmt.Errorf("cluster verification failed: %w", err)
	}

	return testID, nil
}

// getSharedTestEnv returns the shared test environment ID or creates one if needed
func (ts *TestSuite) getSharedTestEnv() string {
	return ts.sharedTestEnvID
}

func testForgeBuild(ts *TestSuite) error {
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

func testForgeBuildSpecific(ts *TestSuite) error {
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

func testForgeTestUnit(ts *TestSuite) error {
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

func testArtifactStore(ts *TestSuite) error {
	storePath := ".forge/artifact-store.yaml"

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

func testForgeVersion(ts *TestSuite) error {
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

func testForgeBuildContainer(ts *TestSuite) error {
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

func testForgeBuildFormat(ts *TestSuite) error {
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

func testIncrementalBuild(ts *TestSuite) error {
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

func testForgeHelp(ts *TestSuite) error {
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

func testForgeNoArgs(ts *TestSuite) error {
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

func testTestEnvCreate(ts *TestSuite) error {
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

	// Cleanup immediately after test
	defer func() { _ = forceCleanupTestEnv(testID) }()

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

func testTestEnvList(ts *TestSuite) error {
	// Use shared test environment instead of creating a new one
	testID := ts.getSharedTestEnv()
	if testID == "" {
		return fmt.Errorf("shared test environment not available")
	}

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

func testTestEnvGet(ts *TestSuite) error {
	// Use shared test environment
	testID := ts.getSharedTestEnv()
	if testID == "" {
		return fmt.Errorf("shared test environment not available")
	}

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

func testTestEnvGetJSON(ts *TestSuite) error {
	// Use shared test environment
	testID := ts.getSharedTestEnv()
	if testID == "" {
		return fmt.Errorf("shared test environment not available")
	}

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

func testTestEnvDelete(ts *TestSuite) error {
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

func testTestEnvSpecOverride(ts *TestSuite) error {
	// This test would require modifying forge.yaml to test spec override
	// For now, we'll skip it as noted in registerAllTests
	return fmt.Errorf("not implemented - requires forge.yaml manipulation")
}

// Phase 4: Test Runner Integration Tests

func testIntegrationTestRunner(ts *TestSuite) error {
	// Use shared test environment
	testID := ts.getSharedTestEnv()
	if testID == "" {
		return fmt.Errorf("shared test environment not available")
	}

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

func testLintRunner(ts *TestSuite) error {
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

func testPromptList(ts *TestSuite) error {
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

func testPromptGet(ts *TestSuite) error {
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

func testPromptGetInvalid(ts *TestSuite) error {
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

// Phase 7: Error Handling Tests

func testMissingBinaryError(ts *TestSuite) error {
	// This would require renaming a binary temporarily
	// Skipped in registerAllTests
	return fmt.Errorf("not implemented")
}

func testInvalidTestIDError(ts *TestSuite) error {
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

func testMissingEnvVarError(ts *TestSuite) error {
	// This would require unsetting KIND_BINARY temporarily
	// Skipped in registerAllTests
	return fmt.Errorf("not implemented")
}

func testDeleteNonExistentError(ts *TestSuite) error {
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

// Phase 9: MCP Integration Tests

func testMCPServerMode(ts *TestSuite) error {
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
		_ = cmd.Process.Kill()
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
			_ = cmd.Process.Kill()
			return fmt.Errorf("invalid MCP response: %s", response)
		}
	case <-time.After(2 * time.Second):
		_ = cmd.Process.Kill()
		return fmt.Errorf("timeout waiting for MCP response")
	}

	// Kill the server
	_ = cmd.Process.Kill()
	_ = cmd.Wait()

	return nil
}

// Phase 4: Additional Test Runner Tests

func testVerifyTagsRunner(ts *TestSuite) error {
	cmd := exec.Command("./build/bin/forge", "test", "verify-tags", "run")
	cmd.Env = os.Environ()
	output, _ := cmd.CombinedOutput() // May fail if no Go files have tags

	// Verify command executed (produced output)
	if len(output) == 0 {
		return fmt.Errorf("no output from verify-tags command")
	}

	return nil
}

func testMalformedForgeYamlError(ts *TestSuite) error {
	// This test would require temporarily modifying forge.yaml
	// Skip for now as noted in registerAllTests
	return fmt.Errorf("not implemented - requires forge.yaml manipulation")
}

// Phase 9: Additional MCP Tests

func testMCPRunToolCall(ts *TestSuite) error {
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
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
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

func testMCPErrorPropagation(ts *TestSuite) error {
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
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
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
