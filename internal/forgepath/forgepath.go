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
// Behavior:
// - If forge module is in dependencies/cache: use module path github.com/alexandremahdhaoui/forge/cmd/{packageName}
// - If running from within forge repo: use relative path ./cmd/{packageName}
// - Otherwise: use versioned module syntax github.com/alexandremahdhaoui/forge/cmd/{packageName}@latest
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

	// Try to use module path first (preferred when forge is in dependencies)
	moduleCmd := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", forgeModule)
	if output, err := moduleCmd.Output(); err == nil {
		modulePath := strings.TrimSpace(string(output))
		if modulePath != "" {
			// Module is available in go.mod or cache, use module path
			return []string{"run", fmt.Sprintf("%s/cmd/%s", forgeModule, packageName)}, nil
		}
	}

	// Try to find forge repository locally
	forgeRepo, err := FindForgeRepo()
	if err != nil {
		// Can't find forge locally, use versioned module syntax
		// This allows `go run` to fetch from the module proxy
		return []string{"run", fmt.Sprintf("%s/cmd/%s@latest", forgeModule, packageName)}, nil
	}

	// Found local forge repo - check if we're running from within it
	cwd, err := os.Getwd()
	if err != nil {
		// Can't get CWD, fall back to versioned module
		return []string{"run", fmt.Sprintf("%s/cmd/%s@latest", forgeModule, packageName)}, nil
	}

	// Check if CWD is inside forge repo (for development)
	absForgeRepo, err := filepath.Abs(forgeRepo)
	if err != nil {
		return []string{"run", fmt.Sprintf("%s/cmd/%s@latest", forgeModule, packageName)}, nil
	}

	absCwd, err := filepath.Abs(cwd)
	if err != nil {
		return []string{"run", fmt.Sprintf("%s/cmd/%s@latest", forgeModule, packageName)}, nil
	}

	// If CWD is inside forge repo, use relative path for development
	if strings.HasPrefix(absCwd, absForgeRepo+string(filepath.Separator)) || absCwd == absForgeRepo {
		// Running from within forge repo - use relative path
		relPath, err := filepath.Rel(absCwd, filepath.Join(absForgeRepo, "cmd", packageName))
		if err != nil {
			// Shouldn't happen, but fall back to versioned module
			return []string{"run", fmt.Sprintf("%s/cmd/%s@latest", forgeModule, packageName)}, nil
		}
		return []string{"run", relPath}, nil
	}

	// We're outside the forge repo - use versioned module syntax
	// This allows customers to run forge tools without having the repo locally
	return []string{"run", fmt.Sprintf("%s/cmd/%s@latest", forgeModule, packageName)}, nil
}
