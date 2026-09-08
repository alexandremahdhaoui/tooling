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

package forge

import (
	"strings"
	"testing"

	"sigs.k8s.io/yaml"
)

func TestEngineConfigParsing(t *testing.T) {
	yamlContent := `
name: test-project
artifactStorePath: .test.yaml

engines:
  - alias: my-formatter
    type: builder
    builder:
      - engine: forge://generic-builder
        spec:
          command: "gofmt"
          args: ["-w", "."]
          env:
            GOFMT_STYLE: "google"
            DEBUG: "true"
          envFile: ".envrc"
          context: "/tmp/test"

  - alias: my-linter
    type: test-runner
    testRunner:
      - engine: forge://generic-test-runner
        spec:
          command: "golangci-lint"
          args: ["run", "./..."]

  - alias: my-testenv
    type: testenv
    testenv:
      - engine: forge://testenv-kind
      - engine: forge://testenv-lcr
        spec:
          enabled: true
`

	var spec Spec
	err := yaml.Unmarshal([]byte(yamlContent), &spec)
	if err != nil {
		t.Fatalf("Failed to unmarshal YAML: %v", err)
	}

	// Verify engines were parsed
	if len(spec.Engines) != 3 {
		t.Fatalf("Expected 3 engines, got %d", len(spec.Engines))
	}

	// Verify first engine (my-formatter) - builder type
	formatter := spec.Engines[0]
	if formatter.Alias != "my-formatter" {
		t.Errorf("Expected alias 'my-formatter', got '%s'", formatter.Alias)
	}
	if formatter.Type != BuilderEngineConfigType {
		t.Errorf("Expected type 'builder', got '%s'", formatter.Type)
	}
	if len(formatter.Builder) != 1 {
		t.Fatalf("Expected 1 builder, got %d", len(formatter.Builder))
	}
	if formatter.Builder[0].Engine != "forge://generic-builder" {
		t.Errorf("Expected engine 'forge://generic-builder', got '%s'", formatter.Builder[0].Engine)
	}
	if formatter.Builder[0].Spec.Command != "gofmt" {
		t.Errorf("Expected command 'gofmt', got '%s'", formatter.Builder[0].Spec.Command)
	}
	if len(formatter.Builder[0].Spec.Args) != 2 {
		t.Fatalf("Expected 2 args, got %d", len(formatter.Builder[0].Spec.Args))
	}
	if formatter.Builder[0].Spec.Args[0] != "-w" {
		t.Errorf("Expected arg '-w', got '%s'", formatter.Builder[0].Spec.Args[0])
	}
	if formatter.Builder[0].Spec.Args[1] != "." {
		t.Errorf("Expected arg '.', got '%s'", formatter.Builder[0].Spec.Args[1])
	}
	if len(formatter.Builder[0].Spec.Env) != 2 {
		t.Fatalf("Expected 2 env vars, got %d", len(formatter.Builder[0].Spec.Env))
	}
	if formatter.Builder[0].Spec.Env["GOFMT_STYLE"] != "google" {
		t.Errorf("Expected env GOFMT_STYLE='google', got '%s'", formatter.Builder[0].Spec.Env["GOFMT_STYLE"])
	}
	if formatter.Builder[0].Spec.Env["DEBUG"] != "true" {
		t.Errorf("Expected env DEBUG='true', got '%s'", formatter.Builder[0].Spec.Env["DEBUG"])
	}
	if formatter.Builder[0].Spec.EnvFile != ".envrc" {
		t.Errorf("Expected envFile '.envrc', got '%s'", formatter.Builder[0].Spec.EnvFile)
	}
	if formatter.Builder[0].Spec.Context != "/tmp/test" {
		t.Errorf("Expected context '/tmp/test', got '%s'", formatter.Builder[0].Spec.Context)
	}

	// Verify second engine (my-linter) - test-runner type
	linter := spec.Engines[1]
	if linter.Alias != "my-linter" {
		t.Errorf("Expected alias 'my-linter', got '%s'", linter.Alias)
	}
	if linter.Type != TestRunnerEngineConfigType {
		t.Errorf("Expected type 'test-runner', got '%s'", linter.Type)
	}
	if len(linter.TestRunner) != 1 {
		t.Fatalf("Expected 1 test runner, got %d", len(linter.TestRunner))
	}
	if linter.TestRunner[0].Engine != "forge://generic-test-runner" {
		t.Errorf("Expected engine 'forge://generic-test-runner', got '%s'", linter.TestRunner[0].Engine)
	}
	if linter.TestRunner[0].Spec.Command != "golangci-lint" {
		t.Errorf("Expected command 'golangci-lint', got '%s'", linter.TestRunner[0].Spec.Command)
	}
	if len(linter.TestRunner[0].Spec.Args) != 2 {
		t.Fatalf("Expected 2 args, got %d", len(linter.TestRunner[0].Spec.Args))
	}
	if linter.TestRunner[0].Spec.Args[0] != "run" {
		t.Errorf("Expected arg 'run', got '%s'", linter.TestRunner[0].Spec.Args[0])
	}
	if linter.TestRunner[0].Spec.Args[1] != "./..." {
		t.Errorf("Expected arg './...', got '%s'", linter.TestRunner[0].Spec.Args[1])
	}

	// Verify third engine (my-testenv) - testenv type
	testenv := spec.Engines[2]
	if testenv.Alias != "my-testenv" {
		t.Errorf("Expected alias 'my-testenv', got '%s'", testenv.Alias)
	}
	if testenv.Type != TestenvEngineConfigType {
		t.Errorf("Expected type 'testenv', got '%s'", testenv.Type)
	}
	if len(testenv.Testenv) != 2 {
		t.Fatalf("Expected 2 testenv engines, got %d", len(testenv.Testenv))
	}
	if testenv.Testenv[0].Engine != "forge://testenv-kind" {
		t.Errorf("Expected engine 'forge://testenv-kind', got '%s'", testenv.Testenv[0].Engine)
	}
	if testenv.Testenv[1].Engine != "forge://testenv-lcr" {
		t.Errorf("Expected engine 'forge://testenv-lcr', got '%s'", testenv.Testenv[1].Engine)
	}
}

func TestEngineConfigOptionalFields(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		validate func(t *testing.T, spec Spec)
	}{
		{
			name: "Minimal builder - only alias, type, and engine",
			yaml: `
name: test-project
artifactStorePath: .test.yaml
engines:
  - alias: minimal
    type: builder
    builder:
      - engine: forge://generic-builder
`,
			validate: func(t *testing.T, spec Spec) {
				if len(spec.Engines) != 1 {
					t.Fatalf("Expected 1 engine, got %d", len(spec.Engines))
				}
				eng := spec.Engines[0]
				if eng.Alias != "minimal" {
					t.Errorf("Expected alias 'minimal', got '%s'", eng.Alias)
				}
				if eng.Type != BuilderEngineConfigType {
					t.Errorf("Expected type 'builder', got '%s'", eng.Type)
				}
				if len(eng.Builder) != 1 {
					t.Fatalf("Expected 1 builder, got %d", len(eng.Builder))
				}
				if eng.Builder[0].Engine != "forge://generic-builder" {
					t.Errorf("Expected engine 'forge://generic-builder', got '%s'", eng.Builder[0].Engine)
				}
				// Verify spec is empty/default
				if eng.Builder[0].Spec.Command != "" {
					t.Errorf("Expected empty command, got '%s'", eng.Builder[0].Spec.Command)
				}
				if len(eng.Builder[0].Spec.Args) != 0 {
					t.Errorf("Expected no args, got %d", len(eng.Builder[0].Spec.Args))
				}
				if len(eng.Builder[0].Spec.Env) != 0 {
					t.Errorf("Expected no env vars, got %d", len(eng.Builder[0].Spec.Env))
				}
				if eng.Builder[0].Spec.EnvFile != "" {
					t.Errorf("Expected empty envFile, got '%s'", eng.Builder[0].Spec.EnvFile)
				}
			},
		},
		{
			name: "Full builder config - all fields",
			yaml: `
name: test-project
artifactStorePath: .test.yaml
engines:
  - alias: full
    type: builder
    builder:
      - engine: forge://generic-builder
        spec:
          command: "test-cmd"
          args: ["arg1", "arg2", "arg3"]
          env:
            VAR1: "value1"
            VAR2: "value2"
            VAR3: "value3"
          envFile: ".env.test"
          context: "/custom/dir"
`,
			validate: func(t *testing.T, spec Spec) {
				if len(spec.Engines) != 1 {
					t.Fatalf("Expected 1 engine, got %d", len(spec.Engines))
				}
				eng := spec.Engines[0]
				if eng.Alias != "full" {
					t.Errorf("Expected alias 'full', got '%s'", eng.Alias)
				}
				if eng.Builder[0].Spec.Command != "test-cmd" {
					t.Errorf("Expected command 'test-cmd', got '%s'", eng.Builder[0].Spec.Command)
				}
				if len(eng.Builder[0].Spec.Args) != 3 {
					t.Fatalf("Expected 3 args, got %d", len(eng.Builder[0].Spec.Args))
				}
				if len(eng.Builder[0].Spec.Env) != 3 {
					t.Fatalf("Expected 3 env vars, got %d", len(eng.Builder[0].Spec.Env))
				}
				if eng.Builder[0].Spec.EnvFile != ".env.test" {
					t.Errorf("Expected envFile '.env.test', got '%s'", eng.Builder[0].Spec.EnvFile)
				}
				if eng.Builder[0].Spec.Context != "/custom/dir" {
					t.Errorf("Expected context '/custom/dir', got '%s'", eng.Builder[0].Spec.Context)
				}
			},
		},
		{
			name: "Empty engines list",
			yaml: `
name: test-project
artifactStorePath: .test.yaml
engines: []
`,
			validate: func(t *testing.T, spec Spec) {
				if spec.Engines == nil {
					t.Error("Expected engines to be non-nil")
				}
				if len(spec.Engines) != 0 {
					t.Errorf("Expected 0 engines, got %d", len(spec.Engines))
				}
			},
		},
		{
			name: "No engines field",
			yaml: `
name: test-project
artifactStorePath: .test.yaml
`,
			validate: func(t *testing.T, spec Spec) {
				if spec.Engines != nil && len(spec.Engines) != 0 {
					t.Errorf("Expected nil or empty engines, got %d", len(spec.Engines))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var spec Spec
			err := yaml.Unmarshal([]byte(tt.yaml), &spec)
			if err != nil {
				t.Fatalf("Failed to unmarshal YAML: %v", err)
			}
			tt.validate(t, spec)
		})
	}
}

func TestEngineConfigDependencyDetectorType(t *testing.T) {
	tests := []struct {
		name        string
		yaml        string
		shouldError bool
		validate    func(t *testing.T, spec Spec, err error)
	}{
		{
			name: "Valid dependency-detector type",
			yaml: `
name: test-project
artifactStorePath: .test.yaml
envFile: .envrc
engines:
  - alias: my-dep-detector
    type: dependency-detector
    dependencyDetector:
      - engine: forge://go-dependency-detector
        spec:
          someConfig: value
`,
			shouldError: false,
			validate: func(t *testing.T, spec Spec, err error) {
				if err != nil {
					t.Fatalf("Unexpected validation error: %v", err)
				}
				if len(spec.Engines) != 1 {
					t.Fatalf("Expected 1 engine, got %d", len(spec.Engines))
				}
				eng := spec.Engines[0]
				if eng.Alias != "my-dep-detector" {
					t.Errorf("Expected alias 'my-dep-detector', got '%s'", eng.Alias)
				}
				if eng.Type != DependencyDetectorEngineConfigType {
					t.Errorf("Expected type 'dependency-detector', got '%s'", eng.Type)
				}
				if len(eng.DependencyDetector) != 1 {
					t.Fatalf("Expected 1 dependency detector, got %d", len(eng.DependencyDetector))
				}
				if eng.DependencyDetector[0].Engine != "forge://go-dependency-detector" {
					t.Errorf("Expected engine 'forge://go-dependency-detector', got '%s'", eng.DependencyDetector[0].Engine)
				}
			},
		},
		{
			name: "Empty dependencyDetector array - should error",
			yaml: `
name: test-project
artifactStorePath: .test.yaml
envFile: .envrc
engines:
  - alias: my-dep-detector
    type: dependency-detector
    dependencyDetector: []
`,
			shouldError: true,
			validate: func(t *testing.T, spec Spec, err error) {
				if err == nil {
					t.Fatal("Expected validation error for empty dependencyDetector array")
				}
				errMsg := err.Error()
				if !containsSubstring(errMsg, "requires at least one dependencyDetector specification") {
					t.Errorf("Expected error about missing dependencyDetector specification, got: %v", err)
				}
			},
		},
		{
			name: "dependency-detector with builder config - should error",
			yaml: `
name: test-project
artifactStorePath: .test.yaml
envFile: .envrc
engines:
  - alias: my-dep-detector
    type: dependency-detector
    dependencyDetector:
      - engine: forge://go-dependency-detector
    builder:
      - engine: forge://generic-builder
`,
			shouldError: true,
			validate: func(t *testing.T, spec Spec, err error) {
				if err == nil {
					t.Fatal("Expected validation error for dependency-detector with builder config")
				}
				errMsg := err.Error()
				if !containsSubstring(errMsg, "but contains builder") {
					t.Errorf("Expected error about builder configuration, got: %v", err)
				}
			},
		},
		{
			name: "dependency-detector with testRunner config - should error",
			yaml: `
name: test-project
artifactStorePath: .test.yaml
envFile: .envrc
engines:
  - alias: my-dep-detector
    type: dependency-detector
    dependencyDetector:
      - engine: forge://go-dependency-detector
    testRunner:
      - engine: forge://generic-test-runner
`,
			shouldError: true,
			validate: func(t *testing.T, spec Spec, err error) {
				if err == nil {
					t.Fatal("Expected validation error for dependency-detector with testRunner config")
				}
				errMsg := err.Error()
				if !containsSubstring(errMsg, "but contains") || !containsSubstring(errMsg, "testRunner") {
					t.Errorf("Expected error about testRunner configuration, got: %v", err)
				}
			},
		},
		{
			name: "dependency-detector with testenv config - should error",
			yaml: `
name: test-project
artifactStorePath: .test.yaml
envFile: .envrc
engines:
  - alias: my-dep-detector
    type: dependency-detector
    dependencyDetector:
      - engine: forge://go-dependency-detector
    testenv:
      - engine: forge://testenv-kind
`,
			shouldError: true,
			validate: func(t *testing.T, spec Spec, err error) {
				if err == nil {
					t.Fatal("Expected validation error for dependency-detector with testenv config")
				}
				errMsg := err.Error()
				if !containsSubstring(errMsg, "but contains") || !containsSubstring(errMsg, "testenv") {
					t.Errorf("Expected error about testenv configuration, got: %v", err)
				}
			},
		},
		{
			name: "dependency-detector with missing engine field - should error",
			yaml: `
name: test-project
artifactStorePath: .test.yaml
envFile: .envrc
engines:
  - alias: my-dep-detector
    type: dependency-detector
    dependencyDetector:
      - spec:
          someConfig: value
`,
			shouldError: true,
			validate: func(t *testing.T, spec Spec, err error) {
				if err == nil {
					t.Fatal("Expected validation error for missing engine field")
				}
				errMsg := err.Error()
				if !containsSubstring(errMsg, "engine") && !containsSubstring(errMsg, "URI") {
					t.Errorf("Expected error about missing engine/URI, got: %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var spec Spec
			err := yaml.Unmarshal([]byte(tt.yaml), &spec)
			if err != nil {
				t.Fatalf("Failed to unmarshal YAML: %v", err)
			}

			// Validate the spec
			err = spec.Validate()
			tt.validate(t, spec, err)
		})
	}
}

func TestTestenvEngineSpecDeferTemplates(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		validate func(t *testing.T, spec Spec)
	}{
		{
			name: "DeferTemplates defaults to false",
			yaml: `
name: test-project
artifactStorePath: .test.yaml
engines:
  - alias: my-testenv
    type: testenv
    testenv:
      - engine: forge://testenv-kind
`,
			validate: func(t *testing.T, spec Spec) {
				if len(spec.Engines) != 1 {
					t.Fatalf("Expected 1 engine, got %d", len(spec.Engines))
				}
				if len(spec.Engines[0].Testenv) != 1 {
					t.Fatalf("Expected 1 testenv, got %d", len(spec.Engines[0].Testenv))
				}
				if spec.Engines[0].Testenv[0].DeferTemplates {
					t.Error("Expected DeferTemplates to default to false")
				}
			},
		},
		{
			name: "DeferTemplates can be set to true",
			yaml: `
name: test-project
artifactStorePath: .test.yaml
engines:
  - alias: my-testenv
    type: testenv
    testenv:
      - engine: forge://testenv-helm
        deferTemplates: true
        spec:
          chart: "my-chart"
          values:
            key: "{{ .Value }}"
`,
			validate: func(t *testing.T, spec Spec) {
				if len(spec.Engines) != 1 {
					t.Fatalf("Expected 1 engine, got %d", len(spec.Engines))
				}
				if len(spec.Engines[0].Testenv) != 1 {
					t.Fatalf("Expected 1 testenv, got %d", len(spec.Engines[0].Testenv))
				}
				testenv := spec.Engines[0].Testenv[0]
				if !testenv.DeferTemplates {
					t.Error("Expected DeferTemplates to be true")
				}
				// Verify spec is preserved with template syntax
				if testenv.Spec == nil {
					t.Fatal("Expected spec to be non-nil")
				}
				if testenv.Spec["chart"] != "my-chart" {
					t.Errorf("Expected chart 'my-chart', got '%v'", testenv.Spec["chart"])
				}
			},
		},
		{
			name: "DeferTemplates explicitly false",
			yaml: `
name: test-project
artifactStorePath: .test.yaml
engines:
  - alias: my-testenv
    type: testenv
    testenv:
      - engine: forge://testenv-kind
        deferTemplates: false
`,
			validate: func(t *testing.T, spec Spec) {
				if len(spec.Engines) != 1 {
					t.Fatalf("Expected 1 engine, got %d", len(spec.Engines))
				}
				if len(spec.Engines[0].Testenv) != 1 {
					t.Fatalf("Expected 1 testenv, got %d", len(spec.Engines[0].Testenv))
				}
				if spec.Engines[0].Testenv[0].DeferTemplates {
					t.Error("Expected DeferTemplates to be false")
				}
			},
		},
		{
			name: "Mixed DeferTemplates in same testenv config",
			yaml: `
name: test-project
artifactStorePath: .test.yaml
engines:
  - alias: my-testenv
    type: testenv
    testenv:
      - engine: forge://testenv-kind
        deferTemplates: false
      - engine: forge://testenv-helm
        deferTemplates: true
        spec:
          values:
            template: "{{ .SomeVar }}"
`,
			validate: func(t *testing.T, spec Spec) {
				if len(spec.Engines) != 1 {
					t.Fatalf("Expected 1 engine, got %d", len(spec.Engines))
				}
				if len(spec.Engines[0].Testenv) != 2 {
					t.Fatalf("Expected 2 testenv entries, got %d", len(spec.Engines[0].Testenv))
				}
				if spec.Engines[0].Testenv[0].DeferTemplates {
					t.Error("Expected first testenv DeferTemplates to be false")
				}
				if !spec.Engines[0].Testenv[1].DeferTemplates {
					t.Error("Expected second testenv DeferTemplates to be true")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var spec Spec
			err := yaml.Unmarshal([]byte(tt.yaml), &spec)
			if err != nil {
				t.Fatalf("Failed to unmarshal YAML: %v", err)
			}
			tt.validate(t, spec)
		})
	}
}

// A registry entry names an engine and nothing else: it validates on its
// own, and one that also carries a type or a composition is refused.
func TestARegistryEntryNamesAnEngineAndComposesNothing(t *testing.T) {
	for _, target := range []string{"./cmd/mine", "../tools/cmd/mine", "/abs/cmd/mine", "forge://example.invalid/tools/cmd/mine@v1.0.0"} {
		entry := EngineConfig{Alias: "mine", Engine: target}
		if err := entry.Validate(); err != nil {
			t.Fatalf("%s must validate: %v", target, err)
		}

		if !entry.IsRegistryEntry() {
			t.Fatalf("%s is a registry entry", target)
		}
	}

	mixed := EngineConfig{Alias: "mine", Engine: "./cmd/mine", Type: BuilderEngineConfigType}
	if err := mixed.Validate(); err == nil || !strings.Contains(err.Error(), "names an engine") {
		t.Fatalf("an entry with an engine and a type must be refused, got %v", err)
	}

	bare := EngineConfig{Alias: "mine", Engine: "cmd/mine"}
	if err := bare.Validate(); err == nil || !strings.Contains(err.Error(), "forge://") {
		t.Fatalf("a target that is neither a URI nor a path must be refused, got %v", err)
	}

	spec := Spec{Engines: []EngineConfig{{Alias: "mine", Engine: "./cmd/mine"}, {Alias: "compose", Type: TestenvEngineConfigType, Testenv: []TestenvEngineSpec{{Engine: "forge://testenv-stub"}}}}}
	if got := spec.Registry(); len(got) != 1 || got["mine"] != "./cmd/mine" {
		t.Fatalf("Registry answers the naming entries only, got %v", got)
	}
}
