package main

import (
	"context"
	"fmt"
	"log"
	"os"
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
	server := mcpserver.New("testenv-lcr", v)

	// Register create tool
	mcpserver.RegisterTool(server, &mcp.Tool{
		Name:        "create",
		Description: "Create a local container registry in a kind cluster",
	}, handleCreateTool)

	// Register delete tool
	mcpserver.RegisterTool(server, &mcp.Tool{
		Name:        "delete",
		Description: "Delete a local container registry from a kind cluster",
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
	log.Printf("Creating local container registry: testID=%s, stage=%s", input.TestID, input.Stage)

	// Redirect stdout to stderr immediately (setup() and other code writes to stdout, but MCP uses stdout for JSON-RPC)
	oldStdout := os.Stdout
	os.Stdout = os.Stderr
	defer func() { os.Stdout = oldStdout }()

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

	// Override config with spec values if provided
	if input.Spec != nil {
		if enabled, ok := input.Spec["enabled"].(bool); ok {
			config.LocalContainerRegistry.Enabled = enabled
		}
		if autoPush, ok := input.Spec["autoPushImages"].(bool); ok {
			config.LocalContainerRegistry.AutoPushImages = autoPush
		}
		if namespace, ok := input.Spec["namespace"].(string); ok {
			config.LocalContainerRegistry.Namespace = namespace
		}
	}

	// Check if local container registry is enabled
	if !config.LocalContainerRegistry.Enabled {
		return mcputil.SuccessResult("Local container registry is disabled, skipping setup"), nil, nil
	}

	// Override kubeconfig path from metadata (if provided by testenv-kind)
	if kubeconfigPath, ok := input.Metadata["testenv-kind.kubeconfigPath"]; ok {
		config.Kindenv.KubeconfigPath = kubeconfigPath
	}

	// Override file paths to use tmpDir
	caCrtPath := filepath.Join(input.TmpDir, "ca.crt")
	credentialPath := filepath.Join(input.TmpDir, "registry-credentials.yaml")

	config.LocalContainerRegistry.CaCrtPath = caCrtPath
	config.LocalContainerRegistry.CredentialPath = credentialPath

	// Call the existing setup logic (stdout already redirected to stderr at function start)
	if err := setup(); err != nil {
		return mcputil.ErrorResult(fmt.Sprintf("Create failed: %v", err)), nil, nil
	}

	// Prepare files map (relative paths within tmpDir)
	files := map[string]string{
		"testenv-lcr.ca.crt":           "ca.crt",
		"testenv-lcr.credentials.yaml": "registry-credentials.yaml",
	}

	// Prepare metadata
	registryFQDN := fmt.Sprintf("%s.%s.svc.cluster.local:5000", Name, config.LocalContainerRegistry.Namespace)
	metadata := map[string]string{
		"testenv-lcr.registryFQDN":   registryFQDN,
		"testenv-lcr.namespace":      config.LocalContainerRegistry.Namespace,
		"testenv-lcr.caCrtPath":      caCrtPath,
		"testenv-lcr.credentialPath": credentialPath,
	}

	// Prepare managed resources (for cleanup)
	managedResources := []string{
		caCrtPath,
		credentialPath,
	}

	// Return success with artifact
	artifact := map[string]interface{}{
		"testID":           input.TestID,
		"files":            files,
		"metadata":         metadata,
		"managedResources": managedResources,
	}

	result, returnedArtifact := mcputil.SuccessResultWithArtifact(
		fmt.Sprintf("Created local container registry: %s", registryFQDN),
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
	log.Printf("Deleting local container registry: testID=%s", input.TestID)

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

	// Check if local container registry is enabled
	if !config.LocalContainerRegistry.Enabled {
		return mcputil.SuccessResult("Local container registry is disabled, skipping teardown"), nil, nil
	}

	// Override kubeconfig path from metadata (if provided)
	if kubeconfigPath, ok := input.Metadata["testenv-kind.kubeconfigPath"]; ok {
		config.Kindenv.KubeconfigPath = kubeconfigPath
	}

	// Call the existing teardown logic
	if err := teardown(); err != nil {
		// Log error but don't fail - best effort cleanup
		log.Printf("Warning: failed to delete local container registry: %v", err)
	}

	return mcputil.SuccessResult("Deleted local container registry"), nil, nil
}
