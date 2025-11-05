//go:build unit

package mcptypes

import (
	"encoding/json"
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
				WorkDir: "/workspace",
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
				WorkDir: "/app",
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
			if tt.input.WorkDir != "" && unmarshaled.WorkDir != tt.input.WorkDir {
				t.Errorf("WorkDir mismatch: got %s, want %s", unmarshaled.WorkDir, tt.input.WorkDir)
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
				Engine: "go://build-go",
			},
		},
		{
			name: "Build input with directories",
			input: BuildInput{
				Name:   "my-app",
				Src:    "./cmd/app",
				Dest:   "./build/bin",
				Engine: "go://build-go",
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
				Engine:  "go://generic-engine",
				Command: "gofmt",
				Args:    []string{"-w", "."},
				Env: map[string]string{
					"GOOS":   "linux",
					"GOARCH": "amd64",
				},
				WorkDir: "/app",
			},
		},
		{
			name: "Format-go specific fields",
			input: BuildInput{
				Name:   "format-code",
				Path:   "./cmd",
				Src:    ".",
				Engine: "go://format-go",
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
				Engine: "go://build-go",
			},
			{
				Name:   "app2",
				Src:    "./cmd/app2",
				Dest:   "./build/bin",
				Engine: "go://build-go",
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
		Engine: "go://build-go",
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
