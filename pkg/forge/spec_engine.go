package forge

import "fmt"

// EngineConfigType specifies the type of engine configuration
type EngineConfigType string

const (
	// BuilderEngineConfigType indicates this engine is for building artifacts
	BuilderEngineConfigType EngineConfigType = "builder"

	// TestRunnerEngineConfigType indicates this engine is for running tests
	TestRunnerEngineConfigType EngineConfigType = "test-runner"

	// TestenvEngineConfigType indicates this engine is for managing test environments
	TestenvEngineConfigType EngineConfigType = "testenv"
)

// EngineConfig defines a custom engine configuration with an alias.
// Engines can be referenced in BuildSpec or TestSpec using alias:// protocol.
type EngineConfig struct {
	// Alias is the name used to reference this engine (e.g., "my-formatter")
	// Can be used as: alias://my-formatter
	Alias string `json:"alias"`

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

	// WorkDir is the working directory for command execution (optional)
	WorkDir string `json:"workDir,omitempty"`
}

// BuilderEngineSpec defines specification for builder-type engines
type BuilderEngineSpec struct {
	// Engine is the builder engine URI (e.g., "go://generic-builder")
	Engine string `json:"engine"`

	// Spec contains the engine-specific configuration
	Spec EngineSpec `json:"spec,omitempty"`
}

// TestRunnerSpec defines specification for test-runner-type engines
type TestRunnerSpec struct {
	// Engine is the test runner engine URI (e.g., "go://generic-test-runner")
	Engine string `json:"engine"`

	// Spec contains the engine-specific configuration
	Spec EngineSpec `json:"spec,omitempty"`
}

// TestenvEngineSpec defines specification for a testenv-subengine component
// Note: "testenv-subengine" is an interface/role, not a formal type
type TestenvEngineSpec struct {
	// Engine is the testenv-subengine URI (e.g., "go://testenv-kind")
	Engine string `json:"engine"`

	// Spec contains engine-specific configuration (free-form)
	Spec map[string]interface{} `json:"spec,omitempty"`
}

// Validate validates the EngineConfig
func (ec *EngineConfig) Validate() error {
	errs := NewValidationErrors()

	// Validate alias
	if err := ValidateRequired(ec.Alias, "alias", "EngineConfig"); err != nil {
		errs.Add(err)
	}

	// Validate type
	if ec.Type == "" {
		errs.AddErrorf("EngineConfig %q: type is required", ec.Alias)
	} else {
		validTypes := []EngineConfigType{BuilderEngineConfigType, TestRunnerEngineConfigType, TestenvEngineConfigType}
		valid := false
		for _, vt := range validTypes {
			if ec.Type == vt {
				valid = true
				break
			}
		}
		if !valid {
			errs.AddErrorf("EngineConfig %q: invalid type %q, must be one of: %q, %q, %q",
				ec.Alias, ec.Type, BuilderEngineConfigType, TestRunnerEngineConfigType, TestenvEngineConfigType)
		}
	}

	// Validate that the correct nested config is used based on type
	builderCount := len(ec.Builder)
	testRunnerCount := len(ec.TestRunner)
	testenvCount := len(ec.Testenv)

	switch ec.Type {
	case BuilderEngineConfigType:
		if testRunnerCount > 0 || testenvCount > 0 {
			errs.AddErrorf("EngineConfig %q: type=builder but contains testRunner or testenv configuration", ec.Alias)
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
		if builderCount > 0 || testenvCount > 0 {
			errs.AddErrorf("EngineConfig %q: type=test-runner but contains builder or testenv configuration", ec.Alias)
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
		if builderCount > 0 || testRunnerCount > 0 {
			errs.AddErrorf("EngineConfig %q: type=testenv but contains builder or testRunner configuration", ec.Alias)
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
