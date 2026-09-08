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

package mcptypes

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestDirectoryParamsJSONMarshaling tests DirectoryParams JSON marshaling/unmarshaling
func TestDirectoryParamsJSONMarshaling(t *testing.T) {
	tests := []struct {
		name     string
		params   DirectoryParams
		expected string
	}{
		{
			name: "All fields populated",
			params: DirectoryParams{
				TmpDir:   "/tmp/test",
				BuildDir: "/build",
				RootDir:  "/root",
			},
			expected: `{"tmpDir":"/tmp/test","buildDir":"/build","rootDir":"/root"}`,
		},
		{
			name:     "Empty struct - omitempty behavior",
			params:   DirectoryParams{},
			expected: `{}`,
		},
		{
			name: "Partial fields",
			params: DirectoryParams{
				TmpDir: "/tmp/test",
			},
			expected: `{"tmpDir":"/tmp/test"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test marshaling
			data, err := json.Marshal(tt.params)
			if err != nil {
				t.Fatalf("Failed to marshal: %v", err)
			}

			if string(data) != tt.expected {
				t.Errorf("Marshaled JSON mismatch:\ngot:  %s\nwant: %s", string(data), tt.expected)
			}

			// Test unmarshaling
			var unmarshaled DirectoryParams
			if err := json.Unmarshal(data, &unmarshaled); err != nil {
				t.Fatalf("Failed to unmarshal: %v", err)
			}

			if unmarshaled != tt.params {
				t.Errorf("Unmarshaled struct mismatch:\ngot:  %+v\nwant: %+v", unmarshaled, tt.params)
			}
		})
	}
}

// TestRunInputJSONMarshaling tests RunInput JSON marshaling/unmarshaling
func TestRunInputJSONMarshaling(t *testing.T) {
	tests := []struct {
		name  string
		input RunInput
	}{
		{
			name: "Required fields only",
			input: RunInput{
				Stage: "unit",
				Name:  "test-1",
			},
		},
		{
			name: "All fields populated",
			input: RunInput{
				Stage:   "integration",
				Name:    "test-2",
				Command: "go test",
				Args:    []string{"-v", "-race"},
				Env: map[string]string{
					"GO_ENV": "test",
				},
				EnvFile: ".env.test",
				DirectoryParams: DirectoryParams{
					TmpDir:   "/tmp/test",
					BuildDir: "/build",
					RootDir:  "/root",
				},
			},
		},
		{
			name: "Generic test runner fields",
			input: RunInput{
				Stage:   "e2e",
				Name:    "test-3",
				Command: "npm test",
				Args:    []string{"--coverage"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test marshaling
			data, err := json.Marshal(tt.input)
			if err != nil {
				t.Fatalf("Failed to marshal: %v", err)
			}

			// Test unmarshaling
			var unmarshaled RunInput
			if err := json.Unmarshal(data, &unmarshaled); err != nil {
				t.Fatalf("Failed to unmarshal: %v", err)
			}

			// Compare required fields
			if unmarshaled.Stage != tt.input.Stage {
				t.Errorf("Stage mismatch: got %s, want %s", unmarshaled.Stage, tt.input.Stage)
			}
			if unmarshaled.Name != tt.input.Name {
				t.Errorf("Name mismatch: got %s, want %s", unmarshaled.Name, tt.input.Name)
			}

			// Compare optional fields if they were set
			if tt.input.Command != "" && unmarshaled.Command != tt.input.Command {
				t.Errorf("Command mismatch: got %s, want %s", unmarshaled.Command, tt.input.Command)
			}

			// Compare directory params
			if tt.input.TmpDir != "" && unmarshaled.TmpDir != tt.input.TmpDir {
				t.Errorf("TmpDir mismatch: got %s, want %s", unmarshaled.TmpDir, tt.input.TmpDir)
			}
		})
	}
}

// TestBuildInputJSONMarshaling tests BuildInput JSON marshaling/unmarshaling
func TestBuildInputJSONMarshaling(t *testing.T) {
	tests := []struct {
		name  string
		input BuildInput
	}{
		{
			name: "Basic build input",
			input: BuildInput{
				Name:   "my-app",
				Src:    "./cmd/app",
				Dest:   "./build/bin",
				Engine: "forge://go-build",
			},
		},
		{
			name: "Build input with directories",
			input: BuildInput{
				Name:   "my-app",
				Src:    "./cmd/app",
				Dest:   "./build/bin",
				Engine: "forge://go-build",
				DirectoryParams: DirectoryParams{
					TmpDir:   "/tmp/build-123",
					BuildDir: "/build",
					RootDir:  "/workspace",
				},
			},
		},
		{
			name: "Generic engine with command",
			input: BuildInput{
				Name:    "format-code",
				Src:     ".",
				Engine:  "forge://generic-engine",
				Command: "gofmt",
				Args:    []string{"-w", "."},
				Env: map[string]string{
					"GOOS":   "linux",
					"GOARCH": "amd64",
				},
			},
		},
		{
			name: "Format-go specific fields",
			input: BuildInput{
				Name:   "format-code",
				Path:   "./cmd",
				Src:    ".",
				Engine: "forge://go-format",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test marshaling
			data, err := json.Marshal(tt.input)
			if err != nil {
				t.Fatalf("Failed to marshal: %v", err)
			}

			// Test unmarshaling
			var unmarshaled BuildInput
			if err := json.Unmarshal(data, &unmarshaled); err != nil {
				t.Fatalf("Failed to unmarshal: %v", err)
			}

			// Compare required fields
			if unmarshaled.Name != tt.input.Name {
				t.Errorf("Name mismatch: got %s, want %s", unmarshaled.Name, tt.input.Name)
			}
			if unmarshaled.Engine != tt.input.Engine {
				t.Errorf("Engine mismatch: got %s, want %s", unmarshaled.Engine, tt.input.Engine)
			}

			// Compare optional fields if they were set
			if tt.input.Src != "" && unmarshaled.Src != tt.input.Src {
				t.Errorf("Src mismatch: got %s, want %s", unmarshaled.Src, tt.input.Src)
			}
			if tt.input.Command != "" && unmarshaled.Command != tt.input.Command {
				t.Errorf("Command mismatch: got %s, want %s", unmarshaled.Command, tt.input.Command)
			}
			if tt.input.Path != "" && unmarshaled.Path != tt.input.Path {
				t.Errorf("Path mismatch: got %s, want %s", unmarshaled.Path, tt.input.Path)
			}
		})
	}
}

// TestBatchBuildInputJSONMarshaling tests BatchBuildInput JSON marshaling/unmarshaling
func TestBatchBuildInputJSONMarshaling(t *testing.T) {
	input := BatchBuildInput{
		Specs: []BuildInput{
			{
				Name:   "app1",
				Src:    "./cmd/app1",
				Dest:   "./build/bin",
				Engine: "forge://go-build",
			},
			{
				Name:   "app2",
				Src:    "./cmd/app2",
				Dest:   "./build/bin",
				Engine: "forge://go-build",
			},
		},
	}

	// Test marshaling
	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Test unmarshaling
	var unmarshaled BatchBuildInput
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Verify specs count
	if len(unmarshaled.Specs) != len(input.Specs) {
		t.Errorf("Specs count mismatch: got %d, want %d", len(unmarshaled.Specs), len(input.Specs))
	}

	// Verify first spec
	if len(unmarshaled.Specs) > 0 && unmarshaled.Specs[0].Name != input.Specs[0].Name {
		t.Errorf("First spec name mismatch: got %s, want %s", unmarshaled.Specs[0].Name, input.Specs[0].Name)
	}
}

// TestRunInputRequiredFields tests that required fields are present
func TestRunInputRequiredFields(t *testing.T) {
	input := RunInput{
		Stage: "unit",
		Name:  "my-test",
	}

	if input.Stage == "" {
		t.Error("Stage should not be empty")
	}
	if input.Name == "" {
		t.Error("Name should not be empty")
	}
}

// TestBuildInputRequiredFields tests that required fields are present
func TestBuildInputRequiredFields(t *testing.T) {
	input := BuildInput{
		Name:   "my-artifact",
		Engine: "forge://go-build",
	}

	if input.Name == "" {
		t.Error("Name should not be empty")
	}
	if input.Engine == "" {
		t.Error("Engine should not be empty")
	}
}

// TestDirectoryParamsEmbedding tests that DirectoryParams is properly embedded
func TestDirectoryParamsEmbedding(t *testing.T) {
	runInput := RunInput{
		Stage: "unit",
		Name:  "test",
		DirectoryParams: DirectoryParams{
			TmpDir:   "/tmp",
			BuildDir: "/build",
			RootDir:  "/root",
		},
	}

	// Verify we can access embedded fields directly
	if runInput.TmpDir != "/tmp" {
		t.Errorf("TmpDir: got %s, want /tmp", runInput.TmpDir)
	}
	if runInput.BuildDir != "/build" {
		t.Errorf("BuildDir: got %s, want /build", runInput.BuildDir)
	}
	if runInput.RootDir != "/root" {
		t.Errorf("RootDir: got %s, want /root", runInput.RootDir)
	}

	buildInput := BuildInput{
		Name:   "artifact",
		Engine: "test",
		DirectoryParams: DirectoryParams{
			TmpDir:   "/tmp2",
			BuildDir: "/build2",
			RootDir:  "/root2",
		},
	}

	// Verify we can access embedded fields directly
	if buildInput.TmpDir != "/tmp2" {
		t.Errorf("TmpDir: got %s, want /tmp2", buildInput.TmpDir)
	}
	if buildInput.BuildDir != "/build2" {
		t.Errorf("BuildDir: got %s, want /build2", buildInput.BuildDir)
	}
	if buildInput.RootDir != "/root2" {
		t.Errorf("RootDir: got %s, want /root2", buildInput.RootDir)
	}
}

// TestDetectDependenciesInputJSONMarshaling tests DetectDependenciesInput JSON marshaling/unmarshaling
func TestDetectDependenciesInputJSONMarshaling(t *testing.T) {
	tests := []struct {
		name  string
		input DetectDependenciesInput
	}{
		{
			name: "Basic input",
			input: DetectDependenciesInput{
				FilePath: "/workspace/cmd/app/main.go",
				FuncName: "main",
			},
		},
		{
			name: "Input with spec",
			input: DetectDependenciesInput{
				FilePath: "/workspace/cmd/app/main.go",
				FuncName: "main",
				Spec: map[string]interface{}{
					"maxDepth":     5,
					"includeTests": false,
				},
			},
		},
		{
			name: "Input with directory params",
			input: DetectDependenciesInput{
				FilePath: "/workspace/cmd/app/main.go",
				FuncName: "main",
				DirectoryParams: DirectoryParams{
					TmpDir:   "/tmp/detect-123",
					BuildDir: "/build",
					RootDir:  "/workspace",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test marshaling
			data, err := json.Marshal(tt.input)
			if err != nil {
				t.Fatalf("Failed to marshal: %v", err)
			}

			// Test unmarshaling
			var unmarshaled DetectDependenciesInput
			if err := json.Unmarshal(data, &unmarshaled); err != nil {
				t.Fatalf("Failed to unmarshal: %v", err)
			}

			// Compare required fields
			if unmarshaled.FilePath != tt.input.FilePath {
				t.Errorf("FilePath mismatch: got %s, want %s", unmarshaled.FilePath, tt.input.FilePath)
			}
			if unmarshaled.FuncName != tt.input.FuncName {
				t.Errorf("FuncName mismatch: got %s, want %s", unmarshaled.FuncName, tt.input.FuncName)
			}

			// Compare directory params if set
			if tt.input.TmpDir != "" && unmarshaled.TmpDir != tt.input.TmpDir {
				t.Errorf("TmpDir mismatch: got %s, want %s", unmarshaled.TmpDir, tt.input.TmpDir)
			}
		})
	}
}

// TestDependencyJSONMarshaling pins the wire shape of a dependency: a path
// and the digest of its content, nothing else. A round trip keeps both.
func TestDependencyJSONMarshaling(t *testing.T) {
	dep := Dependency{
		Path:   "/workspace/pkg/util/helper.go",
		Digest: "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	}
	data, err := json.Marshal(dep)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}
	for _, key := range []string{`"path"`, `"digest"`} {
		if !strings.Contains(string(data), key) {
			t.Errorf("marshaled dependency lacks %s: %s", key, data)
		}
	}
	var unmarshaled Dependency
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}
	if unmarshaled != dep {
		t.Errorf("round trip changed the dependency: got %+v, want %+v", unmarshaled, dep)
	}
}

// TestDetectDependenciesOutputJSONMarshaling tests DetectDependenciesOutput JSON marshaling/unmarshaling
func TestDetectDependenciesOutputJSONMarshaling(t *testing.T) {
	output := DetectDependenciesOutput{
		Dependencies: []Dependency{
			{Path: "/workspace/pkg/util/helper.go", Digest: "sha256:aa"},
			{Path: "/workspace/go.sum", Digest: "sha256:bb"},
		},
	}

	// Test marshaling
	data, err := json.Marshal(output)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Test unmarshaling
	var unmarshaled DetectDependenciesOutput
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Verify dependencies count
	if len(unmarshaled.Dependencies) != len(output.Dependencies) {
		t.Errorf("Dependencies count mismatch: got %d, want %d", len(unmarshaled.Dependencies), len(output.Dependencies))
	}

	// Verify first dependency
	if len(unmarshaled.Dependencies) > 0 {
		if unmarshaled.Dependencies[0] != output.Dependencies[0] {
			t.Errorf("First dependency mismatch: got %+v, want %+v", unmarshaled.Dependencies[0], output.Dependencies[0])
		}
	}
}

// TestDetectDependenciesInputRequiredFields tests that required fields are present
func TestDetectDependenciesInputRequiredFields(t *testing.T) {
	input := DetectDependenciesInput{
		FilePath: "/workspace/cmd/app/main.go",
		FuncName: "main",
	}

	if input.FilePath == "" {
		t.Error("FilePath should not be empty")
	}
	if input.FuncName == "" {
		t.Error("FuncName should not be empty")
	}
}
