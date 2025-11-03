package main

import (
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
)

// Version information (set via ldflags during build, or from build info)
//
// These variables can be set at build time using ldflags:
//   go build -ldflags "-X main.Version=v1.0.0 -X main.CommitSHA=abc1234 -X main.BuildTimestamp=2025-01-01T00:00:00Z"
//
// If not set via ldflags, version info is automatically extracted from Go's build metadata
// which is embedded when using `go install` or building from a git repository.
var (
	Version        = "dev"
	CommitSHA      = "unknown"
	BuildTimestamp = "unknown"
)

// getVersionInfo returns version information from ldflags or Go build info.
//
// Priority order:
// 1. ldflags values (if set via -X flags during build)
// 2. Go build info from debug.ReadBuildInfo() (available with go install)
// 3. Default values ("dev", "unknown")
//
// When installed via `go install github.com/alexandremahdhaoui/forge/cmd/forge@v1.0.0`:
// - Version comes from the module version (v1.0.0 or pseudo-version)
// - Commit comes from vcs.revision (embedded by Go)
// - Timestamp comes from vcs.time (embedded by Go)
func getVersionInfo() (version, commit, timestamp string) {
	// Start with default/ldflags values
	version = Version
	commit = CommitSHA
	timestamp = BuildTimestamp

	// Try to get build info from Go modules (works with go install)
	if info, ok := debug.ReadBuildInfo(); ok {
		// Use module version if available and we don't have a custom version
		if version == "dev" && info.Main.Version != "" && info.Main.Version != "(devel)" {
			version = info.Main.Version
		}

		// Extract VCS information from build settings (requires Go 1.18+)
		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				if commit == "unknown" && len(setting.Value) >= 7 {
					commit = setting.Value[:7] // Short commit hash
				}
			case "vcs.time":
				if timestamp == "unknown" {
					timestamp = setting.Value
				}
			}
		}
	}

	return version, commit, timestamp
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "build":
		if err := runBuild(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "integration":
		if err := runIntegration(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "version", "--version", "-v":
		printVersion()
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`forge - A build orchestration tool

Usage:
  forge build [artifact-name]    Build artifacts from forge.yaml
  forge integration <command>    Manage integration environments
  forge version                  Show version information

Commands:
  build                         Build all artifacts
  integration create [name]     Create integration environment
  integration list              List integration environments
  integration get <id>          Get environment details
  integration delete <id>       Delete integration environment
  version                       Show version information
  help                          Show this help message`)
}

func printVersion() {
	version, commit, timestamp := getVersionInfo()
	fmt.Printf("forge version %s\n", version)
	fmt.Printf("  commit:    %s\n", commit)
	fmt.Printf("  built:     %s\n", timestamp)
	fmt.Printf("  go:        %s\n", runtime.Version())
	fmt.Printf("  platform:  %s/%s\n", runtime.GOOS, runtime.GOARCH)
}
