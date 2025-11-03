package main

import (
	"context"
	"fmt"
	"log"

	"github.com/alexandremahdhaoui/forge/internal/mcpserver"
	"github.com/alexandremahdhaoui/forge/pkg/forge"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// BuildInput represents the input parameters for the build tool.
type BuildInput struct {
	Name         string `json:"name,omitempty"`
	ArtifactName string `json:"artifactName,omitempty"` // Alternative to Name
}

// runMCPServer starts the forge MCP server with stdio transport.
func runMCPServer() error {
	v, _, _ := versionInfo.Get()
	server := mcpserver.New("forge", v)

	// Register build tool
	mcpserver.RegisterTool(server, &mcp.Tool{
		Name:        "build",
		Description: "Build artifacts from forge.yaml configuration",
	}, handleBuildTool)

	// Run the MCP server
	return server.RunDefault()
}

// handleBuildTool handles the "build" tool call from MCP clients.
func handleBuildTool(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input BuildInput,
) (*mcp.CallToolResult, any, error) {
	artifactName := input.Name
	if artifactName == "" {
		artifactName = input.ArtifactName
	}

	log.Printf("Building artifact: %s", artifactName)

	// Load forge.yaml configuration
	config, err := loadConfig()
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Build failed: could not load forge.yaml: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	// Read artifact store
	store, err := forge.ReadArtifactStore(config.ArtifactStorePath)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Build failed: could not read artifact store: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	// Group specs by engine
	engineSpecs := make(map[string][]map[string]any)

	for _, spec := range config.Build.Specs {
		// Filter by artifact name if provided
		if artifactName != "" && spec.Name != artifactName {
			continue
		}

		params := map[string]any{
			"name":   spec.Name,
			"src":    spec.Src,
			"dest":   spec.Dest,
			"engine": spec.Engine,
		}
		engineSpecs[spec.Engine] = append(engineSpecs[spec.Engine], params)
	}

	if len(engineSpecs) == 0 {
		msg := "No artifacts to build"
		if artifactName != "" {
			msg = fmt.Sprintf("No artifact found with name: %s", artifactName)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: msg},
			},
			IsError: true,
		}, nil, nil
	}

	// Build each group using the appropriate engine
	totalBuilt := 0
	var buildErrors []string

	for engineURI, specs := range engineSpecs {
		// Parse engine URI
		_, binaryPath, err := parseEngine(engineURI)
		if err != nil {
			buildErrors = append(buildErrors, fmt.Sprintf("Failed to parse engine %s: %v", engineURI, err))
			continue
		}

		// Use buildBatch if multiple specs, otherwise use build
		var result interface{}
		if len(specs) == 1 {
			result, err = callMCPEngine(binaryPath, "build", specs[0])
		} else {
			params := map[string]any{
				"specs": specs,
			}
			result, err = callMCPEngine(binaryPath, "buildBatch", params)
		}

		if err != nil {
			buildErrors = append(buildErrors, fmt.Sprintf("Build failed for %s: %v", engineURI, err))
			continue
		}

		// Parse artifacts from result
		artifacts, err := parseArtifacts(result)
		if err == nil {
			// Update artifact store
			for _, artifact := range artifacts {
				forge.AddOrUpdateArtifact(&store, artifact)
				totalBuilt++
			}
		}
	}

	// Write updated artifact store
	if err := forge.WriteArtifactStore(config.ArtifactStorePath, store); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Warning: could not write artifact store: %v", err)},
			},
			IsError: false,
		}, nil, nil
	}

	if len(buildErrors) > 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Build completed with errors: %v. Successfully built %d artifact(s)", buildErrors, totalBuilt)},
			},
			IsError: true,
		}, nil, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("Successfully built %d artifact(s)", totalBuilt)},
		},
	}, nil, nil
}
