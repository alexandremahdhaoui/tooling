package main

import (
	"context"
	"fmt"
	"log"

	"github.com/alexandremahdhaoui/forge/internal/mcpserver"
	"github.com/alexandremahdhaoui/forge/pkg/forge"
	"github.com/alexandremahdhaoui/forge/pkg/mcputil"
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

// TestAllResult represents the aggregated results from test-all command.
type TestAllResult struct {
	BuildArtifacts []forge.Artifact   `json:"buildArtifacts"`
	TestReports    []forge.TestReport `json:"testReports"`
	Summary        string             `json:"summary"`
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
	var allArtifacts []forge.Artifact

	for engineURI, specs := range engineSpecs {
		// Parse engine URI
		_, command, args, err := parseEngine(engineURI)
		if err != nil {
			buildErrors = append(buildErrors, fmt.Sprintf("Failed to parse engine %s: %v", engineURI, err))
			continue
		}

		// Use buildBatch if multiple specs, otherwise use build
		var result interface{}
		if len(specs) == 1 {
			result, err = callMCPEngine(command, args, "build", specs[0])
		} else {
			params := map[string]any{
				"specs": specs,
			}
			result, err = callMCPEngine(command, args, "buildBatch", params)
		}

		if err != nil {
			buildErrors = append(buildErrors, fmt.Sprintf("Build failed for %s: %v", engineURI, err))
			continue
		}

		// Parse artifacts from result
		artifacts, err := parseArtifacts(result)
		if err == nil {
			// Update artifact store and collect artifacts
			for _, artifact := range artifacts {
				forge.AddOrUpdateArtifact(&store, artifact)
				allArtifacts = append(allArtifacts, artifact)
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
		// Return with error but include artifacts that were successfully built
		result, artifact := mcputil.ErrorResultWithArtifact(
			fmt.Sprintf("Build completed with errors: %v. Successfully built %d artifact(s)", buildErrors, totalBuilt),
			allArtifacts,
		)
		return result, artifact, nil
	}

	// Return all built artifacts
	result, artifact := mcputil.SuccessResultWithArtifact(
		fmt.Sprintf("Successfully built %d artifact(s)", totalBuilt),
		allArtifacts,
	)
	return result, artifact, nil
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

	// Handle "noop" engine (no environment management)
	if testSpec.Testenv == "" || testSpec.Testenv == "noop" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Test stage %s has no engine configured (engine is 'noop')", testSpec.Name)},
			},
			IsError: true,
		}, nil, nil
	}

	// Resolve engine path
	command, args, err := resolveEngine(testSpec.Testenv, &config)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Failed to resolve engine: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	// Call engine create tool
	result, err := callMCPEngine(command, args, "create", map[string]any{
		"stage": testSpec.Name,
	})
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Failed to create test environment: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	// Extract test ID from result
	var testID string
	if resultMap, ok := result.(map[string]any); ok {
		if id, ok := resultMap["testID"].(string); ok {
			testID = id
		}
	}

	if testID == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Failed to get test ID from engine response"},
			},
			IsError: true,
		}, nil, nil
	}

	// Load artifact store to get the full TestEnvironment
	artifactStorePath, err := forge.GetArtifactStorePath(config.ArtifactStorePath)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Failed to get artifact store path: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	store, err := forge.ReadArtifactStore(artifactStorePath)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Failed to read artifact store: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	// Get the newly created test environment
	env, err := forge.GetTestEnvironment(&store, testID)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Test environment created but not found in artifact store: %s", testID)},
			},
			IsError: true,
		}, nil, nil
	}

	// Return structured TestEnvironment data
	mcpResult, artifact := mcputil.SuccessResultWithArtifact(
		fmt.Sprintf("Successfully created test environment for stage: %s", input.Stage),
		env,
	)
	return mcpResult, artifact, nil
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

	// Load artifact store directly (no stdout printing)
	artifactStorePath, err := forge.GetArtifactStorePath(config.ArtifactStorePath)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Failed to get artifact store path: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	store, err := forge.ReadArtifactStore(artifactStorePath)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Failed to read artifact store: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	// Get test environment
	env, err := forge.GetTestEnvironment(&store, input.TestID)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Test environment not found: %s", input.TestID)},
			},
			IsError: true,
		}, nil, nil
	}

	// Return structured TestEnvironment data
	result, artifact := mcputil.SuccessResultWithArtifact(
		fmt.Sprintf("Successfully retrieved test environment: %s", input.TestID),
		env,
	)
	return result, artifact, nil
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

	// Call testDeleteEnv
	if err := testDeleteEnv(testSpec, []string{input.TestID}); err != nil {
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
// Note: Now lists test REPORTS, not environments (aligned with CLI behavior).
func handleTestListTool(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input TestListInput,
) (*mcp.CallToolResult, any, error) {
	log.Printf("Listing test reports for stage: %s", input.Stage)

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

	// Load artifact store directly (no stdout printing)
	artifactStorePath, err := forge.GetArtifactStorePath(config.ArtifactStorePath)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Failed to get artifact store path: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	store, err := forge.ReadArtifactStore(artifactStorePath)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Failed to read artifact store: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	// List test reports (NOT environments) - aligned with new CLI behavior
	reports := forge.ListTestReports(&store, testSpec.Name)

	// Return structured array of TestReport objects
	result, artifact := mcputil.SuccessResultWithArtifact(
		fmt.Sprintf("Successfully listed %d test report(s) for stage: %s", len(reports), input.Stage),
		reports,
	)
	return result, artifact, nil
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

	// Call testRun - this will execute the tests and store the report
	testRunErr := testRun(&config, testSpec, args)

	// Try to retrieve the most recent test report for this stage from artifact store
	artifactStorePath, err := forge.GetArtifactStorePath(config.ArtifactStorePath)
	if err != nil {
		// If we can't get the report but tests passed, return success without artifact
		if testRunErr == nil {
			result, artifact := mcputil.SuccessResultWithArtifact(
				fmt.Sprintf("Successfully ran tests for stage: %s (test report unavailable)", input.Stage),
				nil,
			)
			return result, artifact, nil
		}
		// If tests failed and we can't get the report, return error
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Test run failed: %v", testRunErr)},
			},
			IsError: true,
		}, nil, nil
	}

	store, err := forge.ReadArtifactStore(artifactStorePath)
	if err != nil {
		// Same fallback logic as above
		if testRunErr == nil {
			result, artifact := mcputil.SuccessResultWithArtifact(
				fmt.Sprintf("Successfully ran tests for stage: %s (test report unavailable)", input.Stage),
				nil,
			)
			return result, artifact, nil
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Test run failed: %v", testRunErr)},
			},
			IsError: true,
		}, nil, nil
	}

	// Get the most recent test report for this stage
	reports := forge.ListTestReports(&store, testSpec.Name)
	var mostRecentReport *forge.TestReport
	if len(reports) > 0 {
		// Reports are sorted by CreatedAt descending, so first one is most recent
		mostRecentReport = reports[0]
	}

	// Determine success/failure and return appropriate result
	if testRunErr != nil {
		// Test run failed
		if mostRecentReport != nil {
			result, artifact := mcputil.ErrorResultWithArtifact(
				fmt.Sprintf("Tests failed for stage: %s", input.Stage),
				mostRecentReport,
			)
			return result, artifact, nil
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Test run failed: %v", testRunErr)},
			},
			IsError: true,
		}, nil, nil
	}

	// Test run succeeded
	if mostRecentReport != nil && mostRecentReport.Status == "failed" {
		// Tests ran but had failures
		result, artifact := mcputil.ErrorResultWithArtifact(
			fmt.Sprintf("Tests failed for stage: %s", input.Stage),
			mostRecentReport,
		)
		return result, artifact, nil
	}

	// Tests passed
	if mostRecentReport != nil {
		result, artifact := mcputil.SuccessResultWithArtifact(
			fmt.Sprintf("Successfully ran tests for stage: %s", input.Stage),
			mostRecentReport,
		)
		return result, artifact, nil
	}

	// Fallback: no report available but tests succeeded
	result, artifact := mcputil.SuccessResultWithArtifact(
		fmt.Sprintf("Successfully ran tests for stage: %s (test report unavailable)", input.Stage),
		nil,
	)
	return result, artifact, nil
}

// handleTestAllTool handles the "test-all" tool call from MCP clients.
func handleTestAllTool(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input TestAllInput,
) (*mcp.CallToolResult, any, error) {
	log.Printf("Running test-all: build all + run all test stages")

	// Call runTestAll
	testAllErr := runTestAll([]string{})

	// Load configuration to get artifact store path
	config, err := loadConfig()
	if err != nil {
		// If we can't load config but test-all succeeded, return success without artifacts
		if testAllErr == nil {
			result, artifact := mcputil.SuccessResultWithArtifact(
				"Successfully completed test-all (results unavailable)",
				nil,
			)
			return result, artifact, nil
		}
		// If test-all failed and we can't load config, return error
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Test-all failed: %v", testAllErr)},
			},
			IsError: true,
		}, nil, nil
	}

	// Read artifact store to get all artifacts and test reports
	artifactStorePath, err := forge.GetArtifactStorePath(config.ArtifactStorePath)
	if err != nil {
		// Same fallback logic
		if testAllErr == nil {
			result, artifact := mcputil.SuccessResultWithArtifact(
				"Successfully completed test-all (results unavailable)",
				nil,
			)
			return result, artifact, nil
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Test-all failed: %v", testAllErr)},
			},
			IsError: true,
		}, nil, nil
	}

	store, err := forge.ReadArtifactStore(artifactStorePath)
	if err != nil {
		// Same fallback logic
		if testAllErr == nil {
			result, artifact := mcputil.SuccessResultWithArtifact(
				"Successfully completed test-all (results unavailable)",
				nil,
			)
			return result, artifact, nil
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Test-all failed: %v", testAllErr)},
			},
			IsError: true,
		}, nil, nil
	}

	// Collect all build artifacts (most recent ones)
	buildArtifacts := store.Artifacts

	// Collect test reports for all test stages
	var testReports []forge.TestReport
	for _, testSpec := range config.Test {
		reports := forge.ListTestReports(&store, testSpec.Name)
		// Get the most recent report for each stage
		if len(reports) > 0 {
			testReports = append(testReports, *reports[0])
		}
	}

	// Create summary
	summary := fmt.Sprintf("%d artifact(s) built, %d test stage(s) run", len(buildArtifacts), len(testReports))

	// Count passed/failed test stages
	passedStages := 0
	failedStages := 0
	for _, report := range testReports {
		if report.Status == "passed" {
			passedStages++
		} else {
			failedStages++
		}
	}
	summary += fmt.Sprintf(", %d passed, %d failed", passedStages, failedStages)

	// Create aggregated result
	testAllResult := TestAllResult{
		BuildArtifacts: buildArtifacts,
		TestReports:    testReports,
		Summary:        summary,
	}

	// Determine if we should return error or success
	if testAllErr != nil || failedStages > 0 {
		result, artifact := mcputil.ErrorResultWithArtifact(
			fmt.Sprintf("Test-all completed with failures: %s", summary),
			testAllResult,
		)
		return result, artifact, nil
	}

	result, artifact := mcputil.SuccessResultWithArtifact(
		fmt.Sprintf("Successfully completed test-all: %s", summary),
		testAllResult,
	)
	return result, artifact, nil
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
