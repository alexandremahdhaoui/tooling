package forge

import (
	"errors"
	"os"
	"time"

	"github.com/alexandremahdhaoui/forge/pkg/flaterrors"
	"sigs.k8s.io/yaml"
)

var (
	errReadingArtifactStore = errors.New("reading artifact store")
	errWritingArtifactStore = errors.New("writing artifact store")
	errArtifactNotFound     = errors.New("artifact not found")
	errInvalidArtifactStore = errors.New("invalid artifact store")
)

// ReadArtifactStore reads the artifact store from the specified path.
// If the file doesn't exist, it returns an empty ArtifactStore without error.
func ReadArtifactStore(path string) (ArtifactStore, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		// If file doesn't exist, return empty store
		if os.IsNotExist(err) {
			return ArtifactStore{Artifacts: []Artifact{}}, nil
		}
		return ArtifactStore{}, flaterrors.Join(err, errReadingArtifactStore)
	}

	out := ArtifactStore{} //nolint:exhaustruct // unmarshal

	if err := yaml.Unmarshal(b, &out); err != nil {
		return ArtifactStore{}, flaterrors.Join(err, errReadingArtifactStore)
	}

	// Initialize empty slice if nil
	if out.Artifacts == nil {
		out.Artifacts = []Artifact{}
	}

	return out, nil
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
