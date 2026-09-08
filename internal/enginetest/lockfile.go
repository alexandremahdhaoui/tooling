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

package enginetest

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// The lockfile invariant every build engine holds, checked over real MCP:
// a build never writes a lockfile, and an engine that declares the frozen
// capability fails on a stale one instead of repairing it. Regenerating a
// lock is `forge-factory lock`, never a build - a build that quietly
// repaired go.sum produced bytes nobody could reproduce and a tree that
// was dirty for the next revision, which is the defect these exist to keep
// out.

// GoFixture is a minimal Go module with a real lockfile: one external
// dependency, so go.sum carries lines a stale test can remove.
type GoFixture struct {
	Dir   string
	GoMod string
	GoSum string
}

// NewGoFixture writes the module and tidies it from the module cache. It
// skips when the cache cannot serve the dependency: the invariant is about
// what an engine writes, not about the network.
func NewGoFixture(t *testing.T) GoFixture {
	t.Helper()

	dir := t.TempDir()

	files := map[string]string{
		"go.mod": "module probe\n\ngo 1.24\n\nrequire sigs.k8s.io/yaml v1.6.0\n",
		"main.go": `package main

import (
	"fmt"

	"sigs.k8s.io/yaml"
)

func main() {
	out, _ := yaml.Marshal(map[string]string{"probe": "lockfile"})
	fmt.Print(string(out))
}
`,
	}

	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("writing the fixture: %v", err)
		}
	}

	tidy := exec.Command("go", "mod", "tidy")
	tidy.Dir = dir
	tidy.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod")

	if out, err := tidy.CombinedOutput(); err != nil {
		t.Skipf("the module cache cannot serve the fixture's dependency: %v\n%s", err, out)
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

	return GoFixture{
		Dir:   dir,
		GoMod: filepath.Join(dir, "go.mod"),
		GoSum: filepath.Join(dir, "go.sum"),
	}
}

// HostPlatform is the os/arch pair of the machine the tests run on.
func HostPlatform() string {
	return runtime.GOOS + "/" + runtime.GOARCH
}

func readOrFail(t *testing.T, path string) []byte {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}

	return data
}

// BuildCall is one build tool call against an engine started in a
// directory, with extra environment for the engine process.
type BuildCall struct {
	Dir  string
	Env  []string
	Args map[string]any
}

// CallBuild starts the engine over MCP in call.Dir, sends one build, and
// answers whether the result was an error and its text.
func CallBuild(t *testing.T, engine Engine, call BuildCall) (bool, string) {
	t.Helper()

	return CallTool(t, engine, "build", call)
}

// CallTool starts the engine over MCP in call.Dir, sends one tool call, and
// answers whether the result was an error and its text.
func CallTool(t *testing.T, engine Engine, tool string, call BuildCall) (bool, string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, engine.BinaryPath, "--mcp")
	cmd.Dir = call.Dir
	cmd.Env = append(os.Environ(), call.Env...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	defer func() { _ = stdin.Close() }()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("starting %s: %v", engine.Name, err)
	}
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}()

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)

	send := func(line string) {
		if _, err := stdin.Write([]byte(line + "\n")); err != nil {
			t.Fatalf("writing to %s: %v", engine.Name, err)
		}
	}

	send(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0.0"}}}`)

	if !scanner.Scan() {
		t.Fatalf("no initialize response from %s: %v\n%s", engine.Name, scanner.Err(), stderr.String())
	}

	send(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)

	arguments, err := json.Marshal(call.Args)
	if err != nil {
		t.Fatalf("encoding the build call: %v", err)
	}

	send(fmt.Sprintf(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":%q,"arguments":%s}}`, tool, arguments))

	var response string

	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, `"id":2`) {
			response = line

			break
		}
	}

	if response == "" {
		t.Fatalf("no build response from %s: %v\nstderr: %s", engine.Name, scanner.Err(), stderr.String())
	}

	var rpc struct {
		Result struct {
			IsError bool `json:"isError"`
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
			// StructuredContent is the typed answer beside the text: the
			// artifact list of a build, the document of a config-validate.
			StructuredContent json.RawMessage `json:"structuredContent"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal([]byte(response), &rpc); err != nil {
		t.Fatalf("parsing the build response: %v\n%s", err, response)
	}

	if rpc.Error != nil {
		return true, rpc.Error.Message + "\n" + stderr.String()
	}

	var text strings.Builder
	for _, c := range rpc.Result.Content {
		text.WriteString(c.Text)
	}

	if len(rpc.Result.StructuredContent) > 0 {
		text.WriteString("\n")
		text.Write(rpc.Result.StructuredContent)
	}

	text.WriteString("\n")
	text.WriteString(stderr.String())

	return rpc.Result.IsError, text.String()
}

// TestABuildNeverWritesALockfile builds the fixture with the engine and
// asserts go.mod and go.sum are byte-identical afterwards, whether the
// build succeeded or not: an engine that cannot build a plain Go module
// still must not touch its lock on the way to failing.
func TestABuildNeverWritesALockfile(t *testing.T, engine Engine, spec map[string]any, env []string, mustBuild bool) {
	t.Helper()

	if _, err := os.Stat(engine.BinaryPath); os.IsNotExist(err) {
		t.Skipf("Binary not found: %s", engine.BinaryPath)
	}

	fixture := NewGoFixture(t)
	before := map[string][]byte{
		"go.mod": readOrFail(t, fixture.GoMod),
		"go.sum": readOrFail(t, fixture.GoSum),
	}

	dest := filepath.Join(fixture.Dir, "bin")

	isError, text := CallBuild(t, engine, BuildCall{
		Dir: fixture.Dir,
		Env: env,
		Args: map[string]any{
			"name": "probe", "engine": "forge://" + engine.Name,
			"src": ".", "dest": dest, "context": fixture.Dir, "rootDir": fixture.Dir,
			"platforms": []string{HostPlatform()},
			"spec":      spec,
		},
	})

	for name, want := range before {
		got := readOrFail(t, filepath.Join(fixture.Dir, name))
		if !bytes.Equal(got, want) {
			t.Fatalf("%s wrote %s during a build; a build never writes a lockfile:\n--- before\n%s\n--- after\n%s",
				engine.Name, name, want, got)
		}
	}

	// An engine that compiles Go must have compiled: a refusal that never
	// touched the lock proves nothing about a build that would have.
	if mustBuild && isError {
		t.Fatalf("%s must build the fixture for this to prove anything:\n%s", engine.Name, text)
	}
}

// TestAStaleLockfileFailsTheBuild removes one go.sum line and asserts a
// frozen build with an engine that declares the capability fails naming
// go.sum, and repairs nothing.
func TestAStaleLockfileFailsTheBuild(t *testing.T, engine Engine, spec map[string]any, env []string) {
	t.Helper()

	if _, err := os.Stat(engine.BinaryPath); os.IsNotExist(err) {
		t.Skipf("Binary not found: %s", engine.BinaryPath)
	}

	fixture := NewGoFixture(t)

	// Stale means the lock no longer covers a module the build needs: the
	// entries for the one package the fixture imports are dropped, so a
	// readonly build has nothing to check the module against.
	var kept []string

	for _, line := range strings.Split(strings.TrimSpace(string(readOrFail(t, fixture.GoSum))), "\n") {
		if !strings.HasPrefix(line, "sigs.k8s.io/yaml ") {
			kept = append(kept, line)
		}
	}

	stale := strings.Join(kept, "\n") + "\n"
	if err := os.WriteFile(fixture.GoSum, []byte(stale), 0o644); err != nil {
		t.Fatalf("staling go.sum: %v", err)
	}

	dest := filepath.Join(fixture.Dir, "bin")

	isError, text := CallBuild(t, engine, BuildCall{
		Dir: fixture.Dir,
		Env: env,
		Args: map[string]any{
			"name": "probe", "engine": "forge://" + engine.Name,
			"src": ".", "dest": dest, "context": fixture.Dir, "rootDir": fixture.Dir,
			"platforms": []string{HostPlatform()},
			"frozen":    true,
			"spec":      spec,
		},
	})

	if !isError {
		t.Fatalf("%s built against a stale go.sum; a frozen build must fail:\n%s", engine.Name, text)
	}

	if !strings.Contains(text, "go.sum") {
		t.Fatalf("%s must name go.sum when the lock is stale:\n%s", engine.Name, text)
	}

	if got := string(readOrFail(t, fixture.GoSum)); got != stale {
		t.Fatalf("%s repaired go.sum on the way to failing; frozen never writes:\n%s", engine.Name, got)
	}
}

// TestAnEngineAnswersItsDeclaration asks config-validate what the engine
// declares and holds the answer to forge-dev.yaml: forge sends frozen to
// exactly the engines that answer true, so a wrong answer here is a wrong
// build there.
func TestAnEngineAnswersItsDeclaration(t *testing.T, engine Engine, wantFrozen bool) {
	t.Helper()

	if _, err := os.Stat(engine.BinaryPath); os.IsNotExist(err) {
		t.Skipf("Binary not found: %s", engine.BinaryPath)
	}

	_, text := CallTool(t, engine, "config-validate", BuildCall{
		Dir:  t.TempDir(),
		Args: map[string]any{"spec": map[string]any{}},
	})

	start := strings.Index(text, "{")
	if start < 0 {
		t.Fatalf("%s answered no document:\n%s", engine.Name, text)
	}

	var output struct {
		Capabilities map[string]any `json:"capabilities"`
	}

	decoder := json.NewDecoder(strings.NewReader(text[start:]))
	if err := decoder.Decode(&output); err != nil {
		t.Fatalf("%s answered no capabilities document: %v\n%s", engine.Name, err, text)
	}

	frozen, _ := output.Capabilities["frozen"].(bool)
	if frozen != wantFrozen {
		t.Fatalf("%s declares frozen=%v in forge-dev.yaml and answered %v", engine.Name, wantFrozen, frozen)
	}
}
