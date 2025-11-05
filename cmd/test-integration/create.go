package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/alexandremahdhaoui/forge/pkg/forge"
)

// cmdCreate creates a new test environment for the given stage.
func cmdCreate(stageName string) error {
	if stageName == "" {
		return fmt.Errorf("stage name is required")
	}

	// Read forge.yaml configuration
	config, err := forge.ReadSpec()
	if err != nil {
		return fmt.Errorf("failed to read forge.yaml: %w", err)
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
		return fmt.Errorf("test stage not found in forge.yaml: %s", stageName)
	}

	// Generate unique test ID
	testID := generateTestID(stageName)

	// Initialize test environment
	env := &forge.TestEnvironment{
		ID:               testID,
		Name:             stageName,
		Status:           forge.TestStatusCreated,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
		ManagedResources: []string{},
		Metadata:         make(map[string]string),
	}

	// Setup kindenv if needed (TODO: implement kindenv integration)
	if needsKindenv() {
		kubeconfigPath, resources, err := setupKindenv(testID)
		if err != nil {
			return fmt.Errorf("failed to setup kindenv: %w", err)
		}
		env.KubeconfigPath = kubeconfigPath
		env.ManagedResources = append(env.ManagedResources, resources...)
	}

	// Setup local-container-registry if needed (TODO: implement registry integration)
	if needsRegistry() {
		registryConfig, resources, err := setupRegistry(testID)
		if err != nil {
			return fmt.Errorf("failed to setup registry: %w", err)
		}
		env.RegistryConfig = registryConfig
		env.ManagedResources = append(env.ManagedResources, resources...)
	}

	// Get artifact store path
	artifactStorePath, err := forge.GetArtifactStorePath(".forge/artifacts.json")
	if err != nil {
		return fmt.Errorf("failed to get artifact store path: %w", err)
	}

	store, err := forge.ReadOrCreateArtifactStore(artifactStorePath)
	if err != nil {
		return fmt.Errorf("failed to read artifact store: %w", err)
	}

	// Add test environment to store
	forge.AddOrUpdateTestEnvironment(&store, env)

	// Write artifact store
	if err := forge.WriteArtifactStore(artifactStorePath, store); err != nil {
		return fmt.Errorf("failed to write artifact store: %w", err)
	}

	// Output test ID
	fmt.Println(testID)

	return nil
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

// needsKindenv checks if kindenv setup is needed.
// TODO: Implement logic to determine if kindenv is required
func needsKindenv() bool {
	// For now, return false. Will be implemented when kindenv integration is added
	return false
}

// setupKindenv sets up a kindenv cluster for the test environment.
// TODO: Implement kindenv integration
func setupKindenv(testID string) (kubeconfigPath string, managedResources []string, err error) {
	// Will be implemented during kindenv integration
	return "", nil, nil
}

// needsRegistry checks if local-container-registry setup is needed.
// TODO: Implement logic to determine if registry is required
func needsRegistry() bool {
	// For now, return false. Will be implemented when registry integration is added
	return false
}

// setupRegistry sets up a local container registry for the test environment.
// TODO: Implement registry integration
func setupRegistry(testID string) (registryConfig map[string]string, managedResources []string, err error) {
	// Will be implemented during registry integration
	return nil, nil, nil
}
