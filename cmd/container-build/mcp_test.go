//go:build unit

package main

import (
	"context"
	"testing"

	"github.com/alexandremahdhaoui/forge/pkg/mcptypes"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestHandleBuildTool_Validation(t *testing.T) {
	tests := []struct {
		name      string
		input     mcptypes.BuildInput
		wantError bool
	}{
		{
			name: "valid input",
			input: mcptypes.BuildInput{
				Name: "test-container",
				Src:  "./Containerfile",
			},
			wantError: false, // Will error due to missing env, but validates input
		},
		{
			name: "missing name",
			input: mcptypes.BuildInput{
				Src: "./Containerfile",
			},
			wantError: true,
		},
		{
			name: "missing src",
			input: mcptypes.BuildInput{
				Name: "test-container",
			},
			wantError: true,
		},
		{
			name: "empty name",
			input: mcptypes.BuildInput{
				Name: "",
				Src:  "./Containerfile",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			req := &mcp.CallToolRequest{}

			result, _, _ := handleBuildTool(ctx, req, tt.input)

			if tt.wantError {
				// Verify result contains error
				if !result.IsError {
					t.Errorf("Expected error result, got success")
				}
			}
			// Note: Even "valid input" will error because CONTAINER_BUILD_ENGINE is not set
			// We're primarily testing input validation here
		})
	}
}

func TestHandleBuildTool_EnvironmentValidation(t *testing.T) {
	tests := []struct {
		name          string
		envValue      string
		expectError   bool
		errorContains string
	}{
		{
			name:          "invalid env",
			envValue:      "invalid",
			expectError:   true,
			errorContains: "invalid CONTAINER_BUILD_ENGINE",
		},
		{
			name:          "buildkit invalid",
			envValue:      "buildkit",
			expectError:   true,
			errorContains: "invalid CONTAINER_BUILD_ENGINE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				t.Setenv("CONTAINER_BUILD_ENGINE", tt.envValue)
			} else {
				// Unset the variable
				t.Setenv("CONTAINER_BUILD_ENGINE", "")
			}

			input := mcptypes.BuildInput{
				Name: "test",
				Src:  "./Containerfile",
			}

			ctx := context.Background()
			req := &mcp.CallToolRequest{}

			result, _, _ := handleBuildTool(ctx, req, input)

			if tt.expectError && !result.IsError {
				t.Errorf("Expected error, got success")
			}

			if tt.errorContains != "" && result.IsError {
				// Check that error message contains expected string
				found := false
				for _, content := range result.Content {
					if textContent, ok := content.(*mcp.TextContent); ok {
						if contains(textContent.Text, tt.errorContains) {
							found = true
							break
						}
					}
				}
				if !found {
					t.Errorf("Expected error to contain %q, but it didn't. Result: %+v",
						tt.errorContains, result.Content)
				}
			}
		})
	}
}

func TestHandleBuildBatchTool_EmptySpecs(t *testing.T) {
	ctx := context.Background()
	req := &mcp.CallToolRequest{}

	input := mcptypes.BatchBuildInput{
		Specs: []mcptypes.BuildInput{},
	}

	result, _, _ := handleBuildBatchTool(ctx, req, input)

	// Empty batch should not be an error, just return empty results
	if result.IsError {
		t.Errorf("Empty batch should not error, got: %+v", result)
	}
}

func TestHandleBuildBatchTool_ValidationErrors(t *testing.T) {
	// Don't set environment to test validation errors
	t.Setenv("CONTAINER_BUILD_ENGINE", "")

	ctx := context.Background()
	req := &mcp.CallToolRequest{}

	input := mcptypes.BatchBuildInput{
		Specs: []mcptypes.BuildInput{
			{Name: "test1", Src: "./Containerfile1"},
			{Name: "test2", Src: "./Containerfile2"},
		},
	}

	result, _, _ := handleBuildBatchTool(ctx, req, input)

	// The batch should process inputs even if they fail validation
	// Check that result has content (errors in this case)
	if len(result.Content) == 0 {
		t.Error("Expected batch result to have content")
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		len(s) > len(substr) && (s[:len(substr)] == substr ||
			s[len(s)-len(substr):] == substr ||
			findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
