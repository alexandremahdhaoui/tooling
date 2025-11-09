//go:build unit

package forge

import (
	"os"
	"path/filepath"
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
  autoPushImages: true
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
	if !lcr.AutoPushImages {
		t.Errorf("AutoPushImages = %v, want true", lcr.AutoPushImages)
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
