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
	"github.com/alexandremahdhaoui/forge/pkg/forge"
	"github.com/alexandremahdhaoui/forge/pkg/mcptypes"
)

// Build implements the BuildFunc for formatting Go code
func Build(ctx context.Context, input mcptypes.BuildInput, spec *Spec) ([]forge.Artifact, error) {
	// Use spec.Path if set, otherwise fall back to input.Path or input.Src
	path := spec.Path
	if path == "" {
		path = input.Path
	}
	if path == "" && input.Src != "" {
		path = input.Src
	}
	if path == "" {
		path = "."
	}

	log.Printf("Formatting Go code at: %s", path)

	if err := formatCode(path); err != nil {
		return nil, fmt.Errorf("formatting failed: %w", err)
	}

	// Return artifact using CreateArtifact (formatted code has no version)
	return engineframework.One(engineframework.CreateArtifact(
		input.Name,
		forge.TypeGenerated,
		path,
	)), nil
}

func formatCode(path string) error {
	gofumptVersion := os.Getenv("GOFUMPT_VERSION")
	if gofumptVersion == "" {
		gofumptVersion = "v0.6.0"
	}

	gofumptPkg := fmt.Sprintf("mvdan.cc/gofumpt@%s", gofumptVersion)

	cmd := exec.Command("go", "run", gofumptPkg, "-w", path)
	cmd.Stdout = os.Stderr // Send to stderr to not interfere with MCP JSON-RPC on stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gofumpt failed: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Formatted Go code at %s\n", path)
	return nil
}
