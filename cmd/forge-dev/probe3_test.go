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
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	mcptypes "github.com/alexandremahdhaoui/forge/pkg/mcptypes"
)

func TestProbeForgeCISpec(t *testing.T) {
	root := t.TempDir()
	createTestGoMod(t, root, "github.com/test/probe")

	engine := filepath.Join(root, "cmd", "ci-manager-dryrun")
	if err := os.MkdirAll(engine, 0o755); err != nil {
		t.Fatal(err)
	}

	spec, err := os.ReadFile("/home/amahdha/workspaces/playground/forge-ci/.forge/spec-cache/engines.v1.yaml")
	if err != nil {
		t.Skipf("the probe needs the playground spec cache: %v", err)
	}

	if err := os.WriteFile(filepath.Join(engine, "spec.openapi.yaml"), spec, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := `name: ci-manager-dryrun
kind: mcp-server
version: 0.1.5
description: probe
openapi:
  specPath: spec.openapi.yaml
generate:
  packageName: main
layout:
  tools:
    - name: reconcile
      description: probe
      input: ReconcileInput
      output: ReconcileOutput
`
	if err := os.WriteFile(filepath.Join(engine, "forge-dev.yaml"), []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(engine, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(engine, "docs", "usage.md"), []byte("# probe\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := generate(context.Background(), mcptypes.BuildInput{
		Name: "ci-manager-dryrun", Src: engine, Engine: "forge://forge-dev",
	}); err != nil {
		t.Fatalf("generate: %v", err)
	}

	out, err := os.ReadFile(filepath.Join(engine, "zz_generated.spec.go"))
	if err != nil {
		t.Fatal(err)
	}

	if n := countOccurrences(string(out), "StatusFromMap"); n > 0 {
		t.Errorf("StatusFromMap emitted %d times", n)
	}

	build := exec.Command("go", "build", "./...")
	build.Dir = root
	build.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod")

	if o, err := build.CombinedOutput(); err != nil {
		t.Fatalf("does not compile: %v\n%s", err, o)
	}
}

func countOccurrences(s, sub string) int {
	n := 0
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			n++
		}
	}
	return n
}
