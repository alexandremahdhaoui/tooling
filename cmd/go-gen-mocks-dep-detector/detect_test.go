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

// ----------------------------------------------------- FIND MOCKERY CONFIG ----------------------------------------- //

func TestFindMockeryConfig_EnvVar(t *testing.T) {
	// Create a temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "custom-config.yaml")
	if err := os.WriteFile(configPath, []byte("packages: {}"), 0o600); err != nil {
		t.Fatalf("failed to create test config: %v", err)
	}

	// Set env var
	t.Setenv("MOCKERY_CONFIG_PATH", configPath)

	// Call findMockeryConfig with a different directory
	result, err := findMockeryConfig(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify env var takes precedence
	if result != configPath {
		t.Errorf("expected %s, got %s", configPath, result)
	}
}

func TestFindMockeryConfig_DotMockeryYaml(t *testing.T) {
	// Use the testdata/valid directory which has .mockery.yaml
	testDir := filepath.Join("testdata", "valid")
	absTestDir, err := filepath.Abs(testDir)
	if err != nil {
		t.Fatalf("failed to get absolute path: %v", err)
	}

	// Clear env var to ensure it doesn't interfere
	t.Setenv("MOCKERY_CONFIG_PATH", "")

	result, err := findMockeryConfig(absTestDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the correct file is found
	expectedPath := filepath.Join(absTestDir, ".mockery.yaml")
	if result != expectedPath {
		t.Errorf("expected %s, got %s", expectedPath, result)
	}
}

func TestFindMockeryConfig_NotFound(t *testing.T) {
	// Create empty temp directory
	tmpDir := t.TempDir()

	// Clear env var
	t.Setenv("MOCKERY_CONFIG_PATH", "")

	_, err := findMockeryConfig(tmpDir)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Verify error message mentions "no mockery config found"
	if !strings.Contains(err.Error(), "no mockery config found") {
		t.Errorf("expected error to contain 'no mockery config found', got: %v", err)
	}
}

func TestFindMockeryConfig_OtherConfigNames(t *testing.T) {
	// Test other config name variants
	configNames := []string{".mockery.yml", "mockery.yaml", "mockery.yml"}

	for _, name := range configNames {
		t.Run(name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, name)
			if err := os.WriteFile(configPath, []byte("packages: {}"), 0o600); err != nil {
				t.Fatalf("failed to create test config: %v", err)
			}

			// Clear env var
			t.Setenv("MOCKERY_CONFIG_PATH", "")

			result, err := findMockeryConfig(tmpDir)
			if err != nil {
				t.Fatalf("unexpected error for %s: %v", name, err)
			}

			if result != configPath {
				t.Errorf("expected %s, got %s", configPath, result)
			}
		})
	}
}

// ----------------------------------------------------- PARSE MOCKERY CONFIG ---------------------------------------- //

func TestParseMockeryConfig_Valid(t *testing.T) {
	testDir := filepath.Join("testdata", "valid")
	configPath := filepath.Join(testDir, ".mockery.yaml")
	absConfigPath, err := filepath.Abs(configPath)
	if err != nil {
		t.Fatalf("failed to get absolute path: %v", err)
	}

	config, err := parseMockeryConfig(absConfigPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify package paths are extracted
	if len(config.Packages) != 1 {
		t.Errorf("expected 1 package, got %d", len(config.Packages))
	}

	expectedPkg := "github.com/test/project/pkg/store"
	if _, exists := config.Packages[expectedPkg]; !exists {
		t.Errorf("expected package %s not found in config", expectedPkg)
	}
}

func TestParseMockeryConfig_InvalidYAML(t *testing.T) {
	testDir := filepath.Join("testdata", "invalid")
	configPath := filepath.Join(testDir, "broken.yaml")
	absConfigPath, err := filepath.Abs(configPath)
	if err != nil {
		t.Fatalf("failed to get absolute path: %v", err)
	}

	_, err = parseMockeryConfig(absConfigPath)
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}

	// Verify error mentions parsing
	if !strings.Contains(err.Error(), "parse") && !strings.Contains(err.Error(), "yaml") {
		t.Errorf("expected error to mention parsing/yaml, got: %v", err)
	}
}

func TestParseMockeryConfig_NoPackages(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".mockery.yaml")

	// Write config with no packages
	content := "with-expecter: true\n"
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to create test config: %v", err)
	}

	config, err := parseMockeryConfig(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should return empty packages, not error
	if len(config.Packages) != 0 {
		t.Errorf("expected 0 packages, got %d", len(config.Packages))
	}
}

// ----------------------------------------------------- RESOLVE PACKAGE TO FILES ------------------------------------ //

func TestResolvePackageToFiles_LocalPackage(t *testing.T) {
	testDir := filepath.Join("testdata", "valid")
	absTestDir, err := filepath.Abs(testDir)
	if err != nil {
		t.Fatalf("failed to get absolute path: %v", err)
	}

	pkgPath := "github.com/test/project/pkg/store"
	files, err := resolvePackageToFiles(pkgPath, absTestDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify store.go is found
	if len(files) == 0 {
		t.Fatal("expected at least one file, got none")
	}

	found := false
	for _, f := range files {
		if strings.HasSuffix(f, "store.go") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected to find store.go in files: %v", files)
	}
}

func TestResolvePackageToFiles_ExternalPackage_ReturnsError(t *testing.T) {
	testDir := filepath.Join("testdata", "valid")
	absTestDir, err := filepath.Abs(testDir)
	if err != nil {
		t.Fatalf("failed to get absolute path: %v", err)
	}

	// External package (not under github.com/test/project)
	pkgPath := "github.com/external/package"
	_, err = resolvePackageToFiles(pkgPath, absTestDir)
	if err == nil {
		t.Fatal("expected error for external package, got nil")
	}

	// Verify error message mentions external/v1
	if !strings.Contains(err.Error(), "external") || !strings.Contains(err.Error(), "v1") {
		t.Errorf("expected error to mention 'external' and 'v1', got: %v", err)
	}
}

func TestResolvePackageToFiles_PackageNotFound(t *testing.T) {
	testDir := filepath.Join("testdata", "valid")
	absTestDir, err := filepath.Abs(testDir)
	if err != nil {
		t.Fatalf("failed to get absolute path: %v", err)
	}

	// Package under module but doesn't exist
	pkgPath := "github.com/test/project/nonexistent"
	_, err = resolvePackageToFiles(pkgPath, absTestDir)
	if err == nil {
		t.Fatal("expected error for nonexistent package, got nil")
	}
}

// ----------------------------------------------------- DETECT MOCK DEPENDENCIES ------------------------------------ //

func TestDetectMockDependencies_FullFlow(t *testing.T) {
	testDir := filepath.Join("testdata", "valid")
	absTestDir, err := filepath.Abs(testDir)
	if err != nil {
		t.Fatalf("failed to get absolute path: %v", err)
	}

	// Clear env var
	t.Setenv("MOCKERY_CONFIG_PATH", "")

	input := mcptypes.DetectMockDependenciesInput{
		RootDir: absTestDir,
	}

	output, err := DetectMockDependencies(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify we have dependencies
	if len(output.Dependencies) == 0 {
		t.Fatal("expected at least one dependency, got none")
	}

	// Check for .mockery.yaml
	foundConfig := false
	foundGoMod := false
	foundStoreGo := false

	for _, dep := range output.Dependencies {
		if !strings.HasPrefix(dep.Digest, forge.DigestPrefix) {
			t.Errorf("expected a content digest, got %q", dep.Digest)
		}
		if strings.HasSuffix(dep.Path, ".mockery.yaml") {
			foundConfig = true
		}
		if strings.HasSuffix(dep.Path, "go.mod") {
			foundGoMod = true
		}
		if strings.HasSuffix(dep.Path, "store.go") {
			foundStoreGo = true
		}
		// Verify timestamp is set
	}

	if !foundConfig {
		t.Error(".mockery.yaml not found in dependencies")
	}
	if !foundGoMod {
		t.Error("go.mod not found in dependencies")
	}
	if !foundStoreGo {
		t.Error("store.go not found in dependencies")
	}
}

func TestDetectMockDependencies_ExternalPackage_LogsWarningButSucceeds(t *testing.T) {
	// Create a temp directory with mockery config referencing external package
	tmpDir := t.TempDir()

	// Create go.mod
	goModContent := "module github.com/test/project\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0o600); err != nil {
		t.Fatalf("failed to create go.mod: %v", err)
	}

	// Create .mockery.yaml with external package
	mockeryContent := `packages:
  github.com/external/package:
    interfaces:
      SomeInterface:
`
	if err := os.WriteFile(filepath.Join(tmpDir, ".mockery.yaml"), []byte(mockeryContent), 0o600); err != nil {
		t.Fatalf("failed to create .mockery.yaml: %v", err)
	}

	// Clear env var
	t.Setenv("MOCKERY_CONFIG_PATH", "")

	input := mcptypes.DetectMockDependenciesInput{
		RootDir: tmpDir,
	}

	// Should succeed (not fail) even with external package
	output, err := DetectMockDependencies(input)
	if err != nil {
		t.Fatalf("expected success with warning, got error: %v", err)
	}

	// Should have at least .mockery.yaml and go.mod as dependencies
	if len(output.Dependencies) < 2 {
		t.Errorf("expected at least 2 dependencies (config and go.mod), got %d", len(output.Dependencies))
	}
}

func TestDetectMockDependencies_ConfigNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	// Clear env var
	t.Setenv("MOCKERY_CONFIG_PATH", "")

	input := mcptypes.DetectMockDependenciesInput{
		RootDir: tmpDir,
	}

	_, err := DetectMockDependencies(input)
	if err == nil {
		t.Fatal("expected error when config not found, got nil")
	}
}

// ----------------------------------------------------- LIST GO FILES ----------------------------------------------- //

func TestListGoFiles_ExcludesTestFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create regular .go file
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0o600); err != nil {
		t.Fatalf("failed to create main.go: %v", err)
	}

	// Create _test.go file
	if err := os.WriteFile(filepath.Join(tmpDir, "main_test.go"), []byte("package main"), 0o600); err != nil {
		t.Fatalf("failed to create main_test.go: %v", err)
	}

	files, err := listGoFiles(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should only find main.go, not main_test.go
	if len(files) != 1 {
		t.Errorf("expected 1 file, got %d: %v", len(files), files)
	}

	if !strings.HasSuffix(files[0], "main.go") {
		t.Errorf("expected main.go, got %s", files[0])
	}
}

func TestListGoFiles_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := listGoFiles(tmpDir)
	if err == nil {
		t.Fatal("expected error for empty directory, got nil")
	}

	if !strings.Contains(err.Error(), "no .go files") {
		t.Errorf("expected error about no .go files, got: %v", err)
	}
}
