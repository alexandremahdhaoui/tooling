package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"sigs.k8s.io/yaml"
)

const (
	promptListURL     = "https://raw.githubusercontent.com/alexandremahdhaoui/forge/refs/heads/main/docs/prompt-list.yaml"
	localPromptList   = "docs/prompt-list.yaml"
	localPromptsDir   = "docs/prompts"
	httpTimeout       = 10 * time.Second
)

// PromptStore represents the prompt-list.yaml structure
type PromptStore struct {
	Version string        `yaml:"version"`
	BaseURL string        `yaml:"baseURL"`
	Prompts []PromptEntry `yaml:"prompts"`
}

// PromptEntry represents a single prompt in the list
type PromptEntry struct {
	Name        string   `yaml:"name"`
	Title       string   `yaml:"title"`
	Description string   `yaml:"description"`
	URL         string   `yaml:"url"`
	Tags        []string `yaml:"tags"`
}

// runPrompt handles the "forge prompt" command
func runPrompt(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: forge prompt <list|get> [prompt-name]")
	}

	operation := args[0]

	switch operation {
	case "list":
		return promptList()
	case "get":
		if len(args) < 2 {
			return fmt.Errorf("usage: forge prompt get <prompt-name>")
		}
		return promptGet(args[1])
	default:
		return fmt.Errorf("unknown operation: %s (valid: list, get)", operation)
	}
}

// promptList lists all available prompts
func promptList() error {
	// Fetch prompt store
	store, err := fetchPromptStore()
	if err != nil {
		return fmt.Errorf("failed to fetch prompt list: %w", err)
	}

	// Print header
	fmt.Printf("Available prompts (version %s):\n\n", store.Version)

	// Find longest name for alignment
	maxNameLen := 0
	for _, prompt := range store.Prompts {
		if len(prompt.Name) > maxNameLen {
			maxNameLen = len(prompt.Name)
		}
	}

	// Print prompts
	for _, prompt := range store.Prompts {
		// Format: name (padded) - title
		fmt.Printf("  %-*s  %s\n", maxNameLen, prompt.Name, prompt.Title)
		fmt.Printf("  %*s  %s\n", maxNameLen, "", prompt.Description)

		// Print tags
		if len(prompt.Tags) > 0 {
			fmt.Printf("  %*s  Tags: %s\n", maxNameLen, "", strings.Join(prompt.Tags, ", "))
		}
		fmt.Println()
	}

	fmt.Printf("Usage: forge prompt get <name>\n")
	fmt.Printf("Example: forge prompt get migrate-makefile\n")

	return nil
}

// promptGet fetches and displays a specific prompt
func promptGet(name string) error {
	// Fetch prompt store
	store, err := fetchPromptStore()
	if err != nil {
		return fmt.Errorf("failed to fetch prompt list: %w", err)
	}

	// Find prompt
	var prompt *PromptEntry
	for i := range store.Prompts {
		if store.Prompts[i].Name == name {
			prompt = &store.Prompts[i]
			break
		}
	}

	if prompt == nil {
		return fmt.Errorf("prompt not found: %s\nRun 'forge prompt list' to see available prompts", name)
	}

	// Try to read from local file first (when running inside the repo)
	localPath := filepath.Join(localPromptsDir, prompt.URL)
	if content, err := os.ReadFile(localPath); err == nil {
		// Print header
		fmt.Printf("# %s\n", prompt.Title)
		fmt.Printf("# %s\n", prompt.Description)
		fmt.Printf("# Source: local (%s)\n", localPath)
		fmt.Println()

		// Print content
		fmt.Print(string(content))
		return nil
	}

	// Fall back to fetching from URL
	promptURL := store.BaseURL + "/" + prompt.URL
	content, err := fetchURL(promptURL)
	if err != nil {
		return fmt.Errorf("failed to fetch prompt: %w", err)
	}

	// Print header
	fmt.Printf("# %s\n", prompt.Title)
	fmt.Printf("# %s\n", prompt.Description)
	fmt.Printf("# URL: %s\n", promptURL)
	fmt.Println()

	// Print content
	fmt.Print(content)

	return nil
}

// fetchPromptStore fetches and parses the prompt-list.yaml
// It first checks if running inside the forge repository and reads locally if available
func fetchPromptStore() (*PromptStore, error) {
	// Try to read from local file first (when running inside the repo)
	if content, err := os.ReadFile(localPromptList); err == nil {
		var store PromptStore
		if err := yaml.Unmarshal(content, &store); err != nil {
			return nil, fmt.Errorf("failed to parse local prompt list: %w", err)
		}
		return &store, nil
	}

	// Fall back to fetching from URL
	content, err := fetchURL(promptListURL)
	if err != nil {
		return nil, err
	}

	var store PromptStore
	if err := yaml.Unmarshal([]byte(content), &store); err != nil {
		return nil, fmt.Errorf("failed to parse prompt list: %w", err)
	}

	return &store, nil
}

// fetchURL fetches content from a URL with timeout
func fetchURL(url string) (string, error) {
	client := &http.Client{
		Timeout: httpTimeout,
	}

	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	return string(body), nil
}
