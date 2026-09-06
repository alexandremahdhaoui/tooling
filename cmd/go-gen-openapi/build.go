// Copyright 2024 Alexandre Mahdhaoui
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/alexandremahdhaoui/forge/internal/util"
	"github.com/alexandremahdhaoui/forge/pkg/engineframework"
	"github.com/alexandremahdhaoui/forge/pkg/engineversion"
	"github.com/alexandremahdhaoui/forge/pkg/forge"
	"github.com/alexandremahdhaoui/forge/pkg/mcptypes"
)

const (
	sourceFileTemplate  = "%s.%s.yaml"
	zzGeneratedFilename = "zz_generated.oapi-codegen.go"

	clientTemplate = `---
package: %[1]s
output: %[2]s
generate:
  client: true
  models: true
  embedded-spec: true
output-options:
  # to make sure that all types are generated
  skip-prune: true
`

	serverTemplate = `---
package: %[1]s
output: %[2]s
generate:
  embedded-spec: true
  models: true
  std-http-server: true
  strict-server: true
output-options:
  skip-prune: true
`
)

// Build implements the BuilderFunc for generating OpenAPI client and server code
func Build(ctx context.Context, input mcptypes.BuildInput, _ *Spec) ([]forge.Artifact, error) {
	log.Printf("Generating OpenAPI code for: %s", input.Name)

	// Extract OpenAPI config from BuildInput.Spec
	config, err := extractOpenAPIConfigFromInput(input)
	if err != nil {
		return nil, fmt.Errorf("failed to extract config: %w", err)
	}

	// A provisioned binary wins when no explicit version is demanded: the
	// workspace's .forge/bin sits first on PATH carrying the pinned build
	// the factory's toolchain section resolved. OAPI_CODEGEN_VERSION still
	// forces the go run form - an explicit demand outranks whatever is
	// installed.
	oapiCodegenVersion := os.Getenv("OAPI_CODEGEN_VERSION")

	var executable string

	if path, err := exec.LookPath("oapi-codegen"); err == nil && oapiCodegenVersion == "" {
		executable = path
	} else {
		if oapiCodegenVersion == "" {
			oapiCodegenVersion = "v2.3.0"
		}

		executable = fmt.Sprintf("go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@%s", oapiCodegenVersion)
	}

	// Call existing generation logic, passing RootDir for relative path resolution
	if err := doGenerate(executable, *config, input.RootDir); err != nil {
		return nil, fmt.Errorf("generation failed: %w", err)
	}

	// Extract spec paths from config for dependency detection
	var specPaths []string
	for _, spec := range config.Specs {
		sourcePath := spec.Source
		if input.RootDir != "" && !filepath.IsAbs(sourcePath) {
			sourcePath = filepath.Join(input.RootDir, sourcePath)
		}
		specPaths = append(specPaths, sourcePath)
	}

	// Detect dependencies for lazy rebuild
	deps, err := detectOpenAPIDependencies(ctx, specPaths, input.RootDir)
	if err != nil {
		// Log warning but don't fail - lazy build is optional optimization
		log.Printf("WARNING: dependency detection failed: %v", err)
		// Return artifact without dependencies (will always rebuild)
		return engineframework.One(engineframework.CreateArtifact(
			input.Name,
			forge.TypeGenerated,
			config.Specs[0].DestinationDir,
		)), nil
	}

	// Return artifact WITH dependencies for lazy rebuild
	artifact := engineframework.CreateArtifact(
		input.Name,
		forge.TypeGenerated,
		config.Specs[0].DestinationDir,
	)
	artifact.Dependencies = deps
	artifact.DependencyDetectorEngine = "forge://go-gen-openapi-dep-detector"
	return engineframework.One(artifact), nil
}

// detectOpenAPIDependencies calls the go-gen-openapi-dep-detector MCP server
// to discover which files the OpenAPI generation depends on.
func detectOpenAPIDependencies(ctx context.Context, specPaths []string, rootDir string) ([]forge.ArtifactDependency, error) {
	// Resolve detector URI to command and args
	// Use GetEffectiveVersion to handle both ldflags version and go run @version
	cmd, args, err := engineframework.ResolveDetector("forge://go-gen-openapi-dep-detector", engineversion.GetEffectiveVersion(Version))
	if err != nil {
		return nil, err
	}

	input := map[string]any{
		"specSources": specPaths,
		"rootDir":     rootDir,
		"resolveRefs": false, // v1: no $ref resolution
	}

	return engineframework.CallDetector(ctx, cmd, args, "detectDependencies", input)
}

func doGenerate(executable string, config forge.GenerateOpenAPIConfig, rootDir string) error {
	cmdName, args := parseExecutable(executable)
	errChan := make(chan error, 100) // Buffered to avoid goroutine leaks
	wg := &sync.WaitGroup{}

	for i := range config.Specs {
		i := i

		// Handle new design: empty Versions array means single BuildSpec per version
		// Source path is already fully resolved in the Spec.Source field
		versions := config.Specs[i].Versions
		if len(versions) == 0 {
			// New design: Source is already resolved, no need to loop over versions
			sourcePath := config.Specs[i].Source

			// Generate client if enabled
			if config.Specs[i].Client.Enabled {
				wg.Add(1)
				go func() {
					defer wg.Done()
					if err := generatePackage(cmdName, args, config, i, "", config.Specs[i].Client, clientTemplate, sourcePath, rootDir); err != nil {
						errChan <- err
					}
				}()
			}

			// Generate server if enabled
			if config.Specs[i].Server.Enabled {
				wg.Add(1)
				go func() {
					defer wg.Done()
					if err := generatePackage(cmdName, args, config, i, "", config.Specs[i].Server, serverTemplate, sourcePath, rootDir); err != nil {
						errChan <- err
					}
				}()
			}
		} else {
			// Old design (backward compatibility): loop over versions
			for _, version := range versions {
				version := version

				sourcePath := templateSourcePath(config, i, version)

				// Generate client if enabled
				if config.Specs[i].Client.Enabled {
					wg.Add(1)
					go func() {
						defer wg.Done()
						if err := generatePackage(cmdName, args, config, i, version, config.Specs[i].Client, clientTemplate, sourcePath, rootDir); err != nil {
							errChan <- err
						}
					}()
				}

				// Generate server if enabled
				if config.Specs[i].Server.Enabled {
					wg.Add(1)
					go func() {
						defer wg.Done()
						if err := generatePackage(cmdName, args, config, i, version, config.Specs[i].Server, serverTemplate, sourcePath, rootDir); err != nil {
							errChan <- err
						}
					}()
				}
			}
		}
	}

	go func() {
		wg.Wait()
		close(errChan)
	}()

	// Collect all errors
	var errors []string
	for err := range errChan {
		if err != nil {
			errors = append(errors, err.Error())
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("generation failed: %s", strings.Join(errors, "; "))
	}

	fmt.Fprintln(os.Stderr, "Successfully generated OpenAPI code")
	return nil
}

func generatePackage(cmdName string, baseArgs []string, config forge.GenerateOpenAPIConfig, specIndex int, version string, opts forge.GenOpts, template string, sourcePath string, rootDir string) error {
	outputPath := templateOutputPath(config, specIndex, opts.PackageName)
	templatedConfig := fmt.Sprintf(template, opts.PackageName, outputPath)

	path, cleanup, err := writeTempCodegenConfig(templatedConfig)
	if err != nil {
		return fmt.Errorf("failed to write temp config: %w", err)
	}
	defer cleanup()

	// Create output directory, handling both relative and absolute paths
	// If outputPath is relative and we have a rootDir, resolve it from rootDir
	actualOutputPath := outputPath
	if rootDir != "" && !filepath.IsAbs(outputPath) {
		actualOutputPath = filepath.Join(rootDir, outputPath)
	}

	if err := os.MkdirAll(filepath.Dir(actualOutputPath), 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	args := append(baseArgs, "--config", path, sourcePath)
	cmd := exec.Command(cmdName, args...)

	// Set working directory to rootDir so relative paths work correctly
	// rootDir is where forge.yaml is located, making relative paths in spec work
	if rootDir != "" {
		cmd.Dir = rootDir
	}

	if err := util.RunCmdWithStdPipes(cmd); err != nil {
		return fmt.Errorf("oapi-codegen failed for %s: %w", opts.PackageName, err)
	}

	return nil
}

func parseExecutable(executable string) (string, []string) {
	split := strings.Split(executable, " ")
	return split[0], split[1:]
}

func writeTempCodegenConfig(templatedConfig string) (string, func(), error) {
	tempFile, err := os.CreateTemp("", "oapi-codegen-*.yaml")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temp file: %w", err)
	}

	cleanup := func() {
		_ = os.RemoveAll(tempFile.Name())
	}

	if _, err := tempFile.WriteString(templatedConfig); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := tempFile.Close(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("failed to close temp file: %w", err)
	}

	return tempFile.Name(), cleanup, nil
}

func templateOutputPath(config forge.GenerateOpenAPIConfig, index int, packageName string) string {
	destDir := config.Defaults.DestinationDir
	if config.Specs[index].DestinationDir != "" {
		destDir = config.Specs[index].DestinationDir
	}

	return filepath.Join(destDir, packageName, zzGeneratedFilename)
}

func templateSourcePath(config forge.GenerateOpenAPIConfig, index int, version string) string {
	if source := config.Specs[index].Source; source != "" {
		return source
	}

	sourceFile := fmt.Sprintf(sourceFileTemplate, config.Specs[index].Name, version)

	sourceDir := config.Defaults.SourceDir
	if config.Specs[index].SourceDir != "" {
		sourceDir = config.Specs[index].SourceDir
	}

	return filepath.Join(sourceDir, sourceFile)
}
