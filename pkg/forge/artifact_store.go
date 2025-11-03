package forge

import (
	"errors"
	"os"
	"time"

	"github.com/alexandremahdhaoui/forge/pkg/flaterrors"
	"sigs.k8s.io/yaml"
)

type Artifact struct {
	// The name of the artifact
	Name string `json:"name"`
	// Type of artifact
	Type string `json:"type"` // e.g.: "container" or "binary"
	// Location of the artifact (can be a url or the path to a file, which must start as a url like file://)
	Location string `json:"location"`
	// Timestamp when the artifact was built
	Timestamp string `json:"timestamp"`
	// Version is the hash/commit
	Version string `json:"version"`
}

type ArtifactStore struct {
	Version          string                      `json:"version"`
	LastUpdated      time.Time                   `json:"lastUpdated"`
	Artifacts        []Artifact                  `json:"artifacts"`
	TestEnvironments map[string]*TestEnvironment `json:"testEnvironments,omitempty"`
}

var (
	errReadingArtifactStore     = errors.New("reading artifact store")
	errWritingArtifactStore     = errors.New("writing artifact store")
	errArtifactNotFound         = errors.New("artifact not found")
	errTestEnvironmentNotFound  = errors.New("test environment not found")
	errInvalidArtifactStore     = errors.New("invalid artifact store")
)

const artifactStoreVersion = "1.0"

// ReadArtifactStore reads the artifact store from the specified path.
// Returns an error if the file doesn't exist.
func ReadArtifactStore(path string) (ArtifactStore, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return ArtifactStore{}, flaterrors.Join(err, errReadingArtifactStore)
	}

	out := ArtifactStore{} //nolint:exhaustruct // unmarshal

	if err := yaml.Unmarshal(b, &out); err != nil {
		return ArtifactStore{}, flaterrors.Join(err, errReadingArtifactStore)
	}

	// Initialize empty slice/map if nil
	if out.Artifacts == nil {
		out.Artifacts = []Artifact{}
	}
	if out.TestEnvironments == nil {
		out.TestEnvironments = make(map[string]*TestEnvironment)
	}
	if out.Version == "" {
		out.Version = artifactStoreVersion
	}

	return out, nil
}

// ReadOrCreateArtifactStore reads the artifact store from the specified path.
// If the file doesn't exist, it returns an initialized empty store.
func ReadOrCreateArtifactStore(path string) (ArtifactStore, error) {
	store, err := ReadArtifactStore(path)
	if err != nil {
		// If file doesn't exist, return empty initialized store
		if errors.Is(err, os.ErrNotExist) {
			return ArtifactStore{
				Version:          artifactStoreVersion,
				LastUpdated:      time.Now().UTC(),
				Artifacts:        []Artifact{},
				TestEnvironments: make(map[string]*TestEnvironment),
			}, nil
		}
		return ArtifactStore{}, err
	}
	return store, nil
}

// WriteArtifactStore writes the artifact store to the specified path.
func WriteArtifactStore(path string, store ArtifactStore) error {
	b, err := yaml.Marshal(store)
	if err != nil {
		return flaterrors.Join(err, errWritingArtifactStore)
	}

	if err := os.WriteFile(path, b, 0o600); err != nil {
		return flaterrors.Join(err, errWritingArtifactStore)
	}

	return nil
}

// AddOrUpdateArtifact adds a new artifact to the store or updates an existing one.
// If an artifact with the same name, type, and version exists, it updates it.
// Otherwise, it appends a new artifact.
func AddOrUpdateArtifact(store *ArtifactStore, artifact Artifact) {
	if store == nil {
		return
	}

	// Initialize slice if nil
	if store.Artifacts == nil {
		store.Artifacts = []Artifact{}
	}

	// Check if artifact with same name, type, and version exists
	for i, existing := range store.Artifacts {
		if existing.Name == artifact.Name &&
			existing.Type == artifact.Type &&
			existing.Version == artifact.Version {
			// Update existing artifact
			store.Artifacts[i] = artifact
			return
		}
	}

	// Append new artifact
	store.Artifacts = append(store.Artifacts, artifact)
}

// GetLatestArtifact finds the most recent artifact with the given name.
// It returns the artifact with the latest timestamp.
func GetLatestArtifact(store ArtifactStore, name string) (Artifact, error) {
	var latest Artifact
	var latestTime time.Time
	found := false

	for _, artifact := range store.Artifacts {
		if artifact.Name != name {
			continue
		}

		// Parse timestamp
		t, err := time.Parse(time.RFC3339, artifact.Timestamp)
		if err != nil {
			// Skip artifacts with invalid timestamps
			continue
		}

		if !found || t.After(latestTime) {
			latest = artifact
			latestTime = t
			found = true
		}
	}

	if !found {
		return Artifact{}, flaterrors.Join(
			errors.New("no artifact found with name: "+name),
			errArtifactNotFound,
		)
	}

	return latest, nil
}

// GetArtifactsByType returns all artifacts of a specific type.
func GetArtifactsByType(store ArtifactStore, artifactType string) []Artifact {
	var results []Artifact

	for _, artifact := range store.Artifacts {
		if artifact.Type == artifactType {
			results = append(results, artifact)
		}
	}

	return results
}

// GetArtifactByNameAndVersion finds an artifact with the given name and version.
func GetArtifactByNameAndVersion(store ArtifactStore, name, version string) (Artifact, error) {
	for _, artifact := range store.Artifacts {
		if artifact.Name == name && artifact.Version == version {
			return artifact, nil
		}
	}

	return Artifact{}, flaterrors.Join(
		errors.New("no artifact found with name: "+name+" and version: "+version),
		errArtifactNotFound,
	)
}

// AddOrUpdateTestEnvironment adds or updates a test environment in the store.
func AddOrUpdateTestEnvironment(store *ArtifactStore, env *TestEnvironment) {
	if store == nil || env == nil {
		return
	}

	// Initialize map if nil
	if store.TestEnvironments == nil {
		store.TestEnvironments = make(map[string]*TestEnvironment)
	}

	// Update timestamps
	env.UpdatedAt = time.Now().UTC()
	store.TestEnvironments[env.ID] = env
	store.LastUpdated = time.Now().UTC()
}

// GetTestEnvironment retrieves a test environment by ID.
func GetTestEnvironment(store *ArtifactStore, id string) (*TestEnvironment, error) {
	if store == nil || store.TestEnvironments == nil {
		return nil, flaterrors.Join(
			errors.New("test environment not found: "+id),
			errTestEnvironmentNotFound,
		)
	}

	env, exists := store.TestEnvironments[id]
	if !exists {
		return nil, flaterrors.Join(
			errors.New("test environment not found: "+id),
			errTestEnvironmentNotFound,
		)
	}

	return env, nil
}

// ListTestEnvironments returns all test environments, optionally filtered by stage name.
// If stageName is empty, returns all test environments.
func ListTestEnvironments(store *ArtifactStore, stageName string) []*TestEnvironment {
	if store == nil || store.TestEnvironments == nil {
		return []*TestEnvironment{}
	}

	var results []*TestEnvironment
	for _, env := range store.TestEnvironments {
		if stageName == "" || env.Name == stageName {
			results = append(results, env)
		}
	}

	return results
}

// DeleteTestEnvironment removes a test environment from the store.
func DeleteTestEnvironment(store *ArtifactStore, id string) error {
	if store == nil || store.TestEnvironments == nil {
		return flaterrors.Join(
			errors.New("test environment not found: "+id),
			errTestEnvironmentNotFound,
		)
	}

	if _, exists := store.TestEnvironments[id]; !exists {
		return flaterrors.Join(
			errors.New("test environment not found: "+id),
			errTestEnvironmentNotFound,
		)
	}

	delete(store.TestEnvironments, id)
	store.LastUpdated = time.Now().UTC()
	return nil
}
