package forge

// LocalContainerRegistry holds the configuration for the local-container-registry tool.
type LocalContainerRegistry struct {
	// Enabled indicates whether the local container registry is enabled.
	Enabled bool `json:"enabled"`
	// CredentialPath is the path to the credentials file for the local container registry.
	CredentialPath string `json:"credentialPath"`
	// CaCrtPath is the path to the CA certificate for the local container registry.
	CaCrtPath string `json:"caCrtPath"`
	// Namespace is the Kubernetes namespace where the local container registry is deployed.
	Namespace string `json:"namespace"`
	// AutoPushImages indicates whether to automatically push images from the artifact store on setup.
	AutoPushImages bool `json:"autoPushImages"`
	// ImagePullSecretNamespaces is a list of namespaces where image pull secrets should be automatically created.
	ImagePullSecretNamespaces []string `json:"imagePullSecretNamespaces,omitempty"`
	// ImagePullSecretName is the name of the image pull secret to create (defaults to "local-container-registry-credentials").
	ImagePullSecretName string `json:"imagePullSecretName,omitempty"`
}
