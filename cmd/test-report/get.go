package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/alexandremahdhaoui/forge/pkg/forge"
)

// cmdGet retrieves and displays details about a specific test report.
func cmdGet(reportID string) error {
	// Get artifact store path from environment variable
	artifactStorePath := os.Getenv("FORGE_ARTIFACT_STORE_PATH")
	if artifactStorePath == "" {
		artifactStorePath = ".forge/artifacts.yaml"
	}

	// Read artifact store
	store, err := forge.ReadArtifactStore(artifactStorePath)
	if err != nil {
		return fmt.Errorf("failed to read artifact store: %w", err)
	}

	// Get test report
	report, err := forge.GetTestReport(&store, reportID)
	if err != nil {
		return fmt.Errorf("failed to get test report: %w", err)
	}

	// Output JSON
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		return fmt.Errorf("failed to encode test report: %w", err)
	}

	return nil
}
