package main

import (
	"path/filepath"

	"github.com/alexandremahdhaoui/tooling/pkg/eventualconfig"
)

const (
	// TLS

	TLSCACert     = "tls-ca-cert"
	TLSCert       = "tls-cert"
	TLSKey        = "tls-key"
	TLSSecretName = "tls-secret-name"

	// Credential

	CredentialMount      = "credential-mount"
	CredentialSecretName = "credential-secret-name"
)

// Mount represents a file mount with a directory and filename.
type Mount struct {
	// Dir is the directory where the file is mounted.
	Dir string
	// Filename is the name of the mounted file.
	Filename string
}

// Path returns the full path of the mounted file.
func (m Mount) Path() string {
	return filepath.Join(m.Dir, m.Filename)
}

// NewEventualConfig creates a new EventualConfig for the local-container-registry tool.
func NewEventualConfig() eventualconfig.EventualConfig { //nolint:ireturn
	return eventualconfig.NewEventualConfig(
		// TLS
		TLSCACert,
		TLSCert,
		TLSKey,
		TLSSecretName,

		// Credential
		CredentialMount,
		CredentialSecretName,
	)
}
