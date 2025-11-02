package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/alexandremahdhaoui/tooling/internal/util"
	"github.com/alexandremahdhaoui/tooling/pkg/flaterrors"
	"github.com/alexandremahdhaoui/tooling/pkg/project"
	"github.com/caarlos0/env/v11"
)

const Name = "build-binary"

// ----------------------------------------------------- MAIN ------------------------------------------------------- //

func main() {
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

// run executes the main logic of the build-binary tool.
// It reads the project configuration, builds all defined binaries, and writes artifacts to the artifact store.
func run() error {
	// I. Read environment variables
	envs := Envs{} //nolint:exhaustruct // unmarshal

	if err := env.Parse(&envs); err != nil {
		return flaterrors.Join(err, errBuildingBinaries)
	}

	// II. Read project configuration
	config, err := project.ReadConfig()
	if err != nil {
		return flaterrors.Join(err, errBuildingBinaries)
	}

	// III. Read artifact store
	store, err := project.ReadArtifactStore(config.Build.ArtifactStorePath)
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
		// Skip if binary spec is empty
		if spec.Binary.Name == "" {
			continue
		}

		if err := buildBinary(envs, spec.Binary, version, timestamp, &store); err != nil {
			return flaterrors.Join(err, errBuildingBinaries)
		}
	}

	// VI. Write artifact store
	if err := project.WriteArtifactStore(config.Build.ArtifactStorePath, store); err != nil {
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
func buildBinary(envs Envs, spec project.BinarySpec, version, timestamp string, store *project.ArtifactStore) error {
	_, _ = fmt.Fprintf(os.Stdout, "⏳ Building binary: %s\n", spec.Name)

	// I. Determine output path
	destination := spec.Destination
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
	args = append(args, spec.Source)

	cmd := exec.Command("go", args...)

	if err := util.RunCmdWithStdPipes(cmd); err != nil {
		return flaterrors.Join(err, errBuildingBinary)
	}

	// IV. Create artifact entry
	artifact := project.Artifact{
		Name:      spec.Name,
		Type:      "binary",
		Location:  outputPath,
		Timestamp: timestamp,
		Version:   version,
	}

	project.AddOrUpdateArtifact(store, artifact)

	_, _ = fmt.Fprintf(os.Stdout, "✅ Built binary: %s (version: %s)\n", spec.Name, version)

	return nil
}

// ----------------------------------------------------- ENVS ------------------------------------------------------- //

// Envs holds the environment variables required by the build-binary tool.
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
