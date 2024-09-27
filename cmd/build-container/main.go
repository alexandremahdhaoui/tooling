package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/alexandremahdhaoui/tooling/internal/util"
	"github.com/alexandremahdhaoui/tooling/pkg/flaterrors"
	"github.com/caarlos0/env"
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

	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	cmd := envs.ContainerEngine
	args := []string{
		"run", "-i",
		"-v", fmt.Sprintf("%s:/workspace", wd),
		"gcr.io/kaniko-project/executor:latest",
		"-f", fmt.Sprintf("./containers/%s/Containerfile", envs.ContainerName),
	}

	for _, buildArg := range envs.BuildArgs {
		args = append(args, "--build-arg", buildArg)
	}

	switch len(envs.Destinations) {
	default:
		for _, dest := range envs.Destinations {
			args = append(args, "-d", dest)
		}
	case 0:
		args = append(args, "--no-push")
	}

	if err := util.RunCmdWithStdPipes(exec.Command(cmd, args...)); err != nil {
		return err
	}

	return nil
}

// ----------------------------------------------------- ENVS ------------------------------------------------------- //

type Envs struct {
	ContainerEngine string   `env:"CONTAINER_ENGINE,required"`
	ContainerName   string   `env:"CONTAINER_NAME,required"`
	BuildArgs       []string `env:"BUILD_ARGS,required"`
	Destinations    []string `env:"DESTINATIONS"`
}

// ----------------------------------------------------- PRINT HELPERS ----------------------------------------------- //

const usage = `USAGE

CONTAINER_ENGINE=%q CONTAINER_NAME=%q BUILD_ARGS=%q %s [BINARY_NAME]

Required environment variables:
    CONTAINER_ENGINE    string			Container engine such as podman or docker.
    CONTAINER_NAME      string      Name of the container to build.
    BUILD_ARGS          []string		List of build args (e.g. "GO_BUILD_LDFLAGS=\"-X main.BuildTimestamp=$(TIMESTAMP)\"").

Optional environment variables:
    DESTINATIONS        []string		List of destinations (e.g. "docker.io/alexandremahdhaoui/test:latest").
`

func printUsage() {
	fmt.Printf(
		usage,
		os.Getenv("CONTAINER_ENGINE"),
		os.Getenv("CONTAINER_NAME"),
		os.Getenv("BUILD_ARGS"),
		os.Args[0],
	)
}

func printSuccess() {
	fmt.Printf("✅ Container built successfully\n")
}

func printFailure(err error) {
	fmt.Printf("❌ Error building container\n%s\n", err.Error())
}
