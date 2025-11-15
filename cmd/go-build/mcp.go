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

// runMCPServer starts the go-build MCP server with stdio transport.
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

	// Extract build options from input
	opts := extractBuildOptionsFromInput(input)

	// Build the binary
	timestamp := time.Now().UTC().Format(time.RFC3339)
	envs := Envs{} // Use default (empty) environment

	// We don't have artifact store in MCP mode, pass nil
	var dummyStore forge.ArtifactStore

	// Note: Pass isMCPMode=true to suppress stdout output that would corrupt JSON-RPC
	if err := buildBinary(envs, spec, version, timestamp, &dummyStore, true, opts); err != nil {
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

// extractBuildOptionsFromInput extracts BuildOptions from BuildInput fields.
// It first checks the Spec field (from forge.yaml BuildSpec.Spec), then falls back to direct Args/Env fields.
// Direct Args/Env fields take precedence over Spec if both are present.
func extractBuildOptionsFromInput(input mcptypes.BuildInput) *BuildOptions {
	opts := &BuildOptions{}

	// First, try to extract from Spec field (from BuildSpec.Spec in forge.yaml)
	if len(input.Spec) > 0 {
		// Extract args from spec
		if argsVal, ok := input.Spec["args"]; ok {
			if args, ok := argsVal.([]interface{}); ok {
				opts.CustomArgs = make([]string, 0, len(args))
				for _, arg := range args {
					if argStr, ok := arg.(string); ok {
						opts.CustomArgs = append(opts.CustomArgs, argStr)
					}
				}
			}
		}

		// Extract env from spec
		if envVal, ok := input.Spec["env"]; ok {
			if env, ok := envVal.(map[string]interface{}); ok {
				opts.CustomEnv = make(map[string]string, len(env))
				for key, val := range env {
					if valStr, ok := val.(string); ok {
						opts.CustomEnv[key] = valStr
					}
				}
			}
		}
	}

	// Direct Args/Env fields take precedence over Spec
	if len(input.Args) > 0 {
		opts.CustomArgs = input.Args
	}

	if len(input.Env) > 0 {
		opts.CustomEnv = input.Env
	}

	// Return nil if no options were extracted
	if len(opts.CustomArgs) == 0 && len(opts.CustomEnv) == 0 {
		return nil
	}

	return opts
}
