package main

import (
	"context"
	"errors"
	"log"
	"os"

	"github.com/alexandremahdhaoui/forge/pkg/flaterrors"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// K8s is a struct that manages the setup of the Kubernetes cluster for the local container registry.
type K8s struct {
	client         client.Client
	kubeconfigPath string
	namespace      string
}

// NewK8s creates a new K8s struct.
func NewK8s(cl client.Client, kubeconfigPath, namespace string) *K8s {
	return &K8s{
		client:         cl,
		kubeconfigPath: kubeconfigPath,
		namespace:      namespace,
	}
}

var errSettingUpK8sCluster = errors.New("setting up k8s cluster")

// Setup sets up the Kubernetes cluster for the local container registry.
// It creates the namespace if it doesn't exist and sets the KUBECONFIG environment variable.
func (k *K8s) Setup(ctx context.Context) error {
	// 1. create the local-container-registry namespace if it doesn't exist
	ns := corev1.Namespace{}
	ns.Name = k.namespace

	// Check if namespace already exists
	existingNs := &corev1.Namespace{}
	err := k.client.Get(ctx, client.ObjectKey{Name: k.namespace}, existingNs)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Namespace doesn't exist, create it
			if err := k.client.Create(ctx, &ns); err != nil {
				return flaterrors.Join(err, errSettingUpK8sCluster)
			}
			log.Printf("Created namespace: %s", k.namespace)
		} else {
			// Other error occurred
			return flaterrors.Join(err, errSettingUpK8sCluster)
		}
	} else {
		// Namespace already exists
		log.Printf("Namespace already exists: %s (skipping creation)", k.namespace)
	}

	// 2. set kubeconfig for the kubectl subprocess which applies the cert manager config
	if err := os.Setenv("KUBECONFIG", k.kubeconfigPath); err != nil {
		return flaterrors.Join(err, errSettingUpK8sCluster)
	}

	return nil
}

var errTearingDownK8sCluster = errors.New("tearing down k8s cluster")

// Teardown tears down the Kubernetes cluster for the local container registry.
// It deletes the namespace and sets the KUBECONFIG environment variable.
func (k *K8s) Teardown(ctx context.Context) error {
	ns := &corev1.Namespace{}
	ns.Name = k.namespace

	if err := k.client.Delete(ctx, ns); !apierrors.IsNotFound(err) && err != nil {
		return flaterrors.Join(err, errTearingDownK8sCluster)
	}

	// 2. set kubeconfig for the kubectl subprocess which applies the cert manager config
	if err := os.Setenv("KUBECONFIG", k.kubeconfigPath); err != nil {
		return flaterrors.Join(err, errTearingDownK8sCluster)
	}

	return nil
}
