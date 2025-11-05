package cli

import (
	"log"
	"os"

	"github.com/alexandremahdhaoui/forge/internal/version"
)

// Config holds the configuration for CLI bootstrap.
type Config struct {
	// Name is the command name (e.g., "build-go", "test-integration")
	Name string

	// Version information (typically set via ldflags)
	Version        string
	CommitSHA      string
	BuildTimestamp string

	// RunCLI is the function to execute in normal CLI mode
	RunCLI func() error

	// RunMCP is the function to execute in MCP server mode (optional)
	// If nil, --mcp flag will result in an error
	RunMCP func() error

	// SuccessHandler is called when RunCLI completes successfully (optional)
	// Defaults to no-op if not provided
	SuccessHandler func()

	// FailureHandler is called when RunCLI returns an error (optional)
	// Receives the error and should print it appropriately
	// Defaults to no-op if not provided
	FailureHandler func(error)
}

// Bootstrap provides a unified entry point for forge CLI commands.
// It handles version flags, MCP mode, and CLI execution with standardized error handling.
//
// This function will call os.Exit and never return.
func Bootstrap(cfg Config) {
	// Initialize version information
	versionInfo := version.New(cfg.Name)
	versionInfo.Version = cfg.Version
	versionInfo.CommitSHA = cfg.CommitSHA
	versionInfo.BuildTimestamp = cfg.BuildTimestamp

	// Check for version flag
	for _, arg := range os.Args[1:] {
		if arg == "version" || arg == "--version" || arg == "-v" {
			versionInfo.Print()
			os.Exit(0)
		}
	}

	// Check for --mcp flag to run as MCP server
	for _, arg := range os.Args[1:] {
		if arg == "--mcp" {
			if cfg.RunMCP == nil {
				log.Printf("Error: MCP mode not supported for %s", cfg.Name)
				os.Exit(1)
			}
			if err := cfg.RunMCP(); err != nil {
				log.Printf("MCP server error: %v", err)
				os.Exit(1)
			}
			os.Exit(0)
		}
	}

	// Normal CLI mode
	if err := cfg.RunCLI(); err != nil {
		if cfg.FailureHandler != nil {
			cfg.FailureHandler(err)
		}
		os.Exit(1)
	}

	if cfg.SuccessHandler != nil {
		cfg.SuccessHandler()
	}
	os.Exit(0)
}

// BootstrapSimple is a convenience wrapper for commands that don't support MCP mode.
func BootstrapSimple(name, version, commitSHA, buildTimestamp string, runCLI func() error) {
	Bootstrap(Config{
		Name:           name,
		Version:        version,
		CommitSHA:      commitSHA,
		BuildTimestamp: buildTimestamp,
		RunCLI:         runCLI,
		RunMCP:         nil,
	})
}
