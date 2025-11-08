package main

import (
	"fmt"

	"github.com/alexandremahdhaoui/forge/pkg/forge"
)

// getEngineConfig returns the full engine configuration for a given alias.
// Returns nil if the alias is not found.
func getEngineConfig(alias string, spec *forge.Spec) *forge.EngineConfig {
	if spec == nil {
		return nil
	}

	for i := range spec.Engines {
		if spec.Engines[i].Alias == alias {
			return &spec.Engines[i]
		}
	}

	return nil
}

// resolveEngineAlias resolves an engine alias to its underlying engine URI.
// Returns an error if the alias is not found in the spec.
func resolveEngineAlias(alias string, spec *forge.Spec) (string, error) {
	config := getEngineConfig(alias, spec)
	if config == nil {
		return "", fmt.Errorf("engine alias not found: %s (check forge.yaml engines section)", alias)
	}

	// For testenv type, return the testenv orchestrator
	if config.Type == forge.TestenvEngineConfigType {
		return "go://testenv", nil
	}

	// For builder type, check if there's exactly one builder engine
	if config.Type == forge.BuilderEngineConfigType {
		if len(config.Builder) == 0 {
			return "", fmt.Errorf("builder alias %s has no builder engines configured", alias)
		}
		if len(config.Builder) > 1 {
			return "", fmt.Errorf("builder alias %s has multiple engines (not yet supported in simple resolution)", alias)
		}
		return config.Builder[0].Engine, nil
	}

	// For test-runner type, check if there's exactly one test runner engine
	if config.Type == forge.TestRunnerEngineConfigType {
		if len(config.TestRunner) == 0 {
			return "", fmt.Errorf("test-runner alias %s has no test runner engines configured", alias)
		}
		if len(config.TestRunner) > 1 {
			return "", fmt.Errorf("test-runner alias %s has multiple engines (not yet supported in simple resolution)", alias)
		}
		return config.TestRunner[0].Engine, nil
	}

	return "", fmt.Errorf("unknown engine type for alias %s", alias)
}

// resolveEngine resolves an engine URI (which may be an alias) to the actual engine binary path.
// This is a wrapper around parseEngine that handles alias resolution.
func resolveEngine(engineURI string, spec *forge.Spec) (string, error) {
	engineType, binaryPathOrAlias, err := parseEngine(engineURI)
	if err != nil {
		return "", err
	}

	// If it's an alias, resolve it
	if engineType == "alias" {
		aliasName := binaryPathOrAlias

		// Get the actual engine URI from the alias
		resolvedURI, err := resolveEngineAlias(aliasName, spec)
		if err != nil {
			return "", err
		}

		// Recursively parse the resolved URI (it should be go://)
		engineType, binaryPath, err := parseEngine(resolvedURI)
		if err != nil {
			return "", fmt.Errorf("failed to parse resolved engine URI %s for alias %s: %w", resolvedURI, aliasName, err)
		}

		if engineType == "alias" {
			return "", fmt.Errorf("circular alias reference detected: %s", aliasName)
		}

		return binaryPath, nil
	}

	// Not an alias, return the binary path directly
	return binaryPathOrAlias, nil
}
