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
	"time"

	"github.com/alexandremahdhaoui/forge/pkg/engineframework"
	"github.com/alexandremahdhaoui/forge/pkg/forge"
	"github.com/alexandremahdhaoui/forge/pkg/mcptypes"
)

// Build implements the BuildFunc for compiling Protocol Buffer files to Go code.
// It uses the typed Spec provided by the generated MCP server setup.
func Build(ctx context.Context, input mcptypes.BuildInput, spec *Spec) ([]forge.Artifact, error) {
	// 1. Log start
	log.Printf("Generating protobuf code for: %s", input.Name)

	// 2. Validate required inputs
	if input.Src == "" {
		return nil, fmt.Errorf("src is required")
	}
	if input.Dest == "" {
		return nil, fmt.Errorf("dest is required")
	}

	// 3. Discover proto files
	protoFiles, err := discoverProtoFiles(input.Src)
	if err != nil {
		return nil, fmt.Errorf("failed to discover proto files: %w", err)
	}
	if len(protoFiles) == 0 {
		return nil, fmt.Errorf("no .proto files found in %s", input.Src)
	}
	log.Printf("Found %d proto files", len(protoFiles))

	// 4. Build dependencies list
	deps, err := buildDependencies(input.Src, protoFiles)
	if err != nil {
		return nil, fmt.Errorf("failed to build dependencies: %w", err)
	}

	// 5. Build absolute proto file paths for protoc
	absProtoFiles := make([]string, len(protoFiles))
	for i, pf := range protoFiles {
		absProtoFiles[i] = filepath.Join(input.Src, pf)
	}

	// 6. Create destination directory
	if err := os.MkdirAll(input.Dest, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create destination directory: %w", err)
	}

	// 7. Build and execute protoc command using typed Spec
	cmd := buildProtocCommand(input.Src, input.Dest, absProtoFiles, spec)
	cmd.Stdout = os.Stderr // MCP mode: redirect to stderr
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("protoc failed: %w", err)
	}

	// 8. Create artifact (DIRECTLY, not using engineframework.CreateArtifact)
	artifact := &forge.Artifact{
		Name:                     input.Name,
		Type:                     forge.TypeProtobuf,
		Location:                 input.Dest,
		Timestamp:                time.Now().UTC().Format(time.RFC3339),
		Dependencies:             deps,
		DependencyDetectorEngine: "forge://go-gen-protobuf",
	}

	log.Printf("Successfully generated protobuf code for %s", input.Name)

	return engineframework.One(artifact), nil
}

// discoverProtoFiles recursively discovers all .proto files in the given directory.
// Returns a slice of relative paths (relative to srcDir).
// Skips hidden directories (starting with '.') and the 'vendor' directory.
func discoverProtoFiles(srcDir string) ([]string, error) {
	// Check if directory exists
	info, err := os.Stat(srcDir)
	if err != nil {
		return nil, fmt.Errorf("failed to access directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", srcDir)
	}

	var protoFiles []string

	err = filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden directories
		if info.IsDir() && strings.HasPrefix(info.Name(), ".") {
			return filepath.SkipDir
		}

		// Skip vendor directory
		if info.IsDir() && info.Name() == "vendor" {
			return filepath.SkipDir
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Only include .proto files
		if strings.HasSuffix(info.Name(), ".proto") {
			// Get relative path
			relPath, err := filepath.Rel(srcDir, path)
			if err != nil {
				return fmt.Errorf("failed to get relative path: %w", err)
			}
			protoFiles = append(protoFiles, relPath)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	return protoFiles, nil
}

// buildDependencies creates ArtifactDependency entries for each proto file.
// The protoFiles should be relative paths (relative to srcDir).
// Returns dependencies with absolute paths and RFC3339 timestamps.
func buildDependencies(srcDir string, protoFiles []string) ([]forge.ArtifactDependency, error) {
	deps := make([]forge.ArtifactDependency, 0, len(protoFiles))

	for _, protoFile := range protoFiles {
		// Resolve to absolute path
		absPath, err := filepath.Abs(filepath.Join(srcDir, protoFile))
		if err != nil {
			return nil, fmt.Errorf("failed to resolve absolute path for %s: %w", protoFile, err)
		}

		// Get file modification time
		info, err := os.Stat(absPath)
		if err != nil {
			return nil, fmt.Errorf("failed to stat file %s: %w", absPath, err)
		}

		// Format timestamp as RFC3339 UTC
		timestamp := info.ModTime().UTC().Format(time.RFC3339)

		deps = append(deps, forge.ArtifactDependency{
			Type:      forge.DependencyTypeFile,
			FilePath:  absPath,
			Timestamp: timestamp,
		})
	}

	return deps, nil
}

// buildProtocCommand constructs the protoc command with all arguments.
// Arguments order:
//  1. --proto_path={srcDir} (FIRST - required for import resolution)
//  2. --go_out={dest}
//  3. --go_opt (default "paths=source_relative")
//  4. --go-grpc_out={dest}
//  5. --go-grpc_opt (default "paths=source_relative")
//  6. User proto_paths: --proto_path={path}
//  7. Plugins: --plugin={plugin}
//  8. Extra args
//  9. Proto files (at the end)
func buildProtocCommand(srcDir string, dest string, protoFiles []string, spec *Spec) *exec.Cmd {
	args := make([]string, 0)

	// 1. Source directory proto_path FIRST (required for import resolution)
	args = append(args, "--proto_path="+srcDir)

	// 2. Go output directory
	args = append(args, "--go_out="+dest)

	// 3. Go options (default to "paths=source_relative")
	goOpt := spec.GoOpt
	if goOpt == "" {
		goOpt = "paths=source_relative"
	}
	args = append(args, "--go_opt="+goOpt)

	// 4. gRPC Go output directory
	args = append(args, "--go-grpc_out="+dest)

	// 5. gRPC Go options (default to "paths=source_relative")
	goGrpcOpt := spec.GoGrpcOpt
	if goGrpcOpt == "" {
		goGrpcOpt = "paths=source_relative"
	}
	args = append(args, "--go-grpc_opt="+goGrpcOpt)

	// 6. User-specified proto_paths
	for _, protoPath := range spec.ProtoPath {
		args = append(args, "--proto_path="+protoPath)
	}

	// 7. Plugins
	for _, plugin := range spec.Plugin {
		args = append(args, "--plugin="+plugin)
	}

	// 8. Extra args
	args = append(args, spec.ExtraArgs...)

	// 9. Proto files at the end
	args = append(args, protoFiles...)

	return exec.Command("protoc", args...)
}
