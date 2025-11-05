package main

import (
	"context"
	"fmt"
	"log"
	"time"

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
	TestID string `json:"testID"`
}

// DeleteInput represents the input for the delete tool.
type DeleteInput struct {
	TestID string `json:"testID"`
}

// ListInput represents the input for the list tool.
type ListInput struct {
	Stage string `json:"stage,omitempty"`
}

// runMCPServer starts the MCP server.
func runMCPServer() error {
	v, _, _ := versionInfo.Get()
	server := mcpserver.New("test-integration", v)

	// Register create tool
	mcpserver.RegisterTool(server, &mcp.Tool{
		Name:        "create",
		Description: "Create a test environment for a given stage",
	}, handleCreateTool)

	// Register get tool
	mcpserver.RegisterTool(server, &mcp.Tool{
		Name:        "get",
		Description: "Get test environment details by ID",
	}, handleGetTool)

	// Register delete tool
	mcpserver.RegisterTool(server, &mcp.Tool{
		Name:        "delete",
		Description: "Delete a test environment by ID",
	}, handleDeleteTool)

	// Register list tool
	mcpserver.RegisterTool(server, &mcp.Tool{
		Name:        "list",
		Description: "List test environments, optionally filtered by stage",
	}, handleListTool)

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

	// Validate inputs
	if result := mcputil.ValidateRequiredWithPrefix("Create failed", map[string]string{
		"stage": input.Stage,
	}); result != nil {
		return result, nil, nil
	}

	// Create test environment (capture output to get test ID)
	testID := ""
	{
		// Read config
		config, err := forge.ReadSpec()
		if err != nil {
			return mcputil.ErrorResult(fmt.Sprintf("Create failed: %v", err)), nil, nil
		}

		// Find TestSpec
		var testSpec *forge.TestSpec
		for i := range config.Test {
			if config.Test[i].Name == input.Stage {
				testSpec = &config.Test[i]
				break
			}
		}

		if testSpec == nil {
			return mcputil.ErrorResult(fmt.Sprintf("Create failed: test stage not found: %s", input.Stage)), nil, nil
		}

		testID = generateTestID(input.Stage)

		// Create environment (simplified version of cmdCreate)
		now := time.Now().UTC()
		env := &forge.TestEnvironment{
			ID:               testID,
			Name:             input.Stage,
			Status:           forge.TestStatusCreated,
			CreatedAt:        now,
			UpdatedAt:        now,
			ManagedResources: []string{},
			Metadata:         make(map[string]string),
		}

		artifactStorePath, err := forge.GetArtifactStorePath(".forge/artifacts.json")
		if err != nil {
			return mcputil.ErrorResult(fmt.Sprintf("Create failed: %v", err)), nil, nil
		}

		store, err := forge.ReadOrCreateArtifactStore(artifactStorePath)
		if err != nil {
			return mcputil.ErrorResult(fmt.Sprintf("Create failed: %v", err)), nil, nil
		}

		forge.AddOrUpdateTestEnvironment(&store, env)

		if err := forge.WriteArtifactStore(artifactStorePath, store); err != nil {
			return mcputil.ErrorResult(fmt.Sprintf("Create failed: %v", err)), nil, nil
		}
	}

	result, returnedArtifact := mcputil.SuccessResultWithArtifact(
		fmt.Sprintf("Created test environment: %s", testID),
		map[string]string{"testID": testID},
	)
	return result, returnedArtifact, nil
}

// handleGetTool handles the "get" tool call from MCP clients.
func handleGetTool(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input GetInput,
) (*mcp.CallToolResult, any, error) {
	log.Printf("Getting test environment: testID=%s", input.TestID)

	// Validate inputs
	if result := mcputil.ValidateRequiredWithPrefix("Get failed", map[string]string{
		"testID": input.TestID,
	}); result != nil {
		return result, nil, nil
	}

	// Get artifact store path
	artifactStorePath, err := forge.GetArtifactStorePath(".forge/artifacts.json")
	if err != nil {
		return mcputil.ErrorResult(fmt.Sprintf("Get failed: %v", err)), nil, nil
	}

	store, err := forge.ReadOrCreateArtifactStore(artifactStorePath)
	if err != nil {
		return mcputil.ErrorResult(fmt.Sprintf("Get failed: %v", err)), nil, nil
	}

	env, err := forge.GetTestEnvironment(&store, input.TestID)
	if err != nil {
		return mcputil.ErrorResult(fmt.Sprintf("Get failed: test environment not found: %s", input.TestID)), nil, nil
	}

	result, returnedArtifact := mcputil.SuccessResultWithArtifact(
		fmt.Sprintf("Test environment: %s (stage: %s, status: %s)", env.ID, env.Name, env.Status),
		env,
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

// handleListTool handles the "list" tool call from MCP clients.
func handleListTool(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input ListInput,
) (*mcp.CallToolResult, any, error) {
	log.Printf("Listing test environments: stage=%s", input.Stage)

	// Get artifact store path
	artifactStorePath, err := forge.GetArtifactStorePath(".forge/artifacts.json")
	if err != nil {
		return mcputil.ErrorResult(fmt.Sprintf("List failed: %v", err)), nil, nil
	}

	store, err := forge.ReadOrCreateArtifactStore(artifactStorePath)
	if err != nil {
		return mcputil.ErrorResult(fmt.Sprintf("List failed: %v", err)), nil, nil
	}

	envs := forge.ListTestEnvironments(&store, input.Stage)

	msg := fmt.Sprintf("Found %d test environment(s)", len(envs))
	if input.Stage != "" {
		msg = fmt.Sprintf("Found %d test environment(s) for stage: %s", len(envs), input.Stage)
	}

	result, returnedArtifact := mcputil.SuccessResultWithArtifact(msg, envs)
	return result, returnedArtifact, nil
}
