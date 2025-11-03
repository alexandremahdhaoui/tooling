package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/alexandremahdhaoui/forge/internal/util"
	"github.com/alexandremahdhaoui/forge/internal/version"
	"github.com/alexandremahdhaoui/forge/pkg/flaterrors"
	"github.com/alexandremahdhaoui/forge/pkg/forge"
	"github.com/caarlos0/env/v11"
)

const Name = "build-go"

// Version information (set via ldflags during build)
var (
	Version        = "dev"
	CommitSHA      = "unknown"
	BuildTimestamp = "unknown"
)

// versionInfo holds build-go's version information
var versionInfo *version.Info

func init() {
	versionInfo = version.New(Name)
	versionInfo.Version = Version
	versionInfo.CommitSHA = CommitSHA
	versionInfo.BuildTimestamp = BuildTimestamp
}

// ----------------------------------------------------- MAIN ------------------------------------------------------- //

func main() {
	// Check for version flag
	for _, arg := range os.Args[1:] {
		if arg == "version" || arg == "--version" || arg == "-v" {
			versionInfo.Print()
			return
		}
	}

	// Check for --mcp flag to run as MCP server
	for _, arg := range os.Args[1:] {
		if arg == "--mcp" {
			if err := runMCPServer(); err != nil {
				log.Printf("MCP server error: %v", err)
				os.Exit(1)
			}
			return
		}
	}

	// Normal CLI mode
	if err := run(); err != nil {
		printFailure(err)
		os.Exit(1)
		return
	}

	printSuccess()
	os.Exit(0)
}

// ----------------------------------------------------- RUN -------------------------------------------------------- //

var errBuildingBinaries = errors.New("building binaries")

// run executes the main logic of the build-go tool.
// It reads the project configuration, builds all defined binaries, and writes artifacts to the artifact store.
func run() error {
	// I. Read environment variables
	envs := Envs{} //nolint:exhaustruct // unmarshal

	if err := env.Parse(&envs); err != nil {
		return flaterrors.Join(err, errBuildingBinaries)
	}

	// II. Read project configuration
	config, err := forge.ReadSpec()
	if err != nil {
		return flaterrors.Join(err, errBuildingBinaries)
	}

	// III. Read artifact store
	store, err := forge.ReadOrCreateArtifactStore(config.ArtifactStorePath)
	if err != nil {
		return flaterrors.Join(err, errBuildingBinaries)
	}

	// IV. Get git version for artifacts
	version, err := getGitVersion()
	if err != nil {
		return flaterrors.Join(err, errBuildingBinaries)
	}

	timestamp := time.Now().UTC().Format(time.RFC3339)

	// V. Build each binary spec
	for _, spec := range config.Build.Specs {
		// Skip if spec name is empty or engine doesn't match
		if spec.Name == "" || spec.Engine != "go://build-go" {
			continue
		}

		if err := buildBinary(envs, spec, version, timestamp, &store, false); err != nil {
			return flaterrors.Join(err, errBuildingBinaries)
		}
	}

	// VI. Write artifact store
	if err := forge.WriteArtifactStore(config.ArtifactStorePath, store); err != nil {
		return flaterrors.Join(err, errBuildingBinaries)
	}

	return nil
}

var errGettingGitVersion = errors.New("getting git version")

// getGitVersion gets the current git commit hash to use as the artifact version.
func getGitVersion() (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", flaterrors.Join(err, errGettingGitVersion)
	}

	version := strings.TrimSpace(string(output))
	if version == "" {
		return "", flaterrors.Join(errors.New("empty git version"), errGettingGitVersion)
	}

	return version, nil
}

var errBuildingBinary = errors.New("building binary")

// buildBinary builds a single binary based on the provided spec and adds it to the artifact store.
// The isMCPMode parameter controls output streams (stdout must be reserved for JSON-RPC).
func buildBinary(
	envs Envs,
	spec forge.BuildSpec,
	version, timestamp string,
	store *forge.ArtifactStore,
	isMCPMode bool,
) error {
	// In MCP mode, write to stderr; in normal mode, write to stdout
	out := os.Stdout
	if isMCPMode {
		out = os.Stderr
	}
	_, _ = fmt.Fprintf(out, "⏳ Building binary: %s\n", spec.Name)

	// I. Determine output path
	destination := spec.Dest
	if destination == "" {
		destination = "./build/bin"
	}

	// Create destination directory if it doesn't exist
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return flaterrors.Join(err, errBuildingBinary)
	}

	outputPath := filepath.Join(destination, spec.Name)

	// II. Set CGO_ENABLED=0 for static binaries
	if err := os.Setenv("CGO_ENABLED", "0"); err != nil {
		return flaterrors.Join(err, errBuildingBinary)
	}

	// III. Build the binary
	args := []string{
		"build",
		"-o", outputPath,
	}

	// Add ldflags if provided
	if envs.GoBuildLDFlags != "" {
		args = append(args, "-ldflags", envs.GoBuildLDFlags)
	}

	// Add source path
	args = append(args, spec.Src)

	cmd := exec.Command("go", args...)

	// In MCP mode, redirect output to stderr to avoid corrupting JSON-RPC stream
	if isMCPMode {
		// Show build output on stderr (safe for MCP)
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return flaterrors.Join(err, errBuildingBinary)
		}
	} else {
		// Normal mode: show all output
		if err := util.RunCmdWithStdPipes(cmd); err != nil {
			return flaterrors.Join(err, errBuildingBinary)
		}
	}

	// IV. Create artifact entry
	artifact := forge.Artifact{
		Name:      spec.Name,
		Type:      "binary",
		Location:  outputPath,
		Timestamp: timestamp,
		Version:   version,
	}

	forge.AddOrUpdateArtifact(store, artifact)

	_, _ = fmt.Fprintf(out, "✅ Built binary: %s (version: %s)\n", spec.Name, version)

	return nil
}

// ----------------------------------------------------- ENVS ------------------------------------------------------- //

// Envs holds the environment variables required by the build-go tool.
type Envs struct {
	// GoBuildLDFlags are the linker flags to pass to the `go build` command.
	GoBuildLDFlags string `env:"GO_BUILD_LDFLAGS"`
}

// ----------------------------------------------------- PRINT HELPERS ----------------------------------------------- //

func printSuccess() {
	_, _ = fmt.Fprintln(os.Stdout, "✅ All binaries built successfully")
}

func printFailure(err error) {
	_, _ = fmt.Fprintf(os.Stderr, "❌ Error building binaries\n%s\n", err.Error())
}
