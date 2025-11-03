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

// versionInfo holds forge's version information
var versionInfo *version.Info

func init() {
	versionInfo = version.New("forge")
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
	case "build":
		if err := runBuild(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "integration":
		if err := runIntegration(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "version", "--version", "-v":
		versionInfo.Print()
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`forge - A build orchestration tool

Usage:
  forge build [artifact-name]    Build artifacts from forge.yaml
  forge integration <command>    Manage integration environments
  forge version                  Show version information

Commands:
  build                         Build all artifacts
  integration create [name]     Create integration environment
  integration list              List integration environments
  integration get <id>          Get environment details
  integration delete <id>       Delete integration environment
  version                       Show version information
  help                          Show this help message`)
}
