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

	"github.com/alexandremahdhaoui/forge/pkg/forge"

	"github.com/alexandremahdhaoui/forge/pkg/mcptypes"
)

func TestDetectOpenAPIDependencies_SingleSpec(t *testing.T) {
	// Get absolute path to test fixture
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	specPath := filepath.Join(cwd, "testdata", "petstore.yaml")

	// Verify test fixture exists
	if _, err := os.Stat(specPath); err != nil {
		t.Fatalf("Test fixture not found: %v", err)
	}

	input := mcptypes.DetectOpenAPIDependenciesInput{
		SpecSources: []string{specPath},
		RootDir:     cwd,
		ResolveRefs: false,
	}

	output, err := DetectOpenAPIDependencies(input)
	if err != nil {
		t.Fatalf("DetectOpenAPIDependencies failed: %v", err)
	}

	// Should have exactly 1 dependency
	if len(output.Dependencies) != 1 {
		t.Errorf("Expected 1 dependency, got %d", len(output.Dependencies))
	}

	// Verify the dependency
	dep := output.Dependencies[0]
	if dep.Path != specPath {
		t.Errorf("Expected Path '%s', got '%s'", specPath, dep.Path)
	}
	if !strings.HasPrefix(dep.Digest, forge.DigestPrefix) {
		t.Errorf("Expected a content digest, got %q", dep.Digest)
	}
}

func TestDetectOpenAPIDependencies_MultipleSpecs(t *testing.T) {
	// Get absolute paths to test fixtures
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	petstorePath := filepath.Join(cwd, "testdata", "petstore.yaml")
	usersPath := filepath.Join(cwd, "testdata", "users.yaml")

	// Verify test fixtures exist
	for _, path := range []string{petstorePath, usersPath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("Test fixture not found: %v", err)
		}
	}

	input := mcptypes.DetectOpenAPIDependenciesInput{
		SpecSources: []string{petstorePath, usersPath},
		RootDir:     cwd,
		ResolveRefs: false,
	}

	output, err := DetectOpenAPIDependencies(input)
	if err != nil {
		t.Fatalf("DetectOpenAPIDependencies failed: %v", err)
	}

	// Should have exactly 2 dependencies
	if len(output.Dependencies) != 2 {
		t.Errorf("Expected 2 dependencies, got %d", len(output.Dependencies))
	}

	// Verify both dependencies are present
	foundPetstore := false
	foundUsers := false
	for _, dep := range output.Dependencies {
		if !strings.HasPrefix(dep.Digest, forge.DigestPrefix) {
			t.Errorf("Expected a content digest, got %q", dep.Digest)
		}
		if dep.Path == petstorePath {
			foundPetstore = true
		}
		if dep.Path == usersPath {
			foundUsers = true
		}
	}

	if !foundPetstore {
		t.Error("Expected petstore.yaml in dependencies")
	}
	if !foundUsers {
		t.Error("Expected users.yaml in dependencies")
	}
}

func TestDetectOpenAPIDependencies_MissingSpec(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	// Use a path that doesn't exist
	missingPath := filepath.Join(cwd, "testdata", "nonexistent.yaml")

	input := mcptypes.DetectOpenAPIDependenciesInput{
		SpecSources: []string{missingPath},
		RootDir:     cwd,
		ResolveRefs: false,
	}

	_, err = DetectOpenAPIDependencies(input)
	if err == nil {
		t.Fatal("Expected error for missing spec file, got nil")
	}

	// Verify error message contains the path
	if err.Error() == "" {
		t.Error("Expected error message to be non-empty")
	}
}

func TestDetectOpenAPIDependencies_EmptyInput(t *testing.T) {
	input := mcptypes.DetectOpenAPIDependenciesInput{
		SpecSources: []string{},
		RootDir:     "",
		ResolveRefs: false,
	}

	output, err := DetectOpenAPIDependencies(input)
	if err != nil {
		t.Fatalf("DetectOpenAPIDependencies failed for empty input: %v", err)
	}

	// Should have 0 dependencies
	if len(output.Dependencies) != 0 {
		t.Errorf("Expected 0 dependencies for empty input, got %d", len(output.Dependencies))
	}
}

func TestDetectOpenAPIDependencies_ResolveRefsWarning(t *testing.T) {
	// Get absolute path to test fixture
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	specPath := filepath.Join(cwd, "testdata", "petstore.yaml")

	input := mcptypes.DetectOpenAPIDependenciesInput{
		SpecSources: []string{specPath},
		RootDir:     cwd,
		ResolveRefs: true, // Set to true to trigger warning
	}

	// This should succeed but log a warning
	output, err := DetectOpenAPIDependencies(input)
	if err != nil {
		t.Fatalf("DetectOpenAPIDependencies failed: %v", err)
	}

	// Should still have 1 dependency even with ResolveRefs=true
	if len(output.Dependencies) != 1 {
		t.Errorf("Expected 1 dependency, got %d", len(output.Dependencies))
	}
}
