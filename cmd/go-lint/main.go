package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/alexandremahdhaoui/forge/internal/mcpserver"
	"github.com/alexandremahdhaoui/forge/internal/version"
	"github.com/alexandremahdhaoui/forge/pkg/mcptypes"
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
	versionInfo = version.New("go-lint")
	versionInfo.Version = Version
	versionInfo.CommitSHA = CommitSHA
	versionInfo.BuildTimestamp = BuildTimestamp
}

// TestReport represents the structured output of a lint run
type TestReport struct {
	Status       string  `json:"status"` // "passed" or "failed"
	ErrorMessage string  `json:"error,omitempty"`
	Duration     float64 `json:"duration"` // seconds
	Total        int     `json:"total"`    // total issues found
	Passed       int     `json:"passed"`   // always 0 or 1
	Failed       int     `json:"failed"`   // 0 if passed, 1 if failed
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
	fmt.Println(`go-lint - Lint Go code using golangci-lint

Usage:
  go-lint <STAGE> <NAME>        Run linter for the given stage
  go-lint --mcp                 Run as MCP server
  go-lint version               Show version information

Arguments:
  STAGE    Test stage name (e.g., "lint")
  NAME     Test run identifier

Examples:
  go-lint lint my-lint-20241103
  go-lint --mcp

Environment Variables:
  GOLANGCI_LINT_VERSION    Version of golangci-lint to use (default: v1.59.1)

Output:
  - Lint output is written to stderr
  - Structured JSON report is written to stdout`)
}

func runMCPServer() error {
	v, _, _ := versionInfo.Get()
	server := mcpserver.New("go-lint", v)

	mcpserver.RegisterTool(server, &mcp.Tool{
		Name:        "run",
		Description: "Run Go linter using golangci-lint",
	}, handleRun)

	return server.RunDefault()
}

func handleRun(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input mcptypes.RunInput,
) (*mcp.CallToolResult, any, error) {
	log.Printf("Running linter: stage=%s, name=%s", input.Stage, input.Name)

	report, err := runLint(input.Stage, input.Name)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Lint execution failed: %v", err)},
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
	// Execute linter and generate report
	report, err := runLint(stage, name)
	if err != nil {
		return fmt.Errorf("lint execution failed: %w", err)
	}

	// Output JSON report to stdout
	if err := json.NewEncoder(os.Stdout).Encode(report); err != nil {
		return fmt.Errorf("failed to encode report: %w", err)
	}

	// Exit with non-zero if linting failed
	if report.Status == "failed" {
		os.Exit(1)
	}

	return nil
}

func runLint(stage, name string) (*TestReport, error) {
	startTime := time.Now()

	golangciVersion := os.Getenv("GOLANGCI_LINT_VERSION")
	if golangciVersion == "" {
		golangciVersion = "v2.6.0"
	}

	golangciPkg := fmt.Sprintf("github.com/golangci/golangci-lint/v2/cmd/golangci-lint@%s", golangciVersion)

	args := []string{"run", golangciPkg, "run", "--fix"}

	cmd := exec.Command("go", args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	// Execute the command
	err := cmd.Run()
	duration := time.Since(startTime).Seconds()

	// Determine status based on exit code
	status := "passed"
	errorMessage := ""
	total := 0
	passed := 1
	failed := 0

	if err != nil {
		status = "failed"
		failed = 1
		passed = 0
		if exitErr, ok := err.(*exec.ExitError); ok {
			total = 1 // At least one issue found
			errorMessage = fmt.Sprintf("linting failed with exit code %d", exitErr.ExitCode())
		} else {
			errorMessage = fmt.Sprintf("failed to execute linter: %v", err)
		}
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
