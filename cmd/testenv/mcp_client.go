package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/alexandremahdhaoui/forge/internal/forgepath"
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

	// If this is a "go run" command, set working directory to forge repository
	// This ensures go run can find the go.mod file
	if command == "go" && len(args) > 0 && args[0] == "run" {
		if forgeRepo, err := forgepath.FindForgeRepo(); err == nil {
			cmd.Dir = forgeRepo
		}
	}

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

// resolveEngineURI resolves an engine URI (go://package) to command and args for execution.
// Returns command, args, and error.
func resolveEngineURI(engineURI string) (string, []string, error) {
	if !strings.HasPrefix(engineURI, "go://") {
		return "", nil, fmt.Errorf("unsupported engine protocol: %s (must start with go://)", engineURI)
	}

	// Remove go:// prefix
	packagePath := strings.TrimPrefix(engineURI, "go://")
	if packagePath == "" {
		return "", nil, fmt.Errorf("empty engine path after go://")
	}

	// Remove version if present (go://testenv-kind@v1.0.0 -> testenv-kind)
	if idx := strings.Index(packagePath, "@"); idx != -1 {
		packagePath = packagePath[:idx]
	}

	// Extract package name (handle full paths like "github.com/user/repo/cmd/tool")
	if strings.Contains(packagePath, "/") {
		parts := strings.Split(packagePath, "/")
		packagePath = parts[len(parts)-1]
	}

	// Use forgepath to build the go run command
	runArgs, err := forgepath.BuildGoRunCommand(packagePath)
	if err != nil {
		return "", nil, fmt.Errorf("failed to build go run command for %s: %w", packagePath, err)
	}

	// Return command and args for go run
	return "go", runArgs, nil
}
