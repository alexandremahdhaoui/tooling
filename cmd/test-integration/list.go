package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/alexandremahdhaoui/forge/pkg/forge"
)

// cmdList lists all test environments, optionally filtered by stage.
func cmdList(stageFilter string) error {
	// Get artifact store path
	artifactStorePath, err := forge.GetArtifactStorePath(".forge/artifacts.json")
	if err != nil {
		return fmt.Errorf("failed to get artifact store path: %w", err)
	}

	store, err := forge.ReadArtifactStore(artifactStorePath)
	if err != nil {
		return fmt.Errorf("failed to read artifact store: %w", err)
	}

	// List test environments
	envs := forge.ListTestEnvironments(&store, stageFilter)

	if len(envs) == 0 {
		if stageFilter != "" {
			fmt.Printf("No test environments found for stage: %s\n", stageFilter)
		} else {
			fmt.Println("No test environments found")
		}
		return nil
	}

	// Display as table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tSTAGE\tSTATUS\tCREATED")
	fmt.Fprintln(w, "--\t-----\t------\t-------")
	for _, env := range envs {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			env.ID,
			env.Name,
			env.Status,
			env.CreatedAt.Format("2006-01-02 15:04"),
		)
	}
	w.Flush()

	return nil
}
