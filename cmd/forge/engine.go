package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const forgeRepoPrefix = "github.com/alexandremahdhaoui/forge/cmd/"

// isForgeRepoEngine checks if the package path is from the forge repository
func isForgeRepoEngine(packagePath string) bool {
	return strings.HasPrefix(packagePath, forgeRepoPrefix)
}

// parseEngine parses an engine URI and returns the engine type and binary path.
// Supports go:// and alias:// protocols:
//   - go://build-go -> installs github.com/alexandremahdhaoui/forge/cmd/build-go@<forge-version>
//   - go://build-go@v1.0.0 -> installs github.com/alexandremahdhaoui/forge/cmd/build-go@v1.0.0
//   - go://github.com/alexandremahdhaoui/forge/cmd/build-go -> installs with forge's version
//   - go://github.com/user/repo/cmd/tool@v1.0.0 -> installs as-is
//   - alias://my-engine -> resolves alias from forge.yaml engines section
func parseEngine(engineURI string) (engineType, binaryPath string, err error) {
	// Check for alias:// protocol - return special marker
	if strings.HasPrefix(engineURI, "alias://") {
		aliasName := strings.TrimPrefix(engineURI, "alias://")
		if aliasName == "" {
			return "", "", fmt.Errorf("empty alias name after alias://")
		}
		// Return special marker - caller will handle resolution
		return "alias", aliasName, nil
	}

	if !strings.HasPrefix(engineURI, "go://") {
		return "", "", fmt.Errorf("unsupported engine protocol: %s (must start with go:// or alias://)", engineURI)
	}

	// Remove go:// prefix
	path := strings.TrimPrefix(engineURI, "go://")
	if path == "" {
		return "", "", fmt.Errorf("empty engine path after go://")
	}

	// Split path and version
	var packagePath, version string
	hasExplicitVersion := false
	if idx := strings.Index(path, "@"); idx != -1 {
		packagePath = path[:idx]
		version = path[idx+1:]
		hasExplicitVersion = true
	} else {
		packagePath = path
	}

	// Expand short names to full package paths
	// If path doesn't contain slashes, it's a short name
	if !strings.Contains(packagePath, "/") {
		packagePath = forgeRepoPrefix + packagePath
	}

	// Determine version to install
	if !hasExplicitVersion {
		if isForgeRepoEngine(packagePath) {
			// Use forge's version for forge repo engines
			forgeVersion, _, _ := versionInfo.Get()
			if forgeVersion != "dev" && forgeVersion != "(devel)" && forgeVersion != "" {
				version = forgeVersion
			} else {
				version = "latest"
			}
		} else {
			version = "latest"
		}
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
		// Check version compatibility for local forge repo engines
		if isForgeRepoEngine(packagePath) {
			if err := checkEngineVersion(localPath, binaryName); err != nil {
				fmt.Fprintf(os.Stderr, "⚠️  Warning: %v\n", err)
			}
		}
		return "mcp", localPath, nil
	}

	// Check if binary exists in PATH
	if foundPath, err := exec.LookPath(binaryName); err == nil {
		// Check version compatibility for forge repo engines
		if isForgeRepoEngine(packagePath) {
			if err := checkEngineVersion(foundPath, binaryName); err != nil {
				fmt.Fprintf(os.Stderr, "⚠️  Warning: %v\n", err)
			}
		}
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
	fmt.Printf("✅ Engine installed: %s@%s\n", binaryName, version)

	// Verify installation and check version
	if isForgeRepoEngine(packagePath) {
		if foundPath, err := exec.LookPath(binaryName); err == nil {
			if err := checkEngineVersion(foundPath, binaryName); err != nil {
				fmt.Fprintf(os.Stderr, "⚠️  Warning: %v\n", err)
			}
		}
	}

	// Return binary name (should now be in PATH)
	return "mcp", binaryName, nil
}

// checkEngineVersion checks if the engine's version matches forge's version
func checkEngineVersion(binaryPath, binaryName string) error {
	// Get forge version
	forgeVersion, _, _ := versionInfo.Get()
	if forgeVersion == "dev" || forgeVersion == "(devel)" {
		// Skip version check for dev builds
		return nil
	}

	// Try to get engine version by running with --version
	cmd := exec.Command(binaryPath, "--version")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		// If --version fails, try -v
		cmd = exec.Command(binaryPath, "-v")
		out.Reset()
		cmd.Stdout = &out
		cmd.Stderr = &out
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("engine %s does not support version checking", binaryName)
		}
	}

	// Parse version from output (expecting format like "tool-name version v1.0.0")
	output := out.String()
	lines := strings.Split(output, "\n")
	if len(lines) > 0 {
		firstLine := strings.TrimSpace(lines[0])
		// Extract version (look for pattern "version vX.Y.Z")
		parts := strings.Fields(firstLine)
		for i, part := range parts {
			if part == "version" && i+1 < len(parts) {
				engineVersion := parts[i+1]
				// Normalize versions for comparison (remove 'v' prefix if present)
				forgeVer := strings.TrimPrefix(forgeVersion, "v")
				engineVer := strings.TrimPrefix(engineVersion, "v")

				// Compare major.minor versions (ignore patch and build metadata)
				forgeVer = strings.Split(forgeVer, "-")[0] // Remove build metadata
				engineVer = strings.Split(engineVer, "-")[0]

				if forgeVer != engineVer && engineVer != "dev" {
					return fmt.Errorf("engine %s version mismatch: forge=%s, engine=%s (consider running: go install %s%s@%s)",
						binaryName, forgeVersion, engineVersion, forgeRepoPrefix, binaryName, forgeVersion)
				}
				break
			}
		}
	}

	return nil
}
