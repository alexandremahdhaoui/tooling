package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// parseEngine parses an engine URI and returns the engine type and binary path.
// Supports go:// protocol with auto-install:
//   - go://build-go -> installs github.com/alexandremahdhaoui/forge/cmd/build-go@latest
//   - go://build-go@v1.0.0 -> installs github.com/alexandremahdhaoui/forge/cmd/build-go@v1.0.0
//   - go://github.com/user/repo/cmd/tool@v1.0.0 -> installs as-is
func parseEngine(engineURI string) (engineType, binaryPath string, err error) {
	if !strings.HasPrefix(engineURI, "go://") {
		return "", "", fmt.Errorf("unsupported engine protocol: %s (must start with go://)", engineURI)
	}

	// Remove go:// prefix
	path := strings.TrimPrefix(engineURI, "go://")
	if path == "" {
		return "", "", fmt.Errorf("empty engine path after go://")
	}

	// Split path and version
	var packagePath, version string
	if idx := strings.Index(path, "@"); idx != -1 {
		packagePath = path[:idx]
		version = path[idx+1:]
	} else {
		packagePath = path
		version = "latest"
	}

	// Expand short names to full package paths
	// If path doesn't contain slashes, it's a short name
	if !strings.Contains(packagePath, "/") {
		packagePath = "github.com/alexandremahdhaoui/forge/cmd/" + packagePath
	}

	// Extract binary name from package path
	parts := strings.Split(packagePath, "/")
	binaryName := parts[len(parts)-1]

	if binaryName == "" {
		return "", "", fmt.Errorf("could not extract binary name from engine URI: %s", engineURI)
	}

	// Try to find the binary in ./build/bin/ first (for local development)
	localPath := filepath.Join("./build/bin", binaryName)
	if _, err := os.Stat(localPath); err == nil {
		return "mcp", localPath, nil
	}

	// Check if binary exists in PATH
	if _, err := exec.LookPath(binaryName); err == nil {
		// Binary found in PATH, use it
		return "mcp", binaryName, nil
	}

	// Binary not found, auto-install it
	fmt.Printf("⏳ Installing engine: %s@%s...\n", packagePath, version)
	installPath := packagePath + "@" + version
	cmd := exec.Command("go", "install", installPath)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", "", fmt.Errorf("failed to install engine %s: %w", installPath, err)
	}
	fmt.Printf("✅ Engine installed: %s\n", binaryName)

	// Return binary name (should now be in PATH)
	return "mcp", binaryName, nil
}
