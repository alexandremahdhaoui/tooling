package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/alexandremahdhaoui/forge/internal/cmdutil"
	"github.com/alexandremahdhaoui/forge/internal/mcpserver"
	"github.com/alexandremahdhaoui/forge/pkg/forge"
	"github.com/alexandremahdhaoui/forge/pkg/mcptypes"
	"github.com/alexandremahdhaoui/forge/pkg/mcputil"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// runMCPServer starts the generic-engine MCP server with stdio transport.
func runMCPServer() error {
	v, _, _ := versionInfo.Get()
	server := mcpserver.New("generic-engine", v)

	// Register build tool
	mcpserver.RegisterTool(server, &mcp.Tool{
		Name:        "build",
		Description: "Execute a shell command and return structured output",
	}, handleBuildTool)

	// Run the MCP server
	return server.RunDefault()
}

// handleBuildTool handles the "build" tool call from MCP clients.
func handleBuildTool(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input mcptypes.BuildInput,
) (*mcp.CallToolResult, any, error) {
	log.Printf("Executing command: %s %v (workDir: %s)", input.Command, input.Args, input.WorkDir)

	// Validate inputs
	if result := mcputil.ValidateRequiredWithPrefix("Build failed", map[string]string{
		"name":    input.Name,
		"command": input.Command,
	}); result != nil {
		return result, nil, nil
	}

	// Convert BuildInput to ExecuteInput
	execInput := ExecuteInput{
		Command: input.Command,
		Args:    input.Args,
		Env:     input.Env,
		EnvFile: input.EnvFile,
		WorkDir: input.WorkDir,
	}

	// Execute command
	output := cmdutil.ExecuteCommand(execInput)

	// Check if command failed
	if output.ExitCode != 0 {
		errorMsg := fmt.Sprintf("Command failed with exit code %d\n", output.ExitCode)
		if output.Error != "" {
			errorMsg += fmt.Sprintf("Error: %s\n", output.Error)
		}
		if output.Stderr != "" {
			errorMsg += fmt.Sprintf("Stderr: %s\n", output.Stderr)
		}
		if output.Stdout != "" {
			errorMsg += fmt.Sprintf("Stdout: %s", output.Stdout)
		}

		return mcputil.ErrorResult(errorMsg), nil, nil
	}

	// Determine location (use WorkDir if specified, otherwise Src or ".")
	location := input.WorkDir
	if location == "" {
		location = input.Src
	}
	if location == "" {
		location = "."
	}

	// Create artifact
	artifact := forge.Artifact{
		Name:      input.Name,
		Type:      "command-output",
		Location:  location,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Version:   fmt.Sprintf("%s-exit%d", input.Command, output.ExitCode),
	}

	// Create success message
	successMsg := fmt.Sprintf("Command executed successfully: %s\nExit code: %d", input.Name, output.ExitCode)
	if output.Stdout != "" {
		log.Printf("Stdout: %s", output.Stdout)
	}
	if output.Stderr != "" {
		log.Printf("Stderr: %s", output.Stderr)
	}

	result, returnedArtifact := mcputil.SuccessResultWithArtifact(successMsg, artifact)
	return result, returnedArtifact, nil
}
