package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// parseEngine parses an engine URI and returns the engine type and binary path.
// Supports go:// protocol: go://build-go, go://build-container, go://github.com/.../cmd/build-go
func parseEngine(engineURI string) (engineType, binaryPath string, err error) {
	if !strings.HasPrefix(engineURI, "go://") {
		return "", "", fmt.Errorf("unsupported engine protocol: %s (must start with go://)", engineURI)
	}

	// Remove go:// prefix
	path := strings.TrimPrefix(engineURI, "go://")
	if path == "" {
		return "", "", fmt.Errorf("empty engine path after go://")
	}

	// Extract binary name from path
	// Examples:
	//   build-go -> build-go
	//   github.com/alexandremahdhaoui/tooling/cmd/build-go -> build-go
	parts := strings.Split(path, "/")
	binaryName := parts[len(parts)-1]

	if binaryName == "" {
		return "", "", fmt.Errorf("could not extract binary name from engine URI: %s", engineURI)
	}

	// Try to find the binary in ./build/bin/ first, then PATH
	localPath := filepath.Join("./build/bin", binaryName)
	if _, err := os.Stat(localPath); err == nil {
		return "mcp", localPath, nil
	}

	// Fall back to binary name (will be found in PATH)
	return "mcp", binaryName, nil
}
