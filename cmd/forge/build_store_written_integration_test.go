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
	"testing"
)

// TestALaterGroupReadsWhatEarlierGroupsRecorded pins the store write
// between engine groups. An entry may read its own repository's records
// (an image assembled from the binaries the entries before it built), and
// it reads them from the store file. Written once at the end of the whole
// build, that file did not exist on a fresh clone when the entry ran, and
// the build died on its own repo. Two groups here: the first records an
// artifact, the second is a different engine URI that greps the record
// out of the file.
func TestALaterGroupReadsWhatEarlierGroupsRecorded(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	repoRoot := "../.."
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Failed to change to repo root: %v", err)
	}

	forgeBin := "./build/bin/forge"
	cmd := exec.Command("go", "build", "-o", forgeBin, "./cmd/forge")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to build forge: %v", err)
	}

	// A fresh clone has no store: the file must be absent when the build
	// starts, or the test proves nothing.
	artifactStorePath := ".forge/test-store-written-artifact-store.yaml"
	_ = os.Remove(artifactStorePath)
	defer func() { _ = os.Remove(artifactStorePath) }()

	testForgeYaml := `name: store-written-test
envFile: .envrc
artifactStorePath: ` + artifactStorePath + `

engines:
  - alias: reads-the-store
    type: builder
    builder:
      - engine: forge://generic-builder

build:
  - name: recorded-first
    src: .
    dest: ./build/bin
    engine: forge://generic-builder
    spec:
      command: "true"
  - name: reads-the-record
    src: .
    dest: ./build/bin
    engine: alias://reads-the-store
    spec:
      command: sh
      args: ["-c", "grep -q 'name: recorded-first' ` + artifactStorePath + `"]
`
	testForgeYamlPath := "forge-test-store-written.yaml"
	if err := os.WriteFile(testForgeYamlPath, []byte(testForgeYaml), 0o644); err != nil {
		t.Fatalf("Failed to write test forge.yaml: %v", err)
	}
	defer func() { _ = os.Remove(testForgeYamlPath) }()

	cmd = exec.Command(forgeBin, "--config", testForgeYamlPath, "build")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("the second group could not read the first group's record: %v\nOutput: %s", err, string(output))
	}
}
