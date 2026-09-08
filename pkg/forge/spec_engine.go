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

// EngineConfigType specifies the type of engine configuration
type EngineConfigType string

const (
	// BuilderEngineConfigType indicates this engine is for building artifacts
	BuilderEngineConfigType EngineConfigType = "builder"

	// TestRunnerEngineConfigType indicates this engine is for running tests
	TestRunnerEngineConfigType EngineConfigType = "test-runner"

	// TestenvEngineConfigType indicates this engine is for managing test environments
	TestenvEngineConfigType EngineConfigType = "testenv"

	// DependencyDetectorEngineConfigType indicates this engine is for detecting dependencies
	DependencyDetectorEngineConfigType EngineConfigType = "dependency-detector"
)

// EngineConfig defines a custom engine configuration with an alias.
// Engines can be referenced in BuildSpec or TestSpec using alias:// protocol.
type EngineConfig struct {
	// Alias is the name used to reference this engine (e.g., "my-formatter")
	// Can be used as: alias://my-formatter
	Alias string `json:"alias"`

	// Engine makes this entry a registry entry: forge://<alias> resolves to
	// it before any built-in fallback. It is a forge:// module path (a
	// factory member or forge's own module, pinned by the register or by an
	// @version), or a path to a main package directory, relative to this
	// forge.yaml or absolute, run from source. An entry with an engine
	// carries no type and no composition list: it names, it does not compose.
	Engine string `json:"engine,omitempty"`

	// Type specifies the engine type: "builder", "test-runner", or "testenv"
	// This field is required and must match one of the EngineConfigType constants
	Type EngineConfigType `json:"type"`

	// Builder specification (only used when Type="builder")
	// List of builders to compose
	Builder []BuilderEngineSpec `json:"builder,omitempty"`

	// TestRunner specification (only used when Type="test-runner")
	// List of test-runners to compose
	TestRunner []TestRunnerSpec `json:"testRunner,omitempty"`

	// Testenv specification (only used when Type="testenv")
	// List of testenv-subengines to compose
	Testenv []TestenvEngineSpec `json:"testenv,omitempty"`

	// DependencyDetector specification (only used when Type="dependency-detector")
	// List of dependency-detectors to compose
	DependencyDetector []DependencyDetectorEngineSpec `json:"dependencyDetector,omitempty"`
}

// EngineSpec contains the specification details for an engine.
type EngineSpec struct {
	// Command is the shell command to execute
	Command string `json:"command,omitempty"`

	// Args are the default arguments to pass to the command
	Args []string `json:"args,omitempty"`

	// Env contains environment variables to set when executing the command
	// These are merged with system environment and .envrc file
	// Precedence: system < envFile < inline env (this field)
	Env map[string]string `json:"env,omitempty"`

	// EnvFile is the path to an environment file (e.g., ".envrc")
	// The file should contain KEY=VALUE pairs, one per line
	EnvFile string `json:"envFile,omitempty"`

	// Context is the context directory for command execution (optional)
	Context string `json:"context,omitempty"`
}

// BuilderEngineSpec defines specification for builder-type engines
type BuilderEngineSpec struct {
	// Engine is the builder engine URI (e.g., "forge://generic-builder")
	Engine string `json:"engine"`

	// Spec contains the engine-specific configuration
	Spec EngineSpec `json:"spec,omitempty"`
}

// TestRunnerSpec defines specification for test-runner-type engines
type TestRunnerSpec struct {
	// Engine is the test runner engine URI (e.g., "forge://generic-test-runner")
	Engine string `json:"engine"`

	// Spec contains the engine-specific configuration
	Spec EngineSpec `json:"spec,omitempty"`
}

// TestenvEngineSpec defines specification for a testenv-subengine component
// Note: "testenv-subengine" is an interface/role, not a formal type
type TestenvEngineSpec struct {
	// Engine is the testenv-subengine URI (e.g., "forge://testenv-kind")
	Engine string `json:"engine"`

	// Spec contains engine-specific configuration (free-form)
	Spec map[string]interface{} `json:"spec,omitempty"`

	// DeferTemplates when true, skips template expansion and passes spec verbatim to sub-engine.
	// This allows sub-engines to handle their own template expansion.
	// Default is false (templates are expanded by the orchestrator).
	DeferTemplates bool `json:"deferTemplates,omitempty" yaml:"deferTemplates,omitempty"`
}

// DependencyDetectorEngineSpec defines specification for dependency-detector-type engines
type DependencyDetectorEngineSpec struct {
	// Engine is the dependency detector engine URI (e.g., "forge://go-dependency-detector")
	Engine string `json:"engine"`

	// Spec contains engine-specific configuration (free-form)
	Spec map[string]interface{} `json:"spec,omitempty"`
}

// Validate validates the EngineConfig
// IsRegistryEntry reports whether this entry names an engine rather than
// composing one.
func (ec *EngineConfig) IsRegistryEntry() bool {
	return ec.Engine != ""
}

// IsEnginePath reports whether a registry target is a path to a main
// package rather than a forge:// URI.
func IsEnginePath(target string) bool {
	return strings.HasPrefix(target, "./") || strings.HasPrefix(target, "../") || strings.HasPrefix(target, "/")
}

func (ec *EngineConfig) Validate() error {
	errs := NewValidationErrors()

	// Validate alias
	if err := ValidateRequired(ec.Alias, "alias", "EngineConfig"); err != nil {
		errs.Add(err)
	}

	if ec.IsRegistryEntry() {
		if ec.Type != "" || len(ec.Builder) > 0 || len(ec.TestRunner) > 0 || len(ec.Testenv) > 0 || len(ec.DependencyDetector) > 0 {
			errs.AddErrorf("EngineConfig %q: an entry with engine names an engine and carries no type or composition; drop one or the other", ec.Alias)
		}

		if !strings.HasPrefix(ec.Engine, "forge://") && !IsEnginePath(ec.Engine) {
			errs.AddErrorf("EngineConfig %q: engine must be a forge:// module path or a path to a main package (./cmd/<name>, ../repo/cmd/<name>, /abs/dir), got %q", ec.Alias, ec.Engine)
		}

		return errs.ErrorOrNil()
	}

	// Validate type
	if ec.Type == "" {
		errs.AddErrorf("EngineConfig %q: type is required", ec.Alias)
	} else {
		validTypes := []EngineConfigType{BuilderEngineConfigType, TestRunnerEngineConfigType, TestenvEngineConfigType, DependencyDetectorEngineConfigType}
		valid := false
		for _, vt := range validTypes {
			if ec.Type == vt {
				valid = true
				break
			}
		}
		if !valid {
			errs.AddErrorf("EngineConfig %q: invalid type %q, must be one of: %q, %q, %q, %q",
				ec.Alias, ec.Type, BuilderEngineConfigType, TestRunnerEngineConfigType, TestenvEngineConfigType, DependencyDetectorEngineConfigType)
		}
	}

	// Validate that the correct nested config is used based on type
	builderCount := len(ec.Builder)
	testRunnerCount := len(ec.TestRunner)
	testenvCount := len(ec.Testenv)
	dependencyDetectorCount := len(ec.DependencyDetector)

	switch ec.Type {
	case BuilderEngineConfigType:
		if testRunnerCount > 0 || testenvCount > 0 || dependencyDetectorCount > 0 {
			errs.AddErrorf("EngineConfig %q: type=builder but contains testRunner, testenv, or dependencyDetector configuration", ec.Alias)
		}
		if builderCount == 0 {
			errs.AddErrorf("EngineConfig %q: type=builder requires at least one builder specification", ec.Alias)
		}
		// Validate each builder spec
		for i, b := range ec.Builder {
			if err := b.Validate(ec.Alias, i); err != nil {
				errs.Add(err)
			}
		}

	case TestRunnerEngineConfigType:
		if builderCount > 0 || testenvCount > 0 || dependencyDetectorCount > 0 {
			errs.AddErrorf("EngineConfig %q: type=test-runner but contains builder, testenv, or dependencyDetector configuration", ec.Alias)
		}
		if testRunnerCount == 0 {
			errs.AddErrorf("EngineConfig %q: type=test-runner requires at least one testRunner specification", ec.Alias)
		}
		// Validate each test runner spec
		for i, tr := range ec.TestRunner {
			if err := tr.Validate(ec.Alias, i); err != nil {
				errs.Add(err)
			}
		}

	case TestenvEngineConfigType:
		if builderCount > 0 || testRunnerCount > 0 || dependencyDetectorCount > 0 {
			errs.AddErrorf("EngineConfig %q: type=testenv but contains builder, testRunner, or dependencyDetector configuration", ec.Alias)
		}
		if testenvCount == 0 {
			errs.AddErrorf("EngineConfig %q: type=testenv requires at least one testenv specification", ec.Alias)
		}
		// Validate each testenv spec
		for i, te := range ec.Testenv {
			if err := te.Validate(ec.Alias, i); err != nil {
				errs.Add(err)
			}
		}

	case DependencyDetectorEngineConfigType:
		if builderCount > 0 || testRunnerCount > 0 || testenvCount > 0 {
			errs.AddErrorf("EngineConfig %q: type=dependency-detector but contains builder, testRunner, or testenv configuration", ec.Alias)
		}
		if dependencyDetectorCount == 0 {
			errs.AddErrorf("EngineConfig %q: type=dependency-detector requires at least one dependencyDetector specification", ec.Alias)
		}
		// Validate each dependency detector spec
		for i, dd := range ec.DependencyDetector {
			if err := dd.Validate(ec.Alias, i); err != nil {
				errs.Add(err)
			}
		}
	}

	return errs.ErrorOrNil()
}

// Validate validates the BuilderEngineSpec
func (bes *BuilderEngineSpec) Validate(alias string, index int) error {
	errs := NewValidationErrors()

	context := fmt.Sprintf("EngineConfig %q builder[%d]", alias, index)
	if err := ValidateURI(bes.Engine, context); err != nil {
		errs.Add(err)
	}

	return errs.ErrorOrNil()
}

// Validate validates the TestRunnerSpec
func (trs *TestRunnerSpec) Validate(alias string, index int) error {
	errs := NewValidationErrors()

	context := fmt.Sprintf("EngineConfig %q testRunner[%d]", alias, index)
	if err := ValidateURI(trs.Engine, context); err != nil {
		errs.Add(err)
	}

	return errs.ErrorOrNil()
}

// Validate validates the TestenvEngineSpec
func (tes *TestenvEngineSpec) Validate(alias string, index int) error {
	errs := NewValidationErrors()

	context := fmt.Sprintf("EngineConfig %q testenv[%d]", alias, index)
	if err := ValidateURI(tes.Engine, context); err != nil {
		errs.Add(err)
	}

	return errs.ErrorOrNil()
}

// Validate validates the DependencyDetectorEngineSpec
func (ddes *DependencyDetectorEngineSpec) Validate(alias string, index int) error {
	errs := NewValidationErrors()

	context := fmt.Sprintf("EngineConfig %q dependencyDetector[%d]", alias, index)
	if err := ValidateURI(ddes.Engine, context); err != nil {
		errs.Add(err)
	}

	return errs.ErrorOrNil()
}

// Registry answers the alias to engine map of this spec's registry entries.
func (s *Spec) Registry() map[string]string {
	out := map[string]string{}

	for _, e := range s.Engines {
		if e.IsRegistryEntry() {
			out[e.Alias] = e.Engine
		}
	}

	return out
}
