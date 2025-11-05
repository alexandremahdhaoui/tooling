package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/alexandremahdhaoui/forge/internal/mcpserver"
	"github.com/alexandremahdhaoui/forge/pkg/forge"
	"github.com/alexandremahdhaoui/forge/pkg/mcputil"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// CreateInput represents the input for the create tool.
type CreateInput struct {
	Stage string `json:"stage"`
}

// GetInput represents the input for the get tool.
type GetInput struct {
	ReportID string `json:"reportID"`
}

// DeleteInput represents the input for the delete tool.
type DeleteInput struct {
	ReportID string `json:"reportID"`
}

// ListInput represents the input for the list tool.
type ListInput struct {
	Stage string `json:"stage,omitempty"`
}

// runMCPServer starts the MCP server.
func runMCPServer() error {
	v, _, _ := versionInfo.Get()
	server := mcpserver.New("test-report", v)

	// Register create tool (no-op for compatibility with test engine interface)
	mcpserver.RegisterTool(server, &mcp.Tool{
		Name:        "create",
		Description: "No-op create operation (test reports are created by test runners)",
	}, handleCreateTool)

	// Register get tool
	mcpserver.RegisterTool(server, &mcp.Tool{
		Name:        "get",
		Description: "Get test report details by ID",
	}, handleGetTool)

	// Register delete tool
	mcpserver.RegisterTool(server, &mcp.Tool{
		Name:        "delete",
		Description: "Delete a test report and its artifacts by ID",
	}, handleDeleteTool)

	// Register list tool
	mcpserver.RegisterTool(server, &mcp.Tool{
		Name:        "list",
		Description: "List test reports, optionally filtered by stage",
	}, handleListTool)

	// Run the MCP server
	return server.RunDefault()
}

// handleCreateTool handles the "create" tool call from MCP clients.
// This is a no-op operation since test reports are created by test runners,
// not by the test engine. This exists only for compatibility with the test engine interface.
func handleCreateTool(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input CreateInput,
) (*mcp.CallToolResult, any, error) {
	log.Printf("Create called (no-op): stage=%s", input.Stage)

	// Validate input
	if result := mcputil.ValidateRequiredWithPrefix("Create failed", map[string]string{
		"stage": input.Stage,
	}); result != nil {
		return result, nil, nil
	}

	// No-op success - test reports are created by test runners during execution
	return mcputil.SuccessResult(fmt.Sprintf("No-op: test reports for stage '%s' are created by test runners during execution", input.Stage)), nil, nil
}

// handleGetTool handles the "get" tool call from MCP clients.
func handleGetTool(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input GetInput,
) (*mcp.CallToolResult, any, error) {
	log.Printf("Getting test report: reportID=%s", input.ReportID)

	// Validate inputs
	if result := mcputil.ValidateRequiredWithPrefix("Get failed", map[string]string{
		"reportID": input.ReportID,
	}); result != nil {
		return result, nil, nil
	}

	// Get artifact store path (environment variable takes precedence)
	artifactStorePath := os.Getenv("FORGE_ARTIFACT_STORE_PATH")
	if artifactStorePath == "" {
		var err error
		artifactStorePath, err = forge.GetArtifactStorePath(".forge/artifacts.yaml")
		if err != nil {
			return mcputil.ErrorResult(fmt.Sprintf("Get failed: %v", err)), nil, nil
		}
	}

	// Read artifact store
	store, err := forge.ReadOrCreateArtifactStore(artifactStorePath)
	if err != nil {
		return mcputil.ErrorResult(fmt.Sprintf("Get failed: %v", err)), nil, nil
	}

	// Get test report
	report, err := forge.GetTestReport(&store, input.ReportID)
	if err != nil {
		return mcputil.ErrorResult(fmt.Sprintf("Get failed: test report not found: %s", input.ReportID)), nil, nil
	}

	result, returnedArtifact := mcputil.SuccessResultWithArtifact(
		fmt.Sprintf("Test report: %s (stage: %s, status: %s, tests: %d/%d passed)",
			report.ID, report.Stage, report.Status,
			report.TestStats.Passed, report.TestStats.Total),
		report,
	)
	return result, returnedArtifact, nil
}

// handleDeleteTool handles the "delete" tool call from MCP clients.
func handleDeleteTool(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input DeleteInput,
) (*mcp.CallToolResult, any, error) {
	log.Printf("Deleting test report: reportID=%s", input.ReportID)

	// Validate inputs
	if result := mcputil.ValidateRequiredWithPrefix("Delete failed", map[string]string{
		"reportID": input.ReportID,
	}); result != nil {
		return result, nil, nil
	}

	// Delete test report (call cmdDelete which handles files and store update)
	if err := cmdDelete(input.ReportID); err != nil {
		return mcputil.ErrorResult(fmt.Sprintf("Delete failed: %v", err)), nil, nil
	}

	return mcputil.SuccessResult(fmt.Sprintf("Deleted test report: %s", input.ReportID)), nil, nil
}

// handleListTool handles the "list" tool call from MCP clients.
func handleListTool(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input ListInput,
) (*mcp.CallToolResult, any, error) {
	log.Printf("Listing test reports: stage=%s", input.Stage)

	// Get artifact store path (environment variable takes precedence)
	artifactStorePath := os.Getenv("FORGE_ARTIFACT_STORE_PATH")
	if artifactStorePath == "" {
		var err error
		artifactStorePath, err = forge.GetArtifactStorePath(".forge/artifacts.yaml")
		if err != nil {
			return mcputil.ErrorResult(fmt.Sprintf("List failed: %v", err)), nil, nil
		}
	}

	// Read artifact store
	store, err := forge.ReadOrCreateArtifactStore(artifactStorePath)
	if err != nil {
		return mcputil.ErrorResult(fmt.Sprintf("List failed: %v", err)), nil, nil
	}

	// Get test reports
	reports := forge.ListTestReports(&store, input.Stage)

	msg := fmt.Sprintf("Found %d test report(s)", len(reports))
	if input.Stage != "" {
		msg = fmt.Sprintf("Found %d test report(s) for stage: %s", len(reports), input.Stage)
	}

	result, returnedArtifact := mcputil.SuccessResultWithArtifact(msg, reports)
	return result, returnedArtifact, nil
}
