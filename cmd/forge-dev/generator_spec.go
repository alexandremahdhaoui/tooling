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
	"bytes"
	"go/format"
)

// SpecTemplateData contains the data passed to the spec.go.tmpl template.
type SpecTemplateData struct {
	// PackageName is the Go package name for the generated file.
	PackageName string
	// ChecksumHeader is the checksum header line (e.g., "SourceChecksum: sha256:...")
	ChecksumHeader string
	// EngineName is the name of the engine.
	EngineName string
	// Properties contains all Spec properties (for backwards compatibility with simple schemas).
	Properties []PropertySchema
	// HasArrayOrMap indicates if any property is an array or map (needs fmt import).
	HasArrayOrMap bool
	// NeedsFmtImport indicates if the generated code needs the fmt import.
	NeedsFmtImport bool
	// Registry provides access to all named schemas for multi-schema generation.
	// When Registry is set, templates can iterate over Registry.GetGenerationOrder()
	// to generate code for multiple schemas in dependency order.
	Registry *SchemaRegistry
	// Types contains all type definitions for the new kin-openapi-based generation path.
	// When Types is set, templates iterate over Types to generate structs in topological order.
	Types []ForgeTypeDefinition
	// MainType is the name of the main entry point type (always "Spec").
	MainType string
	// SpecTypesContext holds external spec types info (nil when disabled).
	SpecTypesContext *SpecTypesContext
}

// GenerateSpecFile generates the zz_generated.spec.go file content.
// It uses the spec.go.tmpl template to generate Go code with:
// - Spec struct with JSON tags
// - FromMap function to parse map[string]interface{} to Spec
// - ToMap function to convert Spec to map[string]interface{}
//
// The registry parameter is optional for backwards compatibility:
// - If nil, only the Spec schema properties are used (simple case)
// - If provided, the template can access all schemas for multi-schema generation
func GenerateSpecFile(schema *SpecSchema, config *Config, checksum string, registry *SchemaRegistry) ([]byte, error) {
	// Prepare template data
	data := SpecTemplateData{
		PackageName:    config.Generate.PackageName,
		ChecksumHeader: ChecksumHeader(checksum),
		EngineName:     config.Name,
		Properties:     schema.Properties,
		HasArrayOrMap:  hasArrayOrMapType(schema.Properties),
		NeedsFmtImport: true, // every FromMap refuses an unknown key with fmt.Errorf
		Registry:       registry,
	}

	// Parse and execute template
	tmpl, err := parseTemplate("spec.go.tmpl")
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, err
	}

	// Format the generated code
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		// Return unformatted code for debugging
		return buf.Bytes(), err
	}

	return formatted, nil
}

// hasArrayOrMapType checks if any property is an array or map type.
// This is used to determine if the fmt import is needed.
func hasArrayOrMapType(properties []PropertySchema) bool {
	for _, p := range properties {
		goType := p.GoType()
		if len(goType) > 2 && goType[:2] == "[]" {
			return true
		}
		if len(goType) > 3 && goType[:3] == "map" {
			return true
		}
	}
	return false
}

// needsFmtImport checks if any property needs the fmt import.
// Returns true only if at least one property has a supported type that
// generates code using fmt.Errorf. Unsupported types like []interface{}
// don't generate code that uses fmt.
func needsFmtImport(properties []PropertySchema) bool {
	supportedTypes := map[string]bool{
		"string":            true,
		"bool":              true,
		"int":               true,
		"float64":           true,
		"[]string":          true,
		"[]int":             true,
		"map[string]string": true,
	}
	for _, p := range properties {
		if supportedTypes[p.GoType()] {
			return true
		}
	}
	return false
}

// GenerateSpecFileFromTypes generates the zz_generated.spec.go file content using ForgeTypeDefinition.
// This is the new generation path that uses kin-openapi-based types.
func GenerateSpecFileFromTypes(types []ForgeTypeDefinition, config *Config, checksum string, specTypesCtx *SpecTypesContext) ([]byte, error) {
	// Determine package name: use specTypesCtx.PackageName when enabled, otherwise config.Generate.PackageName
	packageName := config.Generate.PackageName
	if specTypesCtx != nil {
		packageName = specTypesCtx.PackageName
	}

	// Prepare template data
	data := SpecTemplateData{
		PackageName:      packageName,
		ChecksumHeader:   ChecksumHeader(checksum),
		EngineName:       config.Name,
		Types:            types,
		MainType:         "Spec",
		NeedsFmtImport:   true, // every FromMap refuses an unknown key with fmt.Errorf
		SpecTypesContext: specTypesCtx,
	}

	// Parse and execute template
	tmpl, err := parseTemplate("spec.go.tmpl")
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, err
	}

	// Format the generated code
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		// Return unformatted code for debugging
		return buf.Bytes(), err
	}

	return formatted, nil
}

// GenerateValidateFileFromTypes generates the zz_generated.validate.go file content using ForgeTypeDefinition.
// This is the new generation path that uses kin-openapi-based types.
func GenerateValidateFileFromTypes(types []ForgeTypeDefinition, config *Config, checksum string, specTypesCtx *SpecTypesContext) ([]byte, error) {
	// Prepare template data
	// NOTE: validate.go always uses config.Generate.PackageName (always "main")
	// because it's generated alongside MCP code, not the spec types.
	data := SpecTemplateData{
		PackageName:      config.Generate.PackageName,
		ChecksumHeader:   ChecksumHeader(checksum),
		EngineName:       config.Name,
		Types:            types,
		MainType:         "Spec",
		NeedsFmtImport:   needsFmtImportForValidation(types),
		SpecTypesContext: specTypesCtx,
	}

	// Parse and execute template
	tmpl, err := parseTemplate("validate.go.tmpl")
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, err
	}

	// Format the generated code
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		// Return unformatted code for debugging
		return buf.Bytes(), err
	}

	return formatted, nil
}

// needsFmtImportForTypes checks if any ForgeProperty needs the fmt import.
// Returns true if there are any properties that generate code using fmt.Errorf.
func needsFmtImportForTypes(types []ForgeTypeDefinition) bool {
	supportedTypes := map[string]bool{
		"string":            true,
		"bool":              true,
		"int":               true,
		"float64":           true,
		"[]string":          true,
		"[]int":             true,
		"map[string]string": true,
	}
	for _, t := range types {
		// Skip enum and union types - they don't generate FromMap/ToMap
		if t.IsEnum || t.IsUnion {
			continue
		}
		for _, p := range t.Properties {
			// Properties with supported types need fmt
			if supportedTypes[p.GoType] {
				return true
			}
			// Reference types need fmt for error wrapping
			if p.IsRef || p.IsArrayOfRef {
				return true
			}
			// Arrays and maps need fmt
			if p.IsArray || p.IsMap {
				return true
			}
		}
	}
	return false
}

// needsFmtImportForValidation checks if validation code needs the fmt import.
// Returns true if there are any array of references that need fmt.Sprintf.
func needsFmtImportForValidation(types []ForgeTypeDefinition) bool {
	for _, t := range types {
		// Skip enum and union types
		if t.IsEnum || t.IsUnion {
			continue
		}
		for _, p := range t.Properties {
			// Array of references need fmt.Sprintf for field path
			if p.IsArrayOfRef {
				return true
			}
		}
	}
	return false
}
