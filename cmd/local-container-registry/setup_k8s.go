package main

import (
	"context"
	"errors"
	"os"

	"github.com/alexandremahdhaoui/tooling/pkg/flaterrors"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type K8s struct {
	client         client.Client
	kubeconfigPath string
	namespace      string
}

func NewK8s(cl client.Client, kubeconfigPath, namespace string) *K8s {
	return &K8s{
		client:         cl,
		kubeconfigPath: kubeconfigPath,
		namespace:      namespace,
	}
}

var errSettingUpK8sCluster = errors.New("setting up k8s cluster")

func (k *K8s) Setup(ctx context.Context) error {
	// 1. create the local-container-registry namespace
	ns := corev1.Namespace{}
	ns.Name = k.namespace

	if err := k.client.Create(ctx, &ns); !apierrors.IsAlreadyExists(err) && err != nil {
		return flaterrors.Join(err, errSettingUpK8sCluster)
	}

	// 2. set kubeconfig for the kubectl subprocess which applies the cert manager config
	if err := os.Setenv("KUBECONFIG", k.kubeconfigPath); err != nil {
		return flaterrors.Join(err, errSettingUpK8sCluster)
	}

	return nil
}

var errTearingDownK8sCluster = errors.New("tearing down k8s cluster")

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
