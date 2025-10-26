package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/alexandremahdhaoui/tooling/internal/util"
	"github.com/alexandremahdhaoui/tooling/pkg/flaterrors"
	"github.com/caarlos0/env/v11"
)

// ----------------------------------------------------- MAIN ------------------------------------------------------- //

func main() {
	if err := run(); err != nil {
		printFailure(err)
		os.Exit(1)
		return
	}

	printSuccess()
	os.Exit(0)
}

// ----------------------------------------------------- RUN -------------------------------------------------------- //

// run executes the main logic of the test-go tool.
// It reads environment variables and runs the gotestsum command.
func run() error {
	envs := Envs{} //nolint:exhaustruct // unmarshal

	if err := env.Parse(&envs); err != nil {
		printUsage()
		return flaterrors.Join(err, errors.New("error reading environment variables"))
	}

	cmd := envs.Gotestsum
	args := []string{
		"--junitfile", fmt.Sprintf(".ignore.test-%s.xml", envs.TestTag),
		"--",
		"-tags", envs.TestTag,
		"-race", "./...", "-count=1",
		"-cover", "-coverprofile", fmt.Sprintf(".ignore.test-%s-coverage.out", envs.TestTag),
		"./...",
	}

	if slice := strings.Split(envs.Gotestsum, " "); len(slice) > 1 {
		cmd = slice[0]
		args = append(slice[1:], args...)
	}

	if err := util.RunCmdWithStdPipes(exec.Command(cmd, args...)); err != nil {
		return flaterrors.Join(err, errors.New("error while running gotestsum"))
	}

	return nil
}

// ----------------------------------------------------- ENVS ------------------------------------------------------- //

// Envs holds the environment variables required by the test-go tool.
type Envs struct {
	// TestTag is the tag to target the test, i.e.: "unit", "integration", "functional", or "e2e".
	TestTag string `env:"TEST_TAG,required"`
	// Gotestsum is the path to the gotestsum binary or a `go run` command.
	Gotestsum string `env:"GOTESTSUM,required"`
}

// ----------------------------------------------------- PRINT HELPERS ----------------------------------------------- //

const usage = `USAGE

GOTESTSUM="" TEST_TAG="" %s

With:
    GOTESTSUM   Path to go-test-sum or "go run" command.
    TEST_TAG    Tag to target the test, i.e.: "unit", "integration", "functional", or "e2e".
`

func printUsage() {
	fmt.Printf(usage, os.Args[0])
}

func printSuccess() {
	fmt.Printf("✅ %s tests ran successfully\n", os.Getenv("TEST_TAG"))
}

func printFailure(err error) {
	fmt.Printf("❌ Error while running %s tests\n%s\n", os.Getenv("TEST_TAG"), err.Error())
}
