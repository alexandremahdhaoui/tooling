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
	"fmt"
	"go/format"
	"strings"
)

// MCPTemplateData contains the data passed to the mcp.go.tmpl templates.
type MCPTemplateData struct {
	// PackageName is the Go package name for the generated file.
	PackageName string
	// ChecksumHeader is the checksum header line.
	ChecksumHeader string
	// EngineName is the name of the engine.
	EngineName string
	// EngineType is the contract the engine implements, or generic.
	EngineType EngineType

	// Kind is what the file declared: a contract name, or mcp-server. The
	// engine answers it over config-validate so a caller learns what it is
	// from the engine rather than from a key beside every use of it.
	Kind string
	// SpecTypesContext holds external spec types info (nil when disabled).
	SpecTypesContext *SpecTypesContext
	// Tools are the resolved tools of a generic engine. Empty for every other
	// engine type, whose tools are fixed by their family.
	Tools []GenericTool
	// Platforms is a builder's declared platforms, as a Go literal the
	// template pastes into the engine's Capabilities: `nil` for an engine
	// that declares nothing, which the framework reads as host only.
	Platforms string
	// Frozen is whether the builder declares it reads the frozen input.
	Frozen bool

	// Incremental is whether the builder declares its output may be reused
	// while its inputs are unchanged.
	Incremental bool
}

// platformsLiteral renders a platform declaration as Go source.
func platformsLiteral(platforms []string) string {
	if len(platforms) == 0 {
		return "nil"
	}

	quoted := make([]string, 0, len(platforms))
	for _, p := range platforms {
		quoted = append(quoted, fmt.Sprintf("%q", p))
	}

	return "[]string{" + strings.Join(quoted, ", ") + "}"
}

// GenerateMCPFile generates the zz_generated.mcp.go file content.
// It selects the appropriate template based on the engine type:
// - builder: mcp_builder.go.tmpl
// - test-runner: mcp_testrunner.go.tmpl
// - testenv-subengine: mcp_testenv.go.tmpl
func GenerateMCPFile(config *Config, checksum string, specTypesCtx *SpecTypesContext) ([]byte, error) {
	// Prepare template data
	data := MCPTemplateData{
		PackageName:      config.Generate.PackageName,
		ChecksumHeader:   ChecksumHeader(checksum),
		EngineName:       config.Name,
		EngineType:       config.engineType(),
		Kind:             config.Kind,
		SpecTypesContext: specTypesCtx,
		Tools:            BuildGenericTools(config, specTypesCtx),
		Platforms:        platformsLiteral(config.platforms()),
		Frozen:           config.frozen(),
		Incremental:      config.incremental(),
	}

	// Select template based on engine type
	templateName, err := mcpTemplateName(config.engineType())
	if err != nil {
		return nil, err
	}

	// Parse and execute template
	tmpl, err := parseTemplate(templateName)
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

// mcpTemplateName returns the template filename for the given engine type.
func mcpTemplateName(engineType EngineType) (string, error) {
	switch engineType {
	case EngineTypeBuilder:
		return "mcp_builder.go.tmpl", nil
	case EngineTypeTestRunner:
		return "mcp_testrunner.go.tmpl", nil
	case EngineTypeTestEnvSubengine:
		return "mcp_testenv.go.tmpl", nil
	case EngineTypeDependencyDetector:
		return "mcp_dependency_detector.go.tmpl", nil
	case EngineTypeGeneric:
		return "mcp_generic.go.tmpl", nil
	default:
		return "", fmt.Errorf("unsupported engine type: %s", engineType)
	}
}
