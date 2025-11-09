package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/caarlos0/env/v11"
	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/alexandremahdhaoui/forge/internal/util"
	"github.com/alexandremahdhaoui/forge/internal/version"
	"github.com/alexandremahdhaoui/forge/pkg/flaterrors"
	"github.com/alexandremahdhaoui/forge/pkg/forge"
)

const (
	Name = "testenv-lcr"
)

// Version information (set via ldflags during build)
var (
	Version        = "dev"
	CommitSHA      = "unknown"
	BuildTimestamp = "unknown"
)

// versionInfo holds testenv-lcr's version information
var versionInfo *version.Info

func init() {
	versionInfo = version.New(Name)
	versionInfo.Version = Version
	versionInfo.CommitSHA = CommitSHA
	versionInfo.BuildTimestamp = BuildTimestamp
}

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

// ----------------------------------------------------- MAIN ------------------------------------------------------- //

func main() {
	// Command parsing
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--mcp":
			// Run in MCP server mode
			if err := runMCPServer(); err != nil {
				log.Printf("MCP server error: %v", err)
				os.Exit(1)
			}
			return

		case "version", "--version", "-v":
			versionInfo.Print()
			return

		case "teardown":
			if err := teardown(); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "❌ %s\n", err.Error())
				os.Exit(1)
			}
			os.Exit(0)

		case "push":
			if len(os.Args) < 3 {
				_, _ = fmt.Fprintf(os.Stderr, "❌ Error: push command requires an image name\n")
				_, _ = fmt.Fprintf(os.Stderr, "Usage: %s push <image-name>\n", os.Args[0])
				os.Exit(1)
			}
			if err := push(os.Args[2]); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "❌ %s\n", err.Error())
				os.Exit(1)
			}
			os.Exit(0)

		case "push-all":
			if err := pushAll(); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "❌ %s\n", err.Error())
				os.Exit(1)
			}
			os.Exit(0)

		case "create-image-pull-secret":
			if len(os.Args) < 3 {
				_, _ = fmt.Fprintf(os.Stderr, "❌ Error: create-image-pull-secret command requires a namespace\n")
				_, _ = fmt.Fprintf(os.Stderr, "Usage: %s create-image-pull-secret <namespace> [secret-name]\n", os.Args[0])
				os.Exit(1)
			}
			namespace := os.Args[2]
			secretName := ""
			if len(os.Args) > 3 {
				secretName = os.Args[3]
			}
			if err := createImagePullSecret(namespace, secretName); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "❌ %s\n", err.Error())
				os.Exit(1)
			}
			os.Exit(0)

		case "list-image-pull-secrets":
			namespace := ""
			if len(os.Args) > 2 {
				namespace = os.Args[2]
			}
			if err := listImagePullSecrets(namespace); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "❌ %s\n", err.Error())
				os.Exit(1)
			}
			os.Exit(0)
		}
	}

	// Default: setup
	if err := setup(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "❌ %s\n", err.Error())

		if err := teardown(); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "❌ %s\n", err.Error())
		}

		os.Exit(1)
	}
}

var errSettingLocalContainerRegistry = errors.New("error received while setting up " + Name)

// setup executes the main logic of the `local-container-registry setup` command.
// It reads the project configuration (or uses provided config), creates a Kubernetes client, and sets up the local container registry.
func setup() error {
	return setupWithConfig(nil)
}

// setupWithConfig executes the setup logic with an optional pre-loaded config.
// If cfg is nil, it reads the config from forge.yaml.
func setupWithConfig(cfg *forge.Spec) error {
	_, _ = fmt.Fprintln(os.Stdout, "⏳ Setting up "+Name)
	ctx := context.Background()

	// I. Read config
	var config forge.Spec
	var err error
	if cfg != nil {
		config = *cfg
	} else {
		config, err = forge.ReadSpec()
		if err != nil {
			return flaterrors.Join(err, errSettingLocalContainerRegistry)
		}
	}

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

	// IX. Wait for registry deployment to be ready before auto-pushing
	if config.LocalContainerRegistry.AutoPushImages && len(config.Build) > 0 {
		_, _ = fmt.Fprintln(os.Stdout, "⏳ Waiting for registry to be ready")
		waitCmd := exec.Command(
			"kubectl",
			"wait",
			"--for=condition=available",
			"--timeout=60s",
			"-n", config.LocalContainerRegistry.Namespace,
			"deployment/"+Name,
		)
		waitCmd.Env = append(
			os.Environ(),
			fmt.Sprintf("KUBECONFIG=%s", config.Kindenv.KubeconfigPath),
		)
		if err := util.RunCmdWithStdPipes(waitCmd); err != nil {
			_, _ = fmt.Fprintf(
				os.Stderr,
				"⚠️  Warning: registry deployment not ready: %s\n",
				err.Error(),
			)
		} else {
			if err := pushImagesFromArtifactStore(ctx, config, envs); err != nil {
				// Log warning but don't fail setup if push fails
				_, _ = fmt.Fprintf(os.Stderr, "⚠️  Warning: failed to auto-push images: %s\n", err.Error())
			}
		}
	}

	// X. Create image pull secrets in configured namespaces
	if len(config.LocalContainerRegistry.ImagePullSecretNamespaces) > 0 {
		_, _ = fmt.Fprintf(os.Stdout, "⏳ Creating image pull secrets in %d namespace(s)\n",
			len(config.LocalContainerRegistry.ImagePullSecretNamespaces))

		// Read CA cert for image pull secret
		caCert, err := os.ReadFile(config.LocalContainerRegistry.CaCrtPath)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "⚠️  Warning: failed to read CA cert for image pull secrets: %s\n", err.Error())
		} else {
			imagePullSecret := NewImagePullSecret(
				cl,
				config.LocalContainerRegistry.ImagePullSecretName,
				containerRegistry.FQDN(),
				cred.credentials.Username,
				cred.credentials.Password,
				caCert,
			)

			created, err := imagePullSecret.CreateInNamespaces(ctx, config.LocalContainerRegistry.ImagePullSecretNamespaces)
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "⚠️  Warning: failed to create some image pull secrets: %s\n", err.Error())
			}

			for _, secretName := range created {
				_, _ = fmt.Fprintf(os.Stdout, "✅ Created image pull secret: %s\n", secretName)
			}
		}
	}

	_, _ = fmt.Fprintln(os.Stdout, "✅ Successfully set up "+Name)

	return nil
}

var errTearingDownLocalContainerRegistry = errors.New("error received while tearing down " + Name)

// teardown executes the main logic of the `local-container-registry teardown` command.
// It reads the project configuration, creates a Kubernetes client, and tears down the local container registry.
func teardown() error {
	_, _ = fmt.Fprintln(os.Stdout, "⏳ Tearing down "+Name)

	ctx := context.Background()

	// I. Read project config
	config, err := forge.ReadSpec()
	if err != nil {
		return flaterrors.Join(err, errTearingDownLocalContainerRegistry)
	}

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
		containerRegistry.FQDN(), nil)

	// IV. Delete image pull secrets (best effort)
	_, _ = fmt.Fprintln(os.Stdout, "⏳ Cleaning up image pull secrets")
	secrets, err := ListImagePullSecrets(ctx, cl, "")
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "⚠️  Warning: failed to list image pull secrets: %v\n", err)
	} else {
		for _, secret := range secrets {
			secretObj := &corev1.Secret{}
			secretObj.Name = secret.SecretName
			secretObj.Namespace = secret.Namespace

			if err := cl.Delete(ctx, secretObj); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "⚠️  Warning: failed to delete image pull secret %s/%s: %v\n",
					secret.Namespace, secret.SecretName, err)
			} else {
				_, _ = fmt.Fprintf(os.Stdout, "✅ Deleted image pull secret: %s/%s\n",
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

	_, _ = fmt.Fprintln(os.Stdout, "✅ Torn down "+Name+" successfully")

	return nil
}

var errPushingImage = errors.New("error received while pushing image to " + Name)

// push executes the main logic of the `local-container-registry push <image>` command.
// It pushes a single image to the local container registry.
func push(imageName string) error {
	ctx := context.Background()

	config, err := forge.ReadSpec()
	if err != nil {
		return flaterrors.Join(err, errPushingImage)
	}

	envs, err := readEnvs()
	if err != nil {
		return flaterrors.Join(err, errPushingImage)
	}

	return pushSingleImage(ctx, config, envs, imageName)
}

var errPushingAllImages = errors.New("error received while pushing all images to " + Name)

// pushAll executes the main logic of the `local-container-registry push-all` command.
// It pushes all container images defined in the project configuration from the artifact store.
func pushAll() error {
	ctx := context.Background()

	config, err := forge.ReadSpec()
	if err != nil {
		return flaterrors.Join(err, errPushingAllImages)
	}

	envs, err := readEnvs()
	if err != nil {
		return flaterrors.Join(err, errPushingAllImages)
	}

	return pushImagesFromArtifactStore(ctx, config, envs)
}

var errCreatingImagePullSecretCLI = errors.New("error received while creating image pull secret via CLI")

// createImagePullSecret executes the main logic of the `testenv-lcr create-image-pull-secret <namespace> [secret-name]` command.
// It creates an image pull secret in the specified namespace.
func createImagePullSecret(namespace, secretName string) error {
	ctx := context.Background()

	config, err := forge.ReadSpec()
	if err != nil {
		return flaterrors.Join(err, errCreatingImagePullSecretCLI)
	}

	if !config.LocalContainerRegistry.Enabled {
		_, _ = fmt.Fprintln(os.Stdout, "Local container registry is disabled")
		return nil
	}

	// Create Kubernetes client
	cl, err := createKubeClient(config)
	if err != nil {
		return flaterrors.Join(err, errCreatingImagePullSecretCLI)
	}

	// Read credentials
	credBytes, err := os.ReadFile(config.LocalContainerRegistry.CredentialPath)
	if err != nil {
		return flaterrors.Join(err, errCreatingImagePullSecretCLI)
	}

	var credentials Credentials
	if err := yaml.Unmarshal(credBytes, &credentials); err != nil {
		return flaterrors.Join(err, errCreatingImagePullSecretCLI)
	}

	// Read CA certificate
	caCert, err := os.ReadFile(config.LocalContainerRegistry.CaCrtPath)
	if err != nil {
		return flaterrors.Join(err, errCreatingImagePullSecretCLI)
	}

	// Create container registry to get FQDN
	containerRegistry := NewContainerRegistry(cl, config.LocalContainerRegistry.Namespace, nil)
	registryFQDN := containerRegistry.FQDN()

	// Use provided secret name or default from config
	if secretName == "" {
		secretName = config.LocalContainerRegistry.ImagePullSecretName
	}

	// Create image pull secret
	imagePullSecret := NewImagePullSecret(
		cl,
		secretName,
		registryFQDN,
		credentials.Username,
		credentials.Password,
		caCert,
	)

	secretFullName, err := imagePullSecret.CreateInNamespace(ctx, namespace)
	if err != nil {
		return flaterrors.Join(err, errCreatingImagePullSecretCLI)
	}

	_, _ = fmt.Fprintf(os.Stdout, "✅ Created image pull secret: %s\n", secretFullName)
	return nil
}

var errListingImagePullSecrets = errors.New("error received while listing image pull secrets")

// listImagePullSecrets executes the main logic of the `testenv-lcr list-image-pull-secrets [namespace]` command.
// It lists all image pull secrets created by testenv-lcr, optionally filtered by namespace.
func listImagePullSecrets(namespace string) error {
	ctx := context.Background()

	config, err := forge.ReadSpec()
	if err != nil {
		return flaterrors.Join(err, errListingImagePullSecrets)
	}

	// Create Kubernetes client
	cl, err := createKubeClient(config)
	if err != nil {
		return flaterrors.Join(err, errListingImagePullSecrets)
	}

	// List image pull secrets
	secrets, err := ListImagePullSecrets(ctx, cl, namespace)
	if err != nil {
		return flaterrors.Join(err, errListingImagePullSecrets)
	}

	if len(secrets) == 0 {
		_, _ = fmt.Fprintln(os.Stdout, "No image pull secrets found")
		return nil
	}

	_, _ = fmt.Fprintf(os.Stdout, "Found %d image pull secret(s):\n", len(secrets))
	for _, secret := range secrets {
		_, _ = fmt.Fprintf(os.Stdout, "  - %s/%s (created: %v)\n", secret.Namespace, secret.SecretName, secret.CreatedAt)
	}

	return nil
}

var errCreatingKubernetesClient = errors.New("creating kubernetes client")

// createKubeClient creates a new Kubernetes client from the kubeconfig file specified in the project configuration.
func createKubeClient(config forge.Spec) (client.Client, error) { //nolint:ireturn
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
