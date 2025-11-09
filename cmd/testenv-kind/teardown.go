package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/alexandremahdhaoui/forge/internal/util"
	"github.com/alexandremahdhaoui/forge/pkg/forge"
)

// ----------------------------------------------------- TEARDOWN --------------------------------------------------- //

// teardown executes the main logic of the `kindenv teardown` command.
// It reads the project and kindenv configuration, and then deletes the kind cluster.
func teardown() error {
	// 1. read project Envs.
	config, err := forge.ReadSpec()
	if err != nil {
		return err // TODO: wrap err
	}

	_, _ = fmt.Fprintf(os.Stderr, "⏳ Tearing down kindenv %q\n", config.Name)

	// 2. read kindenv Envs
	cfg, err := readEnvs()
	if err != nil {
		return fmt.Errorf("%s\n❌ ERROR: %w", formatSetupUsage(), err) // TODO: wrap err
	}

	_, _ = fmt.Fprintf(os.Stderr, "%#v\n", cfg)

	// 3. Do
	if err := doTeardown(config, cfg); err != nil {
		return err // TODO: wrap error
	}

	_, _ = fmt.Fprintf(os.Stderr, "✅ Kindenv %q torn down successfully\n", config.Name)

	return nil
}

func doTeardown(config forge.Spec, envs Envs) error {
	cmdName := envs.KindBinary
	args := []string{
		"delete",
		"cluster",
		"--name", config.Name,
	}

	if envs.KindBinaryPrefix != "" {
		cmdName = envs.KindBinaryPrefix
		args = append([]string{envs.KindBinary}, args...)
	}

	cmd := exec.Command(cmdName, args...)

	if err := util.RunCmdWithStdPipes(cmd); err != nil {
		return err // TODO: wrap error
	}

	if err := os.Remove(config.Kindenv.KubeconfigPath); err != nil {
		return err // TODO: wrap error
	}

	return nil
}
