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
	"github.com/alexandremahdhaoui/forge/pkg/forge"
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
	versionInfo = version.New("format-go")
	versionInfo.Version = Version
	versionInfo.CommitSHA = CommitSHA
	versionInfo.BuildTimestamp = BuildTimestamp
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

	// Direct invocation - format current directory
	if err := formatCode("."); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`format-go - Format Go code using gofumpt

Usage:
  format-go [path]          Format Go code at path (default: .)
  format-go --mcp           Run as MCP server
  format-go version         Show version information
  format-go help            Show this help message

Environment Variables:
  GOFUMPT_VERSION          Version of gofumpt to use (default: v0.6.0)`)
}

func runMCPServer() error {
	v, _, _ := versionInfo.Get()
	server := mcpserver.New("format-go", v)

	mcpserver.RegisterTool(server, &mcp.Tool{
		Name:        "build",
		Description: "Format Go code using gofumpt",
	}, handleBuild)

	return server.RunDefault()
}

func handleBuild(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input mcptypes.BuildInput,
) (*mcp.CallToolResult, any, error) {
	path := input.Path
	if path == "" && input.Src != "" {
		path = input.Src
	}
	if path == "" {
		path = "."
	}

	log.Printf("Formatting Go code at: %s", path)

	if err := formatCode(path); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Formatting failed: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	// Return artifact information
	artifact := forge.Artifact{
		Name:      "formatted-code",
		Type:      "formatted",
		Location:  path,
		Timestamp: time.Now().Format(time.RFC3339),
	}

	artifactJSON, _ := json.Marshal(artifact)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(artifactJSON)},
		},
	}, artifact, nil
}

func formatCode(path string) error {
	gofumptVersion := os.Getenv("GOFUMPT_VERSION")
	if gofumptVersion == "" {
		gofumptVersion = "v0.6.0"
	}

	gofumptPkg := fmt.Sprintf("mvdan.cc/gofumpt@%s", gofumptVersion)

	cmd := exec.Command("go", "run", gofumptPkg, "-w", path)
	cmd.Stdout = os.Stderr // Send to stderr to not interfere with MCP JSON-RPC on stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gofumpt failed: %w", err)
	}

	fmt.Fprintf(os.Stderr, "✅ Formatted Go code at %s\n", path)
	return nil
}
