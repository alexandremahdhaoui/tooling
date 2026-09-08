//go:build unit

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
	"os"
	"path/filepath"
	"testing"
)

func genericConfigWith(tools ...ToolConfig) *Config {
	c := &Config{Name: "ci-state-git", Kind: KindMCPServer, Version: "0.1.0"}
	c.Generate.PackageName = "main"
	if len(tools) > 0 {
		c.Layout = &LayoutConfig{Tools: tools}
	}

	return c
}

func validTool() ToolConfig {
	return ToolConfig{Name: "get", Description: "Read one record.", Input: "StateGetInput", Output: "StateGetOutput"}
}

func hasError(errs []ValidationError, field, message string) bool {
	for _, e := range errs {
		if e.Field == field && e.Message == message {
			return true
		}
	}

	return false
}

func TestGenericRequiresAtLeastOneTool(t *testing.T) {
	errs := ValidateConfig(genericConfigWith())

	if !hasError(errs, "layout.tools", "at least one tool is required for kind: mcp-server") {
		t.Errorf("a generic engine with no tools was accepted: %v", errs)
	}
}

// A contract kind's tools are its contract's, so declaring them here is a
// second authority over the same list and is refused.
func TestToolsAreRejectedOnEveryContractKind(t *testing.T) {
	for _, kind := range ContractKinds {
		c := genericConfigWith(validTool())
		c.Kind = kind

		if !hasError(ValidateConfig(c), "layout.tools", "only kind: mcp-server declares tools; a contract kind's tools are the contract's") {
			t.Errorf("%s accepted a tools block", kind)
		}
	}
}

// The fold: a contract kind stands on its own, with no profile key and no
// tools list, and validates.
func TestAContractKindNeedsNothingElse(t *testing.T) {
	for _, kind := range ContractKinds {
		c := &Config{Name: "sample-engine", Kind: kind, Version: "0.1.0"}
		c.OpenAPI.SpecPath = "./spec.openapi.yaml"
		c.Generate.PackageName = "main"

		if errs := ValidateConfig(c); len(errs) > 0 {
			t.Errorf("kind %s was refused: %v", kind, errs)
		}
	}
}

// Capabilities belong to the builder contract, named by kind now that the
// profile key is gone.
func TestOnlyTheBuilderKindDeclaresCapabilities(t *testing.T) {
	c := &Config{Name: "sample-engine", Kind: string(EngineTypeTestRunner), Version: "0.1.0"}
	c.OpenAPI.SpecPath = "./spec.openapi.yaml"
	c.Generate.PackageName = "main"
	c.Capabilities = &CapabilitiesConfig{Frozen: true}

	if !hasError(ValidateConfig(c), "capabilities", "only a builder declares capabilities; this engine's kind is test-runner") {
		t.Errorf("a test-runner declared capabilities: %v", ValidateConfig(c))
	}

	c.Kind = string(EngineTypeBuilder)

	if errs := ValidateConfig(c); len(errs) > 0 {
		t.Errorf("a builder was refused its capabilities: %v", errs)
	}
}

func TestTheOldTypeKeyFailsLoudNamingTheMigration(t *testing.T) {
	c := genericConfigWith(validTool())
	c.Type = "builder"

	if !hasError(ValidateConfig(c), "type", "removed; write kind: builder, or kind: mcp-server with layout.tools") {
		t.Errorf("a stale type: key did not fail with the migration line: %v", ValidateConfig(c))
	}
}

func TestTheOldGenerateToolsKeyFailsLoud(t *testing.T) {
	c := &Config{Name: "x", Kind: KindMCPServer, Version: "0.1.0"}
	c.Generate.PackageName = "main"
	c.Generate.Tools = []ToolConfig{validTool()}

	if !hasError(ValidateConfig(c), "generate.tools", "moved; declare the tools under layout.tools") {
		t.Errorf("a stale generate.tools block did not fail with the move line: %v", ValidateConfig(c))
	}
}

func TestEveryToolFieldIsChecked(t *testing.T) {
	for _, tc := range []struct {
		name    string
		tool    ToolConfig
		field   string
		message string
	}{
		{
			"no name",
			ToolConfig{Description: "x", Input: "In"},
			"layout.tools[0].name", "required field is missing",
		},
		{
			"name is not an identifier",
			ToolConfig{Name: "1bad", Description: "x", Input: "In"},
			"layout.tools[0].name",
			"must be alphanumeric with hyphens or underscores, starting with a letter",
		},
		{
			"name is reserved",
			ToolConfig{Name: "config-validate", Description: "x", Input: "In"},
			"layout.tools[0].name", `"config-validate" is reserved and registered automatically`,
		},
		{
			"no description",
			ToolConfig{Name: "get", Input: "In"},
			"layout.tools[0].description", "required field is missing",
		},
		{
			"no input",
			ToolConfig{Name: "get", Description: "x"},
			"layout.tools[0].input", "required field is missing",
		},
		{
			"input is not a schema name",
			ToolConfig{Name: "get", Description: "x", Input: "notCamel"},
			"layout.tools[0].input",
			"must be a schema name from components.schemas, CamelCase starting with an uppercase letter",
		},
		{
			"output is not a schema name",
			ToolConfig{Name: "get", Description: "x", Input: "In", Output: "notCamel"},
			"layout.tools[0].output",
			"must be a schema name from components.schemas, CamelCase starting with an uppercase letter",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if !hasError(ValidateConfig(genericConfigWith(tc.tool)), tc.field, tc.message) {
				t.Errorf("accepted %+v, wanted %s: %s", tc.tool, tc.field, tc.message)
			}
		})
	}
}

func TestDuplicateToolNamesAreRejected(t *testing.T) {
	errs := ValidateConfig(genericConfigWith(validTool(), validTool()))

	if !hasError(errs, "layout.tools[1].name", `duplicate tool name "get"`) {
		t.Errorf("two tools named get were accepted: %v", errs)
	}
}

func TestHandlersFuncMustBeExported(t *testing.T) {
	c := genericConfigWith(validTool())
	c.Generate.HandlersFunc = "newHandlers"

	if !hasError(ValidateConfig(c), "generate.handlersFunc", "must be an exported Go identifier") {
		t.Error("an unexported handlers constructor was accepted")
	}
}

func TestHandlersFuncDefaults(t *testing.T) {
	if got := genericConfigWith(validTool()).GetHandlersFunc(); got != "NewHandlers" {
		t.Errorf("GetHandlersFunc() = %q, want NewHandlers", got)
	}

	c := genericConfigWith(validTool())
	c.Generate.HandlersFunc = "BuildHandlers"

	if got := c.GetHandlersFunc(); got != "BuildHandlers" {
		t.Errorf("GetHandlersFunc() = %q, want BuildHandlers", got)
	}
}

func TestCamelTurnsAToolNameIntoAnIdentifier(t *testing.T) {
	for in, want := range map[string]string{
		"get":           "Get",
		"state-get":     "StateGet",
		"state_get":     "StateGet",
		"detectDeps":    "DetectDeps",
		"a-b-c":         "ABC",
		"already-Camel": "AlreadyCamel",
	} {
		if got := camel(in); got != want {
			t.Errorf("camel(%q) = %q, want %q", in, got, want)
		}
	}
}

func knownTypes(names ...string) []ForgeTypeDefinition {
	out := make([]ForgeTypeDefinition, 0, len(names))
	for _, n := range names {
		out = append(out, ForgeTypeDefinition{Name: n})
	}

	return out
}

func TestCrossReferenceFindsAMissingSchema(t *testing.T) {
	c := genericConfigWith(validTool())

	errs := ValidateGenericTools(c, knownTypes("Spec", "StateGetInput"))

	if !hasError(errs, "layout.tools[0].output", `schema "StateGetOutput" not found in components.schemas`) {
		t.Errorf("a tool naming a missing output schema was accepted: %v", errs)
	}
}

func TestCrossReferenceRejectsAnEnumOrUnion(t *testing.T) {
	c := genericConfigWith(ToolConfig{Name: "get", Description: "x", Input: "Status"})

	types := []ForgeTypeDefinition{{Name: "Status", IsEnum: true}}

	errs := ValidateGenericTools(c, types)
	if len(errs) == 0 {
		t.Fatal("an enum was accepted as a tool input")
	}

	types = []ForgeTypeDefinition{{Name: "Status", IsUnion: true}}

	if len(ValidateGenericTools(c, types)) == 0 {
		t.Fatal("a union was accepted as a tool input")
	}
}

func TestCrossReferenceCatchesAGeneratedNameCollision(t *testing.T) {
	c := genericConfigWith(ToolConfig{Name: "get", Description: "x", Input: "In"})

	errs := ValidateGenericTools(c, knownTypes("In", "GetFunc"))

	if !hasError(errs, "layout.tools[0].name",
		`tool name "get" generates type "GetFunc", which collides with a schema of that name`) {
		t.Errorf("a tool colliding with a schema was accepted: %v", errs)
	}
}

func TestCrossReferenceCatchesTwoToolsGeneratingOneType(t *testing.T) {
	c := genericConfigWith(
		ToolConfig{Name: "state-get", Description: "x", Input: "In"},
		ToolConfig{Name: "state_get", Description: "x", Input: "In"},
	)

	errs := ValidateGenericTools(c, knownTypes("In"))

	if !hasError(errs, "layout.tools[1].name",
		`tool names "state-get" and "state_get" both generate type "StateGetFunc"`) {
		t.Errorf("two tools generating one type were accepted: %v", errs)
	}
}

func TestCrossReferenceRejectsASchemaNamedHandlers(t *testing.T) {
	c := genericConfigWith(ToolConfig{Name: "get", Description: "x", Input: "In"})

	errs := ValidateGenericTools(c, knownTypes("In", "Handlers"))

	if !hasError(errs, "layout.tools",
		`a schema named "Handlers" collides with the generated Handlers struct`) {
		t.Errorf("a schema named Handlers was accepted: %v", errs)
	}
}

func TestCrossReferenceIgnoresEveryProfile(t *testing.T) {
	c := genericConfigWith(validTool())
	c.Profile = string(EngineTypeBuilder)

	if errs := ValidateGenericTools(c, nil); errs != nil {
		t.Errorf("cross reference ran on a builder: %v", errs)
	}
}

func TestBuildGenericToolsResolvesEveryField(t *testing.T) {
	c := genericConfigWith(
		ToolConfig{Name: "state-get", Description: "Read.", Input: "In", Output: "Out", UseSpec: true},
		ToolConfig{Name: "ping", Description: "Nothing.", Input: "In"},
	)

	tools := BuildGenericTools(c, nil)

	if len(tools) != 2 {
		t.Fatalf("got %d tools, want 2", len(tools))
	}

	if tools[0].GoName != "StateGet" || tools[0].FuncType != "StateGetFunc" {
		t.Errorf("first tool resolved to %+v", tools[0])
	}

	if tools[0].InputType != "In" || tools[0].OutputType != "Out" || !tools[0].UseSpec {
		t.Errorf("first tool types resolved to %+v", tools[0])
	}

	if tools[1].OutputType != "" {
		t.Errorf("a tool with no output resolved an output type: %q", tools[1].OutputType)
	}
}

func TestBuildGenericToolsQualifiesExternalSpecTypes(t *testing.T) {
	c := genericConfigWith(validTool())

	tools := BuildGenericTools(c, &SpecTypesContext{Prefix: "citypes."})

	if tools[0].InputType != "citypes.StateGetInput" {
		t.Errorf("input type = %q, want the package prefix", tools[0].InputType)
	}

	if tools[0].OutputType != "citypes.StateGetOutput" {
		t.Errorf("output type = %q, want the package prefix", tools[0].OutputType)
	}
}

func TestAFreeFormSchemaBecomesAMapNotAnEmptyStruct(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.openapi.yaml")

	if err := os.WriteFile(specPath, []byte(`openapi: 3.0.3
info:
  title: t
  version: "1"
paths: {}
components:
  schemas:
    Spec:
      type: object
      additionalProperties: true
      description: Engine specific configuration.
    Labels:
      type: object
      additionalProperties:
        type: string
    Held:
      type: object
      properties:
        name:
          type: string
`), 0o600); err != nil {
		t.Fatal(err)
	}

	spec, err := LoadOpenAPISpec(specPath)
	if err != nil {
		t.Fatal(err)
	}

	types, err := GenerateForgeTypes(spec, "main")
	if err != nil {
		t.Fatal(err)
	}

	byName := map[string]ForgeTypeDefinition{}
	for _, td := range types {
		byName[td.Name] = td
	}

	if !byName["Spec"].IsMap {
		t.Error("a free form object must be a map, or it drops everything the caller sent")
	}

	if got := byName["Spec"].MapValueType; got != "interface{}" {
		t.Errorf("Spec value type = %q, want interface{}", got)
	}

	if got := byName["Labels"].MapValueType; got != "string" {
		t.Errorf("Labels value type = %q, want string", got)
	}

	if byName["Held"].IsMap {
		t.Error("an object with properties is a struct")
	}
}

func TestAPropertyReferencingAnEnumParsesAsAString(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.openapi.yaml")

	if err := os.WriteFile(specPath, []byte(`openapi: 3.0.3
info:
  title: t
  version: "1"
paths: {}
components:
  schemas:
    Spec:
      type: object
      additionalProperties: true
    Status:
      type: string
      enum: [passed, failed]
    Run:
      type: object
      required: [status]
      properties:
        status:
          $ref: '#/components/schemas/Status'
`), 0o600); err != nil {
		t.Fatal(err)
	}

	spec, err := LoadOpenAPISpec(specPath)
	if err != nil {
		t.Fatal(err)
	}

	types, err := GenerateForgeTypes(spec, "main")
	if err != nil {
		t.Fatal(err)
	}

	for _, td := range types {
		if td.Name != "Run" {
			continue
		}

		for _, p := range td.Properties {
			if p.JsonName != "status" {
				continue
			}

			if !p.IsEnumRef {
				t.Fatal("a property referencing an enum must parse as a string, " +
					"because an enum has no FromMap to call")
			}

			return
		}
	}

	t.Fatal("Run.status not found")
}
