package main

import (
	"fmt"

	"github.com/alexandremahdhaoui/forge/pkg/forge"
)

// loadConfig loads the forge configuration from forge.yaml or custom path.
func loadConfig() (forge.Spec, error) {
	if configPath != "" {
		return forge.ReadSpecFromPath(configPath)
	}
	return forge.ReadSpec()
}

// runConfig handles the config command
func runConfig(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("config subcommand required (validate)")
	}

	subcommand := args[0]

	switch subcommand {
	case "validate":
		return runConfigValidate(args[1:])
	default:
		return fmt.Errorf("unknown config subcommand: %s (available: validate)", subcommand)
	}
}

// runConfigValidate validates the forge.yaml configuration
func runConfigValidate(args []string) error {
	// Determine config path (default: forge.yaml)
	configPath := "forge.yaml"
	if len(args) > 0 {
		configPath = args[0]
	}

	// Read and validate the spec
	spec, err := forge.ReadSpecFromPath(configPath)
	if err != nil {
		return fmt.Errorf("validation failed:\n%v", err)
	}

	// If we got here, validation passed
	fmt.Printf("✅ Configuration is valid: %s\n", configPath)
	fmt.Printf("Project: %s\n", spec.Name)
	fmt.Printf("Build specs: %d\n", len(spec.Build))
	fmt.Printf("Test stages: %d\n", len(spec.Test))
	fmt.Printf("Engine configs: %d\n", len(spec.Engines))

	return nil
}
