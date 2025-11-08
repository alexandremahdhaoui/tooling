//go:build unit

package forge

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"sigs.k8s.io/yaml"
)

func TestAddOrUpdateTestEnvironment(t *testing.T) {
	store := &ArtifactStore{
		Version:          "1.0",
		TestEnvironments: make(map[string]*TestEnvironment),
	}

	env := &TestEnvironment{
		ID:               "test-integration-20241103-abc123",
		Name:             "integration",
		Status:           TestStatusCreated,
		CreatedAt:        time.Now().UTC(),
		ManagedResources: []string{},
		Metadata:         make(map[string]string),
	}

	// Test add
	AddOrUpdateTestEnvironment(store, env)

	if len(store.TestEnvironments) != 1 {
		t.Errorf("Expected 1 test environment, got %d", len(store.TestEnvironments))
	}

	retrieved, ok := store.TestEnvironments[env.ID]
	if !ok {
		t.Fatal("Test environment not found after adding")
	}

	if retrieved.ID != env.ID {
		t.Errorf("Expected ID %s, got %s", env.ID, retrieved.ID)
	}

	if retrieved.Status != TestStatusCreated {
		t.Errorf("Expected status %s, got %s", TestStatusCreated, retrieved.Status)
	}

	// Test update
	env.Status = TestStatusPassed
	oldUpdatedAt := retrieved.UpdatedAt
	time.Sleep(10 * time.Millisecond) // Ensure time difference
	AddOrUpdateTestEnvironment(store, env)

	retrieved = store.TestEnvironments[env.ID]
	if retrieved.Status != TestStatusPassed {
		t.Errorf("Expected status %s, got %s", TestStatusPassed, retrieved.Status)
	}

	if !retrieved.UpdatedAt.After(oldUpdatedAt) {
		t.Error("UpdatedAt should be updated")
	}
}

func TestAddOrUpdateTestEnvironment_NilStore(t *testing.T) {
	env := &TestEnvironment{ID: "test-123"}

	// Should not panic
	AddOrUpdateTestEnvironment(nil, env)
}

func TestAddOrUpdateTestEnvironment_NilEnvironment(t *testing.T) {
	store := &ArtifactStore{
		TestEnvironments: make(map[string]*TestEnvironment),
	}

	// Should not panic
	AddOrUpdateTestEnvironment(store, nil)

	if len(store.TestEnvironments) != 0 {
		t.Error("No environments should be added")
	}
}

func TestGetTestEnvironment(t *testing.T) {
	store := &ArtifactStore{
		Version: "1.0",
		TestEnvironments: map[string]*TestEnvironment{
			"test-123": {
				ID:     "test-123",
				Name:   "integration",
				Status: TestStatusCreated,
			},
		},
	}

	// Test successful get
	env, err := GetTestEnvironment(store, "test-123")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if env.ID != "test-123" {
		t.Errorf("Expected ID test-123, got %s", env.ID)
	}

	// Test not found
	_, err = GetTestEnvironment(store, "nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent environment")
	}
}

func TestGetTestEnvironment_NilStore(t *testing.T) {
	_, err := GetTestEnvironment(nil, "test-123")
	if err == nil {
		t.Error("Expected error for nil store")
	}
}

func TestListTestEnvironments(t *testing.T) {
	now := time.Now().UTC()
	store := &ArtifactStore{
		Version: "1.0",
		TestEnvironments: map[string]*TestEnvironment{
			"test-integration-1": {
				ID:        "test-integration-1",
				Name:      "integration",
				Status:    TestStatusCreated,
				CreatedAt: now,
			},
			"test-integration-2": {
				ID:        "test-integration-2",
				Name:      "integration",
				Status:    TestStatusPassed,
				CreatedAt: now.Add(1 * time.Hour),
			},
			"test-unit-1": {
				ID:        "test-unit-1",
				Name:      "unit",
				Status:    TestStatusCreated,
				CreatedAt: now.Add(2 * time.Hour),
			},
		},
	}

	// Test list all integration environments
	integrationEnvs := ListTestEnvironments(store, "integration")
	if len(integrationEnvs) != 2 {
		t.Errorf("Expected 2 integration environments, got %d", len(integrationEnvs))
	}

	// Verify both integration environments are present
	foundIDs := make(map[string]bool)
	for _, env := range integrationEnvs {
		foundIDs[env.ID] = true
	}
	if !foundIDs["test-integration-1"] || !foundIDs["test-integration-2"] {
		t.Error("Not all integration environments found")
	}

	// Test list unit environments
	unitEnvs := ListTestEnvironments(store, "unit")
	if len(unitEnvs) != 1 {
		t.Errorf("Expected 1 unit environment, got %d", len(unitEnvs))
	}

	if unitEnvs[0].ID != "test-unit-1" {
		t.Errorf("Expected test-unit-1, got %s", unitEnvs[0].ID)
	}

	// Test list all environments (empty stage name)
	allEnvs := ListTestEnvironments(store, "")
	if len(allEnvs) != 3 {
		t.Errorf("Expected 3 total environments, got %d", len(allEnvs))
	}

	// Test nonexistent stage
	noEnvs := ListTestEnvironments(store, "nonexistent")
	if len(noEnvs) != 0 {
		t.Errorf("Expected 0 environments for nonexistent stage, got %d", len(noEnvs))
	}
}

func TestListTestEnvironments_NilStore(t *testing.T) {
	envs := ListTestEnvironments(nil, "integration")
	if len(envs) != 0 {
		t.Error("Expected empty list for nil store")
	}
}

func TestDeleteTestEnvironment(t *testing.T) {
	store := &ArtifactStore{
		Version: "1.0",
		TestEnvironments: map[string]*TestEnvironment{
			"test-123": {
				ID:     "test-123",
				Name:   "integration",
				Status: TestStatusCreated,
			},
		},
	}

	// Test successful delete
	err := DeleteTestEnvironment(store, "test-123")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(store.TestEnvironments) != 0 {
		t.Error("Environment should be deleted")
	}

	// Test delete nonexistent
	err = DeleteTestEnvironment(store, "nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent environment")
	}
}

func TestDeleteTestEnvironment_NilStore(t *testing.T) {
	err := DeleteTestEnvironment(nil, "test-123")
	if err == nil {
		t.Error("Expected error for nil store")
	}
}

func TestArtifactStoreReadWrite(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "test-artifact-store.yaml")

	// Create test store
	now := time.Now().UTC()
	store := ArtifactStore{
		Version:     "1.0",
		LastUpdated: now,
		Artifacts: []Artifact{
			{
				Name:      "test-binary",
				Type:      "binary",
				Location:  "./build/bin/test",
				Timestamp: now.Format(time.RFC3339),
				Version:   "v1.0.0",
			},
		},
		TestEnvironments: map[string]*TestEnvironment{
			"test-123": {
				ID:        "test-123",
				Name:      "integration",
				Status:    TestStatusPassed,
				CreatedAt: now,
				UpdatedAt: now,
				TmpDir:    "/tmp/test-dir",
				Files: map[string]string{
					"testenv-kind.kubeconfig": "kubeconfig",
				},
				ManagedResources: []string{"/tmp/test-dir"},
				Metadata: map[string]string{
					"key": "value",
				},
			},
		},
	}

	// Write store
	err := WriteArtifactStore(storePath, store)
	if err != nil {
		t.Fatalf("Failed to write artifact store: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(storePath); os.IsNotExist(err) {
		t.Fatal("Artifact store file not created")
	}

	// Read store
	readStore, err := ReadArtifactStore(storePath)
	if err != nil {
		t.Fatalf("Failed to read artifact store: %v", err)
	}

	// Verify version
	if readStore.Version != "1.0" {
		t.Errorf("Expected version 1.0, got %s", readStore.Version)
	}

	// Verify artifacts
	if len(readStore.Artifacts) != 1 {
		t.Errorf("Expected 1 artifact, got %d", len(readStore.Artifacts))
	}

	if readStore.Artifacts[0].Name != "test-binary" {
		t.Errorf("Expected artifact name test-binary, got %s", readStore.Artifacts[0].Name)
	}

	// Verify test environments
	if len(readStore.TestEnvironments) != 1 {
		t.Errorf("Expected 1 test environment, got %d", len(readStore.TestEnvironments))
	}

	env, ok := readStore.TestEnvironments["test-123"]
	if !ok {
		t.Fatal("Test environment test-123 not found")
	}

	if env.Name != "integration" {
		t.Errorf("Expected name integration, got %s", env.Name)
	}

	if env.Status != TestStatusPassed {
		t.Errorf("Expected status %s, got %s", TestStatusPassed, env.Status)
	}

	if env.TmpDir != "/tmp/test-dir" {
		t.Errorf("Expected tmpDir /tmp/test-dir, got %s", env.TmpDir)
	}

	if len(env.Files) != 1 {
		t.Errorf("Expected 1 file, got %d", len(env.Files))
	}

	if env.Files["testenv-kind.kubeconfig"] != "kubeconfig" {
		t.Errorf("Expected kubeconfig file, got %v", env.Files)
	}

	if len(env.ManagedResources) != 1 || env.ManagedResources[0] != "/tmp/test-dir" {
		t.Errorf("Expected managed resource /tmp/test-dir, got %v", env.ManagedResources)
	}

	if env.Metadata["key"] != "value" {
		t.Errorf("Expected metadata key=value, got %v", env.Metadata)
	}
}

func TestReadArtifactStore_NonexistentFile(t *testing.T) {
	// ReadArtifactStore should return an error for nonexistent files
	_, err := ReadArtifactStore("/nonexistent/path/store.yaml")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestReadOrCreateArtifactStore_NonexistentFile(t *testing.T) {
	// ReadOrCreateArtifactStore returns an empty store when file doesn't exist (no error)
	store, err := ReadOrCreateArtifactStore("/nonexistent/path/store.yaml")
	if err != nil {
		t.Errorf("Expected no error for nonexistent file, got %v", err)
	}

	// Verify empty store is initialized properly
	if store.Version != "1.0" {
		t.Errorf("Expected version 1.0, got %s", store.Version)
	}

	if store.Artifacts == nil {
		t.Error("Expected initialized artifacts slice")
	}

	if store.TestEnvironments == nil {
		t.Error("Expected initialized test environments map")
	}
}

func TestReadOrCreateArtifactStore_ExistingFile(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "test-store.yaml")

	// Create initial store
	initialStore := ArtifactStore{
		Version:     "1.0",
		LastUpdated: time.Now().UTC(),
		Artifacts: []Artifact{
			{
				Name:      "test-binary",
				Type:      "binary",
				Location:  "./build/bin/test",
				Timestamp: time.Now().Format(time.RFC3339),
				Version:   "v1.0.0",
			},
		},
		TestEnvironments: make(map[string]*TestEnvironment),
	}

	err := WriteArtifactStore(storePath, initialStore)
	if err != nil {
		t.Fatalf("Failed to write initial store: %v", err)
	}

	// ReadOrCreateArtifactStore should read the existing file
	store, err := ReadOrCreateArtifactStore(storePath)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(store.Artifacts) != 1 {
		t.Errorf("Expected 1 artifact, got %d", len(store.Artifacts))
	}

	if store.Artifacts[0].Name != "test-binary" {
		t.Errorf("Expected artifact name test-binary, got %s", store.Artifacts[0].Name)
	}
}

func TestTestEnvironmentStatusConstants(t *testing.T) {
	// Verify status constants are defined
	if TestStatusCreated != "created" {
		t.Errorf("Expected TestStatusCreated to be 'created', got %s", TestStatusCreated)
	}
	if TestStatusRunning != "running" {
		t.Errorf("Expected TestStatusRunning to be 'running', got %s", TestStatusRunning)
	}
	if TestStatusPassed != "passed" {
		t.Errorf("Expected TestStatusPassed to be 'passed', got %s", TestStatusPassed)
	}
	if TestStatusFailed != "failed" {
		t.Errorf("Expected TestStatusFailed to be 'failed', got %s", TestStatusFailed)
	}
	if TestStatusPartiallyDeleted != "partially_deleted" {
		t.Errorf("Expected TestStatusPartiallyDeleted to be 'partially_deleted', got %s", TestStatusPartiallyDeleted)
	}
}

func TestPruneBuildArtifacts_KeepsThreeMostRecent(t *testing.T) {
	now := time.Now().UTC()

	store := &ArtifactStore{
		Version: "1.0",
		Artifacts: []Artifact{
			{
				Name:      "test-binary",
				Type:      "binary",
				Location:  "./build/bin/test1",
				Timestamp: now.Add(-4 * time.Hour).Format(time.RFC3339),
				Version:   "v1.0.0",
			},
			{
				Name:      "test-binary",
				Type:      "binary",
				Location:  "./build/bin/test2",
				Timestamp: now.Add(-3 * time.Hour).Format(time.RFC3339),
				Version:   "v1.0.1",
			},
			{
				Name:      "test-binary",
				Type:      "binary",
				Location:  "./build/bin/test3",
				Timestamp: now.Add(-2 * time.Hour).Format(time.RFC3339),
				Version:   "v1.0.2",
			},
			{
				Name:      "test-binary",
				Type:      "binary",
				Location:  "./build/bin/test4",
				Timestamp: now.Add(-1 * time.Hour).Format(time.RFC3339),
				Version:   "v1.0.3",
			},
			{
				Name:      "test-binary",
				Type:      "binary",
				Location:  "./build/bin/test5",
				Timestamp: now.Format(time.RFC3339),
				Version:   "v1.0.4",
			},
		},
		TestEnvironments: make(map[string]*TestEnvironment),
	}

	PruneBuildArtifacts(store, 3)

	// Should keep only 3 most recent artifacts
	if len(store.Artifacts) != 3 {
		t.Errorf("Expected 3 artifacts after pruning, got %d", len(store.Artifacts))
	}

	// Verify the 3 most recent are kept
	foundVersions := make(map[string]bool)
	for _, artifact := range store.Artifacts {
		foundVersions[artifact.Version] = true
	}

	// Should keep v1.0.4, v1.0.3, v1.0.2 (the 3 most recent)
	expectedVersions := []string{"v1.0.4", "v1.0.3", "v1.0.2"}
	for _, v := range expectedVersions {
		if !foundVersions[v] {
			t.Errorf("Expected to find version %s after pruning", v)
		}
	}

	// Should NOT keep v1.0.0, v1.0.1 (the oldest 2)
	unexpectedVersions := []string{"v1.0.0", "v1.0.1"}
	for _, v := range unexpectedVersions {
		if foundVersions[v] {
			t.Errorf("Expected version %s to be pruned", v)
		}
	}
}

func TestPruneBuildArtifacts_MultipleLTypes(t *testing.T) {
	now := time.Now().UTC()

	store := &ArtifactStore{
		Version: "1.0",
		Artifacts: []Artifact{
			// Binary artifacts (5 total, should keep 3)
			{Name: "app", Type: "binary", Location: "./build/bin/app-v1", Timestamp: now.Add(-4 * time.Hour).Format(time.RFC3339), Version: "v1"},
			{Name: "app", Type: "binary", Location: "./build/bin/app-v2", Timestamp: now.Add(-3 * time.Hour).Format(time.RFC3339), Version: "v2"},
			{Name: "app", Type: "binary", Location: "./build/bin/app-v3", Timestamp: now.Add(-2 * time.Hour).Format(time.RFC3339), Version: "v3"},
			{Name: "app", Type: "binary", Location: "./build/bin/app-v4", Timestamp: now.Add(-1 * time.Hour).Format(time.RFC3339), Version: "v4"},
			{Name: "app", Type: "binary", Location: "./build/bin/app-v5", Timestamp: now.Format(time.RFC3339), Version: "v5"},
			// Container artifacts (4 total, should keep 3)
			{Name: "app", Type: "container", Location: "registry.local/app:c1", Timestamp: now.Add(-3 * time.Hour).Format(time.RFC3339), Version: "c1"},
			{Name: "app", Type: "container", Location: "registry.local/app:c2", Timestamp: now.Add(-2 * time.Hour).Format(time.RFC3339), Version: "c2"},
			{Name: "app", Type: "container", Location: "registry.local/app:c3", Timestamp: now.Add(-1 * time.Hour).Format(time.RFC3339), Version: "c3"},
			{Name: "app", Type: "container", Location: "registry.local/app:c4", Timestamp: now.Format(time.RFC3339), Version: "c4"},
		},
		TestEnvironments: make(map[string]*TestEnvironment),
	}

	PruneBuildArtifacts(store, 3)

	// Should keep 3 binaries + 3 containers = 6 total
	if len(store.Artifacts) != 6 {
		t.Errorf("Expected 6 artifacts after pruning, got %d", len(store.Artifacts))
	}

	// Count by type
	binaryCount := 0
	containerCount := 0
	for _, artifact := range store.Artifacts {
		switch artifact.Type {
		case "binary":
			binaryCount++
		case "container":
			containerCount++
		}
	}

	if binaryCount != 3 {
		t.Errorf("Expected 3 binary artifacts, got %d", binaryCount)
	}

	if containerCount != 3 {
		t.Errorf("Expected 3 container artifacts, got %d", containerCount)
	}
}

func TestPruneBuildArtifacts_MultipleNames(t *testing.T) {
	now := time.Now().UTC()

	store := &ArtifactStore{
		Version: "1.0",
		Artifacts: []Artifact{
			// app1 artifacts (4 total, should keep 3)
			{Name: "app1", Type: "binary", Location: "./build/bin/app1-v1", Timestamp: now.Add(-3 * time.Hour).Format(time.RFC3339), Version: "v1"},
			{Name: "app1", Type: "binary", Location: "./build/bin/app1-v2", Timestamp: now.Add(-2 * time.Hour).Format(time.RFC3339), Version: "v2"},
			{Name: "app1", Type: "binary", Location: "./build/bin/app1-v3", Timestamp: now.Add(-1 * time.Hour).Format(time.RFC3339), Version: "v3"},
			{Name: "app1", Type: "binary", Location: "./build/bin/app1-v4", Timestamp: now.Format(time.RFC3339), Version: "v4"},
			// app2 artifacts (2 total, should keep all)
			{Name: "app2", Type: "binary", Location: "./build/bin/app2-v1", Timestamp: now.Add(-1 * time.Hour).Format(time.RFC3339), Version: "v1"},
			{Name: "app2", Type: "binary", Location: "./build/bin/app2-v2", Timestamp: now.Format(time.RFC3339), Version: "v2"},
		},
		TestEnvironments: make(map[string]*TestEnvironment),
	}

	PruneBuildArtifacts(store, 3)

	// Should keep 3 app1 + 2 app2 = 5 total
	if len(store.Artifacts) != 5 {
		t.Errorf("Expected 5 artifacts after pruning, got %d", len(store.Artifacts))
	}

	// Count by name
	app1Count := 0
	app2Count := 0
	for _, artifact := range store.Artifacts {
		switch artifact.Name {
		case "app1":
			app1Count++
		case "app2":
			app2Count++
		}
	}

	if app1Count != 3 {
		t.Errorf("Expected 3 app1 artifacts, got %d", app1Count)
	}

	if app2Count != 2 {
		t.Errorf("Expected 2 app2 artifacts (no pruning needed), got %d", app2Count)
	}
}

func TestPruneBuildArtifacts_NoLPruningNeeded(t *testing.T) {
	now := time.Now().UTC()

	store := &ArtifactStore{
		Version: "1.0",
		Artifacts: []Artifact{
			{Name: "app", Type: "binary", Location: "./build/bin/app-v1", Timestamp: now.Add(-2 * time.Hour).Format(time.RFC3339), Version: "v1"},
			{Name: "app", Type: "binary", Location: "./build/bin/app-v2", Timestamp: now.Add(-1 * time.Hour).Format(time.RFC3339), Version: "v2"},
			{Name: "app", Type: "binary", Location: "./build/bin/app-v3", Timestamp: now.Format(time.RFC3339), Version: "v3"},
		},
		TestEnvironments: make(map[string]*TestEnvironment),
	}

	PruneBuildArtifacts(store, 3)

	// Should keep all 3 artifacts
	if len(store.Artifacts) != 3 {
		t.Errorf("Expected 3 artifacts (no pruning), got %d", len(store.Artifacts))
	}
}

func TestPruneBuildArtifacts_InvalidTimestamps(t *testing.T) {
	now := time.Now().UTC()

	store := &ArtifactStore{
		Version: "1.0",
		Artifacts: []Artifact{
			{Name: "app", Type: "binary", Location: "./build/bin/app-v1", Timestamp: "invalid-timestamp", Version: "v1"},
			{Name: "app", Type: "binary", Location: "./build/bin/app-v2", Timestamp: now.Add(-1 * time.Hour).Format(time.RFC3339), Version: "v2"},
			{Name: "app", Type: "binary", Location: "./build/bin/app-v3", Timestamp: now.Format(time.RFC3339), Version: "v3"},
		},
		TestEnvironments: make(map[string]*TestEnvironment),
	}

	// Should not panic with invalid timestamps
	PruneBuildArtifacts(store, 3)

	// Should keep all artifacts (1 invalid + 2 valid)
	if len(store.Artifacts) != 3 {
		t.Errorf("Expected 3 artifacts, got %d", len(store.Artifacts))
	}
}

func TestPruneBuildArtifacts_NilStore(t *testing.T) {
	// Should not panic with nil store
	PruneBuildArtifacts(nil, 3)
}

func TestPruneBuildArtifacts_EmptyStore(t *testing.T) {
	store := &ArtifactStore{
		Version:          "1.0",
		Artifacts:        []Artifact{},
		TestEnvironments: make(map[string]*TestEnvironment),
	}

	// Should not panic with empty artifacts
	PruneBuildArtifacts(store, 3)

	if len(store.Artifacts) != 0 {
		t.Errorf("Expected 0 artifacts, got %d", len(store.Artifacts))
	}
}

func TestWriteArtifactStore_AutomaticPruning(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "test-store.yaml")

	now := time.Now().UTC()

	// Create store with more than 3 artifacts of same type+name
	store := ArtifactStore{
		Version:     "1.0",
		LastUpdated: now,
		Artifacts: []Artifact{
			{Name: "app", Type: "binary", Location: "./build/bin/app-v1", Timestamp: now.Add(-4 * time.Hour).Format(time.RFC3339), Version: "v1"},
			{Name: "app", Type: "binary", Location: "./build/bin/app-v2", Timestamp: now.Add(-3 * time.Hour).Format(time.RFC3339), Version: "v2"},
			{Name: "app", Type: "binary", Location: "./build/bin/app-v3", Timestamp: now.Add(-2 * time.Hour).Format(time.RFC3339), Version: "v3"},
			{Name: "app", Type: "binary", Location: "./build/bin/app-v4", Timestamp: now.Add(-1 * time.Hour).Format(time.RFC3339), Version: "v4"},
			{Name: "app", Type: "binary", Location: "./build/bin/app-v5", Timestamp: now.Format(time.RFC3339), Version: "v5"},
		},
		TestEnvironments: make(map[string]*TestEnvironment),
	}

	// Write store (should automatically prune)
	err := WriteArtifactStore(storePath, store)
	if err != nil {
		t.Fatalf("Failed to write artifact store: %v", err)
	}

	// Read back and verify pruning happened
	readStore, err := ReadArtifactStore(storePath)
	if err != nil {
		t.Fatalf("Failed to read artifact store: %v", err)
	}

	if len(readStore.Artifacts) != 3 {
		t.Errorf("Expected 3 artifacts after automatic pruning, got %d", len(readStore.Artifacts))
	}

	// Verify the 3 most recent are kept
	foundVersions := make(map[string]bool)
	for _, artifact := range readStore.Artifacts {
		foundVersions[artifact.Version] = true
	}

	expectedVersions := []string{"v5", "v4", "v3"}
	for _, v := range expectedVersions {
		if !foundVersions[v] {
			t.Errorf("Expected to find version %s after automatic pruning", v)
		}
	}
}

func TestGetArtifactStorePath_WithConfiguredPath(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Create forge.yaml with custom artifact store path
	spec := Spec{
		Name:              "test-project",
		ArtifactStorePath: "/custom/path/artifacts.yaml",
	}

	data, err := yaml.Marshal(spec)
	if err != nil {
		t.Fatalf("Failed to marshal spec: %v", err)
	}

	if err := os.WriteFile("forge.yaml", data, 0o644); err != nil {
		t.Fatalf("Failed to write forge.yaml: %v", err)
	}

	// Test GetArtifactStorePath
	path, err := GetArtifactStorePath(".forge/artifacts.yaml")
	if err != nil {
		t.Fatalf("GetArtifactStorePath failed: %v", err)
	}

	if path != "/custom/path/artifacts.yaml" {
		t.Errorf("Expected configured path '/custom/path/artifacts.yaml', got %s", path)
	}
}

func TestGetArtifactStorePath_WithDefaultPath(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Create forge.yaml without artifact store path
	spec := Spec{
		Name:              "test-project",
		ArtifactStorePath: ".forge/artifacts.yaml",
	}

	data, err := yaml.Marshal(spec)
	if err != nil {
		t.Fatalf("Failed to marshal spec: %v", err)
	}

	if err := os.WriteFile("forge.yaml", data, 0o644); err != nil {
		t.Fatalf("Failed to write forge.yaml: %v", err)
	}

	// Test GetArtifactStorePath - should return default
	path, err := GetArtifactStorePath(".forge/artifacts.yaml")
	if err != nil {
		t.Fatalf("GetArtifactStorePath failed: %v", err)
	}

	if path != ".forge/artifacts.yaml" {
		t.Errorf("Expected default path '.forge/artifacts.yaml', got %s", path)
	}
}

func TestGetArtifactStorePath_NoForgeYaml(t *testing.T) {
	// Create temporary directory without forge.yaml
	tmpDir := t.TempDir()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Test GetArtifactStorePath - should return error
	_, err = GetArtifactStorePath(".forge/artifacts.yaml")
	if err == nil {
		t.Error("Expected error when forge.yaml doesn't exist, got nil")
	}
}
