package forge

// EngineConfig defines a custom engine configuration with an alias.
// Engines can be referenced in BuildSpec or TestSpec using alias:// protocol.
type EngineConfig struct {
	// Alias is the name used to reference this engine (e.g., "my-formatter")
	// Can be used as: alias://my-formatter
	Alias string `json:"alias"`

	// Type specifies the engine type: "build", "test-runner", or "testenv"
	Type string `json:"type,omitempty"`

	// Engine is the underlying engine URI (e.g., "go://generic-builder")
	Engine string `json:"engine"`

	// Config contains the engine-specific configuration
	Config EngineConfigDetails `json:"config,omitempty"`

	// Build configuration (only used when Type="build")
	Build *BuildEngineConfig `json:"build,omitempty"`

	// TestRunner configuration (only used when Type="test-runner")
	TestRunner *TestRunnerConfig `json:"testRunner,omitempty"`

	// Testenv configuration (only used when Type="testenv")
	// List of testenv-subengines to compose
	Testenv []TestenvEngineConfig `json:"testenv,omitempty"`
}

// EngineConfigDetails contains the configuration details for an engine.
type EngineConfigDetails struct {
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

// BuildEngineConfig defines configuration for build-type engines
type BuildEngineConfig struct {
	Engine string              `json:"engine"`
	Config EngineConfigDetails `json:"config,omitempty"`
}

// TestRunnerConfig defines configuration for test-runner-type engines
type TestRunnerConfig struct {
	Engine string              `json:"engine"`
	Config EngineConfigDetails `json:"config,omitempty"`
}

// TestenvEngineConfig defines configuration for a testenv-subengine component
// Note: "testenv-subengine" is an interface/role, not a formal type
type TestenvEngineConfig struct {
	// Engine is the testenv-subengine URI (e.g., "go://testenv-kind")
	Engine string `json:"engine"`

	// Spec contains engine-specific configuration (free-form)
	Spec map[string]interface{} `json:"spec,omitempty"`
}
