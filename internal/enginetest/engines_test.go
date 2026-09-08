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

package enginetest_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alexandremahdhaoui/forge/internal/enginetest"
)

// getRepoRoot returns the repository root directory.
func getRepoRoot(t *testing.T) string {
	t.Helper()

	// Try to find the repo root by looking for go.mod
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("Could not find repository root (no go.mod found)")
		}
		dir = parent
	}
}

func TestAllEnginesHaveVersionSupport(t *testing.T) {
	repoRoot := getRepoRoot(t)
	engines := enginetest.AllEngines(repoRoot)

	for _, engine := range engines {
		t.Run(engine.Name, func(t *testing.T) {
			enginetest.TestBinaryExists(t, engine)
			enginetest.TestVersionCommand(t, engine)
		})
	}
}

func TestAllMCPEnginesHaveMCPSupport(t *testing.T) {
	repoRoot := getRepoRoot(t)
	engines := enginetest.AllEngines(repoRoot)

	for _, engine := range engines {
		if !engine.SupportsMCP {
			continue
		}

		t.Run(engine.Name, func(t *testing.T) {
			enginetest.TestMCPMode(t, engine)
		})
	}
}

func TestEnginesList(t *testing.T) {
	repoRoot := getRepoRoot(t)
	engines := enginetest.AllEngines(repoRoot)

	if len(engines) != 25 {
		t.Errorf("Expected 25 engines, got %d", len(engines))
	}

	expectedEngines := map[string]bool{
		"forge":                  true,
		"go-build":               true,
		"container-build":        true,
		"generic-builder":        true,
		"testenv":                true,
		"testenv-kind":           true,
		"testenv-lcr":            true,
		"testenv-helm-install":   true,
		"go-test":                true,
		"go-lint-licenses":       true,
		"go-lint-tags":           true,
		"generic-test-runner":    true,
		"test-report":            true,
		"go-format":              true,
		"go-lint":                true,
		"go-gen-mocks":           true,
		"go-gen-openapi":         true,
		"forge-e2e":              true,
		"forge-dev":              true,
		"container-build-simple": true,
		"parallel-builder":       true,
		"go-gen-bpf":             true,
		"go-gen-protobuf":        true,
		"go-license-header":      true,
		"rust-license-header":    true,
	}

	for _, engine := range engines {
		if !expectedEngines[engine.Name] {
			t.Errorf("Unexpected engine in list: %s", engine.Name)
		}
		delete(expectedEngines, engine.Name)
	}

	if len(expectedEngines) > 0 {
		for name := range expectedEngines {
			t.Errorf("Missing engine from list: %s", name)
		}
	}
}

func TestMCPEnginesConfiguration(t *testing.T) {
	repoRoot := getRepoRoot(t)
	engines := enginetest.AllEngines(repoRoot)

	// Verify which engines should support MCP
	expectedMCPEngines := map[string]bool{
		"forge":                  true,
		"go-build":               true,
		"container-build":        true,
		"generic-builder":        true,
		"testenv":                true,
		"testenv-kind":           true,
		"testenv-lcr":            true,
		"testenv-helm-install":   true,
		"go-test":                true,
		"go-lint-licenses":       true,
		"go-lint-tags":           true,
		"generic-test-runner":    true,
		"test-report":            true,
		"go-format":              true,
		"go-lint":                true,
		"go-gen-mocks":           true,
		"go-gen-openapi":         true,
		"forge-e2e":              true,
		"forge-dev":              true,
		"container-build-simple": true,
		"parallel-builder":       true,
		"go-gen-bpf":             true,
		"go-gen-protobuf":        true,
		"go-license-header":      true,
		"rust-license-header":    true,
	}

	for _, engine := range engines {
		expected := expectedMCPEngines[engine.Name]
		if engine.SupportsMCP != expected {
			t.Errorf("Engine %s: expected SupportsMCP=%v, got %v",
				engine.Name, expected, engine.SupportsMCP)
		}
	}
}

// TestAllBuildEnginesImplementBuildBatch verifies that all build engines
// implement both "build" and "buildBatch" MCP tools. This test ensures that
// the issue where go-gen-openapi was missing buildBatch never happens again.
func TestAllBuildEnginesImplementBuildBatch(t *testing.T) {
	repoRoot := getRepoRoot(t)
	engines := enginetest.AllEngines(repoRoot)

	// Build engines are those that should implement build and buildBatch tools
	buildEngines := map[string]bool{
		"go-build":        true,
		"container-build": true,
		"go-gen-openapi":  true,
		"go-gen-mocks":    true,
		"generic-builder": true,
	}

	for _, engine := range engines {
		if !buildEngines[engine.Name] {
			continue
		}

		t.Run(engine.Name, func(t *testing.T) {
			enginetest.TestBuildEngineTools(t, engine)
		})
	}
}

// TestEveryBuildEngineHoldsThePlatformContract holds every builder to the
// platform contract over real MCP. A malformed platform is refused by every
// engine, naming it. A platform no toolchain targets is refused by name by
// an engine that declares the host, and admitted by declaration by an
// engine that declares any - its own tooling then decides, which is what
// "any" means. A builder that ignored the platform it was handed would
// answer a host binary under a foreign name, which is the defect this
// exists to keep out.
func TestEveryBuildEngineHoldsThePlatformContract(t *testing.T) {
	repoRoot := getRepoRoot(t)

	// What each builder declares in its forge-dev.yaml.
	declaresAny := map[string]bool{
		"go-build": true, "generic-builder": true, "container-build-simple": true, "parallel-builder": true,
	}
	buildEngines := map[string]bool{
		"container-build": true, "forge-dev": true, "go-format": true, "go-gen-bpf": true,
		"go-gen-mocks": true, "go-gen-openapi": true, "go-gen-protobuf": true,
		"go-license-header": true, "rust-license-header": true,
	}
	for name := range declaresAny {
		buildEngines[name] = true
	}

	for _, engine := range enginetest.AllEngines(repoRoot) {
		if !buildEngines[engine.Name] {
			continue
		}

		t.Run(engine.Name, func(t *testing.T) {
			enginetest.TestPlatformRefusal(t, engine, declaresAny[engine.Name])
		})
	}
}

// lockfileSpecs is what each builder needs in its spec to attempt a build
// of a plain Go module. An engine absent here builds with an empty spec
// and may fail; the invariant holds either way.
var lockfileSpecs = map[string]map[string]any{
	"generic-builder": {"command": "go", "args": []string{"build", "-o", "{{ .Dest }}/probe", "."}},
	// A child's spec is the child's whole build input: the parent hands on
	// platforms, force, frozen (to a child that declares it) and the
	// directories, never the name, engine, src or dest.
	"parallel-builder": {"builders": []any{
		map[string]any{"name": "compile", "engine": "forge://go-build", "spec": map[string]any{
			"name": "probe", "engine": "forge://go-build", "src": ".", "dest": "bin", "spec": map[string]any{},
		}},
	}},
}

// runLocalEnv lets an engine that resolves other engines (parallel-builder)
// find them in this checkout rather than on a module proxy.
func runLocalEnv(repoRoot string) []string {
	return []string{"FORGE_RUN_LOCAL_ENABLED=true", "FORGE_RUN_LOCAL_BASEDIR=" + repoRoot}
}

// TestEveryBuildEngineNeverWritesALockfile is the invariant every builder
// holds whether or not it reads a frozen input: a build never writes a
// lockfile. Regenerating one is `forge-factory lock`, never a build.
func TestEveryBuildEngineNeverWritesALockfile(t *testing.T) {
	repoRoot := getRepoRoot(t)

	buildEngines := map[string]bool{
		"go-build": true, "generic-builder": true, "container-build-simple": true, "parallel-builder": true,
		"container-build": true, "forge-dev": true, "go-format": true, "go-gen-bpf": true,
		"go-gen-mocks": true, "go-gen-openapi": true, "go-gen-protobuf": true,
		"go-license-header": true, "rust-license-header": true,
	}

	for _, engine := range enginetest.AllEngines(repoRoot) {
		if !buildEngines[engine.Name] {
			continue
		}

		t.Run(engine.Name, func(t *testing.T) {
			spec := lockfileSpecs[engine.Name]
			if spec == nil {
				spec = map[string]any{}
			}

			// The three that compile Go must succeed; the rest may refuse a
			// plain module and are held only to leaving its lock alone.
			mustBuild := map[string]bool{"go-build": true, "generic-builder": true, "parallel-builder": true}[engine.Name]

			enginetest.TestABuildNeverWritesALockfile(t, engine, spec, runLocalEnv(repoRoot), mustBuild)
		})
	}
}

// TestAStaleLockfileFailsAFrozenBuild holds the engines that declare the
// frozen capability to it: a go.sum missing one line fails the build
// naming go.sum, and the lock is not repaired on the way to failing.
func TestAStaleLockfileFailsAFrozenBuild(t *testing.T) {
	repoRoot := getRepoRoot(t)

	declaresFrozen := map[string]bool{"go-build": true, "generic-builder": true, "parallel-builder": true}

	for _, engine := range enginetest.AllEngines(repoRoot) {
		if !declaresFrozen[engine.Name] {
			continue
		}

		t.Run(engine.Name, func(t *testing.T) {
			spec := lockfileSpecs[engine.Name]
			if spec == nil {
				spec = map[string]any{}
			}

			enginetest.TestAStaleLockfileFailsTheBuild(t, engine, spec, runLocalEnv(repoRoot))
		})
	}
}

// TestBuildersAnswerTheirFrozenDeclaration holds config-validate's answer
// to what forge-dev.yaml declares, for one engine that reads frozen and one
// that does not.
func TestBuildersAnswerTheirFrozenDeclaration(t *testing.T) {
	repoRoot := getRepoRoot(t)

	want := map[string]bool{"go-build": true, "generic-builder": true, "parallel-builder": true, "go-format": false, "go-gen-mocks": false}

	for _, engine := range enginetest.AllEngines(repoRoot) {
		wantFrozen, listed := want[engine.Name]
		if !listed {
			continue
		}

		t.Run(engine.Name, func(t *testing.T) {
			enginetest.TestAnEngineAnswersItsDeclaration(t, engine, wantFrozen)
		})
	}
}
