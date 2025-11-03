package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"
)

// run executes tests for the given stage and generates a structured report.
// Test output goes to stderr, JSON report goes to stdout.
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

// runTests executes the test suite using gotestsum and returns a structured report.
func runTests(stage, name string) (*TestReport, error) {
	startTime := time.Now()

	// Generate output file paths
	junitFile := fmt.Sprintf(".ignore.test-%s-%s.xml", stage, name)
	coverageFile := fmt.Sprintf(".ignore.test-%s-%s-coverage.out", stage, name)

	// Build gotestsum command
	args := []string{
		"run", "gotest.tools/gotestsum@v1.13.0",
		"--format", "pkgname-and-test-fails",
		"--format-hide-empty-pkg",
		"--junitfile", junitFile,
		"--",
		"-tags", stage,
		"-race",
		"-count=1",
		"-short",
		"-cover",
		"-coverprofile", coverageFile,
		"./...",
	}

	cmd := exec.Command("go", args...)

	// Redirect test output to stderr so JSON report can go to stdout
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	// Execute the command
	err := cmd.Run()
	duration := time.Since(startTime).Seconds()

	// Determine status based on exit code
	status := "passed"
	errorMessage := ""
	if err != nil {
		status = "failed"
		if exitErr, ok := err.(*exec.ExitError); ok {
			errorMessage = fmt.Sprintf("tests failed with exit code %d", exitErr.ExitCode())
		} else {
			errorMessage = fmt.Sprintf("failed to execute tests: %v", err)
		}
	}

	// Parse test statistics from JUnit XML (will be implemented in Task 2.3)
	testStats, statsErr := parseJUnitXML(junitFile)
	if statsErr != nil {
		// If we can't parse stats, create empty stats but don't fail
		testStats = &TestStats{}
	}

	// Parse coverage information (will be implemented in Task 2.3)
	coverage, coverageErr := parseCoverage(coverageFile)
	if coverageErr != nil {
		// If we can't parse coverage, create empty coverage but don't fail
		coverage = &Coverage{FilePath: coverageFile}
	}

	// Create test report
	report := &TestReport{
		Stage:        stage,
		Name:         name,
		Status:       status,
		StartTime:    startTime,
		Duration:     duration,
		TestStats:    *testStats,
		Coverage:     *coverage,
		OutputPath:   junitFile,
		ErrorMessage: errorMessage,
	}

	return report, nil
}
