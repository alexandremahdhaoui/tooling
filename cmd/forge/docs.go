package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sigs.k8s.io/yaml"
)

const (
	docsListURL   = "https://raw.githubusercontent.com/alexandremahdhaoui/forge/refs/heads/main/docs/docs-list.yaml"
	localDocsList = "docs/docs-list.yaml"
	localDocsDir  = "docs"
)

// DocStore represents the docs-list.yaml structure
type DocStore struct {
	Version string     `yaml:"version"`
	BaseURL string     `yaml:"baseURL"`
	Docs    []DocEntry `yaml:"docs"`
}

// DocEntry represents a single document in the list
type DocEntry struct {
	Name        string   `yaml:"name"`
	Title       string   `yaml:"title"`
	Description string   `yaml:"description"`
	URL         string   `yaml:"url"`
	Tags        []string `yaml:"tags"`
}

// runDocs handles the "forge docs" command
func runDocs(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: forge docs <list|get> [doc-name]")
	}

	operation := args[0]

	switch operation {
	case "list":
		return docsList()
	case "get":
		if len(args) < 2 {
			return fmt.Errorf("usage: forge docs get <doc-name>")
		}
		return docsGet(args[1])
	default:
		return fmt.Errorf("unknown operation: %s (valid: list, get)", operation)
	}
}

// docsList lists all available documentation
func docsList() error {
	// Fetch docs store
	store, err := fetchDocsStore()
	if err != nil {
		return fmt.Errorf("failed to fetch docs list: %w", err)
	}

	// Print header
	fmt.Printf("Available documentation (version %s):\n\n", store.Version)

	// Find longest name for alignment
	maxNameLen := 0
	for _, doc := range store.Docs {
		if len(doc.Name) > maxNameLen {
			maxNameLen = len(doc.Name)
		}
	}

	// Print docs
	for _, doc := range store.Docs {
		// Format: name (padded) - title
		fmt.Printf("  %-*s  %s\n", maxNameLen, doc.Name, doc.Title)
		fmt.Printf("  %*s  %s\n", maxNameLen, "", doc.Description)

		// Print tags
		if len(doc.Tags) > 0 {
			fmt.Printf("  %*s  Tags: %s\n", maxNameLen, "", strings.Join(doc.Tags, ", "))
		}
		fmt.Println()
	}

	fmt.Printf("Usage: forge docs get <name>\n")
	fmt.Printf("Example: forge docs get architecture\n")

	return nil
}

// docsGet fetches and displays a specific document
func docsGet(name string) error {
	// Fetch docs store
	store, err := fetchDocsStore()
	if err != nil {
		return fmt.Errorf("failed to fetch docs list: %w", err)
	}

	// Find document
	var doc *DocEntry
	for i := range store.Docs {
		if store.Docs[i].Name == name {
			doc = &store.Docs[i]
			break
		}
	}

	if doc == nil {
		return fmt.Errorf("document not found: %s\nRun 'forge docs list' to see available documentation", name)
	}

	// Try to read from local file first (when running inside the repo)
	localPath := filepath.Join(doc.URL)
	if content, err := os.ReadFile(localPath); err == nil {
		// Print header
		fmt.Printf("# %s\n", doc.Title)
		fmt.Printf("# %s\n", doc.Description)
		fmt.Printf("# Source: local (%s)\n", localPath)
		fmt.Println()

		// Print content
		fmt.Print(string(content))
		return nil
	}

	// Fall back to fetching from URL
	docURL := store.BaseURL + "/" + doc.URL
	content, err := fetchURL(docURL)
	if err != nil {
		return fmt.Errorf("failed to fetch document: %w", err)
	}

	// Print header
	fmt.Printf("# %s\n", doc.Title)
	fmt.Printf("# %s\n", doc.Description)
	fmt.Printf("# URL: %s\n", docURL)
	fmt.Println()

	// Print content
	fmt.Print(content)

	return nil
}

// fetchDocsStore fetches and parses the docs-list.yaml
// It first checks if running inside the forge repository and reads locally if available
func fetchDocsStore() (*DocStore, error) {
	// Try to read from local file first (when running inside the repo)
	if content, err := os.ReadFile(localDocsList); err == nil {
		var store DocStore
		if err := yaml.Unmarshal(content, &store); err != nil {
			return nil, fmt.Errorf("failed to parse local docs list: %w", err)
		}
		return &store, nil
	}

	// Fall back to fetching from URL
	content, err := fetchURL(docsListURL)
	if err != nil {
		return nil, err
	}

	var store DocStore
	if err := yaml.Unmarshal([]byte(content), &store); err != nil {
		return nil, fmt.Errorf("failed to parse docs list: %w", err)
	}

	return &store, nil
}
