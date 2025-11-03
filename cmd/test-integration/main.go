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

// versionInfo holds test-integration's version information
var versionInfo *version.Info

func init() {
	versionInfo = version.New("test-integration")
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
	case "create":
		stageName := ""
		if len(os.Args) >= 3 {
			stageName = os.Args[2]
		}
		if err := cmdCreate(stageName); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "get":
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "Error: test ID required\n\n")
			printUsage()
			os.Exit(1)
		}
		testID := os.Args[2]
		if err := cmdGet(testID); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "delete":
		if len(os.Args) < 3 {
			fmt.Fprintf(os.Stderr, "Error: test ID required\n\n")
			printUsage()
			os.Exit(1)
		}
		testID := os.Args[2]
		if err := cmdDelete(testID); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "list":
		stageFilter := ""
		// Check for --stage flag
		for i, arg := range os.Args[2:] {
			if arg == "--stage" && i+1 < len(os.Args[2:]) {
				stageFilter = os.Args[i+3]
				break
			}
		}
		if err := cmdList(stageFilter); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "version", "--version", "-v":
		versionInfo.Print()
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`test-integration - Manage integration test environments

Usage:
  test-integration create <STAGE>        Create a test environment
  test-integration get <TEST-ID>         Get test environment details
  test-integration delete <TEST-ID>      Delete a test environment
  test-integration list [--stage=<NAME>] List test environments
  test-integration --mcp                  Run as MCP server
  test-integration version                Show version information

Arguments:
  STAGE     Test stage name (e.g., "integration", "e2e")
  TEST-ID   Test environment ID

Examples:
  test-integration create integration
  test-integration get test-integration-20241103-abc123
  test-integration delete test-integration-20241103-abc123
  test-integration list --stage=integration
  test-integration --mcp`)
}
