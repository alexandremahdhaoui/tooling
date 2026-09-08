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
	"errors"
	"fmt"
	"os"

	"github.com/alexandremahdhaoui/forge/pkg/flaterrors"

	"sigs.k8s.io/yaml"
)

const (
	// ConfigPath is the default path to the forge configuration file.
	ConfigPath = "forge.yaml"
)

// Spec represents the forge configuration.
// It is read from the forge.yaml file.
type Spec struct {
	// Name is the name of the project.
	Name string `json:"name"`

	// EnvFile is the path to a global environment file to source before any operations
	EnvFile string `json:"envFile"`

	// Path to the artifact store. The artifact store is a yaml data structures that
	// tracks the name, timestamp etc of all built artifacts
	ArtifactStorePath string `json:"artifactStorePath"`

	// Build holds the build configuration
	Build Build `json:"build"`

	// Test holds the test stage configurations
	Test []TestSpec `json:"test"`

	// Run holds the runnable target configurations
	Run []RunSpec `json:"run,omitempty"`

	// Engines holds custom engine configurations with aliases
	Engines []EngineConfig `json:"engines,omitempty"`

	// CU holds continuous-update configuration for forge-cu.
	CU *CUConfig `json:"cu,omitempty"`

	// Frozen says a build reads the recorded dependency locks and never
	// repairs them: a stale lock fails the build instead of self-healing
	// into bytes nobody can reproduce. Absent means true - a build is a
	// real build unless the repo says otherwise. It reaches only the build
	// engines that declare `capabilities.frozen` in their forge-dev.yaml;
	// every build engine holds the invariant that a build writes no lockfile
	// whether or not it reads this.
	Frozen *bool `json:"frozen,omitempty"`
}

// FrozenBuild answers the repo's frozen setting, true when it declares none.
func (s *Spec) FrozenBuild() bool {
	return s.Frozen == nil || *s.Frozen
}

// Validate validates the Spec
func (s *Spec) Validate() error {
	errs := NewValidationErrors()

	// Validate required fields
	if err := ValidateRequired(s.Name, "name", "Spec"); err != nil {
		errs.Add(err)
	}
	if err := ValidateRequired(s.ArtifactStorePath, "artifactStorePath", "Spec"); err != nil {
		errs.Add(err)
	}
	if err := ValidateRequired(s.EnvFile, "envFile", "Spec"); err != nil {
		errs.Add(err)
	}

	// Validate all build specs
	for i, bs := range s.Build {
		if err := bs.Validate(); err != nil {
			// Add context about which build spec failed
			errs.AddErrorf("build[%d] (%s): %v", i, bs.Name, err)
		}
	}

	// Validate all test specs
	for i, ts := range s.Test {
		if err := ts.Validate(); err != nil {
			// Add context about which test spec failed
			errs.AddErrorf("test[%d] (%s): %v", i, ts.Name, err)
		}
	}

	// A stage's needs name build entries, and an entry has one owner: two
	// stages claiming one entry would each believe the other built it.
	entries := map[string]bool{}
	for _, bs := range s.Build {
		entries[bs.Name] = true
	}

	owners := map[string]string{}

	for _, ts := range s.Test {
		for _, need := range ts.Needs {
			if !entries[need] {
				errs.AddErrorf("test stage %q needs %q, which no build entry declares", ts.Name, need)

				continue
			}

			if owner, taken := owners[need]; taken {
				errs.AddErrorf("build entry %q is needed by both %q and %q; an entry has one owning stage", need, owner, ts.Name)

				continue
			}

			owners[need] = ts.Name
		}
	}

	for i, rs := range s.Run {
		if err := rs.Validate(); err != nil {
			errs.AddErrorf("run[%d] (%s): %v", i, rs.Name, err)
		}
	}

	// Validate all engine configs
	for i, ec := range s.Engines {
		if err := ec.Validate(); err != nil {
			// Add context about which engine config failed
			errs.AddErrorf("engines[%d] (%s): %v", i, ec.Alias, err)
		}
	}

	return errs.ErrorOrNil()
}

// OwnedBuilds maps every build entry a test stage needs to that stage. An
// owned entry is built by its stage, or by name, never by a bare build.
func (s *Spec) OwnedBuilds() map[string]string {
	owners := map[string]string{}

	for _, ts := range s.Test {
		for _, need := range ts.Needs {
			owners[need] = ts.Name
		}
	}

	return owners
}

var errReadingProjectConfig = errors.New("error reading project config")

// ReadSpec reads the forge configuration from the forge.yaml file.
// It returns a Spec struct and an error if the file cannot be read or parsed.
func ReadSpec() (Spec, error) {
	return ReadSpecFromPath(ConfigPath)
}

// ReadSpecFromPath reads the forge configuration from the specified file path.
// It returns a Spec struct and an error if the file cannot be read or parsed.
func ReadSpecFromPath(path string) (Spec, error) {
	b, err := os.ReadFile(path) //nolint:varnamelen
	if err != nil {
		return Spec{}, flaterrors.Join(err, errReadingProjectConfig)
	}

	// Check for deprecated generateOpenAPI configuration before unmarshaling
	// We need to check the raw YAML since the field is removed from the struct
	var rawSpec map[string]interface{}
	if err := yaml.Unmarshal(b, &rawSpec); err != nil {
		return Spec{}, flaterrors.Join(err, errReadingProjectConfig)
	}
	if _, hasGenerateOpenAPI := rawSpec["generateOpenAPI"]; hasGenerateOpenAPI {
		return Spec{}, errors.New("generateOpenAPI configuration is no longer supported. Please migrate to build section. See docs/migration-go-gen-openapi.md for migration instructions")
	}

	// Two engines once had first-class keys at the top of forge.yaml while
	// every other engine is configured under the entry that names it. The
	// keys are gone; the same settings live on the testenv entry's spec.
	for _, retired := range []string{"kindenv", "localContainerRegistry"} {
		if _, has := rawSpec[retired]; has {
			return Spec{}, fmt.Errorf("%s: is not a forge.yaml key; configure the engine under the testenv entry that names it (engines[].testenv[].spec)", retired)
		}
	}

	out := Spec{} //nolint:exhaustruct // unmarshal

	if err := yaml.Unmarshal(b, &out); err != nil {
		return Spec{}, flaterrors.Join(err, errReadingProjectConfig)
	}

	// Apply defaults to test specs
	for i := range out.Test {
		if out.Test[i].Testenv == "" {
			out.Test[i].Testenv = "forge://test-report"
		}
	}

	// Apply default to EnvFile
	if out.EnvFile == "" {
		out.EnvFile = ".envrc"
	}

	// Validate the spec
	if err := out.Validate(); err != nil {
		return Spec{}, flaterrors.Join(err, errReadingProjectConfig)
	}

	return out, nil
}
