package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/alexandremahdhaoui/forge/pkg/forge"
)

// cmdCreate creates a new test environment for the given stage.
// Returns the generated test ID.
func cmdCreate(stageName string) (string, error) {
	if stageName == "" {
		return "", fmt.Errorf("stage name is required")
	}

	// Read forge.yaml configuration
	config, err := forge.ReadSpec()
	if err != nil {
		return "", fmt.Errorf("failed to read forge.yaml: %w", err)
	}

	// Find TestSpec for this stage
	var testSpec *forge.TestSpec
	for i := range config.Test {
		if config.Test[i].Name == stageName {
			testSpec = &config.Test[i]
			break
		}
	}

	if testSpec == nil {
		return "", fmt.Errorf("test stage not found in forge.yaml: %s", stageName)
	}

	// Generate unique test ID
	testID := generateTestID(stageName)

	// Create tmpDir for this test environment
	tmpDir := fmt.Sprintf("/tmp/forge-test-%s-%s", stageName, testID)
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create tmpDir: %w", err)
	}

	// Initialize test environment
	env := &forge.TestEnvironment{
		ID:               testID,
		Name:             stageName,
		Status:           forge.TestStatusCreated,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
		TmpDir:           tmpDir,
		Files:            make(map[string]string),
		ManagedResources: []string{tmpDir}, // tmpDir will be cleaned up
		Metadata:         make(map[string]string),
	}

	// Find the setup alias for this test stage
	setupAlias := testSpec.Testenv
	if setupAlias == "" {
		// No setup configured, just create the environment entry
		fmt.Fprintf(os.Stderr, "No testenv configured for stage %s\n", stageName)
	} else {
		// Strip "alias://" prefix if present
		setupAlias = strings.TrimPrefix(setupAlias, "alias://")

		// Orchestrate testenv-subengines
		if err := orchestrateCreate(config, setupAlias, env); err != nil {
			// Cleanup tmpDir on failure
			os.RemoveAll(tmpDir)
			return "", fmt.Errorf("failed to orchestrate testenv-subengines: %w", err)
		}
	}

	// Get artifact store path
	artifactStorePath, err := forge.GetArtifactStorePath(".forge/artifacts.json")
	if err != nil {
		return "", fmt.Errorf("failed to get artifact store path: %w", err)
	}

	store, err := forge.ReadOrCreateArtifactStore(artifactStorePath)
	if err != nil {
		return "", fmt.Errorf("failed to read artifact store: %w", err)
	}

	// Add test environment to store
	forge.AddOrUpdateTestEnvironment(&store, env)

	// Write artifact store
	if err := forge.WriteArtifactStore(artifactStorePath, store); err != nil {
		return "", fmt.Errorf("failed to write artifact store: %w", err)
	}

	// Output test ID (for CLI usage)
	fmt.Println(testID)

	return testID, nil
}

// generateTestID generates a unique test environment ID.
// Format: test-<stage>-YYYYMMDD-XXXXXXXX
func generateTestID(stageName string) string {
	// Generate random suffix
	randBytes := make([]byte, 4)
	rand.Read(randBytes)
	suffix := hex.EncodeToString(randBytes)

	// Format: test-<stage>-YYYYMMDD-XXXXXXXX
	dateStr := time.Now().Format("20060102")
	return fmt.Sprintf("test-%s-%s-%s", stageName, dateStr, suffix)
}

// orchestrateCreate calls testenv-subengines in order to set up the test environment.
func orchestrateCreate(config forge.Spec, setupAlias string, env *forge.TestEnvironment) error {
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

	// Call each subengine in order
	accumulatedMetadata := make(map[string]string)
	for _, subengine := range subengines {
		fmt.Fprintf(os.Stderr, "Setting up %s...\n", subengine.Engine)

		// Resolve engine URI to binary path
		binaryPath, err := resolveEngineURI(subengine.Engine)
		if err != nil {
			return fmt.Errorf("failed to resolve engine %s: %w", subengine.Engine, err)
		}

		// Prepare parameters for MCP call
		params := map[string]any{
			"testID":   env.ID,
			"stage":    env.Name,
			"tmpDir":   env.TmpDir,
			"metadata": accumulatedMetadata, // Pass accumulated metadata from previous subengines
		}

		// Add spec if provided
		if subengine.Spec != nil && len(subengine.Spec) > 0 {
			params["spec"] = subengine.Spec
		}

		// Call subengine's create tool via MCP
		result, err := callMCPEngine(binaryPath, "create", params)
		if err != nil {
			return fmt.Errorf("failed to create with %s: %w", subengine.Engine, err)
		}

		// Extract response from structured content
		if resultMap, ok := result.(map[string]interface{}); ok {
			// Merge files from subengine response
			if files, ok := resultMap["files"].(map[string]interface{}); ok {
				for key, value := range files {
					if strValue, ok := value.(string); ok {
						env.Files[key] = strValue
					}
				}
			}

			// Merge metadata from subengine response and accumulate for next subengine
			if metadata, ok := resultMap["metadata"].(map[string]interface{}); ok {
				for key, value := range metadata {
					if strValue, ok := value.(string); ok {
						env.Metadata[key] = strValue
						accumulatedMetadata[key] = strValue
					}
				}
			}

			// Add managed resources from subengine response
			if resources, ok := resultMap["managedResources"].([]interface{}); ok {
				for _, resource := range resources {
					if strResource, ok := resource.(string); ok {
						env.ManagedResources = append(env.ManagedResources, strResource)
					}
				}
			}
		}

		fmt.Fprintf(os.Stderr, "  ✓ %s setup complete\n", subengine.Engine)
	}

	return nil
}
