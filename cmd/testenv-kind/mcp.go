package main

import (
	"context"
	"fmt"
	"log"
	"path/filepath"

	"github.com/alexandremahdhaoui/forge/internal/mcpserver"
	"github.com/alexandremahdhaoui/forge/pkg/forge"
	"github.com/alexandremahdhaoui/forge/pkg/mcputil"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// CreateInput represents the input for the create tool.
type CreateInput struct {
	TestID   string            `json:"testID"`         // Test environment ID (required)
	Stage    string            `json:"stage"`          // Test stage name (required)
	TmpDir   string            `json:"tmpDir"`         // Temporary directory for this test environment
	Metadata map[string]string `json:"metadata"`       // Metadata from previous testenv-subengines (optional)
	Spec     map[string]any    `json:"spec,omitempty"` // Optional spec for configuration override
}

// DeleteInput represents the input for the delete tool.
type DeleteInput struct {
	TestID   string            `json:"testID"`   // Test environment ID (required)
	Metadata map[string]string `json:"metadata"` // Metadata from test environment (optional)
}

// runMCPServer starts the MCP server.
func runMCPServer() error {
	v, _, _ := versionInfo.Get()
	server := mcpserver.New("testenv-kind", v)

	// Register create tool
	mcpserver.RegisterTool(server, &mcp.Tool{
		Name:        "create",
		Description: "Create a kind cluster for a test environment",
	}, handleCreateTool)

	// Register delete tool
	mcpserver.RegisterTool(server, &mcp.Tool{
		Name:        "delete",
		Description: "Delete a kind cluster for a test environment",
	}, handleDeleteTool)

	// Run the MCP server
	return server.RunDefault()
}

// handleCreateTool handles the "create" tool call from MCP clients.
func handleCreateTool(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input CreateInput,
) (*mcp.CallToolResult, any, error) {
	log.Printf("Creating kind cluster: testID=%s, stage=%s", input.TestID, input.Stage)

	// Validate inputs
	if result := mcputil.ValidateRequiredWithPrefix("Create failed", map[string]string{
		"testID": input.TestID,
		"stage":  input.Stage,
		"tmpDir": input.TmpDir,
	}); result != nil {
		return result, nil, nil
	}

	// Read forge.yaml configuration
	config, err := forge.ReadSpec()
	if err != nil {
		return mcputil.ErrorResult(fmt.Sprintf("Create failed: %v", err)), nil, nil
	}

	// Read environment variables
	envs, err := readEnvs()
	if err != nil {
		return mcputil.ErrorResult(fmt.Sprintf("Create failed: %v", err)), nil, nil
	}

	// Generate cluster name and kubeconfig path
	clusterName := fmt.Sprintf("%s-%s", config.Name, input.TestID)
	kubeconfigPath := filepath.Join(input.TmpDir, "kubeconfig")

	// Update config with cluster-specific values
	config.Name = clusterName
	config.Kindenv.KubeconfigPath = kubeconfigPath

	// Create the kind cluster
	if err := doSetup(config, envs); err != nil {
		return mcputil.ErrorResult(fmt.Sprintf("Create failed: %v", err)), nil, nil
	}

	// Prepare files map (relative paths within tmpDir)
	files := map[string]string{
		"testenv-kind.kubeconfig": "kubeconfig",
	}

	// Prepare metadata
	metadata := map[string]string{
		"testenv-kind.clusterName":    clusterName,
		"testenv-kind.kubeconfigPath": kubeconfigPath,
	}

	// Prepare managed resources (for cleanup)
	managedResources := []string{
		kubeconfigPath,
	}

	// Return success with artifact
	artifact := map[string]interface{}{
		"testID":           input.TestID,
		"files":            files,
		"metadata":         metadata,
		"managedResources": managedResources,
	}

	result, returnedArtifact := mcputil.SuccessResultWithArtifact(
		fmt.Sprintf("Created kind cluster: %s", clusterName),
		artifact,
	)
	return result, returnedArtifact, nil
}

// handleDeleteTool handles the "delete" tool call from MCP clients.
func handleDeleteTool(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input DeleteInput,
) (*mcp.CallToolResult, any, error) {
	log.Printf("Deleting kind cluster: testID=%s", input.TestID)

	// Validate inputs
	if result := mcputil.ValidateRequiredWithPrefix("Delete failed", map[string]string{
		"testID": input.TestID,
	}); result != nil {
		return result, nil, nil
	}

	// Read forge.yaml configuration
	config, err := forge.ReadSpec()
	if err != nil {
		return mcputil.ErrorResult(fmt.Sprintf("Delete failed: %v", err)), nil, nil
	}

	// Read environment variables
	envs, err := readEnvs()
	if err != nil {
		return mcputil.ErrorResult(fmt.Sprintf("Delete failed: %v", err)), nil, nil
	}

	// Reconstruct cluster name from testID
	clusterName := fmt.Sprintf("%s-%s", config.Name, input.TestID)
	config.Name = clusterName

	// Delete the kind cluster
	if err := doTeardown(config, envs); err != nil {
		// Log error but don't fail - best effort cleanup
		log.Printf("Warning: failed to delete kind cluster: %v", err)
	}

	return mcputil.SuccessResult(fmt.Sprintf("Deleted kind cluster: %s", clusterName)), nil, nil
}
