//go:build unit

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

	"github.com/alexandremahdhaoui/forge/pkg/forge"
)

func TestShouldRebuild_ForceFlag(t *testing.T) {
	store := forge.ArtifactStore{
		Version:     "1.0",
		LastUpdated: time.Now(),
		Artifacts:   []forge.Artifact{},
	}

	needsRebuild, reason, err := shouldRebuild("test-artifact", []string{hostPlatform()}, store, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !needsRebuild {
		t.Error("expected rebuild when force flag is set")
	}
	if reason != "force flag set" {
		t.Errorf("expected reason 'force flag set', got %q", reason)
	}
}

func TestShouldRebuild_NoPreviousBuild(t *testing.T) {
	store := forge.ArtifactStore{
		Version:     "1.0",
		LastUpdated: time.Now(),
		Artifacts:   []forge.Artifact{},
	}

	needsRebuild, reason, err := shouldRebuild("test-artifact", []string{hostPlatform()}, store, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !needsRebuild {
		t.Error("expected rebuild when no previous build exists")
	}
	if !strings.HasPrefix(reason, "no previous build") {
		t.Errorf("expected reason 'no previous build', got %q", reason)
	}
}

func TestShouldRebuild_ArtifactFileMissing(t *testing.T) {
	tmpDir := t.TempDir()
	missingFile := filepath.Join(tmpDir, "missing-artifact")

	store := forge.ArtifactStore{
		Version:     "1.0",
		LastUpdated: time.Now(),
		Artifacts: []forge.Artifact{
			{
				Name:                     "test-artifact",
				Type:                     "binary",
				Location:                 missingFile,
				Timestamp:                time.Now().UTC().Format(time.RFC3339),
				Version:                  "abc123",
				Dependencies:             []forge.ArtifactDependency{},
				DependencyDetectorEngine: "forge://test-detector",
			},
		},
	}

	needsRebuild, reason, err := shouldRebuild("test-artifact", []string{hostPlatform()}, store, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !needsRebuild {
		t.Error("expected rebuild when artifact file is missing")
	}
	if !strings.HasPrefix(reason, "artifact file missing") {
		t.Errorf("expected reason 'artifact file missing', got %q", reason)
	}
}

func TestShouldRebuild_DependenciesNotTracked(t *testing.T) {
	tmpDir := t.TempDir()
	artifactFile := filepath.Join(tmpDir, "test-artifact")

	// Create artifact file
	if err := os.WriteFile(artifactFile, []byte("test"), 0o644); err != nil {
		t.Fatalf("failed to create artifact file: %v", err)
	}

	store := forge.ArtifactStore{
		Version:     "1.0",
		LastUpdated: time.Now(),
		Artifacts: []forge.Artifact{
			{
				Name:                     "test-artifact",
				Type:                     "binary",
				Location:                 artifactFile,
				Timestamp:                time.Now().UTC().Format(time.RFC3339),
				Version:                  "abc123",
				Dependencies:             nil, // No dependencies tracked
				DependencyDetectorEngine: "",
			},
		},
	}

	needsRebuild, reason, err := shouldRebuild("test-artifact", []string{hostPlatform()}, store, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !needsRebuild {
		t.Error("expected rebuild when dependencies not tracked")
	}
	if reason != "dependencies not tracked" {
		t.Errorf("expected reason 'dependencies not tracked', got %q", reason)
	}
}

func TestShouldRebuild_DependencyDetectorNotConfigured(t *testing.T) {
	tmpDir := t.TempDir()
	artifactFile := filepath.Join(tmpDir, "test-artifact")
	depFile := filepath.Join(tmpDir, "dep.go")

	// Create files
	if err := os.WriteFile(artifactFile, []byte("test"), 0o644); err != nil {
		t.Fatalf("failed to create artifact file: %v", err)
	}
	if err := os.WriteFile(depFile, []byte("package main"), 0o644); err != nil {
		t.Fatalf("failed to create dep file: %v", err)
	}

	depStat, _ := os.Stat(depFile)
	depTimestamp := depStat.ModTime().UTC().Format(time.RFC3339)

	store := forge.ArtifactStore{
		Version:     "1.0",
		LastUpdated: time.Now(),
		Artifacts: []forge.Artifact{
			{
				Name:      "test-artifact",
				Type:      "binary",
				Location:  artifactFile,
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				Version:   "abc123",
				Dependencies: []forge.ArtifactDependency{
					{
						Type:      forge.DependencyTypeFile,
						FilePath:  depFile,
						Timestamp: depTimestamp,
					},
				},
				DependencyDetectorEngine: "", // Not configured
			},
		},
	}

	needsRebuild, reason, err := shouldRebuild("test-artifact", []string{hostPlatform()}, store, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !needsRebuild {
		t.Error("expected rebuild when dependency detector not configured")
	}
	if reason != "dependency detector not configured" {
		t.Errorf("expected reason 'dependency detector not configured', got %q", reason)
	}
}

func TestShouldRebuild_DependencyFileMissing(t *testing.T) {
	tmpDir := t.TempDir()
	artifactFile := filepath.Join(tmpDir, "test-artifact")
	missingDep := filepath.Join(tmpDir, "missing-dep.go")

	// Create only artifact file
	if err := os.WriteFile(artifactFile, []byte("test"), 0o644); err != nil {
		t.Fatalf("failed to create artifact file: %v", err)
	}

	store := forge.ArtifactStore{
		Version:     "1.0",
		LastUpdated: time.Now(),
		Artifacts: []forge.Artifact{
			{
				Name:      "test-artifact",
				Type:      "binary",
				Location:  artifactFile,
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				Version:   "abc123",
				Dependencies: []forge.ArtifactDependency{
					{
						Type:      forge.DependencyTypeFile,
						FilePath:  missingDep,
						Timestamp: time.Now().UTC().Format(time.RFC3339),
					},
				},
				DependencyDetectorEngine: "forge://test-detector",
			},
		},
	}

	needsRebuild, reason, err := shouldRebuild("test-artifact", []string{hostPlatform()}, store, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !needsRebuild {
		t.Error("expected rebuild when dependency file is missing")
	}
	expectedReason := "dependency file " + missingDep + " missing"
	if reason != expectedReason {
		t.Errorf("expected reason %q, got %q", expectedReason, reason)
	}
}

func TestShouldRebuild_DependencyModified(t *testing.T) {
	tmpDir := t.TempDir()
	artifactFile := filepath.Join(tmpDir, "test-artifact")
	depFile := filepath.Join(tmpDir, "dep.go")

	// Create files
	if err := os.WriteFile(artifactFile, []byte("test"), 0o644); err != nil {
		t.Fatalf("failed to create artifact file: %v", err)
	}
	if err := os.WriteFile(depFile, []byte("package main"), 0o644); err != nil {
		t.Fatalf("failed to create dep file: %v", err)
	}

	// Get initial timestamp
	depStat, _ := os.Stat(depFile)
	oldTimestamp := depStat.ModTime().UTC().Add(-1 * time.Hour).Format(time.RFC3339)

	store := forge.ArtifactStore{
		Version:     "1.0",
		LastUpdated: time.Now(),
		Artifacts: []forge.Artifact{
			{
				Name:      "test-artifact",
				Type:      "binary",
				Location:  artifactFile,
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				Version:   "abc123",
				Dependencies: []forge.ArtifactDependency{
					{
						Type:      forge.DependencyTypeFile,
						FilePath:  depFile,
						Timestamp: oldTimestamp, // Old timestamp - different from current
					},
				},
				DependencyDetectorEngine: "forge://test-detector",
			},
		},
	}

	needsRebuild, reason, err := shouldRebuild("test-artifact", []string{hostPlatform()}, store, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !needsRebuild {
		t.Error("expected rebuild when dependency is modified")
	}
	expectedReason := "dependency " + depFile + " modified"
	if reason != expectedReason {
		t.Errorf("expected reason %q, got %q", expectedReason, reason)
	}
}

func TestShouldRebuild_AllDependenciesUnchanged(t *testing.T) {
	tmpDir := t.TempDir()
	artifactFile := filepath.Join(tmpDir, "test-artifact")
	depFile1 := filepath.Join(tmpDir, "dep1.go")
	depFile2 := filepath.Join(tmpDir, "dep2.go")

	// Create files
	if err := os.WriteFile(artifactFile, []byte("test"), 0o644); err != nil {
		t.Fatalf("failed to create artifact file: %v", err)
	}
	if err := os.WriteFile(depFile1, []byte("package main"), 0o644); err != nil {
		t.Fatalf("failed to create dep1 file: %v", err)
	}
	if err := os.WriteFile(depFile2, []byte("package lib"), 0o644); err != nil {
		t.Fatalf("failed to create dep2 file: %v", err)
	}

	// Get timestamps
	dep1Stat, _ := os.Stat(depFile1)
	dep1Timestamp := dep1Stat.ModTime().UTC().Format(time.RFC3339)
	dep2Stat, _ := os.Stat(depFile2)
	dep2Timestamp := dep2Stat.ModTime().UTC().Format(time.RFC3339)

	store := forge.ArtifactStore{
		Version:     "1.0",
		LastUpdated: time.Now(),
		Artifacts: []forge.Artifact{
			{
				Name:      "test-artifact",
				Type:      "binary",
				Location:  artifactFile,
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				Version:   "abc123",
				Dependencies: []forge.ArtifactDependency{
					{
						Type:      forge.DependencyTypeFile,
						FilePath:  depFile1,
						Timestamp: dep1Timestamp,
					},
					{
						Type:      forge.DependencyTypeFile,
						FilePath:  depFile2,
						Timestamp: dep2Timestamp,
					},
					{
						Type:            forge.DependencyTypeExternalPackage,
						ExternalPackage: "github.com/foo/bar",
						Semver:          "v1.2.3",
					},
				},
				DependencyDetectorEngine: "forge://test-detector",
			},
		},
	}

	needsRebuild, reason, err := shouldRebuild("test-artifact", []string{hostPlatform()}, store, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if needsRebuild {
		t.Errorf("expected no rebuild when all dependencies unchanged, got reason: %q", reason)
	}
	if reason != "" {
		t.Errorf("expected empty reason when no rebuild needed, got %q", reason)
	}
}

func TestShouldRebuild_ExternalPackagesWithoutGoMod(t *testing.T) {
	tmpDir := t.TempDir()
	artifactFile := filepath.Join(tmpDir, "test-artifact")
	depFile := filepath.Join(tmpDir, "main.go")

	// Create files
	if err := os.WriteFile(artifactFile, []byte("test"), 0o644); err != nil {
		t.Fatalf("failed to create artifact file: %v", err)
	}
	if err := os.WriteFile(depFile, []byte("package main"), 0o644); err != nil {
		t.Fatalf("failed to create dep file: %v", err)
	}

	// Get timestamp
	depStat, _ := os.Stat(depFile)
	depTimestamp := depStat.ModTime().UTC().Format(time.RFC3339)

	store := forge.ArtifactStore{
		Version:     "1.0",
		LastUpdated: time.Now(),
		Artifacts: []forge.Artifact{
			{
				Name:      "test-artifact",
				Type:      "binary",
				Location:  artifactFile,
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				Version:   "abc123",
				Dependencies: []forge.ArtifactDependency{
					{
						Type:      forge.DependencyTypeFile,
						FilePath:  depFile,
						Timestamp: depTimestamp,
					},
					{
						Type:            forge.DependencyTypeExternalPackage,
						ExternalPackage: "github.com/foo/bar",
						Semver:          "v1.2.3",
					},
				},
				DependencyDetectorEngine: "forge://test-detector",
			},
		},
	}

	// Capture stderr to check for warning
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	needsRebuild, _, err := shouldRebuild("test-artifact", []string{hostPlatform()}, store, false)

	// Restore stderr
	w.Close()
	os.Stderr = oldStderr

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if needsRebuild {
		t.Error("expected no rebuild even without go.mod tracked (dependencies unchanged)")
	}

	// Read warning from stderr
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	stderrOutput := string(buf[:n])

	if stderrOutput != "" {
		t.Logf("Warning message (expected): %s", stderrOutput)
	}
}
