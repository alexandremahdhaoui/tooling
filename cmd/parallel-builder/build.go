/*
Copyright 2024 Alexandre Mahdhaoui

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/alexandremahdhaoui/forge/internal/mcpcaller"
	"github.com/alexandremahdhaoui/forge/pkg/forge"
	"github.com/alexandremahdhaoui/forge/pkg/mcptypes"
)

// ParallelBuilderSpec defines the input specification for parallel builds.
// This is kept for internal use as the generated Spec doesn't fully support nested object arrays.
type ParallelBuilderSpec struct {
	// Builders is the list of sub-builder configurations to run in parallel.
	Builders []BuilderConfig `json:"builders"`
}

// BuilderConfig defines a single sub-builder configuration.
type BuilderConfig struct {
	// Name is the optional name for this builder (used in logs/errors).
	Name string `json:"name,omitempty"`
	// Engine is the engine URI (e.g., "forge://build-go", "forge://generic-builder").
	Engine string `json:"engine"`
	// Spec contains the build specification passed to the sub-builder.
	Spec map[string]any `json:"spec"`
}

// Build implements the BuildFunc for executing multiple builders in parallel.
// It uses the typed Spec provided by the generated MCP server setup.
// Note: The generated Spec doesn't fully parse the builders array, so we parse
// the original input.Spec to get the complete ParallelBuilderSpec.
//
// The contract this engine received - platforms, force, frozen, the
// directories - is handed to every child unchanged, and the children's
// artifacts are answered as they are: this engine builds nothing itself and
// records nothing of its own.
func Build(ctx context.Context, input mcptypes.BuildInput, _ *Spec) ([]forge.Artifact, error) {
	// Parse spec - using manual ParallelBuilderSpec since generated Spec doesn't
	// support nested object arrays
	var pbSpec ParallelBuilderSpec
	if err := mapToStruct(input.Spec, &pbSpec); err != nil {
		return nil, fmt.Errorf("failed to parse spec: %w", err)
	}

	if len(pbSpec.Builders) == 0 {
		return nil, fmt.Errorf("no builders specified in spec.builders")
	}

	log.Printf("parallel-builder: starting %d parallel builds", len(pbSpec.Builders))

	// Create MCP caller
	caller := mcpcaller.NewCaller(Version)

	// Results channel
	type result struct {
		artifacts []forge.Artifact
		err       error
		name      string
	}
	results := make(chan result, len(pbSpec.Builders))

	// WaitGroup for goroutines
	var wg sync.WaitGroup

	// Launch parallel builders
	for _, builder := range pbSpec.Builders {
		wg.Add(1)
		go func(b BuilderConfig) {
			defer wg.Done()

			name := b.Name
			if name == "" {
				name = b.Engine
			}

			log.Printf("parallel-builder: starting build for %s", name)

			// Resolve engine
			command, args, err := caller.ResolveEngine(b.Engine)
			if err != nil {
				results <- result{err: fmt.Errorf("[%s] engine resolution failed: %w", name, err), name: name}
				return
			}

			// Call build tool with the contract this engine received on top of
			// what the child's own spec says.
			resp, err := caller.CallMCP(command, args, "build", childInput(input, b.Spec))
			if err != nil {
				results <- result{err: fmt.Errorf("[%s] build failed: %w", name, err), name: name}
				return
			}

			// Parse artifacts from response
			artifacts, err := parseArtifacts(resp)
			if err != nil {
				results <- result{err: fmt.Errorf("[%s] artifact parsing failed: %w", name, err), name: name}
				return
			}

			log.Printf("parallel-builder: completed build for %s", name)
			results <- result{artifacts: artifacts, name: name}
		}(builder)
	}

	// Wait and close channel
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	var artifacts []forge.Artifact
	var errors []error
	for r := range results {
		if r.err != nil {
			log.Printf("parallel-builder: error from %s: %v", r.name, r.err)
			errors = append(errors, r.err)
		} else {
			artifacts = append(artifacts, r.artifacts...)
		}
	}

	if len(errors) > 0 {
		errMsg := fmt.Sprintf("parallel-builder: %d/%d builders failed: ", len(errors), len(pbSpec.Builders))
		for i, err := range errors {
			if i > 0 {
				errMsg += "; "
			}
			errMsg += err.Error()
		}
		return nil, fmt.Errorf("%s", errMsg)
	}

	log.Printf("parallel-builder: all %d builds completed successfully", len(pbSpec.Builders))
	return artifacts, nil
}

// childInput is what a child build is handed: its own spec, then every
// field of the contract this engine received that the child's spec did
// not set. A child that names its own platforms keeps them; one that names
// none builds for the platforms this engine was asked for, so a parallel
// build is one build spread over engines and not a hole in the contract.
func childInput(input mcptypes.BuildInput, spec map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range spec {
		out[k] = v
	}

	inherited := map[string]any{
		"platforms": input.Platforms,
		"force":     input.Force,
		"frozen":    input.Frozen,
		"tmpDir":    input.TmpDir,
		"buildDir":  input.BuildDir,
		"rootDir":   input.RootDir,
	}

	for k, v := range inherited {
		if _, set := out[k]; !set {
			out[k] = v
		}
	}

	return out
}

// parseArtifacts reads what a child build answered: the artifacts list a
// build answers, or one artifact from an engine of an older contract.
func parseArtifacts(resp interface{}) ([]forge.Artifact, error) {
	if resp == nil {
		return nil, fmt.Errorf("the build answered no artifact")
	}

	data, err := json.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	var output mcptypes.BuildOutput
	if err := json.Unmarshal(data, &output); err == nil && len(output.Artifacts) > 0 {
		return output.Artifacts, nil
	}

	var single forge.Artifact
	if err := json.Unmarshal(data, &single); err == nil && single.Name != "" {
		return []forge.Artifact{single}, nil
	}

	return nil, fmt.Errorf("could not parse artifacts from response: %s", string(data))
}

// mapToStruct converts a map to a struct using JSON marshal/unmarshal.
func mapToStruct(m map[string]any, v interface{}) error {
	if m == nil {
		return nil
	}

	data, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("failed to marshal map: %w", err)
	}

	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("failed to unmarshal to struct: %w", err)
	}

	return nil
}
