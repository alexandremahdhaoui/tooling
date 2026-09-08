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
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/alexandremahdhaoui/forge/internal/forgepath"
	"github.com/alexandremahdhaoui/forge/pkg/enginedocs"
	"sigs.k8s.io/yaml"
)

const (
	httpTimeout = 10 * time.Second
)

const (
	docsListURL      = "https://raw.githubusercontent.com/alexandremahdhaoui/forge/refs/heads/main/docs/docs-list.yaml"
	localDocsList    = "docs/docs-list.yaml"
	localDocsDir     = "docs"
	enginesListURL   = "https://raw.githubusercontent.com/alexandremahdhaoui/forge/refs/heads/main/docs/engines-list.yaml"
	localEnginesList = "docs/engines-list.yaml"
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

// EnginesStore represents the engines registry (docs/engines-list.yaml)
type EnginesStore struct {
	Version string        `yaml:"version" json:"version"`
	BaseURL string        `yaml:"baseURL" json:"baseURL"`
	Engines []EngineEntry `yaml:"engines" json:"engines"`
}

// EngineEntry represents a single engine in the registry
type EngineEntry struct {
	Name string `yaml:"name" json:"name"`
	Path string `yaml:"path" json:"path"`
}

// Engine represents an engine with documentation
type Engine struct {
	Name     string `json:"name" yaml:"name"`
	DocCount int    `json:"docCount" yaml:"docCount"`
}

// EngineDoc represents a document with its parent engine
type EngineDoc struct {
	Engine      string   `json:"engine" yaml:"engine"`
	Name        string   `json:"name" yaml:"name"`
	Title       string   `json:"title" yaml:"title"`
	Description string   `json:"description" yaml:"description"`
	Path        string   `json:"path" yaml:"path"`
	Tags        []string `json:"tags,omitempty" yaml:"tags,omitempty"`
}

// runDocs handles the "forge docs" command
func runDocs(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: forge docs <list|get> [args...]\n\n" +
			"Commands:\n" +
			"  list              List all engines with documentation\n" +
			"  list <engine>     List docs for a specific engine\n" +
			"  list all          List all docs from all engines\n" +
			"  get <name>        Get a specific document\n\n" +
			"Options:\n" +
			"  --format=<fmt>    Output format: table (default), json, yaml\n" +
			"  -o <fmt>          Short form of --format")
	}

	operation := args[0]

	switch operation {
	case "list":
		return docsListCommand(args[1:])
	case "get":
		if len(args) < 2 {
			return fmt.Errorf("usage: forge docs get <doc-name>")
		}
		return docsGet(args[1])
	default:
		return fmt.Errorf("unknown operation: %s (valid: list, get)", operation)
	}
}

// docsListCommand handles the "forge docs list" command with subcommands
// Usage:
//   - forge docs list           -> list engines
//   - forge docs list <engine>  -> list docs for engine
//   - forge docs list all       -> list all docs
func docsListCommand(args []string) error {
	// Parse format flag
	format, remaining := parseOutputFormat(args)

	if len(remaining) == 0 {
		// No arguments: list engines
		engines, err := listEngines()
		if err != nil {
			return fmt.Errorf("failed to list engines: %w", err)
		}
		formatEnginesOutput(engines, format)
		return nil
	}

	target := remaining[0]

	if target == "all" {
		// List all docs from all engines
		docs, err := listAllDocs()
		if err != nil {
			return fmt.Errorf("failed to list all docs: %w", err)
		}
		formatDocsOutput(docs, "", format)
		return nil
	}

	// List docs for specific engine
	docs, err := listDocsByEngine(target)
	if err != nil {
		return fmt.Errorf("failed to list docs for engine '%s': %w", target, err)
	}
	formatDocsOutput(docs, target, format)
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

// fetchEnginesStore fetches the engines registry from local file or HTTP.
// In run-local mode it reads the checkout's list.
// Otherwise, it fetches from HTTP (the default for users outside the repo).
func fetchEnginesStore() (*EnginesStore, error) {
	var data []byte
	var err error

	if forgepath.RunLocal() {
		data, err = os.ReadFile(localEnginesList)
		if err != nil {
			return nil, fmt.Errorf("failed to read local engines list in run-local mode: %w", err)
		}
	} else {
		// Default: fetch from HTTP
		content, fetchErr := fetchURL(enginesListURL)
		if fetchErr != nil {
			return nil, fmt.Errorf("failed to fetch engines list from %s: %w", enginesListURL, fetchErr)
		}
		data = []byte(content)
	}

	var store EnginesStore
	if err := yaml.Unmarshal(data, &store); err != nil {
		return nil, fmt.Errorf("failed to parse engines list: %w", err)
	}

	return &store, nil
}

// fetchEngineDocStore fetches a specific engine's doc store via HTTP.
func fetchEngineDocStore(engineName string, enginesStore *EnginesStore) (*enginedocs.DocStore, error) {
	// Find engine in the store
	var enginePath string
	for _, engine := range enginesStore.Engines {
		if engine.Name == engineName {
			enginePath = engine.Path
			break
		}
	}
	if enginePath == "" {
		return nil, fmt.Errorf("engine %q not found in engines registry", engineName)
	}

	// Construct URL and fetch
	url := enginesStore.BaseURL + "/" + enginePath + "/docs/list.yaml"
	content, err := fetchURL(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch engine docs from %s: %w", url, err)
	}

	var store enginedocs.DocStore
	if err := yaml.Unmarshal([]byte(content), &store); err != nil {
		return nil, fmt.Errorf("failed to parse engine docs: %w", err)
	}

	return &store, nil
}

// AggregatedDocEntry extends DocEntry with engine information
type AggregatedDocEntry struct {
	DocEntry
	Engine string `yaml:"engine" json:"engine"`
}

// AggregatedDocsResult represents the combined result of all documentation sources
type AggregatedDocsResult struct {
	GlobalDocs []DocEntry           `json:"globalDocs"`
	EngineDocs []AggregatedDocEntry `json:"engineDocs"`
	Errors     []AggregationError   `json:"errors,omitempty"`
}

// AggregationError represents an error when loading docs from an engine
type AggregationError struct {
	Engine string `json:"engine"`
	Error  string `json:"error"`
}

// discoverEngineDocs finds all engine directories that have docs/list.yaml.
// Returns (paths, nil, nil) in run-local mode.
// Returns (paths, enginesStore, nil) when in HTTP mode (default).
func discoverEngineDocs() ([]string, *EnginesStore, error) {
	if forgepath.RunLocal() {
		// LOCAL MODE: scan cmd/ directory (existing logic)
		entries, err := os.ReadDir("cmd")
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read cmd directory: %w", err)
		}

		var engines []string
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			// Skip forge itself (it has global docs)
			if entry.Name() == "forge" {
				continue
			}

			listPath := filepath.Join("cmd", entry.Name(), "docs", "list.yaml")
			if _, err := os.Stat(listPath); err == nil {
				engines = append(engines, filepath.Join("cmd", entry.Name()))
			}
		}
		return engines, nil, nil
	}

	// HTTP MODE (default): fetch from engines-list.yaml
	enginesStore, err := fetchEnginesStore()
	if err != nil {
		return nil, nil, err
	}

	var engines []string
	for _, engine := range enginesStore.Engines {
		engines = append(engines, engine.Path)
	}
	return engines, enginesStore, nil
}

// listEngines returns all engines with documentation and their doc counts
// It includes the "forge" engine for global docs and all engine-specific docs
func listEngines() ([]Engine, error) {
	var engines []Engine

	// Load global docs first (forge engine)
	globalStore, err := fetchDocsStore()
	if err == nil && len(globalStore.Docs) > 0 {
		engines = append(engines, Engine{
			Name:     "forge",
			DocCount: len(globalStore.Docs),
		})
	}

	// Discover engine directories with docs
	engineDirs, enginesStore, err := discoverEngineDocs()
	if err != nil {
		// If no engines found but we have global docs, that's okay
		if len(engines) > 0 {
			return engines, nil
		}
		return nil, fmt.Errorf("failed to discover engines: %w", err)
	}

	// Load doc count from each engine
	for _, engineDir := range engineDirs {
		engineName := filepath.Base(engineDir)
		listPath := filepath.Join(engineDir, "docs", "list.yaml")

		content, err := os.ReadFile(listPath)
		if err != nil {
			// If local read fails and we have enginesStore, try remote
			if enginesStore != nil {
				store, fetchErr := fetchEngineDocStore(engineName, enginesStore)
				if fetchErr == nil {
					engines = append(engines, Engine{
						Name:     engineName,
						DocCount: len(store.Docs),
					})
				}
			}
			continue // Skip engines that fail to load
		}

		var store enginedocs.DocStore
		if err := yaml.Unmarshal(content, &store); err != nil {
			continue // Skip engines with invalid YAML
		}

		engines = append(engines, Engine{
			Name:     engineName,
			DocCount: len(store.Docs),
		})
	}

	// Sort alphabetically by name
	sort.Slice(engines, func(i, j int) bool {
		return engines[i].Name < engines[j].Name
	})

	return engines, nil
}

// listDocsByEngine returns all docs for a specific engine
// For "forge" engine, returns global docs
// For other engines, returns docs from cmd/{engine}/docs/list.yaml
func listDocsByEngine(engineName string) ([]EngineDoc, error) {
	if engineName == "forge" {
		// Return global forge docs
		store, err := fetchDocsStore()
		if err != nil {
			return nil, fmt.Errorf("failed to load forge docs: %w", err)
		}

		var docs []EngineDoc
		for _, doc := range store.Docs {
			docs = append(docs, EngineDoc{
				Engine:      "forge",
				Name:        doc.Name,
				Title:       doc.Title,
				Description: doc.Description,
				Path:        doc.URL,
				Tags:        doc.Tags,
			})
		}
		return docs, nil
	}

	// Load docs from engine's list.yaml
	listPath := filepath.Join("cmd", engineName, "docs", "list.yaml")
	content, err := os.ReadFile(listPath)
	if err != nil {
		// Local read failed, try remote fetching
		enginesStore, fetchErr := fetchEnginesStore()
		if fetchErr != nil {
			return nil, fmt.Errorf("engine '%s' not found (local: %v, remote: %v)", engineName, err, fetchErr)
		}

		store, fetchErr := fetchEngineDocStore(engineName, enginesStore)
		if fetchErr != nil {
			return nil, fmt.Errorf("engine '%s' not found or has no docs: %w", engineName, fetchErr)
		}

		var docs []EngineDoc
		for _, doc := range store.Docs {
			docs = append(docs, EngineDoc{
				Engine:      engineName,
				Name:        doc.Name,
				Title:       doc.Title,
				Description: doc.Description,
				Path:        doc.URL,
				Tags:        doc.Tags,
			})
		}
		return docs, nil
	}

	var store enginedocs.DocStore
	if err := yaml.Unmarshal(content, &store); err != nil {
		return nil, fmt.Errorf("failed to parse list.yaml for engine '%s': %w", engineName, err)
	}

	var docs []EngineDoc
	for _, doc := range store.Docs {
		docs = append(docs, EngineDoc{
			Engine:      engineName,
			Name:        doc.Name,
			Title:       doc.Title,
			Description: doc.Description,
			Path:        doc.URL,
			Tags:        doc.Tags,
		})
	}

	return docs, nil
}

// listAllDocs returns all docs from all engines (global + engine-specific)
// This is similar to aggregateDocsList but returns EngineDoc format
func listAllDocs() ([]EngineDoc, error) {
	var allDocs []EngineDoc

	// Load global forge docs
	globalStore, err := fetchDocsStore()
	if err == nil {
		for _, doc := range globalStore.Docs {
			allDocs = append(allDocs, EngineDoc{
				Engine:      "forge",
				Name:        doc.Name,
				Title:       doc.Title,
				Description: doc.Description,
				Path:        doc.URL,
				Tags:        doc.Tags,
			})
		}
	}

	// Discover and load engine docs
	engineDirs, enginesStore, err := discoverEngineDocs()
	if err != nil {
		// If no engines found but we have global docs, return those
		if len(allDocs) > 0 {
			return allDocs, nil
		}
		return nil, fmt.Errorf("failed to discover engines: %w", err)
	}

	for _, engineDir := range engineDirs {
		engineName := filepath.Base(engineDir)
		listPath := filepath.Join(engineDir, "docs", "list.yaml")

		content, err := os.ReadFile(listPath)
		if err != nil {
			// If local read fails and we have enginesStore, try remote
			if enginesStore != nil {
				store, fetchErr := fetchEngineDocStore(engineName, enginesStore)
				if fetchErr == nil {
					for _, doc := range store.Docs {
						allDocs = append(allDocs, EngineDoc{
							Engine:      engineName,
							Name:        doc.Name,
							Title:       doc.Title,
							Description: doc.Description,
							Path:        doc.URL,
							Tags:        doc.Tags,
						})
					}
				}
			}
			continue
		}

		var store enginedocs.DocStore
		if err := yaml.Unmarshal(content, &store); err != nil {
			continue
		}

		for _, doc := range store.Docs {
			allDocs = append(allDocs, EngineDoc{
				Engine:      engineName,
				Name:        doc.Name,
				Title:       doc.Title,
				Description: doc.Description,
				Path:        doc.URL,
				Tags:        doc.Tags,
			})
		}
	}

	// Sort by engine name, then by doc name
	sort.Slice(allDocs, func(i, j int) bool {
		if allDocs[i].Engine != allDocs[j].Engine {
			return allDocs[i].Engine < allDocs[j].Engine
		}
		return allDocs[i].Name < allDocs[j].Name
	})

	return allDocs, nil
}

// formatEnginesOutput formats a list of engines for display
func formatEnginesOutput(engines []Engine, format outputFormat) {
	switch format {
	case outputFormatJSON:
		printJSON(map[string]interface{}{"engines": engines})
	case outputFormatYAML:
		printYAML(map[string]interface{}{"engines": engines})
	default:
		// Table format
		if len(engines) == 0 {
			fmt.Println("No engines with documentation found.")
			return
		}

		// Find max engine name length for alignment
		maxNameLen := len("ENGINE")
		for _, e := range engines {
			if len(e.Name) > maxNameLen {
				maxNameLen = len(e.Name)
			}
		}

		// Print header
		fmt.Printf("%-*s  %s\n", maxNameLen, "ENGINE", "DOCS")
		fmt.Printf("%-*s  %s\n", maxNameLen, strings.Repeat("-", maxNameLen), "----")

		// Print rows
		for _, e := range engines {
			fmt.Printf("%-*s  %d\n", maxNameLen, e.Name, e.DocCount)
		}

		fmt.Printf("\nTotal: %d engine(s)\n", len(engines))
		fmt.Println("\nUsage: forge docs list <engine>  # List docs for an engine")
		fmt.Println("       forge docs list all       # List all docs")
	}
}

// formatDocsOutput formats a list of docs for display
// If engineName is provided, shows docs for that engine
// If engineName is empty, shows all docs with engine column
func formatDocsOutput(docs []EngineDoc, engineName string, format outputFormat) {
	switch format {
	case outputFormatJSON:
		if engineName != "" {
			printJSON(map[string]interface{}{"engine": engineName, "docs": docs})
		} else {
			printJSON(map[string]interface{}{"docs": docs})
		}
	case outputFormatYAML:
		if engineName != "" {
			printYAML(map[string]interface{}{"engine": engineName, "docs": docs})
		} else {
			printYAML(map[string]interface{}{"docs": docs})
		}
	default:
		// Table format
		if len(docs) == 0 {
			fmt.Println("No documentation found.")
			return
		}

		// Find max lengths for alignment
		maxEngineLen := len("ENGINE")
		maxNameLen := len("NAME")
		maxTitleLen := len("TITLE")

		for _, d := range docs {
			if len(d.Engine) > maxEngineLen {
				maxEngineLen = len(d.Engine)
			}
			if len(d.Name) > maxNameLen {
				maxNameLen = len(d.Name)
			}
			if len(d.Title) > maxTitleLen {
				maxTitleLen = len(d.Title)
			}
		}

		// Cap title length at 50 for readability
		if maxTitleLen > 50 {
			maxTitleLen = 50
		}

		if engineName != "" {
			// Single engine mode - don't show engine column
			fmt.Printf("Engine: %s\n\n", engineName)
			fmt.Printf("%-*s  %s\n", maxNameLen, "NAME", "TITLE")
			fmt.Printf("%-*s  %s\n", maxNameLen, strings.Repeat("-", maxNameLen), strings.Repeat("-", maxTitleLen))

			for _, d := range docs {
				title := d.Title
				if len(title) > 50 {
					title = title[:47] + "..."
				}
				fmt.Printf("%-*s  %s\n", maxNameLen, d.Name, title)
			}
		} else {
			// All docs mode - show engine column
			fmt.Printf("%-*s  %-*s  %s\n", maxEngineLen, "ENGINE", maxNameLen, "NAME", "TITLE")
			fmt.Printf("%-*s  %-*s  %s\n", maxEngineLen, strings.Repeat("-", maxEngineLen),
				maxNameLen, strings.Repeat("-", maxNameLen), strings.Repeat("-", maxTitleLen))

			for _, d := range docs {
				title := d.Title
				if len(title) > 50 {
					title = title[:47] + "..."
				}
				fmt.Printf("%-*s  %-*s  %s\n", maxEngineLen, d.Engine, maxNameLen, d.Name, title)
			}
		}

		fmt.Printf("\nTotal: %d doc(s)\n", len(docs))
		fmt.Println("\nUsage: forge docs get <engine>/<name>  # Get a document")
	}
}

// aggregatedDocsGet retrieves a document by name, routing to the correct engine
// For names like "go-build/usage", it reads from the go-build engine
// For names without a prefix, it reads from global docs
func aggregatedDocsGet(name string) (string, error) {
	// Check if this is an engine-prefixed name
	if strings.Contains(name, "/") {
		parts := strings.SplitN(name, "/", 2)
		if len(parts) != 2 {
			return "", fmt.Errorf("invalid document name format: %s", name)
		}
		engineName := parts[0]
		docName := parts[1]

		return getEngineDoc(engineName, docName)
	}

	// Fall back to global docs
	return docsGetContent(name)
}

// getEngineDoc retrieves a document from a specific engine
func getEngineDoc(engineName, docName string) (string, error) {
	listPath := filepath.Join("cmd", engineName, "docs", "list.yaml")

	var store *enginedocs.DocStore
	var remoteBaseURL string // Used for constructing doc content URL in remote mode

	content, err := os.ReadFile(listPath)
	if err != nil {
		// Local read failed, try remote fetching
		enginesStore, fetchErr := fetchEnginesStore()
		if fetchErr != nil {
			return "", fmt.Errorf("engine '%s' not found (local: %v, remote: %v)", engineName, err, fetchErr)
		}

		remoteStore, fetchErr := fetchEngineDocStore(engineName, enginesStore)
		if fetchErr != nil {
			return "", fmt.Errorf("engine '%s' not found or has no docs: %w", engineName, fetchErr)
		}
		store = remoteStore
		remoteBaseURL = enginesStore.BaseURL // Use enginesStore.BaseURL for doc content
	} else {
		var localStore enginedocs.DocStore
		if err := yaml.Unmarshal(content, &localStore); err != nil {
			return "", fmt.Errorf("failed to parse list.yaml for engine '%s': %w", engineName, err)
		}
		store = &localStore
	}

	// Find the document
	var doc *enginedocs.DocEntry
	for i := range store.Docs {
		if store.Docs[i].Name == docName {
			doc = &store.Docs[i]
			break
		}
	}

	if doc == nil {
		return "", fmt.Errorf("document '%s' not found in engine '%s'\nRun 'forge docs list' to see available documentation", docName, engineName)
	}

	// Try to read from local file first
	if docContent, err := os.ReadFile(doc.URL); err == nil {
		return string(docContent), nil
	}

	// Fall back to remote URL
	// Priority: 1) store.BaseURL (from engine's list.yaml), 2) remoteBaseURL (from enginesStore)
	baseURL := store.BaseURL
	if baseURL == "" && remoteBaseURL != "" {
		baseURL = remoteBaseURL
	}

	if baseURL != "" {
		docURL := baseURL + "/" + doc.URL
		return fetchURL(docURL)
	}

	return "", fmt.Errorf("document file not found: %s", doc.URL)
}

// docsGetContent retrieves a global document content by name
// This is a helper that returns just the content without printing
func docsGetContent(name string) (string, error) {
	store, err := fetchDocsStore()
	if err != nil {
		return "", fmt.Errorf("failed to fetch docs list: %w", err)
	}

	var doc *DocEntry
	for i := range store.Docs {
		if store.Docs[i].Name == name {
			doc = &store.Docs[i]
			break
		}
	}

	if doc == nil {
		return "", fmt.Errorf("document not found: %s\nRun 'forge docs list' to see available documentation", name)
	}

	// Try to read from local file first
	if content, err := os.ReadFile(doc.URL); err == nil {
		return string(content), nil
	}

	// Fall back to remote URL
	docURL := store.BaseURL + "/" + doc.URL
	return fetchURL(docURL)
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
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	return string(body), nil
}
