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

	if config.Engine == "" {
		return "", fmt.Errorf("engine alias %s has no engine URI configured", alias)
	}

	return config.Engine, nil
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
