package main

import (
	"context"
	"fmt"
	"log"

	"github.com/alexandremahdhaoui/forge/internal/mcpserver"
	"github.com/alexandremahdhaoui/forge/pkg/mcptypes"
	"github.com/alexandremahdhaoui/forge/pkg/mcputil"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// runMCPServer starts the generic-test-runner MCP server with stdio transport.
func runMCPServer() error {
	v, _, _ := versionInfo.Get()
	server := mcpserver.New("generic-test-runner", v)

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
	input mcptypes.RunInput,
) (*mcp.CallToolResult, any, error) {
	log.Printf("Running tests: stage=%s name=%s command=%s", input.Stage, input.Name, input.Command)

	// Validate required fields
	if result := mcputil.ValidateRequiredWithPrefix("Test run failed", map[string]string{
		"stage":   input.Stage,
		"name":    input.Name,
		"command": input.Command,
	}); result != nil {
		return result, nil, nil
	}

	// Convert RunInput to TestInput
	testInput := TestInput{
		Stage:    input.Stage,
		Name:     input.Name,
		Command:  input.Command,
		Args:     input.Args,
		Env:      input.Env,
		EnvFile:  input.EnvFile,
		WorkDir:  input.WorkDir,
		TmpDir:   input.TmpDir,
		BuildDir: input.BuildDir,
		RootDir:  input.RootDir,
	}

	// Run tests
	report, err := runTests(testInput)
	if err != nil {
		return mcputil.ErrorResult(fmt.Sprintf("Test run failed: %v", err)), nil, nil
	}

	// Create status message
	statusMsg := fmt.Sprintf("Tests %s: stage=%s, total=%d, passed=%d, failed=%d, skipped=%d",
		report.Status,
		report.Stage,
		report.TestStats.Total,
		report.TestStats.Passed,
		report.TestStats.Failed,
		report.TestStats.Skipped,
	)

	// Return result with TestReport as artifact based on status
	if report.Status == "failed" {
		result := mcputil.ErrorResult(statusMsg)
		return result, report, nil
	}
	result, returnedArtifact := mcputil.SuccessResultWithArtifact(statusMsg, report)
	return result, returnedArtifact, nil
}
