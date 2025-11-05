package mcputil

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// HandleBatchBuild is a generic handler for batch build operations.
// It takes a slice of build specs and a handler function for single builds.
// The handler should return (result, artifact, error) as per MCP conventions.
//
// Returns:
//   - artifacts: slice of successfully created artifacts
//   - errorMsgs: slice of error messages (one per failed spec)
//
// Example usage:
//
//	artifacts, errorMsgs := mcputil.HandleBatchBuild(ctx, specs,
//	    func(ctx context.Context, spec BuildInput) (*mcp.CallToolResult, any, error) {
//	        return handleBuildTool(ctx, req, spec)
//	    })
func HandleBatchBuild[T any](
	ctx context.Context,
	specs []T,
	handler func(context.Context, T) (*mcp.CallToolResult, any, error),
) (artifacts []any, errorMsgs []string) {
	artifacts = []any{}
	errorMsgs = []string{}

	for _, spec := range specs {
		result, artifact, err := handler(ctx, spec)

		// Check if the operation failed
		if err != nil || (result != nil && result.IsError) {
			errorMsg := extractErrorMessage(result, err)
			errorMsgs = append(errorMsgs, errorMsg)
			continue
		}

		// Collect successful artifact
		if artifact != nil {
			artifacts = append(artifacts, artifact)
		}
	}

	return artifacts, errorMsgs
}

// extractErrorMessage extracts a human-readable error message from MCP result or error.
func extractErrorMessage(result *mcp.CallToolResult, err error) string {
	if err != nil {
		return err.Error()
	}

	if result != nil && len(result.Content) > 0 {
		if textContent, ok := result.Content[0].(*mcp.TextContent); ok {
			return textContent.Text
		}
	}

	return "unknown error"
}

// FormatBatchResult creates an MCP result for batch operations.
// It returns an error result if there were any failures, otherwise a success result.
//
// Parameters:
//   - operationType: description of what was built (e.g., "binaries", "containers")
//   - artifacts: successful artifacts
//   - errorMsgs: error messages from failed operations
func FormatBatchResult(operationType string, artifacts []any, errorMsgs []string) (*mcp.CallToolResult, any) {
	if len(errorMsgs) > 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Batch build completed with errors: %v", errorMsgs)},
			},
			IsError: true,
		}, artifacts
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("Successfully built %d %s", len(artifacts), operationType)},
		},
	}, artifacts
}
