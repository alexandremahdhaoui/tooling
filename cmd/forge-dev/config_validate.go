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
	"fmt"
	"os"
	"path/filepath"

	"github.com/alexandremahdhaoui/forge/pkg/engineframework"
	"github.com/alexandremahdhaoui/forge/pkg/mcptypes"
	"github.com/alexandremahdhaoui/forge/pkg/mcputil"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// handleConfigValidate validates forge-dev configuration.
// It validates both forge-dev.yaml schema and the referenced spec.openapi.yaml.
//
// The input.Spec should contain:
//   - "configPath": string - path to the directory containing forge-dev.yaml
//
// Returns ConfigValidateOutput with valid=true if all validations pass,
// or with errors for invalid configuration.
func handleConfigValidate(
	_ context.Context,
	_ *mcp.CallToolRequest,
	input mcptypes.ConfigValidateInput,
) (*mcp.CallToolResult, any, error) {
	output := validateForgeDevConfig(input)

	// forge-dev declares no platform: it generates code on the machine it
	// runs on. A build entry that declares platforms for a forge-dev cell
	// is refused here, before anything builds.
	if len(input.Platforms) > 0 {
		if err := engineframework.RefusePlatforms(Name, engineframework.Capabilities{}, input.Platforms); err != nil {
			output.Valid = false
			output.Errors = append(output.Errors, mcptypes.ValidationError{Field: "platforms", Message: err.Error()})
		}
	}

	// Return result with appropriate success/error status
	if output.Valid {
		result, artifact := mcputil.SuccessResultWithArtifact(
			"Configuration is valid",
			output,
		)
		return result, artifact, nil
	}

	// Validation failed
	result, artifact := mcputil.SuccessResultWithArtifact(
		fmt.Sprintf("Configuration validation failed with %d error(s)", len(output.Errors)),
		output,
	)
	return result, artifact, nil
}

// validateForgeDevConfig validates the forge-dev configuration.
// It checks:
//  1. forge-dev.yaml exists and is valid
//  2. spec.openapi.yaml exists and is valid
//  3. Spec schema is properly defined in the OpenAPI spec
func validateForgeDevConfig(input mcptypes.ConfigValidateInput) *mcptypes.ConfigValidateOutput {
	var errors []mcptypes.ValidationError
	var warnings []mcptypes.ValidationWarning

	// Get configPath from spec or directoryParams
	configPath := ""
	if input.DirectoryParams != nil && input.DirectoryParams.RootDir != "" {
		configPath = input.DirectoryParams.RootDir
	}
	if pathVal, ok := input.Spec["configPath"].(string); ok && pathVal != "" {
		configPath = pathVal
	}

	if configPath == "" {
		errors = append(errors, mcptypes.ValidationError{
			Field:   "configPath",
			Message: "configPath is required for forge-dev validation",
		})
		return &mcptypes.ConfigValidateOutput{
			Valid:  false,
			Errors: errors,
		}
	}

	// Step 1: Read and validate forge-dev.yaml
	config, err := ReadConfig(configPath)
	if err != nil {
		errors = append(errors, mcptypes.ValidationError{
			Field:   "forge-dev.yaml",
			Message: fmt.Sprintf("failed to read forge-dev.yaml: %v", err),
		})
		return &mcptypes.ConfigValidateOutput{
			Valid:  false,
			Errors: errors,
		}
	}

	// Step 2: Validate forge-dev.yaml schema
	configErrors := ValidateConfig(config)
	for _, configErr := range configErrors {
		errors = append(errors, mcptypes.ValidationError{
			Field:   configErr.Field,
			Message: configErr.Message,
		})
	}

	// If forge-dev.yaml has errors, don't proceed with spec validation
	if len(errors) > 0 {
		return &mcptypes.ConfigValidateOutput{
			Valid:  false,
			Errors: errors,
		}
	}

	// Step 3: Read and validate spec.openapi.yaml using kin-openapi.
	//
	// A spec file that does not exist yet is a warning, not an error: shared
	// specs are materialized by the build's own resolution step, so on a
	// fresh clone `forge config validate` runs before the file can exist.
	// The configuration is still fully validated; only the spec's content
	// waits for the build. A file that exists and fails to parse remains an
	// error - that is a broken spec, not an unresolved one.
	if !config.declaresOpenAPI() {
		return validateCellWithNoOpenAPI(config, configPath, warnings)
	}

	specPath := filepath.Join(configPath, config.OpenAPI.SpecPath)
	if _, statErr := os.Stat(specPath); os.IsNotExist(statErr) {
		warnings = append(warnings, mcptypes.ValidationWarning{
			Field: "spec.openapi.yaml",
			Message: fmt.Sprintf(
				"%s does not exist yet; the build's spec resolution materializes it, so its content is validated at build time",
				config.OpenAPI.SpecPath),
		})

		warnings = warnMissingWiring(config, configPath, warnings)

		return &mcptypes.ConfigValidateOutput{Valid: true, Errors: errors, Warnings: warnings}
	}

	spec, err := LoadOpenAPISpec(specPath)
	if err != nil {
		errors = append(errors, mcptypes.ValidationError{
			Field:   "spec.openapi.yaml",
			Message: fmt.Sprintf("failed to parse OpenAPI spec: %v", err),
		})
		return &mcptypes.ConfigValidateOutput{
			Valid:    false,
			Errors:   errors,
			Warnings: warnings,
		}
	}

	// Generate types using new adapter
	types, err := GenerateForgeTypes(spec, config.Generate.PackageName)
	if err != nil {
		errors = append(errors, mcptypes.ValidationError{
			Field:   "spec.openapi.yaml",
			Message: fmt.Sprintf("failed to generate types from OpenAPI spec: %v", err),
		})

		return &mcptypes.ConfigValidateOutput{
			Valid:    false,
			Errors:   errors,
			Warnings: warnings,
		}
	}

	// A generic engine names its inputs and outputs by schema. Those names can
	// only be checked once the spec has produced its types.
	for _, e := range ValidateGenericTools(config, types) {
		errors = append(errors, mcptypes.ValidationError{
			Field:   e.Field,
			Message: e.Message,
		})
	}

	// Find the Spec type
	var specType *ForgeTypeDefinition
	for i := range types {
		if types[i].Name == "Spec" {
			specType = &types[i]
			break
		}
	}

	// Step 4: Validate Spec schema
	if specType == nil {
		errors = append(errors, mcptypes.ValidationError{
			Field:   "Spec",
			Message: "Spec schema is required but not found in OpenAPI spec",
		})
		return &mcptypes.ConfigValidateOutput{
			Valid:    false,
			Errors:   errors,
			Warnings: warnings,
		}
	}

	if len(specType.Properties) == 0 {
		warnings = append(warnings, mcptypes.ValidationWarning{
			Field:   "Spec",
			Message: "Spec schema has no properties defined (empty spec)",
		})
	}

	warnings = warnMissingProto(config, configPath, warnings)
	warnings = warnMissingWiring(config, configPath, warnings)

	if len(errors) > 0 {
		return &mcptypes.ConfigValidateOutput{
			Valid:    false,
			Errors:   errors,
			Warnings: warnings,
		}
	}

	return &mcptypes.ConfigValidateOutput{
		Valid:    true,
		Warnings: warnings,
	}
}
