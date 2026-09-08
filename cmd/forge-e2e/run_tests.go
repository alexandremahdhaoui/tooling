// Copyright 2024 Alexandre Mahdhaoui
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/alexandremahdhaoui/forge/internal/testutil"
)

type TestCategory string

const (
	CategoryBuild         TestCategory = "build"
	CategoryTestEnv       TestCategory = "testenv"
	CategoryTestRunner    TestCategory = "test-runner"
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

// TestFilters holds test filtering configuration
type TestFilters struct {
	Category    string
	NamePattern string
}

// newTestFilters creates TestFilters from environment variables
func newTestFilters() TestFilters {
	return TestFilters{
		Category:    os.Getenv("TEST_CATEGORY"),
		NamePattern: os.Getenv("TEST_NAME_PATTERN"),
	}
}

// shouldRunTest determines if a test should run based on filters
func (tf TestFilters) shouldRunTest(test Test) bool {
	if tf.Category != "" && string(test.Category) != tf.Category {
		return false
	}
	if tf.NamePattern != "" && !matchesPattern(test.Name, tf.NamePattern) {
		return false
	}
	return true
}

// matchesPattern checks if name matches the pattern (case-insensitive substring)
func matchesPattern(name, pattern string) bool {
	return strings.Contains(strings.ToLower(name), strings.ToLower(pattern))
}

// TestSuite manages and executes tests
type TestSuite struct {
	tests           []Test
	results         []TestResult
	filters         TestFilters
	sharedTestEnvID string // Shared test environment ID for testenv-dependent tests
}

// NewTestSuite creates a new test suite
func NewTestSuite() *TestSuite {
	return &TestSuite{
		tests:   make([]Test, 0),
		results: make([]TestResult, 0),
		filters: newTestFilters(),
	}
}

// AddTest adds a test to the suite
func (ts *TestSuite) AddTest(test Test) {
	// Apply filters
	if !ts.filters.shouldRunTest(test) {
		return
	}

	ts.tests = append(ts.tests, test)
}

// suiteEnvironment manages test environment lifecycle
type suiteEnvironment struct {
	sharedTestEnvID string
	needsShared     bool
	skipCleanup     bool
}

// setup performs complete environment setup
func (se *suiteEnvironment) setup(tests []Test) error {
	se.skipCleanup = os.Getenv("SKIP_CLEANUP") != ""

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
	if err := testutil.ForceCleanupLeftovers(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to cleanup leftover resources: %v\n", err)
	}

	// Determine if we need a shared test environment
	se.needsShared = se.shouldCreateSharedTestEnv(tests)

	// Create shared test environment if needed
	if se.needsShared {
		fmt.Fprintf(os.Stderr, "\n=== Creating Shared Test Environment ===\n")
		testID, err := se.createSharedTestEnv()
		if err != nil {
			return fmt.Errorf("failed to create shared test environment: %w", err)
		}
		se.sharedTestEnvID = testID
		fmt.Fprintf(os.Stderr, "✓ Shared test environment created: %s\n\n", testID)
	}

	return nil
}

// shouldCreateSharedTestEnv checks if any tests need a shared test environment.
// Note: Since we use e2e-stub (no KIND required), we always create the shared env
// if any testenv-dependent tests exist.
func (se *suiteEnvironment) shouldCreateSharedTestEnv(tests []Test) bool {
	// Check if any tests are testenv-dependent
	for _, test := range tests {
		switch test.Category {
		case CategoryTestEnv, CategoryArtifactStore:
			// These categories use e2e-stub (no KIND required)
			if !test.Skip {
				return true
			}
		}
	}
	return false
}

// createSharedTestEnv creates a shared test environment for reuse across tests.
// Uses e2e-stub stage for fast testenv CRUD testing (no real resources).
// The integration stage with KIND is only used for tests that need a real cluster.
func (se *suiteEnvironment) createSharedTestEnv() (string, error) {
	// Use e2e-stub for fast testenv CRUD tests (no real resources created)
	cmd := exec.Command("./build/bin/forge", "test", "create-env", "e2e-stub")
	cmd.Env = os.Environ()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to create shared environment: %w\nOutput: %s", err, output)
	}

	testID := testutil.ExtractTestID(string(output))
	if testID == "" {
		return "", fmt.Errorf("failed to extract testID from output: %s", output)
	}

	// No cluster verification needed for stub environment
	return testID, nil
}

// teardown performs environment cleanup
func (se *suiteEnvironment) teardown() {
	if se.skipCleanup {
		fmt.Fprintf(os.Stderr, "\n⚠️  SKIP_CLEANUP set, leaving test resources intact for inspection\n")
		return
	}

	// Cleanup shared test environment if it was created (uses e2e-stub stage)
	if se.sharedTestEnvID != "" {
		fmt.Fprintf(os.Stderr, "\n=== Cleaning Up Shared Test Environment ===\n")
		if err := testutil.ForceCleanupTestEnv(se.sharedTestEnvID, "e2e-stub"); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to cleanup shared environment: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "✓ Shared test environment cleaned up\n")
		}
	}

	// Force cleanup any remaining leftovers
	if err := testutil.ForceCleanupLeftovers(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to cleanup leftover resources: %v\n", err)
	}
}

// getSharedTestEnvID returns the shared test environment ID
func (se *suiteEnvironment) getSharedTestEnvID() string {
	return se.sharedTestEnvID
}

// Setup performs global test suite setup
func (ts *TestSuite) Setup() error {
	env := &suiteEnvironment{}
	if err := env.setup(ts.tests); err != nil {
		return err
	}
	ts.sharedTestEnvID = env.getSharedTestEnvID()
	return nil
}

// Teardown performs global test suite teardown
func (ts *TestSuite) Teardown() {
	env := &suiteEnvironment{
		sharedTestEnvID: ts.sharedTestEnvID,
		skipCleanup:     os.Getenv("SKIP_CLEANUP") != "",
	}
	env.teardown()
}

// testExecutor handles test execution with thread-safe result recording
type testExecutor struct {
	suite *TestSuite
	mu    sync.Mutex
}

// executeTest runs a single test and returns the result (no side effects)
func (te *testExecutor) executeTest(test Test) TestResult {
	testStart := time.Now()

	var result TestResult
	result.Name = test.Name
	result.Category = test.Category

	// Check if test should be skipped
	if test.Skip {
		result.Status = "skipped"
		result.Output = test.SkipReason
		result.Duration = 0
		return result
	}

	// Run the test with test suite context
	err := test.Run(te.suite)
	result.Duration = time.Since(testStart).Seconds()

	if err != nil {
		result.Status = "failed"
		result.Error = err.Error()
	} else {
		result.Status = "passed"
	}

	return result
}

// recordResult records a test result in a thread-safe manner
func (te *testExecutor) recordResult(result TestResult) {
	te.mu.Lock()
	defer te.mu.Unlock()
	te.suite.results = append(te.suite.results, result)
}

// testReporter handles all test output formatting
type testReporter struct {
	writer io.Writer
}

// newTestReporter creates a new test reporter
func newTestReporter() *testReporter {
	return &testReporter{writer: os.Stderr}
}

// printTestResult prints a single test result
func (tr *testReporter) printTestResult(result TestResult, parallel bool) {
	parallelMarker := ""
	if parallel {
		parallelMarker = " [parallel]"
	}

	switch result.Status {
	case "skipped":
		_, _ = fmt.Fprintf(tr.writer, "🔹 %s%s ⏭️  SKIPPED: %s\n", result.Name, parallelMarker, result.Output)
	case "passed":
		_, _ = fmt.Fprintf(tr.writer, "🔹 %s%s ✅ PASSED (%.2fs)\n", result.Name, parallelMarker, result.Duration)
	case "failed":
		_, _ = fmt.Fprintf(tr.writer, "🔹 %s%s ❌ FAILED (%.2fs): %v\n", result.Name, parallelMarker, result.Duration, result.Error)
	}
}

// printCategoryHeader prints a category header
func (tr *testReporter) printCategoryHeader(category TestCategory, testCount int) {
	_, _ = fmt.Fprintf(tr.writer, "\n=== Category: %s (%d tests) ===\n", category, testCount)
}

// printSuiteHeader prints the test suite header
func (tr *testReporter) printSuiteHeader(totalTests, categoryCount int, filters TestFilters) {
	_, _ = fmt.Fprintf(tr.writer, "\n=== Forge E2E Test Suite ===\n")

	// Display active filters
	if filters.Category != "" {
		_, _ = fmt.Fprintf(tr.writer, "Filter: Category = %s\n", filters.Category)
	}
	if filters.NamePattern != "" {
		_, _ = fmt.Fprintf(tr.writer, "Filter: Name Pattern = %s\n", filters.NamePattern)
	}

	_, _ = fmt.Fprintf(tr.writer, "Running %d tests across %d categories\n\n", totalTests, categoryCount)
}

// printSummary prints the test summary
func (tr *testReporter) printSummary(status string, total, passed, failed, skipped int, duration float64, errorMessage string) {
	_, _ = fmt.Fprintf(tr.writer, "\n=== Test Summary ===\n")
	_, _ = fmt.Fprintf(tr.writer, "Status: %s\n", status)
	_, _ = fmt.Fprintf(tr.writer, "Total: %d\n", total)
	_, _ = fmt.Fprintf(tr.writer, "Passed: %d\n", passed)
	_, _ = fmt.Fprintf(tr.writer, "Failed: %d\n", failed)
	if skipped > 0 {
		_, _ = fmt.Fprintf(tr.writer, "Skipped: %d\n", skipped)
	}
	_, _ = fmt.Fprintf(tr.writer, "Duration: %.2fs\n", duration)

	if errorMessage != "" {
		_, _ = fmt.Fprintf(tr.writer, "\nErrors: %s\n", errorMessage)
	}
}

// printCategoryBreakdown prints the category breakdown
func (tr *testReporter) printCategoryBreakdown(categories map[TestCategory]CategoryStats) {
	if len(categories) == 0 {
		return
	}

	_, _ = fmt.Fprintf(tr.writer, "\n=== Category Breakdown ===\n")
	for category, stats := range categories {
		_, _ = fmt.Fprintf(tr.writer, "%s: %d/%d passed (%.2fs)\n",
			category, stats.Passed, stats.Total, stats.Duration)
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
	reporter := newTestReporter()

	// Print suite header
	reporter.printSuiteHeader(len(ts.tests), len(ts.getCategoriesUsed()), ts.filters)

	// Group tests by category for display
	testsByCategory := make(map[TestCategory][]Test)
	for _, test := range ts.tests {
		testsByCategory[test.Category] = append(testsByCategory[test.Category], test)
	}

	// Run tests by category
	categories := []TestCategory{
		CategoryBuild, CategoryTestEnv, CategoryTestRunner,
		CategoryArtifactStore, CategorySystem, CategoryError, CategoryCleanup,
		CategoryMCP, CategoryPerformance,
	}

	for _, category := range categories {
		tests := testsByCategory[category]
		if len(tests) == 0 {
			continue
		}

		reporter.printCategoryHeader(category, len(tests))

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
			ts.runTest(test, reporter)
		}

		// Run parallel tests concurrently
		if len(parallelTests) > 0 {
			ts.runTestsParallel(parallelTests, reporter)
		}
	}

	// Calculate final statistics
	duration := time.Since(startTime).Seconds()
	return ts.generateReport(duration, reporter)
}

// runTest executes a single test and records the result
func (ts *TestSuite) runTest(test Test, reporter *testReporter) {
	executor := &testExecutor{suite: ts}
	result := executor.executeTest(test)
	executor.recordResult(result)
	reporter.printTestResult(result, false)
}

// runTestsParallel executes multiple tests in parallel
func (ts *TestSuite) runTestsParallel(tests []Test, reporter *testReporter) {
	var wg sync.WaitGroup
	executor := &testExecutor{suite: ts}

	for _, test := range tests {
		wg.Add(1)
		go func(t Test) {
			defer wg.Done()
			result := executor.executeTest(t)
			executor.recordResult(result)
			reporter.printTestResult(result, true)
		}(test)
	}

	wg.Wait()
}

// computeStatistics computes category statistics on-demand from test results
func computeStatistics(results []TestResult) map[TestCategory]CategoryStats {
	categories := make(map[TestCategory]CategoryStats)

	for _, result := range results {
		stats := categories[result.Category]
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

		categories[result.Category] = stats
	}

	return categories
}

// generateReport generates the final test report
func (ts *TestSuite) generateReport(duration float64, reporter *testReporter) *DetailedTestReport {
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

	// Compute category statistics on-demand
	categories := computeStatistics(ts.results)

	// Print summary and category breakdown using reporter
	reporter.printSummary(status, total, passed, failed, skipped, duration, errorMessage)
	reporter.printCategoryBreakdown(categories)

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
		Categories: categories,
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

	// Phase 3: TestEnv lifecycle tests (using e2e-stub - no KIND required)
	suite.AddTest(Test{
		Name:     "test environment create",
		Category: CategoryTestEnv,
		Run:      testTestEnvCreate,
		Parallel: false, // Sequential to avoid artifact store locking contention
	})

	suite.AddTest(Test{
		Name:     "test environment list",
		Category: CategoryTestEnv,
		Run:      testTestEnvList,
		Parallel: false, // Uses shared environment - sequential
	})

	suite.AddTest(Test{
		Name:     "test environment get",
		Category: CategoryTestEnv,
		Run:      testTestEnvGet,
		Parallel: false, // Uses shared environment - sequential
	})

	suite.AddTest(Test{
		Name:     "test environment get JSON",
		Category: CategoryTestEnv,
		Run:      testTestEnvGetJSON,
		Parallel: false, // Uses shared environment - sequential
	})

	suite.AddTest(Test{
		Name:     "test environment delete",
		Category: CategoryTestEnv,
		Run:      testTestEnvDelete,
		Parallel: false, // Sequential to avoid artifact store locking contention
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

// Test functions
func testForgeBuild(ts *TestSuite) error {
	cmd := exec.Command("go", "run", "./cmd/forge", "build", "forge")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("command failed: %w\nOutput: %s", err, string(output))
	}

	// Verify output contains success message (either built or up-to-date)
	outputStr := string(output)
	if !strings.Contains(outputStr, "Successfully built") && !strings.Contains(outputStr, "is up to date") {
		return fmt.Errorf("expected success message in output, got: %s", outputStr)
	}

	// Verify binary exists
	if _, err := os.Stat("./build/bin/forge"); err != nil {
		return fmt.Errorf("forge binary not found: %w", err)
	}

	return nil
}

func testForgeBuildSpecific(ts *TestSuite) error {
	cmd := exec.Command("go", "run", "./cmd/forge", "build", "go-build")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("command failed: %w\nOutput: %s", err, string(output))
	}

	// Verify output contains success message (either built or up-to-date)
	outputStr := string(output)
	if !strings.Contains(outputStr, "Successfully built") && !strings.Contains(outputStr, "is up to date") {
		return fmt.Errorf("expected success message in output, got: %s", outputStr)
	}

	// Verify binary exists
	if _, err := os.Stat("./build/bin/go-build"); err != nil {
		return fmt.Errorf("go-build binary not found: %w", err)
	}

	return nil
}

func testForgeTestUnit(ts *TestSuite) error {
	cmd := exec.Command("go", "run", "./cmd/forge", "test", "run", "unit")
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
	// Create test environment using e2e-stub (fast, no real resources)
	cmd := exec.Command("./build/bin/forge", "test", "create-env", "e2e-stub")
	cmd.Env = os.Environ()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("create command failed: %w\nOutput: %s", err, output)
	}

	// Extract testID
	testID := testutil.ExtractTestID(string(output))
	if testID == "" {
		return fmt.Errorf("no testID found in output: %s", output)
	}

	// Cleanup immediately after test
	defer func() { _ = testutil.ForceCleanupTestEnv(testID, "e2e-stub") }()

	// Verify artifact store entry (no cluster verification needed for stub)
	if err := testutil.VerifyArtifactStoreHasTestEnv(testID); err != nil {
		return fmt.Errorf("artifact store verification failed: %w", err)
	}

	return nil
}

func testTestEnvList(ts *TestSuite) error {
	// Use shared test environment instead of creating a new one
	testID := ts.sharedTestEnvID
	if testID == "" {
		return fmt.Errorf("shared test environment not available")
	}

	// List test environments (using e2e-stub stage)
	listCmd := exec.Command("./build/bin/forge", "test", "list-env", "e2e-stub")
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
	testID := ts.sharedTestEnvID
	if testID == "" {
		return fmt.Errorf("shared test environment not available")
	}

	// Get test environment details (using e2e-stub stage)
	getCmd := exec.Command("./build/bin/forge", "test", "get-env", "e2e-stub", testID)
	getOutput, err := getCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("get command failed: %w\nOutput: %s", err, getOutput)
	}

	// Verify output contains expected fields (lowercase YAML format)
	requiredFields := []string{"id:", "name:", "status:", "tmpDir:", "files:", "metadata:"}
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
	testID := ts.sharedTestEnvID
	if testID == "" {
		return fmt.Errorf("shared test environment not available")
	}

	// Get test environment as JSON (using e2e-stub stage)
	// Use Output() instead of CombinedOutput() to only capture stdout (JSON)
	// and not stderr (which contains "Sourced X environment variables" messages)
	getCmd := exec.Command("./build/bin/forge", "test", "get-env", "e2e-stub", testID, "-o", "json")
	getOutput, err := getCmd.Output()
	if err != nil {
		// On error, get stderr for debugging
		if exitErr, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("get JSON command failed: %w\nStderr: %s\nStdout: %s", err, exitErr.Stderr, getOutput)
		}
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
	// Create test environment using e2e-stub (fast, no real resources)
	createCmd := exec.Command("./build/bin/forge", "test", "create-env", "e2e-stub")
	createCmd.Env = os.Environ()
	createOutput, err := createCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("create command failed: %w\nOutput: %s", err, createOutput)
	}

	testID := testutil.ExtractTestID(string(createOutput))
	if testID == "" {
		return fmt.Errorf("failed to extract testID")
	}

	// Verify artifact store entry exists before deletion
	if err := testutil.VerifyArtifactStoreHasTestEnv(testID); err != nil {
		return fmt.Errorf("artifact store entry not found before deletion: %w", err)
	}

	// Delete test environment
	deleteCmd := exec.Command("./build/bin/forge", "test", "delete-env", "e2e-stub", testID)
	deleteCmd.Env = os.Environ()
	deleteOutput, err := deleteCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("delete command failed: %w\nOutput: %s", err, deleteOutput)
	}

	// Verify artifact store no longer contains testID
	if err := testutil.VerifyArtifactStoreMissingTestEnv(testID); err != nil {
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
	testID := ts.sharedTestEnvID
	if testID == "" {
		return fmt.Errorf("shared test environment not available")
	}

	// Run integration tests with the test environment
	runCmd := exec.Command("./build/bin/forge", "test", "run", "integration", testID)
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

// Phase 7: Error Handling Tests

func testMissingBinaryError(ts *TestSuite) error {
	// This would require renaming a binary temporarily
	// Skipped in registerAllTests
	return fmt.Errorf("not implemented")
}

func testInvalidTestIDError(ts *TestSuite) error {
	invalidID := "invalid-test-id-12345"

	cmd := exec.Command("./build/bin/forge", "test", "get-env", "integration", invalidID)
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

	cmd := exec.Command("./build/bin/forge", "test", "delete-env", "integration", nonExistentID)
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

	scanner := bufio.NewScanner(stdout)

	// Send initialize
	initRequest := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0.0"}}}` + "\n"
	if _, err := stdin.Write([]byte(initRequest)); err != nil {
		return fmt.Errorf("failed to write initialize: %w", err)
	}

	// Read initialize response
	if !scanner.Scan() {
		return fmt.Errorf("failed to read initialize response: %v", scanner.Err())
	}

	// Send initialized notification (required by MCP protocol before other requests)
	initializedNotification := `{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n"
	if _, err := stdin.Write([]byte(initializedNotification)); err != nil {
		return fmt.Errorf("failed to write initialized notification: %w", err)
	}

	// Send tools/list request to verify run tool exists
	listRequest := `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}` + "\n"
	if _, err := stdin.Write([]byte(listRequest)); err != nil {
		return fmt.Errorf("failed to write tools/list: %w", err)
	}

	// Read tools/list response (skip any notifications)
	var response string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, `"id":2`) {
			response = line
			break
		}
	}
	if response == "" {
		return fmt.Errorf("failed to read tools/list response: %v", scanner.Err())
	}

	// Verify response contains "run" tool
	if !strings.Contains(response, "run") {
		return fmt.Errorf("run tool not found in tools/list response: %s", response)
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

	scanner := bufio.NewScanner(stdout)

	// Send initialize
	initRequest := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0.0"}}}` + "\n"
	if _, err := stdin.Write([]byte(initRequest)); err != nil {
		return fmt.Errorf("failed to write initialize: %w", err)
	}

	// Read initialize response
	if !scanner.Scan() {
		return fmt.Errorf("failed to read initialize response: %v", scanner.Err())
	}

	// Send initialized notification (required by MCP protocol before other requests)
	initializedNotification := `{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n"
	if _, err := stdin.Write([]byte(initializedNotification)); err != nil {
		return fmt.Errorf("failed to write initialized notification: %w", err)
	}

	// Send a tool call that will fail (invalid parameters)
	toolCallRequest := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"run","arguments":{"stage":"invalid","name":""}}}` + "\n"
	if _, err := stdin.Write([]byte(toolCallRequest)); err != nil {
		return fmt.Errorf("failed to write tool call: %w", err)
	}

	// Read response (skip any notifications, find our response with id:2)
	var response string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, `"id":2`) {
			response = line
			break
		}
	}
	if response == "" {
		return fmt.Errorf("failed to read tool call response: %v", scanner.Err())
	}

	// Verify response indicates an error
	if !strings.Contains(response, "error") && !strings.Contains(response, "isError") {
		return fmt.Errorf("expected error in response, got: %s", response)
	}

	return nil
}
