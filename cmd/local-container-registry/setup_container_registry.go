package main

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	containerRegistryImage = "docker.io/registry:2"
	containerRegistryPort  = 5000
)

type ContainerRegistry struct {
	client    client.Client
	namespace string
}

func NewContainerRegistry(cl client.Client, namespace string) *ContainerRegistry {
	return &ContainerRegistry{
		client:    cl,
		namespace: namespace,
	}
}

func (r *ContainerRegistry) Setup(ctx context.Context) error {
	podLabels := map[string]string{"app": Name}

	// I. Create Service.
	if err := r.createService(ctx, podLabels); err != nil {
		return err // TODO: wrap err
	}

	// II. Create Deployment.
	if err := r.createDeployment(ctx, podLabels); err != nil {
		return err // TODO: wrap err
	}

	return nil
}

func (r *ContainerRegistry) createDeployment(ctx context.Context, podLabels map[string]string) error {
	// I. Secret volume sources
	tlsSecretVolumeSource := corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{
		SecretName: "TODO", // TODO
	}}

	credentialsVolumeSource := corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{
		SecretName: credentialsSecretName,
	}}

	// II. Deployment.
	deployment := &appsv1.Deployment{}
	deployment.Name = Name
	deployment.Namespace = r.namespace

	deployment.Spec = appsv1.DeploymentSpec{
		Replicas: ptr.To[int32](1),
		Selector: &metav1.LabelSelector{MatchLabels: podLabels},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{Labels: podLabels},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{
					Name:  Name,
					Image: containerRegistryImage,
					VolumeMounts: []corev1.VolumeMount{{
						MountPath: "/tls",
						Name:      "tls",
						ReadOnly:  true,
					}, {
						MountPath: "/credentials",
						Name:      "credentials",
						ReadOnly:  true,
					}},
					Ports: []corev1.ContainerPort{{
						Name:          "https",
						ContainerPort: containerRegistryPort,
						Protocol:      corev1.ProtocolTCP,
					}},
					Env: []corev1.EnvVar{{Name: "TODO"}}, // TODO
				}},
				Volumes: []corev1.Volume{{
					Name:         "tls",
					VolumeSource: tlsSecretVolumeSource,
				}, {
					Name:         "credentials",
					VolumeSource: credentialsVolumeSource,
				}},
				RestartPolicy: corev1.RestartPolicyAlways,
			},
		},
	}

	// III. Create.
	if err := r.client.Create(ctx, deployment); err != nil {
		return err // TODO: wrap err
	}

	// IV. Await readiness.
	// TODO

	return nil
}

func (r *ContainerRegistry) createService(ctx context.Context, podLabels map[string]string) error {
	service := &corev1.Service{}

	service.Name = Name
	service.Namespace = r.namespace

	service.Spec.Selector = podLabels
	service.Spec.Ports = []corev1.ServicePort{{
		Name: "https",
		Port: containerRegistryPort,
	}}
	// TODO: Service "local-container-registry" is invalid: spec.ports: Required value

	if err := r.client.Create(ctx, service); err != nil {
		return err // TODO: wrap err
	}

	return nil
}

func (r *ContainerRegistry) ServiceFQDN() string {
	return fmt.Sprintf("%s.%s.svc.cluster.local", Name, r.namespace)
}

// -- Container registry config

const containerRegistryConfigTemplate = `
auth:
  silly:
     realm: silly-realm
     service: silly-service
    token:
      autoredirect: true
      realm: token-realm
      service: token-service
      issuer: registry-token-issuer
      rootcertbundle: /root/certs/bundle
      jwks: /path/to/jwks
      signingalgorithms:
        - EdDSA
        - HS256
    htpasswd:
      realm: basic-realm
      path: /path/to/htpasswd

http:
  addr: localhost:{{ .port }}
  host: https://{{ .fqdn }}:{{ .port }}
  tls:
    certificate: {{ .serverCert }}
    key: {{ .serverKey }}
    clientcas:
      - {{ .caCert }}
`
