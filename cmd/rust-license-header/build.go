// Copyright 2026 Alexandre Mahdhaoui
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

	"github.com/alexandremahdhaoui/forge/internal/licenseheader"
	"github.com/alexandremahdhaoui/forge/pkg/engineframework"
	"github.com/alexandremahdhaoui/forge/pkg/forge"
	"github.com/alexandremahdhaoui/forge/pkg/mcptypes"
)

// Build implements the BuildFunc for adding Apache License 2.0 headers to
// Rust source files. It never rewrites a file that already has a license
// header, so an existing copyright year is never touched. Safe to run
// repeatedly: a file with a header is always left untouched.
//
// The actual scan/detect/write logic lives in internal/licenseheader,
// shared with go-license-header and go-lint-licenses. This file is just the
// Rust-specific wiring: the .rs extension and this engine's Spec.
func Build(ctx context.Context, input mcptypes.BuildInput, spec *Spec) ([]forge.Artifact, error) {
	rootDir := spec.RootDir
	if rootDir == "" {
		rootDir = input.Path
	}
	if rootDir == "" && input.Src != "" {
		rootDir = input.Src
	}
	if rootDir == "" {
		rootDir = "."
	}

	log.Printf("Adding license headers to Rust files at: %s", rootDir)

	exclude := licenseheader.DefaultExcludeDirs()
	for _, name := range spec.ExcludeDirs {
		exclude[name] = true
	}

	stats, err := licenseheader.AddHeaders(rootDir, spec.Holder, []string{".rs"}, exclude)
	if err != nil {
		return nil, fmt.Errorf("adding license headers: %w", err)
	}

	fmt.Fprintf(
		os.Stderr,
		"rust-license-header: %d file(s) scanned, %d already had a header, %d generated file(s) skipped, %d header(s) added\n",
		stats.Total, stats.AlreadyLicensed, stats.GeneratedSkipped, stats.Added,
	)

	return engineframework.One(engineframework.CreateArtifact(
		"rust-license-headers",
		forge.TypeGenerated,
		rootDir,
	)), nil
}
