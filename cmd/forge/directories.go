package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

// ForgeDirs contains standardized directory paths for forge operations.
type ForgeDirs struct {
	// TmpDir is the temporary directory for non-build artifacts (e.g., test reports, coverage files)
	// Format: ./tmp/tmp-<uuid> (absolute path)
	TmpDir string `json:"tmpDir"`

	// BuildDir is the directory for build outputs (e.g., binaries, containers)
	// Format: ./build (absolute path)
	BuildDir string `json:"buildDir"`

	// RootDir is the repository root directory for code generation
	// Format: . (absolute path)
	RootDir string `json:"rootDir"`
}

// createForgeDirs creates and returns standardized forge directories.
// It creates the tmp directory with a unique UUID suffix and returns absolute paths.
func createForgeDirs() (*ForgeDirs, error) {
	// Get current working directory (root dir)
	rootDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}

	// Build dir is ./build
	buildDir := filepath.Join(rootDir, "build")
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create build directory: %w", err)
	}

	// Tmp dir is ./tmp/tmp-<uuid>
	tmpBase := filepath.Join(rootDir, "tmp")
	if err := os.MkdirAll(tmpBase, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create tmp base directory: %w", err)
	}

	tmpDir := filepath.Join(tmpBase, fmt.Sprintf("tmp-%s", uuid.New().String()))
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create tmp directory: %w", err)
	}

	return &ForgeDirs{
		TmpDir:   tmpDir,
		BuildDir: buildDir,
		RootDir:  rootDir,
	}, nil
}

// ToMap converts ForgeDirs to a map for easy merging with MCP parameters.
func (d *ForgeDirs) ToMap() map[string]any {
	return map[string]any{
		"tmpDir":   d.TmpDir,
		"buildDir": d.BuildDir,
		"rootDir":  d.RootDir,
	}
}

// cleanupOldTmpDirs removes tmp directories older than the specified number of runs to keep.
// It keeps the N most recent tmp-* directories based on modification time.
func cleanupOldTmpDirs(keepCount int) error {
	rootDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	tmpBase := filepath.Join(rootDir, "tmp")
	if _, err := os.Stat(tmpBase); os.IsNotExist(err) {
		return nil // No tmp directory, nothing to clean
	}

	// Read all entries in tmp/
	entries, err := os.ReadDir(tmpBase)
	if err != nil {
		return fmt.Errorf("failed to read tmp directory: %w", err)
	}

	// Collect tmp-* directories with their modification times
	type dirInfo struct {
		path    string
		modTime int64
	}
	var tmpDirs []dirInfo

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if len(name) < 4 || name[:4] != "tmp-" {
			continue
		}

		fullPath := filepath.Join(tmpBase, name)
		info, err := os.Stat(fullPath)
		if err != nil {
			continue
		}

		tmpDirs = append(tmpDirs, dirInfo{
			path:    fullPath,
			modTime: info.ModTime().Unix(),
		})
	}

	// If we have fewer directories than keepCount, nothing to clean
	if len(tmpDirs) <= keepCount {
		return nil
	}

	// Sort by modification time (oldest first)
	for i := 0; i < len(tmpDirs)-1; i++ {
		for j := i + 1; j < len(tmpDirs); j++ {
			if tmpDirs[i].modTime > tmpDirs[j].modTime {
				tmpDirs[i], tmpDirs[j] = tmpDirs[j], tmpDirs[i]
			}
		}
	}

	// Remove oldest directories, keeping only keepCount most recent
	dirsToRemove := len(tmpDirs) - keepCount
	for i := 0; i < dirsToRemove; i++ {
		if err := os.RemoveAll(tmpDirs[i].path); err != nil {
			// Log but don't fail on cleanup errors
			fmt.Fprintf(os.Stderr, "Warning: failed to remove old tmp directory %s: %v\n", tmpDirs[i].path, err)
		}
	}

	return nil
}
