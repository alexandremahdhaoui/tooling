package main

import (
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// TestReport represents the structured output of a test run.
type TestReport struct {
	// Stage is the test stage name (e.g., "unit", "integration")
	Stage string `json:"stage"`

	// Name is the test run identifier
	Name string `json:"name"`

	// Status is the overall test result ("passed" or "failed")
	Status string `json:"status"`

	// StartTime is when the test run started
	StartTime time.Time `json:"startTime"`

	// Duration is the total test duration in seconds
	Duration float64 `json:"duration"`

	// TestStats contains test execution statistics
	TestStats TestStats `json:"testStats"`

	// Coverage contains code coverage information
	Coverage Coverage `json:"coverage"`

	// OutputPath is the path to detailed test output files
	OutputPath string `json:"outputPath,omitempty"`

	// ErrorMessage contains error details if the test run failed
	ErrorMessage string `json:"errorMessage,omitempty"`
}

// TestStats contains statistics about test execution.
type TestStats struct {
	// Total is the total number of tests
	Total int `json:"total"`

	// Passed is the number of tests that passed
	Passed int `json:"passed"`

	// Failed is the number of tests that failed
	Failed int `json:"failed"`

	// Skipped is the number of tests that were skipped
	Skipped int `json:"skipped"`
}

// Coverage contains code coverage information.
type Coverage struct {
	// Percentage is the code coverage percentage (0-100)
	Percentage float64 `json:"percentage"`

	// FilePath is the path to the coverage file
	FilePath string `json:"filePath,omitempty"`
}

// JUnit XML structures for parsing test results
type junitTestSuites struct {
	TestSuites []junitTestSuite `xml:"testsuite"`
}

type junitTestSuite struct {
	Tests    int             `xml:"tests,attr"`
	Failures int             `xml:"failures,attr"`
	Skipped  int             `xml:"skipped,attr"`
	TestCase []junitTestCase `xml:"testcase"`
}

type junitTestCase struct {
	Name    string          `xml:"name,attr"`
	Failure *junitFailure   `xml:"failure,omitempty"`
	Skipped *junitSkipped   `xml:"skipped,omitempty"`
}

type junitFailure struct {
	Message string `xml:"message,attr"`
}

type junitSkipped struct{}

// parseJUnitXML parses JUnit XML output and extracts test statistics.
func parseJUnitXML(xmlPath string) (*TestStats, error) {
	// Read XML file
	data, err := os.ReadFile(xmlPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read JUnit XML file: %w", err)
	}

	// Parse XML
	var suites junitTestSuites
	if err := xml.Unmarshal(data, &suites); err != nil {
		return nil, fmt.Errorf("failed to parse JUnit XML: %w", err)
	}

	// Aggregate statistics from all test suites
	stats := &TestStats{}
	for _, suite := range suites.TestSuites {
		stats.Total += suite.Tests
		stats.Failed += suite.Failures
		stats.Skipped += suite.Skipped
	}
	stats.Passed = stats.Total - stats.Failed - stats.Skipped

	return stats, nil
}

// parseCoverage parses coverage file and extracts coverage percentage.
func parseCoverage(coveragePath string) (*Coverage, error) {
	// Check if coverage file exists
	if _, err := os.Stat(coveragePath); err != nil {
		return nil, fmt.Errorf("coverage file not found: %w", err)
	}

	// Parse coverage using go tool cover
	cmd := exec.Command("go", "tool", "cover", "-func", coveragePath)
	output, err := cmd.Output()
	if err != nil {
		return &Coverage{FilePath: coveragePath}, fmt.Errorf("failed to parse coverage: %w", err)
	}

	// Extract total coverage percentage from last line
	// Format: "total:                          (statements)    XX.X%"
	lines := strings.Split(string(output), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "total:") {
			// Extract percentage
			parts := strings.Fields(line)
			if len(parts) > 0 {
				percentStr := parts[len(parts)-1]
				percentStr = strings.TrimSuffix(percentStr, "%")
				var percentage float64
				if _, err := fmt.Sscanf(percentStr, "%f", &percentage); err == nil {
					return &Coverage{
						Percentage: percentage,
						FilePath:   coveragePath,
					}, nil
				}
			}
		}
	}

	// If we couldn't parse the percentage, return 0
	return &Coverage{FilePath: coveragePath}, nil
}
