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
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestGenerateSpecFile(t *testing.T) {
	tests := []struct {
		name       string
		schema     *SpecSchema
		config     *Config
		checksum   string
		wantFields []string
		wantFuncs  []string
		wantErr    bool
	}{
		{
			name: "basic string fields",
			schema: &SpecSchema{
				Properties: []PropertySchema{
					{Name: "name", Type: "string", Required: true, Description: "The name field"},
					{Name: "version", Type: "string", Required: false, Description: "Optional version"},
				},
				Required: []string{"name"},
			},
			config: &Config{
				Name: "test-engine",
				Kind: KindMCPServer, Profile: "builder",
				Generate: GenerateConfig{
					PackageName: "main",
				},
			},
			checksum:   "sha256:abc123",
			wantFields: []string{"Name string", "Version string"},
			wantFuncs:  []string{"func FromMap(", "func (s *Spec) ToMap()"},
			wantErr:    false,
		},
		{
			name: "multiple types",
			schema: &SpecSchema{
				Properties: []PropertySchema{
					{Name: "count", Type: "integer", Required: false, Description: "Item count"},
					{Name: "enabled", Type: "boolean", Required: false, Description: "Is enabled"},
					{Name: "ratio", Type: "number", Required: false, Description: "A ratio"},
				},
			},
			config: &Config{
				Name: "test-engine",
				Kind: KindMCPServer, Profile: "builder",
				Generate: GenerateConfig{
					PackageName: "main",
				},
			},
			checksum:   "sha256:def456",
			wantFields: []string{"Count int", "Enabled bool", "Ratio float64"},
			wantFuncs:  []string{"func FromMap(", "func (s *Spec) ToMap()"},
			wantErr:    false,
		},
		{
			name: "array and map types",
			schema: &SpecSchema{
				Properties: []PropertySchema{
					{
						Name:        "args",
						Type:        "array",
						Description: "Arguments",
						Items:       &PropertySchema{Type: "string"},
					},
					{
						Name:                 "env",
						Type:                 "object",
						Description:          "Environment variables",
						AdditionalProperties: &PropertySchema{Type: "string"},
					},
				},
			},
			config: &Config{
				Name: "test-engine",
				Kind: KindMCPServer, Profile: "builder",
				Generate: GenerateConfig{
					PackageName: "main",
				},
			},
			checksum:   "sha256:ghi789",
			wantFields: []string{"Args []string", "Env map[string]string"},
			wantFuncs:  []string{"func FromMap(", "func (s *Spec) ToMap()"},
			wantErr:    false,
		},
		{
			name: "empty schema",
			schema: &SpecSchema{
				Properties: []PropertySchema{},
			},
			config: &Config{
				Name: "test-engine",
				Kind: KindMCPServer, Profile: "builder",
				Generate: GenerateConfig{
					PackageName: "main",
				},
			},
			checksum:   "sha256:empty",
			wantFields: []string{},
			wantFuncs:  []string{"func FromMap(", "func (s *Spec) ToMap()"},
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Pass nil registry for backwards compatibility tests
			got, err := GenerateSpecFile(tt.schema, tt.config, tt.checksum, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateSpecFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			code := string(got)

			// Check that the code contains expected fields
			for _, field := range tt.wantFields {
				if !strings.Contains(code, field) {
					t.Errorf("Generated code missing field %q", field)
				}
			}

			// Check that the code contains expected functions
			for _, fn := range tt.wantFuncs {
				if !strings.Contains(code, fn) {
					t.Errorf("Generated code missing function %q", fn)
				}
			}

			// Check that the code contains the checksum
			if !strings.Contains(code, tt.checksum) {
				t.Errorf("Generated code missing checksum %q", tt.checksum)
			}

			// Verify the generated code compiles
			fset := token.NewFileSet()
			_, parseErr := parser.ParseFile(fset, "spec.go", got, parser.AllErrors)
			if parseErr != nil {
				t.Errorf("Generated code does not compile: %v\nCode:\n%s", parseErr, code)
			}
		})
	}
}

func TestGenerateSpecFile_Compiles(t *testing.T) {
	// Test with a more complex schema to ensure generated code compiles
	schema := &SpecSchema{
		Properties: []PropertySchema{
			{Name: "name", Type: "string", Required: true, Description: "Engine name"},
			{Name: "version", Type: "string", Required: false, Description: "Version"},
			{Name: "count", Type: "integer", Required: false, Description: "Count"},
			{Name: "enabled", Type: "boolean", Required: false, Description: "Enabled flag"},
			{Name: "ratio", Type: "number", Required: false, Description: "A ratio"},
			{
				Name:        "args",
				Type:        "array",
				Description: "Arguments",
				Items:       &PropertySchema{Type: "string"},
			},
			{
				Name:                 "env",
				Type:                 "object",
				Description:          "Environment variables",
				AdditionalProperties: &PropertySchema{Type: "string"},
			},
		},
		Required: []string{"name"},
	}

	config := &Config{
		Name: "complex-engine",
		Kind: KindMCPServer, Profile: "builder",
		Generate: GenerateConfig{
			PackageName: "main",
		},
	}

	// Pass nil registry for backwards compatibility tests
	got, err := GenerateSpecFile(schema, config, "sha256:test", nil)
	if err != nil {
		t.Fatalf("GenerateSpecFile() error = %v", err)
	}

	// Verify the generated code compiles
	fset := token.NewFileSet()
	_, parseErr := parser.ParseFile(fset, "spec.go", got, parser.AllErrors)
	if parseErr != nil {
		t.Errorf("Generated code does not compile: %v\nCode:\n%s", parseErr, string(got))
	}
}

func TestHasArrayOrMapType(t *testing.T) {
	tests := []struct {
		name       string
		properties []PropertySchema
		want       bool
	}{
		{
			name:       "empty",
			properties: []PropertySchema{},
			want:       false,
		},
		{
			name: "only simple types",
			properties: []PropertySchema{
				{Name: "s", Type: "string"},
				{Name: "i", Type: "integer"},
				{Name: "b", Type: "boolean"},
			},
			want: false,
		},
		{
			name: "has array",
			properties: []PropertySchema{
				{Name: "s", Type: "string"},
				{Name: "arr", Type: "array", Items: &PropertySchema{Type: "string"}},
			},
			want: true,
		},
		{
			name: "has map",
			properties: []PropertySchema{
				{Name: "s", Type: "string"},
				{Name: "m", Type: "object", AdditionalProperties: &PropertySchema{Type: "string"}},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasArrayOrMapType(tt.properties)
			if got != tt.want {
				t.Errorf("hasArrayOrMapType() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestGenerateSpecFileFromTypes tests the new ForgeTypeDefinition-based generation path.
func TestGenerateSpecFileFromTypes(t *testing.T) {
	tests := []struct {
		name       string
		types      []ForgeTypeDefinition
		config     *Config
		checksum   string
		wantFields []string
		wantFuncs  []string
		wantErr    bool
	}{
		{
			name: "basic string fields",
			types: []ForgeTypeDefinition{
				{
					Name:     "Spec",
					JsonName: "Spec",
					Properties: []ForgeProperty{
						{Name: "Name", JsonName: "name", GoType: "string", Required: true, Description: "The name field"},
						{Name: "Version", JsonName: "version", GoType: "string", Required: false, Description: "Optional version"},
					},
				},
			},
			config: &Config{
				Name: "test-engine",
				Kind: KindMCPServer, Profile: "builder",
				Generate: GenerateConfig{
					PackageName: "main",
				},
			},
			checksum:   "sha256:abc123",
			wantFields: []string{"Name string", "Version string"},
			wantFuncs:  []string{"func SpecFromMap(", "func (s *Spec) ToMap()"},
			wantErr:    false,
		},
		{
			name: "multiple types",
			types: []ForgeTypeDefinition{
				{
					Name:     "Spec",
					JsonName: "Spec",
					Properties: []ForgeProperty{
						{Name: "Count", JsonName: "count", GoType: "int", Required: false, Description: "Item count"},
						{Name: "Enabled", JsonName: "enabled", GoType: "bool", Required: false, Description: "Is enabled"},
						{Name: "Ratio", JsonName: "ratio", GoType: "float64", Required: false, Description: "A ratio"},
					},
				},
			},
			config: &Config{
				Name: "test-engine",
				Kind: KindMCPServer, Profile: "builder",
				Generate: GenerateConfig{
					PackageName: "main",
				},
			},
			checksum:   "sha256:def456",
			wantFields: []string{"Count int", "Enabled bool", "Ratio float64"},
			wantFuncs:  []string{"func SpecFromMap(", "func (s *Spec) ToMap()"},
			wantErr:    false,
		},
		{
			name: "array and map types",
			types: []ForgeTypeDefinition{
				{
					Name:     "Spec",
					JsonName: "Spec",
					Properties: []ForgeProperty{
						{Name: "Args", JsonName: "args", GoType: "[]string", IsArray: true, ArrayItemType: "string", Description: "Arguments"},
						{Name: "Env", JsonName: "env", GoType: "map[string]string", IsMap: true, MapValueType: "string", Description: "Environment variables"},
					},
				},
			},
			config: &Config{
				Name: "test-engine",
				Kind: KindMCPServer, Profile: "builder",
				Generate: GenerateConfig{
					PackageName: "main",
				},
			},
			checksum:   "sha256:ghi789",
			wantFields: []string{"Args []string", "Env map[string]string"},
			wantFuncs:  []string{"func SpecFromMap(", "func (s *Spec) ToMap()"},
			wantErr:    false,
		},
		{
			name: "reference types",
			types: []ForgeTypeDefinition{
				{
					Name:     "VMResource",
					JsonName: "VMResource",
					Properties: []ForgeProperty{
						{Name: "Name", JsonName: "name", GoType: "string", Required: true},
					},
				},
				{
					Name:     "Spec",
					JsonName: "Spec",
					Properties: []ForgeProperty{
						{Name: "VM", JsonName: "vm", GoType: "VMResource", IsRef: true, RefType: "VMResource"},
					},
				},
			},
			config: &Config{
				Name: "test-engine",
				Kind: KindMCPServer, Profile: "builder",
				Generate: GenerateConfig{
					PackageName: "main",
				},
			},
			checksum:   "sha256:ref123",
			wantFields: []string{"type VMResource struct", "type Spec struct", "VM VMResource"},
			wantFuncs:  []string{"func VMResourceFromMap(", "func SpecFromMap("},
			wantErr:    false,
		},
		{
			name: "array of references",
			types: []ForgeTypeDefinition{
				{
					Name:     "Item",
					JsonName: "Item",
					Properties: []ForgeProperty{
						{Name: "ID", JsonName: "id", GoType: "string", Required: true},
					},
				},
				{
					Name:     "Spec",
					JsonName: "Spec",
					Properties: []ForgeProperty{
						{Name: "Items", JsonName: "items", GoType: "[]Item", IsArray: true, IsArrayOfRef: true, ArrayItemType: "Item"},
					},
				},
			},
			config: &Config{
				Name: "test-engine",
				Kind: KindMCPServer, Profile: "builder",
				Generate: GenerateConfig{
					PackageName: "main",
				},
			},
			checksum:   "sha256:arrref123",
			wantFields: []string{"type Item struct", "type Spec struct", "Items []Item"},
			wantFuncs:  []string{"func ItemFromMap(", "func SpecFromMap("},
			wantErr:    false,
		},
		{
			name:  "empty types",
			types: []ForgeTypeDefinition{},
			config: &Config{
				Name: "test-engine",
				Kind: KindMCPServer, Profile: "builder",
				Generate: GenerateConfig{
					PackageName: "main",
				},
			},
			checksum:   "sha256:empty",
			wantFields: []string{},
			wantFuncs:  []string{},
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GenerateSpecFileFromTypes(tt.types, tt.config, tt.checksum, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateSpecFileFromTypes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			code := string(got)

			// Check that the code contains expected fields
			for _, field := range tt.wantFields {
				if !strings.Contains(code, field) {
					t.Errorf("Generated code missing field %q\nCode:\n%s", field, code)
				}
			}

			// Check that the code contains expected functions
			for _, fn := range tt.wantFuncs {
				if !strings.Contains(code, fn) {
					t.Errorf("Generated code missing function %q\nCode:\n%s", fn, code)
				}
			}

			// Check that the code contains the checksum
			if !strings.Contains(code, tt.checksum) {
				t.Errorf("Generated code missing checksum %q", tt.checksum)
			}

			// Verify the generated code compiles
			if len(code) > 0 {
				fset := token.NewFileSet()
				_, parseErr := parser.ParseFile(fset, "spec.go", got, parser.AllErrors)
				if parseErr != nil {
					t.Errorf("Generated code does not compile: %v\nCode:\n%s", parseErr, code)
				}
			}
		})
	}
}

// TestGenerateValidateFileFromTypes tests the new ForgeTypeDefinition-based validation generation path.
func TestGenerateValidateFileFromTypes(t *testing.T) {
	tests := []struct {
		name      string
		types     []ForgeTypeDefinition
		config    *Config
		checksum  string
		wantFuncs []string
		wantErr   bool
	}{
		{
			name: "basic validation",
			types: []ForgeTypeDefinition{
				{
					Name:     "Spec",
					JsonName: "Spec",
					Properties: []ForgeProperty{
						{Name: "Name", JsonName: "name", GoType: "string", Required: true},
						{Name: "Count", JsonName: "count", GoType: "int", Required: false},
					},
				},
			},
			config: &Config{
				Name: "test-engine",
				Kind: KindMCPServer, Profile: "builder",
				Generate: GenerateConfig{
					PackageName: "main",
				},
			},
			checksum:  "sha256:val123",
			wantFuncs: []string{"func ValidateSpec(", "func Validate(", "func ValidateMap("},
			wantErr:   false,
		},
		{
			name: "multiple types validation",
			types: []ForgeTypeDefinition{
				{
					Name:     "Item",
					JsonName: "Item",
					Properties: []ForgeProperty{
						{Name: "ID", JsonName: "id", GoType: "string", Required: true},
					},
				},
				{
					Name:     "Spec",
					JsonName: "Spec",
					Properties: []ForgeProperty{
						{Name: "Item", JsonName: "item", GoType: "Item", IsRef: true, RefType: "Item"},
					},
				},
			},
			config: &Config{
				Name: "test-engine",
				Kind: KindMCPServer, Profile: "builder",
				Generate: GenerateConfig{
					PackageName: "main",
				},
			},
			checksum:  "sha256:multival123",
			wantFuncs: []string{"func ValidateSpec(", "func ValidateItem("},
			wantErr:   false,
		},
		{
			name:  "empty types",
			types: []ForgeTypeDefinition{},
			config: &Config{
				Name: "test-engine",
				Kind: KindMCPServer, Profile: "builder",
				Generate: GenerateConfig{
					PackageName: "main",
				},
			},
			checksum:  "sha256:empty",
			wantFuncs: []string{},
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GenerateValidateFileFromTypes(tt.types, tt.config, tt.checksum, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateValidateFileFromTypes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			code := string(got)

			// Check that the code contains expected functions
			for _, fn := range tt.wantFuncs {
				if !strings.Contains(code, fn) {
					t.Errorf("Generated code missing function %q\nCode:\n%s", fn, code)
				}
			}

			// Check that the code contains the checksum
			if !strings.Contains(code, tt.checksum) {
				t.Errorf("Generated code missing checksum %q", tt.checksum)
			}

			// Verify the generated code compiles
			if len(code) > 0 {
				fset := token.NewFileSet()
				_, parseErr := parser.ParseFile(fset, "spec.go", got, parser.AllErrors)
				if parseErr != nil {
					t.Errorf("Generated code does not compile: %v\nCode:\n%s", parseErr, code)
				}
			}
		})
	}
}

// TestGenerateSpecFileFromTypes_PackageName verifies that the generated spec file
// uses the correct package name based on SpecTypesContext.
// - When specTypesCtx == nil: uses config.Generate.PackageName (e.g., "main")
// - When specTypesCtx.PackageName == "v1": uses "v1"
func TestGenerateSpecFileFromTypes_PackageName(t *testing.T) {
	types := []ForgeTypeDefinition{
		{
			Name:     "Spec",
			JsonName: "Spec",
			Properties: []ForgeProperty{
				{Name: "Name", JsonName: "name", GoType: "string", Required: true, Description: "The name field"},
			},
		},
	}

	config := &Config{
		Name: "test-engine",
		Kind: KindMCPServer, Profile: "builder",
		Generate: GenerateConfig{
			PackageName: "main",
		},
	}

	tests := []struct {
		name           string
		specTypesCtx   *SpecTypesContext
		wantPackage    string
		wantNotPackage string
	}{
		{
			name:           "nil specTypesCtx uses config.Generate.PackageName",
			specTypesCtx:   nil,
			wantPackage:    "package main",
			wantNotPackage: "package v1",
		},
		{
			name: "specTypesCtx.PackageName overrides config package name",
			specTypesCtx: &SpecTypesContext{
				ImportPath:  "github.com/test/project/pkg/api/v1",
				PackageName: "v1",
				Prefix:      "v1.",
				OutputDir:   "/tmp/test/pkg/api/v1",
			},
			wantPackage:    "package v1",
			wantNotPackage: "package main",
		},
		{
			name: "specTypesCtx with different package name",
			specTypesCtx: &SpecTypesContext{
				ImportPath:  "github.com/test/project/pkg/api/v2alpha1",
				PackageName: "v2alpha1",
				Prefix:      "v2alpha1.",
				OutputDir:   "/tmp/test/pkg/api/v2alpha1",
			},
			wantPackage:    "package v2alpha1",
			wantNotPackage: "package main",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GenerateSpecFileFromTypes(types, config, "sha256:test", tt.specTypesCtx)
			if err != nil {
				t.Fatalf("GenerateSpecFileFromTypes() error = %v", err)
			}

			code := string(got)

			// Verify the expected package declaration is present
			if !strings.Contains(code, tt.wantPackage) {
				t.Errorf("Generated code missing expected package declaration %q\nCode:\n%s", tt.wantPackage, code)
			}

			// Verify the other package declaration is NOT present
			if strings.Contains(code, tt.wantNotPackage) {
				t.Errorf("Generated code should not contain %q\nCode:\n%s", tt.wantNotPackage, code)
			}

			// Verify the generated code compiles
			fset := token.NewFileSet()
			_, parseErr := parser.ParseFile(fset, "spec.go", got, parser.AllErrors)
			if parseErr != nil {
				t.Errorf("Generated code does not compile: %v\nCode:\n%s", parseErr, code)
			}
		})
	}
}

// TestGenerateSpecFileFromTypes_StructureWithExternalPackage verifies the generated code structure
// is correct when using external spec types (SpecTypesContext enabled).
// All FromMap and ToMap functions should be generated correctly.
func TestGenerateSpecFileFromTypes_StructureWithExternalPackage(t *testing.T) {
	types := []ForgeTypeDefinition{
		{
			Name:     "Item",
			JsonName: "Item",
			Properties: []ForgeProperty{
				{Name: "ID", JsonName: "id", GoType: "string", Required: true},
			},
		},
		{
			Name:     "Spec",
			JsonName: "Spec",
			Properties: []ForgeProperty{
				{Name: "Name", JsonName: "name", GoType: "string", Required: true},
				{Name: "Items", JsonName: "items", GoType: "[]Item", IsArray: true, IsArrayOfRef: true, ArrayItemType: "Item"},
			},
		},
	}

	config := &Config{
		Name: "test-engine",
		Kind: KindMCPServer, Profile: "builder",
		Generate: GenerateConfig{
			PackageName: "main",
		},
	}

	specTypesCtx := &SpecTypesContext{
		ImportPath:  "github.com/test/project/pkg/api/v1",
		PackageName: "v1",
		Prefix:      "v1.",
		OutputDir:   "/tmp/test/pkg/api/v1",
	}

	got, err := GenerateSpecFileFromTypes(types, config, "sha256:structure-test", specTypesCtx)
	if err != nil {
		t.Fatalf("GenerateSpecFileFromTypes() error = %v", err)
	}

	code := string(got)

	// Verify package declaration
	if !strings.Contains(code, "package v1") {
		t.Errorf("Generated code missing 'package v1'\nCode:\n%s", code)
	}

	// Verify type definitions exist
	expectedTypes := []string{
		"type Item struct",
		"type Spec struct",
	}
	for _, expectedType := range expectedTypes {
		if !strings.Contains(code, expectedType) {
			t.Errorf("Generated code missing type definition %q\nCode:\n%s", expectedType, code)
		}
	}

	// Verify FromMap functions exist
	expectedFromMapFuncs := []string{
		"func ItemFromMap(",
		"func SpecFromMap(",
		"func FromMap(",
	}
	for _, expectedFunc := range expectedFromMapFuncs {
		if !strings.Contains(code, expectedFunc) {
			t.Errorf("Generated code missing FromMap function %q\nCode:\n%s", expectedFunc, code)
		}
	}

	// Verify ToMap methods exist
	expectedToMapMethods := []string{
		"func (s *Item) ToMap()",
		"func (s *Spec) ToMap()",
	}
	for _, expectedMethod := range expectedToMapMethods {
		if !strings.Contains(code, expectedMethod) {
			t.Errorf("Generated code missing ToMap method %q\nCode:\n%s", expectedMethod, code)
		}
	}

	// Verify the generated code compiles
	fset := token.NewFileSet()
	_, parseErr := parser.ParseFile(fset, "spec.go", got, parser.AllErrors)
	if parseErr != nil {
		t.Errorf("Generated code does not compile: %v\nCode:\n%s", parseErr, code)
	}
}

// TestNeedsFmtImportForValidation tests the needsFmtImportForValidation helper function.
func TestNeedsFmtImportForValidation(t *testing.T) {
	tests := []struct {
		name  string
		types []ForgeTypeDefinition
		want  bool
	}{
		{
			name:  "empty types",
			types: []ForgeTypeDefinition{},
			want:  false,
		},
		{
			name: "simple types do not need fmt for validation",
			types: []ForgeTypeDefinition{
				{
					Name: "Spec",
					Properties: []ForgeProperty{
						{Name: "Name", GoType: "string"},
					},
				},
			},
			want: false,
		},
		{
			name: "array of ref needs fmt for validation",
			types: []ForgeTypeDefinition{
				{
					Name: "Spec",
					Properties: []ForgeProperty{
						{Name: "Items", GoType: "[]Item", IsArrayOfRef: true},
					},
				},
			},
			want: true,
		},
		{
			name: "enum type skipped",
			types: []ForgeTypeDefinition{
				{
					Name:       "Status",
					IsEnum:     true,
					EnumValues: []string{"active", "inactive"},
				},
			},
			want: false,
		},
		{
			name: "union type skipped",
			types: []ForgeTypeDefinition{
				{
					Name:          "MyUnion",
					IsUnion:       true,
					UnionVariants: []string{"TypeA", "TypeB"},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := needsFmtImportForValidation(tt.types)
			if got != tt.want {
				t.Errorf("needsFmtImportForValidation() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestGenerateSpecFileFromTypes_MultipleNonPointerRefs is a regression test for the
// duplicate variable declaration bug in ToMap() when a struct has multiple non-pointer
// $ref fields. The bug occurred because the template used:
//
//	refMap := s.FieldA.ToMap()
//	...
//	refMap := s.FieldB.ToMap()  // ERROR: duplicate declaration
//
// The fix scopes refMap within the if statement:
//
//	if refMap := s.FieldA.ToMap(); len(refMap) > 0 { ... }
//	if refMap := s.FieldB.ToMap(); len(refMap) > 0 { ... }
func TestGenerateSpecFileFromTypes_MultipleNonPointerRefs(t *testing.T) {
	// Define types where Spec has multiple non-pointer $ref fields
	types := []ForgeTypeDefinition{
		{
			Name:     "NetworkConfig",
			JsonName: "NetworkConfig",
			Properties: []ForgeProperty{
				{Name: "Subnet", JsonName: "subnet", GoType: "string", Required: true},
			},
		},
		{
			Name:     "StorageConfig",
			JsonName: "StorageConfig",
			Properties: []ForgeProperty{
				{Name: "Size", JsonName: "size", GoType: "int", Required: true},
			},
		},
		{
			Name:     "ComputeConfig",
			JsonName: "ComputeConfig",
			Properties: []ForgeProperty{
				{Name: "Cores", JsonName: "cores", GoType: "int", Required: true},
			},
		},
		{
			Name:     "Spec",
			JsonName: "Spec",
			Properties: []ForgeProperty{
				{Name: "Name", JsonName: "name", GoType: "string", Required: true},
				// Three non-pointer reference fields - this would trigger the bug
				// if refMap is not scoped properly in ToMap()
				{Name: "Network", JsonName: "network", GoType: "NetworkConfig", IsRef: true, RefType: "NetworkConfig", IsPointer: false},
				{Name: "Storage", JsonName: "storage", GoType: "StorageConfig", IsRef: true, RefType: "StorageConfig", IsPointer: false},
				{Name: "Compute", JsonName: "compute", GoType: "ComputeConfig", IsRef: true, RefType: "ComputeConfig", IsPointer: false},
			},
		},
	}

	config := &Config{
		Name: "test-engine",
		Kind: KindMCPServer, Profile: "builder",
		Generate: GenerateConfig{
			PackageName: "main",
		},
	}

	got, err := GenerateSpecFileFromTypes(types, config, "sha256:multiref-test", nil)
	if err != nil {
		t.Fatalf("GenerateSpecFileFromTypes() error = %v", err)
	}

	code := string(got)

	// Verify all three ToMap reference handling blocks exist
	expectedPatterns := []string{
		"s.Network.ToMap()",
		"s.Storage.ToMap()",
		"s.Compute.ToMap()",
	}
	for _, pattern := range expectedPatterns {
		if !strings.Contains(code, pattern) {
			t.Errorf("Generated code missing pattern %q\nCode:\n%s", pattern, code)
		}
	}

	// The critical test: verify the generated code compiles
	// This would fail with "refMap redeclared in this block" if the bug exists
	fset := token.NewFileSet()
	_, parseErr := parser.ParseFile(fset, "spec.go", got, parser.AllErrors)
	if parseErr != nil {
		t.Errorf("Generated code does not compile (likely duplicate variable declaration bug): %v\nCode:\n%s", parseErr, code)
	}
}
