// Package cli provides common CLI bootstrapping functionality for forge commands.
//
// This package eliminates duplicated main() function logic across 18+ command binaries
// by providing a unified bootstrap mechanism that handles:
//   - Version information initialization from ldflags
//   - Version flag handling (--version, -v, version)
//   - MCP server mode handling (--mcp flag)
//   - Standardized error handling and exit codes
//
// Example usage:
//
//	package main
//
//	import (
//	    "github.com/alexandremahdhaoui/forge/internal/cli"
//	)
//
//	// Version information (set via ldflags)
//	var (
//	    Version        = "dev"
//	    CommitSHA      = "unknown"
//	    BuildTimestamp = "unknown"
//	)
//
//	func main() {
//	    cli.Bootstrap(cli.Config{
//	        Name:           "my-command",
//	        Version:        Version,
//	        CommitSHA:      CommitSHA,
//	        BuildTimestamp: BuildTimestamp,
//	        RunCLI:         runCLI,
//	        RunMCP:         runMCP,
//	    })
//	}
//
//	func runCLI() error {
//	    // Command-specific CLI logic
//	    return nil
//	}
//
//	func runMCP() error {
//	    // Command-specific MCP server logic
//	    return nil
//	}
package cli
