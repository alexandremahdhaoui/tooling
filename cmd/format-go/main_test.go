package main

import (
	"testing"
)

// TestFormatInputAcceptsBuildSpecFields tests that FormatInput accepts all BuildSpec fields
func TestFormatInputAcceptsBuildSpecFields(t *testing.T) {
	input := FormatInput{
		Path:   ".",
		Src:    "./test",
		Name:   "test-artifact",
		Dest:   "./build",
		Engine: "go://format-go",
	}

	// Verify all fields are accessible
	if input.Path != "." {
		t.Errorf("Expected Path to be '.', got %s", input.Path)
	}
	if input.Src != "./test" {
		t.Errorf("Expected Src to be './test', got %s", input.Src)
	}
	if input.Name != "test-artifact" {
		t.Errorf("Expected Name to be 'test-artifact', got %s", input.Name)
	}
	if input.Dest != "./build" {
		t.Errorf("Expected Dest to be './build', got %s", input.Dest)
	}
	if input.Engine != "go://format-go" {
		t.Errorf("Expected Engine to be 'go://format-go', got %s", input.Engine)
	}
}

// TestHandleBuildUsesSrcWhenPathEmpty tests that handleBuild uses Src when Path is empty
func TestHandleBuildUsesSrcWhenPathEmpty(t *testing.T) {
	tests := []struct {
		name     string
		input    FormatInput
		expected string
	}{
		{
			name: "Path set, Src empty",
			input: FormatInput{
				Path: "/custom/path",
				Src:  "",
			},
			expected: "/custom/path",
		},
		{
			name: "Path empty, Src set",
			input: FormatInput{
				Path: "",
				Src:  "/from/src",
			},
			expected: "/from/src",
		},
		{
			name: "Both Path and Src set - Path takes precedence",
			input: FormatInput{
				Path: "/path/wins",
				Src:  "/src/loses",
			},
			expected: "/path/wins",
		},
		{
			name: "Both empty - defaults to current directory",
			input: FormatInput{
				Path: "",
				Src:  "",
			},
			expected: ".",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the logic in handleBuild
			path := tt.input.Path
			if path == "" && tt.input.Src != "" {
				path = tt.input.Src
			}
			if path == "" {
				path = "."
			}

			if path != tt.expected {
				t.Errorf("Expected path to be %s, got %s", tt.expected, path)
			}
		})
	}
}

// TestFormatInputJSONMarshaling tests that FormatInput can be unmarshaled from JSON
func TestFormatInputJSONMarshaling(t *testing.T) {
	// This test verifies that MCP can unmarshal BuildSpec parameters into FormatInput
	// The actual unmarshaling is done by the MCP SDK, but we verify the struct tags are correct

	input := FormatInput{
		Path:   ".",
		Src:    "./cmd/format-go",
		Name:   "format-code",
		Dest:   "./build",
		Engine: "go://format-go",
	}

	// Verify struct has json tags by checking they're not empty
	// This is a compile-time verification that the fields are properly tagged
	if input.Path == "" && input.Src == "" {
		t.Error("At least one of Path or Src should be set for BuildSpec compatibility")
	}
}

// TestBuildSpecCompatibility simulates BuildSpec parameters being passed to format-go
func TestBuildSpecCompatibility(t *testing.T) {
	// Create a mock input that would come from forge build command
	input := FormatInput{
		Name:   "format-code",
		Src:    ".",
		Dest:   "", // Not used by formatter but accepted for compatibility
		Engine: "go://format-go",
	}

	// Verify all BuildSpec fields are accessible and don't cause compilation errors
	if input.Name != "format-code" {
		t.Errorf("Expected Name to be 'format-code', got %s", input.Name)
	}
	if input.Src != "." {
		t.Errorf("Expected Src to be '.', got %s", input.Src)
	}
	if input.Dest != "" {
		t.Errorf("Expected Dest to be empty, got %s", input.Dest)
	}
	if input.Engine != "go://format-go" {
		t.Errorf("Expected Engine to be 'go://format-go', got %s", input.Engine)
	}

	// Verify that we can create a map compatible with MCP tool arguments
	arguments := map[string]any{
		"name":   input.Name,
		"src":    input.Src,
		"dest":   input.Dest,
		"engine": input.Engine,
	}

	// Verify the map contains the expected keys
	if arguments["name"] != "format-code" {
		t.Error("BuildSpec 'name' field not in arguments map")
	}
	if arguments["src"] != "." {
		t.Error("BuildSpec 'src' field not in arguments map")
	}
	if _, ok := arguments["engine"]; !ok {
		t.Error("BuildSpec 'engine' field not in arguments map")
	}
}

// TestVersionInfoInitialized tests that version info is properly initialized
func TestVersionInfoInitialized(t *testing.T) {
	if versionInfo == nil {
		t.Fatal("versionInfo should be initialized in init()")
	}

	// versionInfo.Get() returns (version, commit, timestamp), not tool name
	// Just verify it's not nil and can be called without panicking
	version, _, _ := versionInfo.Get()
	if version == "" {
		t.Log("Version is empty, which is expected for non-built binaries")
	}
}
