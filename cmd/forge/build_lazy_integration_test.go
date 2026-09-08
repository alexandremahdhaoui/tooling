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

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alexandremahdhaoui/forge/pkg/forge"
)

// clearArtifactsOnly removes all artifacts from the store while preserving
// testEnvironments and testReports, so a test starts fresh on artifacts
// without interfering with testenv cleanup.
func clearArtifactsOnly(t *testing.T, artifactStorePath string) {
	t.Helper()

	store, err := forge.ReadArtifactStore(artifactStorePath)
	if err != nil {
		return
	}

	store.Artifacts = nil

	if err := forge.WriteArtifactStore(artifactStorePath, store); err != nil {
		t.Logf("Warning: failed to clear artifacts: %v", err)
	}
}

// lazyFixture is a one-binary Go module in a temporary directory with its
// own forge.yaml, so every test here edits files nobody else owns. The
// freshness rule is content: a touch changes nothing, a one-byte edit
// changes everything, and a hand-edited output is stale by the same rule.
type lazyFixture struct {
	dir      string
	forgeBin string
	repoRoot string
	main     string
	goSum    string
	output   string
	store    string
}

func newLazyFixture(t *testing.T) *lazyFixture {
	t.Helper()

	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}

	forgeBin := filepath.Join(repoRoot, "build", "bin", "forge")
	build := exec.Command("go", "build", "-o", forgeBin, "./cmd/forge")
	build.Dir = repoRoot
	build.Env = append(os.Environ(), "GOWORK=off")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building forge: %v\n%s", err, out)
	}

	dir := t.TempDir()
	files := map[string]string{
		"forge.yaml": `name: lazy-fixture
artifactStorePath: .forge/artifact-store.yaml
build:
  - name: app
    src: ./cmd/app
    dest: ./build/bin
    engine: forge://go-build
`,
		"go.mod": "module example.invalid/lazy\n\ngo 1.24\n",
		// An entry for a module nothing requires: go tolerates it, the
		// detector records the file, and a test can change it safely.
		"go.sum":          "example.invalid/unused v1.0.0 h1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\n",
		"cmd/app/main.go": "package main\n\nfunc main() {}\n",
		".gitignore":      "/build/\n/.forge/\n",
		".envrc":          "",
	}

	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// go-build stamps binaries with a git SHA and refuses a directory with
	// no commit, so the fixture is a repository with one.
	for _, argv := range [][]string{
		{"git", "init", "-q"},
		{"git", "-c", "user.name=probe", "-c", "user.email=probe@example.invalid", "add", "."},
		{"git", "-c", "user.name=probe", "-c", "user.email=probe@example.invalid", "-c", "commit.gpgsign=false", "commit", "-q", "-m", "fixture"},
	} {
		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.Dir = dir

		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %v\n%s", argv, err, out)
		}
	}

	return &lazyFixture{
		dir:      dir,
		forgeBin: forgeBin,
		repoRoot: repoRoot,
		main:     filepath.Join(dir, "cmd", "app", "main.go"),
		goSum:    filepath.Join(dir, "go.sum"),
		output:   filepath.Join(dir, "build", "bin", "app"),
		store:    filepath.Join(dir, ".forge", "artifact-store.yaml"),
	}
}

// build runs `forge build app` in the fixture and answers what forge said.
func (f *lazyFixture) build(t *testing.T) string {
	t.Helper()

	cmd := exec.Command(f.forgeBin, "build", "app")
	cmd.Dir = f.dir
	cmd.Env = append(os.Environ(),
		"GOWORK=off",
		"FORGE_RUN_LOCAL_ENABLED=true",
		"FORGE_RUN_LOCAL_BASEDIR="+f.repoRoot,
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("forge build: %v\n%s", err, out)
	}

	return string(out)
}

func (f *lazyFixture) expectBuilt(t *testing.T, out, reason string) {
	t.Helper()

	if !strings.Contains(out, "Building app") || !strings.Contains(out, reason) {
		t.Fatalf("expected a build for %q, got:\n%s", reason, out)
	}
}

func (f *lazyFixture) expectSkipped(t *testing.T, out string) {
	t.Helper()

	if !strings.Contains(out, "Skipping app (unchanged)") {
		t.Fatalf("expected the build to be skipped, got:\n%s", out)
	}
}

func (f *lazyFixture) record(t *testing.T) forge.Artifact {
	t.Helper()

	store, err := forge.ReadArtifactStore(f.store)
	if err != nil {
		t.Fatalf("reading the store: %v", err)
	}

	artifact, err := forge.GetLatestArtifact(store, "app", hostPlatform())
	if err != nil {
		t.Fatalf("the record for app: %v", err)
	}

	return artifact
}

func appendTo(t *testing.T, path, text string) {
	t.Helper()

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := f.WriteString(text); err != nil {
		t.Fatal(err)
	}

	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

// The first build has no record; the detector records go.mod, go.sum and
// the entry file with their digests, and the output with its own; the
// second build finds every digest unchanged and skips.
func TestLazyRebuild_ARecordedBuildIsSkipped(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	f := newLazyFixture(t)
	f.expectBuilt(t, f.build(t), "no previous build")

	artifact := f.record(t)
	for _, want := range []string{"go.mod", "go.sum", "main.go"} {
		found := false
		for _, dep := range artifact.Dependencies {
			if filepath.Base(dep.Path) == want {
				found = true
				if !strings.HasPrefix(dep.Digest, forge.DigestPrefix) {
					t.Errorf("%s recorded with no digest: %+v", want, dep)
				}
			}
		}
		if !found {
			t.Errorf("%s is not among the recorded dependencies: %+v", want, artifact.Dependencies)
		}
	}
	if !strings.HasPrefix(artifact.Digest, forge.DigestPrefix) {
		t.Errorf("the output carries no digest: %+v", artifact)
	}

	f.expectSkipped(t, f.build(t))
}

// A touch changes a modification time and nothing else. The rule reads
// content, so nothing rebuilds. This is the case that separates digests
// from the mtime rule they replaced: a fresh clone dates every file today.
func TestLazyRebuild_ATouchDoesNotRebuild(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	f := newLazyFixture(t)
	f.build(t)

	now := time.Now().Add(time.Hour)
	for _, path := range []string{f.main, f.goSum, filepath.Join(f.dir, "go.mod")} {
		if err := os.Chtimes(path, now, now); err != nil {
			t.Fatal(err)
		}
	}

	f.expectSkipped(t, f.build(t))
}

// One byte in the entry file rebuilds, naming the file.
func TestLazyRebuild_AnEditedSourceRebuilds(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	f := newLazyFixture(t)
	f.build(t)
	appendTo(t, f.main, "\n// edited\n")
	f.expectBuilt(t, f.build(t), "dependency "+f.main+" changed")
	f.expectSkipped(t, f.build(t))
}

// The lock is a dependency beside the manifest: a changed go.sum is a
// changed closure, and the module it names is not recorded on its own.
func TestLazyRebuild_AChangedGoSumRebuilds(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	f := newLazyFixture(t)
	f.build(t)
	appendTo(t, f.goSum, "example.invalid/unused v1.0.1 h1:BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB=\n")
	f.expectBuilt(t, f.build(t), "dependency "+f.goSum+" changed")
}

// The output carries its own digest, so a binary edited after the build
// is stale by the same rule as an edited source, and a deleted one too.
func TestLazyRebuild_AHandEditedOutputRebuilds(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	f := newLazyFixture(t)
	f.build(t)
	appendTo(t, f.output, "\x00")
	f.expectBuilt(t, f.build(t), "changed since it was built")
	f.expectSkipped(t, f.build(t))
}

func TestLazyRebuild_ArtifactDeleted(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	f := newLazyFixture(t)
	f.build(t)

	if err := os.Remove(f.output); err != nil {
		t.Fatal(err)
	}

	f.expectBuilt(t, f.build(t), "missing")
}

// A record with no dependencies says nothing about freshness, so it
// rebuilds: the rule trusts a detector's answer and never a bare mtime.
func TestLazyRebuild_WithoutDependencyTracking(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	f := newLazyFixture(t)
	f.build(t)

	store, err := forge.ReadArtifactStore(f.store)
	if err != nil {
		t.Fatal(err)
	}
	for i := range store.Artifacts {
		store.Artifacts[i].Dependencies = nil
	}
	if err := forge.WriteArtifactStore(f.store, store); err != nil {
		t.Fatal(err)
	}

	f.expectBuilt(t, f.build(t), "dependencies not tracked")
	f.expectSkipped(t, f.build(t))
}
