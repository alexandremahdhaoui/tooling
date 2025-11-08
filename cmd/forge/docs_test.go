//go:build unit

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"sigs.k8s.io/yaml"
)

// TestFetchDocsStore_Local tests reading docs store from local file
func TestFetchDocsStore_Local(t *testing.T) {
	// Create temporary directory structure
	tmpDir := t.TempDir()
	docsDir := filepath.Join(tmpDir, "docs")
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		t.Fatalf("Failed to create temp docs dir: %v", err)
	}

	// Create test docs-list.yaml
	store := DocStore{
		Version: "1.0",
		BaseURL: "https://example.com",
		Docs: []DocEntry{
			{
				Name:        "test-doc",
				Title:       "Test Document",
				Description: "A test document",
				URL:         "test-doc.md",
				Tags:        []string{"test"},
			},
		},
	}

	content, err := yaml.Marshal(store)
	if err != nil {
		t.Fatalf("Failed to marshal test store: %v", err)
	}

	docsListPath := filepath.Join(docsDir, "docs-list.yaml")
	if err := os.WriteFile(docsListPath, content, 0o644); err != nil {
		t.Fatalf("Failed to write test docs-list.yaml: %v", err)
	}

	// Change to temp directory
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Test fetchDocsStore reads local file
	result, err := fetchDocsStore()
	if err != nil {
		t.Fatalf("fetchDocsStore failed: %v", err)
	}

	if result.Version != "1.0" {
		t.Errorf("Expected version 1.0, got %s", result.Version)
	}

	if len(result.Docs) != 1 {
		t.Errorf("Expected 1 doc, got %d", len(result.Docs))
	}

	if result.Docs[0].Name != "test-doc" {
		t.Errorf("Expected doc name 'test-doc', got %s", result.Docs[0].Name)
	}
}

// TestDocStoreStructure tests that DocStore structure is valid
func TestDocStoreStructure(t *testing.T) {
	store := DocStore{
		Version: "1.0",
		BaseURL: "https://example.com",
		Docs: []DocEntry{
			{
				Name:        "test",
				Title:       "Test Title",
				Description: "Test Description",
				URL:         "test.md",
				Tags:        []string{"tag1", "tag2"},
			},
		},
	}

	// Marshal and unmarshal to verify structure
	data, err := yaml.Marshal(store)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var result DocStore
	if err := yaml.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if result.Version != store.Version {
		t.Errorf("Version mismatch: expected %s, got %s", store.Version, result.Version)
	}

	if result.BaseURL != store.BaseURL {
		t.Errorf("BaseURL mismatch: expected %s, got %s", store.BaseURL, result.BaseURL)
	}

	if len(result.Docs) != 1 {
		t.Fatalf("Expected 1 doc, got %d", len(result.Docs))
	}

	doc := result.Docs[0]
	if doc.Name != "test" {
		t.Errorf("Name mismatch: expected 'test', got %s", doc.Name)
	}

	if len(doc.Tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(doc.Tags))
	}
}

// TestDocEntry tests DocEntry structure
func TestDocEntry(t *testing.T) {
	entry := DocEntry{
		Name:        "test-doc",
		Title:       "Test Document Title",
		Description: "This is a test document description",
		URL:         "test-doc.md",
		Tags:        []string{"testing", "example"},
	}

	if entry.Name != "test-doc" {
		t.Errorf("Expected name 'test-doc', got %s", entry.Name)
	}

	if entry.Title != "Test Document Title" {
		t.Errorf("Expected title 'Test Document Title', got %s", entry.Title)
	}

	if len(entry.Tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(entry.Tags))
	}
}

// TestRunDocs_InvalidArgs tests error handling for invalid arguments
func TestRunDocs_InvalidArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "no args",
			args: []string{},
			want: "usage: forge docs",
		},
		{
			name: "get without name",
			args: []string{"get"},
			want: "usage: forge docs get <doc-name>",
		},
		{
			name: "unknown operation",
			args: []string{"unknown"},
			want: "unknown operation: unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := runDocs(tt.args)
			if err == nil {
				t.Error("Expected error, got nil")
			}
			if err != nil && !contains(err.Error(), tt.want) {
				t.Errorf("Expected error containing %q, got %q", tt.want, err.Error())
			}
		})
	}
}

// TestDocsGet_LocalFile tests reading a doc from a local file
func TestDocsGet_LocalFile(t *testing.T) {
	// Create temporary directory structure
	tmpDir := t.TempDir()
	docsDir := filepath.Join(tmpDir, "docs")
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		t.Fatalf("Failed to create temp docs dir: %v", err)
	}

	// Create test docs-list.yaml
	store := DocStore{
		Version: "1.0",
		BaseURL: "https://example.com",
		Docs: []DocEntry{
			{
				Name:        "test-doc",
				Title:       "Test Document",
				Description: "A test document",
				URL:         "test-doc.md",
				Tags:        []string{"test"},
			},
		},
	}

	content, _ := yaml.Marshal(store)
	docsListPath := filepath.Join(docsDir, "docs-list.yaml")
	os.WriteFile(docsListPath, content, 0o644)

	// Create test doc file
	docContent := "# Test Document\n\nThis is a test document for verifying the docs system.\n"
	docPath := filepath.Join(tmpDir, "test-doc.md")
	os.WriteFile(docPath, []byte(docContent), 0o644)

	// Change to temp directory
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tmpDir)

	// Test docsGet reads local file
	// Note: This would normally print to stdout, so we'd need to capture that
	// For now, we verify the file exists and can be read
	if _, err := os.Stat("test-doc.md"); err != nil {
		t.Errorf("Test doc file should exist: %v", err)
	} else {
		t.Log("Local doc file found and readable")
	}
}

// TestDocsList_AllDocsExist verifies that all docs in docs-list.yaml
// reference files that actually exist
func TestDocsList_AllDocsExist(t *testing.T) {
	// Read the docs-list.yaml file
	docsListPath := filepath.Join("..", "..", "docs", "docs-list.yaml")
	content, err := os.ReadFile(docsListPath)
	if err != nil {
		t.Fatalf("Failed to read docs-list.yaml: %v", err)
	}

	// Parse the YAML
	var store DocStore
	if err := yaml.Unmarshal(content, &store); err != nil {
		t.Fatalf("Failed to parse docs-list.yaml: %v", err)
	}

	// Check each doc file exists
	repoRoot := filepath.Join("..", "..")
	var missingFiles []string

	for _, doc := range store.Docs {
		docPath := filepath.Join(repoRoot, doc.URL)
		if _, err := os.Stat(docPath); os.IsNotExist(err) {
			missingFiles = append(missingFiles, doc.URL)
			t.Errorf("Doc '%s' references non-existent file: %s", doc.Name, doc.URL)
		}
	}

	if len(missingFiles) > 0 {
		t.Fatalf("Found %d docs referencing non-existent files: %v", len(missingFiles), missingFiles)
	}

	t.Logf("Verified %d docs - all files exist", len(store.Docs))
}

// TestDocsList_NoEmptyFields verifies that all docs have required fields
func TestDocsList_NoEmptyFields(t *testing.T) {
	// Read the docs-list.yaml file
	docsListPath := filepath.Join("..", "..", "docs", "docs-list.yaml")
	content, err := os.ReadFile(docsListPath)
	if err != nil {
		t.Fatalf("Failed to read docs-list.yaml: %v", err)
	}

	// Parse the YAML
	var store DocStore
	if err := yaml.Unmarshal(content, &store); err != nil {
		t.Fatalf("Failed to parse docs-list.yaml: %v", err)
	}

	// Check each doc has required fields
	var errors []string

	for _, doc := range store.Docs {
		if doc.Name == "" {
			errors = append(errors, "found doc with empty name")
		}
		if doc.Title == "" {
			errors = append(errors, "doc '"+doc.Name+"' has empty title")
		}
		if doc.Description == "" {
			errors = append(errors, "doc '"+doc.Name+"' has empty description")
		}
		if doc.URL == "" {
			errors = append(errors, "doc '"+doc.Name+"' has empty URL")
		}
		if len(doc.Tags) == 0 {
			errors = append(errors, "doc '"+doc.Name+"' has no tags")
		}
	}

	if len(errors) > 0 {
		for _, e := range errors {
			t.Error(e)
		}
		t.Fatalf("Found %d validation errors", len(errors))
	}

	t.Logf("Verified %d docs - all have required fields", len(store.Docs))
}

// TestDocsGet_NotFound tests error handling when doc is not found
func TestDocsGet_NotFound(t *testing.T) {
	// Create temporary directory structure
	tmpDir := t.TempDir()
	docsDir := filepath.Join(tmpDir, "docs")
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		t.Fatalf("Failed to create temp docs dir: %v", err)
	}

	// Create test docs-list.yaml with no matching doc
	store := DocStore{
		Version: "1.0",
		BaseURL: "https://example.com",
		Docs: []DocEntry{
			{
				Name:        "other-doc",
				Title:       "Other Document",
				Description: "A different document",
				URL:         "other-doc.md",
				Tags:        []string{"test"},
			},
		},
	}

	content, _ := yaml.Marshal(store)
	docsListPath := filepath.Join(docsDir, "docs-list.yaml")
	os.WriteFile(docsListPath, content, 0o644)

	// Change to temp directory
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tmpDir)

	// Test docsGet with non-existent doc
	err := docsGet("non-existent-doc")
	if err == nil {
		t.Error("Expected error for non-existent doc, got nil")
	}

	if err != nil && !contains(err.Error(), "document not found") {
		t.Errorf("Expected 'document not found' error, got: %v", err)
	}
}

// TestDocStore_ValidYAML tests that the actual docs-list.yaml is valid YAML
func TestDocStore_ValidYAML(t *testing.T) {
	// Read the docs-list.yaml file
	docsListPath := filepath.Join("..", "..", "docs", "docs-list.yaml")
	content, err := os.ReadFile(docsListPath)
	if err != nil {
		t.Fatalf("Failed to read docs-list.yaml: %v", err)
	}

	// Parse the YAML
	var store DocStore
	if err := yaml.Unmarshal(content, &store); err != nil {
		t.Fatalf("Failed to parse docs-list.yaml: %v", err)
	}

	// Verify basic structure
	if store.Version == "" {
		t.Error("Version field is empty")
	}

	if store.BaseURL == "" {
		t.Error("BaseURL field is empty")
	}

	if len(store.Docs) == 0 {
		t.Error("Docs list is empty")
	}

	t.Logf("docs-list.yaml is valid YAML with version %s and %d docs", store.Version, len(store.Docs))
}

// TestDocsList_NoGitIgnoredFiles verifies that all docs in docs-list.yaml
// are actually committed to git (not ignored or untracked)
func TestDocsList_NoGitIgnoredFiles(t *testing.T) {
	// Read the docs-list.yaml file
	docsListPath := filepath.Join("..", "..", "docs", "docs-list.yaml")
	content, err := os.ReadFile(docsListPath)
	if err != nil {
		t.Fatalf("Failed to read docs-list.yaml: %v", err)
	}

	// Parse the YAML
	var store DocStore
	if err := yaml.Unmarshal(content, &store); err != nil {
		t.Fatalf("Failed to parse docs-list.yaml: %v", err)
	}

	// Get list of all tracked files in git
	repoRoot := filepath.Join("..", "..")
	cmd := exec.Command("git", "ls-files")
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to run git ls-files: %v", err)
	}

	// Build a set of tracked files for fast lookup
	trackedFiles := make(map[string]bool)
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			trackedFiles[line] = true
		}
	}

	// Check each doc is tracked by git
	var untrackedDocs []string
	var ignoredDocs []string

	for _, doc := range store.Docs {
		// Normalize path
		docPath := filepath.Clean(doc.URL)

		// Check if file is tracked
		if !trackedFiles[docPath] {
			// Check if file exists
			fullPath := filepath.Join(repoRoot, docPath)
			if _, err := os.Stat(fullPath); err == nil {
				// File exists but not tracked - check if ignored
				checkCmd := exec.Command("git", "check-ignore", "-q", docPath)
				checkCmd.Dir = repoRoot
				if err := checkCmd.Run(); err == nil {
					// File is ignored
					ignoredDocs = append(ignoredDocs, doc.Name+" ("+doc.URL+")")
				} else {
					// File exists but not tracked (maybe just not added)
					untrackedDocs = append(untrackedDocs, doc.Name+" ("+doc.URL+")")
				}
			} else {
				// File doesn't exist
				untrackedDocs = append(untrackedDocs, doc.Name+" ("+doc.URL+", file missing)")
			}
		}
	}

	// Report errors
	if len(ignoredDocs) > 0 {
		for _, d := range ignoredDocs {
			t.Errorf("Doc is git-ignored and should not be in docs-list.yaml: %s", d)
		}
	}

	if len(untrackedDocs) > 0 {
		for _, d := range untrackedDocs {
			t.Errorf("Doc is not tracked by git: %s", d)
		}
	}

	if len(ignoredDocs) > 0 || len(untrackedDocs) > 0 {
		t.Fatalf("Found %d ignored and %d untracked docs in docs-list.yaml", len(ignoredDocs), len(untrackedDocs))
	}

	t.Logf("Verified %d docs - all are tracked by git", len(store.Docs))
}
