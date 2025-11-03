package main

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/alexandremahdhaoui/forge/pkg/forge"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// BuildInput represents the input parameters for the build tool.
type BuildInput struct {
	Name   string `json:"name"`
	Src    string `json:"src"`
	Dest   string `json:"dest,omitempty"`
	Engine string `json:"engine"`
}

// runMCPServer starts the build-container MCP server with stdio transport.
func runMCPServer() error {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "build-container",
		Version: "v1.0.0",
	}, nil)

	// Register build tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "build",
		Description: "Build a container image using Kaniko",
	}, handleBuildTool)

	// Register buildBatch tool
	mcp.AddTool(server, &mcp.Tool{
		Name:        "buildBatch",
		Description: "Build multiple container images using Kaniko",
	}, handleBuildBatchTool)

	ctx := context.Background()
	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
		log.Printf("MCP server failed: %v", err)
		return err
	}

	return nil
}

// handleBuildTool handles the "build" tool call from MCP clients.
func handleBuildTool(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input BuildInput,
) (*mcp.CallToolResult, any, error) {
	log.Printf("Building container: %s from %s", input.Name, input.Src)

	// Validate inputs
	if input.Name == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Build failed: missing required field 'name'"},
			},
			IsError: true,
		}, nil, nil
	}

	if input.Src == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Build failed: missing required field 'src'"},
			},
			IsError: true,
		}, nil, nil
	}

	// Get git version
	version, err := getGitVersionForMCP()
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Build failed: could not get git version: %v", err)},
			},
			IsError: true,
		}, nil, nil
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
	envs := Envs{
		ContainerEngine: "docker",  // Default for MCP mode
		KanikoCacheDir:  "~/.kaniko-cache",
	}

	var dummyStore forge.ArtifactStore
	// Pass isMCPMode=true to suppress stdout output that would corrupt JSON-RPC
	if err := buildContainer(envs, spec, version, timestamp, &dummyStore, true); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Build failed: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	// Create artifact response
	artifact := forge.Artifact{
		Name:      input.Name,
		Type:      "container",
		Location:  fmt.Sprintf("%s:%s", input.Name, version),
		Timestamp: timestamp,
		Version:   version,
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("Built container: %s successfully (version: %s)", input.Name, version)},
		},
	}, artifact, nil
}

// BatchBuildInput represents the input for building multiple containers.
type BatchBuildInput struct {
	Specs []BuildInput `json:"specs"`
}

// handleBuildBatchTool handles batch building of multiple containers.
func handleBuildBatchTool(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input BatchBuildInput,
) (*mcp.CallToolResult, any, error) {
	log.Printf("Building %d containers in batch", len(input.Specs))

	artifacts := []forge.Artifact{}
	errors := []string{}

	for _, spec := range input.Specs {
		result, artifact, err := handleBuildTool(ctx, req, spec)
		if err != nil || (result != nil && result.IsError) {
			errorMsg := "unknown error"
			if err != nil {
				errorMsg = err.Error()
			} else if len(result.Content) > 0 {
				if textContent, ok := result.Content[0].(*mcp.TextContent); ok {
					errorMsg = textContent.Text
				}
			}
			errors = append(errors, fmt.Sprintf("%s: %s", spec.Name, errorMsg))
			continue
		}
		if artifact != nil {
			if art, ok := artifact.(forge.Artifact); ok {
				artifacts = append(artifacts, art)
			}
		}
	}

	if len(errors) > 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Batch build completed with errors: %v", errors)},
			},
			IsError: true,
		}, artifacts, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("Successfully built %d containers", len(artifacts))},
		},
	}, artifacts, nil
}

// getGitVersionForMCP gets the git version for MCP builds.
func getGitVersionForMCP() (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	version := strings.TrimSpace(string(output))
	if version == "" {
		return "", fmt.Errorf("empty git version")
	}

	return version, nil
}
