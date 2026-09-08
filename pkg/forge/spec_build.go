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

package forge

import (
	"fmt"
	"strings"
)

// Build holds the list of artifacts to build
type Build []BuildSpec

// BuildSpec represents a single artifact to build
type BuildSpec struct {
	// Name of the artifact to build
	Name string `json:"name"`
	// Path to the source, e.g.:
	// - ./cmd/<NAME>
	// - ./containers/<NAME>/Containerfile
	Src string `json:"src"`
	// The destination of the artifact, e.g.:
	// - "./build/bin/<NAME>"
	// - can be left empty for container images
	Dest string `json:"dest,omitempty"`
	// Context is the build context directory or git URL. Empty defaults to "."
	Context string `json:"context,omitempty"`
	// Engine that will build this artifact, e.g.:
	// - forge://container-build (forge://github.com/alexandremahdhaoui/forge/cmd/container-build)
	// - forge://go-build        (forge://github.com/alexandremahdhaoui/forge/cmd/go-build)
	Engine string `json:"engine"`
	// Platforms names the os/arch pairs this artifact builds for, e.g.
	// ["linux/amd64", "linux/arm64"]. Every build builds all of them, each
	// recorded as its own artifact carrying its os and arch; absent means
	// the machine forge runs on. Nothing narrows or widens the list: there
	// is no flag, the declaration is the whole of the policy. It is also the
	// declaration that this artifact is PUBLIC: what a release ships is what
	// carries a platform, and a repo's own tool that declares none stays
	// home. The engine refuses a platform it cannot build.
	Platforms []string `json:"platforms,omitempty"`
	// Spec contains engine-specific configuration (free-form)
	// Supports fields like: command, args, env, envFile, context
	// For container-build engine, also supports:
	//   - dependsOn: []DependsOnSpec - list of dependency detectors to run
	// The exact fields supported depend on the engine being used
	Spec map[string]interface{} `json:"spec,omitempty"`
}

// DependsOnSpec defines a dependency detector configuration
type DependsOnSpec struct {
	// Engine is the URI of the dependency-detector engine (e.g., "forge://go-dependency-detector")
	Engine string `json:"engine"`

	// Spec contains engine-specific configuration for the dependency detector
	Spec map[string]interface{} `json:"spec,omitempty"`
}

// Validate validates the BuildSpec
func (bs *BuildSpec) Validate() error {
	errs := NewValidationErrors()

	// Validate required fields
	if err := ValidateRequired(bs.Name, "name", "BuildSpec"); err != nil {
		errs.Add(err)
	}
	if err := ValidateRequired(bs.Src, "src", "BuildSpec"); err != nil {
		errs.Add(err)
	}

	// Validate engine URI
	if err := ValidateURI(bs.Engine, "BuildSpec.engine"); err != nil {
		errs.Add(err)
	}

	// A platform is os/arch and nothing else: the name a binary travels
	// under is built from it, so a malformed one would ship silently.
	for _, platform := range bs.Platforms {
		if parts := strings.Split(platform, "/"); len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			errs.Add(fmt.Errorf(
				"BuildSpec.platforms: %q is not <os>/<arch> (e.g. linux/amd64)", platform))
		}
	}

	return errs.ErrorOrNil()
}

// ParseDependsOn extracts and validates dependsOn field from BuildSpec.Spec
// Returns nil slice if dependsOn not present (no error)
// Returns error if dependsOn present but invalid structure
func ParseDependsOn(spec map[string]interface{}) ([]DependsOnSpec, error) {
	dependsOnRaw, ok := spec["dependsOn"]
	if !ok {
		return nil, nil // not present, not an error
	}

	dependsOnSlice, ok := dependsOnRaw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("dependsOn must be an array, got %T", dependsOnRaw)
	}

	result := make([]DependsOnSpec, 0, len(dependsOnSlice))
	for i, item := range dependsOnSlice {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("dependsOn[%d] must be an object, got %T", i, item)
		}

		engine, ok := itemMap["engine"].(string)
		if !ok || engine == "" {
			return nil, fmt.Errorf("dependsOn[%d].engine is required and must be a string", i)
		}

		specMap, _ := itemMap["spec"].(map[string]interface{})

		result = append(result, DependsOnSpec{
			Engine: engine,
			Spec:   specMap,
		})
	}

	return result, nil
}
