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
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/caarlos0/env/v11"
	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/alexandremahdhaoui/forge/pkg/engineframework"
	"github.com/alexandremahdhaoui/forge/pkg/flaterrors"
)

// ----------------------------------------------------- ENVS ------------------------------------------------------- //

// Envs holds the environment variables required by the local-container-registry tool.
type Envs struct {
	// ContainerEngineExecutable is the path to the container engine executable (e.g., docker, podman).
	ContainerEngineExecutable string `env:"CONTAINER_ENGINE"`
	// PrependCmd is an optional command to prepend to privileged operations (e.g., "sudo").
	PrependCmd string `env:"PREPEND_CMD"`
	// ElevatedPrependCmd is an optional command to prepend to operations requiring elevated permissions (e.g., "sudo -E").
	// This is used for operations like modifying /etc/hosts that require root access.
	ElevatedPrependCmd string `env:"ELEVATED_PREPEND_CMD"`
}

var errReadingEnvVars = errors.New("reading environment variables")

// readEnvs reads the environment variables required by the local-container-registry tool.
func readEnvs() (Envs, error) {
	out := Envs{} //nolint:exhaustruct // unmarshal

	if err := env.Parse(&out); err != nil {
		return Envs{}, flaterrors.Join(err, errReadingEnvVars)
	}

	return out, nil
}

// ----------------------------------------------------- PORT FORWARDERS --------------------------------------------- //

// activePortForwarders tracks active port-forwarders by testID for cleanup.
var activePortForwarders = make(map[string]*PortForwarder)

// portForwardersMu protects concurrent access to activePortForwarders.
var portForwardersMu sync.Mutex

// ----------------------------------------------------- CREATE/DELETE (MCP) ----------------------------------------- //

// Create implements the CreateFunc for creating a local container registry.
// The spec parameter contains typed fields, but we also use input.Spec for images parsing.
func Create(ctx context.Context, input engineframework.CreateInput, spec *Spec) (*engineframework.TestEnvArtifact, error) {
	log.Printf("Creating local container registry: testID=%s, stage=%s", input.TestID, input.Stage)

	// RootDir is available via input.RootDir for resolving relative paths
	// Currently unused, but available for future features that may need path resolution
	// (e.g., relative image build contexts, local Dockerfile paths)
	_ = input.RootDir // Acknowledge availability for consistency

	// Redirect stdout to stderr (setup() writes to stdout, but MCP uses stdout for JSON-RPC)
	oldStdout := os.Stdout
	os.Stdout = os.Stderr
	defer func() { os.Stdout = oldStdout }()

	// The stage's spec is the whole configuration; the paths join it below.
	config := configFrom(spec)

	// The images come off the same spec; the shape was checked by FromMap,
	// the meaning (a local:// name, exactly one credential source) here.
	var images []ImageSource
	if spec != nil {
		if err := ValidateImages(spec.Images); err != nil {
			return nil, fmt.Errorf("invalid images configuration: %w", err)
		}
		images = spec.Images
	}

	// Check if local container registry is enabled
	if !config.LocalContainerRegistry.Enabled {
		log.Printf("Local container registry is disabled, skipping setup")
		return &engineframework.TestEnvArtifact{
			TestID:           input.TestID,
			Files:            map[string]string{},
			Metadata:         map[string]string{"testenv-lcr.enabled": "false"},
			ManagedResources: []string{},
		}, nil
	}

	// Extract clusterName from metadata (required for port leasing and containerd trust)
	clusterName, ok := input.Metadata["testenv-kind.clusterName"]
	if !ok || clusterName == "" {
		return nil, fmt.Errorf("testenv-kind not executed: missing clusterName in metadata - containerd trust cannot be configured")
	}

	// Acquire dynamic port for this cluster using the port lease manager.
	// This port is used for NodePort, service port, target port, container port, and port-forward.
	portLeaseManager := NewPortLeaseManager()
	dynamicPort, err := portLeaseManager.AcquirePort(clusterName)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire port: %w", err)
	}
	log.Printf("Acquired dynamic port %d for cluster %s", dynamicPort, clusterName)

	// Track whether we need to release the port on failure
	portAcquired := true
	var portForwarder *PortForwarder

	// Defer cleanup on failure
	defer func() {
		if err != nil {
			// Cleanup port-forwarder if started
			if portForwarder != nil {
				portForwarder.Stop()
			}
			// Release port lease if acquired
			if portAcquired {
				if releaseErr := portLeaseManager.ReleasePort(clusterName); releaseErr != nil {
					log.Printf("Warning: failed to release port lease on cleanup: %v", releaseErr)
				}
			}
		}
	}()

	// Override kubeconfig path from environment (primary source, from testenv-kind)
	// Fallback to metadata for backward compatibility
	kubeconfigPath := ""
	if envKubeconfig, ok := input.Env["KUBECONFIG"]; ok && envKubeconfig != "" {
		kubeconfigPath = envKubeconfig
		log.Printf("Using KUBECONFIG from environment: %s", kubeconfigPath)
	} else if metadataKubeconfig, ok := input.Metadata["testenv-kind.kubeconfigPath"]; ok && metadataKubeconfig != "" {
		kubeconfigPath = metadataKubeconfig
		log.Printf("Using kubeconfig from metadata (backward compatibility): %s", kubeconfigPath)
	}

	if kubeconfigPath != "" {
		config.Kindenv.KubeconfigPath = kubeconfigPath
	}

	// Override file paths to use tmpDir
	caCrtPath := filepath.Join(input.TmpDir, "ca.crt")
	credentialPath := filepath.Join(input.TmpDir, "registry-credentials.yaml")

	config.LocalContainerRegistry.CaCrtPath = caCrtPath
	config.LocalContainerRegistry.CredentialPath = credentialPath

	// Call the existing setup logic with the overridden config and dynamic port
	if err := setupWithConfig(config, dynamicPort); err != nil {
		return nil, fmt.Errorf("failed to setup local container registry: %w", err)
	}

	// Construct registryFQDN with dynamic port
	registryFQDN := fmt.Sprintf("%s.%s.svc.cluster.local:%d", Name, config.LocalContainerRegistry.Namespace, dynamicPort)

	// Start port-forward to make registry accessible from host
	portForwarder = NewPortForwarder(config, config.LocalContainerRegistry.Namespace, dynamicPort)
	if err = portForwarder.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start port-forward: %w", err)
	}

	// Store port-forwarder in map for cleanup during delete
	portForwardersMu.Lock()
	activePortForwarders[input.TestID] = portForwarder
	portForwardersMu.Unlock()

	// Configure containerd trust on Kind nodes (Phase 2)
	// This must happen AFTER CA cert is exported (by setupWithConfig) and BEFORE we return metadata
	log.Printf("Configuring containerd trust for cluster %s", clusterName)
	if err = configureContainerdTrust(clusterName, registryFQDN, caCrtPath); err != nil {
		return nil, fmt.Errorf("failed to configure containerd trust: %w", err)
	}

	// IMPORTANT: The containerd restart on Kind nodes disrupts ALL pods, including the registry.
	// The port-forward loses connection to the pod. We need to:
	// 1. Wait for the registry deployment to be ready again
	// 2. Stop the old (dead) port-forward
	// 3. Start a new port-forward

	// Create Kubernetes client (needed for waiting for deployment)
	cl, err := createKubeClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kube client: %w", err)
	}

	// Wait for registry deployment to be ready after containerd restart
	log.Printf("Waiting for registry deployment to be ready after containerd restart...")
	containerRegistry := NewContainerRegistry(cl, config.LocalContainerRegistry.Namespace, nil)
	if err = containerRegistry.awaitDeployment(ctx); err != nil {
		return nil, fmt.Errorf("failed to wait for registry deployment after containerd restart: %w", err)
	}
	log.Printf("Registry deployment is ready")

	// Stop the old port-forward (it's dead anyway from "lost connection to pod")
	portForwarder.Stop()

	// Start a new port-forward
	portForwarder = NewPortForwarder(config, config.LocalContainerRegistry.Namespace, dynamicPort)
	if err = portForwarder.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to restart port-forward after containerd restart: %w", err)
	}
	log.Printf("Port-forward restarted successfully")

	// Update the stored port-forwarder
	portForwardersMu.Lock()
	activePortForwarders[input.TestID] = portForwarder
	portForwardersMu.Unlock()

	// Prepare files map (relative paths within tmpDir)
	files := map[string]string{
		"testenv-lcr.ca.crt":           "ca.crt",
		"testenv-lcr.credentials.yaml": "registry-credentials.yaml",
	}

	// Prepare metadata
	metadata := map[string]string{
		"testenv-lcr.registryFQDN":   registryFQDN,
		"testenv-lcr.namespace":      config.LocalContainerRegistry.Namespace,
		"testenv-lcr.caCrtPath":      caCrtPath,
		"testenv-lcr.credentialPath": credentialPath,
		"testenv-lcr.enabled":        "true",
		"testenv-lcr.port":           fmt.Sprintf("%d", dynamicPort),
	}

	// Prepare managed resources (for cleanup)
	managedResources := []string{
		caCrtPath,
		credentialPath,
	}

	// Process images if configured
	if len(images) > 0 {
		envs, err := readEnvs()
		if err != nil {
			return nil, fmt.Errorf("failed to read environment: %w", err)
		}
		if err := processImages(ctx, images, config, envs, dynamicPort); err != nil {
			return nil, fmt.Errorf("failed to process images: %w", err)
		}
	}

	// Add image pull secret information if they were created
	if len(config.LocalContainerRegistry.ImagePullSecretNamespaces) > 0 {
		secrets, err := ListImagePullSecrets(ctx, cl, "")
		if err != nil {
			log.Printf("Warning: failed to list image pull secrets: %v", err)
		} else {
			secretCount := 0
			for _, secret := range secrets {
				// Add to metadata
				key := fmt.Sprintf("testenv-lcr.imagePullSecret.%d", secretCount)
				metadata[key+".namespace"] = secret.Namespace
				metadata[key+".secretName"] = secret.SecretName
				secretCount++
			}
			if secretCount > 0 {
				metadata["testenv-lcr.imagePullSecretCount"] = fmt.Sprintf("%d", secretCount)
			}
		}
	}

	// Construct registry hostname (without port)
	registryHost := fmt.Sprintf("%s.%s.svc.cluster.local", Name, config.LocalContainerRegistry.Namespace)

	// Prepare environment variables to export for template expansion in subsequent testenv sub-engines.
	// These can be referenced in forge.yaml specs using {{.Env.VARIABLE_NAME}} syntax.
	// Example usage in testenv-helm-install:
	//   values:
	//     image:
	//       repository: "{{.Env.TESTENV_LCR_FQDN}}/myapp"
	envOutput := map[string]string{
		"TESTENV_LCR_FQDN":      registryFQDN,                            // Full registry address with port (e.g., testenv-lcr.testenv-lcr.svc.cluster.local:31906)
		"TESTENV_LCR_HOST":      registryHost,                            // Registry hostname without port (e.g., testenv-lcr.testenv-lcr.svc.cluster.local)
		"TESTENV_LCR_PORT":      fmt.Sprintf("%d", dynamicPort),          // Registry port (e.g., 31906)
		"TESTENV_LCR_NAMESPACE": config.LocalContainerRegistry.Namespace, // Kubernetes namespace (e.g., testenv-lcr)
		"TESTENV_LCR_CA_CERT":   caCrtPath,                               // Absolute path to CA certificate
	}

	return &engineframework.TestEnvArtifact{
		TestID:           input.TestID,
		Files:            files,
		Metadata:         metadata,
		ManagedResources: managedResources,
		Env:              envOutput,
	}, nil
}

// Delete implements the DeleteFunc for deleting a local container registry.
func Delete(ctx context.Context, input engineframework.DeleteInput, spec *Spec) error {
	log.Printf("Deleting local container registry: testID=%s", input.TestID)

	// Check if registry was enabled
	if enabled, ok := input.Metadata["testenv-lcr.enabled"]; ok && enabled == "false" {
		log.Printf("Local container registry was disabled, skipping teardown")
		return nil
	}

	// Stop port-forward if running for this testID
	portForwardersMu.Lock()
	if pf, ok := activePortForwarders[input.TestID]; ok {
		pf.Stop()
		delete(activePortForwarders, input.TestID)
		log.Printf("Stopped port-forward for testID=%s", input.TestID)
	}
	portForwardersMu.Unlock()

	config := configFrom(spec)

	// Check if local container registry is enabled
	if !config.LocalContainerRegistry.Enabled {
		log.Printf("Local container registry is disabled, skipping teardown")
		return nil
	}

	// Override kubeconfig path from metadata (if provided)
	// Note: DeleteInput doesn't have Env field, so we only use metadata
	if kubeconfigPath, ok := input.Metadata["testenv-kind.kubeconfigPath"]; ok {
		config.Kindenv.KubeconfigPath = kubeconfigPath
	}

	// Call the existing teardown logic (best-effort)
	if err := teardownWithConfig(config); err != nil {
		// Log error but don't fail - best effort cleanup
		log.Printf("Warning: failed to teardown local container registry: %v", err)
	}

	// Release port lease for this cluster (best-effort)
	if clusterName, ok := input.Metadata["testenv-kind.clusterName"]; ok && clusterName != "" {
		portLeaseManager := NewPortLeaseManager()
		if err := portLeaseManager.ReleasePort(clusterName); err != nil {
			log.Printf("Warning: failed to release port lease for cluster %s: %v", clusterName, err)
		} else {
			log.Printf("Released port lease for cluster %s", clusterName)
		}
	}

	return nil
}

// ----------------------------------------------------- SETUP/TEARDOWN ---------------------------------------------- //

var errSettingLocalContainerRegistry = errors.New("error received while setting up " + Name)

// setupWithConfig stands the registry up as this run's configuration says.
func setupWithConfig(config runConfig, dynamicPort int32) error {
	_, _ = fmt.Fprintln(os.Stdout, "Setting up "+Name)
	ctx := context.Background()

	if !config.LocalContainerRegistry.Enabled {
		_, _ = fmt.Fprintln(os.Stdout, Name+" is disabled")
		return nil
	}

	envs, err := readEnvs()
	if err != nil {
		return flaterrors.Join(err, errSettingLocalContainerRegistry)
	}

	eventualConfig := NewEventualConfig()

	// II. Create client.
	cl, err := createKubeClient(config)
	if err != nil {
		return flaterrors.Join(err, errSettingLocalContainerRegistry)
	}

	/// III. Initialize adapters
	containerRegistry := NewContainerRegistry(
		cl,
		config.LocalContainerRegistry.Namespace,
		eventualConfig,
	)

	// Set dynamic port if provided (from MCP handler via PortLeaseManager)
	if dynamicPort > 0 {
		containerRegistry.SetDynamicPort(dynamicPort)
	}
	k8s := NewK8s(cl, config.Kindenv.KubeconfigPath, config.LocalContainerRegistry.Namespace)

	cred := NewCredential(
		cl,
		envs.ContainerEngineExecutable,
		config.LocalContainerRegistry.CredentialPath,
		config.LocalContainerRegistry.Namespace,
		eventualConfig)

	tls := NewTLS(
		cl,
		config.LocalContainerRegistry.CaCrtPath,
		config.LocalContainerRegistry.Namespace,
		containerRegistry.FQDN(),
		config.Kindenv.KubeconfigPath,
		eventualConfig)

	// IV. Set up K8s
	if err := k8s.Setup(ctx); err != nil {
		return flaterrors.Join(err, errSettingLocalContainerRegistry)
	}

	// V. Set up credentials.
	if err := cred.Setup(ctx); err != nil {
		return flaterrors.Join(err, errSettingLocalContainerRegistry)
	}

	// VI. Set up TLS
	if err := tls.Setup(ctx); err != nil {
		return flaterrors.Join(err, errSettingLocalContainerRegistry)
	}

	// VII. Set up container registry in k8s
	if err := containerRegistry.Setup(ctx); err != nil {
		return flaterrors.Join(err, errSettingLocalContainerRegistry)
	}

	// VIII. Add /etc/hosts entry
	if err := addHostsEntry(containerRegistry.FQDN(), envs.ElevatedPrependCmd); err != nil {
		return flaterrors.Join(err, errSettingLocalContainerRegistry)
	}

	// IX. Create image pull secrets in configured namespaces
	if len(config.LocalContainerRegistry.ImagePullSecretNamespaces) > 0 {
		_, _ = fmt.Fprintf(os.Stdout, "Creating image pull secrets in %d namespace(s)\n",
			len(config.LocalContainerRegistry.ImagePullSecretNamespaces))

		// Read CA cert for image pull secret
		caCert, err := os.ReadFile(config.LocalContainerRegistry.CaCrtPath)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Warning: failed to read CA cert for image pull secrets: %s\n", err.Error())
		} else {
			// Include port in registry FQDN for Docker credential matching
			// Docker/containerd match credentials by full registry address including port
			registryFQDNWithPort := fmt.Sprintf("%s:%d", containerRegistry.FQDN(), containerRegistry.Port())
			imagePullSecret := NewImagePullSecret(
				cl,
				config.LocalContainerRegistry.ImagePullSecretName,
				registryFQDNWithPort,
				cred.credentials.Username,
				cred.credentials.Password,
				caCert,
			)

			created, err := imagePullSecret.CreateInNamespaces(ctx, config.LocalContainerRegistry.ImagePullSecretNamespaces)
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Warning: failed to create some image pull secrets: %s\n", err.Error())
			}

			for _, secretName := range created {
				_, _ = fmt.Fprintf(os.Stdout, "Created image pull secret: %s\n", secretName)
			}
		}
	}

	_, _ = fmt.Fprintln(os.Stdout, "Successfully set up "+Name)

	return nil
}

var errTearingDownLocalContainerRegistry = errors.New("error received while tearing down " + Name)

// teardownWithConfig tears the registry down as this run's configuration
// says: the kubeconfig came from the cluster engine's metadata, the
// namespace from the stage's spec.
func teardownWithConfig(config runConfig) error {
	_, _ = fmt.Fprintln(os.Stdout, "Tearing down "+Name)

	ctx := context.Background()

	envs, err := readEnvs()
	if err != nil {
		return flaterrors.Join(err, errTearingDownLocalContainerRegistry)
	}

	// II. Create client.
	cl, err := createKubeClient(config)
	if err != nil {
		return flaterrors.Join(err, errTearingDownLocalContainerRegistry)
	}

	// III. Initialize adapters
	k8s := NewK8s(cl, config.Kindenv.KubeconfigPath, config.LocalContainerRegistry.Namespace)
	containerRegistry := NewContainerRegistry(cl, config.LocalContainerRegistry.Namespace, nil)

	tls := NewTLS(
		cl,
		config.LocalContainerRegistry.CaCrtPath,
		config.LocalContainerRegistry.Namespace,
		containerRegistry.FQDN(),
		config.Kindenv.KubeconfigPath,
		nil)

	// IV. Delete image pull secrets (best effort)
	_, _ = fmt.Fprintln(os.Stdout, "Cleaning up image pull secrets")
	secrets, err := ListImagePullSecrets(ctx, cl, "")
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Warning: failed to list image pull secrets: %v\n", err)
	} else {
		for _, secret := range secrets {
			secretObj := &corev1.Secret{}
			secretObj.Name = secret.SecretName
			secretObj.Namespace = secret.Namespace

			if err := cl.Delete(ctx, secretObj); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Warning: failed to delete image pull secret %s/%s: %v\n",
					secret.Namespace, secret.SecretName, err)
			} else {
				_, _ = fmt.Fprintf(os.Stdout, "Deleted image pull secret: %s/%s\n",
					secret.Namespace, secret.SecretName)
			}
		}
	}

	// V. Tear down K8s
	if err := k8s.Teardown(ctx); err != nil {
		return flaterrors.Join(err, errTearingDownLocalContainerRegistry)
	}

	// VI. Tear down TLS
	if err := tls.Teardown(); err != nil {
		return flaterrors.Join(err, errTearingDownLocalContainerRegistry)
	}

	// VII. Remove /etc/hosts entry
	if err := removeHostsEntry(containerRegistry.FQDN(), envs.ElevatedPrependCmd); err != nil {
		return flaterrors.Join(err, errTearingDownLocalContainerRegistry)
	}

	_, _ = fmt.Fprintln(os.Stdout, "Torn down "+Name+" successfully")

	return nil
}

// ----------------------------------------------------- KUBERNETES CLIENT ------------------------------------------- //

var errCreatingKubernetesClient = errors.New("creating kubernetes client")

// createKubeClient creates a new Kubernetes client from the kubeconfig file specified in the project configuration.
func createKubeClient(config runConfig) (client.Client, error) { //nolint:ireturn
	b, err := os.ReadFile(config.Kindenv.KubeconfigPath)
	if err != nil {
		return nil, flaterrors.Join(err, errCreatingKubernetesClient)
	}

	restConfig, err := clientcmd.RESTConfigFromKubeConfig(b)
	if err != nil {
		return nil, flaterrors.Join(err, errCreatingKubernetesClient)
	}

	sch := runtime.NewScheme()

	if err := flaterrors.Join(
		appsv1.AddToScheme(sch),
		corev1.AddToScheme(sch),
		certmanagerv1.AddToScheme(sch),
	); err != nil {
		return nil, flaterrors.Join(err, errCreatingKubernetesClient)
	}

	cl, err := client.New(restConfig, client.Options{Scheme: sch}) //nolint:exhaustruct
	if err != nil {
		return nil, flaterrors.Join(err, errCreatingKubernetesClient)
	}

	return cl, nil
}
