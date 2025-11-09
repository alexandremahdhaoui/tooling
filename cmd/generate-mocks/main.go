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
	versionInfo = version.New("generate-mocks")
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

	// Direct invocation - generate mocks
	if err := generateMocks(""); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`generate-mocks - Generate Go mocks using mockery

Usage:
  generate-mocks                Generate mocks
  generate-mocks --mcp          Run as MCP server
  generate-mocks version        Show version information
  generate-mocks help           Show this help message

Environment Variables:
  MOCKERY_VERSION              Version of mockery to use (default: v2.42.0)
  MOCKS_DIR                    Directory to clean/generate mocks (default: ./internal/util/mocks)`)
}

func runMCPServer() error {
	v, _, _ := versionInfo.Get()
	server := mcpserver.New("generate-mocks", v)

	mcpserver.RegisterTool(server, &mcp.Tool{
		Name:        "build",
		Description: "Generate Go mocks using mockery",
	}, handleBuild)

	return server.RunDefault()
}

func handleBuild(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input mcptypes.BuildInput,
) (*mcp.CallToolResult, any, error) {
	log.Printf("Generating mocks")

	// Get mocksDir from environment variable
	mocksDir := os.Getenv("MOCKS_DIR")

	if err := generateMocks(mocksDir); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Mock generation failed: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	// Return artifact information
	artifact := forge.Artifact{
		Name:      "mocks",
		Type:      "generated",
		Location:  getMocksDir(mocksDir),
		Timestamp: time.Now().Format(time.RFC3339),
	}

	artifactJSON, _ := json.Marshal(artifact)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(artifactJSON)},
		},
	}, artifact, nil
}

func getMocksDir(mocksDir string) string {
	if mocksDir != "" {
		return mocksDir
	}
	if envDir := os.Getenv("MOCKS_DIR"); envDir != "" {
		return envDir
	}
	return "./internal/util/mocks"
}

func generateMocks(mocksDir string) error {
	mockeryVersion := os.Getenv("MOCKERY_VERSION")
	if mockeryVersion == "" {
		mockeryVersion = "v3.5.5"
	}

	mockery := fmt.Sprintf("github.com/vektra/mockery/v3@%s", mockeryVersion)

	// Clean mocks directory
	dir := getMocksDir(mocksDir)
	if err := os.RemoveAll(dir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to clean mocks directory: %w", err)
	}

	// Generate mocks
	cmd := exec.Command("go", "run", mockery)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mockery failed: %w", err)
	}

	fmt.Fprintf(os.Stderr, "✅ Generated mocks in %s\n", dir)
	return nil
}
