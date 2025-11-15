package main

import (
	"context"
	"fmt"
	"log"

	"github.com/alexandremahdhaoui/forge/internal/mcpserver"
	"github.com/alexandremahdhaoui/forge/pkg/mcptypes"
	"github.com/alexandremahdhaoui/forge/pkg/mcputil"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// runMCPServerImpl starts the go-test MCP server with stdio transport.
// It creates an MCP server, registers tools, and runs the server until stdin closes.
func runMCPServerImpl() error {
	v, _, _ := versionInfo.Get()
	server := mcpserver.New("go-test", v)

	// Register run tool
	mcpserver.RegisterTool(server, &mcp.Tool{
		Name:        "run",
		Description: "Run tests for a given stage and generate structured report",
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
	log.Printf("Running tests: stage=%s name=%s", input.Stage, input.Name)

	// Validate inputs
	if result := mcputil.ValidateRequiredWithPrefix("Test run failed", map[string]string{
		"stage": input.Stage,
		"name":  input.Name,
	}); result != nil {
		return result, nil, nil
	}

	// Run tests (pass tmpDir if provided, otherwise use current directory)
	tmpDir := input.TmpDir
	if tmpDir == "" {
		tmpDir = "." // Fallback to current directory for backward compatibility
	}

	// Pass testenv information to tests via environment variables
	testEnv := make(map[string]string)
	if input.TestenvTmpDir != "" {
		testEnv["FORGE_TESTENV_TMPDIR"] = input.TestenvTmpDir
	}
	if len(input.ArtifactFiles) > 0 {
		// Pass each artifact file as an environment variable
		for key, relPath := range input.ArtifactFiles {
			// Construct absolute path if testenvTmpDir is available
			var absPath string
			if input.TestenvTmpDir != "" {
				absPath = fmt.Sprintf("%s/%s", input.TestenvTmpDir, relPath)
			} else {
				absPath = relPath
			}
			// Convert key to env var name (e.g., "testenv-kind.kubeconfig" -> "FORGE_ARTIFACT_TESTENV_KIND_KUBECONFIG")
			envKey := fmt.Sprintf("FORGE_ARTIFACT_%s", normalizeEnvKey(key))
			testEnv[envKey] = absPath
		}
	}
	if len(input.TestenvMetadata) > 0 {
		// Pass metadata as environment variables
		for key, value := range input.TestenvMetadata {
			envKey := fmt.Sprintf("FORGE_METADATA_%s", normalizeEnvKey(key))
			testEnv[envKey] = value
		}
	}

	report, junitFile, coverageFile, err := runTests(input.Stage, input.Name, tmpDir, testEnv)
	if err != nil {
		return mcputil.ErrorResult(fmt.Sprintf("Test run failed: %v", err)), nil, nil
	}

	// Store report in artifact store
	if err := storeTestReport(report, junitFile, coverageFile); err != nil {
		// Log warning but don't fail
		log.Printf("Warning: failed to store test report: %v", err)
	}

	// Create success message
	statusMsg := fmt.Sprintf("Tests %s: stage=%s, total=%d, passed=%d, failed=%d, skipped=%d, coverage=%.1f%%",
		report.Status,
		report.Stage,
		report.TestStats.Total,
		report.TestStats.Passed,
		report.TestStats.Failed,
		report.TestStats.Skipped,
		report.Coverage.Percentage,
	)

	// Return result with TestReport as artifact based on status
	if report.Status == "failed" {
		result := mcputil.ErrorResult(statusMsg)
		return result, report, nil
	}
	result, returnedArtifact := mcputil.SuccessResultWithArtifact(statusMsg, report)
	return result, returnedArtifact, nil
}

// normalizeEnvKey converts a key to an environment variable friendly format.
// Example: "testenv-kind.kubeconfig" -> "TESTENV_KIND_KUBECONFIG"
func normalizeEnvKey(key string) string {
	result := ""
	for i := 0; i < len(key); i++ {
		c := key[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			if c >= 'a' && c <= 'z' {
				result += string(c - 32) // Convert to uppercase
			} else {
				result += string(c)
			}
		} else {
			result += "_"
		}
	}
	return result
}
