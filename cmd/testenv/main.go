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

// versionInfo holds testenv's version information
var versionInfo *version.Info

func init() {
	versionInfo = version.New("testenv")
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
		if _, err := cmdCreate(stageName); err != nil {
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
	fmt.Println(`testenv - Orchestrate test environments

Usage:
  testenv create <STAGE>        Create a test environment
  testenv delete <TEST-ID>      Delete a test environment
  testenv --mcp                 Run as MCP server
  testenv version               Show version information

Arguments:
  STAGE     Test stage name (e.g., "integration", "e2e")
  TEST-ID   Test environment ID

Examples:
  testenv create integration
  testenv delete test-integration-20241103-abc123
  testenv --mcp

Note:
  Use 'forge test <stage> get/list' to view test environments.
  testenv only handles create/delete operations.`)
}
