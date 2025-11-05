package forge

// EngineConfig defines a custom engine configuration with an alias.
// Engines can be referenced in BuildSpec or TestSpec using alias:// protocol.
type EngineConfig struct {
	// Alias is the name used to reference this engine (e.g., "my-formatter")
	// Can be used as: alias://my-formatter
	Alias string `json:"alias"`

	// Engine is the underlying engine URI (e.g., "go://generic-engine")
	Engine string `json:"engine"`

	// Config contains the engine-specific configuration
	Config EngineConfigDetails `json:"config,omitempty"`
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
