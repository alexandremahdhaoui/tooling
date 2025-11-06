package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/alexandremahdhaoui/forge/internal/mcpserver"
	"github.com/alexandremahdhaoui/forge/pkg/mcputil"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// CreateInput represents the input for the create tool.
type CreateInput struct {
	Stage string `json:"stage"`
}

// DeleteInput represents the input for the delete tool.
type DeleteInput struct {
	TestID string `json:"testID"`
}

// runMCPServer starts the MCP server.
func runMCPServer() error {
	v, _, _ := versionInfo.Get()
	server := mcpserver.New("testenv", v)

	// Register create tool
	mcpserver.RegisterTool(server, &mcp.Tool{
		Name:        "create",
		Description: "Create a test environment for a given stage",
	}, handleCreateTool)

	// Register delete tool
	mcpserver.RegisterTool(server, &mcp.Tool{
		Name:        "delete",
		Description: "Delete a test environment by ID",
	}, handleDeleteTool)

	// NOTE: get/list are NOT implemented here
	// forge handles get/list by reading the artifact store directly

	// Run the MCP server
	return server.RunDefault()
}

// handleCreateTool handles the "create" tool call from MCP clients.
func handleCreateTool(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input CreateInput,
) (*mcp.CallToolResult, any, error) {
	log.Printf("Creating test environment: stage=%s", input.Stage)

	// Redirect stdout to stderr immediately (cmdCreate writes to stdout, but MCP uses stdout for JSON-RPC)
	oldStdout := os.Stdout
	os.Stdout = os.Stderr
	defer func() { os.Stdout = oldStdout }()

	// Validate inputs
	if result := mcputil.ValidateRequiredWithPrefix("Create failed", map[string]string{
		"stage": input.Stage,
	}); result != nil {
		return result, nil, nil
	}

	// Call cmdCreate to do the actual work (including orchestration)
	testID, err := cmdCreate(input.Stage)
	if err != nil {
		return mcputil.ErrorResult(fmt.Sprintf("Create failed: %v", err)), nil, nil
	}

	result, returnedArtifact := mcputil.SuccessResultWithArtifact(
		fmt.Sprintf("Created test environment: %s", testID),
		map[string]string{"testID": testID},
	)
	return result, returnedArtifact, nil
}

// handleDeleteTool handles the "delete" tool call from MCP clients.
func handleDeleteTool(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input DeleteInput,
) (*mcp.CallToolResult, any, error) {
	log.Printf("Deleting test environment: testID=%s", input.TestID)

	// Validate inputs
	if result := mcputil.ValidateRequiredWithPrefix("Delete failed", map[string]string{
		"testID": input.TestID,
	}); result != nil {
		return result, nil, nil
	}

	// Delete test environment (call cmdDelete)
	if err := cmdDelete(input.TestID); err != nil {
		return mcputil.ErrorResult(fmt.Sprintf("Delete failed: %v", err)), nil, nil
	}

	return mcputil.SuccessResult(fmt.Sprintf("Deleted test environment: %s", input.TestID)), nil, nil
}
