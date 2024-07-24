package main

import (
	"context"
	"fmt"
	"os"

	"github.com/alexandremahdhaoui/tooling/pkg/project"
	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	Name = "local-container-registry"
)

// TODO: implement me:
//  - The point of this binary is to set up a local container registry onto which we may push container images.
//  - Once images are pushed to the registry they can be used in the kindenv and for chart-testing.
//  - Finally, the binary should also take care of cleaning up the registry.
// Consideration: should the local-container-registry run as a container in the default namespace we must ensure
// connectivity between pods and the registry.

func main() {
	// teardown
	if len(os.Args) > 1 && os.Args[1] == "teardown" {
		if err := teardown(); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "❌ %s\n", err.Error())
			os.Exit(1)
		}

		os.Exit(0)
	}

	if err := setup(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "❌ %s\n", err.Error())

		if err := teardown(); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "❌ %s\n", err.Error())
		}

		os.Exit(1)
	}
}

func setup() error {
	_, _ = fmt.Fprintln(os.Stdout, "⏳ Setting up "+Name)
	ctx := context.Background()

	// I. Read project config
	config, err := project.ReadConfig()
	if err != nil {
		return err // TODO: wrap err
	}

	if !config.LocalContainerRegistry.Enabled {
		_, _ = fmt.Fprintln(os.Stdout, Name+" is disabled")
		return nil
	}

	// II. Create client.
	cl, err := createKubeClient(config)
	if err != nil {
		return err // TODO: wrap err
	}

	/// III. Initialize adapters
	containerRegistry := NewContainerRegistry(cl, config.LocalContainerRegistry.Namespace)
	k8s := NewK8s(cl, config.Kindenv.KubeconfigPath, config.LocalContainerRegistry.Namespace)

	cred := NewCredential(
		cl,
		config.LocalContainerRegistry.CredentialPath,
		config.LocalContainerRegistry.Namespace)

	tls := NewTLS(
		cl,
		config.LocalContainerRegistry.CaCrtPath,
		config.LocalContainerRegistry.Namespace,
		containerRegistry.ServiceFQDN())

	// IV. Set up K8s
	if err := k8s.Setup(ctx); err != nil {
		return err // TODO: wrap err
	}

	// V. Set up credentials.
	if err := cred.Setup(ctx); err != nil {
		return err // TODO: wrap err
	}

	// VI. Set up TLS
	if err := tls.Setup(ctx); err != nil {
		return err // TODO: wrap err
	}

	// VII. Set up container registry in k8s
	if err := containerRegistry.Setup(ctx); err != nil {
		return err // TODO: wrap err
	}

	// How to make required images available in the container registry?

	_, _ = fmt.Fprintln(os.Stdout, "✅ Successfully set up "+Name)

	return nil
}

func teardown() error {
	_, _ = fmt.Fprintln(os.Stdout, "⏳ Tearing down "+Name)

	ctx := context.Background()

	// I. Read project config
	config, err := project.ReadConfig()
	if err != nil {
		return err // TODO: wrap err
	}

	// II. Create client.
	cl, err := createKubeClient(config)
	if err != nil {
		return err // TODO: wrap err
	}

	// III. Initialize adapters
	k8s := NewK8s(cl, config.Kindenv.KubeconfigPath, config.LocalContainerRegistry.Namespace)
	containerRegistry := NewContainerRegistry(cl, config.LocalContainerRegistry.Namespace)

	tls := NewTLS(
		cl,
		config.LocalContainerRegistry.CaCrtPath,
		config.LocalContainerRegistry.Namespace,
		containerRegistry.ServiceFQDN())

	// III. Tear down K8s
	if err := k8s.Teardown(ctx); err != nil {
		return err // TODO: wrap err
	}

	// IV. Tear down TLS
	if err := tls.Teardown(); err != nil {
		return err // TODO: wrap err
	}

	_, _ = fmt.Fprintln(os.Stdout, "✅ Torn down "+Name+" successfully")

	return nil
}

func createKubeClient(config project.Config) (client.Client, error) { //nolint:ireturn
	b, err := os.ReadFile(config.Kindenv.KubeconfigPath)
	if err != nil {
		return nil, err // TODO: wrap err
	}

	restConfig, err := clientcmd.RESTConfigFromKubeConfig(b)
	if err != nil {
		return nil, err // TODO: wrap err
	}

	sch := runtime.NewScheme()

	if err := corev1.AddToScheme(sch); err != nil {
		return nil, err // TODO: wrap err
	}

	if err := certmanagerv1.AddToScheme(sch); err != nil {
		return nil, err // TODO: wrap err
	}

	cl, err := client.New(restConfig, client.Options{Scheme: sch})
	if err != nil {
		return nil, err // TODO: wrap err
	}

	return cl, nil
}
