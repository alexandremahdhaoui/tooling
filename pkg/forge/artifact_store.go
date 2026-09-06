// Copyright 2024 Alexandre Mahdhaoui
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"time"

	"github.com/alexandremahdhaoui/forge/pkg/flaterrors"
	"sigs.k8s.io/yaml"
)

// Dependency type constants
const (
	DependencyTypeFile            = "file"
	DependencyTypeExternalPackage = "externalPackage"
)

// ArtifactDependency represents a dependency tracked for an artifact
type ArtifactDependency struct {
	// Type is either "file" or "externalPackage"
	Type string `json:"type" yaml:"type"`
	// FilePath is the absolute path to file dependency (if Type=file)
	FilePath string `json:"filePath,omitempty" yaml:"filePath,omitempty"`
	// ExternalPackage is the package identifier (if Type=externalPackage, e.g., "github.com/foo/bar")
	ExternalPackage string `json:"externalPackage,omitempty" yaml:"externalPackage,omitempty"`
	// Timestamp is RFC3339 timestamp in UTC (if Type=file, e.g., "2025-11-23T10:00:00Z")
	Timestamp string `json:"timestamp,omitempty" yaml:"timestamp,omitempty"`
	// Semver is semantic version (if Type=externalPackage, supports pseudo-versions like "v0.0.0-20231010123456-abcdef123456")
	Semver string `json:"semver,omitempty" yaml:"semver,omitempty"`
}

type Artifact struct {
	// The name of the artifact, as the build entry names it. A cross-built
	// binary keeps the entry's name; the platform is the two fields below,
	// never a suffix somebody has to parse back out of the name.
	Name string `json:"name" yaml:"name"`
	// Type of artifact, one of ArtifactTypes.
	Type ArtifactType `json:"type" yaml:"type"`
	// OS and Arch are the platform this artifact was built for, both set or
	// both empty. Empty means the artifact is not tied to one: generated
	// code, a command's output, a multi-architecture image layout.
	OS   string `json:"os,omitempty" yaml:"os,omitempty"`
	Arch string `json:"arch,omitempty" yaml:"arch,omitempty"`
	// Location of the artifact (can be a url or the path to a file, which must start as a url like file://)
	Location string `json:"location" yaml:"location"`
	// Timestamp when the artifact was built
	Timestamp string `json:"timestamp" yaml:"timestamp"`
	// Version is the hash/commit
	Version string `json:"version" yaml:"version"`
	// Dependencies is the list of dependencies tracked for this artifact
	Dependencies []ArtifactDependency `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
	// DependencyDetectorEngine is the URI of the dependency detector used (optional)
	DependencyDetectorEngine string `json:"dependencyDetectorEngine,omitempty" yaml:"dependencyDetectorEngine,omitempty"`
	// DependencyDetectorSpec contains configuration for the dependency detector (optional)
	DependencyDetectorSpec map[string]interface{} `json:"dependencyDetectorSpec,omitempty" yaml:"dependencyDetectorSpec,omitempty"`
}

// ArtifactSummary is a lightweight view of an Artifact without dependencies or version details.
type ArtifactSummary struct {
	Name      string       `json:"name" yaml:"name"`
	Type      ArtifactType `json:"type" yaml:"type"`
	OS        string       `json:"os,omitempty" yaml:"os,omitempty"`
	Arch      string       `json:"arch,omitempty" yaml:"arch,omitempty"`
	Location  string       `json:"location" yaml:"location"`
	Timestamp string       `json:"timestamp" yaml:"timestamp"`
}

// Summary returns a lightweight summary of this Artifact.
func (a Artifact) Summary() ArtifactSummary {
	return ArtifactSummary{
		Name:      a.Name,
		Type:      a.Type,
		OS:        a.OS,
		Arch:      a.Arch,
		Location:  a.Location,
		Timestamp: a.Timestamp,
	}
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

	TestenvID string `json:"testenvID,omitempty"`

	// CreatedAt is when this report was stored
	CreatedAt time.Time `json:"createdAt"`

	// UpdatedAt is when this report was last updated
	UpdatedAt time.Time `json:"updatedAt"`
}

// TestReportSummary is a lightweight view of a TestReport without stats, coverage, or error details.
type TestReportSummary struct {
	ID        string    `json:"id" yaml:"id"`
	Stage     string    `json:"stage" yaml:"stage"`
	Status    string    `json:"status" yaml:"status"`
	StartTime time.Time `json:"startTime" yaml:"startTime"`
}

// Summary returns a lightweight summary of this TestReport.
func (r TestReport) Summary() TestReportSummary {
	return TestReportSummary{
		ID:        r.ID,
		Stage:     r.Stage,
		Status:    r.Status,
		StartTime: r.StartTime,
	}
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
	// Enabled indicates whether coverage was actually calculated.
	// If false, this runner does not produce coverage (e.g., lint tools).
	// If true, Percentage contains the actual coverage value.
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Percentage is the code coverage percentage (0-100)
	Percentage float64 `json:"percentage" yaml:"percentage"`

	// FilePath is the path to the coverage file
	FilePath string `json:"filePath,omitempty" yaml:"filePath,omitempty"`
}

type ArtifactStore struct {
	Version          string                      `json:"version"`
	LastUpdated      time.Time                   `json:"lastUpdated"`
	Artifacts        []Artifact                  `json:"artifacts"`
	TestEnvironments map[string]*TestEnvironment `json:"testEnvironments,omitempty"`
	TestReports      map[string]*TestReport      `json:"testReports,omitempty"`
}

// Validate validates the ArtifactDependency
func (ad *ArtifactDependency) Validate() error {
	errs := NewValidationErrors()

	// Validate Type is either "file" or "externalPackage"
	if ad.Type != DependencyTypeFile && ad.Type != DependencyTypeExternalPackage {
		errs.AddErrorf("ArtifactDependency: type must be %q or %q, got %q", DependencyTypeFile, DependencyTypeExternalPackage, ad.Type)
	}

	// Validate file dependency fields
	if ad.Type == DependencyTypeFile {
		if ad.FilePath == "" {
			errs.AddErrorf("ArtifactDependency: filePath is required when type=%q", DependencyTypeFile)
		}
		if ad.Timestamp == "" {
			errs.AddErrorf("ArtifactDependency: timestamp is required when type=%q", DependencyTypeFile)
		} else {
			// Validate timestamp is RFC3339
			if _, err := time.Parse(time.RFC3339, ad.Timestamp); err != nil {
				errs.AddErrorf("ArtifactDependency: timestamp must be RFC3339 format, got %q: %v", ad.Timestamp, err)
			}
		}
		// Validate no mixed fields
		if ad.ExternalPackage != "" {
			errs.AddErrorf("ArtifactDependency: file dependency cannot have externalPackage field set")
		}
	}

	// Validate external package dependency fields
	if ad.Type == DependencyTypeExternalPackage {
		if ad.ExternalPackage == "" {
			errs.AddErrorf("ArtifactDependency: externalPackage is required when type=%q", DependencyTypeExternalPackage)
		}
		// Validate no mixed fields
		if ad.FilePath != "" {
			errs.AddErrorf("ArtifactDependency: externalPackage dependency cannot have filePath field set")
		}
		if ad.Timestamp != "" {
			errs.AddErrorf("ArtifactDependency: externalPackage dependency cannot have timestamp field set")
		}
	}

	return errs.ErrorOrNil()
}

// Validate validates the Artifact
func (a *Artifact) Validate() error {
	errs := NewValidationErrors()

	// Validate required fields
	if err := ValidateRequired(a.Name, "name", "Artifact"); err != nil {
		errs.Add(err)
	}
	if err := a.Type.Validate(); err != nil {
		errs.Add(err)
	}
	if (a.OS == "") != (a.Arch == "") {
		errs.AddErrorf("%v: os=%q arch=%q", ErrArtifactPlatform, a.OS, a.Arch)
	}
	if err := ValidateRequired(a.Location, "location", "Artifact"); err != nil {
		errs.Add(err)
	}

	// Validate dependencies
	for i, dep := range a.Dependencies {
		if err := dep.Validate(); err != nil {
			errs.AddErrorf("dependencies[%d]: %v", i, err)
		}
	}

	return errs.ErrorOrNil()
}

// Validate validates the ArtifactStore
func (as *ArtifactStore) Validate() error {
	errs := NewValidationErrors()

	// Validate version
	if err := ValidateRequired(as.Version, "version", "ArtifactStore"); err != nil {
		errs.Add(err)
	}

	// Validate all artifacts
	for i, artifact := range as.Artifacts {
		if err := artifact.Validate(); err != nil {
			errs.AddErrorf("artifacts[%d] (%s): %v", i, artifact.Name, err)
		}
	}

	// Note: TestEnvironments and TestReports don't need deep validation here
	// as they are managed internally by forge

	return errs.ErrorOrNil()
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

	// A store is a cache, and a record an older forge wrote under a type
	// the vocabulary no longer carries is a stale cache entry, not a broken
	// store: it is dropped, said so, and the next build writes a fresh one.
	// A record with a half-named platform is the same case.
	kept := out.Artifacts[:0]
	dropped := 0

	for _, artifact := range out.Artifacts {
		if err := artifact.Type.Validate(); err != nil || (artifact.OS == "") != (artifact.Arch == "") {
			dropped++

			continue
		}

		kept = append(kept, artifact)
	}

	if dropped > 0 {
		fmt.Fprintf(os.Stderr, "forge: dropping %d artifact record(s) an older forge wrote to %s; the next build rewrites them\n", dropped, path)
	}

	out.Artifacts = kept

	// Validate the artifact store
	if err := out.Validate(); err != nil {
		return ArtifactStore{}, flaterrors.Join(err, errInvalidArtifactStore, errReadingArtifactStore)
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

	// Group artifacts by identity: type, name and platform. A host binary
	// and its cross-built siblings are three artifacts, pruned apart.
	groups := make(map[string][]Artifact)
	for _, artifact := range store.Artifacts {
		key := artifact.identity()
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

// lockArtifactStore acquires an exclusive file lock for the artifact store.
// The lock is held on a separate .lock file to avoid interfering with reads.
// The caller must call unlockArtifactStore to release the lock.
func lockArtifactStore(path string) (*os.File, error) {
	lockPath := path + ".lock"

	// Ensure the directory exists
	dir := filepath.Dir(lockPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, flaterrors.Join(err, errors.New("failed to create lock directory"))
	}

	// Open or create the lock file
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, flaterrors.Join(err, errors.New("failed to open lock file"))
	}

	// Acquire exclusive lock (LOCK_EX) - blocks until lock is available
	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX); err != nil {
		_ = lockFile.Close()
		return nil, flaterrors.Join(err, errors.New("failed to acquire lock"))
	}

	return lockFile, nil
}

// unlockArtifactStore releases the file lock and closes the lock file.
func unlockArtifactStore(lockFile *os.File) error {
	if lockFile == nil {
		return nil
	}

	// Release the lock
	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN); err != nil {
		_ = lockFile.Close()
		return flaterrors.Join(err, errors.New("failed to release lock"))
	}

	// Close the lock file
	return lockFile.Close()
}

// WriteArtifactStore writes the artifact store to the specified path.
// Before writing, it prunes old build artifacts to keep only the 3 most recent per type+name.
// This function uses file locking to prevent concurrent write conflicts.
//
// IMPORTANT: This function performs an atomic read-merge-write to prevent race conditions.
// After acquiring the lock, it re-reads the current store from disk and merges TestEnvironments
// and TestReports to preserve entries that may have been written by concurrent processes.
func WriteArtifactStore(path string, store ArtifactStore) error {
	// Acquire exclusive lock
	lockFile, err := lockArtifactStore(path)
	if err != nil {
		return flaterrors.Join(err, errWritingArtifactStore)
	}
	defer func() { _ = unlockArtifactStore(lockFile) }()

	// Re-read current store from disk while holding the lock
	// This prevents race conditions where concurrent processes lose each other's writes
	currentStore, err := ReadArtifactStore(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return flaterrors.Join(err, errWritingArtifactStore)
	}

	// Merge TestEnvironments: preserve entries from disk that aren't in the incoming store
	// The incoming store's entries take precedence (they're newer)
	if currentStore.TestEnvironments != nil {
		if store.TestEnvironments == nil {
			store.TestEnvironments = make(map[string]*TestEnvironment)
		}
		for id, env := range currentStore.TestEnvironments {
			if _, exists := store.TestEnvironments[id]; !exists {
				store.TestEnvironments[id] = env
			}
		}
	}

	// Merge TestReports: preserve entries from disk that aren't in the incoming store
	// The incoming store's entries take precedence (they're newer)
	if currentStore.TestReports != nil {
		if store.TestReports == nil {
			store.TestReports = make(map[string]*TestReport)
		}
		for id, report := range currentStore.TestReports {
			if _, exists := store.TestReports[id]; !exists {
				store.TestReports[id] = report
			}
		}
	}

	// Prune old build artifacts (keep only 3 most recent per type+name)
	PruneBuildArtifacts(&store, 3)

	b, err := yaml.Marshal(store)
	if err != nil {
		return flaterrors.Join(err, errWritingArtifactStore)
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return flaterrors.Join(err, errWritingArtifactStore)
	}

	if err := os.WriteFile(path, b, 0o600); err != nil {
		return flaterrors.Join(err, errWritingArtifactStore)
	}

	return nil
}

// identity is what tells two artifacts apart in the store: type, name and
// platform. Two builds of one identity are versions of the same thing.
func (a Artifact) identity() string {
	return string(a.Type) + ":" + a.Name + ":" + a.Platform()
}

// AddOrUpdateArtifact adds a new artifact to the store or updates an existing one.
// If an artifact with the same identity (type, name, platform) and version
// exists, it updates it. Otherwise, it appends a new artifact.
func AddOrUpdateArtifact(store *ArtifactStore, artifact Artifact) {
	if store == nil {
		return
	}

	// Initialize slice if nil
	if store.Artifacts == nil {
		store.Artifacts = []Artifact{}
	}

	for i, existing := range store.Artifacts {
		if existing.identity() == artifact.identity() &&
			existing.Version == artifact.Version {
			// Update existing artifact
			store.Artifacts[i] = artifact
			return
		}
	}

	// Append new artifact
	store.Artifacts = append(store.Artifacts, artifact)
}

// GetLatestArtifact finds the most recent artifact with the given name and
// platform. The platform is "<os>/<arch>", or empty for an artifact not tied
// to one. It returns the artifact with the latest timestamp.
func GetLatestArtifact(store ArtifactStore, name, platform string) (Artifact, error) {
	var latest Artifact
	var latestTime time.Time
	found := false

	for _, artifact := range store.Artifacts {
		if artifact.Name != name || artifact.Platform() != platform {
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
			errors.New("no artifact found with name: "+name+" for platform: "+platformOrHost(platform)),
			errArtifactNotFound,
		)
	}

	return latest, nil
}

// GetArtifactsByType returns all artifacts of a specific type.
func GetArtifactsByType(store ArtifactStore, artifactType ArtifactType) []Artifact {
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

	// Backward compatibility: initialize Env map if nil (old artifact store entries)
	if env.Env == nil {
		env.Env = make(map[string]string)
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
// DEPRECATED: Use AtomicDeleteTestEnvironment instead for proper atomic operations.
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

// AtomicDeleteTestEnvironment atomically deletes a test environment from the artifact store.
// This function handles file locking internally and reads/writes the store atomically.
// Use this function instead of DeleteTestEnvironment + WriteArtifactStore to avoid race conditions.
func AtomicDeleteTestEnvironment(path string, id string) error {
	// Acquire exclusive lock
	lockFile, err := lockArtifactStore(path)
	if err != nil {
		return flaterrors.Join(err, errWritingArtifactStore)
	}
	defer func() { _ = unlockArtifactStore(lockFile) }()

	// Read current store from disk while holding the lock
	store, err := ReadArtifactStore(path)
	if err != nil {
		return flaterrors.Join(err, errWritingArtifactStore)
	}

	// Delete the test environment
	if store.TestEnvironments == nil {
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

	// Prune old build artifacts (keep only 3 most recent per type+name)
	PruneBuildArtifacts(&store, 3)

	// Write directly without merge (we already read the current state)
	b, err := yaml.Marshal(store)
	if err != nil {
		return flaterrors.Join(err, errWritingArtifactStore)
	}

	if err := os.WriteFile(path, b, 0o600); err != nil {
		return flaterrors.Join(err, errWritingArtifactStore)
	}

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
// DEPRECATED: Use AtomicDeleteTestReport instead for proper atomic operations.
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

// AtomicDeleteTestReport atomically deletes a test report from the artifact store.
// This function handles file locking internally and reads/writes the store atomically.
// Use this function instead of DeleteTestReport + WriteArtifactStore to avoid race conditions.
// Note: This does not delete the actual artifact files. Callers should handle
// file cleanup separately using the report.ArtifactFiles list.
func AtomicDeleteTestReport(path string, id string) error {
	// Acquire exclusive lock
	lockFile, err := lockArtifactStore(path)
	if err != nil {
		return flaterrors.Join(err, errWritingArtifactStore)
	}
	defer func() { _ = unlockArtifactStore(lockFile) }()

	// Read current store from disk while holding the lock
	store, err := ReadArtifactStore(path)
	if err != nil {
		return flaterrors.Join(err, errWritingArtifactStore)
	}

	// Delete the test report
	if store.TestReports == nil {
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

	// Prune old build artifacts (keep only 3 most recent per type+name)
	PruneBuildArtifacts(&store, 3)

	// Write directly without merge (we already read the current state)
	b, err := yaml.Marshal(store)
	if err != nil {
		return flaterrors.Join(err, errWritingArtifactStore)
	}

	if err := os.WriteFile(path, b, 0o600); err != nil {
		return flaterrors.Join(err, errWritingArtifactStore)
	}

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

// platformOrHost names a platform in a message; the empty platform is the
// artifact of no platform, which is what a generator or a command leaves.
func platformOrHost(platform string) string {
	if platform == "" {
		return "none"
	}

	return platform
}
