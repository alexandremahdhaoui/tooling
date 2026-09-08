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
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestIntegration_BuildViaForge tests go-gen-openapi via forge build command.
func TestIntegration_BuildViaForge(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup test environment
	tmpDir := t.TempDir()

	// Create minimal OpenAPI spec
	openAPISpec := `openapi: 3.0.0
info:
  title: Test API
  version: v1
paths:
  /health:
    get:
      operationId: getHealth
      responses:
        '200':
          description: Health check
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/HealthStatus'
components:
  schemas:
    HealthStatus:
      type: object
      required:
        - ok
      properties:
        ok:
          type: boolean
`
	specPath := filepath.Join(tmpDir, "test-api.v1.yaml")
	if err := os.WriteFile(specPath, []byte(openAPISpec), 0o644); err != nil {
		t.Fatalf("Failed to write OpenAPI spec: %v", err)
	}

	// Create empty .envrc (required by forge default)
	if err := os.WriteFile(filepath.Join(tmpDir, ".envrc"), []byte(""), 0o644); err != nil {
		t.Fatalf("Failed to create .envrc: %v", err)
	}

	// Create forge.yaml
	forgeYaml := `name: test-openapi
artifactStorePath: .ignore.artifact-store.yaml

build:
  - name: test-api-v1
    src: ./test-api.v1.yaml
    dest: ./generated
    engine: forge://go-gen-openapi
    spec:
      sourceFile: ./test-api.v1.yaml
      destinationDir: ./generated
      client:
        enabled: true
        packageName: testclient
`
	forgeYamlPath := filepath.Join(tmpDir, "forge.yaml")
	if err := os.WriteFile(forgeYamlPath, []byte(forgeYaml), 0o644); err != nil {
		t.Fatalf("Failed to write forge.yaml: %v", err)
	}

	// Initialize a git repo (required for versioning)
	gitInit := exec.Command("git", "init")
	gitInit.Dir = tmpDir
	if err := gitInit.Run(); err != nil {
		t.Fatalf("Failed to init git: %v", err)
	}

	gitConfig1 := exec.Command("git", "config", "user.email", "test@example.com")
	gitConfig1.Dir = tmpDir
	_ = gitConfig1.Run()

	gitConfig2 := exec.Command("git", "config", "user.name", "Test User")
	gitConfig2.Dir = tmpDir
	_ = gitConfig2.Run()

	gitAdd := exec.Command("git", "add", ".")
	gitAdd.Dir = tmpDir
	if err := gitAdd.Run(); err != nil {
		t.Fatalf("Failed to add files: %v", err)
	}

	gitCommit := exec.Command("git", "commit", "-m", "Initial commit")
	gitCommit.Dir = tmpDir
	if err := gitCommit.Run(); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Get current working directory (cmd/go-gen-openapi)
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	// Build go-gen-openapi binary
	buildGoGenOpenAPI := exec.Command("go", "build", "-o", "go-gen-openapi", ".")
	buildGoGenOpenAPI.Dir = originalDir
	if err := buildGoGenOpenAPI.Run(); err != nil {
		t.Fatalf("Failed to build go-gen-openapi binary: %v", err)
	}
	defer os.Remove(filepath.Join(originalDir, "go-gen-openapi"))

	// Get forge repo root (go up from cmd/go-gen-openapi to repo root)
	forgeRepoRoot, err := filepath.Abs(filepath.Join(originalDir, "../.."))
	if err != nil {
		t.Fatalf("Failed to get forge repo root: %v", err)
	}

	// Build forge CLI binary (if not already built)
	forgeExe := filepath.Join(forgeRepoRoot, "build/bin/forge")
	if _, err := os.Stat(forgeExe); os.IsNotExist(err) {
		buildForge := exec.Command("go", "build", "-o", forgeExe, "./cmd/forge")
		buildForge.Dir = forgeRepoRoot
		if err := buildForge.Run(); err != nil {
			t.Fatalf("Failed to build forge CLI: %v", err)
		}
	}

	// Run forge build test-api-v1 in temp directory
	cmd := exec.Command(forgeExe, "build", "test-api-v1")
	cmd.Dir = tmpDir
	cmd.Env = append(os.Environ(), "FORGE_RUN_LOCAL_ENABLED=true", "FORGE_RUN_LOCAL_BASEDIR="+forgeRepoRoot, "FORGE_DEBUG=1")
	t.Logf("Running: %s build test-api-v1", forgeExe)
	t.Logf("In directory: %s", tmpDir)
	t.Logf("FORGE_RUN_LOCAL_BASEDIR=%s", forgeRepoRoot)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("forge build failed: %v\nOutput: %s", err, string(output))
	}

	t.Logf("forge build output:\n%s", string(output))

	// Verify generated code exists at expected location
	generatedFile := filepath.Join(tmpDir, "generated", "testclient", "zz_generated.oapi-codegen.go")
	if _, err := os.Stat(generatedFile); os.IsNotExist(err) {
		t.Fatalf("Generated code not created at %s", generatedFile)
	}

	// Verify file contains expected content (package name)
	content, err := os.ReadFile(generatedFile)
	if err != nil {
		t.Fatalf("Failed to read generated file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "package testclient") {
		t.Errorf("Generated code does not contain expected package declaration 'package testclient'")
	}

	// Verify it contains expected types/functions from the OpenAPI spec
	if !strings.Contains(contentStr, "HealthStatus") {
		t.Errorf("Generated code does not contain expected type 'HealthStatus'")
	}

	if !strings.Contains(contentStr, "GetHealth") {
		t.Errorf("Generated code does not contain expected operation 'GetHealth'")
	}

	t.Log("✅ Successfully generated OpenAPI code via forge build")
}
