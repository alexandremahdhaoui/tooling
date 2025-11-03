package main

import (
	"context"
	"fmt"
	"log"

	"github.com/alexandremahdhaoui/forge/internal/mcpserver"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RunInput represents the input parameters for the run tool.
type RunInput struct {
	Stage string `json:"stage"`
	Name  string `json:"name"`
}

// runMCPServerImpl starts the test-runner-go MCP server with stdio transport.
// It creates an MCP server, registers tools, and runs the server until stdin closes.
func runMCPServerImpl() error {
	v, _, _ := versionInfo.Get()
	server := mcpserver.New("test-runner-go", v)

	// Register run tool
	mcpserver.RegisterTool(server, &mcp.Tool{
		Name:        "run",
		Description: "Run tests for a given stage and generate structured report",
	}, handleRunTool)

	// Run the MCP server
	return server.RunDefault()
}

// handleRunTool handles the "run" tool call from MCP clients.
func handleRunTool(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input RunInput,
) (*mcp.CallToolResult, any, error) {
	log.Printf("Running tests: stage=%s name=%s", input.Stage, input.Name)

	// Validate inputs
	if input.Stage == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Test run failed: missing required field 'stage'"},
			},
			IsError: true,
		}, nil, nil
	}

	if input.Name == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Test run failed: missing required field 'name'"},
			},
			IsError: true,
		}, nil, nil
	}

	// Run tests
	report, err := runTests(input.Stage, input.Name)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Test run failed: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	// Create success message
	statusMsg := fmt.Sprintf("Tests %s: stage=%s, total=%d, passed=%d, failed=%d, skipped=%d, coverage=%.1f%%",
		report.Status,
		report.Stage,
		report.TestStats.Total,
		report.TestStats.Passed,
		report.TestStats.Failed,
		report.TestStats.Skipped,
		report.Coverage.Percentage,
	)

	// Return result with TestReport as artifact
	isError := report.Status == "failed"
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: statusMsg},
		},
		IsError: isError,
	}, report, nil
}
