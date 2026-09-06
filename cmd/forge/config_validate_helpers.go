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

package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/alexandremahdhaoui/forge/pkg/forge"
	"github.com/alexandremahdhaoui/forge/pkg/mcptypes"
)

// engineReference represents a reference to an engine extracted from forge.Spec.
// It captures the URI, the type of spec (build/test/testenv), the spec name,
// and the spec configuration to pass to the engine for validation.
type engineReference struct {
	// URI is the engine URI (e.g., "forge://go-build", "alias://my-engine")
	URI string

	// SpecType is the type of spec this reference came from: "build", "test", or "testenv"
	SpecType string

	// SpecName is the name of the build/test spec this reference came from
	SpecName string

	// Platforms is what a build entry declares it builds for; empty for a
	// test or testenv reference.
	Platforms []string

	// Spec is the engine-specific configuration to pass for validation.
	// For build engines: BuildSpec.Spec
	// For test runners: TestSpec.Spec
	// For testenv: nil (testenv gets forgeSpec instead)
	Spec map[string]interface{}

	// Src is the build entry's source directory, handed to the engine as
	// DirectoryParams.RootDir. An engine that validates files under the
	// entry's directory - forge-dev reads forge-dev.yaml and the OpenAPI
	// spec there - refused every entry without it, because the build passes
	// the directory and the validation never did.
	Src string
}

// extractEngineURIs extracts all unique engine URIs from a forge.Spec.
// It iterates over build specs and test specs, extracting engine references
// for build engines, test runners, and testenv orchestrators.
// Returns a deduplicated list of engine references (by URI).
func extractEngineURIs(spec forge.Spec) []engineReference {
	// Use a map to track seen URIs for deduplication
	seen := make(map[string]bool)
	var refs []engineReference

	// Extract from build specs. The key is engine plus source directory, not
	// engine alone: one engine can serve many entries - every forge-dev cell
	// names forge://forge-dev - and deduplicating on the URI validated the
	// first cell and silently skipped the rest.
	for _, bs := range spec.Build {
		key := bs.Engine + "\x00" + bs.Src
		if bs.Engine != "" && !seen[key] {
			seen[key] = true
			refs = append(refs, engineReference{
				URI:       bs.Engine,
				SpecType:  "build",
				SpecName:  bs.Name,
				Spec:      bs.Spec,
				Src:       bs.Src,
				Platforms: bs.Platforms,
			})
		}
	}

	// Extract from test specs
	for _, ts := range spec.Test {
		// Extract runner URI
		if ts.Runner != "" && !seen[ts.Runner] {
			seen[ts.Runner] = true
			refs = append(refs, engineReference{
				URI:      ts.Runner,
				SpecType: "test",
				SpecName: ts.Name,
				Spec:     ts.Spec,
			})
		}

		// Extract testenv URI if set and not "noop" or empty
		if ts.Testenv != "" && ts.Testenv != "noop" && !seen[ts.Testenv] {
			seen[ts.Testenv] = true
			refs = append(refs, engineReference{
				URI:      ts.Testenv,
				SpecType: "testenv",
				SpecName: ts.Name,
				Spec:     nil, // testenv gets forgeSpec instead
			})
		}
	}

	return refs
}

// validateEngineSpec validates a single engine's spec by calling its config-validate MCP tool.
// It parses the engine URI, resolves aliases, prepares the ConfigValidateInput, calls the engine,
// and parses the result. If the MCP call fails, it returns a ConfigValidateOutput with InfraError set.
//
// Parameters:
//   - ctx: context for the MCP call
//   - ref: the engine reference containing URI, SpecType, SpecName, and Spec
//   - forgeSpec: the complete forge.Spec (for testenv orchestrators that need access to kindenv, etc.)
//   - configPath: the path to forge.yaml (for error messages)
//
// Returns:
//   - *mcptypes.ConfigValidateOutput: the validation result from the engine
//   - error: only if there's a programming error (not validation failures)
func validateEngineSpec(ctx context.Context, ref engineReference, forgeSpec *forge.Spec, configPath string) (*mcptypes.ConfigValidateOutput, error) {
	// Resolve the engine URI to command and args
	// This handles both forge:// and alias:// URIs
	command, args, err := resolveEngine(ref.URI, forgeSpec)
	if err != nil {
		// Return as InfraError - we couldn't even resolve the engine
		return &mcptypes.ConfigValidateOutput{
			Valid:      false,
			InfraError: fmt.Sprintf("failed to resolve engine %s: %v", ref.URI, err),
		}, nil
	}

	// Prepare the ConfigValidateInput. The entry's source directory rides
	// as RootDir exactly as a build passes it, so an engine that validates
	// files under the entry - forge-dev and its cells - sees the same
	// directory the build would hand it.
	// The entry's declared platforms ride along, so a build engine refuses
	// here, before anything builds, a platform outside what it declares.
	input := mcptypes.ConfigValidateInput{
		Spec:       ref.Spec,
		ForgeSpec:  forgeSpec,
		ConfigPath: configPath,
		SpecType:   ref.SpecType,
		SpecName:   ref.SpecName,
		Platforms:  ref.Platforms,
	}

	if ref.Src != "" {
		input.DirectoryParams = &mcptypes.DirectoryParams{RootDir: ref.Src}
	}

	// Convert ConfigValidateInput to map[string]any for callMCPEngine
	// We use JSON marshal/unmarshal to do the conversion
	inputBytes, err := json.Marshal(input)
	if err != nil {
		return &mcptypes.ConfigValidateOutput{
			Valid:      false,
			InfraError: fmt.Sprintf("failed to marshal config-validate input for engine %s: %v", ref.URI, err),
		}, nil
	}

	var params map[string]any
	if err := json.Unmarshal(inputBytes, &params); err != nil {
		return &mcptypes.ConfigValidateOutput{
			Valid:      false,
			InfraError: fmt.Sprintf("failed to convert config-validate input for engine %s: %v", ref.URI, err),
		}, nil
	}

	// Call the engine's config-validate tool
	result, err := callMCPEngine(command, args, "config-validate", params)
	if err != nil {
		// MCP call failed - return as InfraError
		return &mcptypes.ConfigValidateOutput{
			Valid:      false,
			InfraError: fmt.Sprintf("failed to call config-validate on engine %s: %v", ref.URI, err),
		}, nil
	}

	// Parse the result into ConfigValidateOutput
	output, err := parseConfigValidateOutput(result)
	if err != nil {
		return &mcptypes.ConfigValidateOutput{
			Valid:      false,
			InfraError: fmt.Sprintf("failed to parse config-validate output from engine %s: %v", ref.URI, err),
		}, nil
	}

	return output, nil
}

// parseConfigValidateOutput parses the MCP tool result into a ConfigValidateOutput.
// The result can be either a map[string]any or a struct that needs JSON conversion.
func parseConfigValidateOutput(result interface{}) (*mcptypes.ConfigValidateOutput, error) {
	if result == nil {
		// No result - assume valid (engine may not have implemented config-validate)
		return &mcptypes.ConfigValidateOutput{
			Valid: true,
		}, nil
	}

	// Convert result to JSON and back to ConfigValidateOutput
	resultBytes, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	var output mcptypes.ConfigValidateOutput
	if err := json.Unmarshal(resultBytes, &output); err != nil {
		return nil, fmt.Errorf("failed to unmarshal result: %w", err)
	}

	return &output, nil
}

// validationResult pairs an engine reference with its validation output.
// Used by aggregateResults to combine results from multiple engines.
type validationResult struct {
	Ref    engineReference
	Output *mcptypes.ConfigValidateOutput
}

// aggregateResults combines validation results from multiple engines into a single output.
// It handles infrastructure errors, validation errors, and warnings from all engines.
//
// The aggregation logic:
//  1. Initializes combined output with Valid=true, empty Errors/Warnings
//  2. For each result:
//     a. If output.InfraError is set: creates ValidationError with Field="", Message=InfraError, Engine=ref.URI
//     b. If output.Valid is false: sets Engine, SpecType, SpecName, and Path on each error
//     c. Collects all warnings from all engines
//  3. Returns combined ConfigValidateOutput
//
// Parameters:
//   - results: slice of validationResult containing engine references and their outputs
//
// Returns:
//   - *mcptypes.ConfigValidateOutput: the combined validation result
func aggregateResults(results []validationResult) *mcptypes.ConfigValidateOutput {
	combined := &mcptypes.ConfigValidateOutput{
		Valid:    true,
		Errors:   []mcptypes.ValidationError{},
		Warnings: []mcptypes.ValidationWarning{},
	}

	for _, r := range results {
		if r.Output == nil {
			continue
		}

		// Build path context based on spec type
		// This helps users locate errors in forge.yaml
		basePath := buildPathContext(r.Ref)

		// Handle infrastructure errors
		if r.Output.InfraError != "" {
			combined.Valid = false
			combined.Errors = append(combined.Errors, mcptypes.ValidationError{
				Field:    "",
				Message:  r.Output.InfraError,
				Engine:   r.Ref.URI,
				SpecType: r.Ref.SpecType,
				SpecName: r.Ref.SpecName,
				Path:     basePath,
			})
		}

		// Handle validation errors
		if !r.Output.Valid {
			combined.Valid = false
			for _, err := range r.Output.Errors {
				// Set engine context if not already set
				if err.Engine == "" {
					err.Engine = r.Ref.URI
				}
				// Set spec context
				err.SpecType = r.Ref.SpecType
				err.SpecName = r.Ref.SpecName
				// Prepend path context if not already set
				if len(err.Path) == 0 {
					err.Path = basePath
				}
				combined.Errors = append(combined.Errors, err)
			}
		}

		// Collect all warnings
		combined.Warnings = append(combined.Warnings, r.Output.Warnings...)
	}

	return combined
}

// buildPathContext creates a path slice for an engine reference.
// This helps locate errors in the forge.yaml structure.
func buildPathContext(ref engineReference) []string {
	switch ref.SpecType {
	case "build":
		return []string{"build", ref.SpecName}
	case "test":
		return []string{"test", ref.SpecName}
	case "testenv":
		return []string{"test", ref.SpecName, "testenv"}
	case "testenv-subengine":
		// For testenv subengines, the path is more complex
		// The SpecName might be like "integration[2]" indicating the subengine index
		return []string{"testenv", ref.SpecName}
	default:
		if ref.SpecName != "" {
			return []string{ref.SpecType, ref.SpecName}
		}
		return nil
	}
}
