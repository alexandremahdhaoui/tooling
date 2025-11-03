package forge

import (
	"errors"
	"fmt"
	"os"

	"sigs.k8s.io/yaml"
)

var (
	errReadingIntegrationEnvStore  = errors.New("reading integration environment store")
	errWritingIntegrationEnvStore  = errors.New("writing integration environment store")
	errIntegrationEnvNotFound      = errors.New("integration environment not found")
)

// IntegrationEnvironment represents a managed integration environment.
type IntegrationEnvironment struct {
	// ID is the unique identifier for this environment
	ID string `json:"id"`

	// Name is the human-friendly name for this environment
	Name string `json:"name"`

	// Created is the RFC3339 timestamp when this environment was created
	Created string `json:"created"`

	// Components maps component names to their configuration
	Components map[string]Component `json:"components"`
}

// Component represents a component within an integration environment.
type Component struct {
	// Enabled indicates if this component is active
	Enabled bool `json:"enabled"`

	// Ready indicates if the component is fully set up
	Ready bool `json:"ready"`

	// ConnectionInfo contains connection details (kubeconfig path, credentials, etc.)
	ConnectionInfo map[string]string `json:"connectionInfo"`
}

// IntegrationEnvironmentStore stores all integration environments.
type IntegrationEnvironmentStore struct {
	Environments []IntegrationEnvironment `json:"environments"`
}

// ReadIntegrationEnvStore reads the integration environment store from the specified path.
func ReadIntegrationEnvStore(path string) (IntegrationEnvironmentStore, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return IntegrationEnvironmentStore{Environments: []IntegrationEnvironment{}}, nil
		}
		return IntegrationEnvironmentStore{}, fmt.Errorf("%w: %v", errReadingIntegrationEnvStore, err)
	}

	var store IntegrationEnvironmentStore
	if err := yaml.Unmarshal(b, &store); err != nil {
		return IntegrationEnvironmentStore{}, fmt.Errorf("%w: %v", errReadingIntegrationEnvStore, err)
	}

	if store.Environments == nil {
		store.Environments = []IntegrationEnvironment{}
	}

	return store, nil
}

// WriteIntegrationEnvStore writes the integration environment store to the specified path.
func WriteIntegrationEnvStore(path string, store IntegrationEnvironmentStore) error {
	b, err := yaml.Marshal(store)
	if err != nil {
		return fmt.Errorf("%w: %v", errWritingIntegrationEnvStore, err)
	}

	if err := os.WriteFile(path, b, 0o600); err != nil {
		return fmt.Errorf("%w: %v", errWritingIntegrationEnvStore, err)
	}

	return nil
}

// AddEnvironment adds a new environment to the store.
func AddEnvironment(store *IntegrationEnvironmentStore, env IntegrationEnvironment) {
	if store.Environments == nil {
		store.Environments = []IntegrationEnvironment{}
	}
	store.Environments = append(store.Environments, env)
}

// GetEnvironment retrieves an environment by ID.
func GetEnvironment(store IntegrationEnvironmentStore, id string) (IntegrationEnvironment, error) {
	for _, env := range store.Environments {
		if env.ID == id {
			return env, nil
		}
	}
	return IntegrationEnvironment{}, fmt.Errorf("%w: %s", errIntegrationEnvNotFound, id)
}

// DeleteEnvironment removes an environment from the store by ID.
func DeleteEnvironment(store *IntegrationEnvironmentStore, id string) error {
	for i, env := range store.Environments {
		if env.ID == id {
			store.Environments = append(store.Environments[:i], store.Environments[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("%w: %s", errIntegrationEnvNotFound, id)
}
