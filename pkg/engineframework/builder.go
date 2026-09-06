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
	"fmt"
	"log"

	"github.com/alexandremahdhaoui/forge/pkg/forge"
	"github.com/alexandremahdhaoui/forge/pkg/mcpserver"
	"github.com/alexandremahdhaoui/forge/pkg/mcptypes"
	"github.com/alexandremahdhaoui/forge/pkg/mcputil"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// BuilderFunc is the signature for build operations.
//
// Implementations must:
//   - Validate input fields (required fields should be checked, use RequireString etc. from spec.go)
//   - Execute the build operation (compile code, generate files, etc.)
//   - Return Artifact on success (with Name, Type, Location, Version if applicable, Timestamp)
//   - Return error on failure (business logic errors, not MCP errors)
//
// The framework handles:
//   - MCP tool registration
//   - Batch operation support
//   - Result formatting
//   - Error conversion to MCP responses
//
// Example:
//
//	func myBuildFunc(ctx context.Context, input mcptypes.BuildInput) (*forge.Artifact, error) {
//	    // Extract and validate spec fields
//	    sourceFile, err := RequireString(input.Spec, "sourceFile")
//	    if err != nil {
//	        return nil, err
//	    }
//
//	    // Execute build logic
//	    if err := compileSources(sourceFile); err != nil {
//	        return nil, fmt.Errorf("compilation failed: %w", err)
//	    }
//
//	    // Return artifact
//	    return []forge.Artifact{*CreateArtifact(input.Name, forge.TypeBinary, "./build/bin/"+input.Name)}, nil
//	}
//
// A build answers a list: one artifact per platform in input.Platforms, or
// one artifact tied to no platform. The framework has already refused any
// platform outside the engine's declared Capabilities, so the function sees
// only what it can build.
type BuilderFunc func(ctx context.Context, input mcptypes.BuildInput) ([]forge.Artifact, error)

// BuilderConfig configures builder tool registration.
//
// Fields:
//   - Name: Engine name (e.g., "go-build", "container-build")
//   - Version: Engine version string (e.g., "1.0.0" or git commit hash)
//   - BuildFunc: The build implementation function
//   - Capabilities: what the engine declares it can build; empty means the
//     host platform only
//
// Example:
//
//	config := BuilderConfig{
//	    Name:      "my-builder",
//	    Version:   "1.0.0",
//	    BuildFunc: myBuildFunc,
//	}
type BuilderConfig struct {
	Name         string       // Engine name (e.g., "go-build")
	Version      string       // Engine version
	BuildFunc    BuilderFunc  // Build implementation
	Capabilities Capabilities // What the engine declares; empty is host only
}

// RegisterBuilderTools registers build and buildBatch tools with the MCP server.
//
// This function automatically:
//   - Registers "build" tool that calls the BuildFunc
//   - Registers "buildBatch" tool that handles multiple builds in parallel
//   - Validates required input fields (Name, Engine)
//   - Converts BuilderFunc errors to MCP error responses
//   - Formats successful results with artifact information
//   - Uses mcputil.HandleBatchBuild for batch processing
//
// Parameters:
//   - server: The MCP server instance
//   - config: Builder configuration with Name, Version, and BuildFunc
//
// Returns:
//   - nil on success
//   - error if tool registration fails (e.g., duplicate tool names)
//
// Example:
//
//	func runMCPServer() error {
//	    server := mcpserver.New("my-builder", "1.0.0")
//
//	    config := BuilderConfig{
//	        Name:      "my-builder",
//	        Version:   "1.0.0",
//	        BuildFunc: myBuildFunc,
//	    }
//
//	    if err := RegisterBuilderTools(server, config); err != nil {
//	        return err
//	    }
//
//	    return server.RunDefault()
//	}
func RegisterBuilderTools(server *mcpserver.Server, config BuilderConfig) error {
	// Register build tool
	mcpserver.RegisterTool(server, &mcp.Tool{
		Name:        "build",
		Description: fmt.Sprintf("Build a single artifact using %s. Called by forge with parameters from forge.yaml build[] entries.", config.Name),
	}, makeBuildHandler(config))

	// Register buildBatch tool
	mcpserver.RegisterTool(server, &mcp.Tool{
		Name:        "buildBatch",
		Description: fmt.Sprintf("Build multiple artifacts in a single batch call using %s. Forge uses this when multiple forge.yaml build[] entries share the same engine.", config.Name),
	}, makeBatchBuildHandler(config))

	return nil
}

// makeBuildHandler creates an MCP handler function from a BuilderFunc.
//
// The returned handler:
//   - Validates required input fields (Name, Engine)
//   - Calls the BuilderFunc with the input
//   - Converts BuilderFunc errors to MCP error responses
//   - Formats successful results with artifact information
//
// This is an internal helper function used by RegisterBuilderTools.
func makeBuildHandler(config BuilderConfig) func(context.Context, *mcp.CallToolRequest, mcptypes.BuildInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input mcptypes.BuildInput) (*mcp.CallToolResult, any, error) {
		log.Printf("Building %s using %s", input.Name, config.Name)

		// Validate required input fields
		if result := mcputil.ValidateRequiredWithPrefix("Build failed", map[string]string{
			"name":   input.Name,
			"engine": input.Engine,
		}); result != nil {
			return result, nil, nil
		}

		// The platform contract, held before the engine's own code runs:
		// every requested platform is one this engine declared. An engine
		// handed a platform it cannot build answers a refusal, never a host
		// binary under a foreign name.
		if err := RefusePlatforms(config.Name, config.Capabilities, input.Platforms); err != nil {
			return mcputil.ErrorResult(fmt.Sprintf("Build failed: %v", err)), nil, nil
		}

		// Call the BuilderFunc
		artifacts, err := config.BuildFunc(ctx, input)
		if err != nil {
			return mcputil.ErrorResult(fmt.Sprintf("Build failed: %v", err)), nil, nil
		}

		for _, artifact := range artifacts {
			if err := artifact.Validate(); err != nil {
				return mcputil.ErrorResult(fmt.Sprintf("Build failed: %s answered an invalid artifact: %v", config.Name, err)), nil, nil
			}
		}

		// Return success with the artifacts, the same shape a batch answers.
		result, returned := mcputil.SuccessResultWithArtifact(
			fmt.Sprintf("Build succeeded: %s (%d artifact(s))", input.Name, len(artifacts)),
			mcptypes.BuildOutput{Artifacts: artifacts},
		)
		return result, returned, nil
	}
}

// makeBatchBuildHandler creates an MCP batch handler function from a BuilderFunc.
//
// The returned handler:
//   - Validates each build input
//   - Calls the BuilderFunc for each spec in parallel (via mcputil.HandleBatchBuild)
//   - Aggregates results and errors
//   - Formats batch result with all artifacts and error messages
//
// This is an internal helper function used by RegisterBuilderTools.
func makeBatchBuildHandler(config BuilderConfig) func(context.Context, *mcp.CallToolRequest, mcptypes.BatchBuildInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input mcptypes.BatchBuildInput) (*mcp.CallToolResult, any, error) {
		log.Printf("Building %d artifacts in batch using %s", len(input.Specs), config.Name)

		// Create single-build handler for batch processing
		singleBuildHandler := makeBuildHandler(config)

		// Use generic batch handler from mcputil
		outputs, errorMsgs := mcputil.HandleBatchBuild(ctx, input.Specs, func(ctx context.Context, spec mcptypes.BuildInput) (*mcp.CallToolResult, any, error) {
			return singleBuildHandler(ctx, req, spec)
		})

		// Each spec answered a list; the batch answers one flat list.
		artifacts := []any{}

		for _, out := range outputs {
			if output, ok := out.(mcptypes.BuildOutput); ok {
				for _, artifact := range output.Artifacts {
					artifacts = append(artifacts, artifact)
				}
			}
		}

		// Format the batch result
		result, returnedArtifacts := mcputil.FormatBatchResult("artifacts", artifacts, errorMsgs)
		return result, returnedArtifacts, nil
	}
}
