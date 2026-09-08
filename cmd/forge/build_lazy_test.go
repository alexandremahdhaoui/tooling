//go:build unit

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
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alexandremahdhaoui/forge/pkg/forge"
)

// The freshness rule is the comparison of content digests and nothing
// else. These cases are the whole of it: no record rebuilds; a record with
// no dependencies rebuilds; a missing or changed dependency rebuilds; an
// output that carries a digest and is missing or edited rebuilds; a touch
// that changes no byte rebuilds nothing.

func writeFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func recorded(t *testing.T, path string) forge.ArtifactDependency {
	t.Helper()

	dep, err := forge.DependencyOf(path)
	if err != nil {
		t.Fatal(err)
	}

	return dep
}

func storeWith(artifact forge.Artifact) forge.ArtifactStore {
	artifact.Timestamp = time.Now().UTC().Format(time.RFC3339)

	return forge.ArtifactStore{Artifacts: []forge.Artifact{artifact}}
}

func binaryArtifact(t *testing.T, output string, deps ...forge.ArtifactDependency) forge.Artifact {
	t.Helper()

	digest, err := forge.DigestFile(output)
	if err != nil {
		t.Fatal(err)
	}

	return forge.Artifact{
		Name: "test-artifact", Type: forge.TypeBinary, Location: output, Version: "v1",
		Digest: digest, Dependencies: deps, DependencyDetectorEngine: "forge://go-dependency-detector",
	}
}

func TestShouldRebuild_NoPreviousBuild(t *testing.T) {
	rebuild, reason, err := shouldRebuild("test-artifact", []string{hostPlatform()}, forge.ArtifactStore{})
	if err != nil {
		t.Fatal(err)
	}

	if !rebuild || !strings.Contains(reason, "no previous build") {
		t.Fatalf("no record must rebuild, got %v %q", rebuild, reason)
	}
}

func TestShouldRebuild_DependenciesNotTracked(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "bin")
	writeFile(t, out, "binary")

	store := storeWith(binaryArtifact(t, out))

	rebuild, reason, err := shouldRebuild("test-artifact", []string{hostPlatform()}, store)
	if err != nil {
		t.Fatal(err)
	}

	if !rebuild || reason != "dependencies not tracked" {
		t.Fatalf("a record with no dependencies must rebuild, got %v %q", rebuild, reason)
	}
}

func TestShouldRebuild_DependencyMissing(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "bin")
	writeFile(t, out, "binary")

	gone := filepath.Join(dir, "gone.go")
	writeFile(t, gone, "package main")
	dep := recorded(t, gone)
	_ = os.Remove(gone)

	store := storeWith(binaryArtifact(t, out, dep))

	rebuild, reason, err := shouldRebuild("test-artifact", []string{hostPlatform()}, store)
	if err != nil {
		t.Fatal(err)
	}

	if !rebuild || !strings.Contains(reason, "missing") {
		t.Fatalf("a missing dependency must rebuild, got %v %q", rebuild, reason)
	}
}

func TestShouldRebuild_DependencyChanged(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "bin")
	writeFile(t, out, "binary")

	src := filepath.Join(dir, "main.go")
	writeFile(t, src, "package main")
	dep := recorded(t, src)
	writeFile(t, src, "package main // edited")

	store := storeWith(binaryArtifact(t, out, dep))

	rebuild, reason, err := shouldRebuild("test-artifact", []string{hostPlatform()}, store)
	if err != nil {
		t.Fatal(err)
	}

	if !rebuild || !strings.Contains(reason, "changed") {
		t.Fatalf("a changed dependency must rebuild, got %v %q", rebuild, reason)
	}
}

// A touch is not a change. The old rule compared modification times to the
// second, so a fresh clone, a checkout, or a stray touch rebuilt everything
// it had no reason to; the digest sees the same bytes and skips.
func TestShouldRebuild_TouchWithoutEditDoesNotRebuild(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "bin")
	writeFile(t, out, "binary")

	src := filepath.Join(dir, "main.go")
	writeFile(t, src, "package main")
	dep := recorded(t, src)

	later := time.Now().Add(2 * time.Hour)
	if err := os.Chtimes(src, later, later); err != nil {
		t.Fatal(err)
	}

	store := storeWith(binaryArtifact(t, out, dep))

	rebuild, reason, err := shouldRebuild("test-artifact", []string{hostPlatform()}, store)
	if err != nil {
		t.Fatal(err)
	}

	if rebuild {
		t.Fatalf("a touch that changes no byte must not rebuild, got %q", reason)
	}
}

func TestShouldRebuild_AllUnchanged(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "bin")
	writeFile(t, out, "binary")

	a := filepath.Join(dir, "a.go")
	b := filepath.Join(dir, "b.go")
	writeFile(t, a, "package main // a")
	writeFile(t, b, "package main // b")

	store := storeWith(binaryArtifact(t, out, recorded(t, a), recorded(t, b)))

	rebuild, reason, err := shouldRebuild("test-artifact", []string{hostPlatform()}, store)
	if err != nil {
		t.Fatal(err)
	}

	if rebuild {
		t.Fatalf("unchanged inputs and output must not rebuild, got %q", reason)
	}
}

// The output carries its digest, so an edited binary is stale by the same
// rule as an edited source, and a deleted one likewise.
func TestShouldRebuild_OutputEditedOrMissing(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "bin")
	writeFile(t, out, "binary")

	src := filepath.Join(dir, "main.go")
	writeFile(t, src, "package main")

	store := storeWith(binaryArtifact(t, out, recorded(t, src)))

	writeFile(t, out, "binary, edited by hand")

	rebuild, reason, err := shouldRebuild("test-artifact", []string{hostPlatform()}, store)
	if err != nil {
		t.Fatal(err)
	}

	if !rebuild || !strings.Contains(reason, "changed since it was built") {
		t.Fatalf("an edited output must rebuild, got %v %q", rebuild, reason)
	}

	_ = os.Remove(out)

	rebuild, reason, err = shouldRebuild("test-artifact", []string{hostPlatform()}, store)
	if err != nil {
		t.Fatal(err)
	}

	if !rebuild || !strings.Contains(reason, "missing") {
		t.Fatalf("a missing output must rebuild, got %v %q", rebuild, reason)
	}
}

// An artifact with no digest of its own - an image layout, a directory of
// generated files - is judged on its dependencies alone, so its location
// is never opened.
func TestShouldRebuild_AnOutputWithNoDigestIsJudgedOnItsInputs(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "spec.yaml")
	writeFile(t, src, "openapi: 3.0.0")

	store := storeWith(forge.Artifact{
		Name: "test-artifact", Type: forge.TypeContainer, Location: "file://" + filepath.Join(dir, "never-here"), Version: "v1",
		Dependencies: []forge.ArtifactDependency{recorded(t, src)}, DependencyDetectorEngine: "forge://x",
	})

	rebuild, reason, err := shouldRebuild("test-artifact", []string{hostPlatform()}, store)
	if err != nil {
		t.Fatal(err)
	}

	if rebuild {
		t.Fatalf("unchanged inputs must not rebuild an artifact that carries no digest, got %q", reason)
	}
}
