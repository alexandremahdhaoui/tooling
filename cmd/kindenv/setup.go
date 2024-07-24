package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/alexandremahdhaoui/tooling/internal/util"
	"github.com/alexandremahdhaoui/tooling/pkg/project"
	"github.com/caarlos0/env/v11"
)

// ----------------------------------------------------- USAGE ------------------------------------------------------ //

const (
	//nolint:dupword
	setupUsageTemplate = `
## Setup

The setup command may expect the following env variables:
%s`
)

func formatSetupUsage() string {
	return fmt.Sprintf(setupUsageTemplate, util.FormatExpectedEnvList[Envs]())
}

// ----------------------------------------------------- CONFIG ----------------------------------------------------- //

type Envs struct {
	KindBinary string `env:"KIND_BINARY,required"`

	// TODO: make use of the below variables.
	ContainerRegistryBaseURL string `env:"CONTAINER_REGISTRY_BASE_URL"`
	ContainerEngineBinary    string `env:"CONTAINER_ENGINE_BINARY"`
	HelmBinary               string `env:"HELM_BINARY"`
}

func readEnvs() (Envs, error) {
	out := Envs{} //nolint:exhaustruct // unmarshal

	if err := env.Parse(&out); err != nil {
		return Envs{}, err // TODO: wrap err
	}

	return out, nil
}

// ----------------------------------------------------- SETUP ------------------------------------------------------ //

func setup() error {
	// 1. read project Envs.
	config, err := project.ReadConfig()
	if err != nil {
		return err // TODO: wrap err
	}

	_, _ = fmt.Fprintf(os.Stdout, "⏳ Setting up kindenv %q\n", config.Name)

	// 2. read kindenv Envs
	envs, err := readEnvs()
	if err != nil {
		return fmt.Errorf("%s\n❌ ERROR: %w", formatSetupUsage(), err) // TODO: wrap err
	}

	// 3. Do
	if err := doSetup(config, envs); err != nil {
		return errors.Join(err, doTeardown(config, envs))
	}

	_, _ = fmt.Fprintf(os.Stdout, "✅ kindenv %q set up successfully\n", config.Name)

	return nil
}

func doSetup(pCfg project.Config, envs Envs) error {
	// 1. kind create cluster and wait.
	cmd := exec.Command(
		envs.KindBinary,
		"create",
		"cluster",
		"--name", pCfg.Name,
		"--kubeconfig", pCfg.Kindenv.KubeconfigPath,
		"--wait", "5m",
	)

	if err := util.RunCmdWithStdPipes(cmd); err != nil {
		return err // TODO: wrap error
	}

	// 2. TODO: setup communication towards local-registry.

	// 3. TODO: setup communication towards any provided registry (e.g. required if users wants to install some apps into their kind cluster). It can be any OCI registry. (to support helm chart)

	// 4. TODO: setup communication CONTAINER_ENGINE login & HELM login.

	return nil
}
