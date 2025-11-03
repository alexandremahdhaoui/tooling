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

// versionInfo holds kindenv's version information
var versionInfo *version.Info

func init() {
	versionInfo = version.New("kindenv")
	versionInfo.Version = Version
	versionInfo.CommitSHA = CommitSHA
	versionInfo.BuildTimestamp = BuildTimestamp
}

const (
	// Available commands
	setupCommand    = "setup"
	teardownCommand = "teardown"
	helpCommand     = "usage"
	versionCommand  = "version"
)

// ----------------------------------------------------- USAGE ------------------------------------------------------ //

const (
	banner        = "# KINDENV\n\n"
	usageTemplate = `## Usage

%s [command]

Available commands:
  - %q
  - %q
`
)

// usage prints the usage instructions for the kindenv tool.
func usage() error {
	arg0 := fmt.Sprintf("go run \"%s/hack/kindenv\"", os.Getenv("PWD"))
	_, _ = fmt.Fprintf(os.Stdout, usageTemplate, arg0, setupCommand, teardownCommand)

	return nil
}

// ----------------------------------------------------- MAIN ------------------------------------------------------- //

func main() {
	// Check for version flag first
	if len(os.Args) > 1 {
		arg := os.Args[1]
		if arg == "version" || arg == "--version" || arg == "-v" {
			versionInfo.Print()
			return
		}
	}

	_, _ = fmt.Fprint(os.Stdout, banner)

	// 1. Print usageTemplate or

	if len(
		os.Args,
	) < 2 { //nolint:gomnd // if no specified subcommand then print usageTemplate and exit.
		_ = usage()

		os.Exit(1)
	}

	// 2. Switch command.

	var command func() error

	switch os.Args[1] {
	case setupCommand:
		command = setup
	case teardownCommand:
		command = teardown
	case helpCommand:
		command = usage
	case versionCommand:
		versionInfo.Print()
		return
	}

	// 3. Execute command

	if err := command(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	os.Exit(0)
}
