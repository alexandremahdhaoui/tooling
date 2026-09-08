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
	"path/filepath"
	"testing"
)

// TestEngineResolution_FromExternalProject tests that engine URI resolution works
// when forge is called from outside the forge repository (simulating a user project).
func TestEngineResolution_FromExternalProject(t *testing.T) {
	// Create a temporary directory to simulate an external user project
	tmpDir := t.TempDir()

	// Create a minimal forge.yaml in the temp directory
	forgeYAML := `name: test-external-project
artifactStorePath: .ignore.artifact-store.yaml

build:
  - name: test-binary
    src: ./cmd/test
    dest: ./build/bin
    engine: forge://go-build

test:
  - name: integration
    testenv: forge://testenv
    runner: forge://go-test
`
	if err := os.WriteFile(filepath.Join(tmpDir, "forge.yaml"), []byte(forgeYAML), 0o644); err != nil {
		t.Fatalf("Failed to create forge.yaml: %v", err)
	}

	// Save current working directory (which should be the forge repo)
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}

	// Find the forge repository root (go up to find go.mod)
	forgeRoot := originalWd
	for {
		if _, err := os.Stat(filepath.Join(forgeRoot, "go.mod")); err == nil {
			// Found go.mod, check if it's the forge repo
			break
		}
		parent := filepath.Dir(forgeRoot)
		if parent == forgeRoot {
			t.Fatalf("Could not find forge repository root")
		}
		forgeRoot = parent
	}

	// Set FORGE_RUN_LOCAL_ENABLED and FORGE_RUN_LOCAL_BASEDIR so forgepath.FindForgeRepo() can locate it
	// even when we change to a different directory
	t.Setenv("FORGE_RUN_LOCAL_ENABLED", "true")
	t.Setenv("FORGE_RUN_LOCAL_BASEDIR", forgeRoot)

	defer func() {
		// Restore original working directory
		if err := os.Chdir(originalWd); err != nil {
			t.Errorf("Failed to restore working directory: %v", err)
		}
	}()

	// Change to the temporary directory (simulating running forge from user project)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Test parsing various engine URIs
	testCases := []struct {
		name      string
		engineURI string
		wantType  string
		wantCmd   string
	}{
		{
			name:      "go-build engine",
			engineURI: "forge://go-build",
			wantType:  "mcp",
			wantCmd:   "go",
		},
		{
			name:      "testenv engine",
			engineURI: "forge://testenv",
			wantType:  "mcp",
			wantCmd:   "go",
		},
		{
			name:      "testenv-kind engine",
			engineURI: "forge://testenv-kind",
			wantType:  "mcp",
			wantCmd:   "go",
		},
		{
			name:      "go-test engine",
			engineURI: "forge://go-test",
			wantType:  "mcp",
			wantCmd:   "go",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Parse the engine URI
			gotType, gotCmd, gotArgs, err := parseEngine(tc.engineURI, "v0.9.0")
			if err != nil {
				t.Fatalf("parseEngine(%q) error = %v, want nil", tc.engineURI, err)
			}

			// Verify engine type
			if gotType != tc.wantType {
				t.Errorf("parseEngine(%q) type = %q, want %q", tc.engineURI, gotType, tc.wantType)
			}

			// Verify command is "go"
			if gotCmd != tc.wantCmd {
				t.Errorf("parseEngine(%q) command = %q, want %q", tc.engineURI, gotCmd, tc.wantCmd)
			}

			// Verify args contain "run" (may be at index 0 or after -C flag)
			runIndex := -1
			for i, arg := range gotArgs {
				if arg == "run" {
					runIndex = i
					break
				}
			}
			if runIndex == -1 {
				t.Errorf("parseEngine(%q) args = %v, want 'run' argument", tc.engineURI, gotArgs)
			}

			// If using -C flag, verify it comes before "run"
			if len(gotArgs) >= 2 && gotArgs[0] == "-C" {
				if runIndex < 2 {
					t.Errorf("parseEngine(%q) args = %v, want 'run' after -C flag", tc.engineURI, gotArgs)
				}
			}

			// Verify args contain forge package path after "run"
			if runIndex >= 0 && runIndex+1 < len(gotArgs) {
				// Args should be like ["run", "github.com/alexandremahdhaoui/forge/cmd/go-build"]
				// or ["-C", "/path/to/forge", "run", "./cmd/go-build"]
				packagePath := gotArgs[runIndex+1]
				if packagePath == "" {
					t.Errorf("parseEngine(%q) package path is empty", tc.engineURI)
				}
			} else if runIndex >= 0 {
				t.Errorf("parseEngine(%q) args = %v, want package path after 'run'", tc.engineURI, gotArgs)
			}

			t.Logf("parseEngine(%q) -> command=%s, args=%v", tc.engineURI, gotCmd, gotArgs)
		})
	}
}

// TestEngineResolution_AliasHandling tests that alias:// URIs are still handled correctly
func TestEngineResolution_AliasHandling(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()

	// Create a forge.yaml with engine aliases
	forgeYAML := `name: test-alias-project
artifactStorePath: .ignore.artifact-store.yaml

engines:
  - alias: my-builder
    type: builder
    builder:
      - engine: forge://go-build

test:
  - name: unit
    testenv: alias://my-testenv
    runner: forge://go-test
`
	if err := os.WriteFile(filepath.Join(tmpDir, "forge.yaml"), []byte(forgeYAML), 0o644); err != nil {
		t.Fatalf("Failed to create forge.yaml: %v", err)
	}

	// Save and restore working directory
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(originalWd); err != nil {
			t.Errorf("Failed to restore working directory: %v", err)
		}
	}()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Test parsing an alias URI
	gotType, gotCmd, gotArgs, err := parseEngine("alias://my-builder", "v0.9.0")
	if err != nil {
		t.Fatalf("parseEngine('alias://my-builder') error = %v, want nil", err)
	}

	// For aliases, the type should be "alias" and command should be the alias name
	if gotType != "alias" {
		t.Errorf("parseEngine('alias://my-builder') type = %q, want 'alias'", gotType)
	}

	if gotCmd != "my-builder" {
		t.Errorf("parseEngine('alias://my-builder') command = %q, want 'my-builder'", gotCmd)
	}

	if gotArgs != nil {
		t.Errorf("parseEngine('alias://my-builder') args = %v, want nil", gotArgs)
	}

	t.Logf("parseEngine('alias://my-builder') -> type=%s, command=%s, args=%v", gotType, gotCmd, gotArgs)
}

// TestEngineResolution_ErrorCases tests error handling in engine resolution
func TestEngineResolution_ErrorCases(t *testing.T) {
	testCases := []struct {
		name      string
		engineURI string
		wantErr   bool
	}{
		{
			name:      "unsupported protocol",
			engineURI: "http://example.com",
			wantErr:   true,
		},
		{
			name:      "empty forge:// path",
			engineURI: "forge://",
			wantErr:   true,
		},
		{
			name:      "empty alias:// path",
			engineURI: "alias://",
			wantErr:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, _, err := parseEngine(tc.engineURI, "v0.9.0")
			if (err != nil) != tc.wantErr {
				t.Errorf("parseEngine(%q) error = %v, wantErr = %v", tc.engineURI, err, tc.wantErr)
			}
		})
	}
}
