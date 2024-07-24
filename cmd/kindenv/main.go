package main

import (
	"fmt"
	"os"
)

const (

	// Available commands
	setupCommand    = "setup"
	teardownCommand = "teardown"
	helpCommand     = "usage"
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

func usage() error {
	arg0 := fmt.Sprintf("go run \"%s/hack/kindenv\"", os.Getenv("PWD"))
	_, _ = fmt.Fprintf(os.Stdout, usageTemplate, arg0, setupCommand, teardownCommand)

	return nil
}

// ----------------------------------------------------- MAIN ------------------------------------------------------- //

func main() {
	_, _ = fmt.Fprint(os.Stdout, banner)

	// 1. Print usageTemplate or

	if len(os.Args) < 2 { //nolint:gomnd // if no specified subcommand then print usageTemplate and exit.
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
	}

	// 3. Execute command

	if err := command(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	os.Exit(0)
}
