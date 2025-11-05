package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/alexandremahdhaoui/forge/pkg/forge"
)

// cmdList lists all test reports, optionally filtered by stage.
func cmdList(stageFilter string) error {
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

	// Get test reports (optionally filtered by stage)
	reports := forge.ListTestReports(&store, stageFilter)

	// Sort reports by StartTime (newest first)
	sort.Slice(reports, func(i, j int) bool {
		return reports[i].StartTime.After(reports[j].StartTime)
	})

	// Output JSON array
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(reports); err != nil {
		return fmt.Errorf("failed to encode test reports: %w", err)
	}

	return nil
}
