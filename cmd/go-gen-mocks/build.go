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

	"github.com/alexandremahdhaoui/forge/pkg/engineframework"
	"github.com/alexandremahdhaoui/forge/pkg/engineversion"
	"github.com/alexandremahdhaoui/forge/pkg/forge"
	"github.com/alexandremahdhaoui/forge/pkg/mcptypes"
)

// Build implements the BuilderFunc for generating Go mocks using mockery
func Build(ctx context.Context, input mcptypes.BuildInput, spec *Spec) ([]forge.Artifact, error) {
	log.Printf("Generating mocks")

	// Get mocksDir from environment variable
	mocksDir := os.Getenv("MOCKS_DIR")

	if err := generateMocks(mocksDir); err != nil {
		return nil, fmt.Errorf("mock generation failed: %w", err)
	}

	// Detect dependencies for lazy rebuild
	deps, err := detectMockDependencies(ctx, input.RootDir)
	if err != nil {
		// Log warning but don't fail - lazy build is optional optimization
		log.Printf("WARNING: dependency detection failed: %v", err)
		// Return artifact without dependencies (will always rebuild)
		return engineframework.One(engineframework.CreateArtifact(
			input.Name,
			forge.TypeGenerated,
			getMocksDir(mocksDir),
		)), nil
	}

	// Return artifact WITH dependencies for lazy rebuild
	artifact := engineframework.CreateArtifact(
		input.Name,
		forge.TypeGenerated,
		getMocksDir(mocksDir),
	)
	artifact.Dependencies = deps
	artifact.DependencyDetectorEngine = "forge://go-gen-mocks-dep-detector"
	return engineframework.One(artifact), nil
}

// detectMockDependencies calls the go-gen-mocks-dep-detector MCP server
// to discover which files the mock generation depends on.
func detectMockDependencies(ctx context.Context, rootDir string) ([]forge.ArtifactDependency, error) {
	// Resolve detector URI to command and args
	// Use GetEffectiveVersion to handle both ldflags version and go run @version
	cmd, args, err := engineframework.ResolveDetector("forge://go-gen-mocks-dep-detector", engineversion.GetEffectiveVersion(Version))
	if err != nil {
		return nil, err
	}

	// Handle empty RootDir (use current working directory)
	workDir := rootDir
	if workDir == "" {
		workDir, _ = os.Getwd()
	}

	input := map[string]any{
		"rootDir": workDir,
	}

	return engineframework.CallDetector(ctx, cmd, args, "detectDependencies", input)
}

func getMocksDir(mocksDir string) string {
	if mocksDir != "" {
		return mocksDir
	}
	if envDir := os.Getenv("MOCKS_DIR"); envDir != "" {
		return envDir
	}
	return "./internal/util/mocks"
}

func generateMocks(mocksDir string) error {
	mockeryVersion := os.Getenv("MOCKERY_VERSION")

	// Clean mocks directory
	dir := getMocksDir(mocksDir)
	if err := os.RemoveAll(dir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to clean mocks directory: %w", err)
	}

	// A provisioned binary wins when no explicit version is demanded: the
	// workspace's .forge/bin sits first on PATH carrying the pinned build
	// the factory's toolchain section resolved. MOCKERY_VERSION still
	// forces the go run form - an explicit demand outranks whatever is
	// installed.
	var cmd *exec.Cmd

	if path, err := exec.LookPath("mockery"); err == nil && mockeryVersion == "" {
		cmd = exec.Command(path)
	} else {
		if mockeryVersion == "" {
			mockeryVersion = "v3.5.5"
		}

		cmd = exec.Command("go", "run", fmt.Sprintf("github.com/vektra/mockery/v3@%s", mockeryVersion))
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mockery failed: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Successfully generated mocks in %s\n", dir)
	return nil
}
