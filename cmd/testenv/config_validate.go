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
	"log"
	"strings"

	"github.com/alexandremahdhaoui/forge/pkg/forge"
	"github.com/alexandremahdhaoui/forge/pkg/mcptypes"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// handleConfigValidate handles the config-validate MCP tool for the testenv orchestrator.
// It performs recursive validation by:
// 1. Validating its own spec structure (checking that subengines have engine fields)
// 2. Extracting subengine URIs from the spec
// 3. For each subengine, determining config from forgeSpec using the mapping:
//   - forge://testenv-kind        -> forgeSpec.Kindenv
//   - forge://testenv-lcr         -> forgeSpec.LocalContainerRegistry
//   - forge://testenv-helm-install -> subengine.spec (passed directly)
//   - alias://...              -> Resolved from forgeSpec.Engines[]
//
// 4. Calling each subengine's config-validate tool
// 5. Aggregating all results
func handleConfigValidate(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input mcptypes.ConfigValidateInput,
) (*mcp.CallToolResult, any, error) {
	log.Printf("testenv: validating configuration")

	output := validateTestenvSpec(ctx, input)

	// Return as structured MCP result
	// Note: We don't set IsError=true for validation failures.
	// The Valid field in output indicates whether validation passed.
	// IsError should only be used for infrastructure failures.
	msg := "Configuration is valid"
	if !output.Valid {
		msg = fmt.Sprintf("Configuration validation failed with %d error(s)", len(output.Errors))
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: msg},
		},
	}, output, nil
}

// validateTestenvSpec performs the recursive validation of the testenv spec.
func validateTestenvSpec(ctx context.Context, input mcptypes.ConfigValidateInput) *mcptypes.ConfigValidateOutput {
	var errors []mcptypes.ValidationError
	var warnings []mcptypes.ValidationWarning

	// If forgeSpec is nil, we can't do recursive validation
	if input.ForgeSpec == nil {
		// We can still validate the spec structure
		log.Printf("testenv: forgeSpec is nil, performing basic validation only")
	}

	// Step 1: Validate own spec structure
	// The testenv orchestrator itself doesn't have specific spec fields to validate
	// when called directly via forge://testenv. It's an orchestrator that reads subengines
	// from the engine alias config.

	// Step 2: Find the testenv engine config from forgeSpec
	// The testenv is typically referenced via alias://<alias>, and the alias contains
	// the list of subengines to orchestrate.
	//
	// However, when testenv receives config-validate, we need to know which alias was used.
	// The SpecName in input tells us which test stage, but we need to look up the testenv
	// configuration from the forge.Spec.

	// If we have forgeSpec, look up the testenv engine config for this stage
	var subengines []forge.TestenvEngineSpec

	if input.ForgeSpec != nil {
		subengines = extractSubenginesFromForgeSpec(input.ForgeSpec, input.SpecName)
	}

	// If no subengines found, check if spec contains direct subengine definitions
	if len(subengines) == 0 && len(input.Spec) > 0 {
		extracted, extractErr := extractSubenginesFromSpec(input.Spec)
		if extractErr != nil {
			errors = append(errors, *extractErr)
		} else {
			subengines = extracted
		}
	}

	// If we still have no subengines, that's OK for forge://testenv or forge://test-report
	// as they might be used directly without orchestration
	if len(subengines) == 0 {
		log.Printf("testenv: no subengines to validate")
		return &mcptypes.ConfigValidateOutput{
			Valid:    len(errors) == 0,
			Errors:   errors,
			Warnings: warnings,
		}
	}

	// Step 3 & 4: For each subengine, determine config and call config-validate
	results := make([]validationResult, 0, len(subengines))

	for i, subengine := range subengines {
		// Validate that each subengine has an engine field
		if subengine.Engine == "" {
			errors = append(errors, mcptypes.ValidationError{
				Field:   fmt.Sprintf("testenv[%d].engine", i),
				Message: "subengine must have an engine field",
			})
			continue
		}

		// Determine the spec to pass to the subengine
		subengineSpec := getSubengineConfig(subengine.Engine, subengine.Spec, input.ForgeSpec)

		// Resolve the engine URI to command and args
		engine, err := resolveEngineURI(subengine.Engine)
		if err != nil {
			results = append(results, validationResult{
				Ref: engineReference{
					URI:      subengine.Engine,
					SpecType: "testenv-subengine",
					SpecName: fmt.Sprintf("%s[%d]", input.SpecName, i),
				},
				Output: &mcptypes.ConfigValidateOutput{
					Valid:      false,
					InfraError: fmt.Sprintf("failed to resolve engine %s: %v", subengine.Engine, err),
				},
			})
			continue
		}

		// Prepare ConfigValidateInput for the subengine
		subInput := mcptypes.ConfigValidateInput{
			Spec:       subengineSpec,
			ForgeSpec:  input.ForgeSpec,
			ConfigPath: input.ConfigPath,
			SpecType:   "testenv-subengine",
			SpecName:   fmt.Sprintf("%s[%d]", input.SpecName, i),
		}

		// Convert to params map for MCP call
		params := configValidateInputToParams(subInput)

		// Call the subengine's config-validate tool
		result, err := callMCPEngine(engine, "config-validate", params)
		if err != nil {
			results = append(results, validationResult{
				Ref: engineReference{
					URI:      subengine.Engine,
					SpecType: "testenv-subengine",
					SpecName: fmt.Sprintf("%s[%d]", input.SpecName, i),
				},
				Output: &mcptypes.ConfigValidateOutput{
					Valid:      false,
					InfraError: fmt.Sprintf("failed to call config-validate on engine %s: %v", subengine.Engine, err),
				},
			})
			continue
		}

		// Parse the result
		output, err := parseConfigValidateOutput(result)
		if err != nil {
			results = append(results, validationResult{
				Ref: engineReference{
					URI:      subengine.Engine,
					SpecType: "testenv-subengine",
					SpecName: fmt.Sprintf("%s[%d]", input.SpecName, i),
				},
				Output: &mcptypes.ConfigValidateOutput{
					Valid:      false,
					InfraError: fmt.Sprintf("failed to parse config-validate output from engine %s: %v", subengine.Engine, err),
				},
			})
			continue
		}

		results = append(results, validationResult{
			Ref: engineReference{
				URI:      subengine.Engine,
				SpecType: "testenv-subengine",
				SpecName: fmt.Sprintf("%s[%d]", input.SpecName, i),
			},
			Output: output,
		})

		log.Printf("testenv: validated subengine %s: valid=%v", subengine.Engine, output.Valid)
	}

	// Step 5: Aggregate all results
	aggregated := aggregateResults(results)

	// Merge any errors we collected during own validation
	if len(errors) > 0 {
		aggregated.Valid = false
		aggregated.Errors = append(errors, aggregated.Errors...)
	}

	// Merge warnings
	aggregated.Warnings = append(warnings, aggregated.Warnings...)

	return aggregated
}

// extractSubenginesFromForgeSpec looks up the testenv configuration for a test stage.
// It finds the test spec by name, then looks up the alias to get subengines.
func extractSubenginesFromForgeSpec(forgeSpec *forge.Spec, stageName string) []forge.TestenvEngineSpec {
	if forgeSpec == nil {
		return nil
	}

	// Find the test spec for this stage
	var testSpec *forge.TestSpec
	for i := range forgeSpec.Test {
		if forgeSpec.Test[i].Name == stageName {
			testSpec = &forgeSpec.Test[i]
			break
		}
	}

	if testSpec == nil {
		return nil
	}

	// Check if testenv is an alias reference
	testenvURI := testSpec.Testenv
	if testenvURI == "" {
		return nil
	}

	// Handle alias:// references
	if strings.HasPrefix(testenvURI, "alias://") {
		alias := strings.TrimPrefix(testenvURI, "alias://")
		for i := range forgeSpec.Engines {
			if forgeSpec.Engines[i].Alias == alias && forgeSpec.Engines[i].Type == forge.TestenvEngineConfigType {
				return forgeSpec.Engines[i].Testenv
			}
		}
	}

	// For direct forge:// references (like forge://testenv), there are no subengines
	// The caller is using testenv directly without orchestration
	return nil
}

// extractSubenginesFromSpec extracts subengines from the input spec map.
// This handles the case where subengines are passed directly in the spec.
func extractSubenginesFromSpec(spec map[string]interface{}) ([]forge.TestenvEngineSpec, *mcptypes.ValidationError) {
	subenginesRaw, ok := spec["subengines"]
	if !ok {
		return nil, nil
	}

	// Try to convert to []interface{}
	subenginesList, ok := subenginesRaw.([]interface{})
	if !ok {
		return nil, &mcptypes.ValidationError{
			Field:   "spec.subengines",
			Message: fmt.Sprintf("expected array, got %T", subenginesRaw),
		}
	}

	subengines := make([]forge.TestenvEngineSpec, 0, len(subenginesList))
	for i, item := range subenginesList {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			return nil, &mcptypes.ValidationError{
				Field:   fmt.Sprintf("spec.subengines[%d]", i),
				Message: fmt.Sprintf("expected object, got %T", item),
			}
		}

		// Extract engine field
		engine, _ := itemMap["engine"].(string)

		// Extract spec field
		var subSpec map[string]interface{}
		if specRaw, ok := itemMap["spec"]; ok {
			if specMap, ok := specRaw.(map[string]interface{}); ok {
				subSpec = specMap
			}
		}

		// Extract deferTemplates field
		deferTemplates, _ := itemMap["deferTemplates"].(bool)

		subengines = append(subengines, forge.TestenvEngineSpec{
			Engine:         engine,
			Spec:           subSpec,
			DeferTemplates: deferTemplates,
		})
	}

	return subengines, nil
}

// getSubengineConfig answers the spec a subengine is validated against: the
// spec on its own entry, for every engine alike. Two engines used to be
// handed a top-level forge.yaml key instead, so `config validate` checked
// data the engine never ran with while the entry's own spec went unchecked.
func getSubengineConfig(engineURI string, subengineSpec map[string]interface{}, forgeSpec *forge.Spec) map[string]interface{} {
	// If forgeSpec is nil, just return the subengine spec
	if forgeSpec == nil {
		return subengineSpec
	}

	switch {
	case engineURI == "forge://testenv-helm-install":
		// Return subengine spec directly (contains helm charts config)
		return subengineSpec

	case strings.HasPrefix(engineURI, "alias://"):
		// Resolve alias and return its spec
		alias := strings.TrimPrefix(engineURI, "alias://")
		for _, ec := range forgeSpec.Engines {
			if ec.Alias == alias {
				// For testenv aliases, we still pass the subengine spec
				return subengineSpec
			}
		}
		return subengineSpec

	default:
		// For other engines (forge://test-report, etc.), pass subengine spec directly
		return subengineSpec
	}
}

// configValidateInputToParams converts ConfigValidateInput to map[string]any for MCP calls.
func configValidateInputToParams(input mcptypes.ConfigValidateInput) map[string]any {
	data, err := json.Marshal(input)
	if err != nil {
		return nil
	}

	var params map[string]any
	if err := json.Unmarshal(data, &params); err != nil {
		return nil
	}

	return params
}

// parseConfigValidateOutput parses the MCP tool result into a ConfigValidateOutput.
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

// engineReference represents a reference to an engine for validation result tracking.
type engineReference struct {
	URI      string
	SpecType string
	SpecName string
}

// validationResult pairs an engine reference with its validation output.
type validationResult struct {
	Ref    engineReference
	Output *mcptypes.ConfigValidateOutput
}

// aggregateResults combines validation results from multiple subengines into a single output.
// It adds path context to help locate errors in the forge.yaml structure.
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

		// Build path context for this subengine
		// SpecName is like "integration[2]" indicating stage and subengine index
		basePath := []string{"testenv", r.Ref.SpecName, "spec"}

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
