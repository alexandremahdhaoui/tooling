package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/alexandremahdhaoui/forge/internal/version"
)

// Version information (set via ldflags during build)
var (
	Version        = "dev"
	CommitSHA      = "unknown"
	BuildTimestamp = "unknown"
)

var versionInfo *version.Info

func init() {
	versionInfo = version.New("generic-test-runner")
	versionInfo.Version = Version
	versionInfo.CommitSHA = CommitSHA
	versionInfo.BuildTimestamp = BuildTimestamp
}

// ExecuteInput contains the parameters for command execution
type ExecuteInput struct {
	Command string            // Command to execute
	Args    []string          // Command arguments
	Env     map[string]string // Environment variables
	EnvFile string            // Path to environment file (optional)
	WorkDir string            // Working directory (optional)
}

// ExecuteOutput contains the result of command execution
type ExecuteOutput struct {
	ExitCode int    // Command exit code
	Stdout   string // Standard output
	Stderr   string // Standard error
	Error    string // Error message if execution failed
}

// TestInput contains the parameters for test execution
type TestInput struct {
	Stage    string            // Test stage name
	Name     string            // Test name
	Command  string            // Command to execute
	Args     []string          // Command arguments
	Env      map[string]string // Environment variables
	EnvFile  string            // Path to environment file
	WorkDir  string            // Working directory
	TmpDir   string            // Temporary directory for artifacts
	BuildDir string            // Build directory
	RootDir  string            // Repository root directory
}

// TestReport represents the structured output of a test run
type TestReport struct {
	Stage     string    `json:"stage"`
	Name      string    `json:"name"`
	Status    string    `json:"status"` // "passed" or "failed"
	Timestamp string    `json:"timestamp"`
	TestStats TestStats `json:"testStats"`
	Coverage  Coverage  `json:"coverage"`
}

// TestStats contains test execution statistics
type TestStats struct {
	Total   int `json:"total"`
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
}

// Coverage contains test coverage information
type Coverage struct {
	Percentage float64 `json:"percentage"`
}

// loadEnvFile loads environment variables from a file
func loadEnvFile(path string) (map[string]string, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return make(map[string]string), nil
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read env file: %w", err)
	}

	envVars := make(map[string]string)
	lines := strings.Split(string(content), "\n")

	for lineNum, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		line = strings.TrimPrefix(line, "export ")
		line = strings.TrimSpace(line)

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid format in env file at line %d: %s", lineNum+1, line)
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		if len(value) >= 2 {
			if (strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"")) ||
				(strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'")) {
				value = value[1 : len(value)-1]
			}
		}

		envVars[key] = value
	}

	return envVars, nil
}

// executeCommand executes a shell command with the given parameters
func executeCommand(input ExecuteInput) ExecuteOutput {
	cmd := exec.Command(input.Command, input.Args...)

	if input.WorkDir != "" {
		cmd.Dir = input.WorkDir
	}

	env := os.Environ()

	if input.EnvFile != "" {
		envFileVars, err := loadEnvFile(input.EnvFile)
		if err != nil {
			return ExecuteOutput{
				ExitCode: -1,
				Error:    fmt.Sprintf("failed to load env file: %v", err),
			}
		}
		for key, value := range envFileVars {
			env = append(env, fmt.Sprintf("%s=%s", key, value))
		}
	}

	for key, value := range input.Env {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}

	cmd.Env = env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	output := ExecuteOutput{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			output.ExitCode = exitErr.ExitCode()
		} else {
			output.ExitCode = -1
			output.Error = err.Error()
		}
	} else {
		output.ExitCode = 0
	}

	return output
}

// runTests executes test commands and generates a TestReport
func runTests(input TestInput) (*TestReport, error) {
	execInput := ExecuteInput{
		Command: input.Command,
		Args:    input.Args,
		Env:     input.Env,
		EnvFile: input.EnvFile,
		WorkDir: input.WorkDir,
	}

	output := executeCommand(execInput)

	// Create test report based on exit code
	status := "passed"
	passed := 1
	failed := 0

	if output.ExitCode != 0 {
		status = "failed"
		passed = 0
		failed = 1
	}

	report := &TestReport{
		Stage:     input.Stage,
		Name:      input.Name,
		Status:    status,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		TestStats: TestStats{
			Total:   1,
			Passed:  passed,
			Failed:  failed,
			Skipped: 0,
		},
		Coverage: Coverage{
			Percentage: 0.0, // Generic test runner doesn't parse coverage
		},
	}

	return report, nil
}

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--mcp":
			if err := runMCPServer(); err != nil {
				log.Printf("MCP server error: %v", err)
				os.Exit(1)
			}
			return
		case "version", "--version", "-v":
			versionInfo.Print()
			return
		case "help", "--help", "-h":
			printUsage()
			return
		}
	}

	printUsage()
}

func printUsage() {
	fmt.Print(`generic-test-runner - Execute arbitrary test commands as a test runner

Usage:
  generic-test-runner --mcp      Run as MCP server
  generic-test-runner version    Show version information
  generic-test-runner help       Show this help message

Description:
  generic-test-runner is a generic test command executor that can be used as a test
  runner in Forge. It wraps shell commands and provides MCP server functionality for
  integration with the Forge test system.

  When running as an MCP server (--mcp), it exposes a "run" tool that accepts
  command, args, environment variables, and working directory configuration.

Example (via MCP):
  The generic-test-runner is typically invoked via Forge using engine aliases:

  engines:
    - alias: my-linter
      engine: go://generic-test-runner
      config:
        command: "golangci-lint"
        args: ["run", "./..."]

  test:
    - name: lint
      engine: "noop"
      runner: alias://my-linter
`)
}
