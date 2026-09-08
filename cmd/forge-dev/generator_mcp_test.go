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

func TestGenerateMCPFile_Builder(t *testing.T) {
	config := &Config{
		Name: "test-builder",
		Kind: KindMCPServer, Profile: "builder",
		Version: "1.0.0",
		Generate: GenerateConfig{
			PackageName: "main",
		},
	}

	got, err := GenerateMCPFile(config, "sha256:builder123", nil)
	if err != nil {
		t.Fatalf("GenerateMCPFile() error = %v", err)
	}

	code := string(got)

	// Check for builder-specific content
	wantContains := []string{
		"type BuildFunc func(",
		"func SetupMCPServer(",
		"buildFn BuildFunc",
		"func wrapBuildFunc(",
		"engineframework.BuilderFunc",
		"engineframework.BuilderConfig",
		"mcptypes.BuildInput",
		"[]forge.Artifact",
		"var Capabilities = engineframework.Capabilities{Platforms: nil, Frozen: false}",
		"engineframework.RefusePlatforms(\"test-builder\"",
		"handleConfigValidate",
		"sha256:builder123",
	}

	for _, want := range wantContains {
		if !strings.Contains(code, want) {
			t.Errorf("Generated builder code missing %q", want)
		}
	}

	// Verify the generated code compiles
	fset := token.NewFileSet()
	_, parseErr := parser.ParseFile(fset, "mcp.go", got, parser.AllErrors)
	if parseErr != nil {
		t.Errorf("Generated builder code does not compile: %v\nCode:\n%s", parseErr, code)
	}
}

func TestGenerateMCPFile_TestRunner(t *testing.T) {
	config := &Config{
		Name: "test-runner",
		Kind: KindMCPServer, Profile: "test-runner",
		Version: "1.0.0",
		Generate: GenerateConfig{
			PackageName: "main",
		},
	}

	got, err := GenerateMCPFile(config, "sha256:runner456", nil)
	if err != nil {
		t.Fatalf("GenerateMCPFile() error = %v", err)
	}

	code := string(got)

	// Check for test-runner-specific content
	wantContains := []string{
		"type TestRunnerFunc func(",
		"func SetupMCPServer(",
		"runFn TestRunnerFunc",
		"func wrapTestRunnerFunc(",
		"engineframework.TestRunnerFunc",
		"engineframework.TestRunnerConfig",
		"mcptypes.RunInput",
		"*forge.TestReport",
		"handleConfigValidate",
		"sha256:runner456",
	}

	for _, want := range wantContains {
		if !strings.Contains(code, want) {
			t.Errorf("Generated test-runner code missing %q", want)
		}
	}

	// Verify the generated code compiles
	fset := token.NewFileSet()
	_, parseErr := parser.ParseFile(fset, "mcp.go", got, parser.AllErrors)
	if parseErr != nil {
		t.Errorf("Generated test-runner code does not compile: %v\nCode:\n%s", parseErr, code)
	}
}

func TestGenerateMCPFile_TestEnvSubengine(t *testing.T) {
	config := &Config{
		Name: "testenv-kind",
		Kind: KindMCPServer, Profile: "testenv-subengine",
		Version: "1.0.0",
		Generate: GenerateConfig{
			PackageName: "main",
		},
	}

	got, err := GenerateMCPFile(config, "sha256:testenv789", nil)
	if err != nil {
		t.Fatalf("GenerateMCPFile() error = %v", err)
	}

	code := string(got)

	// Check for testenv-subengine-specific content
	wantContains := []string{
		"type CreateFunc func(",
		"type DeleteFunc func(",
		"func SetupMCPServer(",
		"createFn CreateFunc",
		"deleteFn DeleteFunc",
		"func wrapCreateFunc(",
		"func wrapDeleteFunc(",
		"engineframework.CreateFunc",
		"engineframework.DeleteFunc",
		"engineframework.CreateInput",
		"engineframework.DeleteInput",
		"engineframework.TestEnvArtifact",
		"engineframework.TestEnvSubengineConfig",
		"handleConfigValidate",
		"sha256:testenv789",
	}

	for _, want := range wantContains {
		if !strings.Contains(code, want) {
			t.Errorf("Generated testenv-subengine code missing %q", want)
		}
	}

	// Verify the generated code compiles
	fset := token.NewFileSet()
	_, parseErr := parser.ParseFile(fset, "mcp.go", got, parser.AllErrors)
	if parseErr != nil {
		t.Errorf("Generated testenv-subengine code does not compile: %v\nCode:\n%s", parseErr, code)
	}
}

func TestGenerateMCPFile_DependencyDetector(t *testing.T) {
	config := &Config{
		Name: "go-dependency-detector",
		Kind: KindMCPServer, Profile: "dependency-detector",
		Version: "1.0.0",
		Generate: GenerateConfig{
			PackageName: "main",
		},
	}

	got, err := GenerateMCPFile(config, "sha256:depdetector123", nil)
	if err != nil {
		t.Fatalf("GenerateMCPFile() error = %v", err)
	}

	code := string(got)

	// Check for dependency-detector-specific content
	wantContains := []string{
		"func SetupMCPServerBase(",
		"name string, version string",
		"mcpserver.New(name, version)",
		"config-validate",
		"handleConfigValidate",
		"ValidateMap(input.Spec)",
		"sha256:depdetector123",
	}

	for _, want := range wantContains {
		if !strings.Contains(code, want) {
			t.Errorf("Generated dependency-detector code missing %q", want)
		}
	}

	// Verify it does NOT contain SetupMCPServer (only SetupMCPServerBase)
	// The dependency detector template should have SetupMCPServerBase, not SetupMCPServer
	// We need to check that it's the Base variant
	if strings.Contains(code, "func SetupMCPServer(") && !strings.Contains(code, "func SetupMCPServerBase(") {
		t.Error("Generated dependency-detector code should have SetupMCPServerBase, not SetupMCPServer")
	}

	// Verify the generated code compiles
	fset := token.NewFileSet()
	_, parseErr := parser.ParseFile(fset, "mcp.go", got, parser.AllErrors)
	if parseErr != nil {
		t.Errorf("Generated dependency-detector code does not compile: %v\nCode:\n%s", parseErr, code)
	}
}

func TestGenerateMCPFile_InvalidType(t *testing.T) {
	config := &Config{
		Name:    "invalid-engine",
		Kind:    KindMCPServer,
		Profile: "invalid",
		Version: "1.0.0",
		Generate: GenerateConfig{
			PackageName: "main",
		},
	}

	_, err := GenerateMCPFile(config, "sha256:invalid", nil)
	if err == nil {
		t.Error("GenerateMCPFile() expected error for invalid engine type")
	}
	if !strings.Contains(err.Error(), "unsupported engine type") {
		t.Errorf("GenerateMCPFile() error = %v, want error containing 'unsupported engine type'", err)
	}
}

func TestMcpTemplateName(t *testing.T) {
	tests := []struct {
		engineType EngineType
		want       string
		wantErr    bool
	}{
		{EngineTypeBuilder, "mcp_builder.go.tmpl", false},
		{EngineTypeTestRunner, "mcp_testrunner.go.tmpl", false},
		{EngineTypeTestEnvSubengine, "mcp_testenv.go.tmpl", false},
		{EngineTypeDependencyDetector, "mcp_dependency_detector.go.tmpl", false},
		{EngineType("invalid"), "", true},
	}

	for _, tt := range tests {
		t.Run(string(tt.engineType), func(t *testing.T) {
			got, err := mcpTemplateName(tt.engineType)
			if (err != nil) != tt.wantErr {
				t.Errorf("mcpTemplateName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("mcpTemplateName() = %v, want %v", got, tt.want)
			}
		})
	}
}

// A builder that declares frozen carries it into its generated
// Capabilities, so the framework admits the input and config-validate
// answers the declaration. One that declares nothing carries false.
func TestABuilderCarriesItsFrozenDeclaration(t *testing.T) {
	config := &Config{
		Name: "frozen-builder",
		Kind: KindMCPServer, Profile: "builder",
		Version:      "1.0.0",
		Capabilities: &CapabilitiesConfig{Platforms: PlatformsConfig{"any"}, Frozen: true},
		Generate:     GenerateConfig{PackageName: "main"},
	}

	got, err := GenerateMCPFile(config, "sha256:frozen", nil)
	if err != nil {
		t.Fatalf("GenerateMCPFile() error = %v", err)
	}

	if !strings.Contains(string(got), `engineframework.Capabilities{Platforms: []string{"any"}, Frozen: true}`) {
		t.Fatalf("the frozen declaration must reach the generated Capabilities:\n%s", got)
	}

	if !strings.Contains(string(got), "output.Capabilities = Capabilities.Declaration()") {
		t.Fatal("config-validate must answer the declaration")
	}
}
