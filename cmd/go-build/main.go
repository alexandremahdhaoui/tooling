package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/alexandremahdhaoui/forge/internal/cli"
	"github.com/alexandremahdhaoui/forge/internal/util"
	"github.com/alexandremahdhaoui/forge/pkg/flaterrors"
	"github.com/alexandremahdhaoui/forge/pkg/forge"
	"github.com/caarlos0/env/v11"
)

const Name = "go-build"

// Version information (set via ldflags during build)
var (
	Version        = "dev"
	CommitSHA      = "unknown"
	BuildTimestamp = "unknown"
)

// ----------------------------------------------------- MAIN ------------------------------------------------------- //

func main() {
	cli.Bootstrap(cli.Config{
		Name:           Name,
		Version:        Version,
		CommitSHA:      CommitSHA,
		BuildTimestamp: BuildTimestamp,
		RunCLI:         run,
		RunMCP:         runMCPServer,
		SuccessHandler: printSuccess,
		FailureHandler: printFailure,
	})
}

// ----------------------------------------------------- RUN -------------------------------------------------------- //

var errBuildingBinaries = errors.New("building binaries")

// run executes the main logic of the go-build tool.
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
	for _, spec := range config.Build {
		// Skip if spec name is empty or engine doesn't match
		if spec.Name == "" || spec.Engine != "go://go-build" {
			continue
		}

		// Extract build options from spec if provided
		opts := extractBuildOptions(spec)

		if err := buildBinary(envs, spec, version, timestamp, &store, false, opts); err != nil {
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

// extractBuildOptions extracts BuildOptions from a BuildSpec's Spec field.
func extractBuildOptions(spec forge.BuildSpec) *BuildOptions {
	if len(spec.Spec) == 0 {
		return nil
	}

	opts := &BuildOptions{}

	// Extract args if present
	if argsVal, ok := spec.Spec["args"]; ok {
		if args, ok := argsVal.([]interface{}); ok {
			opts.CustomArgs = make([]string, 0, len(args))
			for _, arg := range args {
				if argStr, ok := arg.(string); ok {
					opts.CustomArgs = append(opts.CustomArgs, argStr)
				}
			}
		}
	}

	// Extract env if present
	if envVal, ok := spec.Spec["env"]; ok {
		if env, ok := envVal.(map[string]interface{}); ok {
			opts.CustomEnv = make(map[string]string, len(env))
			for key, val := range env {
				if valStr, ok := val.(string); ok {
					opts.CustomEnv[key] = valStr
				}
			}
		}
	}

	// Return nil if no options were extracted
	if len(opts.CustomArgs) == 0 && len(opts.CustomEnv) == 0 {
		return nil
	}

	return opts
}

var errBuildingBinary = errors.New("building binary")

// BuildOptions contains optional build configuration that can override defaults.
type BuildOptions struct {
	// CustomArgs are additional arguments to pass to `go build` (e.g., "-tags=netgo")
	CustomArgs []string
	// CustomEnv are environment variables to set for the build (e.g., {"GOOS": "linux"})
	CustomEnv map[string]string
}

// buildBinary builds a single binary based on the provided spec and adds it to the artifact store.
// The isMCPMode parameter controls output streams (stdout must be reserved for JSON-RPC).
func buildBinary(
	envs Envs,
	spec forge.BuildSpec,
	version, timestamp string,
	store *forge.ArtifactStore,
	isMCPMode bool,
	opts *BuildOptions,
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

	// II. Set environment variables
	// Set CGO_ENABLED=0 for static binaries (can be overridden by custom env)
	if err := os.Setenv("CGO_ENABLED", "0"); err != nil {
		return flaterrors.Join(err, errBuildingBinary)
	}

	// Apply custom environment variables if provided
	if opts != nil && len(opts.CustomEnv) > 0 {
		for key, value := range opts.CustomEnv {
			if err := os.Setenv(key, value); err != nil {
				return flaterrors.Join(err, errBuildingBinary)
			}
		}
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

	// Add custom args if provided
	if opts != nil && len(opts.CustomArgs) > 0 {
		args = append(args, opts.CustomArgs...)
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

// Envs holds the environment variables required by the go-build tool.
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
