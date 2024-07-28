package main

import (
	"context"
	"errors"
	"github.com/alexandremahdhaoui/tooling/pkg/eventualconfig"
	"github.com/alexandremahdhaoui/tooling/pkg/flaterrors"
	certmanagermetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	"os/exec"
	"strings"

	"github.com/alexandremahdhaoui/tooling/internal/util"
	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	certManagerManifests = `https://github.com/cert-manager/cert-manager/releases/download/v1.15.1/cert-manager.yaml`

	tlsResourceName = Name + "-tls"
	tlsMountDir     = "/etc/tls"
)

type TLS struct {
	client client.Client

	caCrtPath           string
	namespace           string
	registryServiceFQDN string

	ec eventualconfig.EventualConfig
}

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

func (t *TLS) Setup(ctx context.Context) error {
	// 1. Install cert-manager.
	helmRepoAdd := exec.Command("helm", strings.Split(
		"repo add jetstack https://charts.jetstack.io --force-update", " ")...)
	if err := util.RunCmdWithStdPipes(helmRepoAdd); err != nil {
		return flaterrors.Join(err, errSettingUpTLS)
	}

	helmInstall := exec.Command("helm", strings.Split(
		"install cert-manager jetstack/cert-manager "+
			"--namespace cert-manager "+
			"--create-namespace "+
			"--version v1.15.1 "+
			"--set crds.enabled=true", " ")...)
	if err := util.RunCmdWithStdPipes(helmInstall); err != nil {
		return flaterrors.Join(err, errSettingUpTLS)
	}

	// TODO: await cert-manager pods are running

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

	// 4. Pass the tls secret name to EventualConfig.
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

func (t *TLS) ResourceName() string {
	return tlsResourceName
}

var errTearingDownTLS = errors.New("tearing down TLS")

func (t *TLS) Teardown() error {
	cmd := exec.Command("kubectl", "delete", "-f", certManagerManifests)

	if err := util.RunCmdWithStdPipes(cmd); err != nil {
		return flaterrors.Join(err, errTearingDownTLS)
	}

	return nil
}
