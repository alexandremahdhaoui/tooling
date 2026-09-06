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
// Go source files. It never rewrites a file that already has a license
// header, so an existing copyright year is never touched. Safe to run
// repeatedly: a file with a header is always left untouched.
//
// This is go-lint-licenses' missing other half: go-lint-licenses only
// verifies and fails a build when a header is missing, it never writes one.
// This engine is what actually fixes the failure it reports.
//
// The actual scan/detect/write logic lives in internal/licenseheader,
// shared with rust-license-header and go-lint-licenses. This file is just
// the Go-specific wiring: the .go extension and this engine's Spec.
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

	log.Printf("Adding license headers to Go files at: %s", rootDir)

	exclude := licenseheader.DefaultExcludeDirs()
	for _, name := range spec.ExcludeDirs {
		exclude[name] = true
	}

	stats, err := licenseheader.AddHeaders(rootDir, spec.Holder, []string{".go"}, exclude)
	if err != nil {
		return nil, fmt.Errorf("adding license headers: %w", err)
	}

	fmt.Fprintf(
		os.Stderr,
		"go-license-header: %d file(s) scanned, %d already had a header, %d generated file(s) skipped, %d header(s) added\n",
		stats.Total, stats.AlreadyLicensed, stats.GeneratedSkipped, stats.Added,
	)

	return engineframework.One(engineframework.CreateArtifact(
		"go-license-headers",
		forge.TypeGenerated,
		rootDir,
	)), nil
}
