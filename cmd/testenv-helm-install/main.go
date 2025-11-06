package main

import (
	"fmt"
	"os"

	"github.com/alexandremahdhaoui/forge/internal/version"
)

// Version information (set via ldflags during build)
var (
	Version        = "dev"
	CommitSHA      = "unknown"
	BuildTimestamp = "unknown"
)

// versionInfo holds testenv-helm-install's version information
var versionInfo *version.Info

func init() {
	versionInfo = version.New("testenv-helm-install")
	versionInfo.Version = Version
	versionInfo.CommitSHA = CommitSHA
	versionInfo.BuildTimestamp = BuildTimestamp
}

const (
	// Available commands
	createCommand  = "create"
	deleteCommand  = "delete"
	helpCommand    = "usage"
	versionCommand = "version"
)

// ----------------------------------------------------- USAGE ------------------------------------------------------ //

const (
	banner        = "# TESTENV-HELM-INSTALL\n\n"
	usageTemplate = `## Usage

%s [command]

Available commands:
  --mcp              Run as MCP server
  create             Install helm charts
  delete             Uninstall helm charts
  version            Show version information
  usage              Show this help message
`
)

// usage prints the usage instructions for the testenv-helm-install tool.
func usage() error {
	arg0 := "testenv-helm-install"
	_, _ = fmt.Fprintf(os.Stdout, usageTemplate, arg0)
	return nil
}

// ----------------------------------------------------- MAIN ------------------------------------------------------- //

func main() {
	// Check for version and MCP flags first
	if len(os.Args) > 1 {
		arg := os.Args[1]
		switch arg {
		case "--mcp":
			// Run in MCP server mode
			if err := runMCPServer(); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
				os.Exit(1)
			}
			return
		case "version", "--version", "-v":
			versionInfo.Print()
			return
		case "usage", "help", "--help", "-h":
			_, _ = fmt.Fprint(os.Stdout, banner)
			_ = usage()
			return
		}
	}

	_, _ = fmt.Fprint(os.Stdout, banner)

	// 1. Print usageTemplate or

	if len(os.Args) < 2 { //nolint:gomnd // if no specified subcommand then print usageTemplate and exit.
		_ = usage()
		os.Exit(1)
	}

	// 2. Switch command.

	var command func() error

	switch os.Args[1] {
	case createCommand:
		command = createCLI
	case deleteCommand:
		command = deleteCLI
	case helpCommand:
		command = usage
	case versionCommand:
		versionInfo.Print()
		return
	default:
		_, _ = fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		_ = usage()
		os.Exit(1)
	}

	// 3. Execute command

	if err := command(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	os.Exit(0)
}

// createCLI handles the create command when run via CLI (for debugging)
func createCLI() error {
	return fmt.Errorf("CLI create command not yet implemented - use --mcp mode")
}

// deleteCLI handles the delete command when run via CLI (for debugging)
func deleteCLI() error {
	return fmt.Errorf("CLI delete command not yet implemented - use --mcp mode")
}
