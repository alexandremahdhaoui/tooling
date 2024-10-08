package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/alexandremahdhaoui/tooling/internal/util"
	"github.com/alexandremahdhaoui/tooling/pkg/flaterrors"
	"github.com/caarlos0/env/v11"
)

// ----------------------------------------------------- ENVS ------------------------------------------------------- //

func main() {
	if err := run(); err != nil {
		printFailure(err)
		os.Exit(1)
		return
	}

	printSuccess()
	os.Exit(0)
}

// ----------------------------------------------------- ENVS ------------------------------------------------------- //

func run() error {
	envs := Envs{} //nolint:exhaustruct // unmarshal

	if err := env.Parse(&envs); err != nil {
		printUsage()
		return flaterrors.Join(err, errors.New("error reading environment variables"))
	}

	if err := os.Setenv("CG0_ENABLED", "0"); err != nil {
		return err
	}

	cmd := "go"
	args := []string{
		"build",
		"-ldflags", envs.GoBuildLDFlags,
		"-o", fmt.Sprintf("./build/bin/%s", envs.BinaryName),
		fmt.Sprintf("./cmd/%s", envs.BinaryName),
	}

	if err := util.RunCmdWithStdPipes(exec.Command(cmd, args...)); err != nil {
		return err
	}

	return nil
}

// ----------------------------------------------------- ENVS ------------------------------------------------------- //

type Envs struct {
	BinaryName     string `env:"BINARY_NAME,required"`
	GoBuildLDFlags string `env:"GO_BUILD_LDFLAGS,required"`
}

// ----------------------------------------------------- PRINT HELPERS ----------------------------------------------- //

const usage = `USAGE

BINARY_NAME="%s" GO_BUILD_LDFLAGS="%s" %s [BINARY_NAME]

Required environment variables:
    BINARY_NAME         Name of the binary to build.
    GO_BUILD_LDFLAGS    Go linker flags.
`

func printUsage() {
	fmt.Printf(usage, os.Getenv("BINARY_NAME"), os.Getenv("GO_BUILD_LDFLAGS"), os.Args[0])
}

func printSuccess() {
	fmt.Printf("✅ Binary built successfully\n")
}

func printFailure(err error) {
	fmt.Printf("❌ Error building binary\n%s\n", err.Error())
}
