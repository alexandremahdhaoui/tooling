package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/alexandremahdhaoui/forge/pkg/flaterrors"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	defaultImagePullSecretName = "local-container-registry-credentials"
	imagePullSecretLabel       = "app.kubernetes.io/managed-by"
	imagePullSecretLabelValue  = "testenv-lcr"
)

// ImagePullSecret manages the creation of Kubernetes image pull secrets for the local container registry.
type ImagePullSecret struct {
	client       client.Client
	secretName   string
	registryFQDN string
	username     string
	password     string
	caCert       []byte
}

// NewImagePullSecret creates a new ImagePullSecret struct.
func NewImagePullSecret(
	cl client.Client,
	secretName, registryFQDN, username, password string,
	caCert []byte,
) *ImagePullSecret {
	if secretName == "" {
		secretName = defaultImagePullSecretName
	}

	return &ImagePullSecret{
		client:       cl,
		secretName:   secretName,
		registryFQDN: registryFQDN,
		username:     username,
		password:     password,
		caCert:       caCert,
	}
}

// dockerConfigJSON represents the structure of .dockerconfigjson for image pull secrets.
type dockerConfigJSON struct {
	Auths map[string]dockerConfigEntry `json:"auths"`
}

type dockerConfigEntry struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Auth     string `json:"auth"`
}

var errCreatingImagePullSecret = errors.New("failed to create image pull secret")

// CreateInNamespace creates an image pull secret in the specified namespace.
// It creates the namespace if it doesn't exist and returns the full secret name.
func (ips *ImagePullSecret) CreateInNamespace(ctx context.Context, namespace string) (string, error) {
	// 1. Ensure namespace exists
	if err := ips.ensureNamespace(ctx, namespace); err != nil {
		return "", flaterrors.Join(err, errCreatingImagePullSecret)
	}

	// 2. Generate dockerconfigjson
	dockerConfig, err := ips.generateDockerConfigJSON()
	if err != nil {
		return "", flaterrors.Join(err, errCreatingImagePullSecret)
	}

	// 3. Create secret
	secret := &corev1.Secret{} //nolint:exhaustruct
	secret.Name = ips.secretName
	secret.Namespace = namespace
	secret.Type = corev1.SecretTypeDockerConfigJson
	secret.Data = map[string][]byte{
		corev1.DockerConfigJsonKey: dockerConfig,
	}

	// Add labels for tracking
	secret.Labels = map[string]string{
		imagePullSecretLabel: imagePullSecretLabelValue,
	}

	if err := ips.client.Create(ctx, secret); err != nil {
		return "", flaterrors.Join(err, errCreatingImagePullSecret)
	}

	return fmt.Sprintf("%s/%s", namespace, ips.secretName), nil
}

// CreateInNamespaces creates image pull secrets in multiple namespaces.
// Returns a map of namespace to secret name and any errors encountered.
func (ips *ImagePullSecret) CreateInNamespaces(ctx context.Context, namespaces []string) (map[string]string, error) {
	created := make(map[string]string)
	var errs []error

	for _, namespace := range namespaces {
		secretFullName, err := ips.CreateInNamespace(ctx, namespace)
		if err != nil {
			errs = append(errs, fmt.Errorf("namespace %s: %w", namespace, err))
			continue
		}
		created[namespace] = secretFullName
	}

	if len(errs) > 0 {
		return created, errors.Join(errs...)
	}

	return created, nil
}

var errGeneratingDockerConfig = errors.New("failed to generate docker config")

func (ips *ImagePullSecret) generateDockerConfigJSON() ([]byte, error) {
	// Create auth string (base64 of username:password)
	auth := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", ips.username, ips.password)))

	config := dockerConfigJSON{
		Auths: map[string]dockerConfigEntry{
			ips.registryFQDN: {
				Username: ips.username,
				Password: ips.password,
				Auth:     auth,
			},
		},
	}

	data, err := json.Marshal(config)
	if err != nil {
		return nil, flaterrors.Join(err, errGeneratingDockerConfig)
	}

	return data, nil
}

var errEnsuringNamespace = errors.New("failed to ensure namespace exists")

func (ips *ImagePullSecret) ensureNamespace(ctx context.Context, namespace string) error {
	ns := &corev1.Namespace{} //nolint:exhaustruct
	ns.Name = namespace

	if err := ips.client.Get(ctx, client.ObjectKey{Name: namespace}, ns); err != nil {
		if !apierrors.IsNotFound(err) {
			return flaterrors.Join(err, errEnsuringNamespace)
		}

		// Namespace doesn't exist, create it
		ns.Labels = map[string]string{
			imagePullSecretLabel: imagePullSecretLabelValue,
		}

		if err := ips.client.Create(ctx, ns); err != nil {
			return flaterrors.Join(err, errEnsuringNamespace)
		}
	}

	return nil
}

// ListImagePullSecrets lists all image pull secrets created by testenv-lcr across all namespaces.
// If namespace is provided, it filters to that namespace only.
func ListImagePullSecrets(ctx context.Context, cl client.Client, namespace string) ([]ImagePullSecretInfo, error) {
	secretList := &corev1.SecretList{}
	listOpts := []client.ListOption{
		client.MatchingLabels{
			imagePullSecretLabel: imagePullSecretLabelValue,
		},
	}

	if namespace != "" {
		listOpts = append(listOpts, client.InNamespace(namespace))
	}

	if err := cl.List(ctx, secretList, listOpts...); err != nil {
		return nil, fmt.Errorf("failed to list image pull secrets: %w", err)
	}

	var result []ImagePullSecretInfo
	for _, secret := range secretList.Items {
		if secret.Type != corev1.SecretTypeDockerConfigJson {
			continue
		}

		result = append(result, ImagePullSecretInfo{
			Namespace:  secret.Namespace,
			SecretName: secret.Name,
			CreatedAt:  secret.CreationTimestamp.Time,
		})
	}

	return result, nil
}

// ImagePullSecretInfo contains information about an image pull secret.
type ImagePullSecretInfo struct {
	Namespace  string `json:"namespace"`
	SecretName string `json:"secretName"`
	CreatedAt  any    `json:"createdAt"` // time.Time but using any for JSON serialization flexibility
}
