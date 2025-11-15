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
	"github.com/caarlos0/env/v11"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// runMCPServer starts the container-build MCP server with stdio transport.
func runMCPServer() error {
	server := mcpserver.New(Name, Version)

	// Register build tool
	mcpserver.RegisterTool(server, &mcp.Tool{
		Name:        "build",
		Description: "Build a container image using docker, kaniko, or podman (set CONTAINER_BUILD_ENGINE)",
	}, handleBuildTool)

	// Register buildBatch tool
	mcpserver.RegisterTool(server, &mcp.Tool{
		Name:        "buildBatch",
		Description: "Build multiple container images using docker, kaniko, or podman (set CONTAINER_BUILD_ENGINE)",
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
	log.Printf("Building container: %s from %s", input.Name, input.Src)

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

	// Build the container
	timestamp := time.Now().UTC().Format(time.RFC3339)

	// Parse environment variables (CONTAINER_BUILD_ENGINE is required)
	envs := Envs{} //nolint:exhaustruct
	if err := env.Parse(&envs); err != nil {
		return mcputil.ErrorResult(fmt.Sprintf("Build failed: %v (CONTAINER_BUILD_ENGINE required)", err)), nil, nil
	}

	// Validate container engine
	if err := validateContainerEngine(envs.BuildEngine); err != nil {
		return mcputil.ErrorResult(fmt.Sprintf("Build failed: %v", err)), nil, nil
	}

	var dummyStore forge.ArtifactStore
	// Pass isMCPMode=true to suppress stdout output that would corrupt JSON-RPC
	if err := buildContainer(envs, spec, version, timestamp, &dummyStore, true); err != nil {
		return mcputil.ErrorResult(fmt.Sprintf("Build failed: %v", err)), nil, nil
	}

	// Create artifact response
	artifact := forge.Artifact{
		Name:      input.Name,
		Type:      "container",
		Location:  fmt.Sprintf("%s:%s", input.Name, version),
		Timestamp: timestamp,
		Version:   version,
	}

	result, returnedArtifact := mcputil.SuccessResultWithArtifact(
		fmt.Sprintf("Built container: %s successfully (version: %s)", input.Name, version),
		artifact,
	)
	return result, returnedArtifact, nil
}

// handleBuildBatchTool handles batch building of multiple containers.
func handleBuildBatchTool(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input mcptypes.BatchBuildInput,
) (*mcp.CallToolResult, any, error) {
	log.Printf("Building %d containers in batch", len(input.Specs))

	// Use generic batch handler
	artifacts, errorMsgs := mcputil.HandleBatchBuild(ctx, input.Specs, func(ctx context.Context, spec mcptypes.BuildInput) (*mcp.CallToolResult, any, error) {
		return handleBuildTool(ctx, req, spec)
	})

	// Format the result
	result, returnedArtifacts := mcputil.FormatBatchResult("containers", artifacts, errorMsgs)
	return result, returnedArtifacts, nil
}
