package main

import (
	"encoding/json"
	"fmt"

	"github.com/alexandremahdhaoui/tooling/pkg/forge"
)

func runBuild(args []string) error {
	// Load forge.yaml configuration
	config, err := loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load forge.yaml: %w", err)
	}

	// Read artifact store
	store, err := forge.ReadArtifactStore(config.Build.ArtifactStorePath)
	if err != nil {
		return fmt.Errorf("failed to read artifact store: %w", err)
	}

	// Filter specs if artifact name provided
	var artifactName string
	if len(args) > 0 {
		artifactName = args[0]
	}

	// Group specs by engine
	engineSpecs := make(map[string][]map[string]any)

	for _, spec := range config.Build.Specs {
		// Filter by artifact name if provided
		if artifactName != "" && spec.Name != artifactName {
			continue
		}

		// Use the engine specified in the BuildSpec
		engine := spec.Engine
		params := map[string]any{
			"name":   spec.Name,
			"src":    spec.Src,
			"dest":   spec.Dest,
			"engine": engine,
		}
		engineSpecs[engine] = append(engineSpecs[engine], params)
	}

	if len(engineSpecs) == 0 {
		if artifactName != "" {
			return fmt.Errorf("no artifact found with name: %s", artifactName)
		}
		fmt.Println("No artifacts to build")
		return nil
	}

	// Build each group using the appropriate engine
	totalBuilt := 0
	for engineURI, specs := range engineSpecs {
		fmt.Printf("Building %d artifact(s) with %s...\n", len(specs), engineURI)

		// Parse engine URI
		_, binaryPath, err := parseEngine(engineURI)
		if err != nil {
			return fmt.Errorf("failed to parse engine %s: %w", engineURI, err)
		}

		// Use buildBatch if multiple specs, otherwise use build
		var result interface{}
		if len(specs) == 1 {
			result, err = callMCPEngine(binaryPath, "build", specs[0])
		} else {
			params := map[string]any{
				"specs": specs,
			}
			result, err = callMCPEngine(binaryPath, "buildBatch", params)
		}

		if err != nil {
			return fmt.Errorf("build failed: %w", err)
		}

		// Parse artifacts from result
		artifacts, err := parseArtifacts(result)
		if err != nil {
			fmt.Printf("Warning: could not parse artifacts: %v\n", err)
		} else {
			// Update artifact store
			for _, artifact := range artifacts {
				forge.AddOrUpdateArtifact(&store, artifact)
				totalBuilt++
			}
		}
	}

	// Write updated artifact store
	if err := forge.WriteArtifactStore(config.Build.ArtifactStorePath, store); err != nil {
		return fmt.Errorf("failed to write artifact store: %w", err)
	}

	fmt.Printf("✅ Successfully built %d artifact(s)\n", totalBuilt)
	return nil
}

// parseArtifacts converts MCP result to forge.Artifact slice.
func parseArtifacts(result interface{}) ([]forge.Artifact, error) {
	// Result could be a single artifact or array of artifacts
	// Try to convert to JSON and back to parse it
	data, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}

	// Try parsing as single artifact first
	var singleArtifact forge.Artifact
	if err := json.Unmarshal(data, &singleArtifact); err == nil && singleArtifact.Name != "" {
		return []forge.Artifact{singleArtifact}, nil
	}

	// Try parsing as array of artifacts
	var multipleArtifacts []forge.Artifact
	if err := json.Unmarshal(data, &multipleArtifacts); err == nil {
		return multipleArtifacts, nil
	}

	return nil, fmt.Errorf("could not parse artifacts from result")
}
