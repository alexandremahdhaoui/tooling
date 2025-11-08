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

// TestCreateInput represents the input parameters for the test-create tool.
type TestCreateInput struct {
	Stage string `json:"stage"`
}

// TestGetInput represents the input parameters for the test-get tool.
type TestGetInput struct {
	Stage  string `json:"stage"`
	TestID string `json:"testID"`
	Format string `json:"format,omitempty"` // json, yaml, or table (default)
}

// TestDeleteInput represents the input parameters for the test-delete tool.
type TestDeleteInput struct {
	Stage  string `json:"stage"`
	TestID string `json:"testID"`
}

// TestListInput represents the input parameters for the test-list tool.
type TestListInput struct {
	Stage  string `json:"stage"`
	Format string `json:"format,omitempty"` // json, yaml, or table (default)
}

// TestRunInput represents the input parameters for the test-run tool.
type TestRunInput struct {
	Stage  string `json:"stage"`
	TestID string `json:"testID,omitempty"` // Optional - auto-creates if not provided
}

// TestAllInput represents the input parameters for the test-all tool.
type TestAllInput struct {
	// No parameters - runs all tests
}

// PromptListInput represents the input parameters for the prompt-list tool.
type PromptListInput struct {
	// No parameters - lists all prompts
}

// PromptGetInput represents the input parameters for the prompt-get tool.
type PromptGetInput struct {
	Name string `json:"name"`
}

// ConfigValidateInput represents the input parameters for the config-validate tool.
type ConfigValidateInput struct {
	ConfigPath string `json:"configPath,omitempty"` // Defaults to forge.yaml
}

// DocsListInput represents the input parameters for the docs-list tool.
type DocsListInput struct {
	// No parameters - lists all docs
}

// DocsGetInput represents the input parameters for the docs-get tool.
type DocsGetInput struct {
	Name string `json:"name"`
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

	// Register test-create tool
	mcpserver.RegisterTool(server, &mcp.Tool{
		Name:        "test-create",
		Description: "Create a test environment for a specific test stage",
	}, handleTestCreateTool)

	// Register test-get tool
	mcpserver.RegisterTool(server, &mcp.Tool{
		Name:        "test-get",
		Description: "Get details of a test environment by ID",
	}, handleTestGetTool)

	// Register test-delete tool
	mcpserver.RegisterTool(server, &mcp.Tool{
		Name:        "test-delete",
		Description: "Delete a test environment by ID",
	}, handleTestDeleteTool)

	// Register test-list tool
	mcpserver.RegisterTool(server, &mcp.Tool{
		Name:        "test-list",
		Description: "List all test environments for a specific test stage",
	}, handleTestListTool)

	// Register test-run tool
	mcpserver.RegisterTool(server, &mcp.Tool{
		Name:        "test-run",
		Description: "Run tests for a specific test stage",
	}, handleTestRunTool)

	// Register test-all tool
	mcpserver.RegisterTool(server, &mcp.Tool{
		Name:        "test-all",
		Description: "Build all artifacts and run all test stages sequentially",
	}, handleTestAllTool)

	// Register prompt-list tool
	mcpserver.RegisterTool(server, &mcp.Tool{
		Name:        "prompt-list",
		Description: "List all available documentation prompts",
	}, handlePromptListTool)

	// Register prompt-get tool
	mcpserver.RegisterTool(server, &mcp.Tool{
		Name:        "prompt-get",
		Description: "Get a specific documentation prompt by name",
	}, handlePromptGetTool)

	// Register config-validate tool
	mcpserver.RegisterTool(server, &mcp.Tool{
		Name:        "config-validate",
		Description: "Validate forge.yaml configuration",
	}, handleConfigValidateTool)

	// Register docs-list tool
	mcpserver.RegisterTool(server, &mcp.Tool{
		Name:        "docs-list",
		Description: "List all available documentation",
	}, handleDocsListTool)

	// Register docs-get tool
	mcpserver.RegisterTool(server, &mcp.Tool{
		Name:        "docs-get",
		Description: "Get a specific documentation by name",
	}, handleDocsGetTool)

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

	for _, spec := range config.Build {
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

// handleTestCreateTool handles the "test-create" tool call from MCP clients.
func handleTestCreateTool(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input TestCreateInput,
) (*mcp.CallToolResult, any, error) {
	log.Printf("Creating test environment for stage: %s", input.Stage)

	// Load configuration
	config, err := loadConfig()
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Failed to load configuration: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	// Find test spec
	testSpec := findTestSpec(config.Test, input.Stage)
	if testSpec == nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Test stage not found: %s", input.Stage)},
			},
			IsError: true,
		}, nil, nil
	}

	// Call testCreate
	if err := testCreate(testSpec); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Failed to create test environment: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("Successfully created test environment for stage: %s", input.Stage)},
		},
	}, nil, nil
}

// handleTestGetTool handles the "test-get" tool call from MCP clients.
func handleTestGetTool(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input TestGetInput,
) (*mcp.CallToolResult, any, error) {
	log.Printf("Getting test environment: stage=%s, testID=%s", input.Stage, input.TestID)

	// Load configuration
	config, err := loadConfig()
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Failed to load configuration: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	// Find test spec
	testSpec := findTestSpec(config.Test, input.Stage)
	if testSpec == nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Test stage not found: %s", input.Stage)},
			},
			IsError: true,
		}, nil, nil
	}

	// Build args for testGet
	args := []string{input.TestID}
	if input.Format != "" {
		args = append([]string{"-o", input.Format}, args...)
	}

	// Call testGet (note: it prints to stdout, we'll capture that behavior)
	if err := testGet(testSpec, args); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Failed to get test environment: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("Successfully retrieved test environment: %s", input.TestID)},
		},
	}, nil, nil
}

// handleTestDeleteTool handles the "test-delete" tool call from MCP clients.
func handleTestDeleteTool(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input TestDeleteInput,
) (*mcp.CallToolResult, any, error) {
	log.Printf("Deleting test environment: stage=%s, testID=%s", input.Stage, input.TestID)

	// Load configuration
	config, err := loadConfig()
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Failed to load configuration: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	// Find test spec
	testSpec := findTestSpec(config.Test, input.Stage)
	if testSpec == nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Test stage not found: %s", input.Stage)},
			},
			IsError: true,
		}, nil, nil
	}

	// Call testDelete
	if err := testDelete(testSpec, []string{input.TestID}); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Failed to delete test environment: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("Successfully deleted test environment: %s", input.TestID)},
		},
	}, nil, nil
}

// handleTestListTool handles the "test-list" tool call from MCP clients.
func handleTestListTool(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input TestListInput,
) (*mcp.CallToolResult, any, error) {
	log.Printf("Listing test environments for stage: %s", input.Stage)

	// Load configuration
	config, err := loadConfig()
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Failed to load configuration: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	// Find test spec
	testSpec := findTestSpec(config.Test, input.Stage)
	if testSpec == nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Test stage not found: %s", input.Stage)},
			},
			IsError: true,
		}, nil, nil
	}

	// Build args for testList
	var args []string
	if input.Format != "" {
		args = []string{"-o", input.Format}
	}

	// Call testList (note: it prints to stdout)
	if err := testList(testSpec, args); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Failed to list test environments: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("Successfully listed test environments for stage: %s", input.Stage)},
		},
	}, nil, nil
}

// handleTestRunTool handles the "test-run" tool call from MCP clients.
func handleTestRunTool(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input TestRunInput,
) (*mcp.CallToolResult, any, error) {
	log.Printf("Running tests for stage: %s", input.Stage)

	// Load configuration
	config, err := loadConfig()
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Failed to load configuration: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	// Find test spec
	testSpec := findTestSpec(config.Test, input.Stage)
	if testSpec == nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Test stage not found: %s", input.Stage)},
			},
			IsError: true,
		}, nil, nil
	}

	// Build args for testRun
	var args []string
	if input.TestID != "" {
		args = []string{input.TestID}
	}

	// Call testRun
	if err := testRun(&config, testSpec, args); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Test run failed: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("Successfully ran tests for stage: %s", input.Stage)},
		},
	}, nil, nil
}

// handleTestAllTool handles the "test-all" tool call from MCP clients.
func handleTestAllTool(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input TestAllInput,
) (*mcp.CallToolResult, any, error) {
	log.Printf("Running test-all: build all + run all test stages")

	// Call runTestAll
	if err := runTestAll([]string{}); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Test-all failed: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: "Successfully completed test-all"},
		},
	}, nil, nil
}

// handlePromptListTool handles the "prompt-list" tool call from MCP clients.
func handlePromptListTool(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input PromptListInput,
) (*mcp.CallToolResult, any, error) {
	log.Printf("Listing available prompts")

	// Call promptList (note: it prints to stdout)
	if err := promptList(); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Failed to list prompts: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: "Successfully listed prompts"},
		},
	}, nil, nil
}

// handlePromptGetTool handles the "prompt-get" tool call from MCP clients.
func handlePromptGetTool(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input PromptGetInput,
) (*mcp.CallToolResult, any, error) {
	log.Printf("Getting prompt: %s", input.Name)

	// Call promptGet (note: it prints to stdout)
	if err := promptGet(input.Name); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Failed to get prompt: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("Successfully retrieved prompt: %s", input.Name)},
		},
	}, nil, nil
}

// handleConfigValidateTool handles the "config-validate" tool call from MCP clients.
func handleConfigValidateTool(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input ConfigValidateInput,
) (*mcp.CallToolResult, any, error) {
	configPath := input.ConfigPath
	if configPath == "" {
		configPath = "forge.yaml"
	}

	log.Printf("Validating config: %s", configPath)

	// Call runConfigValidate
	if err := runConfigValidate([]string{configPath}); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Config validation failed: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("Configuration is valid: %s", configPath)},
		},
	}, nil, nil
}

// handleDocsListTool handles the "docs-list" tool call from MCP clients.
func handleDocsListTool(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input DocsListInput,
) (*mcp.CallToolResult, any, error) {
	log.Printf("Listing available documentation")

	// Call docsList (note: it prints to stdout)
	if err := docsList(); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Failed to list documentation: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: "Successfully listed documentation"},
		},
	}, nil, nil
}

// handleDocsGetTool handles the "docs-get" tool call from MCP clients.
func handleDocsGetTool(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input DocsGetInput,
) (*mcp.CallToolResult, any, error) {
	log.Printf("Getting documentation: %s", input.Name)

	// Call docsGet (note: it prints to stdout)
	if err := docsGet(input.Name); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Failed to get documentation: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("Successfully retrieved documentation: %s", input.Name)},
		},
	}, nil, nil
}
