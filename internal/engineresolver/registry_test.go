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

package engineresolver

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge/pkg/forge"
)

// A registry entry that names a source directory is built from that
// directory and run as a binary; the caller's working directory is kept.
func TestARegisteredPathIsBuiltFromSource(t *testing.T) {
	t.Setenv("FORGE_RUN_LOCAL_ENABLED", "")

	module := t.TempDir()
	writeFile(t, filepath.Join(module, "go.mod"), "module example.invalid/engines\n\ngo 1.24\n")
	writeFile(t, filepath.Join(module, "cmd", "probe", "main.go"), "package main\n\nfunc main() {}\n")

	spec := &forge.Spec{Engines: []forge.EngineConfig{{Alias: "probe", Engine: "./cmd/probe"}}}

	registry, err := Load(spec, module)
	if err != nil {
		t.Fatal(err)
	}

	Use(registry)
	t.Cleanup(func() { Use(Registry{}) })

	inv, err := ResolveForgeURI("forge://probe", "v0.0.0")
	if err != nil {
		t.Fatal(err)
	}

	want := filepath.Join(module, "build", "local-engines", "probe")
	if inv.Command != want || len(inv.Args) != 0 {
		t.Fatalf("expected the built binary %s with no args, got %+v", want, inv)
	}

	if _, err := os.Stat(want); err != nil {
		t.Fatalf("the binary must exist: %v", err)
	}
}

// A name in no ring resolves as it always did, to forge's own module.
func TestAnUnregisteredShortNameFallsThroughToForge(t *testing.T) {
	t.Setenv("FORGE_RUN_LOCAL_ENABLED", "")
	Use(Registry{})

	inv, err := ResolveForgeURI("forge://go-build", "v1.2.3")
	if err != nil {
		t.Fatal(err)
	}

	joined := strings.Join(inv.Args, " ")
	if inv.Command != "go" || !strings.Contains(joined, "github.com/alexandremahdhaoui/forge/cmd/go-build") {
		t.Fatalf("expected forge's own module, got %+v", inv)
	}
}

// A registry entry pointing at another short name is a registry naming
// the registry, and is refused by name.
func TestARegistryEntryMayNotNameAnotherShortName(t *testing.T) {
	Use(Registry{inner: map[string]string{"mine": "forge://go-build"}, innerDir: t.TempDir()})
	t.Cleanup(func() { Use(Registry{}) })

	_, err := ResolveForgeURI("forge://mine", "v1.2.3")
	if err == nil || !strings.Contains(err.Error(), "mine") || !strings.Contains(err.Error(), "short name") {
		t.Fatalf("expected a refusal naming the entry, got %v", err)
	}
}

// The factory's .forge/engines.yaml is the outer ring: the repo's own
// entry wins, the factory's answers what the repo does not name, and a
// lone checkout with no factory above it has no outer ring.
func TestTheFactoryEnginesFileIsTheOuterRing(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "member")
	writeFile(t, filepath.Join(root, "forge-factory.yaml"), "name: x\n")
	writeFile(t, filepath.Join(root, FactoryEnginesPath), "engines:\n  - alias: shared\n    engine: forge://example.invalid/tools/cmd/shared\n  - alias: mine\n    engine: forge://example.invalid/tools/cmd/theirs\n")
	writeFile(t, filepath.Join(repo, "forge.yaml"), "name: member\n")

	spec := &forge.Spec{Engines: []forge.EngineConfig{{Alias: "mine", Engine: "./cmd/mine"}}}

	registry, err := Load(spec, repo)
	if err != nil {
		t.Fatal(err)
	}

	if target, dir, ok := registry.Lookup("mine"); !ok || target != "./cmd/mine" || dir != repo {
		t.Fatalf("the repo's own entry must win, got %q %q %v", target, dir, ok)
	}

	if target, dir, ok := registry.Lookup("shared"); !ok || target != "forge://example.invalid/tools/cmd/shared" || dir != root {
		t.Fatalf("the factory's entry must answer, got %q %q %v", target, dir, ok)
	}

	if _, _, ok := registry.Lookup("absent"); ok {
		t.Fatal("a name in no ring must miss")
	}

	lone, err := Load(spec, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	if _, _, ok := lone.Lookup("shared"); ok {
		t.Fatal("a lone checkout has no outer ring")
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
