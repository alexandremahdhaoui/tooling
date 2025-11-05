// Package forge provides artifact store management for tracking built artifacts and test environments.
//
// The artifact store automatically prunes old build artifacts to prevent unbounded growth:
//   - Only the 3 most recent artifacts are kept for each unique type:name combination
//   - Pruning occurs automatically on every WriteArtifactStore() call
//   - Test environments are NOT pruned - all test history is retained
//
// Example usage:
//
//	store, _ := forge.ReadOrCreateArtifactStore(".forge/artifacts.yaml")
//	forge.AddOrUpdateArtifact(&store, forge.Artifact{
//	    Name: "my-app",
//	    Type: "binary",
//	    Location: "./build/bin/my-app",
//	    Timestamp: time.Now().Format(time.RFC3339),
//	    Version: "v1.0.0",
//	})
//	forge.WriteArtifactStore(".forge/artifacts.yaml", store) // Automatically prunes old artifacts
package forge

import (
	"errors"
	"os"
	"sort"
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

// TestReport represents a test execution report stored in the artifact store.
type TestReport struct {
	// ID is the unique identifier for this test report (UUID)
	ID string `json:"id"`

	// Stage is the test stage name (e.g., "unit", "integration", "e2e")
	Stage string `json:"stage"`

	// Status is the overall test result ("passed" or "failed")
	Status string `json:"status"`

	// StartTime is when the test run started
	StartTime time.Time `json:"startTime"`

	// Duration is the total test duration in seconds
	Duration float64 `json:"duration"`

	// TestStats contains test execution statistics
	TestStats TestStats `json:"testStats"`

	// Coverage contains code coverage information
	Coverage Coverage `json:"coverage"`

	// ArtifactFiles lists all artifact files created by this test run (e.g., XML reports, coverage files)
	ArtifactFiles []string `json:"artifactFiles,omitempty"`

	// OutputPath is the path to detailed test output files
	OutputPath string `json:"outputPath,omitempty"`

	// ErrorMessage contains error details if the test run failed
	ErrorMessage string `json:"errorMessage,omitempty"`

	// CreatedAt is when this report was stored
	CreatedAt time.Time `json:"createdAt"`

	// UpdatedAt is when this report was last updated
	UpdatedAt time.Time `json:"updatedAt"`
}

// TestStats contains statistics about test execution.
type TestStats struct {
	// Total is the total number of tests
	Total int `json:"total"`

	// Passed is the number of tests that passed
	Passed int `json:"passed"`

	// Failed is the number of tests that failed
	Failed int `json:"failed"`

	// Skipped is the number of tests that were skipped
	Skipped int `json:"skipped"`
}

// Coverage contains code coverage information.
type Coverage struct {
	// Percentage is the code coverage percentage (0-100)
	Percentage float64 `json:"percentage"`

	// FilePath is the path to the coverage file
	FilePath string `json:"filePath,omitempty"`
}

type ArtifactStore struct {
	Version          string                      `json:"version"`
	LastUpdated      time.Time                   `json:"lastUpdated"`
	Artifacts        []Artifact                  `json:"artifacts"`
	TestEnvironments map[string]*TestEnvironment `json:"testEnvironments,omitempty"`
	TestReports      map[string]*TestReport      `json:"testReports,omitempty"`
}

var (
	errReadingArtifactStore    = errors.New("reading artifact store")
	errWritingArtifactStore    = errors.New("writing artifact store")
	errArtifactNotFound        = errors.New("artifact not found")
	errTestEnvironmentNotFound = errors.New("test environment not found")
	errTestReportNotFound      = errors.New("test report not found")
	errInvalidArtifactStore    = errors.New("invalid artifact store")
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
	if out.TestReports == nil {
		out.TestReports = make(map[string]*TestReport)
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
				TestReports:      make(map[string]*TestReport),
			}, nil
		}
		return ArtifactStore{}, err
	}
	return store, nil
}

// PruneBuildArtifacts keeps only the N most recent artifacts for each type+name combination.
// Test environments are NOT pruned - only build artifacts are affected.
func PruneBuildArtifacts(store *ArtifactStore, keepCount int) {
	if store == nil || len(store.Artifacts) == 0 {
		return
	}

	// Group artifacts by type+name
	groups := make(map[string][]Artifact)
	for _, artifact := range store.Artifacts {
		key := artifact.Type + ":" + artifact.Name
		groups[key] = append(groups[key], artifact)
	}

	// For each group, keep only the N most recent
	var prunedArtifacts []Artifact
	for _, artifacts := range groups {
		// Sort by timestamp (newest first)
		sort.Slice(artifacts, func(i, j int) bool {
			ti, errI := time.Parse(time.RFC3339, artifacts[i].Timestamp)
			tj, errJ := time.Parse(time.RFC3339, artifacts[j].Timestamp)
			// If parsing fails, keep the artifact at the end
			if errI != nil {
				return false
			}
			if errJ != nil {
				return true
			}
			return ti.After(tj)
		})

		// Keep only N most recent
		if len(artifacts) > keepCount {
			artifacts = artifacts[:keepCount]
		}
		prunedArtifacts = append(prunedArtifacts, artifacts...)
	}

	store.Artifacts = prunedArtifacts
}

// WriteArtifactStore writes the artifact store to the specified path.
// Before writing, it prunes old build artifacts to keep only the 3 most recent per type+name.
func WriteArtifactStore(path string, store ArtifactStore) error {
	// Prune old build artifacts (keep only 3 most recent per type+name)
	PruneBuildArtifacts(&store, 3)

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

// AddOrUpdateTestReport adds or updates a test report in the store.
func AddOrUpdateTestReport(store *ArtifactStore, report *TestReport) {
	if store == nil || report == nil {
		return
	}

	// Initialize map if nil
	if store.TestReports == nil {
		store.TestReports = make(map[string]*TestReport)
	}

	// Update timestamps
	now := time.Now().UTC()
	if report.CreatedAt.IsZero() {
		report.CreatedAt = now
	}
	report.UpdatedAt = now

	store.TestReports[report.ID] = report
	store.LastUpdated = now
}

// GetTestReport retrieves a test report by ID.
func GetTestReport(store *ArtifactStore, id string) (*TestReport, error) {
	if store == nil || store.TestReports == nil {
		return nil, flaterrors.Join(
			errors.New("test report not found: "+id),
			errTestReportNotFound,
		)
	}

	report, exists := store.TestReports[id]
	if !exists {
		return nil, flaterrors.Join(
			errors.New("test report not found: "+id),
			errTestReportNotFound,
		)
	}

	return report, nil
}

// ListTestReports returns all test reports, optionally filtered by stage name.
// If stageName is empty, returns all test reports.
func ListTestReports(store *ArtifactStore, stageName string) []*TestReport {
	if store == nil || store.TestReports == nil {
		return []*TestReport{}
	}

	var results []*TestReport
	for _, report := range store.TestReports {
		if stageName == "" || report.Stage == stageName {
			results = append(results, report)
		}
	}

	return results
}

// DeleteTestReport removes a test report from the store.
// Note: This does not delete the actual artifact files. Callers should handle
// file cleanup separately using the report.ArtifactFiles list.
func DeleteTestReport(store *ArtifactStore, id string) error {
	if store == nil || store.TestReports == nil {
		return flaterrors.Join(
			errors.New("test report not found: "+id),
			errTestReportNotFound,
		)
	}

	if _, exists := store.TestReports[id]; !exists {
		return flaterrors.Join(
			errors.New("test report not found: "+id),
			errTestReportNotFound,
		)
	}

	delete(store.TestReports, id)
	store.LastUpdated = time.Now().UTC()
	return nil
}

// GetArtifactStorePath returns the configured artifact store path from forge.yaml,
// or the provided default path if not configured.
//
// This is a convenience function that encapsulates the common pattern of:
//  1. Reading forge.yaml
//  2. Getting the ArtifactStorePath from config
//  3. Using a default if not set
//
// Example usage:
//
//	path, err := forge.GetArtifactStorePath(".forge/artifacts.yaml")
//	if err != nil {
//	    return fmt.Errorf("failed to get artifact store path: %w", err)
//	}
//	store, err := forge.ReadOrCreateArtifactStore(path)
func GetArtifactStorePath(defaultPath string) (string, error) {
	config, err := ReadSpec()
	if err != nil {
		return "", err
	}

	if config.ArtifactStorePath != "" {
		return config.ArtifactStorePath, nil
	}

	return defaultPath, nil
}
