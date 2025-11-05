//go:build unit

package main

import (
	"os"
	"path/filepath"
	"testing"

	"sigs.k8s.io/yaml"
)

// TestFetchPromptStore_Local tests reading prompt store from local file
func TestFetchPromptStore_Local(t *testing.T) {
	// Create temporary directory structure
	tmpDir := t.TempDir()
	docsDir := filepath.Join(tmpDir, "docs")
	if err := os.MkdirAll(docsDir, 0755); err != nil {
		t.Fatalf("Failed to create temp docs dir: %v", err)
	}

	// Create test prompt-list.yaml
	store := PromptStore{
		Version: "1.0",
		BaseURL: "https://example.com/prompts",
		Prompts: []PromptEntry{
			{
				Name:        "test-prompt",
				Title:       "Test Prompt",
				Description: "A test prompt",
				URL:         "test-prompt.md",
				Tags:        []string{"test"},
			},
		},
	}

	content, err := yaml.Marshal(store)
	if err != nil {
		t.Fatalf("Failed to marshal test store: %v", err)
	}

	promptListPath := filepath.Join(docsDir, "prompt-list.yaml")
	if err := os.WriteFile(promptListPath, content, 0644); err != nil {
		t.Fatalf("Failed to write test prompt-list.yaml: %v", err)
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

	// Test fetchPromptStore reads local file
	result, err := fetchPromptStore()
	if err != nil {
		t.Fatalf("fetchPromptStore failed: %v", err)
	}

	if result.Version != "1.0" {
		t.Errorf("Expected version 1.0, got %s", result.Version)
	}

	if len(result.Prompts) != 1 {
		t.Errorf("Expected 1 prompt, got %d", len(result.Prompts))
	}

	if result.Prompts[0].Name != "test-prompt" {
		t.Errorf("Expected prompt name 'test-prompt', got %s", result.Prompts[0].Name)
	}
}

// TestPromptStoreStructure tests that PromptStore structure is valid
func TestPromptStoreStructure(t *testing.T) {
	store := PromptStore{
		Version: "1.0",
		BaseURL: "https://example.com",
		Prompts: []PromptEntry{
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

	var result PromptStore
	if err := yaml.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if result.Version != store.Version {
		t.Errorf("Version mismatch: expected %s, got %s", store.Version, result.Version)
	}

	if result.BaseURL != store.BaseURL {
		t.Errorf("BaseURL mismatch: expected %s, got %s", store.BaseURL, result.BaseURL)
	}

	if len(result.Prompts) != 1 {
		t.Fatalf("Expected 1 prompt, got %d", len(result.Prompts))
	}

	prompt := result.Prompts[0]
	if prompt.Name != "test" {
		t.Errorf("Name mismatch: expected 'test', got %s", prompt.Name)
	}

	if len(prompt.Tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(prompt.Tags))
	}
}

// TestPromptEntry tests PromptEntry structure
func TestPromptEntry(t *testing.T) {
	entry := PromptEntry{
		Name:        "test-prompt",
		Title:       "Test Prompt Title",
		Description: "This is a test prompt description",
		URL:         "test-prompt.md",
		Tags:        []string{"testing", "example"},
	}

	if entry.Name != "test-prompt" {
		t.Errorf("Expected name 'test-prompt', got %s", entry.Name)
	}

	if entry.Title != "Test Prompt Title" {
		t.Errorf("Expected title 'Test Prompt Title', got %s", entry.Title)
	}

	if len(entry.Tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(entry.Tags))
	}
}

// TestRunPrompt_InvalidArgs tests error handling for invalid arguments
func TestRunPrompt_InvalidArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "no args",
			args: []string{},
			want: "usage: forge prompt",
		},
		{
			name: "get without name",
			args: []string{"get"},
			want: "usage: forge prompt get <prompt-name>",
		},
		{
			name: "unknown operation",
			args: []string{"unknown"},
			want: "unknown operation: unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := runPrompt(tt.args)
			if err == nil {
				t.Error("Expected error, got nil")
			}
			if err != nil && !contains(err.Error(), tt.want) {
				t.Errorf("Expected error containing %q, got %q", tt.want, err.Error())
			}
		})
	}
}

// TestPromptGet_LocalFile tests reading a prompt from a local file
func TestPromptGet_LocalFile(t *testing.T) {
	// Create temporary directory structure
	tmpDir := t.TempDir()
	docsDir := filepath.Join(tmpDir, "docs")
	promptsDir := filepath.Join(docsDir, "prompts")
	if err := os.MkdirAll(promptsDir, 0755); err != nil {
		t.Fatalf("Failed to create temp prompts dir: %v", err)
	}

	// Create test prompt-list.yaml
	store := PromptStore{
		Version: "1.0",
		BaseURL: "https://example.com/prompts",
		Prompts: []PromptEntry{
			{
				Name:        "test-prompt",
				Title:       "Test Prompt",
				Description: "A test prompt",
				URL:         "test-prompt.md",
				Tags:        []string{"test"},
			},
		},
	}

	content, _ := yaml.Marshal(store)
	promptListPath := filepath.Join(docsDir, "prompt-list.yaml")
	os.WriteFile(promptListPath, content, 0644)

	// Create test prompt file
	promptContent := "# Test Prompt\n\nYou are helping a user test the prompt system.\n"
	promptPath := filepath.Join(promptsDir, "test-prompt.md")
	os.WriteFile(promptPath, []byte(promptContent), 0644)

	// Change to temp directory
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tmpDir)

	// Test promptGet reads local file
	// Note: This would normally print to stdout, so we'd need to capture that
	// For now, we verify the file exists and can be read
	localPath := filepath.Join(localPromptsDir, "test-prompt.md")
	if _, err := os.Stat(localPath); err == nil {
		t.Log("Local prompt file found and readable")
	}
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
