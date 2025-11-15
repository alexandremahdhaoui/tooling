package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/alexandremahdhaoui/forge/internal/gitutil"
	"github.com/alexandremahdhaoui/forge/internal/mcpserver"
	"github.com/alexandremahdhaoui/forge/pkg/forge"
	"github.com/alexandremahdhaoui/forge/pkg/mcptypes"
	"github.com/alexandremahdhaoui/forge/pkg/mcputil"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// runMCPServer starts the build-go MCP server with stdio transport.
// It creates an MCP server, registers tools, and runs the server until stdin closes.
func runMCPServer() error {
	server := mcpserver.New(Name, Version)

	// Register build tool
	mcpserver.RegisterTool(server, &mcp.Tool{
		Name:        "build",
		Description: "Build a single Go binary from source",
	}, handleBuildTool)

	// Register buildBatch tool
	mcpserver.RegisterTool(server, &mcp.Tool{
		Name:        "buildBatch",
		Description: "Build multiple Go binaries from source",
	}, handleBuildBatchTool)

	// Run the MCP server
	return server.RunDefault()
}

// handleBuildTool handles the "build" tool call from MCP clients.
func handleBuildTool(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input mcptypes.BuildInput,
) (*mcp.CallToolResult, any, error) {
	log.Printf("Building binary: %s from %s", input.Name, input.Src)

	// Validate inputs
	if result := mcputil.ValidateRequiredWithPrefix("Build failed", map[string]string{
		"name": input.Name,
		"src":  input.Src,
	}); result != nil {
		return result, nil, nil
	}

	// Get git version
	version, err := gitutil.GetCurrentCommitSHA()
	if err != nil {
		return mcputil.ErrorResult(fmt.Sprintf("Build failed: could not get git version: %v", err)), nil, nil
	}

	// Create BuildSpec from input
	spec := forge.BuildSpec{
		Name:   input.Name,
		Src:    input.Src,
		Dest:   input.Dest,
		Engine: input.Engine,
	}

	// Build the binary
	timestamp := time.Now().UTC().Format(time.RFC3339)
	envs := Envs{} // Use default (empty) environment

	// We don't have artifact store in MCP mode, pass nil
	var dummyStore forge.ArtifactStore

	// Note: Pass isMCPMode=true to suppress stdout output that would corrupt JSON-RPC
	if err := buildBinary(envs, spec, version, timestamp, &dummyStore, true); err != nil {
		return mcputil.ErrorResult(fmt.Sprintf("Build failed: %v", err)), nil, nil
	}

	// Determine final location
	dest := spec.Dest
	if dest == "" {
		dest = "./build/bin"
	}

	// Create artifact response
	artifact := forge.Artifact{
		Name:      input.Name,
		Type:      "binary",
		Location:  fmt.Sprintf("%s/%s", dest, input.Name),
		Timestamp: timestamp,
		Version:   version,
	}

	result, returnedArtifact := mcputil.SuccessResultWithArtifact(
		fmt.Sprintf("Built binary: %s successfully (version: %s)", input.Name, version),
		artifact,
	)
	return result, returnedArtifact, nil
}

// handleBuildBatchTool handles batch building of multiple binaries.
func handleBuildBatchTool(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input mcptypes.BatchBuildInput,
) (*mcp.CallToolResult, any, error) {
	log.Printf("Building %d binaries in batch", len(input.Specs))

	// Use generic batch handler
	artifacts, errorMsgs := mcputil.HandleBatchBuild(ctx, input.Specs, func(ctx context.Context, spec mcptypes.BuildInput) (*mcp.CallToolResult, any, error) {
		return handleBuildTool(ctx, req, spec)
	})

	// Format the result
	result, returnedArtifacts := mcputil.FormatBatchResult("binaries", artifacts, errorMsgs)
	return result, returnedArtifacts, nil
}
