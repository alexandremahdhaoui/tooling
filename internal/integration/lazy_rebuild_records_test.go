//go:build integration

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

// Package integration contains end-to-end integration tests for forge components.
package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/alexandremahdhaoui/forge/pkg/forge"
)

// TestLazyRebuildFollowsAnyDetectorsRecords pins that the freshness rule
// reads the record and nothing else: a generator's detector (mocks,
// openapi) records the files it read and the files it wrote, and forge
// judges them by digest without knowing which engine wrote them. The test
// stands in for those detectors by writing the records itself, over files
// it owns, so a touch can be proven inert and an edit proven decisive.
func TestLazyRebuildFollowsAnyDetectorsRecords(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	t.Setenv("FORGE_RUN_LOCAL_ENABLED", "true")

	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("FORGE_RUN_LOCAL_BASEDIR", repoRoot)

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Failed to change to repo root: %v", err)
	}

	forgeBin := "./build/bin/forge"
	buildForge := exec.Command("go", "build", "-o", forgeBin, "./cmd/forge")
	buildForge.Env = append(os.Environ(), "GOWORK=off")
	if out, err := buildForge.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build forge: %v\n%s", err, out)
	}

	artifactStorePath := ".forge/test-lazy-records-artifact-store.yaml"
	_ = os.Remove(artifactStorePath)
	defer func() { _ = os.Remove(artifactStorePath) }()

	testForgeYamlPath := "forge-test-lazy-records.yaml"
	testForgeYaml := `name: lazy-records-test
artifactStorePath: ` + artifactStorePath + `

build:
  - name: test-records-artifact
    src: ./cmd/go-lint
    dest: ./build/bin
    engine: forge://go-build
`
	if err := os.WriteFile(testForgeYamlPath, []byte(testForgeYaml), 0o644); err != nil {
		t.Fatalf("Failed to write test forge.yaml: %v", err)
	}
	defer func() { _ = os.Remove(testForgeYamlPath) }()

	build := func(step string) string {
		t.Helper()
		t.Log(step)

		cmd := exec.Command(forgeBin, "--config", testForgeYamlPath, "build", "test-records-artifact")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("forge build failed: %v\nOutput: %s", err, out)
		}

		return string(out)
	}

	out := build("Step 1: first build")
	if !strings.Contains(out, "Building test-records-artifact") {
		t.Fatalf("Expected a build, got: %s", out)
	}

	// The records a generator's detector would write: a config it read and
	// a file it wrote, both owned by this test.
	configFile := filepath.Join(t.TempDir(), "generator.yaml")
	writtenFile := filepath.Join(t.TempDir(), "zz_generated.out.go")
	for path, content := range map[string]string{configFile: "packages: {}\n", writtenFile: "package out\n"} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	record := func(step string) {
		t.Helper()
		t.Log(step)

		store, err := forge.ReadArtifactStore(artifactStorePath)
		if err != nil {
			t.Fatalf("Failed to read artifact store: %v", err)
		}

		platform := runtime.GOOS + "/" + runtime.GOARCH
		artifact, err := forge.GetLatestArtifact(store, "test-records-artifact", platform)
		if err != nil {
			t.Fatalf("Artifact not found in store: %v", err)
		}

		artifact.Dependencies = nil
		for _, path := range []string{configFile, writtenFile} {
			dep, err := forge.DependencyOf(path)
			if err != nil {
				t.Fatal(err)
			}
			artifact.Dependencies = append(artifact.Dependencies, dep)
		}
		artifact.DependencyDetectorEngine = "forge://a-generator-of-this-tests-choosing"
		forge.AddOrUpdateArtifact(&store, artifact)

		if err := forge.WriteArtifactStore(artifactStorePath, store); err != nil {
			t.Fatalf("Failed to write artifact store: %v", err)
		}
	}

	record("Step 2: record the generator's reads and writes by digest")

	out = build("Step 3: nothing changed, the build is skipped")
	if !strings.Contains(out, "Skipping test-records-artifact (unchanged)") {
		t.Fatalf("Expected 'Skipping', got: %s", out)
	}

	now := time.Now().Add(time.Hour)
	for _, path := range []string{configFile, writtenFile} {
		if err := os.Chtimes(path, now, now); err != nil {
			t.Fatal(err)
		}
	}

	out = build("Step 4: a touch is not a change")
	if !strings.Contains(out, "Skipping test-records-artifact (unchanged)") {
		t.Fatalf("Expected 'Skipping' after a touch, got: %s", out)
	}

	if err := os.WriteFile(writtenFile, []byte("package out // hand-edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out = build("Step 5: a hand-edited generated file regenerates")
	if !strings.Contains(out, "Building test-records-artifact") || !strings.Contains(out, "dependency "+writtenFile+" changed") {
		t.Fatalf("Expected a build naming the edited file, got: %s", out)
	}

	record("Step 6: re-record after the rebuild")

	out = build("Step 7: converged again")
	if !strings.Contains(out, "Skipping test-records-artifact (unchanged)") {
		t.Fatalf("Expected 'Skipping' after re-recording, got: %s", out)
	}
}
