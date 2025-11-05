package main

import (
	"fmt"
	"log"
	"os"

	"github.com/alexandremahdhaoui/forge/internal/cmdutil"
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
	versionInfo = version.New("generic-engine")
	versionInfo.Version = Version
	versionInfo.CommitSHA = CommitSHA
	versionInfo.BuildTimestamp = BuildTimestamp
}

// Type aliases for convenience
type ExecuteInput = cmdutil.ExecuteInput
type ExecuteOutput = cmdutil.ExecuteOutput

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

	// Default: print usage (command execution will be added in Task 2.2)
	printUsage()
}

func printUsage() {
	fmt.Print(`generic-engine - Execute arbitrary shell commands as a build engine

Usage:
  generic-engine --mcp           Run as MCP server
  generic-engine version         Show version information
  generic-engine help            Show this help message

Description:
  generic-engine is a generic command executor that can be used as a build engine
  in Forge. It wraps shell commands and provides MCP server functionality for
  integration with the Forge build system.

  When running as an MCP server (--mcp), it exposes a "build" tool that accepts
  command, args, environment variables, and working directory configuration.

Environment Variables:
  None specific to this tool. Environment variables can be passed via MCP calls.

Example (via MCP):
  The generic-engine is typically invoked via Forge using engine aliases:

  engines:
    - alias: my-formatter
      engine: go://generic-engine
      config:
        command: "gofmt"
        args: ["-w", "."]
        env:
          GOFMT_STYLE: "google"

  build:
    - name: format-code
      src: .
      engine: alias://my-formatter
`)
}
