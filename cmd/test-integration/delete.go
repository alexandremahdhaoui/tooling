package main

import (
	"fmt"
	"os"

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

	// Cleanup kindenv if exists (TODO: implement in Phase 5)
	if env.KubeconfigPath != "" {
		if err := cleanupKindenv(env); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to cleanup kindenv: %v\n", err)
		}
	}

	// Cleanup registry if exists (TODO: implement in Phase 5)
	if len(env.RegistryConfig) > 0 {
		if err := cleanupRegistry(env); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to cleanup registry: %v\n", err)
		}
	}

	// Delete managed resources
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

// cleanupKindenv cleans up kindenv resources for the test environment.
// TODO: Implement in Phase 5 (Task 5.1)
func cleanupKindenv(env *forge.TestEnvironment) error {
	// Will be implemented during kindenv path refactoring
	return nil
}

// cleanupRegistry cleans up registry resources for the test environment.
// TODO: Implement in Phase 5 (Task 5.2)
func cleanupRegistry(env *forge.TestEnvironment) error {
	// Will be implemented during registry path refactoring
	return nil
}
