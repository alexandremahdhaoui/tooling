//go:build unit

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge/pkg/forge"
)

// TestBuildContainerDocker tests the docker mode implementation.
func TestBuildContainerDocker(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()

	// Create a minimal Containerfile
	containerfile := filepath.Join(tmpDir, "Containerfile")
	content := `FROM alpine:3.20
CMD ["echo", "test"]`
	if err := os.WriteFile(containerfile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// Change to temp directory
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Create test spec
	spec := forge.BuildSpec{
		Name:   "test-docker-image",
		Src:    "Containerfile",
		Engine: "go://container-build",
	}

	// Create artifact store
	store := forge.ArtifactStore{
		Artifacts: []forge.Artifact{},
	}

	// Create test envs
	envs := Envs{
		BuildEngine: "docker",
		BuildArgs:   []string{"TEST_ARG=value"},
	}

	version := "test-version-123"
	timestamp := "2024-01-01T00:00:00Z"

	// Note: This test will fail if docker is not installed or not available
	// We'll check for docker availability first
	if _, err := os.Stat("/usr/bin/docker"); os.IsNotExist(err) {
		if _, err := os.Stat("/usr/local/bin/docker"); os.IsNotExist(err) {
			t.Skip("Docker not available, skipping test")
		}
	}

	// Call buildContainerDocker
	// Note: This will actually try to build, which may fail without docker
	// In a real test environment, you'd want to mock exec.Command
	err = buildContainerDocker(envs, spec, version, timestamp, &store, false)

	// We expect this to fail without docker, but we're testing the function structure
	// The real test is that it doesn't panic and returns an error when docker isn't available
	if err != nil {
		// This is expected if docker isn't running or available
		t.Logf("Expected error without docker daemon: %v", err)

		// Verify error contains meaningful information
		if !strings.Contains(err.Error(), "building container") {
			t.Errorf("Error should mention 'building container', got: %v", err)
		}
	} else {
		// If it succeeded, verify artifact was added
		if len(store.Artifacts) == 0 {
			t.Error("Artifact should have been added to store")
		} else {
			artifact := store.Artifacts[0]
			if artifact.Name != spec.Name {
				t.Errorf("Artifact name = %s, want %s", artifact.Name, spec.Name)
			}
			if artifact.Type != "container" {
				t.Errorf("Artifact type = %s, want container", artifact.Type)
			}
			if artifact.Version != version {
				t.Errorf("Artifact version = %s, want %s", artifact.Version, version)
			}
			expectedLocation := "test-docker-image:test-version-123"
			if artifact.Location != expectedLocation {
				t.Errorf("Artifact location = %s, want %s", artifact.Location, expectedLocation)
			}

			// Cleanup: remove test image
			_ = os.RemoveAll(tmpDir)
		}
	}
}

// TestBuildContainerKaniko tests the kaniko mode implementation.
func TestBuildContainerKaniko(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()

	// Create a minimal Containerfile
	containerfile := filepath.Join(tmpDir, "Containerfile")
	content := `FROM alpine:3.20
CMD ["echo", "test"]`
	if err := os.WriteFile(containerfile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// Change to temp directory
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Create test spec
	spec := forge.BuildSpec{
		Name:   "test-kaniko-image",
		Src:    "Containerfile",
		Engine: "go://container-build",
	}

	// Create artifact store
	store := forge.ArtifactStore{
		Artifacts: []forge.Artifact{},
	}

	// Create test envs with custom cache dir
	cacheDir := filepath.Join(tmpDir, ".kaniko-cache")
	envs := Envs{
		BuildEngine:    "kaniko",
		BuildArgs:      []string{"TEST_ARG=value"},
		KanikoCacheDir: cacheDir,
	}

	version := "test-version-456"
	timestamp := "2024-01-01T00:00:00Z"

	// Check if docker is available (kaniko runs in docker)
	if _, err := os.Stat("/usr/bin/docker"); os.IsNotExist(err) {
		if _, err := os.Stat("/usr/local/bin/docker"); os.IsNotExist(err) {
			t.Skip("Docker not available, skipping test (kaniko requires docker)")
		}
	}

	// Call buildContainerKaniko
	err = buildContainerKaniko(envs, spec, version, timestamp, &store, false)

	// We expect this to fail without docker daemon, but we're testing the function structure
	if err != nil {
		// This is expected if docker isn't running
		t.Logf("Expected error without docker daemon: %v", err)

		// Verify error contains meaningful information
		if !strings.Contains(err.Error(), "building container") {
			t.Errorf("Error should mention 'building container', got: %v", err)
		}

		// Verify cache directory was created even if build failed
		if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
			t.Error("Cache directory should have been created")
		}
	} else {
		// If it succeeded, verify artifact was added
		if len(store.Artifacts) == 0 {
			t.Error("Artifact should have been added to store")
		} else {
			artifact := store.Artifacts[0]
			if artifact.Name != spec.Name {
				t.Errorf("Artifact name = %s, want %s", artifact.Name, spec.Name)
			}
			if artifact.Type != "container" {
				t.Errorf("Artifact type = %s, want container", artifact.Type)
			}
			if artifact.Version != version {
				t.Errorf("Artifact version = %s, want %s", artifact.Version, version)
			}

			// Verify tar file was cleaned up
			tarPath := filepath.Join(tmpDir, ".ignore.test-kaniko-image.tar")
			if _, err := os.Stat(tarPath); err == nil {
				t.Error("Tar file should have been cleaned up")
			}
		}
	}
}

// TestBuildContainerPodman tests the podman mode implementation.
func TestBuildContainerPodman(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()

	// Create a minimal Containerfile
	containerfile := filepath.Join(tmpDir, "Containerfile")
	content := `FROM alpine:3.20
CMD ["echo", "test"]`
	if err := os.WriteFile(containerfile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// Change to temp directory
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Create test spec
	spec := forge.BuildSpec{
		Name:   "test-podman-image",
		Src:    "Containerfile",
		Engine: "go://container-build",
	}

	// Create artifact store
	store := forge.ArtifactStore{
		Artifacts: []forge.Artifact{},
	}

	// Create test envs
	envs := Envs{
		BuildEngine: "podman",
		BuildArgs:   []string{"TEST_ARG=value"},
	}

	version := "test-version-789"
	timestamp := "2024-01-01T00:00:00Z"

	// Check if podman is available
	if _, err := os.Stat("/usr/bin/podman"); os.IsNotExist(err) {
		if _, err := os.Stat("/usr/local/bin/podman"); os.IsNotExist(err) {
			t.Skip("Podman not available, skipping test")
		}
	}

	// Call buildContainerPodman
	err = buildContainerPodman(envs, spec, version, timestamp, &store, false)

	// We expect this to fail without podman, but we're testing the function structure
	if err != nil {
		// This is expected if podman isn't installed
		t.Logf("Expected error without podman: %v", err)

		// Verify error contains meaningful information
		if !strings.Contains(err.Error(), "building container") {
			t.Errorf("Error should mention 'building container', got: %v", err)
		}
	} else {
		// If it succeeded, verify artifact was added
		if len(store.Artifacts) == 0 {
			t.Error("Artifact should have been added to store")
		} else {
			artifact := store.Artifacts[0]
			if artifact.Name != spec.Name {
				t.Errorf("Artifact name = %s, want %s", artifact.Name, spec.Name)
			}
			if artifact.Type != "container" {
				t.Errorf("Artifact type = %s, want container", artifact.Type)
			}
			if artifact.Version != version {
				t.Errorf("Artifact version = %s, want %s", artifact.Version, version)
			}
		}
	}
}

// TestBuildContainerDispatcher tests the dispatcher routing logic.
func TestBuildContainerDispatcher(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()

	// Create a minimal Containerfile
	containerfile := filepath.Join(tmpDir, "Containerfile")
	content := `FROM alpine:3.20
CMD ["echo", "test"]`
	if err := os.WriteFile(containerfile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// Change to temp directory
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Create test spec
	spec := forge.BuildSpec{
		Name:   "test-dispatcher-image",
		Src:    "Containerfile",
		Engine: "go://container-build",
	}

	version := "test-version-dispatch"
	timestamp := "2024-01-01T00:00:00Z"

	tests := []struct {
		name        string
		buildEngine string
		shouldError bool
		errorMsg    string
	}{
		{
			name:        "docker mode",
			buildEngine: "docker",
			shouldError: false, // May error if docker not available, but dispatcher works
		},
		{
			name:        "kaniko mode",
			buildEngine: "kaniko",
			shouldError: false, // May error if docker not available, but dispatcher works
		},
		{
			name:        "podman mode",
			buildEngine: "podman",
			shouldError: false, // May error if podman not available, but dispatcher works
		},
		{
			name:        "invalid mode",
			buildEngine: "invalid",
			shouldError: true,
			errorMsg:    "unsupported container engine",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envs := Envs{
				BuildEngine: tt.buildEngine,
			}

			// Create fresh store for each test
			testStore := forge.ArtifactStore{
				Artifacts: []forge.Artifact{},
			}

			err := buildContainer(envs, spec, version, timestamp, &testStore, false)

			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected error for %s, got nil", tt.name)
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing %q, got: %v", tt.errorMsg, err)
				}
			} else {
				// For valid modes, error may occur if tool not installed, but not due to dispatcher
				if err != nil {
					// Verify it's not a dispatcher error
					if strings.Contains(err.Error(), "unsupported container engine") {
						t.Errorf("Dispatcher failed for valid engine %s: %v", tt.buildEngine, err)
					}
					// Otherwise it's just the build tool not being available
					t.Logf("Build failed (expected if %s not installed): %v", tt.buildEngine, err)
				}
			}
		})
	}
}

// TestAddArtifactToStore tests the artifact addition logic.
func TestAddArtifactToStore(t *testing.T) {
	store := forge.ArtifactStore{
		Artifacts: []forge.Artifact{},
	}

	name := "test-container"
	version := "abc123def"
	timestamp := "2024-01-01T00:00:00Z"

	// Add artifact
	addArtifactToStore(&store, name, version, timestamp)

	// Verify artifact was added
	if len(store.Artifacts) != 1 {
		t.Fatalf("Expected 1 artifact, got %d", len(store.Artifacts))
	}

	artifact := store.Artifacts[0]

	// Verify artifact fields
	if artifact.Name != name {
		t.Errorf("Artifact name = %s, want %s", artifact.Name, name)
	}

	if artifact.Type != "container" {
		t.Errorf("Artifact type = %s, want container", artifact.Type)
	}

	expectedLocation := "test-container:abc123def"
	if artifact.Location != expectedLocation {
		t.Errorf("Artifact location = %s, want %s", artifact.Location, expectedLocation)
	}

	if artifact.Version != version {
		t.Errorf("Artifact version = %s, want %s", artifact.Version, version)
	}

	if artifact.Timestamp != timestamp {
		t.Errorf("Artifact timestamp = %s, want %s", artifact.Timestamp, timestamp)
	}

	// Add same artifact again with new version
	newVersion := "xyz789abc"
	newTimestamp := "2024-01-02T00:00:00Z"
	addArtifactToStore(&store, name, newVersion, newTimestamp)

	// Should have 2 artifacts (different versions)
	if len(store.Artifacts) != 2 {
		t.Fatalf("Expected 2 artifacts with different versions, got %d", len(store.Artifacts))
	}

	// Verify second artifact has new version
	secondArtifact := store.Artifacts[1]
	if secondArtifact.Version != newVersion {
		t.Errorf("Second artifact version = %s, want %s", secondArtifact.Version, newVersion)
	}

	// Add same artifact again with same name and version (should update timestamp)
	newerTimestamp := "2024-01-03T00:00:00Z"
	addArtifactToStore(&store, name, newVersion, newerTimestamp)

	// Should still have 2 artifacts (updated timestamp on second one)
	if len(store.Artifacts) != 2 {
		t.Fatalf("Expected 2 artifacts after timestamp update, got %d", len(store.Artifacts))
	}

	updatedArtifact := store.Artifacts[1]
	if updatedArtifact.Timestamp != newerTimestamp {
		t.Errorf("Updated artifact timestamp = %s, want %s", updatedArtifact.Timestamp, newerTimestamp)
	}
}

// TestTagImage tests the image tagging logic.
func TestTagImage(t *testing.T) {
	// This test verifies the function signature and error handling
	// We can't actually test docker tag without docker running

	tests := []struct {
		name            string
		containerEngine string
		imageID         string
		tag             string
		wantErr         bool
	}{
		{
			name:            "invalid engine",
			containerEngine: "nonexistent-engine-xyz",
			imageID:         "test-image:latest",
			tag:             "test-image:v1",
			wantErr:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tagImage(tt.containerEngine, tt.imageID, tt.tag, false)

			if (err != nil) != tt.wantErr {
				t.Errorf("tagImage() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestGetImageIDFromTar tests the tar image ID extraction logic.
func TestGetImageIDFromTar(t *testing.T) {
	// This test verifies error handling when tar file doesn't exist
	tmpDir := t.TempDir()
	nonExistentTar := filepath.Join(tmpDir, "nonexistent.tar")

	_, err := getImageIDFromTar("docker", nonExistentTar)
	if err == nil {
		t.Error("Expected error for non-existent tar file")
	}

	if !strings.Contains(err.Error(), "getting image ID from tar") {
		t.Errorf("Error should mention 'getting image ID from tar', got: %v", err)
	}
}
