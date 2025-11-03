package main

import (
	"fmt"
	"time"

	"github.com/alexandremahdhaoui/forge/pkg/forge"
)

// cmdGet retrieves and displays a test environment by ID.
func cmdGet(testID string) error {
	if testID == "" {
		return fmt.Errorf("test ID is required")
	}

	// Read forge.yaml to get artifact store path
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

	// Display information
	fmt.Printf("ID: %s\n", env.ID)
	fmt.Printf("Stage: %s\n", env.Name)
	fmt.Printf("Status: %s\n", env.Status)
	fmt.Printf("Created: %s\n", env.CreatedAt.Format(time.RFC3339))
	fmt.Printf("Updated: %s\n", env.UpdatedAt.Format(time.RFC3339))

	if env.ArtifactPath != "" {
		fmt.Printf("Artifact Path: %s\n", env.ArtifactPath)
	}

	if env.KubeconfigPath != "" {
		fmt.Printf("Kubeconfig: %s\n", env.KubeconfigPath)
	}

	if len(env.RegistryConfig) > 0 {
		fmt.Println("Registry Config:")
		for k, v := range env.RegistryConfig {
			fmt.Printf("  %s: %s\n", k, v)
		}
	}

	if len(env.ManagedResources) > 0 {
		fmt.Println("Managed Resources:")
		for _, resource := range env.ManagedResources {
			fmt.Printf("  - %s\n", resource)
		}
	}

	if len(env.Metadata) > 0 {
		fmt.Println("Metadata:")
		for k, v := range env.Metadata {
			fmt.Printf("  %s: %s\n", k, v)
		}
	}

	return nil
}
