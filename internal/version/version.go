package version

import (
	"fmt"
	"runtime"
	"runtime/debug"
)

// Info holds version information for a tool.
type Info struct {
	// ToolName is the name of the tool
	ToolName string
	// Version is set via ldflags or from build info
	Version string
	// CommitSHA is set via ldflags or from build info
	CommitSHA string
	// BuildTimestamp is set via ldflags or from build info
	BuildTimestamp string
}

// Get returns version information, attempting to read from build info if not set via ldflags.
func (i *Info) Get() (version, commit, timestamp string) {
	version = i.Version
	commit = i.CommitSHA
	timestamp = i.BuildTimestamp

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

// Print outputs formatted version information to stdout.
func (i *Info) Print() {
	version, commit, timestamp := i.Get()
	fmt.Printf("%s version %s\n", i.ToolName, version)
	fmt.Printf("  commit:    %s\n", commit)
	fmt.Printf("  built:     %s\n", timestamp)
	fmt.Printf("  go:        %s\n", runtime.Version())
	fmt.Printf("  platform:  %s/%s\n", runtime.GOOS, runtime.GOARCH)
}

// String returns a one-line version string.
func (i *Info) String() string {
	version, _, _ := i.Get()
	return fmt.Sprintf("%s version %s", i.ToolName, version)
}

// New creates a new Info with default values.
func New(toolName string) *Info {
	return &Info{
		ToolName:       toolName,
		Version:        "dev",
		CommitSHA:      "unknown",
		BuildTimestamp: "unknown",
	}
}
