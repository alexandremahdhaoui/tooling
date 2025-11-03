package main

import (
	"github.com/alexandremahdhaoui/forge/pkg/forge"
)

// loadConfig loads the forge configuration from forge.yaml.
func loadConfig() (forge.Spec, error) {
	return forge.ReadSpec()
}
