package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"text/template"
	"time"

	"k8s.io/apimachinery/pkg/types"

	"github.com/alexandremahdhaoui/forge/pkg/eventualconfig"
	"github.com/alexandremahdhaoui/forge/pkg/flaterrors"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	containerRegistryImage = "docker.io/registry:2"
	containerRegistryPort  = 5000

	registryConfigConfigMapName = Name + "-config"
	registryConfigFilename      = "config.yml"
	registryConfigMountDir      = "/etc/docker/registry"
)

// ContainerRegistry is a struct that manages the setup of the local container registry.
type ContainerRegistry struct {
	client    client.Client
	namespace string

	ec eventualconfig.EventualConfig
}

// NewContainerRegistry creates a new ContainerRegistry struct.
func NewContainerRegistry(cl client.Client, namespace string, ec eventualconfig.EventualConfig) *ContainerRegistry {
	return &ContainerRegistry{
		client:    cl,
		namespace: namespace,
		ec:        ec,
	}
}

var errSettingUpContainerRegistry = errors.New("setting up container registry")

// Setup sets up the local container registry.
// It creates a ConfigMap, a Service, a Deployment, and awaits the Deployment's readiness.
func (r *ContainerRegistry) Setup(ctx context.Context) error {
	labels := map[string]string{"app": Name}

	// I. Create ConfigMap
	if err := r.createConfigMap(ctx, labels); err != nil {
		return flaterrors.Join(err, errSettingUpContainerRegistry)
	}

	// II. Create Service.
	if err := r.createService(ctx, labels); err != nil {
		return flaterrors.Join(err, errSettingUpContainerRegistry)
	}

	// III. Create Deployment.
	if err := r.createDeployment(ctx, labels); err != nil {
		return flaterrors.Join(err, errSettingUpContainerRegistry)
	}

	// IV. Await Deployment readiness.
	if err := r.awaitDeployment(ctx); err != nil {
		return flaterrors.Join(err, errSettingUpContainerRegistry)
	}

	return nil
}

var errCreatingDeployment = errors.New("creating deployment")

//nolint:funlen // long deployment struct.
func (r *ContainerRegistry) createDeployment(ctx context.Context, labels map[string]string) error {
	// I. Read EventualConfig.
	var errs error

	credName, err := eventualconfig.AwaitValue[string](r.ec, CredentialSecretName)
	errs = flaterrors.Join(errs, err)

	tlsSecretName, err := eventualconfig.AwaitValue[string](r.ec, TLSSecretName)
	errs = flaterrors.Join(errs, err)

	credMount, err := eventualconfig.AwaitValue[Mount](r.ec, CredentialMount)
	errs = flaterrors.Join(errs, err)

	tlsMount, err := eventualconfig.AwaitValue[Mount](r.ec, TLSKey)
	errs = flaterrors.Join(errs, err)

	if errs != nil {
		return flaterrors.Join(err, errCreatingDeployment)
	}

	// II. Secret volume sources.
	credVol := "credentials"
	regVol := "registry-config"
	tlsVol := "tls"

	credVolSrc := corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{
		SecretName: credName,
	}}

	regVolSrc := corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{
		LocalObjectReference: corev1.LocalObjectReference{Name: r.ConfigMapName()},
	}}

	tlsVolSrc := corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{
		SecretName: tlsSecretName,
	}}

	// III. Deployment.
	deployment := &appsv1.Deployment{}
	deployment.Name = Name
	deployment.Namespace = r.namespace

	deployment.Spec = appsv1.DeploymentSpec{ //nolint:exhaustruct
		Replicas: ptr.To[int32](1),
		Selector: &metav1.LabelSelector{MatchLabels: labels}, //nolint:exhaustruct
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{Labels: labels}, //nolint:exhaustruct
			Spec: corev1.PodSpec{ //nolint:exhaustruct
				Containers: []corev1.Container{{
					Name:  Name,
					Image: containerRegistryImage,

					VolumeMounts: []corev1.VolumeMount{{
						MountPath: credMount.Dir,
						Name:      credVol,
						ReadOnly:  true,
					}, {
						MountPath: r.Mount().Dir,
						Name:      regVol,
						ReadOnly:  true,
					}, {
						MountPath: tlsMount.Dir,
						Name:      tlsVol,
						ReadOnly:  true,
					}},

					Ports: []corev1.ContainerPort{{
						Name:          "https",
						ContainerPort: containerRegistryPort,
						Protocol:      corev1.ProtocolTCP,
					}},
				}},

				Volumes: []corev1.Volume{{
					Name:         credVol,
					VolumeSource: credVolSrc,
				}, {
					Name:         regVol,
					VolumeSource: regVolSrc,
				}, {
					Name:         tlsVol,
					VolumeSource: tlsVolSrc,
				}},

				RestartPolicy: corev1.RestartPolicyAlways,
			},
		},
	}

	// IV. Create.
	if err := r.client.Create(ctx, deployment); err != nil {
		return flaterrors.Join(err, errCreatingDeployment)
	}

	return nil
}

var errAwaitingDeploymentReadiness = errors.New("awaiting deployment readiness")

func (r *ContainerRegistry) awaitDeployment(ctx context.Context) error {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return flaterrors.Join(ctx.Err(), errAwaitingDeploymentReadiness)
		case <-ticker.C:
			deploy := &appsv1.Deployment{} //nolint:exhaustruct
			nsName := types.NamespacedName{
				Namespace: r.namespace,
				Name:      Name,
			}

			if err := r.client.Get(ctx, nsName, deploy); err != nil {
				return flaterrors.Join(err, errAwaitingDeploymentReadiness)
			}

			if deploy.Status.ReadyReplicas > 0 {
				return nil
			}
		}
	}
}

var errCreatingService = errors.New("creating service")

func (r *ContainerRegistry) createService(ctx context.Context, labels map[string]string) error {
	service := &corev1.Service{} //nolint:exhaustruct

	service.Name = Name
	service.Namespace = r.namespace
	service.Labels = labels

	service.Spec.Selector = labels
	service.Spec.Ports = []corev1.ServicePort{{
		Name: "https",
		Port: r.Port(),
	}}

	if err := r.client.Create(ctx, service); err != nil {
		return flaterrors.Join(err, errCreatingService)
	}

	return nil
}

// FQDN returns the fully qualified domain name of the container registry service.
func (r *ContainerRegistry) FQDN() string {
	return fmt.Sprintf("%s.%s.svc.cluster.local", Name, r.namespace)
}

// Port returns the port of the container registry service.
func (r *ContainerRegistry) Port() int32 {
	return containerRegistryPort
}

// ConfigMapName returns the name of the ConfigMap for the container registry.
func (r *ContainerRegistry) ConfigMapName() string {
	return registryConfigConfigMapName
}

// Mount returns the Mount struct for the container registry configuration.
func (r *ContainerRegistry) Mount() Mount {
	return Mount{
		Dir:      registryConfigMountDir,
		Filename: registryConfigFilename,
	}
}

// -- Container registry config

type registryConfig struct {
	FQDN string
	Port int32

	CredentialPath string

	CACertPath     string
	ServerCertPath string
	ServerKeyPath  string
}

const registryConfigTemplate = `version: 0.1
auth:
  htpasswd:
    realm: basic-realm
    path: {{ .CredentialPath }}

http:
  addr: 0.0.0.0:{{ .Port }}
  host: https://{{ .FQDN }}:{{ .Port }}
  tls:
    certificate: {{ .ServerCertPath }}
    key: {{ .ServerKeyPath }}
#    clientcas: # This fields enables mTLS.
#      - {{ .CACertPath }}

storage:
  filesystem:
    rootdirectory: /var/lib/registry
`

var errCreatingConfigMap = errors.New("creating configmap")

// createConfigMap will template the registry config and create a config map in k8s. This ConfigMap will be later
// mounted to the local-container-registry pod.
func (r *ContainerRegistry) createConfigMap(ctx context.Context, labels map[string]string) error {
	// I. get registry config
	var errs error

	credMount, err := eventualconfig.AwaitValue[Mount](r.ec, CredentialMount)
	errs = flaterrors.Join(errs, err)

	caCert, err := eventualconfig.AwaitValue[Mount](r.ec, TLSCACert)
	errs = flaterrors.Join(errs, err)

	tlsCert, err := eventualconfig.AwaitValue[Mount](r.ec, TLSCert)
	errs = flaterrors.Join(errs, err)

	tlsKey, err := eventualconfig.AwaitValue[Mount](r.ec, TLSKey)
	errs = flaterrors.Join(errs, err)

	if errs != nil {
		return flaterrors.Join(err, errCreatingConfigMap)
	}

	config := registryConfig{
		FQDN:           r.FQDN(),
		Port:           r.Port(),
		CredentialPath: credMount.Path(),
		CACertPath:     caCert.Path(),
		ServerCertPath: tlsCert.Path(),
		ServerKeyPath:  tlsKey.Path(),
	}

	// II. Template file.
	buf := bytes.NewBuffer(make([]byte, 0))

	tmpl, err := template.New("").Parse(registryConfigTemplate)
	if err != nil {
		return flaterrors.Join(err, errCreatingConfigMap)
	}

	if err := tmpl.Execute(buf, config); err != nil {
		return flaterrors.Join(err, errCreatingConfigMap)
	}

	// III. Create config map
	cm := &corev1.ConfigMap{} //nolint:exhaustruct

	cm.Name = registryConfigConfigMapName
	cm.Namespace = r.namespace
	cm.Labels = labels

	templatedConfig, err := io.ReadAll(buf)
	if err != nil {
		return flaterrors.Join(err, errCreatingConfigMap)
	}

	cm.Data = map[string]string{
		r.Mount().Filename: string(templatedConfig),
	}

	if err := r.client.Create(ctx, cm); err != nil {
		return flaterrors.Join(err, errCreatingConfigMap)
	}

	return nil
}
