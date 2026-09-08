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

package engineframework

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/alexandremahdhaoui/forge/internal/engineresolver"
	"github.com/alexandremahdhaoui/forge/internal/forgepath"
	"github.com/alexandremahdhaoui/forge/pkg/forge"
	"github.com/alexandremahdhaoui/forge/pkg/mcptypes"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ResolveDetector parses a detector URI and returns the command and args to execute it.
// Detectors only support forge:// URIs.
//
// Parameters:
//   - detectorURI: URI of the detector (e.g., "forge://go-dependency-detector")
//   - forgeVersion: Version of forge to use (e.g., "v0.9.0")
//
// Returns:
//   - cmd: The command to execute (always "go")
//   - args: Arguments for the command (e.g., ["run", "github.com/.../cmd/detector@v0.9.0"])
//   - err: Error if the URI is invalid or resolution fails
//
// Example usage:
//
//	cmd, args, err := ResolveDetector("forge://go-dependency-detector", "v0.9.0")
//	// cmd = "go"
//	// args = ["run", "github.com/alexandremahdhaoui/forge/cmd/go-dependency-detector@v0.9.0"]
func ResolveDetector(detectorURI, forgeVersion string) (cmd string, args []string, err error) {
	if strings.HasPrefix(detectorURI, "go://") {
		return "", nil, fmt.Errorf("%s: %s", detectorURI, engineresolver.GoSchemeError)
	}

	// Validate URI starts with forge://
	if !strings.HasPrefix(detectorURI, "forge://") {
		return "", nil, fmt.Errorf("unsupported detector protocol: %s (must start with forge://)", detectorURI)
	}

	// Extract detector name from URI
	detectorName := strings.TrimPrefix(detectorURI, "forge://")
	if detectorName == "" {
		return "", nil, fmt.Errorf("empty detector name after forge://")
	}

	// Build the go run command using forgepath
	cmd, runArgs, err := forgepath.EngineCommand(detectorName, forgeVersion)
	if err != nil {
		return "", nil, fmt.Errorf("failed to resolve detector command for %s: %w", detectorName, err)
	}

	return cmd, runArgs, nil
}

// FindDetector locates a dependency detector binary by name.
// It searches in the following order:
//  1. PATH environment variable
//  2. ./build/bin directory (common for forge self-build)
//
// Returns the absolute path to the binary or an error if not found.
//
// Deprecated: FindDetector only works when CWD is the forge repository.
// Use ResolveDetector() + CallDetector() instead, which works from any directory
// by using `go run` with versioned module paths.
func FindDetector(name string) (string, error) {
	// Try to find in PATH
	path, err := exec.LookPath(name)
	if err == nil {
		return path, nil
	}

	// Try in build directory (common for forge self-build)
	buildPath := filepath.Join(".", "build", "bin", name)
	if _, err := os.Stat(buildPath); err == nil {
		absPath, err := filepath.Abs(buildPath)
		if err != nil {
			return "", fmt.Errorf("found detector at %s but failed to resolve absolute path: %w", buildPath, err)
		}
		return absPath, nil
	}

	return "", fmt.Errorf("%s not found in PATH or ./build/bin", name)
}

// CallDetector calls a detector MCP server and returns dependencies.
// It spawns the detector as a subprocess, connects via MCP, calls the specified tool,
// and converts the response to []forge.ArtifactDependency.
//
// Parameters:
//   - ctx: context for the operation
//   - cmd: command to execute (e.g., "go")
//   - args: arguments for the command (e.g., ["run", "github.com/.../cmd/detector@v0.9.0"])
//   - toolName: name of the MCP tool to call (e.g., "detectDependencies")
//   - input: input parameters for the tool (will be serialized to JSON)
//
// Returns:
//   - []forge.ArtifactDependency: list of detected dependencies
//   - error: if connection fails, tool call fails, or response parsing fails
func CallDetector(ctx context.Context, cmd string, args []string, toolName string, input any) ([]forge.ArtifactDependency, error) {
	// Create command to spawn MCP server (append --mcp flag)
	execCmd := exec.Command(cmd, append(args, "--mcp")...)
	execCmd.Env = os.Environ()
	execCmd.Stderr = os.Stderr // Forward logs

	// Create MCP client
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "detector-client",
		Version: "v1.0.0",
	}, nil)

	// Create command transport
	transport := &mcp.CommandTransport{
		Command: execCmd,
	}

	// Connect to the MCP server
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to detector: %w", err)
	}
	defer func() { _ = session.Close() }()

	// Call the tool
	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      toolName,
		Arguments: input,
	})
	if err != nil {
		return nil, fmt.Errorf("MCP tool call failed: %w", err)
	}

	// Check if result indicates an error
	if result.IsError {
		errMsg := "unknown error"
		if len(result.Content) > 0 {
			if textContent, ok := result.Content[0].(*mcp.TextContent); ok {
				errMsg = textContent.Text
			}
		}
		return nil, fmt.Errorf("detection failed: %s", errMsg)
	}

	// Parse structured content
	if result.StructuredContent == nil {
		return nil, fmt.Errorf("no structured content returned from detector")
	}

	// Convert structured content to DetectDependenciesOutput
	var output mcptypes.DetectDependenciesOutput
	jsonBytes, err := json.Marshal(result.StructuredContent)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal detector output: %w", err)
	}

	if err := json.Unmarshal(jsonBytes, &output); err != nil {
		return nil, fmt.Errorf("failed to unmarshal detector output: %w", err)
	}

	// Convert mcptypes.Dependency to forge.ArtifactDependency
	artifactDeps := make([]forge.ArtifactDependency, len(output.Dependencies))
	for i, dep := range output.Dependencies {
		artifactDeps[i] = forge.ArtifactDependency{Path: dep.Path, Digest: dep.Digest}
	}

	return artifactDeps, nil
}
