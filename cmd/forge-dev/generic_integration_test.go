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
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge/pkg/mcptypes"
)

const genericSpec = `openapi: 3.0.3
info:
  title: ci-state-git Spec Schema
  version: 0.1.0
  description: A generic engine with four tools.
components:
  schemas:
    Spec:
      type: object
      description: Where the state lives.
      required: [path]
      properties:
        path:
          type: string
          description: The state repository.
    StateGetInput:
      type: object
      required: [kind]
      properties:
        kind:
          type: string
          description: The record family.
        key:
          type: string
          description: The record within its kind.
        spec:
          type: object
          additionalProperties: true
          description: Engine specific configuration.
    Status:
      type: string
      enum: [pending, passed, failed]
      description: What a read came to.
    StateGetOutput:
      type: object
      required: [found]
      properties:
        found:
          type: boolean
          description: Whether the record exists.
        payload:
          type: string
          description: The record body.
        status:
          $ref: '#/components/schemas/Status'
    StateListOutput:
      type: object
      required: [keys]
      properties:
        keys:
          type: array
          items:
            type: string
          description: The keys under one kind.
`

const genericConfig = `name: ci-state-git
kind: mcp-server
version: 0.1.0
description: Read and write CI state in a git repo.
openapi:
  specPath: ./spec.openapi.yaml
generate:
  packageName: main
  docsBaseURL: https://raw.githubusercontent.com/alexandremahdhaoui/forge-ci/refs/heads/main
layout:
  tools:
    - name: get
      description: Read one record.
      input: StateGetInput
      output: StateGetOutput
      useSpec: true
    - name: list
      description: List the keys under one kind.
      input: StateGetInput
      output: StateListOutput
    - name: put
      description: Write one record and return nothing.
      input: StateGetInput
      useSpec: true
    - name: ping
      description: Return nothing and read no spec.
      input: StateGetInput
`

// handlers is what an engine author writes. The generated code never contains
// it, so writing it here is the same work a real engine does.
const genericHandlers = `package main

import "context"

func NewHandlers() Handlers {
	return Handlers{
		Get: func(_ context.Context, in StateGetInput, spec *Spec) (*StateGetOutput, error) {
			found := spec.Path != "" && in.Kind != ""
			return &StateGetOutput{Found: found}, nil
		},
		List: func(_ context.Context, in StateGetInput) (*StateListOutput, error) {
			return &StateListOutput{Keys: []string{in.Kind}}, nil
		},
		Put: func(_ context.Context, _ StateGetInput, _ *Spec) error {
			return nil
		},
		Ping: func(_ context.Context, _ StateGetInput) error {
			return nil
		},
	}
}
`

func writeGenericEngine(t *testing.T, root string) string {
	t.Helper()

	engine := filepath.Join(root, "cmd", "ci-state-git")

	if err := os.MkdirAll(filepath.Join(engine, "docs"), 0o750); err != nil {
		t.Fatal(err)
	}

	for name, body := range map[string]string{
		"forge-dev.yaml":    genericConfig,
		"spec.openapi.yaml": genericSpec,
		"docs/usage.md":     "# ci-state-git\n\nRead and write CI state.\n",
		"handlers.go":       genericHandlers,
	} {
		if err := os.WriteFile(filepath.Join(engine, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	return engine
}

func TestGenericEngineGeneratesAndCompiles(t *testing.T) {
	root := t.TempDir()
	createTestGoMod(t, root, "github.com/test/generic")

	engine := writeGenericEngine(t, root)

	artifact, err := generate(context.Background(), mcptypes.BuildInput{
		Name: "ci-state-git", Src: engine, Engine: "forge://forge-dev",
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	if artifact == nil {
		t.Fatal("generate returned no artifact")
	}

	for _, name := range []string{
		"zz_generated.main.go",
		"zz_generated.mcp.go",
		"zz_generated.spec.go",
		"zz_generated.validate.go",
		"zz_generated.docs.go",
		"docs/schema.md",
		"docs/list.yaml",
	} {
		if _, err := os.Stat(filepath.Join(engine, name)); err != nil {
			t.Errorf("expected %s to be generated: %v", name, err)
		}
	}

	// This is the assertion that matters. Per template parsing misses duplicate
	// identifiers, missing imports and a Handlers field whose type does not
	// match its wrapper, all of which are cross file.
	build := exec.Command("go", "build", "./...")
	build.Dir = root
	build.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod")

	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("generated engine does not compile: %v\n%s", err, out)
	}
}

func TestGenericEngineGeneratesOneTypeAndWrapperPerTool(t *testing.T) {
	root := t.TempDir()
	createTestGoMod(t, root, "github.com/test/generic")

	engine := writeGenericEngine(t, root)

	if _, err := generate(context.Background(), mcptypes.BuildInput{
		Name: "ci-state-git", Src: engine, Engine: "forge://forge-dev",
	}); err != nil {
		t.Fatalf("generate: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(engine, "zz_generated.mcp.go"))
	if err != nil {
		t.Fatal(err)
	}

	got := string(raw)

	for _, want := range []string{
		"type GetFunc func(",
		"type ListFunc func(",
		"type Handlers struct",
		"Get GetFunc",
		"List ListFunc",
		"func SetupMCPServer(name string, version string, handlers Handlers)",
		"func wrapGet(",
		"func wrapList(",
		"func wrapPut(",
		"func wrapPing(",
		"type PutFunc func(",
		"type PingFunc func(",
		`Name:        "get"`,
		`Name:        "list"`,
		`Name:        "config-validate"`,
		"Handlers.Get is required",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("generated mcp file is missing %q", want)
		}
	}

	// useSpec is true for get and false for list, so only get parses a Spec.
	if !strings.Contains(got, "spec, err := FromMap(input.Spec)") {
		t.Error("the useSpec tool does not parse its Spec")
	}

	// get and put set useSpec. list and ping do not.
	if strings.Count(got, "FromMap(input.Spec)") != 2 {
		t.Errorf("expected exactly the two useSpec tools to parse a Spec, got %d",
			strings.Count(got, "FromMap(input.Spec)"))
	}

	// The four tools cover every combination of output and useSpec, which is
	// what makes the compile test meaningful.
	for _, want := range []string{
		"output, err := fn(ctx, input, spec)",
		"output, err := fn(ctx, input)",
		"err = fn(ctx, input, spec)",
		"err := fn(ctx, input)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("no wrapper generated %q, so that branch is untested", want)
		}
	}
}

func TestGenericEngineRejectsAToolNamingAnUnknownSchema(t *testing.T) {
	root := t.TempDir()
	createTestGoMod(t, root, "github.com/test/generic")

	engine := writeGenericEngine(t, root)

	broken := strings.Replace(genericConfig, "input: StateGetInput", "input: NoSuchSchema", 1)

	if err := os.WriteFile(filepath.Join(engine, "forge-dev.yaml"), []byte(broken), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := generate(context.Background(), mcptypes.BuildInput{
		Name: "ci-state-git", Src: engine, Engine: "forge://forge-dev",
	})
	if err == nil {
		t.Fatal("generate accepted a tool naming a schema that does not exist")
	}

	if !strings.Contains(err.Error(), "NoSuchSchema") {
		t.Errorf("error does not name the missing schema: %v", err)
	}
}

func TestGenericEngineDocsUseTheConfiguredBaseURL(t *testing.T) {
	root := t.TempDir()
	createTestGoMod(t, root, "github.com/test/generic")

	engine := writeGenericEngine(t, root)

	if _, err := generate(context.Background(), mcptypes.BuildInput{
		Name: "ci-state-git", Src: engine, Engine: "forge://forge-dev",
	}); err != nil {
		t.Fatalf("generate: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(engine, "docs", "list.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	got := string(raw)

	if !strings.Contains(got, "forge-ci/refs/heads/main") {
		t.Errorf("list.yaml does not carry the configured docsBaseURL:\n%s", got)
	}

	if strings.Contains(got, "/forge/refs") {
		t.Errorf("list.yaml still advertises forge's own URL:\n%s", got)
	}

	if !strings.Contains(got, "url: \"cmd/ci-state-git/docs/usage.md\"") {
		t.Errorf("list.yaml is missing the url enginedocs reads:\n%s", got)
	}
}

func TestGenericEngineWithExternalSpecTypesCompiles(t *testing.T) {
	root := t.TempDir()
	createTestGoMod(t, root, "github.com/test/generic")

	engine := writeGenericEngine(t, root)

	external := strings.Replace(genericConfig, `  packageName: main`, `  packageName: main
  specTypes:
    enabled: true
    outputPath: pkg/wire
    packageName: wire`, 1)

	if err := os.WriteFile(filepath.Join(engine, "forge-dev.yaml"), []byte(external), 0o600); err != nil {
		t.Fatal(err)
	}

	// The handlers must use the external package once spec types move there.
	handlers := strings.ReplaceAll(genericHandlers, "StateGetInput", "wire.StateGetInput")
	handlers = strings.ReplaceAll(handlers, "StateGetOutput", "wire.StateGetOutput")
	handlers = strings.ReplaceAll(handlers, "StateListOutput", "wire.StateListOutput")
	handlers = strings.ReplaceAll(handlers, "*Spec", "*wire.Spec")
	handlers = strings.Replace(handlers, `import "context"`,
		"import (\n\t\"context\"\n\n\twire \"github.com/test/generic/pkg/wire\"\n)", 1)

	if err := os.WriteFile(filepath.Join(engine, "handlers.go"), []byte(handlers), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := generate(context.Background(), mcptypes.BuildInput{
		Name: "ci-state-git", Src: engine, Engine: "forge://forge-dev",
	}); err != nil {
		t.Fatalf("generate: %v", err)
	}

	build := exec.Command("go", "build", "./...")
	build.Dir = root
	build.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod")

	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("a generic engine with external spec types does not compile: %v\n%s", err, out)
	}
}
