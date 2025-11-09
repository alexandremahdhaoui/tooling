// Package forgepath provides utilities for locating the forge source repository
// and constructing commands to execute forge tools via `go run`.
package forgepath

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

const (
	forgeModule     = "github.com/alexandremahdhaoui/forge"
	forgeRepoEnvVar = "FORGE_REPO_PATH"
)

var (
	// Cache for forge repository path to avoid repeated filesystem/command operations
	cachedForgeRepoPath string
	cachedForgeRepoErr  error
	cacheOnce           sync.Once
)

// FindForgeRepo locates the forge source repository using multiple detection methods.
// It checks in the following order:
// 1. FORGE_REPO_PATH environment variable
// 2. Go module cache using `go list -m -f '{{.Dir}}' github.com/alexandremahdhaoui/forge`
// 3. Walking up from os.Executable() to find forge repository
//
// Returns the absolute path to the forge repository or an error if not found.
func FindForgeRepo() (string, error) {
	cacheOnce.Do(func() {
		cachedForgeRepoPath, cachedForgeRepoErr = findForgeRepoUncached()
	})
	return cachedForgeRepoPath, cachedForgeRepoErr
}

// findForgeRepoUncached performs the actual forge repository detection without caching.
func findForgeRepoUncached() (string, error) {
	// Method 1: Check FORGE_REPO_PATH environment variable
	if envPath := os.Getenv(forgeRepoEnvVar); envPath != "" {
		absPath, err := filepath.Abs(envPath)
		if err != nil {
			return "", fmt.Errorf("failed to resolve FORGE_REPO_PATH: %w", err)
		}
		if IsForgeRepo(absPath) {
			return absPath, nil
		}
		return "", fmt.Errorf("FORGE_REPO_PATH points to non-forge directory: %s", absPath)
	}

	// Method 2: Use `go list` to find the module in Go's module cache
	cmd := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", forgeModule)
	output, err := cmd.Output()
	if err == nil {
		modulePath := strings.TrimSpace(string(output))
		if modulePath != "" && IsForgeRepo(modulePath) {
			return modulePath, nil
		}
	}

	// Method 3: Walk up from os.Executable() to find forge repository
	execPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get executable path: %w", err)
	}

	// Resolve symlinks
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve executable symlinks: %w", err)
	}

	// Walk up the directory tree
	dir := filepath.Dir(execPath)
	for {
		if IsForgeRepo(dir) {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("forge repository not found (checked: env var, go list, executable path)")
}

// IsForgeRepo checks if the given directory is the forge repository.
// It verifies by checking:
// 1. go.mod exists and contains the forge module path
// 2. cmd/forge/main.go exists (main forge CLI)
func IsForgeRepo(dir string) bool {
	// Check if go.mod exists and contains forge module
	goModPath := filepath.Join(dir, "go.mod")
	goModContent, err := os.ReadFile(goModPath)
	if err != nil {
		return false
	}

	// Check if go.mod declares the forge module
	if !strings.Contains(string(goModContent), forgeModule) {
		return false
	}

	// Check if cmd/forge/main.go exists
	forgeMainPath := filepath.Join(dir, "cmd", "forge", "main.go")
	if _, err := os.Stat(forgeMainPath); err != nil {
		return false
	}

	return true
}

// BuildGoRunCommand constructs the command arguments for executing a forge MCP server
// via `go run`. The returned slice is suitable for use with exec.Command("go", args...).
//
// It prefers using the module path (github.com/alexandremahdhaoui/forge/cmd/{packageName})
// for portability, but falls back to the local repository path if needed.
//
// Example usage:
//
//	args, err := BuildGoRunCommand("testenv-kind")
//	// Returns: ["run", "github.com/alexandremahdhaoui/forge/cmd/testenv-kind"]
//	cmd := exec.Command("go", args...)
func BuildGoRunCommand(packageName string) ([]string, error) {
	if packageName == "" {
		return nil, fmt.Errorf("package name cannot be empty")
	}

	// Try to use module path first (preferred for portability)
	moduleCmd := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", forgeModule)
	if output, err := moduleCmd.Output(); err == nil {
		modulePath := strings.TrimSpace(string(output))
		if modulePath != "" {
			// Module is available, use module path
			return []string{"run", fmt.Sprintf("%s/cmd/%s", forgeModule, packageName)}, nil
		}
	}

	// Fall back to local repository path
	forgeRepo, err := FindForgeRepo()
	if err != nil {
		return nil, fmt.Errorf("failed to locate forge repository: %w", err)
	}

	// Use local path
	return []string{"run", filepath.Join(forgeRepo, "cmd", packageName)}, nil
}
