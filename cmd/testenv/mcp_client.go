package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// projectRoot is determined at package initialization to handle cases
// where tests or code changes the working directory.
var projectRoot string

func init() {
	// Find project root at startup
	cwd, err := os.Getwd()
	if err != nil {
		return
	}

	// Walk up to find forge.yaml or go.mod
	for cwd != "/" && cwd != "." {
		if _, err := os.Stat(filepath.Join(cwd, "forge.yaml")); err == nil {
			projectRoot = cwd
			return
		}
		if _, err := os.Stat(filepath.Join(cwd, "go.mod")); err == nil {
			projectRoot = cwd
			return
		}
		parent := filepath.Dir(cwd)
		if parent == cwd {
			break
		}
		cwd = parent
	}
}

// callMCPEngine calls an MCP engine with the specified tool and parameters.
// It spawns the engine process with --mcp flag, sets up stdio transport, and calls the tool.
func callMCPEngine(binaryPath string, toolName string, params interface{}) (interface{}, error) {
	// Create command to spawn MCP server (without context to avoid premature termination)
	cmd := exec.Command(binaryPath, "--mcp")

	// Inherit environment variables from parent process
	cmd.Env = os.Environ()

	// Forward stderr from the MCP server to show logs
	// Stdin/Stdout are used for JSON-RPC, but stderr is free for logs
	cmd.Stderr = os.Stderr

	// Create MCP client
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "testenv-client",
		Version: "v1.0.0",
	}, nil)

	// Create command transport
	transport := &mcp.CommandTransport{
		Command: cmd,
	}

	// Use a background context for connection (let the tool timeout internally)
	// The MCP server itself will handle timeouts for operations
	ctx := context.Background()

	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MCP server %s: %w", binaryPath, err)
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

	// Call the tool with a timeout context
	// Use 6 minutes to allow for helm's internal 3-4 minute timeout + buffer
	toolCtx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()

	result, err := session.CallTool(toolCtx, &mcp.CallToolParams{
		Name:      toolName,
		Arguments: arguments,
	})
	if err != nil {
		if toolCtx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("MCP tool call timed out after 6 minutes: %w", err)
		}
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
		return nil, fmt.Errorf("operation failed: %s", errMsg)
	}

	// Return the structured content if available
	if result.StructuredContent != nil {
		return result.StructuredContent, nil
	}

	// If no structured content, return nil
	return nil, nil
}

// resolveEngineURI resolves an engine URI (go://package) to a binary path.
// It returns an absolute path to handle cases where tests change working directory.
func resolveEngineURI(engineURI string) (string, error) {
	if !strings.HasPrefix(engineURI, "go://") {
		return "", fmt.Errorf("unsupported engine protocol: %s (must start with go://)", engineURI)
	}

	// Remove go:// prefix
	packagePath := strings.TrimPrefix(engineURI, "go://")
	if packagePath == "" {
		return "", fmt.Errorf("empty engine path after go://")
	}

	// Remove version if present (go://testenv-kind@v1.0.0 -> testenv-kind)
	if idx := strings.Index(packagePath, "@"); idx != -1 {
		packagePath = packagePath[:idx]
	}

	// If we have projectRoot from init, use it
	if projectRoot != "" {
		return filepath.Join(projectRoot, "build", "bin", packagePath), nil
	}

	// Fallback: use relative path from current directory
	return fmt.Sprintf("./build/bin/%s", packagePath), nil
}
