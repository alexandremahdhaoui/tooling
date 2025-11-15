package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/alexandremahdhaoui/forge/internal/orchestrate"
	"github.com/alexandremahdhaoui/forge/pkg/forge"
)

// normalizeEngineURI maps deprecated engine URIs to their current equivalents.
// Returns the normalized URI and whether a deprecated URI was used.
func normalizeEngineURI(uri string) (string, bool) {
	deprecated := map[string]string{
		"go://build-container": "go://container-build",
	}

	if newURI, ok := deprecated[uri]; ok {
		return newURI, true // deprecated
	}

	return uri, false // not deprecated
}

func runBuild(args []string) error {
	// Load forge.yaml configuration
	config, err := loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load forge.yaml: %w", err)
	}

	// Read artifact store
	store, err := forge.ReadOrCreateArtifactStore(config.ArtifactStorePath)
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

	for _, spec := range config.Build {
		// Filter by artifact name if provided
		if artifactName != "" && spec.Name != artifactName {
			continue
		}

		// Normalize engine URI and warn if deprecated
		normalizedEngine, wasDeprecated := normalizeEngineURI(spec.Engine)
		if wasDeprecated {
			_, _ = fmt.Fprintf(os.Stderr,
				"⚠️  DEPRECATED: %s is deprecated, use %s instead (in spec: %s)\n",
				spec.Engine, normalizedEngine, spec.Name)
		}

		// Use the normalized engine
		engine := normalizedEngine
		params := map[string]any{
			"name":   spec.Name,
			"src":    spec.Src,
			"dest":   spec.Dest,
			"engine": engine,
		}

		// Merge spec fields from BuildSpec into params
		if len(spec.Spec) > 0 {
			for k, v := range spec.Spec {
				params[k] = v
			}
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

	// Create forge directories for build operations
	dirs, err := createForgeDirs()
	if err != nil {
		return fmt.Errorf("failed to create forge directories: %w", err)
	}

	// Clean up old tmp directories (keep last 10 runs)
	if err := cleanupOldTmpDirs(10); err != nil {
		// Log warning but don't fail
		fmt.Fprintf(os.Stderr, "Warning: failed to cleanup old tmp directories: %v\n", err)
	}

	// Build each group using the appropriate engine
	totalBuilt := 0
	for engineURI, specs := range engineSpecs {
		fmt.Printf("Building %d artifact(s) with %s...\n", len(specs), engineURI)

		// Check if this is a multi-engine alias
		var artifacts []forge.Artifact
		if strings.HasPrefix(engineURI, "alias://") {
			aliasName := strings.TrimPrefix(engineURI, "alias://")
			engineConfig := getEngineConfig(aliasName, &config)
			if engineConfig == nil {
				return fmt.Errorf("engine alias not found: %s", aliasName)
			}

			if engineConfig.Type == forge.BuilderEngineConfigType && len(engineConfig.Builder) > 1 {
				// Multi-engine builder - use orchestrator
				fmt.Printf("  Multi-engine builder detected (%d engines)\n", len(engineConfig.Builder))

				// Create builder orchestrator
				orchestrator := orchestrate.NewBuilderOrchestrator(
					callMCPEngine,
					func(uri string) (string, []string, error) {
						return resolveEngine(uri, &config)
					},
				)

				// Prepare directories map
				dirsMap := map[string]any{
					"tmpDir":   dirs.TmpDir,
					"buildDir": dirs.BuildDir,
					"rootDir":  dirs.RootDir,
				}

				// Execute orchestration
				artifacts, err = orchestrator.Orchestrate(engineConfig.Builder, specs, dirsMap)
				if err != nil {
					return fmt.Errorf("multi-engine build failed: %w", err)
				}
			} else {
				// Single-engine alias - resolve to actual engine
				command, args, err := resolveEngine(engineURI, &config)
				if err != nil {
					return fmt.Errorf("failed to resolve engine %s: %w", engineURI, err)
				}

				artifacts, err = buildWithSingleEngine(command, args, specs, dirs, engineConfig)
				if err != nil {
					return fmt.Errorf("build failed: %w", err)
				}
			}
		} else {
			// Direct go:// URI - single engine
			command, args, err := resolveEngine(engineURI, &config)
			if err != nil {
				return fmt.Errorf("failed to resolve engine %s: %w", engineURI, err)
			}

			artifacts, err = buildWithSingleEngine(command, args, specs, dirs, nil)
			if err != nil {
				return fmt.Errorf("build failed: %w", err)
			}
		}

		// Update artifact store
		for _, artifact := range artifacts {
			forge.AddOrUpdateArtifact(&store, artifact)
			totalBuilt++
		}
	}

	// Write updated artifact store
	if err := forge.WriteArtifactStore(config.ArtifactStorePath, store); err != nil {
		return fmt.Errorf("failed to write artifact store: %w", err)
	}

	fmt.Printf("✅ Successfully built %d artifact(s)\n", totalBuilt)
	return nil
}

// buildWithSingleEngine handles building with a single engine (either direct go:// URI or single-engine alias).
func buildWithSingleEngine(
	command string,
	args []string,
	specs []map[string]any,
	dirs *ForgeDirs,
	engineConfig *forge.EngineConfig,
) ([]forge.Artifact, error) {
	// Prepare specs with injected directories and config
	specsWithConfig := make([]map[string]any, len(specs))
	for i, spec := range specs {
		// Clone the spec
		clonedSpec := make(map[string]any)
		for k, v := range spec {
			clonedSpec[k] = v
		}

		// Inject directories
		clonedSpec["tmpDir"] = dirs.TmpDir
		clonedSpec["buildDir"] = dirs.BuildDir
		clonedSpec["rootDir"] = dirs.RootDir

		// Inject engine-specific config if provided
		if engineConfig != nil && engineConfig.Type == forge.BuilderEngineConfigType && len(engineConfig.Builder) > 0 {
			builderSpec := engineConfig.Builder[0].Spec
			if builderSpec.Command != "" {
				clonedSpec["command"] = builderSpec.Command
			}
			if len(builderSpec.Args) > 0 {
				clonedSpec["args"] = builderSpec.Args
			}
			if len(builderSpec.Env) > 0 {
				clonedSpec["env"] = builderSpec.Env
			}
			if builderSpec.EnvFile != "" {
				clonedSpec["envFile"] = builderSpec.EnvFile
			}
			if builderSpec.WorkDir != "" {
				clonedSpec["workDir"] = builderSpec.WorkDir
			}
		}

		specsWithConfig[i] = clonedSpec
	}

	// Call MCP engine (use build for single spec, buildBatch for multiple)
	var result interface{}
	var err error
	if len(specsWithConfig) == 1 {
		result, err = callMCPEngine(command, args, "build", specsWithConfig[0])
	} else {
		params := map[string]any{
			"specs": specsWithConfig,
		}
		result, err = callMCPEngine(command, args, "buildBatch", params)
	}

	if err != nil {
		return nil, err
	}

	// Parse and return artifacts
	return parseArtifacts(result)
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
