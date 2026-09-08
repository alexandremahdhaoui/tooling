//go:build integration

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

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alexandremahdhaoui/forge/internal/forgepath"
	"github.com/alexandremahdhaoui/forge/pkg/forge"
)

func TestGenerateTestID(t *testing.T) {
	// Test ID generation
	testID1 := generateTestID("integration")
	testID2 := generateTestID("integration")

	// Verify format: test-{stage}-{date}-{random}
	if !strings.HasPrefix(testID1, "test-integration-") {
		t.Errorf("Expected ID to start with 'test-integration-', got %s", testID1)
	}

	// Verify uniqueness
	if testID1 == testID2 {
		t.Error("Expected unique test IDs")
	}

	// Test with different stage names
	testID3 := generateTestID("unit")
	if !strings.HasPrefix(testID3, "test-unit-") {
		t.Errorf("Expected ID to start with 'test-unit-', got %s", testID3)
	}
}

func TestGenerateTestID_Format(t *testing.T) {
	testID := generateTestID("e2e")

	// Split and verify parts
	parts := strings.Split(testID, "-")
	if len(parts) != 4 {
		t.Errorf("Expected 4 parts in test ID, got %d: %s", len(parts), testID)
	}

	if parts[0] != "test" {
		t.Errorf("Expected first part 'test', got %s", parts[0])
	}

	if parts[1] != "e2e" {
		t.Errorf("Expected second part 'e2e', got %s", parts[1])
	}

	// Verify date format (YYYYMMDD)
	if len(parts[2]) != 8 {
		t.Errorf("Expected date part to be 8 characters, got %d: %s", len(parts[2]), parts[2])
	}

	// Verify random suffix is hex
	if len(parts[3]) != 8 {
		t.Errorf("Expected random part to be 8 characters, got %d: %s", len(parts[3]), parts[3])
	}
}

func TestCmdCreate_Integration(t *testing.T) {
	// This test verifies cmdCreate works with a minimal forge.yaml
	// It will fail if test-report engine isn't available (which is expected behavior)

	// Find the forge repository path
	forgeRepoPath, err := forgepath.FindForgeRepo()
	if err != nil {
		t.Fatalf("Failed to find forge repository: %v", err)
	}

	// Set FORGE_RUN_LOCAL_ENABLED and FORGE_RUN_LOCAL_BASEDIR so engines can be resolved from local source
	t.Setenv("FORGE_RUN_LOCAL_ENABLED", "true")
	t.Setenv("FORGE_RUN_LOCAL_BASEDIR", forgeRepoPath)

	// Create temporary directory for test
	tmpDir := t.TempDir()
	artifactStorePath := filepath.Join(tmpDir, "artifact-store.yaml")

	// Create minimal forge.yaml (testenv will default to forge://test-report)
	forgeYAML := `name: test-project
artifactStorePath: ` + artifactStorePath + `
test:
  - name: integration
    runner: "forge://go-test"
`
	forgeYAMLPath := filepath.Join(tmpDir, "forge.yaml")
	err = os.WriteFile(forgeYAMLPath, []byte(forgeYAML), 0o644)
	if err != nil {
		t.Fatalf("Failed to write forge.yaml: %v", err)
	}

	// Change to temp directory
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	// Run cmdCreate - will fail if forge://test-report isn't available
	testID, err := cmdCreate("integration")
	if err != nil {
		t.Fatalf("cmdCreate failed: %v", err)
	}
	if testID == "" {
		t.Fatal("Expected testID to be returned")
	}

	// Verify artifact store was created
	store, err := forge.ReadArtifactStore(artifactStorePath)
	if err != nil {
		t.Fatalf("Failed to read artifact store: %v", err)
	}

	// Verify test environment was created
	if len(store.TestEnvironments) != 1 {
		t.Errorf("Expected 1 test environment, got %d", len(store.TestEnvironments))
	}

	// Find the created environment
	var env *forge.TestEnvironment
	for _, e := range store.TestEnvironments {
		env = e
		break
	}

	if env == nil {
		t.Fatal("No test environment found")
	}

	// Verify environment properties
	if env.Name != "integration" {
		t.Errorf("Expected name 'integration', got %s", env.Name)
	}

	if env.Status != forge.TestStatusCreated {
		t.Errorf("Expected status 'created', got %s", env.Status)
	}

	if !strings.HasPrefix(env.ID, "test-integration-") {
		t.Errorf("Expected ID to start with 'test-integration-', got %s", env.ID)
	}

	// Verify timestamps
	if env.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}

	if env.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should not be zero")
	}
}

func TestCmdDelete_Integration(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()
	artifactStorePath := filepath.Join(tmpDir, "artifact-store.yaml")

	// Create test environment
	now := time.Now().UTC()
	testEnv := &forge.TestEnvironment{
		ID:               "test-integration-20241103-xyz789",
		Name:             "integration",
		Status:           forge.TestStatusCreated,
		CreatedAt:        now,
		UpdatedAt:        now,
		ManagedResources: []string{},
	}

	store := forge.ArtifactStore{
		Version:     "1.0",
		LastUpdated: now,
		Artifacts:   []forge.Artifact{},
		TestEnvironments: map[string]*forge.TestEnvironment{
			testEnv.ID: testEnv,
		},
	}

	err := forge.WriteArtifactStore(artifactStorePath, store)
	if err != nil {
		t.Fatalf("Failed to write artifact store: %v", err)
	}

	// Create forge.yaml
	forgeYAML := `name: test-project
artifactStorePath: ` + artifactStorePath
	forgeYAMLPath := filepath.Join(tmpDir, "forge.yaml")
	err = os.WriteFile(forgeYAMLPath, []byte(forgeYAML), 0o644)
	if err != nil {
		t.Fatalf("Failed to write forge.yaml: %v", err)
	}

	// Change to temp directory
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	// Run cmdDelete
	err = cmdDelete(testEnv.ID)
	if err != nil {
		t.Fatalf("cmdDelete failed: %v", err)
	}

	// Verify environment was deleted
	store, err = forge.ReadArtifactStore(artifactStorePath)
	if err != nil {
		t.Fatalf("Failed to read artifact store after delete: %v", err)
	}

	if len(store.TestEnvironments) != 0 {
		t.Errorf("Expected 0 test environments after delete, got %d", len(store.TestEnvironments))
	}
}

func TestCmdDelete_NonexistentID(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()
	artifactStorePath := filepath.Join(tmpDir, "artifact-store.yaml")

	// Create empty artifact store
	store := forge.ArtifactStore{
		Version:          "1.0",
		LastUpdated:      time.Now().UTC(),
		Artifacts:        []forge.Artifact{},
		TestEnvironments: make(map[string]*forge.TestEnvironment),
	}

	err := forge.WriteArtifactStore(artifactStorePath, store)
	if err != nil {
		t.Fatalf("Failed to write artifact store: %v", err)
	}

	// Create forge.yaml
	forgeYAML := `name: test-project
artifactStorePath: ` + artifactStorePath
	forgeYAMLPath := filepath.Join(tmpDir, "forge.yaml")
	err = os.WriteFile(forgeYAMLPath, []byte(forgeYAML), 0o644)
	if err != nil {
		t.Fatalf("Failed to write forge.yaml: %v", err)
	}

	// Change to temp directory
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	// Run cmdDelete with nonexistent ID
	err = cmdDelete("nonexistent-id")
	if err == nil {
		t.Error("Expected error for nonexistent ID")
	}
}
