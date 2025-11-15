package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/alexandremahdhaoui/forge/pkg/eventualconfig"
	"github.com/alexandremahdhaoui/forge/pkg/flaterrors"
	certmanagermetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"

	"github.com/alexandremahdhaoui/forge/internal/util"
	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	certManagerManifests = `https://github.com/cert-manager/cert-manager/releases/download/v1.15.1/cert-manager.yaml`

	tlsResourceName = Name + "-tls"
	tlsMountDir     = "/etc/tls"
)

// TLS is a struct that manages the setup of TLS for the local container registry.
type TLS struct {
	client client.Client

	caCrtPath           string
	namespace           string
	registryServiceFQDN string

	ec eventualconfig.EventualConfig
}

// NewTLS creates a new TLS struct.
func NewTLS(
	cl client.Client,
	caCrtPath, registryNamespace, registryServiceFQDN string,
	ec eventualconfig.EventualConfig,
) *TLS {
	return &TLS{
		client: cl,

		caCrtPath:           caCrtPath,
		namespace:           registryNamespace,
		registryServiceFQDN: registryServiceFQDN,

		ec: ec,
	}
}

var errSettingUpTLS = errors.New("error setting up TLS")

// Setup sets up TLS for the local container registry.
// It installs cert-manager, creates a self-signed issuer, creates a certificate, and passes the TLS secret name to EventualConfig.
func (t *TLS) Setup(ctx context.Context) error {
	// 1. Install cert-manager.
	helmRepoAdd := exec.Command("helm", strings.Split(
		"repo add jetstack https://charts.jetstack.io --force-update", " ")...)
	if err := util.RunCmdWithStdPipes(helmRepoAdd); err != nil {
		return flaterrors.Join(err, errSettingUpTLS)
	}

	helmInstall := exec.Command("helm", strings.Split(
		"upgrade --install cert-manager jetstack/cert-manager "+
			"--namespace cert-manager "+
			"--create-namespace "+
			"--version v1.15.1 "+
			"--set crds.enabled=true "+
			"--wait "+
			"--timeout 5m", " ")...)
	if err := util.RunCmdWithStdPipes(helmInstall); err != nil {
		return flaterrors.Join(err, errSettingUpTLS)
	}

	// 2. Create self signed issuer.
	issuer := &certmanagerv1.Issuer{}

	issuer.Name = t.ResourceName()
	issuer.Namespace = t.namespace
	issuer.Spec.SelfSigned = &certmanagerv1.SelfSignedIssuer{}

	if err := t.client.Create(ctx, issuer); err != nil {
		return flaterrors.Join(err, errSettingUpTLS)
	}

	// 3. Create certificate in registry namespace.
	cert := &certmanagerv1.Certificate{}

	cert.Name = t.ResourceName()
	cert.Namespace = t.namespace

	cert.Spec.DNSNames = []string{t.registryServiceFQDN}
	cert.Spec.SecretName = t.ResourceName()
	cert.Spec.IssuerRef.Name = t.ResourceName()

	if err := t.client.Create(ctx, cert); err != nil {
		return flaterrors.Join(err, errSettingUpTLS)
	}

	// 4. Export CA certificate to file
	if err := t.exportCACert(ctx); err != nil {
		return flaterrors.Join(err, errSettingUpTLS)
	}

	// 5. Pass the tls secret name to EventualConfig.
	if err := errors.Join(
		t.ec.SetValue(TLSSecretName, cert.Name),
		t.ec.SetValue(TLSCACert, Mount{Dir: tlsMountDir, Filename: certmanagermetav1.TLSCAKey}),
		t.ec.SetValue(TLSCert, Mount{Dir: tlsMountDir, Filename: "tls.crt"}),
		t.ec.SetValue(TLSKey, Mount{Dir: tlsMountDir, Filename: "tls.key"}),
	); err != nil {
		return flaterrors.Join(err, errSettingUpTLS)
	}

	return nil
}

// ResourceName returns the name of the TLS resources.
func (t *TLS) ResourceName() string {
	return tlsResourceName
}

var errTearingDownTLS = errors.New("tearing down TLS")

var errExportingCACert = errors.New("failed to export CA certificate")

// exportCACert waits for the certificate to be ready and exports the CA cert to a file.
func (t *TLS) exportCACert(ctx context.Context) error {
	// Wait for the secret to be created and contain the CA cert
	secret := &corev1.Secret{}
	secretKey := client.ObjectKey{
		Namespace: t.namespace,
		Name:      t.ResourceName(),
	}

	// Retry for up to 60 seconds
	var lastErr error
	for i := 0; i < 60; i++ {
		if err := t.client.Get(ctx, secretKey, secret); err != nil {
			lastErr = err
			time.Sleep(1 * time.Second)
			continue
		}

		// Check if CA cert exists in the secret
		caCert, ok := secret.Data[certmanagermetav1.TLSCAKey]
		if !ok || len(caCert) == 0 {
			lastErr = errors.New("CA certificate not found in secret")
			time.Sleep(1 * time.Second)
			continue
		}

		// Write CA cert to file
		if err := os.WriteFile(t.caCrtPath, caCert, 0o600); err != nil {
			return flaterrors.Join(err, errExportingCACert)
		}

		return nil
	}

	return flaterrors.Join(lastErr, errExportingCACert)
}

// Teardown tears down TLS for the local container registry.
// It deletes the cert-manager manifests.
func (t *TLS) Teardown() error {
	cmd := exec.Command("kubectl", "delete", "-f", certManagerManifests)

	if err := util.RunCmdWithStdPipes(cmd); err != nil {
		return flaterrors.Join(err, errTearingDownTLS)
	}

	return nil
}
