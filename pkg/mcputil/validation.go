package mcputil

import (
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ValidateRequired checks if required fields are present and returns an MCP error result if not.
// Returns nil if all fields are valid.
//
// Parameters:
//   - fields: map of field name to field value
//
// Example usage:
//
//	if result := mcputil.ValidateRequired(map[string]string{
//	    "name": input.Name,
//	    "stage": input.Stage,
//	}); result != nil {
//	    return result, nil, nil
//	}
func ValidateRequired(fields map[string]string) *mcp.CallToolResult {
	for fieldName, fieldValue := range fields {
		if fieldValue == "" {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: fmt.Sprintf("Operation failed: missing required field '%s'", fieldName)},
				},
				IsError: true,
			}
		}
	}
	return nil
}

// ValidateRequiredWithPrefix checks required fields and uses a custom error prefix.
// This allows customizing the error message (e.g., "Build failed:" vs "Test run failed:").
//
// Parameters:
//   - prefix: error message prefix (e.g., "Build failed", "Test run failed")
//   - fields: map of field name to field value
//
// Example usage:
//
//	if result := mcputil.ValidateRequiredWithPrefix("Build failed", map[string]string{
//	    "name": input.Name,
//	    "src": input.Src,
//	}); result != nil {
//	    return result, nil, nil
//	}
func ValidateRequiredWithPrefix(prefix string, fields map[string]string) *mcp.CallToolResult {
	for fieldName, fieldValue := range fields {
		if fieldValue == "" {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: fmt.Sprintf("%s: missing required field '%s'", prefix, fieldName)},
				},
				IsError: true,
			}
		}
	}
	return nil
}
