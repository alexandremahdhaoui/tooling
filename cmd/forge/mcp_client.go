package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// callMCPEngine calls an MCP engine with the specified tool and parameters.
// It spawns the engine process with --mcp flag, sets up stdio transport, and calls the tool.
// The command and args parameters specify how to execute the MCP server:
//   - For go run: command="go", args=["run", "package/path"]
//   - For binary: command="binary-path", args=nil
func callMCPEngine(command string, args []string, toolName string, params interface{}) (interface{}, error) {
	// Create command to spawn MCP server
	// Append --mcp flag to the args
	cmdArgs := append(args, "--mcp")
	cmd := exec.Command(command, cmdArgs...)

	// Inherit environment variables from parent process
	cmd.Env = os.Environ()

	// Forward stderr from the MCP server to show build logs
	// Stdin/Stdout are used for JSON-RPC, but stderr is free for logs
	cmd.Stderr = os.Stderr

	// Create MCP client
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "forge-client",
		Version: "v1.0.0",
	}, nil)

	// Create command transport
	transport := &mcp.CommandTransport{
		Command: cmd,
	}

	// Connect to the MCP server
	ctx := context.Background()
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MCP server %s %v: %w", command, args, err)
	}
	defer func() { _ = session.Close() }()

	// Convert params to map[string]any for CallTool
	var arguments map[string]any
	switch p := params.(type) {
	case map[string]any:
		arguments = p
	default:
		// If params is a struct, we need to convert it
		// For now, assume it's already in the right format
		arguments = params.(map[string]any)
	}

	// Call the tool
	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      toolName,
		Arguments: arguments,
	})
	if err != nil {
		return nil, fmt.Errorf("MCP tool call failed: %w", err)
	}

	// Check if result indicates an error
	if result.IsError {
		errMsg := "unknown error"
		if len(result.Content) > 0 {
			if textContent, ok := result.Content[0].(*mcp.TextContent); ok {
				errMsg = textContent.Text
			}
		}
		return nil, fmt.Errorf("build failed: %s", errMsg)
	}

	// Return the structured content if available
	if result.StructuredContent != nil {
		return result.StructuredContent, nil
	}

	// If no structured content, return nil (caller should handle this)
	// This avoids returning the raw mcp.CallToolResult which would print as "&{...}"
	return nil, nil
}
