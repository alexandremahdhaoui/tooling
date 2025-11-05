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
    engine: go://generic-engine
    config:
      command: "gofmt"
      args: ["-w", "."]
      env:
        GOFMT_STYLE: "google"
        DEBUG: "true"
      envFile: ".envrc"
      workDir: "/tmp/test"

  - alias: my-linter
    engine: go://generic-test-runner
    config:
      command: "golangci-lint"
      args: ["run", "./..."]
`

	var spec Spec
	err := yaml.Unmarshal([]byte(yamlContent), &spec)
	if err != nil {
		t.Fatalf("Failed to unmarshal YAML: %v", err)
	}

	// Verify engines were parsed
	if len(spec.Engines) != 2 {
		t.Fatalf("Expected 2 engines, got %d", len(spec.Engines))
	}

	// Verify first engine (my-formatter)
	formatter := spec.Engines[0]
	if formatter.Alias != "my-formatter" {
		t.Errorf("Expected alias 'my-formatter', got '%s'", formatter.Alias)
	}
	if formatter.Engine != "go://generic-engine" {
		t.Errorf("Expected engine 'go://generic-engine', got '%s'", formatter.Engine)
	}
	if formatter.Config.Command != "gofmt" {
		t.Errorf("Expected command 'gofmt', got '%s'", formatter.Config.Command)
	}
	if len(formatter.Config.Args) != 2 {
		t.Fatalf("Expected 2 args, got %d", len(formatter.Config.Args))
	}
	if formatter.Config.Args[0] != "-w" {
		t.Errorf("Expected arg '-w', got '%s'", formatter.Config.Args[0])
	}
	if formatter.Config.Args[1] != "." {
		t.Errorf("Expected arg '.', got '%s'", formatter.Config.Args[1])
	}
	if len(formatter.Config.Env) != 2 {
		t.Fatalf("Expected 2 env vars, got %d", len(formatter.Config.Env))
	}
	if formatter.Config.Env["GOFMT_STYLE"] != "google" {
		t.Errorf("Expected env GOFMT_STYLE='google', got '%s'", formatter.Config.Env["GOFMT_STYLE"])
	}
	if formatter.Config.Env["DEBUG"] != "true" {
		t.Errorf("Expected env DEBUG='true', got '%s'", formatter.Config.Env["DEBUG"])
	}
	if formatter.Config.EnvFile != ".envrc" {
		t.Errorf("Expected envFile '.envrc', got '%s'", formatter.Config.EnvFile)
	}
	if formatter.Config.WorkDir != "/tmp/test" {
		t.Errorf("Expected workDir '/tmp/test', got '%s'", formatter.Config.WorkDir)
	}

	// Verify second engine (my-linter)
	linter := spec.Engines[1]
	if linter.Alias != "my-linter" {
		t.Errorf("Expected alias 'my-linter', got '%s'", linter.Alias)
	}
	if linter.Engine != "go://generic-test-runner" {
		t.Errorf("Expected engine 'go://generic-test-runner', got '%s'", linter.Engine)
	}
	if linter.Config.Command != "golangci-lint" {
		t.Errorf("Expected command 'golangci-lint', got '%s'", linter.Config.Command)
	}
	if len(linter.Config.Args) != 2 {
		t.Fatalf("Expected 2 args, got %d", len(linter.Config.Args))
	}
	if linter.Config.Args[0] != "run" {
		t.Errorf("Expected arg 'run', got '%s'", linter.Config.Args[0])
	}
	if linter.Config.Args[1] != "./..." {
		t.Errorf("Expected arg './...', got '%s'", linter.Config.Args[1])
	}
}

func TestEngineConfigOptionalFields(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		validate func(t *testing.T, spec Spec)
	}{
		{
			name: "Minimal config - only alias and engine",
			yaml: `
name: test-project
artifactStorePath: .test.yaml
engines:
  - alias: minimal
    engine: go://generic-engine
`,
			validate: func(t *testing.T, spec Spec) {
				if len(spec.Engines) != 1 {
					t.Fatalf("Expected 1 engine, got %d", len(spec.Engines))
				}
				eng := spec.Engines[0]
				if eng.Alias != "minimal" {
					t.Errorf("Expected alias 'minimal', got '%s'", eng.Alias)
				}
				if eng.Engine != "go://generic-engine" {
					t.Errorf("Expected engine 'go://generic-engine', got '%s'", eng.Engine)
				}
				// Verify config is empty/default
				if eng.Config.Command != "" {
					t.Errorf("Expected empty command, got '%s'", eng.Config.Command)
				}
				if len(eng.Config.Args) != 0 {
					t.Errorf("Expected no args, got %d", len(eng.Config.Args))
				}
				if len(eng.Config.Env) != 0 {
					t.Errorf("Expected no env vars, got %d", len(eng.Config.Env))
				}
				if eng.Config.EnvFile != "" {
					t.Errorf("Expected empty envFile, got '%s'", eng.Config.EnvFile)
				}
			},
		},
		{
			name: "Full config - all fields",
			yaml: `
name: test-project
artifactStorePath: .test.yaml
engines:
  - alias: full
    engine: go://generic-engine
    config:
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
				if eng.Config.Command != "test-cmd" {
					t.Errorf("Expected command 'test-cmd', got '%s'", eng.Config.Command)
				}
				if len(eng.Config.Args) != 3 {
					t.Fatalf("Expected 3 args, got %d", len(eng.Config.Args))
				}
				if len(eng.Config.Env) != 3 {
					t.Fatalf("Expected 3 env vars, got %d", len(eng.Config.Env))
				}
				if eng.Config.EnvFile != ".env.test" {
					t.Errorf("Expected envFile '.env.test', got '%s'", eng.Config.EnvFile)
				}
				if eng.Config.WorkDir != "/custom/dir" {
					t.Errorf("Expected workDir '/custom/dir', got '%s'", eng.Config.WorkDir)
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
