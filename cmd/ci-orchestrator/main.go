package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/alexandremahdhaoui/forge/internal/mcpserver"
	"github.com/alexandremahdhaoui/forge/internal/version"
	"github.com/alexandremahdhaoui/forge/pkg/mcputil"
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
	versionInfo = version.New("ci-orchestrator")
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
		if err := runMCPServer(); err != nil {
			log.Printf("MCP server error: %v", err)
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
	fmt.Print(`ci-orchestrator - Orchestrate CI pipelines (not yet implemented)

Usage:
  ci-orchestrator --mcp           Run as MCP server (not yet implemented)
  ci-orchestrator version         Show version information
  ci-orchestrator help            Show this help message

Description:
  ci-orchestrator is a placeholder for future CI pipeline orchestration
  functionality. Currently not implemented.
`)
}

// RunInput represents the input for the run tool.
type RunInput struct {
	Pipeline string `json:"pipeline"`
}

func runMCPServer() error {
	v, _, _ := versionInfo.Get()
	server := mcpserver.New("ci-orchestrator", v)

	// Register run tool (not yet implemented)
	mcpserver.RegisterTool(server, &mcp.Tool{
		Name:        "run",
		Description: "Run CI pipeline (not yet implemented)",
	}, handleRunTool)

	return server.RunDefault()
}

func handleRunTool(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input RunInput,
) (*mcp.CallToolResult, any, error) {
	log.Printf("Run called (not yet implemented): pipeline=%s", input.Pipeline)
	return mcputil.ErrorResult("ci-orchestrator: not yet implemented"), nil, nil
}
