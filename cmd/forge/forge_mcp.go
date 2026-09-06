// Copyright 2024 Alexandre Mahdhaoui
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/alexandremahdhaoui/forge/pkg/forge"
	"github.com/alexandremahdhaoui/forge/pkg/mcpserver"
	"github.com/alexandremahdhaoui/forge/pkg/mcputil"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// cwdMu serializes handlers that use setupCWD with a non-empty cwd.
// os.Chdir and configPath are process-wide global state, so concurrent handlers that
// change the working directory must be serialized.
var cwdMu sync.Mutex

// originalCWD captures the process working directory at startup, before any handler
// changes it. Relative cwd values are resolved against this path.
var originalCWD string

func init() {
	var err error
	originalCWD, err = os.Getwd()
	if err != nil {
		log.Fatalf("Failed to capture original working directory: %v", err)
	}
}

// setupCWD changes the working directory to cwd and resets configPath
// so that loadConfig reads forge.yaml from the new directory. It returns a cleanup
// function that restores the previous CWD and configPath and unlocks the mutex.
//
// If cwd is empty, it returns a no-op cleanup function without acquiring the mutex.
func setupCWD(cwd string) (func(), error) {
	if cwd == "" {
		return func() {}, nil
	}

	// Resolve relative paths against the original CWD
	if !filepath.IsAbs(cwd) {
		cwd = filepath.Join(originalCWD, cwd)
	}

	// Validate directory exists
	if _, err := os.Stat(cwd); err != nil {
		return nil, fmt.Errorf("cwd does not exist: %s", cwd)
	}

	cwdMu.Lock()

	savedCWD, err := os.Getwd()
	if err != nil {
		cwdMu.Unlock()
		return nil, fmt.Errorf("failed to get current working directory: %v", err)
	}
	savedConfigPath := configPath

	if err := os.Chdir(cwd); err != nil {
		cwdMu.Unlock()
		return nil, fmt.Errorf("failed to change to project directory %s: %v", cwd, err)
	}

	// Reset configPath so loadConfig reads forge.yaml from the new CWD
	configPath = ""

	cleanup := func() {
		_ = os.Chdir(savedCWD)
		configPath = savedConfigPath
		cwdMu.Unlock()
	}

	return cleanup, nil
}

// BuildInput represents the input parameters for the build tool.
type BuildInput struct {
	Name         string `json:"name,omitempty" jsonschema:"Build target name from forge.yaml build[].name. Omit to build all targets."`
	ArtifactName string `json:"artifactName,omitempty" jsonschema:"Alternative to name for specifying the build target"`
	Force        bool   `json:"force,omitempty" jsonschema:"Force rebuild even if artifacts are up to date. Passed to engine as force=true."`
	Frozen       bool   `json:"frozen,omitempty" jsonschema:"Build strictly against the recorded dependency lock and never repair it; a stale lock fails the build."`
	// Platforms narrows what builds to these os/arch pairs, the same filter
	// `forge build --platforms` is: an entry builds the platforms it
	// declares, and only those of them named here. Empty builds every
	// declared platform, the host for an entry that declares none.
	Platforms []string `json:"platforms,omitempty" jsonschema:"os/arch pairs to build, a subset of what each entry declares; empty builds every declared platform"`
	CWD       string   `json:"cwd,omitempty" jsonschema:"Absolute or relative path to the project directory containing forge.yaml. Overrides the server working directory."`
}

// BuildGetInput represents the input parameters for the build-get tool.
type BuildGetInput struct {
	Name string `json:"name" jsonschema:"Artifact name to retrieve details for"`
	CWD  string `json:"cwd,omitempty" jsonschema:"Absolute or relative path to the project directory containing forge.yaml. Overrides the server working directory."`
}

// TestCreateInput represents the input parameters for the test-create tool.
type TestCreateInput struct {
	Stage string `json:"stage" jsonschema:"Test stage name from forge.yaml test[].name"`
	CWD   string `json:"cwd,omitempty" jsonschema:"Absolute or relative path to the project directory containing forge.yaml. Overrides the server working directory."`
}

// TestGetInput represents the input parameters for the test-get tool.
type TestGetInput struct {
	Stage  string `json:"stage" jsonschema:"Test stage name from forge.yaml test[].name"`
	TestID string `json:"testID" jsonschema:"Test environment ID returned by test-create or test-run"`
	Format string `json:"format,omitempty" jsonschema:"Output format: json, yaml, or table (default: table)"`
	CWD    string `json:"cwd,omitempty" jsonschema:"Absolute or relative path to the project directory containing forge.yaml. Overrides the server working directory."`
}

// TestDeleteInput represents the input parameters for the test-delete tool.
type TestDeleteInput struct {
	Stage  string `json:"stage" jsonschema:"Test stage name from forge.yaml test[].name"`
	TestID string `json:"testID" jsonschema:"Test environment ID to delete"`
	Force  bool   `json:"force,omitempty" jsonschema:"Also delete the test reports that ran in this environment. Without it the call refuses when such reports exist."`
	CWD    string `json:"cwd,omitempty" jsonschema:"Absolute or relative path to the project directory containing forge.yaml. Overrides the server working directory."`
}

// TestListInput represents the input parameters for the test-list tool.
type TestListInput struct {
	Stage  string `json:"stage" jsonschema:"Test stage name from forge.yaml test[].name"`
	Format string `json:"format,omitempty" jsonschema:"Output format: json, yaml, or table (default: table)"`
	CWD    string `json:"cwd,omitempty" jsonschema:"Absolute or relative path to the project directory containing forge.yaml. Overrides the server working directory."`
}

// TestRunInput represents the input parameters for the test-run tool.
type TestRunInput struct {
	Stage  string `json:"stage" jsonschema:"Test stage name from forge.yaml test[].name"`
	TestID string `json:"testID,omitempty" jsonschema:"Existing test environment ID to reuse. If omitted a new environment is created and cleaned up automatically."`
	CWD    string `json:"cwd,omitempty" jsonschema:"Absolute or relative path to the project directory containing forge.yaml. Overrides the server working directory."`
}

// TestAllInput represents the input parameters for the test-all tool.
type TestAllInput struct {
	Force  bool   `json:"force,omitempty" jsonschema:"Force rebuild of all artifacts before running tests."`
	Frozen bool   `json:"frozen,omitempty" jsonschema:"Build strictly against the recorded dependency lock and never repair it; a stale lock fails the build."`
	CWD    string `json:"cwd,omitempty" jsonschema:"Absolute or relative path to the project directory containing forge.yaml. Overrides the server working directory."`
}

// TestAllResult represents the aggregated results from test-all command.
type TestAllResult struct {
	BuildArtifacts []forge.ArtifactSummary   `json:"buildArtifacts"`
	TestReports    []forge.TestReportSummary `json:"testReports"`
	Summary        string                    `json:"summary"`
	StoppedEarly   bool                      `json:"stoppedEarly"` // True if execution stopped due to failure
}

// BuildResult represents the result of a build operation.
type BuildResult struct {
	Artifacts []forge.ArtifactSummary `json:"artifacts"`
	Summary   string                  `json:"summary"`
}

// TestListResult represents the result of listing test reports.
type TestListResult struct {
	Reports []forge.TestReportSummary `json:"reports"`
	Stage   string                    `json:"stage"`
	Count   int                       `json:"count"`
}

// ConfigValidateInput represents the input parameters for the config-validate tool.
type ConfigValidateInput struct {
	ConfigPath string `json:"configPath,omitempty" jsonschema:"Path to forge.yaml file. Defaults to forge.yaml in the current directory."`
	CWD        string `json:"cwd,omitempty" jsonschema:"Absolute or relative path to the project directory containing forge.yaml. Overrides the server working directory."`
}

// DocsListInput represents the input parameters for the docs-list tool.
type DocsListInput struct {
	// Engine name to list docs for. If empty, lists all engines.
	// Use "all" to list all docs from all engines.
	Engine string `json:"engine,omitempty" jsonschema:"Engine name to list docs for. Omit to list engines. Use 'all' for all docs across engines."`
	CWD    string `json:"cwd,omitempty" jsonschema:"Absolute or relative path to the project directory containing forge.yaml. Overrides the server working directory."`
}

// DocsListResult represents the result of listing docs.
type DocsListResult struct {
	// Engines is populated when listing engines (no engine specified)
	Engines []Engine `json:"engines,omitempty"`
	// Docs is populated when listing docs for an engine or all docs
	Docs []EngineDoc `json:"docs,omitempty"`
	// Engine is the engine filter used (if any)
	Engine string `json:"engine,omitempty"`
	// Summary message
	Summary string `json:"summary"`
}

// DocsGetInput represents the input parameters for the docs-get tool.
type DocsGetInput struct {
	Name string `json:"name" jsonschema:"Documentation name in format 'engine/docname' or 'docname' for global docs"`
	CWD  string `json:"cwd,omitempty" jsonschema:"Absolute or relative path to the project directory containing forge.yaml. Overrides the server working directory."`
}

// ListInput represents the input parameters for the list tool.
type ListInput struct {
	Category string `json:"category,omitempty" jsonschema:"Filter results: 'build' for build targets only, 'test' for test stages only. Omit for both."`
	CWD      string `json:"cwd,omitempty" jsonschema:"Absolute or relative path to the project directory containing forge.yaml. Overrides the server working directory."`
}

// ListResult represents the result of listing targets.
type ListResult struct {
	BuildTargets []string `json:"buildTargets,omitempty"`
	TestStages   []string `json:"testStages,omitempty"`
}

// runMCPServer starts the forge MCP server with stdio transport.
func runMCPServer() error {
	v, _, _ := versionInfo.Get()
	server := mcpserver.New("forge", v)

	// Register build tool
	mcpserver.RegisterTool(server, &mcp.Tool{
		Name:        "build",
		Description: "Build one or all artifacts defined in forge.yaml build[] entries. Without a name parameter, builds all artifacts. Set force=true to rebuild even if artifacts are up to date. Returns lightweight summaries; use build-get for full details including dependencies. Each build[] entry in forge.yaml requires name, src, dest, and engine fields.",
	}, handleBuildTool)

	// Register build-get tool
	mcpserver.RegisterTool(server, &mcp.Tool{
		Name:        "build-get",
		Description: "Get full details of a previously built artifact by name, including dependencies, version, checksum, and timestamps. Use the list tool first to discover available build target names.",
	}, handleBuildGetTool)

	// Register test-create tool
	mcpserver.RegisterTool(server, &mcp.Tool{
		Name:        "test-create",
		Description: "Create a persistent test environment for a stage defined in forge.yaml test[]. The stage must have a testenv engine configured. Returns full environment details including files, metadata, and managed resources. Use test-delete to clean up when done.",
	}, handleTestCreateTool)

	// Register test-get tool
	mcpserver.RegisterTool(server, &mcp.Tool{
		Name:        "test-get",
		Description: "Get full details of a test environment by stage and testID, including artifact files, metadata, managed resources, and environment variables. Use test-list to find available testIDs for a stage.",
	}, handleTestGetTool)

	// Register test-delete tool
	mcpserver.RegisterTool(server, &mcp.Tool{
		Name:        "test-delete",
		Description: "Delete a test environment by stage and testID. Tears down managed resources (clusters, registries) and cleans up temporary files.",
	}, handleTestDeleteTool)

	// Register test-list tool
	mcpserver.RegisterTool(server, &mcp.Tool{
		Name:        "test-list",
		Description: "List test reports for a specific stage defined in forge.yaml test[]. Returns lightweight summaries with id, status, and timing. Use test-get with the stage and testID for full environment details.",
	}, handleTestListTool)

	// Register test-run tool
	mcpserver.RegisterTool(server, &mcp.Tool{
		Name:        "test-run",
		Description: "Run tests for a stage defined in forge.yaml test[]. Auto-creates a test environment if needed and cleans it up after. Optionally pass a testID to run against an existing environment. Returns a full test report with stats, coverage, and error details.",
	}, handleTestRunTool)

	// Register test-all tool
	mcpserver.RegisterTool(server, &mcp.Tool{
		Name:        "test-all",
		Description: "Build all artifacts then run all test stages sequentially as defined in forge.yaml. Set force=true to force rebuild all artifacts before testing. Stops on first failure (fail-fast). Auto-creates and cleans up test environments. Use build-get and test-get for full details on individual results.",
	}, handleTestAllTool)

	// Register config-validate tool
	mcpserver.RegisterTool(server, &mcp.Tool{
		Name:        "config-validate",
		Description: "Validate the forge.yaml configuration file, including all build[], test[], and engine-specific spec sections. Returns structured validation results with per-field error locations.",
	}, handleConfigValidateTool)

	// Register docs-list tool
	mcpserver.RegisterTool(server, &mcp.Tool{
		Name:        "docs-list",
		Description: "List available documentation from forge engines. Without engine parameter: lists engines that have docs. With engine='all': lists all docs across engines. With a specific engine name: lists docs for that engine. Use docs-get with the returned name to retrieve content.",
	}, handleDocsListTool)

	// Register docs-get tool
	mcpserver.RegisterTool(server, &mcp.Tool{
		Name:        "docs-get",
		Description: "Retrieve documentation content by name. Use the format 'engine/docname' for engine-specific docs, or just 'docname' for global forge docs. Use docs-list first to discover available names.",
	}, handleDocsGetTool)

	// Register list tool
	mcpserver.RegisterTool(server, &mcp.Tool{
		Name:        "list",
		Description: "List available build targets and test stages defined in forge.yaml. Optionally filter by category ('build' or 'test'). Use the returned names with build, test-run, and other tools.",
	}, handleListTool)

	// Run the MCP server
	return server.RunDefault()
}

// handleBuildTool handles the "build" tool call from MCP clients.
func handleBuildTool(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input BuildInput,
) (*mcp.CallToolResult, any, error) {
	cleanup, err := setupCWD(input.CWD)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Failed to set project directory: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}
	defer cleanup()

	// Resolve workspace (soft error -- warning only)
	if wsErr := resolveWorkspace(); wsErr != nil {
		log.Printf("Warning: workspace resolution failed: %v", wsErr)
	}

	// Handle legacy ArtifactName field
	name := input.Name
	if name == "" {
		name = input.ArtifactName
	}

	log.Printf("Building artifact: %s", name)

	// Call shared build logic
	// The flag's global, set for this call and cleared after: buildAll reads
	// it the way the CLI sets it, and two MCP builds never overlap because
	// the server handles one call at a time.
	buildPlatforms = input.Platforms
	defer func() { buildPlatforms = nil }()

	buildAllResult, err := buildAll(name, input.Force, input.Frozen)

	// Convert BuildAllResult to MCP response format
	return formatBuildMCPResult(buildAllResult, err)
}

// formatBuildMCPResult converts a BuildAllResult into an MCP response.
func formatBuildMCPResult(buildAllResult *BuildAllResult, buildErr error) (*mcp.CallToolResult, any, error) {
	if buildErr != nil && buildAllResult == nil {
		// Fatal error before any builds started
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Build failed: %v", buildErr)},
			},
			IsError: true,
		}, nil, nil
	}

	// Convert artifacts to lightweight summaries
	var artifactSummaries []forge.ArtifactSummary
	if buildAllResult != nil {
		artifactSummaries = make([]forge.ArtifactSummary, 0, len(buildAllResult.Artifacts))
		for _, a := range buildAllResult.Artifacts {
			artifactSummaries = append(artifactSummaries, a.Summary())
		}
	}

	totalBuilt := 0
	if buildAllResult != nil {
		totalBuilt = buildAllResult.TotalBuilt
	}

	// Create BuildResult wrapper for MCP response
	buildResult := BuildResult{
		Artifacts: artifactSummaries,
		Summary:   fmt.Sprintf("Successfully built %d artifact(s)", totalBuilt),
	}

	if buildErr != nil {
		// Partial error: some builds succeeded, some failed
		buildResult.Summary = fmt.Sprintf("Build completed with errors: %v. Successfully built %d artifact(s)",
			buildAllResult.BuildErrors, totalBuilt)
		result, artifact := mcputil.ErrorResultWithArtifact(buildResult.Summary, buildResult)
		return result, artifact, nil
	}

	// Return all built artifacts wrapped in BuildResult
	result, artifact := mcputil.SuccessResultWithArtifact(buildResult.Summary, buildResult)
	return result, artifact, nil
}

// handleBuildGetTool handles the "build-get" tool call from MCP clients.
func handleBuildGetTool(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input BuildGetInput,
) (*mcp.CallToolResult, any, error) {
	cleanup, err := setupCWD(input.CWD)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Failed to set project directory: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}
	defer cleanup()

	log.Printf("Getting artifact details: %s", input.Name)

	config, err := loadConfig()
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Failed to load configuration: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	store, err := forge.ReadArtifactStore(config.ArtifactStorePath)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Failed to read artifact store: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	artifact, err := forge.GetLatestArtifact(store, input.Name, hostPlatform())
	if err != nil {
		// A generator's or a command's record carries no platform.
		artifact, err = forge.GetLatestArtifact(store, input.Name, "")
	}
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Artifact not found: %s", input.Name)},
			},
			IsError: true,
		}, nil, nil
	}

	result, art := mcputil.SuccessResultWithArtifact(
		fmt.Sprintf("Successfully retrieved artifact: %s", input.Name),
		artifact,
	)
	return result, art, nil
}

// handleTestCreateTool handles the "test-create" tool call from MCP clients.
func handleTestCreateTool(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input TestCreateInput,
) (*mcp.CallToolResult, any, error) {
	cleanup, err := setupCWD(input.CWD)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Failed to set project directory: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}
	defer cleanup()

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
	cleanup, err := setupCWD(input.CWD)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Failed to set project directory: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}
	defer cleanup()

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
	cleanup, err := setupCWD(input.CWD)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Failed to set project directory: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}
	defer cleanup()

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

	if err := testDeleteEnv(testSpec, deleteEnvArgs(input.TestID, input.Force)); err != nil {
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
	cleanup, err := setupCWD(input.CWD)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Failed to set project directory: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}
	defer cleanup()

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

	// Convert to lightweight summaries (use test-get for full details)
	summaries := make([]forge.TestReportSummary, 0, len(reports))
	for _, r := range reports {
		if r != nil {
			summaries = append(summaries, r.Summary())
		}
	}

	// Wrap in TestListResult object
	testListResult := TestListResult{
		Reports: summaries,
		Stage:   input.Stage,
		Count:   len(summaries),
	}

	// Return structured TestListResult object
	result, artifact := mcputil.SuccessResultWithArtifact(
		fmt.Sprintf("Successfully listed %d test report(s) for stage: %s", len(summaries), input.Stage),
		testListResult,
	)
	return result, artifact, nil
}

// handleTestRunTool handles the "test-run" tool call from MCP clients.
func handleTestRunTool(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input TestRunInput,
) (*mcp.CallToolResult, any, error) {
	cleanup, err := setupCWD(input.CWD)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Failed to set project directory: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}
	defer cleanup()

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
	// testRun returns the testID and error, but we don't need the testID for MCP
	_, testRunErr := testRun(&config, testSpec, args)

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
	cleanup, err := setupCWD(input.CWD)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Failed to set project directory: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}
	defer cleanup()

	// Resolve workspace (soft error -- warning only)
	if wsErr := resolveWorkspace(); wsErr != nil {
		log.Printf("Warning: workspace resolution failed: %v", wsErr)
	}

	log.Printf("Running test-all: build all + run all test stages")

	// Call runTestAll
	testAllErr := runTestAll([]string{}, input.Force, input.Frozen)

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

	// Convert build artifacts to lightweight summaries
	buildSummaries := make([]forge.ArtifactSummary, 0, len(store.Artifacts))
	for _, a := range store.Artifacts {
		buildSummaries = append(buildSummaries, a.Summary())
	}

	// Collect test reports ONLY for completed stages (fail-fast behavior)
	// With fail-fast, not all config.Test stages will have run.
	// Sort reports by StartTime descending to pick the most recent for each stage,
	// avoiding stale reports from previous runs.
	var reportSummaries []forge.TestReportSummary
	for _, testSpec := range config.Test {
		reports := forge.ListTestReports(&store, testSpec.Name)
		if len(reports) > 0 {
			sort.Slice(reports, func(i, j int) bool {
				return reports[i].StartTime.After(reports[j].StartTime)
			})
			reportSummaries = append(reportSummaries, reports[0].Summary())
		}
		// If no report exists for this stage, it means execution stopped before this stage
		// due to fail-fast, so we don't include it in results
	}

	// Determine if execution stopped early (fewer reports than total test stages)
	stoppedEarly := len(reportSummaries) < len(config.Test)

	// Create summary
	var summary string
	if stoppedEarly {
		summary = fmt.Sprintf("%d artifact(s) built, %d of %d test stage(s) run (stopped early due to failure)",
			len(buildSummaries), len(reportSummaries), len(config.Test))
	} else {
		summary = fmt.Sprintf("%d artifact(s) built, %d test stage(s) run",
			len(buildSummaries), len(reportSummaries))
	}

	// Count passed/failed test stages
	passedStages := 0
	failedStages := 0
	for _, report := range reportSummaries {
		if report.Status == "passed" {
			passedStages++
		} else {
			failedStages++
		}
	}
	summary += fmt.Sprintf(", %d passed, %d failed", passedStages, failedStages)

	// Create aggregated result
	testAllResult := TestAllResult{
		BuildArtifacts: buildSummaries,
		TestReports:    reportSummaries,
		Summary:        summary,
		StoppedEarly:   stoppedEarly,
	}

	// Determine if we should return error or success.
	//
	// Three cases:
	// 1. failedStages > 0: test stages actually failed — report failures with summary.
	// 2. testAllErr != nil but failedStages == 0: build or infra error occurred but
	//    test reports in the artifact store are stale (from a previous run). Surface
	//    the actual error instead of the misleading "0 failed" summary.
	// 3. Both nil/zero: success.
	if failedStages > 0 {
		result, artifact := mcputil.ErrorResultWithArtifact(
			fmt.Sprintf("Test-all completed with failures: %s", summary),
			testAllResult,
		)
		return result, artifact, nil
	}

	if testAllErr != nil {
		result, artifact := mcputil.ErrorResultWithArtifact(
			fmt.Sprintf("Test-all failed: %v", testAllErr),
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

// handleConfigValidateTool handles the "config-validate" tool call from MCP clients.
// It returns structured ConfigValidateOutput for programmatic use by AI agents and MCP clients.
func handleConfigValidateTool(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input ConfigValidateInput,
) (*mcp.CallToolResult, any, error) {
	cleanup, err := setupCWD(input.CWD)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Failed to set project directory: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}
	defer cleanup()

	cfgPath := input.ConfigPath
	if cfgPath == "" {
		cfgPath = "forge.yaml"
	}

	log.Printf("Validating config: %s", cfgPath)

	// Call validateConfig to get structured output
	output := validateConfig(cfgPath)

	// Return structured output with appropriate success/error status
	if output.Valid {
		result, artifact := mcputil.SuccessResultWithArtifact(
			fmt.Sprintf("Configuration is valid: %s", cfgPath),
			output,
		)
		return result, artifact, nil
	}

	// Validation failed - return with error flag and structured output
	errorCount := len(output.Errors)
	if output.InfraError != "" {
		errorCount = 1 // InfraError counts as one error
	}
	result, artifact := mcputil.ErrorResultWithArtifact(
		fmt.Sprintf("Configuration validation failed with %d error(s)", errorCount),
		output,
	)
	return result, artifact, nil
}

// handleDocsListTool handles the "docs-list" tool call from MCP clients.
// Behavior depends on the engine parameter:
//   - Empty: Lists all engines with doc counts
//   - "all": Lists all docs from all engines
//   - "<engine>": Lists docs for specific engine
func handleDocsListTool(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input DocsListInput,
) (*mcp.CallToolResult, any, error) {
	cleanup, err := setupCWD(input.CWD)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Failed to set project directory: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}
	defer cleanup()

	log.Printf("Listing documentation: engine=%q", input.Engine)

	var result DocsListResult

	switch input.Engine {
	case "":
		// List engines
		engines, err := listEngines()
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: fmt.Sprintf("Failed to list engines: %v", err)},
				},
				IsError: true,
			}, nil, nil
		}
		result.Engines = engines
		result.Summary = fmt.Sprintf("Found %d engine(s) with documentation", len(engines))
	case "all":
		// List all docs from all engines
		docs, err := listAllDocs()
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: fmt.Sprintf("Failed to list all docs: %v", err)},
				},
				IsError: true,
			}, nil, nil
		}
		result.Docs = docs
		result.Summary = fmt.Sprintf("Found %d doc(s) from all engines", len(docs))
	default:
		// List docs for specific engine
		docs, err := listDocsByEngine(input.Engine)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: fmt.Sprintf("Failed to list docs for engine '%s': %v", input.Engine, err)},
				},
				IsError: true,
			}, nil, nil
		}
		result.Docs = docs
		result.Engine = input.Engine
		result.Summary = fmt.Sprintf("Found %d doc(s) for engine '%s'", len(docs), input.Engine)
	}

	// Return structured result with artifact
	mcpResult, artifact := mcputil.SuccessResultWithArtifact(result.Summary, result)
	return mcpResult, artifact, nil
}

// handleDocsGetTool handles the "docs-get" tool call from MCP clients.
// It routes to the correct engine based on the name format:
// - "engine/docname" -> routes to engine-specific docs
// - "docname" -> routes to global forge docs
func handleDocsGetTool(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input DocsGetInput,
) (*mcp.CallToolResult, any, error) {
	cleanup, err := setupCWD(input.CWD)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Failed to set project directory: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}
	defer cleanup()

	log.Printf("Getting documentation: %s", input.Name)

	// Use aggregatedDocsGet for proper routing
	content, err := aggregatedDocsGet(input.Name)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Failed to get documentation: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	// Return the document content
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: content},
		},
	}, nil, nil
}

// handleListTool handles the "list" tool call from MCP clients.
// It returns available build targets and test stages from forge.yaml.
func handleListTool(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input ListInput,
) (*mcp.CallToolResult, any, error) {
	cleanup, err := setupCWD(input.CWD)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Failed to set project directory: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}
	defer cleanup()

	log.Printf("Listing targets: category=%s", input.Category)

	// Validate category if provided
	// Note: In Go, JSON null and empty string both deserialize to ""
	if input.Category != "" && input.Category != "build" && input.Category != "test" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Invalid category: %s (valid: build, test)", input.Category)},
			},
			IsError: true,
		}, nil, nil
	}

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

	// Build result based on category
	result := ListResult{}

	if input.Category == "" || input.Category == "build" {
		result.BuildTargets = make([]string, 0, len(config.Build))
		for _, b := range config.Build {
			result.BuildTargets = append(result.BuildTargets, b.Name)
		}
	}

	if input.Category == "" || input.Category == "test" {
		result.TestStages = make([]string, 0, len(config.Test))
		for _, t := range config.Test {
			result.TestStages = append(result.TestStages, t.Name)
		}
	}

	// Generate summary message
	var msg string
	switch input.Category {
	case "build":
		msg = fmt.Sprintf("Found %d build target(s)", len(result.BuildTargets))
	case "test":
		msg = fmt.Sprintf("Found %d test stage(s)", len(result.TestStages))
	default:
		msg = fmt.Sprintf("Found %d build target(s) and %d test stage(s)", len(result.BuildTargets), len(result.TestStages))
	}

	mcpResult, artifact := mcputil.SuccessResultWithArtifact(msg, result)
	return mcpResult, artifact, nil
}
