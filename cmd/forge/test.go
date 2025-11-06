package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/alexandremahdhaoui/forge/pkg/forge"
	"github.com/google/uuid"
	"sigs.k8s.io/yaml"
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

		return testRun(&config, testSpec, args[2:])
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
		return testGet(testSpec, args[2:])
	case "delete":
		return testDelete(testSpec, args[2:])
	case "list":
		return testList(testSpec, args[2:])
	case "run":
		return testRun(&config, testSpec, args[2:])
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
	if testSpec.Testenv == "" || testSpec.Testenv == "noop" {
		return fmt.Errorf("test stage %s has no engine configured (engine is 'noop')", testSpec.Name)
	}

	// Load config and resolve engine path
	config, err := forge.ReadSpec()
	if err != nil {
		return fmt.Errorf("failed to read forge.yaml: %w", err)
	}

	enginePath, err := resolveEngine(testSpec.Testenv, &config)
	if err != nil {
		return fmt.Errorf("failed to resolve engine: %w", err)
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

// testGet retrieves and displays test environment/report details from the artifact store.
func testGet(testSpec *forge.TestSpec, args []string) error {
	// Parse output format flag
	format, remainingArgs := parseOutputFormat(args)

	if len(remainingArgs) < 1 {
		return fmt.Errorf("usage: forge test <STAGE> get <TEST-ID> [-o json|yaml]")
	}

	testID := remainingArgs[0]

	// Read forge configuration to get artifact store path
	config, err := forge.ReadSpec()
	if err != nil {
		return fmt.Errorf("failed to read forge.yaml: %w", err)
	}

	// Read artifact store DIRECTLY - NO MCP
	artifactStorePath, err := forge.GetArtifactStorePath(config.ArtifactStorePath)
	if err != nil {
		return fmt.Errorf("failed to get artifact store path: %w", err)
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

	// Convert to map for display
	resultMap := testEnvironmentToMap(env)

	// Print result in requested format
	switch format {
	case outputFormatJSON:
		printJSON(resultMap)
	case outputFormatYAML:
		printYAML(resultMap)
	default:
		printTestEnvironmentTable(resultMap)
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
	if testSpec.Testenv == "" || testSpec.Testenv == "noop" {
		artifactStorePath, err := forge.GetArtifactStorePath(".forge/artifacts.json")
		if err != nil {
			return fmt.Errorf("failed to get artifact store path: %w", err)
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

	// Resolve engine path
	config, err := forge.ReadSpec()
	if err != nil {
		return fmt.Errorf("failed to read forge.yaml: %w", err)
	}

	enginePath, err := resolveEngine(testSpec.Testenv, &config)
	if err != nil {
		return fmt.Errorf("failed to resolve engine: %w", err)
	}

	// Determine the parameter name based on engine
	paramName := "testID"
	if strings.Contains(testSpec.Testenv, "test-report") {
		paramName = "reportID"
	}

	// Call engine delete tool
	result, err := callMCPEngine(enginePath, "delete", map[string]any{
		paramName: testID,
	})
	if err != nil {
		return fmt.Errorf("failed to delete: %w", err)
	}

	// Print result (may contain deletion details)
	if resultMap, ok := result.(map[string]any); ok {
		if success, ok := resultMap["success"].(bool); ok && success {
			fmt.Printf("Successfully deleted: %s\n", testID)
		} else {
			printJSON(resultMap)
		}
	} else {
		fmt.Printf("Deleted: %s\n", testID)
	}

	return nil
}

// testList lists test environments/reports for a stage by calling the engine.
func testList(testSpec *forge.TestSpec, args []string) error {
	// Parse output format flag
	format, _ := parseOutputFormat(args)

	config, err := forge.ReadSpec()
	if err != nil {
		return fmt.Errorf("failed to read forge.yaml: %w", err)
	}

	// Read artifact store DIRECTLY - NO MCP
	artifactStorePath, err := forge.GetArtifactStorePath(config.ArtifactStorePath)
	if err != nil {
		return fmt.Errorf("failed to get artifact store path: %w", err)
	}

	store, err := forge.ReadArtifactStore(artifactStorePath)
	if err != nil {
		return fmt.Errorf("failed to read artifact store: %w", err)
	}

	// List test environments (filter by stage name)
	envs := forge.ListTestEnvironments(&store, testSpec.Name)

	// Print result in requested format
	if len(envs) == 0 {
		fmt.Printf("No test environments found for stage: %s\n", testSpec.Name)
		return nil
	}

	// Convert to slice of maps for display
	resultSlice := make([]any, len(envs))
	for i, env := range envs {
		resultSlice[i] = testEnvironmentToMap(env)
	}

	switch format {
	case outputFormatJSON:
		printJSON(resultSlice)
	case outputFormatYAML:
		printYAML(resultSlice)
	default:
		printTestEnvironmentsTable(resultSlice)
	}

	return nil
}

// testRun runs tests via the test runner.
func testRun(config *forge.Spec, testSpec *forge.TestSpec, args []string) error {
	var testID string

	// If test ID provided, use it; otherwise auto-create environment
	if len(args) > 0 {
		testID = args[0]
	} else {
		// Auto-create environment if engine is configured
		if testSpec.Testenv != "" && testSpec.Testenv != "noop" {
			// Parse engine and create environment
			engineType, enginePath, err := parseEngine(testSpec.Testenv)
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

	// Resolve runner URI (handles aliases)
	runnerPath, err := resolveEngine(testSpec.Runner, config)
	if err != nil {
		return fmt.Errorf("failed to resolve runner %s: %w", testSpec.Runner, err)
	}

	// Get runner config if this is an alias
	var runnerConfig *forge.EngineConfig
	if strings.HasPrefix(testSpec.Runner, "alias://") {
		aliasName := strings.TrimPrefix(testSpec.Runner, "alias://")
		runnerConfig = getEngineConfig(aliasName, config)
	}

	// Generate test name
	testName := fmt.Sprintf("%s-%s", testSpec.Name, time.Now().Format("20060102-150405"))

	// Create forge directories for this run
	dirs, err := createForgeDirs()
	if err != nil {
		return fmt.Errorf("failed to create forge directories: %w", err)
	}

	// Clean up old tmp directories (keep last 10 runs)
	if err := cleanupOldTmpDirs(10); err != nil {
		// Log warning but don't fail
		fmt.Fprintf(os.Stderr, "Warning: failed to cleanup old tmp directories: %v\n", err)
	}

	// Call runner
	fmt.Printf("Running tests: stage=%s, name=%s\n", testSpec.Name, testName)

	// Generate report ID
	reportID := uuid.New().String()

	// Prepare MCP parameters with directory paths
	params := map[string]any{
		"id":       reportID,
		"stage":    testSpec.Name,
		"name":     testName,
		"tmpDir":   dirs.TmpDir,
		"buildDir": dirs.BuildDir,
		"rootDir":  dirs.RootDir,
	}

	// If testID is provided, get artifact files from test environment
	if testID != "" {
		artifactStorePath, err := forge.GetArtifactStorePath(config.ArtifactStorePath)
		if err != nil {
			return fmt.Errorf("failed to get artifact store path: %w", err)
		}

		store, err := forge.ReadArtifactStore(artifactStorePath)
		if err != nil {
			return fmt.Errorf("failed to read artifact store: %w", err)
		}

		env, err := forge.GetTestEnvironment(&store, testID)
		if err != nil {
			return fmt.Errorf("test environment not found: %s", testID)
		}

		// Pass artifact files (relative paths)
		if len(env.Files) > 0 {
			params["artifactFiles"] = env.Files
		}
		// Pass testenv tmpDir so runner can construct full paths
		if env.TmpDir != "" {
			params["testenvTmpDir"] = env.TmpDir
		}
		// Pass testenv metadata
		if len(env.Metadata) > 0 {
			params["testenvMetadata"] = env.Metadata
		}
	}

	// Inject runner config if present (for alias:// runners)
	if runnerConfig != nil && runnerConfig.Config.Command != "" {
		if runnerConfig.Config.Command != "" {
			params["command"] = runnerConfig.Config.Command
		}
		if len(runnerConfig.Config.Args) > 0 {
			params["args"] = runnerConfig.Config.Args
		}
		if len(runnerConfig.Config.Env) > 0 {
			params["env"] = runnerConfig.Config.Env
		}
		if runnerConfig.Config.EnvFile != "" {
			params["envFile"] = runnerConfig.Config.EnvFile
		}
		if runnerConfig.Config.WorkDir != "" {
			params["workDir"] = runnerConfig.Config.WorkDir
		}
	}

	result, err := callMCPEngine(runnerPath, "run", params)
	if err != nil {
		// Update test environment status to failed if we have a test ID
		if testID != "" {
			updateTestStatus(testID, "failed")
		}
		return fmt.Errorf("test run failed: %w", err)
	}

	// Store test report in artifact store
	if err := storeTestReportFromResult(result, config); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to store test report: %v\n", err)
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

// storeTestReportFromResult stores a test report from MCP engine result.
func storeTestReportFromResult(result any, config *forge.Spec) error {
	resultMap, ok := result.(map[string]any)
	if !ok {
		return fmt.Errorf("result is not a map")
	}

	// Convert map to TestReport
	reportJSON, err := json.Marshal(resultMap)
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}

	var report forge.TestReport
	if err := json.Unmarshal(reportJSON, &report); err != nil {
		return fmt.Errorf("failed to unmarshal test report: %w", err)
	}

	// Get artifact store path
	artifactStorePath := config.ArtifactStorePath
	if artifactStorePath == "" {
		artifactStorePath = ".forge/artifacts.yaml"
	}

	// Read or create artifact store
	store, err := forge.ReadOrCreateArtifactStore(artifactStorePath)
	if err != nil {
		return fmt.Errorf("failed to read artifact store: %w", err)
	}

	// Add or update test report
	forge.AddOrUpdateTestReport(&store, &report)

	// Write artifact store
	if err := forge.WriteArtifactStore(artifactStorePath, store); err != nil {
		return fmt.Errorf("failed to write artifact store: %w", err)
	}

	return nil
}

// updateTestStatus updates the status of a test environment in the artifact store.
func updateTestStatus(testID, status string) {
	artifactStorePath, err := forge.GetArtifactStorePath(".forge/artifacts.json")
	if err != nil {
		return
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

// outputFormat represents the desired output format
type outputFormat string

const (
	outputFormatTable outputFormat = "table"
	outputFormatJSON  outputFormat = "json"
	outputFormatYAML  outputFormat = "yaml"
)

// parseOutputFormat extracts the output format flag from args
// Supports: -o json, -ojson, -o yaml, -oyaml
// Returns: format, remaining args
func parseOutputFormat(args []string) (outputFormat, []string) {
	format := outputFormatTable // default
	remaining := make([]string, 0, len(args))

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-o" && i+1 < len(args) {
			// -o json or -o yaml
			switch args[i+1] {
			case "json":
				format = outputFormatJSON
			case "yaml":
				format = outputFormatYAML
			default:
				remaining = append(remaining, arg)
				continue
			}
			i++ // skip next arg
		} else if arg == "-ojson" {
			format = outputFormatJSON
		} else if arg == "-oyaml" {
			format = outputFormatYAML
		} else {
			remaining = append(remaining, arg)
		}
	}

	return format, remaining
}

// printJSON prints a value as formatted JSON to stdout.
func printJSON(v any) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(v); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
	}
}

// printYAML prints a value as YAML to stdout.
func printYAML(v any) {
	data, err := yaml.Marshal(v)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding YAML: %v\n", err)
		return
	}
	fmt.Print(string(data))
}

// printTestReportsTable prints test reports as a table.
func printTestReportsTable(reports []any) {
	if len(reports) == 0 {
		return
	}

	// Print header
	fmt.Printf("%-36s  %-10s  %-20s  %-10s\n", "ID", "STATUS", "CREATED", "COVERAGE")
	fmt.Printf("%s  %s  %s  %s\n",
		strings.Repeat("-", 36),
		strings.Repeat("-", 10),
		strings.Repeat("-", 20),
		strings.Repeat("-", 10))

	// Print rows
	for _, r := range reports {
		report, ok := r.(map[string]any)
		if !ok {
			continue
		}

		id := getStringField(report, "id")
		status := getStringField(report, "status")

		// Get created timestamp
		createdStr := "-"
		if createdAt, ok := report["createdAt"].(string); ok {
			if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
				createdStr = t.Format("2006-01-02 15:04:05")
			}
		}

		// Get coverage percentage
		coverageStr := "-"
		if coverage, ok := report["coverage"].(map[string]any); ok {
			if pct, ok := coverage["percentage"].(float64); ok {
				coverageStr = fmt.Sprintf("%.1f%%", pct)
			}
		}

		fmt.Printf("%-36s  %-10s  %-20s  %-10s\n",
			id,
			truncate(status, 10),
			createdStr,
			coverageStr)
	}
}

// printTestReportTable prints a single test report as a table.
func printTestReportTable(report map[string]any) {
	fmt.Printf("Test Report: %s\n", getStringField(report, "id"))
	fmt.Printf("Stage:       %s\n", getStringField(report, "stage"))
	fmt.Printf("Status:      %s\n", getStringField(report, "status"))

	if createdAt, ok := report["createdAt"].(string); ok {
		if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
			fmt.Printf("Created:     %s\n", t.Format("2006-01-02 15:04:05"))
		}
	}

	if duration, ok := report["duration"].(float64); ok {
		fmt.Printf("Duration:    %.2fs\n", duration)
	}

	if stats, ok := report["testStats"].(map[string]any); ok {
		if total, ok := stats["total"].(float64); ok {
			fmt.Printf("Tests:       %.0f total", total)
			if passed, ok := stats["passed"].(float64); ok {
				fmt.Printf(", %.0f passed", passed)
			}
			if failed, ok := stats["failed"].(float64); ok {
				fmt.Printf(", %.0f failed", failed)
			}
			fmt.Println()
		}
	}

	if coverage, ok := report["coverage"].(map[string]any); ok {
		if pct, ok := coverage["percentage"].(float64); ok {
			fmt.Printf("Coverage:    %.1f%%\n", pct)
		}
	}

	if outputPath, ok := report["outputPath"].(string); ok && outputPath != "" {
		fmt.Printf("Output:      %s\n", outputPath)
	}
}

// getStringField safely gets a string field from a map.
func getStringField(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// truncate truncates a string to the specified length.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// testEnvironmentToMap converts a TestEnvironment to a map for display.
func testEnvironmentToMap(env *forge.TestEnvironment) map[string]any {
	return map[string]any{
		"id":               env.ID,
		"name":             env.Name,
		"status":           env.Status,
		"createdAt":        env.CreatedAt.Format(time.RFC3339),
		"updatedAt":        env.UpdatedAt.Format(time.RFC3339),
		"tmpDir":           env.TmpDir,
		"files":            env.Files,
		"managedResources": env.ManagedResources,
		"metadata":         env.Metadata,
	}
}

// printTestEnvironmentTable prints a test environment in table format.
func printTestEnvironmentTable(env map[string]any) {
	fmt.Println("\n=== Test Environment ===")
	fmt.Printf("ID:          %s\n", getStringField(env, "id"))
	fmt.Printf("Name:        %s\n", getStringField(env, "name"))
	fmt.Printf("Status:      %s\n", getStringField(env, "status"))
	fmt.Printf("Created:     %s\n", getStringField(env, "createdAt"))
	fmt.Printf("Updated:     %s\n", getStringField(env, "updatedAt"))
	fmt.Printf("TmpDir:      %s\n", getStringField(env, "tmpDir"))

	if files, ok := env["files"].(map[string]string); ok && len(files) > 0 {
		fmt.Println("\nFiles:")
		for key, path := range files {
			fmt.Printf("  %s: %s\n", key, path)
		}
	}

	if metadata, ok := env["metadata"].(map[string]string); ok && len(metadata) > 0 {
		fmt.Println("\nMetadata:")
		for key, value := range metadata {
			fmt.Printf("  %s: %s\n", key, value)
		}
	}
}

// printTestEnvironmentsTable prints a list of test environments in table format.
func printTestEnvironmentsTable(envs []any) {
	fmt.Println("\n=== Test Environments ===")
	fmt.Printf("%-40s %-15s %-10s %-20s\n", "ID", "NAME", "STATUS", "CREATED")
	fmt.Println(strings.Repeat("-", 90))

	for _, item := range envs {
		if env, ok := item.(map[string]any); ok {
			id := truncate(getStringField(env, "id"), 40)
			name := truncate(getStringField(env, "name"), 15)
			status := truncate(getStringField(env, "status"), 10)
			created := truncate(getStringField(env, "createdAt"), 20)
			fmt.Printf("%-40s %-15s %-10s %-20s\n", id, name, status, created)
		}
	}
}
