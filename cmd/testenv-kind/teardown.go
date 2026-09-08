// Copyright 2024 Alexandre Mahdhaoui
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"os"
	"os/exec"

	"github.com/alexandremahdhaoui/forge/internal/util"
)

// ----------------------------------------------------- TEARDOWN --------------------------------------------------- //

func doTeardown(config cluster, envs Envs) error {
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

	// Only remove kubeconfig file if path is set
	// Path might be empty if cleanup is called without proper metadata
	if config.KubeconfigPath != "" {
		if err := os.Remove(config.KubeconfigPath); err != nil {
			// Log warning but don't fail - file might already be deleted
			// or cleanup might be called multiple times
			if !os.IsNotExist(err) {
				return err
			}
		}
	}

	return nil
}
