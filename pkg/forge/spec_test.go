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

package forge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestReadSpec_LocalContainerRegistryNamespaceDefault tests that the namespace
// defaults to "testenv-lcr" when not specified in forge.yaml
func TestReadSpec_LocalContainerRegistryNamespaceDefault(t *testing.T) {
	tests := []struct {
		name              string
		yamlContent       string
		expectedNamespace string
		description       string
	}{
		{
			name: "missing_entire_section",
			yamlContent: `
name: test-project
artifactStorePath: .forge/artifact-store.yaml
`,
			expectedNamespace: "testenv-lcr",
			description:       "Should default to testenv-lcr when entire localContainerRegistry section is missing",
		},
		{
			name: "section_exists_namespace_missing",
			yamlContent: `
name: test-project
artifactStorePath: .forge/artifact-store.yaml
localContainerRegistry:
  enabled: true
  autoPushImages: true
`,
			expectedNamespace: "testenv-lcr",
			description:       "Should default to testenv-lcr when section exists but namespace field is missing",
		},
		{
			name: "explicit_namespace_preserved",
			yamlContent: `
name: test-project
artifactStorePath: .forge/artifact-store.yaml
localContainerRegistry:
  enabled: true
  namespace: custom-namespace
`,
			expectedNamespace: "custom-namespace",
			description:       "Should preserve explicit namespace value",
		},
		{
			name: "empty_namespace_gets_default",
			yamlContent: `
name: test-project
artifactStorePath: .forge/artifact-store.yaml
localContainerRegistry:
  enabled: true
  namespace: ""
`,
			expectedNamespace: "testenv-lcr",
			description:       "Should apply default when namespace is explicitly set to empty string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory
			tmpDir := t.TempDir()
			yamlPath := filepath.Join(tmpDir, "forge.yaml")

			// Write test YAML
			if err := os.WriteFile(yamlPath, []byte(tt.yamlContent), 0o644); err != nil {
				t.Fatalf("Failed to write test YAML: %v", err)
			}

			// Change to temp directory
			originalDir, err := os.Getwd()
			if err != nil {
				t.Fatalf("Failed to get working directory: %v", err)
			}
			defer func() {
				if err := os.Chdir(originalDir); err != nil {
					t.Errorf("Failed to restore working directory: %v", err)
				}
			}()

			if err := os.Chdir(tmpDir); err != nil {
				t.Fatalf("Failed to change to temp directory: %v", err)
			}

			// Read spec
			spec, err := ReadSpec()
			if err != nil {
				t.Fatalf("ReadSpec() error = %v, want nil", err)
			}

			// Verify namespace
			if spec.LocalContainerRegistry.Namespace != tt.expectedNamespace {
				t.Errorf("%s: namespace = %q, want %q",
					tt.description,
					spec.LocalContainerRegistry.Namespace,
					tt.expectedNamespace,
				)
			}
		})
	}
}

// TestReadSpec_LocalContainerRegistryOtherFieldsPreserved tests that setting
// the namespace default doesn't affect other LocalContainerRegistry fields
func TestReadSpec_LocalContainerRegistryOtherFieldsPreserved(t *testing.T) {
	yamlContent := `
name: test-project
artifactStorePath: .forge/artifact-store.yaml
localContainerRegistry:
  enabled: true
  credentialPath: /custom/path/credentials.yaml
  caCrtPath: /custom/path/ca.crt
  imagePullSecretName: my-secret
  imagePullSecretNamespaces:
    - namespace1
    - namespace2
`

	// Create temporary directory and YAML file
	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "forge.yaml")
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("Failed to write test YAML: %v", err)
	}

	// Change to temp directory
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Errorf("Failed to restore working directory: %v", err)
		}
	}()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Read spec
	spec, err := ReadSpec()
	if err != nil {
		t.Fatalf("ReadSpec() error = %v, want nil", err)
	}

	// Verify all fields are preserved
	lcr := spec.LocalContainerRegistry

	if lcr.Namespace != "testenv-lcr" {
		t.Errorf("Namespace = %q, want %q", lcr.Namespace, "testenv-lcr")
	}
	if !lcr.Enabled {
		t.Errorf("Enabled = %v, want true", lcr.Enabled)
	}
	if lcr.CredentialPath != "/custom/path/credentials.yaml" {
		t.Errorf("CredentialPath = %q, want %q", lcr.CredentialPath, "/custom/path/credentials.yaml")
	}
	if lcr.CaCrtPath != "/custom/path/ca.crt" {
		t.Errorf("CaCrtPath = %q, want %q", lcr.CaCrtPath, "/custom/path/ca.crt")
	}
	if lcr.ImagePullSecretName != "my-secret" {
		t.Errorf("ImagePullSecretName = %q, want %q", lcr.ImagePullSecretName, "my-secret")
	}
	if len(lcr.ImagePullSecretNamespaces) != 2 {
		t.Errorf("ImagePullSecretNamespaces length = %d, want 2", len(lcr.ImagePullSecretNamespaces))
	}
}

// TestReadSpec_EnvFileDefault tests that the envFile field
// defaults to ".envrc" when not specified in forge.yaml
func TestReadSpec_EnvFileDefault(t *testing.T) {
	tests := []struct {
		name            string
		yamlContent     string
		expectedEnvFile string
		description     string
	}{
		{
			name: "missing_envFile_field",
			yamlContent: `
name: test-project
artifactStorePath: .forge/artifact-store.yaml
`,
			expectedEnvFile: ".envrc",
			description:     "Should default to .envrc when envFile field is missing",
		},
		{
			name: "empty_envFile_value",
			yamlContent: `
name: test-project
artifactStorePath: .forge/artifact-store.yaml
envFile: ""
`,
			expectedEnvFile: ".envrc",
			description:     "Should default to .envrc when envFile is explicitly set to empty string",
		},
		{
			name: "explicit_envFile_value",
			yamlContent: `
name: test-project
artifactStorePath: .forge/artifact-store.yaml
envFile: "custom.env"
`,
			expectedEnvFile: "custom.env",
			description:     "Should preserve explicit envFile value",
		},
		{
			name: "envrc_explicit",
			yamlContent: `
name: test-project
artifactStorePath: .forge/artifact-store.yaml
envFile: ".envrc"
`,
			expectedEnvFile: ".envrc",
			description:     "Should preserve explicit .envrc value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory
			tmpDir := t.TempDir()
			yamlPath := filepath.Join(tmpDir, "forge.yaml")

			// Write test YAML
			if err := os.WriteFile(yamlPath, []byte(tt.yamlContent), 0o644); err != nil {
				t.Fatalf("Failed to write test YAML: %v", err)
			}

			// Change to temp directory
			originalDir, err := os.Getwd()
			if err != nil {
				t.Fatalf("Failed to get working directory: %v", err)
			}
			defer func() {
				if err := os.Chdir(originalDir); err != nil {
					t.Errorf("Failed to restore working directory: %v", err)
				}
			}()

			if err := os.Chdir(tmpDir); err != nil {
				t.Fatalf("Failed to change to temp directory: %v", err)
			}

			// Read spec
			spec, err := ReadSpec()
			if err != nil {
				t.Fatalf("ReadSpec() error = %v, want nil", err)
			}

			// Verify envFile
			if spec.EnvFile != tt.expectedEnvFile {
				t.Errorf("%s: envFile = %q, want %q",
					tt.description,
					spec.EnvFile,
					tt.expectedEnvFile,
				)
			}
		})
	}
}

// TestReadSpec_GenerateOpenAPIDeprecatedError tests that using the deprecated
// generateOpenAPI field returns a clear error message
func TestReadSpec_GenerateOpenAPIDeprecatedError(t *testing.T) {
	yamlContent := `
name: test-project
artifactStorePath: .forge/artifact-store.yaml
generateOpenAPI:
  defaults:
    sourceDir: ./api
    destinationDir: ./pkg/generated
  specs:
    - name: example-api
      versions:
        - v1
`

	// Create temporary directory and YAML file
	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "forge.yaml")
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("Failed to write test YAML: %v", err)
	}

	// Read spec from path - should return error
	spec, err := ReadSpecFromPath(yamlPath)

	// Verify error is returned
	if err == nil {
		t.Fatalf("ReadSpecFromPath() error = nil, want error about deprecated generateOpenAPI")
	}

	// Verify error message guides user to migration doc
	expectedErrorSubstring := "generateOpenAPI configuration is no longer supported"
	if !containsSubstring(err.Error(), expectedErrorSubstring) {
		t.Errorf("Error message = %q, want to contain %q", err.Error(), expectedErrorSubstring)
	}

	expectedMigrationDoc := "docs/migration-go-gen-openapi.md"
	if !containsSubstring(err.Error(), expectedMigrationDoc) {
		t.Errorf("Error message = %q, want to contain %q", err.Error(), expectedMigrationDoc)
	}

	// Verify spec is empty (zero value)
	if spec.Name != "" {
		t.Errorf("Expected zero-value spec when error occurs, got spec with name %q", spec.Name)
	}
}

// containsSubstring checks if a string contains a substring
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && stringContains(s, substr))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// A stage's needs name build entries, and an entry has one owner: the
// stage that needs it builds it, a bare build leaves it alone.
func TestAStageNeedsBuildEntriesAndOwnsThem(t *testing.T) {
	spec := Spec{
		Name: "x", ArtifactStorePath: ".forge/store.yaml", EnvFile: ".envrc",
		Build: []BuildSpec{{Name: "fixture", Src: ".", Engine: "forge://container-build"}},
		Test: []TestSpec{
			{Name: "integration", Runner: "forge://go-test", Needs: []string{"fixture"}},
		},
	}

	if err := spec.Validate(); err != nil {
		t.Fatalf("a stage may need a declared entry: %v", err)
	}

	if got := spec.OwnedBuilds()["fixture"]; got != "integration" {
		t.Fatalf("the needing stage owns the entry, got owner %q", got)
	}

	spec.Test[0].Needs = []string{"nothing-declares-this"}
	if err := spec.Validate(); err == nil || !strings.Contains(err.Error(), "nothing-declares-this") {
		t.Fatalf("an unknown need must be refused by name, got %v", err)
	}

	spec.Test[0].Needs = []string{"fixture"}
	spec.Test = append(spec.Test, TestSpec{Name: "e2e", Runner: "forge://go-test", Needs: []string{"fixture"}})
	err := spec.Validate()
	if err == nil || !strings.Contains(err.Error(), "integration") || !strings.Contains(err.Error(), "e2e") {
		t.Fatalf("two owners must be refused naming both stages, got %v", err)
	}
}

// Frozen is declared once, repo-wide, and absent means true: a build is a
// real build unless the repo says otherwise.
func TestFrozenIsTrueUnlessTheRepoSaysOtherwise(t *testing.T) {
	var spec Spec
	if !spec.FrozenBuild() {
		t.Fatal("absent must read as frozen")
	}

	off := false
	spec.Frozen = &off

	if spec.FrozenBuild() {
		t.Fatal("frozen: false must read as not frozen")
	}
}
