package main

import (
	"fmt"
	"log"
	"os"

	"github.com/alexandremahdhaoui/forge/internal/version"
)

// Version information (set via ldflags during build)
var (
	Version        = "dev"
	CommitSHA      = "unknown"
	BuildTimestamp = "unknown"
)

// versionInfo holds go-test's version information
var versionInfo *version.Info

func init() {
	versionInfo = version.New("go-test")
	versionInfo.Version = Version
	versionInfo.CommitSHA = CommitSHA
	versionInfo.BuildTimestamp = BuildTimestamp
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
	fmt.Println(`go-test - Run Go tests and generate structured reports

Usage:
  go-test <STAGE> <NAME>     Run tests for the given stage
  go-test --mcp               Run as MCP server
  go-test version             Show version information

Arguments:
  STAGE    Test stage name (e.g., "unit", "integration", "e2e")
  NAME     Test run identifier (used for output files)

Examples:
  go-test unit mytest
  go-test integration smoke-20241103
  go-test --mcp

Output:
  - Test output is written to stderr
  - Structured JSON report is written to stdout`)
}

// runMCPServer starts the MCP server.
func runMCPServer() error {
	return runMCPServerImpl()
}
