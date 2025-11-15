package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/alexandremahdhaoui/forge/internal/version"
)

// Version information (set via ldflags during build)
var (
	Version        = "dev"
	CommitSHA      = "unknown"
	BuildTimestamp = "unknown"
)

var versionInfo *version.Info

func init() {
	versionInfo = version.New("test-runner-go-verify-tags")
	versionInfo.Version = Version
	versionInfo.CommitSHA = CommitSHA
	versionInfo.BuildTimestamp = BuildTimestamp
}

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--mcp":
			// Run in MCP server mode
			if err := runMCPServer(); err != nil {
				fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
				os.Exit(1)
			}
			return
		case "version", "--version", "-v":
			versionInfo.Print()
			return
		case "help", "--help", "-h":
			printUsage()
			return
		}
	}

	// Get the root directory to search (default to current directory)
	rootDir := "."
	if len(os.Args) > 1 {
		rootDir = os.Args[1]
	}

	// Find all test files
	testFiles, err := findTestFiles(rootDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error finding test files: %v\n", err)
		os.Exit(1)
	}

	if len(testFiles) == 0 {
		fmt.Println("No test files found")
		return
	}

	fmt.Printf("Checking %d test files for build tags...\n", len(testFiles))

	// Verify each test file has a build tag
	var filesWithoutTags []string
	for _, file := range testFiles {
		hasBuildTag, err := checkBuildTag(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error checking %s: %v\n", file, err)
			continue
		}
		if !hasBuildTag {
			filesWithoutTags = append(filesWithoutTags, file)
		}
	}

	// Report results
	if len(filesWithoutTags) > 0 {
		fmt.Fprintf(os.Stderr, "\n❌ Found %d test file(s) without build tags:\n\n", len(filesWithoutTags))

		// Print table header
		fmt.Fprintf(os.Stderr, "%-80s  %s\n", "FILE PATH", "MISSING TAG")
		fmt.Fprintf(os.Stderr, "%s  %s\n",
			"--------------------------------------------------------------------------------",
			"------------")

		// Print each file
		for _, file := range filesWithoutTags {
			relPath := file
			if len(relPath) > 80 {
				// Truncate long paths with ellipsis
				relPath = "..." + relPath[len(relPath)-77:]
			}
			fmt.Fprintf(os.Stderr, "%-80s  ❌\n", relPath)
		}

		fmt.Fprintf(os.Stderr, "\nTest files must have one of these build tags:\n")
		fmt.Fprintf(os.Stderr, "  //go:build unit\n")
		fmt.Fprintf(os.Stderr, "  //go:build integration\n")
		fmt.Fprintf(os.Stderr, "  //go:build e2e\n")
		os.Exit(1)
	}

	fmt.Printf("✅ All test files have valid build tags\n")
}

// findTestFiles recursively finds all *_test.go files
func findTestFiles(root string) ([]string, error) {
	var testFiles []string

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip vendor, .git, .tmp directories
		if info.IsDir() {
			name := info.Name()
			if name == "vendor" || name == ".git" || name == ".tmp" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if it's a test file
		if strings.HasSuffix(path, "_test.go") {
			testFiles = append(testFiles, path)
		}

		return nil
	})

	return testFiles, err
}

// checkBuildTag checks if a file has a valid build tag
func checkBuildTag(filePath string) (bool, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return false, err
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)

	// Check first few lines for build tag
	lineCount := 0
	for scanner.Scan() && lineCount < 5 {
		line := strings.TrimSpace(scanner.Text())
		lineCount++

		// Check for go:build directive
		if strings.HasPrefix(line, "//go:build") {
			// Verify it's one of our expected tags
			if strings.Contains(line, "unit") ||
				strings.Contains(line, "integration") ||
				strings.Contains(line, "e2e") {
				return true, nil
			}
		}

		// Skip empty lines and comments, but stop at package declaration
		if strings.HasPrefix(line, "package ") {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return false, err
	}

	return false, nil
}

// runMCPServer starts the MCP server.
func runMCPServer() error {
	return runMCPServerImpl()
}

// verifyTags performs the tag verification and returns results.
// Returns (filesWithoutTags, totalFiles, error).
func verifyTags(rootDir string) ([]string, int, error) {
	// Find all test files
	testFiles, err := findTestFiles(rootDir)
	if err != nil {
		return nil, 0, fmt.Errorf("error finding test files: %w", err)
	}

	if len(testFiles) == 0 {
		return []string{}, 0, nil
	}

	// Verify each test file has a build tag
	var filesWithoutTags []string
	for _, file := range testFiles {
		hasBuildTag, err := checkBuildTag(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error checking %s: %v\n", file, err)
			continue
		}
		if !hasBuildTag {
			filesWithoutTags = append(filesWithoutTags, file)
		}
	}

	return filesWithoutTags, len(testFiles), nil
}

func printUsage() {
	fmt.Print(`test-runner-go-verify-tags - Verify all test files have build tags

Usage:
  test-runner-go-verify-tags [directory]    Verify test files in directory (default: .)
  test-runner-go-verify-tags --mcp          Run as MCP server
  test-runner-go-verify-tags version        Show version information
  test-runner-go-verify-tags help           Show this help message

Description:
  This tool recursively searches for *_test.go files and verifies that each
  file has a valid build tag. Valid build tags are:
    //go:build unit
    //go:build integration
    //go:build e2e

  The tool exits with code 1 if any test files are missing build tags.

Examples:
  test-runner-go-verify-tags .
  test-runner-go-verify-tags ./cmd/forge
  test-runner-go-verify-tags /path/to/project
`)
}
