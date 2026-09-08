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
	"testing"

	"github.com/alexandremahdhaoui/forge/pkg/forge"
	"github.com/alexandremahdhaoui/forge/pkg/mcptypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// -----------------------------------------------------------------------------
// Tests for childInput and parseArtifacts
// -----------------------------------------------------------------------------

// The contract this engine received reaches every child: a child that names
// nothing builds for the platforms the parent was asked for, frozen the same
// way, in the same directories. A child that names its own platforms keeps
// them - the child's spec is the child's word.
func TestChildInputInheritsTheContract(t *testing.T) {
	parent := mcptypes.BuildInput{
		Platforms: []string{"linux/amd64", "linux/arm64"},
		Frozen:    true,
		Force:     true,
		DirectoryParams: mcptypes.DirectoryParams{
			TmpDir: "/tmp/x", BuildDir: "/build", RootDir: "/root",
		},
	}

	got := childInput(parent, map[string]any{"name": "child", "engine": "forge://go-build"}, true)

	assert.Equal(t, []string{"linux/amd64", "linux/arm64"}, got["platforms"])
	assert.Equal(t, true, got["frozen"])
	assert.Equal(t, true, got["force"])
	assert.Equal(t, "/tmp/x", got["tmpDir"])
	assert.Equal(t, "/build", got["buildDir"])
	assert.Equal(t, "/root", got["rootDir"])
	assert.Equal(t, "child", got["name"])

	own := childInput(parent, map[string]any{"name": "child", "platforms": []any{"linux/amd64"}}, true)
	assert.Equal(t, []any{"linux/amd64"}, own["platforms"], "a child's own platforms win")
}

// A child that declares no frozen capability is handed no frozen input:
// it would refuse one by name, and a parallel build must not turn a
// generator into a refusal because its sibling is a compiler.
func TestAChildThatDeclaresNoFrozenIsHandedNone(t *testing.T) {
	parent := mcptypes.BuildInput{Platforms: []string{"linux/amd64"}, Frozen: true}

	got := childInput(parent, map[string]any{"name": "child", "engine": "forge://go-gen-mocks"}, false)

	_, sent := got["frozen"]
	assert.False(t, sent, "frozen must not reach a child that never declared it")
	assert.Equal(t, []string{"linux/amd64"}, got["platforms"])
}

func TestParseArtifacts_NilResponseIsAnError(t *testing.T) {
	_, err := parseArtifacts(nil)
	require.Error(t, err)
}

func TestParseArtifacts_ReadsTheBuildOutputList(t *testing.T) {
	resp := map[string]any{
		"artifacts": []any{
			map[string]any{"name": "a", "type": "binary", "os": "linux", "arch": "amd64", "location": "/build/bin/a", "version": "v1"},
			map[string]any{"name": "a", "type": "binary", "os": "linux", "arch": "arm64", "location": "/build/bin/a_linux_arm64", "version": "v1"},
		},
	}

	got, err := parseArtifacts(resp)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, forge.TypeBinary, got[0].Type)
	assert.Equal(t, "linux/arm64", got[1].Platform())
}

func TestParseArtifacts_ReadsOneArtifactOfTheOlderShape(t *testing.T) {
	got, err := parseArtifacts(map[string]any{"name": "one", "type": "generated", "location": "."})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "one", got[0].Name)
}

func TestParseArtifacts_UnreadableIsAnError(t *testing.T) {
	_, err := parseArtifacts(map[string]any{"nothing": "here"})
	require.Error(t, err)
}

// -----------------------------------------------------------------------------
// Tests for mapToStruct
// -----------------------------------------------------------------------------

func TestMapToStruct_NilMap(t *testing.T) {
	var spec ParallelBuilderSpec
	err := mapToStruct(nil, &spec)

	assert.NoError(t, err)
	// spec should remain at zero value
	assert.Empty(t, spec.Builders)
}

func TestMapToStruct_ValidMap(t *testing.T) {
	input := map[string]any{
		"builders": []any{
			map[string]any{
				"name":   "builder-1",
				"engine": "forge://go-build",
				"spec": map[string]any{
					"src":  "./cmd/app",
					"dest": "./build/bin",
				},
			},
			map[string]any{
				"name":   "builder-2",
				"engine": "forge://go-build",
				"spec": map[string]any{
					"src":  "./cmd/tool",
					"dest": "./build/bin",
				},
			},
		},
	}

	var spec ParallelBuilderSpec
	err := mapToStruct(input, &spec)

	require.NoError(t, err)
	require.Len(t, spec.Builders, 2)

	assert.Equal(t, "builder-1", spec.Builders[0].Name)
	assert.Equal(t, "forge://go-build", spec.Builders[0].Engine)
	assert.Equal(t, "./cmd/app", spec.Builders[0].Spec["src"])
	assert.Equal(t, "./build/bin", spec.Builders[0].Spec["dest"])

	assert.Equal(t, "builder-2", spec.Builders[1].Name)
	assert.Equal(t, "forge://go-build", spec.Builders[1].Engine)
	assert.Equal(t, "./cmd/tool", spec.Builders[1].Spec["src"])
	assert.Equal(t, "./build/bin", spec.Builders[1].Spec["dest"])
}

func TestMapToStruct_EmptyBuildersArray(t *testing.T) {
	input := map[string]any{
		"builders": []any{},
	}

	var spec ParallelBuilderSpec
	err := mapToStruct(input, &spec)

	require.NoError(t, err)
	assert.Empty(t, spec.Builders)
}

func TestMapToStruct_BuilderWithoutName(t *testing.T) {
	input := map[string]any{
		"builders": []any{
			map[string]any{
				"engine": "forge://go-build",
				"spec": map[string]any{
					"src": "./cmd/app",
				},
			},
		},
	}

	var spec ParallelBuilderSpec
	err := mapToStruct(input, &spec)

	require.NoError(t, err)
	require.Len(t, spec.Builders, 1)
	assert.Empty(t, spec.Builders[0].Name)
	assert.Equal(t, "forge://go-build", spec.Builders[0].Engine)
}

func TestMapToStruct_InvalidTarget(t *testing.T) {
	input := map[string]any{
		"builders": []any{
			map[string]any{
				"engine": "forge://go-build",
			},
		},
	}

	// Try to unmarshal into a non-pointer
	var str string
	err := mapToStruct(input, &str)

	// json.Unmarshal will return an error for mismatched types
	assert.Error(t, err)
}

func TestMapToStruct_BuilderConfig(t *testing.T) {
	// Test that individual BuilderConfig fields are correctly parsed
	input := map[string]any{
		"builders": []any{
			map[string]any{
				"name":   "test-builder",
				"engine": "forge://parallel-builder",
				"spec": map[string]any{
					"nested": map[string]any{
						"key": "value",
					},
					"array": []any{1, 2, 3},
				},
			},
		},
	}

	var spec ParallelBuilderSpec
	err := mapToStruct(input, &spec)

	require.NoError(t, err)
	require.Len(t, spec.Builders, 1)

	builder := spec.Builders[0]
	assert.Equal(t, "test-builder", builder.Name)
	assert.Equal(t, "forge://parallel-builder", builder.Engine)

	// Check nested spec values
	nested, ok := builder.Spec["nested"].(map[string]any)
	require.True(t, ok, "nested should be a map")
	assert.Equal(t, "value", nested["key"])

	arr, ok := builder.Spec["array"].([]any)
	require.True(t, ok, "array should be a slice")
	assert.Len(t, arr, 3)
}

// -----------------------------------------------------------------------------
// Tests for ParallelBuilderSpec and BuilderConfig types
// -----------------------------------------------------------------------------

func TestParallelBuilderSpec_ZeroValue(t *testing.T) {
	var spec ParallelBuilderSpec
	assert.Nil(t, spec.Builders)
}

func TestBuilderConfig_ZeroValue(t *testing.T) {
	var config BuilderConfig
	assert.Empty(t, config.Name)
	assert.Empty(t, config.Engine)
	assert.Nil(t, config.Spec)
}
