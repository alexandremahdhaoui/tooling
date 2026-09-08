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
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alexandremahdhaoui/forge/pkg/mcptypes"
)

func writeLangFixture(t *testing.T, dir, language string) {
	t.Helper()

	createRequiredDocs(t, dir)

	config := fmt.Sprintf(`name: echo-engine
kind: mcp-server
version: 0.1.0
description: Echoes its arguments back.
language: %s
runtime:
  env:
    - ECHO_GREETING
  files:
    - config.yaml
openapi:
  specPath: ./spec.openapi.yaml
generate:
  packageName: main
  docsBaseURL: https://example.com/raw
layout:
  tools:
    - name: echo
      description: Echo the input back.
      input: EchoInput
      output: EchoOutput
`, language)
	if err := os.WriteFile(filepath.Join(dir, "forge-dev.yaml"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}

	spec := `openapi: 3.0.3
info:
  title: echo-engine Spec Schema
  version: 0.1.0
components:
  schemas:
    Spec:
      type: object
      properties:
        greeting:
          type: string
          description: The greeting to prepend.
        loud:
          type: boolean
          description: Shout.
      required:
        - greeting
    EchoInput:
      type: object
      properties:
        value:
          type: string
    EchoOutput:
      type: object
      properties:
        value:
          type: string
`
	if err := os.WriteFile(filepath.Join(dir, "spec.openapi.yaml"), []byte(spec), 0o644); err != nil {
		t.Fatal(err)
	}
}

func generateLangFixture(t *testing.T, language string) string {
	t.Helper()

	dir := t.TempDir()
	writeLangFixture(t, dir, language)

	_, err := generate(context.Background(), mcptypes.BuildInput{
		Name: "echo-engine", Src: dir, Engine: "forge://forge-dev",
	})
	if err != nil {
		t.Fatalf("generate() for %s: %v", language, err)
	}

	return dir
}

func TestRunnableManifestCarriesTheInputs(t *testing.T) {
	dir := generateLangFixture(t, "python")

	raw, err := os.ReadFile(filepath.Join(dir, GeneratedRunnableFile))
	if err != nil {
		t.Fatalf("the runnable manifest must exist: %v", err)
	}

	content := string(raw)
	for _, want := range []string{
		"# SourceChecksum: ",
		"name: echo-engine",
		"- ECHO_GREETING",
		"- config.yaml",
		"- greeting",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("runnable manifest missing %q:\n%s", want, content)
		}
	}

	if strings.Contains(content, "loud") {
		t.Error("an optional spec key must not appear in the inputs")
	}
}

func TestNonGoGenerationEmitsTheLanguageServerAndNoGoFiles(t *testing.T) {
	for language, file := range LangMainFiles {
		t.Run(language, func(t *testing.T) {
			dir := generateLangFixture(t, language)

			if _, err := os.Stat(filepath.Join(dir, file)); err != nil {
				t.Fatalf("%s server file must exist: %v", language, err)
			}

			for _, goFile := range []string{GeneratedSpecFile, GeneratedMCPFile, GeneratedMainFile} {
				if _, err := os.Stat(filepath.Join(dir, goFile)); err == nil {
					t.Errorf("%s generation must not emit %s", language, goFile)
				}
			}

			for _, doc := range []string{"docs/schema.md", "docs/list.yaml"} {
				if _, err := os.Stat(filepath.Join(dir, doc)); err != nil {
					t.Errorf("%s generation must emit %s: %v", language, doc, err)
				}
			}
		})
	}
}

// A language server regenerates to the same bytes over the same inputs;
// nothing in the generator decides whether to run, forge's digests do.
func TestRegenerationOfALanguageServerIsDeterministic(t *testing.T) {
	dir := generateLangFixture(t, "python")

	path := filepath.Join(dir, LangMainFiles["python"])
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	_, err = generate(context.Background(), mcptypes.BuildInput{
		Name: "echo-engine", Src: dir, Engine: "forge://forge-dev",
	})
	if err != nil {
		t.Fatal(err)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if string(after) != string(before) {
		t.Error("regenerating over unchanged inputs must write the same bytes")
	}
}

func TestALanguageOutsideGenericIsRejected(t *testing.T) {
	errs := ValidateConfig(&Config{
		Name: "x", Kind: KindMCPServer, Profile: "builder", Version: "0.1.0", Language: "python",
		Generate: GenerateConfig{PackageName: "main"},
	})

	found := false
	for _, e := range errs {
		if e.Field == "language" {
			found = true
		}
	}

	if !found {
		t.Fatal("a non-generic engine in another language must fail validation")
	}
}

type mcpProbe struct {
	cmd  *exec.Cmd
	in   io.WriteCloser
	out  *bufio.Scanner
	next int
}

func startProbe(t *testing.T, command string, args ...string) *mcpProbe {
	t.Helper()

	cmd := exec.Command(command, args...)
	cmd.Stderr = os.Stderr

	in, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}

	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("starting %s: %v", command, err)
	}

	t.Cleanup(func() {
		_ = in.Close()
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})

	return &mcpProbe{cmd: cmd, in: in, out: bufio.NewScanner(out), next: 1}
}

func (p *mcpProbe) call(t *testing.T, method string, params map[string]any) map[string]any {
	t.Helper()

	id := p.next
	p.next++

	msg, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": id, "method": method, "params": params,
	})

	if _, err := p.in.Write(append(msg, '\n')); err != nil {
		t.Fatalf("writing %s: %v", method, err)
	}

	deadline := time.After(15 * time.Second)
	done := make(chan bool, 1)

	var line string

	go func() {
		ok := p.out.Scan()
		line = p.out.Text()
		done <- ok
	}()

	select {
	case ok := <-done:
		if !ok {
			t.Fatalf("the engine closed its stdout during %s", method)
		}
	case <-deadline:
		t.Fatalf("no answer to %s within 15s", method)
	}

	var response map[string]any
	if err := json.Unmarshal([]byte(line), &response); err != nil {
		t.Fatalf("decoding the answer to %s: %v: %s", method, err, line)
	}

	result, _ := response["result"].(map[string]any)
	if result == nil {
		t.Fatalf("%s answered with no result: %s", method, line)
	}

	return result
}

func speakMCP(t *testing.T, probe *mcpProbe) {
	t.Helper()

	init := probe.call(t, "initialize", map[string]any{
		"protocolVersion": "2025-06-18",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "probe", "version": "0"},
	})

	server, _ := init["serverInfo"].(map[string]any)
	if server["name"] != "echo-engine" {
		t.Fatalf("serverInfo.name = %v", server["name"])
	}

	list := probe.call(t, "tools/list", map[string]any{})

	tools, _ := list["tools"].([]any)
	names := []string{}

	for _, tool := range tools {
		m, _ := tool.(map[string]any)
		names = append(names, fmt.Sprint(m["name"]))
	}

	joined := strings.Join(names, ",")
	if !strings.Contains(joined, "echo") || !strings.Contains(joined, "config-validate") {
		t.Fatalf("tools/list = %s", joined)
	}

	invalid := probe.call(t, "tools/call", map[string]any{
		"name": "config-validate", "arguments": map[string]any{"spec": map[string]any{}},
	})
	if invalid["isError"] != true {
		t.Fatalf("a spec missing its required key must fail validation: %v", invalid)
	}

	valid := probe.call(t, "tools/call", map[string]any{
		"name": "config-validate", "arguments": map[string]any{"spec": map[string]any{"greeting": "hi"}},
	})
	if valid["isError"] == true {
		t.Fatalf("a complete spec must validate: %v", valid)
	}

	echoed := probe.call(t, "tools/call", map[string]any{
		"name": "echo", "arguments": map[string]any{"value": "ping"},
	})

	structured, _ := echoed["structuredContent"].(map[string]any)
	if structured["value"] != "ping" {
		t.Fatalf("echo must answer its input back: %v", echoed)
	}
}

func TestTheGeneratedPythonEngineSpeaksMCP(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("no python3 on this host")
	}

	dir := generateLangFixture(t, "python")

	handlers := `def handle(tool, arguments):
    if tool == "echo":
        return {"value": arguments.get("value", "")}
    return {}
`
	if err := os.WriteFile(filepath.Join(dir, "handlers.py"), []byte(handlers), 0o644); err != nil {
		t.Fatal(err)
	}

	probe := startProbe(t, python, filepath.Join(dir, LangMainFiles["python"]))
	speakMCP(t, probe)
}

func TestTheGeneratedTypescriptEngineSpeaksMCP(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("no node on this host")
	}

	dir := generateLangFixture(t, "typescript")

	handlers := `export function handle(tool, args) {
  if (tool === "echo") {
    return { value: args["value"] ?? "" };
  }
  return {};
}
`
	if err := os.WriteFile(filepath.Join(dir, "handlers.ts"), []byte(handlers), 0o644); err != nil {
		t.Fatal(err)
	}

	generated, err := os.ReadFile(filepath.Join(dir, LangMainFiles["typescript"]))
	if err != nil {
		t.Fatal(err)
	}

	stripped := stripTypescriptTypes(string(generated))
	entry := filepath.Join(dir, "main.mjs")

	if err := os.WriteFile(entry, []byte(stripped), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.Rename(filepath.Join(dir, "handlers.ts"), filepath.Join(dir, "handlers.mjs")); err != nil {
		t.Fatal(err)
	}

	probe := startProbe(t, node, entry)
	speakMCP(t, probe)
}

// stripTypescriptTypes turns the generated server into plain ESM so node runs
// it with no compiler: the test proves the protocol, tsc proves the types in
// the repos that adopt it.
func stripTypescriptTypes(src string) string {
	src = strings.ReplaceAll(src, `from "./handlers"`, `from "./handlers.mjs"`)
	src = strings.ReplaceAll(src, "const REQUIRED_SPEC_KEYS: string[]", "const REQUIRED_SPEC_KEYS")
	src = strings.ReplaceAll(src, "type Json = Record<string, unknown>;", "")

	for _, pair := range [][2]string{
		{"(args: Json): Json", "(args)"},
		{"(name: string, args: Json): Json", "(name, args)"},
		{"(id: unknown, result: Json): void", "(id, result)"},
		{"(id: unknown, code: number, message: string): void", "(id, code, message)"},
		{"(line: string) =>", "(line) =>"},
		{"let msg: Json;", "let msg;"},
		{" as Json | undefined", ""},
		{" as string | undefined", ""},
		{" as Json", ""},
		{" as string", ""},
	} {
		src = strings.ReplaceAll(src, pair[0], pair[1])
	}

	return src
}
