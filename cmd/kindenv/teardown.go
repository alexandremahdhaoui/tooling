package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/alexandremahdhaoui/tooling/internal/util"
	"github.com/alexandremahdhaoui/tooling/pkg/project"
)

// ----------------------------------------------------- TEARDOWN --------------------------------------------------- //

func teardown() error {
	// 1. read project Envs.
	config, err := project.ReadConfig()
	if err != nil {
		return err // TODO: wrap err
	}

	_, _ = fmt.Fprintf(os.Stdout, "⏳ Tearing down kindenv %q\n", config.Name)

	// 2. read kindenv Envs
	cfg, err := readEnvs()
	if err != nil {
		return fmt.Errorf("%s\n❌ ERROR: %w", formatSetupUsage(), err) // TODO: wrap err
	}

	_, _ = fmt.Fprintf(os.Stdout, "%#v\n", cfg)

	// 3. Do
	if err := doTeardown(config, cfg); err != nil {
		return err // TODO: wrap error
	}

	_, _ = fmt.Fprintf(os.Stdout, "✅ Kindenv %q torn down successfully\n", config.Name)

	return nil
}

func doTeardown(config project.Config, envs Envs) error {
	// 1. kind create cluster and wait.
	cmd := exec.Command(
		envs.KindBinary,
		"delete",
		"cluster",
		"--name", config.Name,
	)

	if err := util.RunCmdWithStdPipes(cmd); err != nil {
		return err // TODO: wrap error
	}

	if err := os.Remove(config.Kindenv.KubeconfigPath); err != nil {
		return err // TODO: wrap error
	}

	return nil
}
