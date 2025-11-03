package main

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/alexandremahdhaoui/tooling/pkg/forge"
)

const integrationEnvStorePath = ".ignore.integration-envs.yaml"

func runIntegration(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("integration command requires a subcommand (create, list, get, delete)")
	}

	subcommand := args[0]

	switch subcommand {
	case "create":
		return integrationCreate(args[1:])
	case "list":
		return integrationList()
	case "get":
		return integrationGet(args[1:])
	case "delete":
		return integrationDelete(args[1:])
	default:
		return fmt.Errorf("unknown integration subcommand: %s", subcommand)
	}
}

func integrationCreate(args []string) error {
	envName := "default"
	if len(args) > 0 {
		envName = args[0]
	}

	// Load forge.yaml to get kindenv config
	config, err := loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load forge.yaml: %w", err)
	}

	// Generate unique ID
	envID := fmt.Sprintf("env-%s", time.Now().Format("20060102-150405"))

	// Create environment structure
	env := forge.IntegrationEnvironment{
		ID:      envID,
		Name:    envName,
		Created: time.Now().UTC().Format(time.RFC3339),
		Components: make(map[string]forge.Component),
	}

	// Setup kindenv if configured
	if config.Kindenv.KubeconfigPath != "" {
		fmt.Printf("⏳ Setting up kindenv for environment %s...\n", envID)
		if err := setupKindenv(config, envID); err != nil {
			return fmt.Errorf("failed to setup kindenv: %w", err)
		}

		env.Components["kindenv"] = forge.Component{
			Enabled: true,
			Ready:   true,
			ConnectionInfo: map[string]string{
				"kubeconfigPath": config.Kindenv.KubeconfigPath,
			},
		}
		fmt.Println("✅ kindenv setup complete")
	}

	// Setup local-container-registry if enabled
	if config.LocalContainerRegistry.Enabled {
		fmt.Printf("⏳ Setting up local-container-registry for environment %s...\n", envID)
		if err := setupLocalContainerRegistry(config); err != nil {
			return fmt.Errorf("failed to setup local-container-registry: %w", err)
		}

		env.Components["localContainerRegistry"] = forge.Component{
			Enabled: true,
			Ready:   true,
			ConnectionInfo: map[string]string{
				"credentialPath": config.LocalContainerRegistry.CredentialPath,
				"caCrtPath":      config.LocalContainerRegistry.CaCrtPath,
				"namespace":      config.LocalContainerRegistry.Namespace,
			},
		}
		fmt.Println("✅ local-container-registry setup complete")
	}

	// Read store
	store, err := forge.ReadIntegrationEnvStore(integrationEnvStorePath)
	if err != nil {
		return fmt.Errorf("failed to read integration env store: %w", err)
	}

	// Add environment
	forge.AddEnvironment(&store, env)

	// Write store
	if err := forge.WriteIntegrationEnvStore(integrationEnvStorePath, store); err != nil {
		return fmt.Errorf("failed to write integration env store: %w", err)
	}

	fmt.Printf("✅ Created integration environment: %s (ID: %s)\n", envName, envID)
	return nil
}

// setupKindenv calls the kindenv binary to setup a kind cluster
func setupKindenv(config forge.Spec, envID string) error {
	// Find kindenv binary - check local build directory first, then PATH
	kindenvBinary := "./build/bin/kindenv"
	if _, err := os.Stat(kindenvBinary); os.IsNotExist(err) {
		kindenvBinary = "kindenv"
	}

	// Create command to call kindenv setup
	cmd := exec.Command(kindenvBinary, "setup")
	cmd.Env = os.Environ() // Inherit environment variables

	// Set stdout and stderr to current process
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Run the command
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("kindenv setup failed: %w", err)
	}

	return nil
}

// teardownKindenv calls the kindenv binary to teardown a kind cluster
func teardownKindenv() error {
	// Find kindenv binary - check local build directory first, then PATH
	kindenvBinary := "./build/bin/kindenv"
	if _, err := os.Stat(kindenvBinary); os.IsNotExist(err) {
		kindenvBinary = "kindenv"
	}

	// Create command to call kindenv teardown
	cmd := exec.Command(kindenvBinary, "teardown")
	cmd.Env = os.Environ() // Inherit environment variables

	// Set stdout and stderr to current process
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Run the command
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("kindenv teardown failed: %w", err)
	}

	return nil
}

// setupLocalContainerRegistry calls the local-container-registry binary to setup a container registry
func setupLocalContainerRegistry(config forge.Spec) error {
	// Find local-container-registry binary - check local build directory first, then PATH
	registryBinary := "./build/bin/local-container-registry"
	if _, err := os.Stat(registryBinary); os.IsNotExist(err) {
		registryBinary = "local-container-registry"
	}

	// Create command to call local-container-registry setup (default command)
	cmd := exec.Command(registryBinary)
	cmd.Env = os.Environ() // Inherit environment variables

	// Set stdout and stderr to current process
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Run the command
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("local-container-registry setup failed: %w", err)
	}

	return nil
}

// teardownLocalContainerRegistry calls the local-container-registry binary to teardown the registry
func teardownLocalContainerRegistry() error {
	// Find local-container-registry binary - check local build directory first, then PATH
	registryBinary := "./build/bin/local-container-registry"
	if _, err := os.Stat(registryBinary); os.IsNotExist(err) {
		registryBinary = "local-container-registry"
	}

	// Create command to call local-container-registry teardown
	cmd := exec.Command(registryBinary, "teardown")
	cmd.Env = os.Environ() // Inherit environment variables

	// Set stdout and stderr to current process
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Run the command
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("local-container-registry teardown failed: %w", err)
	}

	return nil
}

func integrationList() error {
	store, err := forge.ReadIntegrationEnvStore(integrationEnvStorePath)
	if err != nil {
		return fmt.Errorf("failed to read integration env store: %w", err)
	}

	if len(store.Environments) == 0 {
		fmt.Println("No integration environments found")
		return nil
	}

	fmt.Printf("%-30s %-20s %-25s\n", "ID", "NAME", "CREATED")
	fmt.Println("--------------------------------------------------------------------------------")
	for _, env := range store.Environments {
		fmt.Printf("%-30s %-20s %-25s\n", env.ID, env.Name, env.Created)
	}

	return nil
}

func integrationGet(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("integration get requires an environment ID")
	}

	envID := args[0]

	store, err := forge.ReadIntegrationEnvStore(integrationEnvStorePath)
	if err != nil {
		return fmt.Errorf("failed to read integration env store: %w", err)
	}

	env, err := forge.GetEnvironment(store, envID)
	if err != nil {
		return fmt.Errorf("environment not found: %w", err)
	}

	fmt.Printf("ID: %s\n", env.ID)
	fmt.Printf("Name: %s\n", env.Name)
	fmt.Printf("Created: %s\n", env.Created)
	fmt.Println("\nComponents:")
	for name, component := range env.Components {
		fmt.Printf("  %s:\n", name)
		fmt.Printf("    Enabled: %v\n", component.Enabled)
		fmt.Printf("    Ready: %v\n", component.Ready)
		if len(component.ConnectionInfo) > 0 {
			fmt.Println("    Connection Info:")
			for k, v := range component.ConnectionInfo {
				fmt.Printf("      %s: %s\n", k, v)
			}
		}
	}

	return nil
}

func integrationDelete(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("integration delete requires an environment ID")
	}

	envID := args[0]

	store, err := forge.ReadIntegrationEnvStore(integrationEnvStorePath)
	if err != nil {
		return fmt.Errorf("failed to read integration env store: %w", err)
	}

	// Get environment to check components before deleting
	env, err := forge.GetEnvironment(store, envID)
	if err != nil {
		return fmt.Errorf("failed to get environment: %w", err)
	}

	// Teardown local-container-registry first (before kindenv)
	if registryComp, exists := env.Components["localContainerRegistry"]; exists && registryComp.Ready {
		fmt.Printf("⏳ Tearing down local-container-registry for environment %s...\n", envID)
		if err := teardownLocalContainerRegistry(); err != nil {
			fmt.Printf("Warning: local-container-registry teardown failed: %v\n", err)
			// Continue with deletion even if teardown fails
		} else {
			fmt.Println("✅ local-container-registry teardown complete")
		}
	}

	// Teardown kindenv if it exists
	if kindenvComp, exists := env.Components["kindenv"]; exists && kindenvComp.Ready {
		fmt.Printf("⏳ Tearing down kindenv for environment %s...\n", envID)
		if err := teardownKindenv(); err != nil {
			fmt.Printf("Warning: kindenv teardown failed: %v\n", err)
			// Continue with deletion even if teardown fails
		} else {
			fmt.Println("✅ kindenv teardown complete")
		}
	}

	// Delete environment from store
	if err := forge.DeleteEnvironment(&store, envID); err != nil {
		return fmt.Errorf("failed to delete environment: %w", err)
	}

	if err := forge.WriteIntegrationEnvStore(integrationEnvStorePath, store); err != nil {
		return fmt.Errorf("failed to write integration env store: %w", err)
	}

	fmt.Printf("✅ Deleted integration environment: %s\n", envID)
	return nil
}
