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

// testGet retrieves and displays test environment/report details by calling the engine.
func testGet(testSpec *forge.TestSpec, args []string) error {
	// Parse output format flag
	format, remainingArgs := parseOutputFormat(args)

	if len(remainingArgs) < 1 {
		return fmt.Errorf("usage: forge test <STAGE> get <TEST-ID> [-o json|yaml]")
	}

	testID := remainingArgs[0]

	// Resolve engine path
	config, err := forge.ReadSpec()
	if err != nil {
		return fmt.Errorf("failed to read forge.yaml: %w", err)
	}

	enginePath, err := resolveEngine(testSpec.Engine, &config)
	if err != nil {
		return fmt.Errorf("failed to resolve engine: %w", err)
	}

	// Determine the parameter name based on engine
	paramName := "testID"
	if strings.Contains(testSpec.Engine, "test-report") {
		paramName = "reportID"
	}

	// Call engine get tool
	result, err := callMCPEngine(enginePath, "get", map[string]any{
		paramName: testID,
	})
	if err != nil {
		return fmt.Errorf("failed to get test details: %w", err)
	}

	// Print result in requested format
	if resultMap, ok := result.(map[string]any); ok {
		switch format {
		case outputFormatJSON:
			printJSON(resultMap)
		case outputFormatYAML:
			printYAML(resultMap)
		default:
			printTestReportTable(resultMap)
		}
	} else {
		fmt.Printf("%v\n", result)
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

	enginePath, err := resolveEngine(testSpec.Engine, &config)
	if err != nil {
		return fmt.Errorf("failed to resolve engine: %w", err)
	}

	// Determine the parameter name based on engine
	paramName := "testID"
	if strings.Contains(testSpec.Engine, "test-report") {
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

	enginePath, err := resolveEngine(testSpec.Engine, &config)
	if err != nil {
		return fmt.Errorf("failed to resolve engine: %w", err)
	}

	// Call engine list tool
	result, err := callMCPEngine(enginePath, "list", map[string]any{
		"stage": testSpec.Name,
	})
	if err != nil {
		return fmt.Errorf("failed to list: %w", err)
	}

	// Print result in requested format
	if result == nil {
		fmt.Printf("No items found for stage: %s\n", testSpec.Name)
	} else if resultSlice, ok := result.([]any); ok {
		if len(resultSlice) == 0 {
			fmt.Printf("No items found for stage: %s\n", testSpec.Name)
		} else {
			switch format {
			case outputFormatJSON:
				printJSON(resultSlice)
			case outputFormatYAML:
				printYAML(resultSlice)
			default:
				printTestReportsTable(resultSlice)
			}
		}
	} else if resultMap, ok := result.(map[string]any); ok {
		switch format {
		case outputFormatJSON:
			printJSON(resultMap)
		case outputFormatYAML:
			printYAML(resultMap)
		default:
			fmt.Printf("%v\n", resultMap)
		}
	} else {
		fmt.Printf("%v\n", result)
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
