package project

type LocalContainerRegistry struct {
	Enabled        bool   `json:"enabled"`
	CredentialPath string `json:"credentialPath"`
	CaCrtPath      string `json:"caCrtPath"`
	Namespace      string `json:"namespace"`
}
