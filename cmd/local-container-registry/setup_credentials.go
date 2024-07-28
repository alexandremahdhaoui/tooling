package main

import (
	"context"
	"errors"
	"github.com/alexandremahdhaoui/tooling/pkg/eventualconfig"
	"github.com/alexandremahdhaoui/tooling/pkg/flaterrors"
	"math/rand"
	"os"
	"os/exec"
	"sigs.k8s.io/yaml"
	"time"

	corev1 "k8s.io/api/core/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	htpasswdContainerImage = "docker.io/httpd:2"

	credSecName = Name + "-credentials"

	credMountFile = "credential.htpasswd"
	credMountDir  = "/etc/credentials"
)

type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type Credential struct {
	client                    client.Client
	containerEngineExecutable string
	credentials               Credentials
	credentialsPath           string
	namespace                 string

	ec eventualconfig.EventualConfig
}

func NewCredential(
	cl client.Client,
	containerEngineExecutable, credentialsPath, namespace string,
	ec eventualconfig.EventualConfig,
) *Credential {
	return &Credential{
		client:                    cl,
		containerEngineExecutable: containerEngineExecutable,
		credentials: Credentials{
			Username: generateRandomString(32),
			Password: generateRandomString(32),
		},
		credentialsPath: credentialsPath,
		namespace:       namespace,

		ec: ec,
	}
}

var (
	errSettingUpCredentials = errors.New("failed to set up credentials")
)

func (c *Credential) Setup(ctx context.Context) error {
	// 1. write credentials.
	if err := c.writeCredentials(); err != nil {
		return flaterrors.Join(err, errSettingUpCredentials)
	}

	// 2. create htpasswd hash.
	h, err := c.hashCredentials()
	if err != nil {
		return flaterrors.Join(err, errSettingUpCredentials)
	}

	dirFile := Mount{
		Dir:      credMountDir,
		Filename: credMountFile,
	}

	// 3. create credential secret.
	credentialsSecret := &corev1.Secret{} //nolint:exhaustruct

	credentialsSecret.Name = credSecName
	credentialsSecret.Namespace = c.namespace
	credentialsSecret.Type = corev1.SecretTypeOpaque

	credentialsSecret.Data = map[string][]byte{
		dirFile.Filename: h,
	}

	if err := c.client.Create(ctx, credentialsSecret); err != nil {
		return flaterrors.Join(err, errSettingUpCredentials)
	}

	// 4. declare shared values
	if err := flaterrors.Join(
		c.ec.SetValue(CredentialSecretName, credSecName),
		c.ec.SetValue(CredentialMount, dirFile),
	); err != nil {
		return flaterrors.Join(err, errSettingUpCredentials)
	}

	return nil
}

var errWritingCredentialsToFile = errors.New("failed to write credentials to file")

func (c *Credential) writeCredentials() error {
	b, err := yaml.Marshal(c.credentials)
	if err != nil {
		return flaterrors.Join(err, errWritingCredentialsToFile)
	}

	if err := os.WriteFile(c.credentialsPath, b, 0o600); err != nil {
		return flaterrors.Join(err, errWritingCredentialsToFile)
	}

	return nil
}

var errHashingCredentials = errors.New("failed to hash credentials")

func (c *Credential) hashCredentials() ([]byte, error) {
	cmd := exec.Command(
		c.containerEngineExecutable,
		"run", "--rm", "-i", "-t",
		"--entrypoint", "htpasswd",
		htpasswdContainerImage,
		"-Bbn",
		c.credentials.Username,
		c.credentials.Password,
	)

	b, err := cmd.Output()
	if err != nil {
		return nil, flaterrors.Join(err, errHashingCredentials)
	}

	return b, nil
}

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

var seededRand = rand.New(rand.NewSource(time.Now().UnixNano()))

func generateRandomString(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}

	return string(b)
}
