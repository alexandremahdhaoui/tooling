//go:build unit

package forge

import (
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
      - engine: go://generic-builder
        spec:
          command: "gofmt"
          args: ["-w", "."]
          env:
            GOFMT_STYLE: "google"
            DEBUG: "true"
          envFile: ".envrc"
          workDir: "/tmp/test"

  - alias: my-linter
    type: test-runner
    testRunner:
      - engine: go://generic-test-runner
        spec:
          command: "golangci-lint"
          args: ["run", "./..."]

  - alias: my-testenv
    type: testenv
    testenv:
      - engine: go://testenv-kind
      - engine: go://testenv-lcr
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
	if formatter.Builder[0].Engine != "go://generic-builder" {
		t.Errorf("Expected engine 'go://generic-builder', got '%s'", formatter.Builder[0].Engine)
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
	if formatter.Builder[0].Spec.WorkDir != "/tmp/test" {
		t.Errorf("Expected workDir '/tmp/test', got '%s'", formatter.Builder[0].Spec.WorkDir)
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
	if linter.TestRunner[0].Engine != "go://generic-test-runner" {
		t.Errorf("Expected engine 'go://generic-test-runner', got '%s'", linter.TestRunner[0].Engine)
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
	if testenv.Testenv[0].Engine != "go://testenv-kind" {
		t.Errorf("Expected engine 'go://testenv-kind', got '%s'", testenv.Testenv[0].Engine)
	}
	if testenv.Testenv[1].Engine != "go://testenv-lcr" {
		t.Errorf("Expected engine 'go://testenv-lcr', got '%s'", testenv.Testenv[1].Engine)
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
      - engine: go://generic-builder
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
				if eng.Builder[0].Engine != "go://generic-builder" {
					t.Errorf("Expected engine 'go://generic-builder', got '%s'", eng.Builder[0].Engine)
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
      - engine: go://generic-builder
        spec:
          command: "test-cmd"
          args: ["arg1", "arg2", "arg3"]
          env:
            VAR1: "value1"
            VAR2: "value2"
            VAR3: "value3"
          envFile: ".env.test"
          workDir: "/custom/dir"
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
				if eng.Builder[0].Spec.WorkDir != "/custom/dir" {
					t.Errorf("Expected workDir '/custom/dir', got '%s'", eng.Builder[0].Spec.WorkDir)
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
