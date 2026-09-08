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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/alexandremahdhaoui/forge/internal/util"
	"github.com/caarlos0/env/v11"
)

// ----------------------------------------------------- CONFIG ----------------------------------------------------- //

// Envs holds the environment variables required by the kindenv tool.
type Envs struct {
	// KindBinary is the path to the kind binary.
	KindBinary string `env:"KIND_BINARY,required"`
	// KindBinaryPrefix is a prefix to add to the kind binary command (e.g., sudo).
	KindBinaryPrefix string `env:"KIND_BINARY_PREFIX"`

	// TODO: make use of the below variables.
	ContainerRegistryBaseURL string `env:"CONTAINER_REGISTRY_BASE_URL"`
	ContainerEngineBinary    string `env:"CONTAINER_ENGINE_BINARY"`
	HelmBinary               string `env:"HELM_BINARY"`
}

// readEnvs reads the environment variables required by the kindenv tool.
func readEnvs() (Envs, error) {
	out := Envs{} //nolint:exhaustruct // unmarshal

	if err := env.Parse(&out); err != nil {
		return Envs{}, err // TODO: wrap err
	}

	return out, nil
}

// ----------------------------------------------------- SETUP ------------------------------------------------------ //

// kindConfigContent is the Kind cluster configuration YAML that enables containerd
// to use the /etc/containerd/certs.d directory for registry-specific TLS certificates.
// This is required for Kind versions < v0.27.0 and is harmless for newer versions.
const kindConfigContent = `kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
containerdConfigPatches:
- |-
  [plugins."io.containerd.grpc.v1.cri".registry]
    config_path = "/etc/containerd/certs.d"
`

// generateKindConfig creates a Kind cluster configuration file with containerdConfigPatches
// that enable the /etc/containerd/certs.d directory for registry-specific TLS certificates.
// It writes the config to {tmpDir}/kind-config.yaml and returns the absolute path.
func generateKindConfig(tmpDir string) (string, error) {
	configPath := filepath.Join(tmpDir, "kind-config.yaml")

	if err := os.WriteFile(configPath, []byte(kindConfigContent), 0o600); err != nil {
		return "", fmt.Errorf("failed to write kind config file: %w", err)
	}

	return configPath, nil
}

// cluster is the one kind cluster a create or delete acts on: its name and
// where its kubeconfig lives. Both are decided per run, never read from
// forge.yaml.
type cluster struct {
	Name           string
	KubeconfigPath string
}

func doSetup(pCfg cluster, envs Envs) error {
	// 0. Generate Kind config file with containerd patches for TLS trust.
	tmpDir := filepath.Dir(pCfg.KubeconfigPath)
	kindConfigPath, err := generateKindConfig(tmpDir)
	if err != nil {
		return fmt.Errorf("failed to generate kind config: %w", err)
	}

	// 1. Allow prefixing kind binary with "sudo".
	cmdName := envs.KindBinary
	args := []string{
		"create",
		"cluster",
		"--name", pCfg.Name,
		"--kubeconfig", pCfg.KubeconfigPath,
		"--config", kindConfigPath,
		"--wait", "5m",
	}

	if envs.KindBinaryPrefix != "" {
		cmdName = envs.KindBinaryPrefix
		args = append([]string{envs.KindBinary}, args...)
	}

	// 2. kind create cluster and wait.
	cmd := exec.Command(cmdName, args...)

	if err := util.RunCmdWithStdPipes(cmd); err != nil {
		return err // TODO: wrap error
	}

	// 3. chown kubeconfig
	if envs.KindBinaryPrefix == "sudo" { // TODO: Make this a bit more robust (e.g. use which or something)
		chownCmd := exec.Command(
			envs.KindBinaryPrefix,
			"chown",
			fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()),
			pCfg.KubeconfigPath,
		)

		if err := util.RunCmdWithStdPipes(chownCmd); err != nil {
			return err // TODO: wrap err
		}
	}

	// 3. TODO: setup communication towards local-registry.

	// 4. TODO: setup communication towards any provided registry (e.g. required if users wants to install some apps into their kind cluster). It can be any OCI registry. (to support helm chart)

	// 5. TODO: setup communication CONTAINER_ENGINE login & HELM login.

	return nil
}
