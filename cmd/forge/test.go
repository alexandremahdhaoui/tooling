package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/alexandremahdhaoui/forge/pkg/forge"
)

// runTest handles the "forge test" command.
func runTest(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: forge test <STAGE> <OPERATION> [args...]")
	}

	stage := args[0]

	// Handle special case: forge test <stage> run (no operation, just stage and action)
	// Example: forge test unit run
	// This is a shorthand for running tests
	if len(args) >= 2 && args[1] == "run" {
		// Read config
		config, err := forge.ReadSpec()
		if err != nil {
			return fmt.Errorf("failed to read forge.yaml: %w", err)
		}

		// Find TestSpec for stage
		testSpec := findTestSpec(config.Test, stage)
		if testSpec == nil {
			return fmt.Errorf("test stage not found: %s", stage)
		}

		return testRun(testSpec, args[2:])
	}

	// Handle operation-based commands
	// Example: forge test integration create
	//          forge test integration get <test-id>
	if len(args) < 2 {
		return fmt.Errorf("usage: forge test <STAGE> <OPERATION> [args...]")
	}

	operation := args[1]

	// Read config
	config, err := forge.ReadSpec()
	if err != nil {
		return fmt.Errorf("failed to read forge.yaml: %w", err)
	}

	// Find TestSpec for stage
	testSpec := findTestSpec(config.Test, stage)
	if testSpec == nil {
		return fmt.Errorf("test stage not found: %s", stage)
	}

	// Route to operation
	switch operation {
	case "create":
		return testCreate(testSpec)
	case "get":
		return testGet(args[2:])
	case "delete":
		return testDelete(testSpec, args[2:])
	case "list":
		return testList(stage)
	case "run":
		return testRun(testSpec, args[2:])
	default:
		return fmt.Errorf("unknown operation: %s (valid: create, get, delete, list, run)", operation)
	}
}

// findTestSpec finds a TestSpec by name.
func findTestSpec(specs []forge.TestSpec, name string) *forge.TestSpec {
	for i := range specs {
		if specs[i].Name == name {
			return &specs[i]
		}
	}
	return nil
}

// testCreate creates a test environment via the engine.
func testCreate(testSpec *forge.TestSpec) error {
	// Handle "noop" engine (no environment management)
	if testSpec.Engine == "" || testSpec.Engine == "noop" {
		return fmt.Errorf("test stage %s has no engine configured (engine is 'noop')", testSpec.Name)
	}

	// Parse engine URI and resolve binary
	engineType, enginePath, err := parseEngine(testSpec.Engine)
	if err != nil {
		return fmt.Errorf("failed to parse engine URI: %w", err)
	}

	if engineType != "mcp" {
		return fmt.Errorf("unsupported engine type: %s (only 'mcp' is supported)", engineType)
	}

	// Call engine create tool
	result, err := callMCPEngine(enginePath, "create", map[string]any{
		"stage": testSpec.Name,
	})
	if err != nil {
		return fmt.Errorf("failed to create test environment: %w", err)
	}

	// Extract test ID from result
	var testID string
	if resultMap, ok := result.(map[string]any); ok {
		if id, ok := resultMap["testID"].(string); ok {
			testID = id
		}
	}

	// Output test ID
	if testID != "" {
		fmt.Println(testID)
	}

	return nil
}

// testGet retrieves and displays test environment details.
func testGet(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: forge test <STAGE> get <TEST-ID>")
	}

	testID := args[0]

	// Read config to get artifact store path
	config, err := forge.ReadSpec()
	if err != nil {
		return fmt.Errorf("failed to read forge.yaml: %w", err)
	}

	// Load artifact store
	artifactStorePath := config.ArtifactStorePath
	if artifactStorePath == "" {
		artifactStorePath = ".forge/artifacts.json"
	}

	store, err := forge.ReadArtifactStore(artifactStorePath)
	if err != nil {
		return fmt.Errorf("failed to read artifact store: %w", err)
	}

	// Get test environment
	env, err := forge.GetTestEnvironment(&store, testID)
	if err != nil {
		return fmt.Errorf("test environment not found: %s", testID)
	}

	// Display details (similar to test-integration get)
	fmt.Printf("Test Environment: %s\n", env.ID)
	fmt.Printf("Stage: %s\n", env.Name)
	fmt.Printf("Status: %s\n", env.Status)
	fmt.Printf("Created: %s\n", env.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Updated: %s\n", env.UpdatedAt.Format("2006-01-02 15:04:05"))

	if env.KubeconfigPath != "" {
		fmt.Printf("Kubeconfig: %s\n", env.KubeconfigPath)
	}

	if len(env.RegistryConfig) > 0 {
		fmt.Println("Registry Config:")
		for k, v := range env.RegistryConfig {
			fmt.Printf("  %s: %s\n", k, v)
		}
	}

	return nil
}

// testDelete deletes a test environment via the engine.
func testDelete(testSpec *forge.TestSpec, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: forge test <STAGE> delete <TEST-ID>")
	}

	testID := args[0]

	// Handle noop engine (just remove from artifact store)
	if testSpec.Engine == "" || testSpec.Engine == "noop" {
		config, err := forge.ReadSpec()
		if err != nil {
			return fmt.Errorf("failed to read forge.yaml: %w", err)
		}

		artifactStorePath := config.ArtifactStorePath
		if artifactStorePath == "" {
			artifactStorePath = ".forge/artifacts.json"
		}

		store, err := forge.ReadArtifactStore(artifactStorePath)
		if err != nil {
			return fmt.Errorf("failed to read artifact store: %w", err)
		}

		if err := forge.DeleteTestEnvironment(&store, testID); err != nil {
			return fmt.Errorf("failed to delete test environment: %w", err)
		}

		if err := forge.WriteArtifactStore(artifactStorePath, store); err != nil {
			return fmt.Errorf("failed to write artifact store: %w", err)
		}

		fmt.Printf("Deleted test environment: %s\n", testID)
		return nil
	}

	// Parse engine URI and resolve binary
	engineType, enginePath, err := parseEngine(testSpec.Engine)
	if err != nil {
		return fmt.Errorf("failed to parse engine URI: %w", err)
	}

	if engineType != "mcp" {
		return fmt.Errorf("unsupported engine type: %s", engineType)
	}

	// Call engine delete tool
	_, err = callMCPEngine(enginePath, "delete", map[string]any{
		"testID": testID,
	})
	if err != nil {
		return fmt.Errorf("failed to delete test environment: %w", err)
	}

	fmt.Printf("Deleted test environment: %s\n", testID)
	return nil
}

// testList lists test environments for a stage.
func testList(stage string) error {
	config, err := forge.ReadSpec()
	if err != nil {
		return fmt.Errorf("failed to read forge.yaml: %w", err)
	}

	artifactStorePath := config.ArtifactStorePath
	if artifactStorePath == "" {
		artifactStorePath = ".forge/artifacts.json"
	}

	store, err := forge.ReadArtifactStore(artifactStorePath)
	if err != nil {
		return fmt.Errorf("failed to read artifact store: %w", err)
	}

	// List environments filtered by stage
	envs := forge.ListTestEnvironments(&store, stage)

	if len(envs) == 0 {
		fmt.Printf("No test environments found for stage: %s\n", stage)
		return nil
	}

	// Display as table
	fmt.Printf("Test environments for stage: %s\n\n", stage)
	fmt.Printf("%-40s  %-10s  %-20s\n", "ID", "STATUS", "CREATED")
	fmt.Printf("%s  %s  %s\n", strings.Repeat("-", 40), strings.Repeat("-", 10), strings.Repeat("-", 20))
	for _, env := range envs {
		fmt.Printf("%-40s  %-10s  %-20s\n",
			env.ID,
			env.Status,
			env.CreatedAt.Format("2006-01-02 15:04"),
		)
	}

	return nil
}

// testRun runs tests via the test runner.
func testRun(testSpec *forge.TestSpec, args []string) error {
	var testID string

	// If test ID provided, use it; otherwise auto-create environment
	if len(args) > 0 {
		testID = args[0]
	} else {
		// Auto-create environment if engine is configured
		if testSpec.Engine != "" && testSpec.Engine != "noop" {
			// Parse engine and create environment
			engineType, enginePath, err := parseEngine(testSpec.Engine)
			if err != nil {
				return fmt.Errorf("failed to parse engine URI: %w", err)
			}

			if engineType == "mcp" {
				result, err := callMCPEngine(enginePath, "create", map[string]any{
					"stage": testSpec.Name,
				})
				if err != nil {
					return fmt.Errorf("failed to create test environment: %w", err)
				}

				// Extract test ID
				if resultMap, ok := result.(map[string]any); ok {
					if id, ok := resultMap["testID"].(string); ok {
						testID = id
						fmt.Printf("Created test environment: %s\n", testID)
					}
				}
			}
		}
	}

	// Parse runner URI
	if testSpec.Runner == "" {
		return fmt.Errorf("no test runner configured for stage: %s", testSpec.Name)
	}

	runnerType, runnerPath, err := parseEngine(testSpec.Runner)
	if err != nil {
		return fmt.Errorf("failed to parse runner URI: %w", err)
	}

	if runnerType != "mcp" {
		return fmt.Errorf("unsupported runner type: %s", runnerType)
	}

	// Generate test name
	testName := fmt.Sprintf("%s-%s", testSpec.Name, time.Now().Format("20060102-150405"))

	// Call runner
	fmt.Printf("Running tests: stage=%s, name=%s\n", testSpec.Name, testName)

	result, err := callMCPEngine(runnerPath, "run", map[string]any{
		"stage": testSpec.Name,
		"name":  testName,
	})
	if err != nil {
		// Update test environment status to failed if we have a test ID
		if testID != "" {
			updateTestStatus(testID, "failed")
		}
		return fmt.Errorf("test run failed: %w", err)
	}

	// Update test environment status based on result
	if testID != "" {
		// Extract status from result
		status := "passed"
		if resultMap, ok := result.(map[string]any); ok {
			if s, ok := resultMap["status"].(string); ok {
				status = s
			}
		}
		updateTestStatus(testID, status)
	}

	// Display test report summary
	fmt.Println("\nTest Results:")
	if resultMap, ok := result.(map[string]any); ok {
		if status, ok := resultMap["status"].(string); ok {
			fmt.Printf("Status: %s\n", status)
		}
		if stats, ok := resultMap["testStats"].(map[string]any); ok {
			if total, ok := stats["total"].(float64); ok {
				fmt.Printf("Total: %.0f\n", total)
			}
			if passed, ok := stats["passed"].(float64); ok {
				fmt.Printf("Passed: %.0f\n", passed)
			}
			if failed, ok := stats["failed"].(float64); ok {
				fmt.Printf("Failed: %.0f\n", failed)
			}
		}
		if coverage, ok := resultMap["coverage"].(map[string]any); ok {
			if pct, ok := coverage["percentage"].(float64); ok {
				fmt.Printf("Coverage: %.1f%%\n", pct)
			}
		}
	}

	return nil
}

// updateTestStatus updates the status of a test environment in the artifact store.
func updateTestStatus(testID, status string) {
	config, err := forge.ReadSpec()
	if err != nil {
		return
	}

	artifactStorePath := config.ArtifactStorePath
	if artifactStorePath == "" {
		artifactStorePath = ".forge/artifacts.json"
	}

	store, err := forge.ReadArtifactStore(artifactStorePath)
	if err != nil {
		return
	}

	env, err := forge.GetTestEnvironment(&store, testID)
	if err != nil {
		return
	}

	env.Status = status
	env.UpdatedAt = time.Now().UTC()

	forge.AddOrUpdateTestEnvironment(&store, env)
	forge.WriteArtifactStore(artifactStorePath, store)
}
