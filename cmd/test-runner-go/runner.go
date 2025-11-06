package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/alexandremahdhaoui/forge/pkg/forge"
	"github.com/google/uuid"
)

// run executes tests for the given stage and generates a structured report.
// Test output goes to stderr, JSON report goes to stdout.
func run(stage, name string) error {
	// Use current directory as tmpDir for backward compatibility
	tmpDir := "."

	// Execute tests and generate report (no testenv for CLI mode)
	report, junitFile, coverageFile, err := runTests(stage, name, tmpDir, nil)
	if err != nil {
		return fmt.Errorf("test execution failed: %w", err)
	}

	// Store report in artifact store
	if err := storeTestReport(report, junitFile, coverageFile); err != nil {
		// Log error but don't fail - storing is best effort
		fmt.Fprintf(os.Stderr, "Warning: failed to store test report in artifact store: %v\n", err)
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

// runTests executes the test suite using gotestsum and returns a structured report along with artifact file paths.
// testEnv contains environment variables to pass to the test process (e.g., artifact file paths, metadata).
func runTests(stage, name, tmpDir string, testEnv map[string]string) (*TestReport, string, string, error) {
	startTime := time.Now()

	// Generate output file paths in tmpDir
	junitFile := filepath.Join(tmpDir, fmt.Sprintf("test-%s-%s.xml", stage, name))
	coverageFile := filepath.Join(tmpDir, fmt.Sprintf("test-%s-%s-coverage.out", stage, name))

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
		"-cover",
		"-coverprofile", coverageFile,
		"./...",
	}

	cmd := exec.Command("go", args...)

	// Inherit current environment and add testenv variables
	cmd.Env = os.Environ()
	if testEnv != nil {
		for key, value := range testEnv {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
		}
	}

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

	return report, junitFile, coverageFile, nil
}

// storeTestReport stores the test report in the artifact store.
func storeTestReport(report *TestReport, junitFile, coverageFile string) error {
	// Get artifact store path (environment variable takes precedence)
	artifactStorePath := os.Getenv("FORGE_ARTIFACT_STORE_PATH")
	if artifactStorePath == "" {
		var err error
		artifactStorePath, err = forge.GetArtifactStorePath(".forge/artifacts.yaml")
		if err != nil {
			return fmt.Errorf("failed to get artifact store path: %w", err)
		}
	}

	// Read or create artifact store
	store, err := forge.ReadOrCreateArtifactStore(artifactStorePath)
	if err != nil {
		return fmt.Errorf("failed to read artifact store: %w", err)
	}

	// Generate report ID (UUID)
	reportID := uuid.New().String()

	// Build list of artifact files
	var artifactFiles []string
	if junitFile != "" {
		artifactFiles = append(artifactFiles, junitFile)
	}
	if coverageFile != "" {
		artifactFiles = append(artifactFiles, coverageFile)
	}

	// Create TestReport for artifact store
	storeReport := &forge.TestReport{
		ID:            reportID,
		Stage:         report.Stage,
		Status:        report.Status,
		StartTime:     report.StartTime,
		Duration:      report.Duration,
		TestStats:     forge.TestStats(report.TestStats),
		Coverage:      forge.Coverage(report.Coverage),
		ArtifactFiles: artifactFiles,
		OutputPath:    report.OutputPath,
		ErrorMessage:  report.ErrorMessage,
	}

	// Add or update test report
	forge.AddOrUpdateTestReport(&store, storeReport)

	// Write artifact store
	if err := forge.WriteArtifactStore(artifactStorePath, store); err != nil {
		return fmt.Errorf("failed to write artifact store: %w", err)
	}

	return nil
}
