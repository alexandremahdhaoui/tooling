package mcputil

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ErrorResult creates a standardized MCP error result.
//
// Parameters:
//   - message: error message to display
//
// Example usage:
//
//	return mcputil.ErrorResult(fmt.Sprintf("Build failed: %v", err)), nil, nil
func ErrorResult(message string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: message},
		},
		IsError: true,
	}
}

// SuccessResult creates a standardized MCP success result.
//
// Parameters:
//   - message: success message to display
//
// Example usage:
//
//	return mcputil.SuccessResult("Build completed successfully"), nil, nil
func SuccessResult(message string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: message},
		},
		IsError: false,
	}
}

// SuccessResultWithArtifact creates a success result that returns an artifact.
// This is the most common pattern for MCP tool responses.
//
// Parameters:
//   - message: success message to display
//   - artifact: the artifact to return (typically forge.Artifact or similar)
//
// Returns:
//   - result: the MCP CallToolResult
//   - artifact: the artifact (passed through for MCP handler return)
//
// Example usage:
//
//	result, artifact := mcputil.SuccessResultWithArtifact("Built successfully", myArtifact)
//	return result, artifact, nil
func SuccessResultWithArtifact(message string, artifact any) (*mcp.CallToolResult, any) {
	result := &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: message},
		},
		IsError: false,
	}
	return result, artifact
}
