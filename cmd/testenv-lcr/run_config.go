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

// defaultNamespace is where the registry lands when the stage names none.
const defaultNamespace = "testenv-lcr"

// runConfig is what one create or delete acts on: what the stage declared
// in its spec, and the paths this run decided (the kubeconfig the cluster
// engine handed over, the files written under tmpDir). Nothing here is read
// from forge.yaml: the registry is configured where its engine is named,
// under the testenv entry's spec, and nowhere else.
type runConfig struct {
	Kindenv                kubeconfigRef
	LocalContainerRegistry registrySettings
}

type kubeconfigRef struct {
	KubeconfigPath string
}

type registrySettings struct {
	Enabled                   bool
	CredentialPath            string
	CaCrtPath                 string
	Namespace                 string
	ImagePullSecretNamespaces []string
	ImagePullSecretName       string
}

// configFrom answers the run configuration a declared spec asks for. A nil
// spec is a stage that declared nothing, which is a disabled registry.
func configFrom(spec *Spec) runConfig {
	if spec == nil {
		return runConfig{LocalContainerRegistry: registrySettings{Namespace: defaultNamespace}}
	}

	namespace := spec.Namespace
	if namespace == "" {
		namespace = defaultNamespace
	}

	return runConfig{
		LocalContainerRegistry: registrySettings{
			Enabled:                   spec.Enabled,
			Namespace:                 namespace,
			ImagePullSecretNamespaces: spec.ImagePullSecretNamespaces,
			ImagePullSecretName:       spec.ImagePullSecretName,
		},
	}
}
