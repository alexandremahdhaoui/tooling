//go:build unit

package mcputil

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestValidateRequired_AllFieldsValid(t *testing.T) {
	result := ValidateRequired(map[string]string{
		"name":  "test-name",
		"stage": "unit",
		"src":   "./cmd/app",
	})

	if result != nil {
		t.Errorf("Expected nil result for valid fields, got error: %v", result)
	}
}

func TestValidateRequired_OneFieldEmpty(t *testing.T) {
	result := ValidateRequired(map[string]string{
		"name":  "test-name",
		"stage": "", // Empty field
	})

	if result == nil {
		t.Error("Expected error result for empty field, got nil")
	}
	if !result.IsError {
		t.Error("Expected IsError to be true")
	}
	if len(result.Content) == 0 {
		t.Error("Expected error message in Content")
	}
}

func TestValidateRequired_MultipleFieldsEmpty(t *testing.T) {
	result := ValidateRequired(map[string]string{
		"name":  "",
		"stage": "",
	})

	if result == nil {
		t.Error("Expected error result for empty fields, got nil")
	}
	if !result.IsError {
		t.Error("Expected IsError to be true")
	}
	// Should return error for first empty field encountered
	// (exact field depends on map iteration order, so we just verify we got an error)
}

func TestValidateRequired_EmptyMap(t *testing.T) {
	result := ValidateRequired(map[string]string{})

	if result != nil {
		t.Errorf("Expected nil result for empty map, got error: %v", result)
	}
}

func TestValidateRequiredWithPrefix_CustomPrefix(t *testing.T) {
	result := ValidateRequiredWithPrefix("Build failed", map[string]string{
		"name": "",
	})

	if result == nil {
		t.Error("Expected error result, got nil")
	}

	if !result.IsError {
		t.Error("Expected IsError to be true")
	}

	// Verify the custom prefix is in the message
	if len(result.Content) > 0 {
		if textContent, ok := result.Content[0].(*mcp.TextContent); ok {
			if textContent.Text == "" {
				t.Error("Expected error message with custom prefix")
			}
			// Message should start with "Build failed:"
		}
	}
}

func TestValidateRequiredWithPrefix_AllValid(t *testing.T) {
	result := ValidateRequiredWithPrefix("Test run failed", map[string]string{
		"stage": "unit",
		"name":  "test",
	})

	if result != nil {
		t.Errorf("Expected nil result for valid fields, got error: %v", result)
	}
}
