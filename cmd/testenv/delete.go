package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/alexandremahdhaoui/forge/pkg/forge"
)

// cmdDelete deletes a test environment by ID.
func cmdDelete(testID string) error {
	if testID == "" {
		return fmt.Errorf("test ID is required")
	}

	// Get artifact store path
	artifactStorePath, err := forge.GetArtifactStorePath(".forge/artifacts.json")
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

	// Find the test stage configuration
	var testSpec *forge.TestSpec
	config, err := forge.ReadSpec()
	if err != nil {
		return fmt.Errorf("failed to read forge.yaml: %w", err)
	}

	for i := range config.Test {
		if config.Test[i].Name == env.Name {
			testSpec = &config.Test[i]
			break
		}
	}

	// Orchestrate testenv-subengine cleanup in REVERSE order
	if testSpec != nil && testSpec.Testenv != "" {
		if strings.HasPrefix(testSpec.Testenv, "go://") {
			// Direct engine URI - call delete tool directly
			fmt.Fprintf(os.Stderr, "Tearing down %s...\n", testSpec.Testenv)

			binaryPath, err := resolveEngineURI(testSpec.Testenv)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to resolve engine %s: %v\n", testSpec.Testenv, err)
			} else {
				// Prepare parameters - test-report uses reportID, others use testID
				params := map[string]any{}
				if testSpec.Testenv == "go://test-report" {
					params["reportID"] = testID
				} else {
					params["testID"] = testID
				}

				_, err = callMCPEngine(binaryPath, "delete", params)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to delete with %s: %v\n", testSpec.Testenv, err)
				} else {
					fmt.Fprintf(os.Stderr, "  ✓ %s teardown complete\n", testSpec.Testenv)
				}
			}
		} else {
			// Alias reference - orchestrate subengines
			setupAlias := strings.TrimPrefix(testSpec.Testenv, "alias://")

			if err := orchestrateDelete(config, setupAlias, env); err != nil {
				// Log error but continue with cleanup (best effort)
				fmt.Fprintf(os.Stderr, "Warning: failed to orchestrate cleanup: %v\n", err)
			}
		}
	}

	// Delete managed resources (including tmpDir)
	for _, resource := range env.ManagedResources {
		if err := os.RemoveAll(resource); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to remove resource %s: %v\n", resource, err)
		}
	}

	// Remove from artifact store
	if err := forge.DeleteTestEnvironment(&store, testID); err != nil {
		return fmt.Errorf("failed to delete test environment: %w", err)
	}

	// Write updated artifact store
	if err := forge.WriteArtifactStore(artifactStorePath, store); err != nil {
		return fmt.Errorf("failed to write artifact store: %w", err)
	}

	// Print to stderr to avoid interfering with MCP JSON output
	fmt.Fprintf(os.Stderr, "Deleted test environment: %s\n", testID)
	return nil
}

// orchestrateDelete calls testenv-subengines in REVERSE order to tear down the test environment.
func orchestrateDelete(config forge.Spec, setupAlias string, env *forge.TestEnvironment) error {
	// Resolve the alias to get engine configuration
	var engineConfig *forge.EngineConfig
	for i := range config.Engines {
		if config.Engines[i].Alias == setupAlias {
			engineConfig = &config.Engines[i]
			break
		}
	}

	if engineConfig == nil {
		return fmt.Errorf("engine alias not found: %s", setupAlias)
	}

	// Verify it's a testenv type
	if engineConfig.Type != "testenv" {
		return fmt.Errorf("engine %s is not a testenv type (got: %s)", setupAlias, engineConfig.Type)
	}

	// Get the list of testenv-subengines
	subengines := engineConfig.Testenv
	if len(subengines) == 0 {
		return fmt.Errorf("no testenv-subengines configured for %s", setupAlias)
	}

	// Call each subengine in REVERSE order for cleanup
	for i := len(subengines) - 1; i >= 0; i-- {
		subengine := subengines[i]
		fmt.Fprintf(os.Stderr, "Tearing down %s...\n", subengine.Engine)

		// Resolve engine URI to binary path
		binaryPath, err := resolveEngineURI(subengine.Engine)
		if err != nil {
			// Log error but continue with other subengines (best effort)
			fmt.Fprintf(os.Stderr, "  Warning: failed to resolve engine %s: %v\n", subengine.Engine, err)
			continue
		}

		// Prepare parameters for MCP call
		params := map[string]any{
			"testID":   env.ID,
			"metadata": env.Metadata, // Pass environment metadata for cleanup
		}

		// Call subengine's delete tool via MCP
		_, err = callMCPEngine(binaryPath, "delete", params)
		if err != nil {
			// Log error but continue with other subengines (best effort)
			fmt.Fprintf(os.Stderr, "  Warning: failed to delete with %s: %v\n", subengine.Engine, err)
			continue
		}

		fmt.Fprintf(os.Stderr, "  ✓ %s teardown complete\n", subengine.Engine)
	}

	return nil
}
