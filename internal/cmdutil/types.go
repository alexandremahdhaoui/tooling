package cmdutil

// ExecuteInput contains the parameters for command execution.
type ExecuteInput struct {
	Command string            // Command to execute
	Args    []string          // Command arguments
	Env     map[string]string // Environment variables
	EnvFile string            // Path to environment file (optional)
	WorkDir string            // Working directory (optional)
}

// ExecuteOutput contains the result of command execution.
type ExecuteOutput struct {
	ExitCode int    // Command exit code
	Stdout   string // Standard output
	Stderr   string // Standard error
	Error    string // Error message if execution failed
}
