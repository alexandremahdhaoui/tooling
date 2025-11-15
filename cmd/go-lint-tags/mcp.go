package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/alexandremahdhaoui/forge/internal/mcpserver"
	"github.com/alexandremahdhaoui/forge/pkg/forge"
	"github.com/alexandremahdhaoui/forge/pkg/mcptypes"
	"github.com/alexandremahdhaoui/forge/pkg/mcputil"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// runMCPServerImpl starts the test-runner-go-verify-tags MCP server with stdio transport.
// It creates an MCP server, registers tools, and runs the server until stdin closes.
func runMCPServerImpl() error {
	v, _, _ := versionInfo.Get()
	server := mcpserver.New("test-runner-go-verify-tags", v)

	// Register run tool
	mcpserver.RegisterTool(server, &mcp.Tool{
		Name:        "run",
		Description: "Verify that all test files have valid build tags (unit, integration, or e2e)",
	}, handleRunTool)

	// Run the MCP server
	return server.RunDefault()
}

// handleRunTool handles the "run" tool call from MCP clients.
func handleRunTool(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input mcptypes.RunInput,
) (*mcp.CallToolResult, any, error) {
	log.Printf("Create called (no-op): stage=%s", input.Stage)

	startTime := time.Now()

	// Default root directory
	rootDir := "."
	if input.RootDir != "" {
		rootDir = input.RootDir
	}

	// Run verification
	filesWithoutTags, totalFiles, err := verifyTags(rootDir)
	duration := time.Since(startTime).Seconds()

	// Generate report ID
	reportID := uuid.New().String()

	// Build base report
	report := &forge.TestReport{
		ID:        reportID,
		Stage:     input.Stage,
		StartTime: startTime,
		Duration:  duration,
		TestStats: forge.TestStats{
			Total:   totalFiles,
			Passed:  totalFiles - len(filesWithoutTags),
			Failed:  len(filesWithoutTags),
			Skipped: 0,
		},
		Coverage: forge.Coverage{
			Percentage: 0, // No coverage for verify-tags
		},
	}

	if err != nil {
		report.Status = "failed"
		report.ErrorMessage = fmt.Sprintf("Verification failed: %v", err)
		report.TestStats = forge.TestStats{Total: 0, Passed: 0, Failed: 0, Skipped: 0}

		result := mcputil.ErrorResult(report.ErrorMessage)
		return result, report, nil
	}

	if len(filesWithoutTags) > 0 {
		report.Status = "failed"

		// Build detailed error message
		var details strings.Builder
		details.WriteString(fmt.Sprintf("Found %d test file(s) without build tags out of %d total files", len(filesWithoutTags), totalFiles))
		details.WriteString("\n\nFiles missing build tags:\n")
		for _, file := range filesWithoutTags {
			details.WriteString(fmt.Sprintf("  - %s\n", file))
		}
		details.WriteString("\nTest files must have one of these build tags:\n")
		details.WriteString("  //go:build unit\n")
		details.WriteString("  //go:build integration\n")
		details.WriteString("  //go:build e2e\n")

		report.ErrorMessage = details.String()

		result := mcputil.ErrorResult(details.String())
		return result, report, nil
	}

	report.Status = "passed"
	successMsg := fmt.Sprintf("✅ All %d test files have valid build tags", totalFiles)

	result, returnedArtifact := mcputil.SuccessResultWithArtifact(successMsg, report)
	return result, returnedArtifact, nil
}
