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
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge/pkg/mcptypes"
	"github.com/stretchr/testify/require"
)

const cellLocalConfigSpec = `openapi: 3.0.3
info:
  title: cell local Spec Schema
  version: 0.1.0
components:
  schemas:
    Spec:
      type: object
      properties:
        greetingStore:
          type: string
          description: The chosen store.
`

// twoStubCaller answers the main generator first and the config generator
// second, keeping the model each of them received.
type twoStubCaller struct {
	answers []map[string]interface{}
	models  []GeneratorModel
	calls   int
}

func (s *twoStubCaller) ResolveEngine(_ string) (string, []string, error) {
	return "stub", nil, nil
}

func (s *twoStubCaller) CallMCP(_ string, _ []string, _ string, params interface{}) (interface{}, error) {
	if m, ok := params.(GeneratorModel); ok {
		s.models = append(s.models, m)
	}

	answer := s.answers[s.calls]
	s.calls++

	return answer, nil
}

func withTwoStubs(t *testing.T, stub *twoStubCaller) {
	t.Helper()

	previous := newGeneratorCaller
	newGeneratorCaller = func() generatorCaller { return stub }
	t.Cleanup(func() { newGeneratorCaller = previous })
}

func hexagonalCellYaml(configGeneratorBlock string) string {
	return `name: fixture-gui
kind: gui
generator: forge://example.com/org/gui-gen/engines/gui-gen
version: 0.1.0
language: rust
openapi:
  specPath: ./spec.openapi.yaml
generate:
  packageName: main
  docsBaseURL: https://example.com/raw
` + configGeneratorBlock
}

func TestACellRunsItsConfigGeneratorAfterItsOwnGenerator(t *testing.T) {
	dir := t.TempDir()
	writeKindFixture(t, dir, hexagonalCellYaml(
		"configGenerator: forge://example.com/org/configgen/cmd/configgen-gen\n"))

	stub := &twoStubCaller{answers: []map[string]interface{}{
		answerWith("zz_generated_lib.rs"),
		answerWith("zz_generated_config.rs"),
	}}
	withTwoStubs(t, stub)

	_, err := generate(context.Background(), mcptypes.BuildInput{Name: "fixture-gui", Src: dir, Engine: "forge://forge-dev"})
	require.NoError(t, err)

	require.Equal(t, 2, stub.calls)
	require.FileExists(t, filepath.Join(dir, "zz_generated_lib.rs"))
	require.FileExists(t, filepath.Join(dir, "zz_generated_config.rs"))
}

func TestTheConfigGeneratorAnswerJoinsTheRecordedListInsteadOfReplacingIt(t *testing.T) {
	dir := t.TempDir()
	writeKindFixture(t, dir, hexagonalCellYaml(
		"configGenerator: forge://example.com/org/configgen/cmd/configgen-gen\n"))

	stub := &twoStubCaller{answers: []map[string]interface{}{
		answerWith("zz_generated_lib.rs"),
		answerWith("zz_generated_config.rs"),
	}}
	withTwoStubs(t, stub)

	_, err := generate(context.Background(), mcptypes.BuildInput{Name: "fixture-gui", Src: dir, Engine: "forge://forge-dev"})
	require.NoError(t, err)

	recorded, err := ReadGeneratedFiles(filepath.Join(dir, GeneratedRunnableFile))
	require.NoError(t, err)
	require.Equal(t, []string{"zz_generated_lib.rs", "zz_generated_config.rs"}, recorded)
}

func TestTheMainGeneratorFilesSurviveASecondRunWithAConfigGenerator(t *testing.T) {
	dir := t.TempDir()
	writeKindFixture(t, dir, hexagonalCellYaml(
		"configGenerator: forge://example.com/org/configgen/cmd/configgen-gen\n"))

	answers := []map[string]interface{}{
		answerWith("zz_generated_lib.rs"),
		answerWith("zz_generated_config.rs"),
	}

	withTwoStubs(t, &twoStubCaller{answers: answers})
	_, err := generate(context.Background(), mcptypes.BuildInput{Name: "fixture-gui", Src: dir, Engine: "forge://forge-dev"})
	require.NoError(t, err)

	withTwoStubs(t, &twoStubCaller{answers: answers})
	_, err = generate(context.Background(), mcptypes.BuildInput{Name: "fixture-gui", Src: dir, Engine: "forge://forge-dev"})
	require.NoError(t, err)

	require.FileExists(t, filepath.Join(dir, "zz_generated_lib.rs"))
	require.FileExists(t, filepath.Join(dir, "zz_generated_config.rs"))
}

func TestASecondRunOfACellWithAConfigGeneratorRemovesNothing(t *testing.T) {
	dir := t.TempDir()
	writeKindFixture(t, dir, hexagonalCellYaml(
		"configGenerator: forge://example.com/org/configgen/cmd/configgen-gen\n"))

	answers := []map[string]interface{}{
		answerWith("zz_generated_lib.rs"),
		answerWith("zz_generated_config.rs"),
	}

	withTwoStubs(t, &twoStubCaller{answers: answers})
	_, err := generate(context.Background(), mcptypes.BuildInput{Name: "fixture-gui", Src: dir, Engine: "forge://forge-dev"})
	require.NoError(t, err)

	var logged strings.Builder

	previousOutput := log.Writer()
	log.SetOutput(&logged)
	t.Cleanup(func() { log.SetOutput(previousOutput) })

	withTwoStubs(t, &twoStubCaller{answers: answers})
	_, err = generate(context.Background(), mcptypes.BuildInput{Name: "fixture-gui", Src: dir, Engine: "forge://forge-dev"})
	require.NoError(t, err)

	require.NotContains(t, logged.String(), "removed the stale generated file")
}

func TestTheConfigGeneratorReadsTheCellLocalSchemaWhenTheMainGeneratorWroteOne(t *testing.T) {
	dir := t.TempDir()
	writeKindFixture(t, dir, hexagonalCellYaml(
		"configGenerator: forge://example.com/org/configgen/cmd/configgen-gen\n"))

	require.NoError(t, os.WriteFile(
		filepath.Join(dir, GeneratedConfigSpecFile), []byte(cellLocalConfigSpec), 0o644))

	stub := &twoStubCaller{answers: []map[string]interface{}{
		answerWith("zz_generated_lib.rs"),
		answerWith("zz_generated_config.rs"),
	}}
	withTwoStubs(t, stub)

	_, err := generate(context.Background(), mcptypes.BuildInput{Name: "fixture-gui", Src: dir, Engine: "forge://forge-dev"})
	require.NoError(t, err)

	require.Contains(t, stub.models[0].OpenAPISpec, "greeting")
	require.Equal(t, cellLocalConfigSpec, stub.models[1].OpenAPISpec)
}

func TestTheConfigGeneratorFallsBackToTheCellSpecWhenThereIsNoLocalSchema(t *testing.T) {
	dir := t.TempDir()
	writeKindFixture(t, dir, hexagonalCellYaml(
		"configGenerator: forge://example.com/org/configgen/cmd/configgen-gen\n"))

	stub := &twoStubCaller{answers: []map[string]interface{}{
		answerWith("zz_generated_lib.rs"),
		answerWith("zz_generated_config.rs"),
	}}
	withTwoStubs(t, stub)

	_, err := generate(context.Background(), mcptypes.BuildInput{Name: "fixture-gui", Src: dir, Engine: "forge://forge-dev"})
	require.NoError(t, err)

	require.Equal(t, stub.models[0].OpenAPISpec, stub.models[1].OpenAPISpec)
	require.Contains(t, stub.models[1].OpenAPISpec, "greeting")
}

func TestTheConfigGeneratorReceivesIdentityAndTheSpecSchemaAndNeitherLayoutNorWiringNorProto(t *testing.T) {
	dir := t.TempDir()
	writeKindFixture(t, dir, customKindYaml()+
		"proto:\n  specPath: ./hello.v1.proto\n"+
		"wiring:\n  specPath: ./wiring.yaml\n"+
		"configGenerator: forge://example.com/org/configgen/cmd/configgen-gen\n")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "hello.v1.proto"), []byte(smallProto), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "wiring.yaml"), []byte(smallWiring), 0o644))

	stub := &twoStubCaller{answers: []map[string]interface{}{
		answerWith("zz_generated_lib.rs"),
		answerWith("zz_generated_config.rs"),
	}}
	withTwoStubs(t, stub)

	_, err := generate(context.Background(), mcptypes.BuildInput{Name: "fixture-gui", Src: dir, Engine: "forge://forge-dev"})
	require.NoError(t, err)
	require.Len(t, stub.models, 2)

	main, config := stub.models[0], stub.models[1]

	require.NotNil(t, main.Layout)
	require.Equal(t, smallProto, main.ProtoSpec)
	require.Equal(t, smallWiring, main.WiringSpec)

	require.Equal(t, "fixture-gui", config.Name)
	require.Nil(t, config.Layout)
	require.Empty(t, config.ProtoSpec)
	require.Empty(t, config.WiringSpec)
	require.NotEmpty(t, config.OpenAPISpec)
}

func TestTheConfigGeneratorFilesLandUnderTheOutputDirectoryTheCellNames(t *testing.T) {
	dir := t.TempDir()
	writeKindFixture(t, dir, hexagonalCellYaml(
		"configGenerator:\n  engine: forge://example.com/org/configgen/cmd/configgen-gen\n  outputDir: src/config\n"))

	stub := &twoStubCaller{answers: []map[string]interface{}{
		answerWith("zz_generated_lib.rs"),
		answerWith("zz_generated_config.rs"),
	}}
	withTwoStubs(t, stub)

	_, err := generate(context.Background(), mcptypes.BuildInput{Name: "fixture-gui", Src: dir, Engine: "forge://forge-dev"})
	require.NoError(t, err)

	require.FileExists(t, filepath.Join(dir, "src", "config", "zz_generated_config.rs"))
	require.NoFileExists(t, filepath.Join(dir, "zz_generated_config.rs"))

	recorded, err := ReadGeneratedFiles(filepath.Join(dir, GeneratedRunnableFile))
	require.NoError(t, err)
	require.Contains(t, recorded, "src/config/zz_generated_config.rs")
}

func TestAPlainStringConfigGeneratorStillReadsAsTheEngineWithNoOutputDirectory(t *testing.T) {
	dir := t.TempDir()
	writeForgeDevYaml(t, dir, hexagonalCellYaml(
		"configGenerator: forge://example.com/org/configgen/cmd/configgen-gen\n"))

	config, err := ReadConfig(dir)

	require.NoError(t, err)
	require.Equal(t, "forge://example.com/org/configgen/cmd/configgen-gen", config.ConfigGenerator.Engine)
	require.Empty(t, config.ConfigGenerator.OutputDir)
}

func TestAConfigGeneratorPathThatIsNotNamedZzGeneratedFailsTheSameWay(t *testing.T) {
	dir := t.TempDir()
	writeKindFixture(t, dir, hexagonalCellYaml(
		"configGenerator: forge://example.com/org/configgen/cmd/configgen-gen\n"))

	stub := &twoStubCaller{answers: []map[string]interface{}{
		answerWith("zz_generated_lib.rs"),
		answerWith("config.rs"),
	}}
	withTwoStubs(t, stub)

	_, err := generate(context.Background(), mcptypes.BuildInput{Name: "fixture-gui", Src: dir, Engine: "forge://forge-dev"})

	require.Error(t, err)
	require.Contains(t, err.Error(), "config.rs is not named zz_generated")
	require.Contains(t, err.Error(), "forge://example.com/org/configgen/cmd/configgen-gen")
}

func TestTheRestAPIKindRunsTheConfigGeneratorItNames(t *testing.T) {
	dir := t.TempDir()
	createRequiredDocs(t, dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "forge-dev.yaml"),
		[]byte(restFixtureYaml()+"configGenerator: forge://example.com/org/configgen/cmd/configgen-gen\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.openapi.yaml"), []byte(restSpec), 0o644))

	stub := &twoStubCaller{answers: []map[string]interface{}{
		answerWith("zz_generated_config.go"),
	}}
	withTwoStubs(t, stub)

	_, err := generate(context.Background(), mcptypes.BuildInput{Name: "fixture-rest", Src: dir, Engine: "forge://forge-dev"})
	require.NoError(t, err)

	require.Equal(t, 1, stub.calls)
	require.FileExists(t, filepath.Join(dir, "zz_generated_config.go"))
}

func TestAnOutputDirectoryThatEscapesTheEngineDirectoryFailsValidationNamingIt(t *testing.T) {
	for _, outputDir := range []string{"/etc/config", "../outside"} {
		config := &Config{
			Name:            "fixture-gui",
			Kind:            KindBinary,
			Version:         "0.1.0",
			OpenAPI:         OpenAPIConfig{SpecPath: "./spec.openapi.yaml"},
			Generate:        GenerateConfig{PackageName: "main"},
			ConfigGenerator: ConfigGeneratorConfig{Engine: "forge://example.com/org/configgen/cmd/configgen-gen", OutputDir: outputDir},
		}

		errs := ValidateConfig(config)

		var found bool
		for _, e := range errs {
			if e.Field == "configGenerator.outputDir" && strings.Contains(e.Message, outputDir) {
				found = true
			}
		}
		require.True(t, found, "%s must be refused by name, got %+v", outputDir, errs)
	}
}

func TestAnOutputDirectoryInsideTheEngineDirectoryPassesValidation(t *testing.T) {
	config := &Config{
		Name:            "fixture-gui",
		Kind:            KindBinary,
		Version:         "0.1.0",
		OpenAPI:         OpenAPIConfig{SpecPath: "./spec.openapi.yaml"},
		Generate:        GenerateConfig{PackageName: "main"},
		ConfigGenerator: ConfigGeneratorConfig{Engine: "forge://example.com/org/configgen/cmd/configgen-gen", OutputDir: "src/config"},
	}

	for _, e := range ValidateConfig(config) {
		require.NotEqual(t, "configGenerator.outputDir", e.Field)
	}
}
