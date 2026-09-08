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

// buildBpf2goArgs constructs the bpf2go command line arguments.
// Arguments order:
//  1. --go-package=<pkg>
//  2. --output-dir=<dest>
//  3. --output-stem=<stem>
//  4. --tags=<t1>,<t2>
//  5. --type=<t1> --type=<t2> (for each type)
//  6. <ident>
//  7. <src>
//  8. -- <cflags...> (if any)
func buildBpf2goArgs(src, dest string, spec *Spec) []string {
	args := make([]string, 0)

	// Compute goPackage with default
	goPackage := spec.GoPackage
	if goPackage == "" {
		goPackage = filepath.Base(dest)
	}

	// Compute outputStem with default
	outputStem := spec.OutputStem
	if outputStem == "" {
		outputStem = "zz_generated"
	}

	// Compute tags with default
	tags := spec.Tags
	if len(tags) == 0 {
		tags = []string{"linux"}
	}

	// 1. Go package
	args = append(args, "--go-package", goPackage)

	// 2. Output directory
	args = append(args, "--output-dir", dest)

	// 3. Output stem
	args = append(args, "--output-stem", outputStem)

	// 4. Tags (comma-joined)
	if len(tags) > 0 {
		args = append(args, "--tags", strings.Join(tags, ","))
	}

	// 5. Types (one --type flag per type)
	for _, t := range spec.Types {
		args = append(args, "--type", t)
	}

	// 6. Ident
	args = append(args, spec.Ident)

	// 7. Source file
	args = append(args, src)

	// 8. C flags after "--" separator
	if len(spec.Cflags) > 0 {
		args = append(args, "--")
		args = append(args, spec.Cflags...)
	}

	return args
}

// Build implements the BuildFunc for generating Go code from BPF C source files
func Build(ctx context.Context, input mcptypes.BuildInput, spec *Spec) ([]forge.Artifact, error) {
	// 1. Log start
	log.Printf("Generating BPF code for: %s", input.Name)

	// 2. Validate required inputs
	if input.Src == "" {
		return nil, fmt.Errorf("src is required")
	}
	if input.Dest == "" {
		return nil, fmt.Errorf("dest is required")
	}

	// 3. Validate source file exists and is a file (not directory)
	srcInfo, err := os.Stat(input.Src)
	if err != nil {
		return nil, fmt.Errorf("source file not found: %s", input.Src)
	}
	if srcInfo.IsDir() {
		return nil, fmt.Errorf("src must be a file, not directory")
	}
	log.Printf("Source file: %s", input.Src)

	// 4. Create destination directory
	if err := os.MkdirAll(input.Dest, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create dest directory: %w", err)
	}

	// 5. Build bpf2go command arguments using typed spec
	bpf2goArgs := buildBpf2goArgs(input.Src, input.Dest, spec)

	// 6. Execute bpf2go via "go run" (following go-gen-mocks pattern)
	// Compute bpf2goVersion with default
	bpf2goVersion := spec.Bpf2goVersion
	if bpf2goVersion == "" {
		bpf2goVersion = "latest"
	}
	bpf2goTool := fmt.Sprintf("github.com/cilium/ebpf/cmd/bpf2go@%s", bpf2goVersion)
	args := []string{"run", bpf2goTool}
	args = append(args, bpf2goArgs...)

	log.Printf("Executing: go %s", strings.Join(args, " "))

	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Stdout = os.Stderr // MCP mode: redirect to stderr
	cmd.Stderr = os.Stderr

	// Set CC environment variable if specified (preserve existing environment)
	if spec.Cc != "" {
		cmd.Env = append(os.Environ(), "BPF2GO_CC="+spec.Cc)
	}

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("bpf2go failed: %w", err)
	}

	// 7. Build dependencies
	deps, err := buildDependencies(input.Src, srcInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to build dependencies: %w", err)
	}

	// 8. Create and return artifact
	artifact := &forge.Artifact{
		Name:                     input.Name,
		Type:                     forge.TypeBPF,
		Location:                 input.Dest,
		Timestamp:                time.Now().UTC().Format(time.RFC3339),
		Dependencies:             deps,
		DependencyDetectorEngine: "forge://go-gen-bpf",
	}

	log.Printf("Successfully generated BPF code for %s", input.Name)

	return engineframework.One(artifact), nil
}

// buildDependencies creates ArtifactDependency entries for the source file.
// Returns dependencies with absolute paths and RFC3339 timestamps.
func buildDependencies(srcPath string, srcInfo os.FileInfo) ([]forge.ArtifactDependency, error) {
	// Resolve to absolute path
	absPath, err := filepath.Abs(srcPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve absolute path for %s: %w", srcPath, err)
	}

	dep, err := forge.DependencyOf(absPath)
	if err != nil {
		return nil, err
	}

	return []forge.ArtifactDependency{dep}, nil
}
