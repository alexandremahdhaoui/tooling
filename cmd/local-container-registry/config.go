package main

import (
	"github.com/alexandremahdhaoui/tooling/pkg/eventualconfig"
	"path/filepath"
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

type Mount struct {
	Dir      string
	Filename string
}

func (m Mount) Path() string {
	return filepath.Join(m.Dir, m.Filename)
}

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
