package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/alexandremahdhaoui/forge/internal/cli"
	"github.com/alexandremahdhaoui/forge/internal/util"
	"github.com/alexandremahdhaoui/forge/pkg/flaterrors"
	"github.com/alexandremahdhaoui/forge/pkg/forge"
	"github.com/caarlos0/env/v11"
)

const Name = "container-build"

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

var (
	errBuildingContainers     = errors.New("building containers")
	errInvalidContainerEngine = errors.New("invalid CONTAINER_BUILD_ENGINE")
)

// validateContainerEngine validates that the container engine is one of the supported values.
func validateContainerEngine(engine string) error {
	validEngines := []string{"docker", "kaniko", "podman"}
	for _, valid := range validEngines {
		if engine == valid {
			return nil
		}
	}
	return fmt.Errorf("%w: must be one of %v, got %q",
		errInvalidContainerEngine, validEngines, engine)
}

// run executes the main logic of the container-build tool.
// It reads the project configuration, builds all defined containers, and writes artifacts to the artifact store.
func run() error {
	// I. Read environment variables
	envs := Envs{} //nolint:exhaustruct // unmarshal

	if err := env.Parse(&envs); err != nil {
		printUsage()
		return flaterrors.Join(err, errBuildingContainers)
	}

	// Validate container engine
	if err := validateContainerEngine(envs.BuildEngine); err != nil {
		printUsage()
		return flaterrors.Join(err, errBuildingContainers)
	}

	// II. Read project configuration
	config, err := forge.ReadSpec()
	if err != nil {
		return flaterrors.Join(err, errBuildingContainers)
	}

	// III. Read artifact store
	store, err := forge.ReadOrCreateArtifactStore(config.ArtifactStorePath)
	if err != nil {
		return flaterrors.Join(err, errBuildingContainers)
	}

	// IV. Get git version for artifacts
	version, err := getGitVersion()
	if err != nil {
		return flaterrors.Join(err, errBuildingContainers)
	}

	timestamp := time.Now().UTC().Format(time.RFC3339)

	// V. Build each container spec
	for _, spec := range config.Build {
		// Skip if spec name is empty or engine doesn't match
		if spec.Name == "" || spec.Engine != "go://container-build" {
			continue
		}

		if err := buildContainer(envs, spec, version, timestamp, &store, false); err != nil {
			return flaterrors.Join(err, errBuildingContainers)
		}
	}

	// VI. Write artifact store
	if err := forge.WriteArtifactStore(config.ArtifactStorePath, store); err != nil {
		return flaterrors.Join(err, errBuildingContainers)
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

var errBuildingContainer = errors.New("building container")

// tagImage tags an image with a specific tag.
func tagImage(containerEngine, imageID, tag string, isMCPMode bool) error {
	cmd := exec.Command(containerEngine, "tag", imageID, tag)
	return runCmd(cmd, isMCPMode)
}

// addArtifactToStore adds a container artifact to the store.
func addArtifactToStore(
	store *forge.ArtifactStore,
	name, version, timestamp string,
) {
	artifact := forge.Artifact{
		Name:      name,
		Type:      "container",
		Location:  fmt.Sprintf("%s:%s", name, version),
		Timestamp: timestamp,
		Version:   version,
	}
	forge.AddOrUpdateArtifact(store, artifact)
}

// printBuildStart prints build start message.
func printBuildStart(out io.Writer, name string) {
	_, _ = fmt.Fprintf(out, "⏳ Building container: %s\n", name)
}

// printBuildSuccess prints build success message.
func printBuildSuccess(out io.Writer, name, version string) {
	_, _ = fmt.Fprintf(out, "✅ Built container: %s (version: %s)\n", name, version)
}

// buildContainerDocker builds a container using native docker build.
func buildContainerDocker(
	envs Envs,
	spec forge.BuildSpec,
	version, timestamp string,
	store *forge.ArtifactStore,
	isMCPMode bool,
) error {
	out := os.Stdout
	if isMCPMode {
		out = os.Stderr
	}

	printBuildStart(out, spec.Name)

	wd, err := os.Getwd()
	if err != nil {
		return flaterrors.Join(err, errBuildingContainer)
	}

	// Build image tags
	imageWithVersion := fmt.Sprintf("%s:%s", spec.Name, version)
	imageLatest := fmt.Sprintf("%s:latest", spec.Name)

	// Build using docker build
	cmd := exec.Command("docker", "build",
		"-f", spec.Src,
		"-t", imageWithVersion,
		"-t", imageLatest,
		wd,
	)

	// Add build args if provided
	for _, buildArg := range envs.BuildArgs {
		cmd.Args = append(cmd.Args, "--build-arg", buildArg)
	}

	if err := runCmd(cmd, isMCPMode); err != nil {
		return flaterrors.Join(err, errBuildingContainer)
	}

	// Add to artifact store
	addArtifactToStore(store, spec.Name, version, timestamp)

	printBuildSuccess(out, spec.Name, version)
	return nil
}

// buildContainerPodman builds a container using native podman build.
func buildContainerPodman(
	envs Envs,
	spec forge.BuildSpec,
	version, timestamp string,
	store *forge.ArtifactStore,
	isMCPMode bool,
) error {
	out := os.Stdout
	if isMCPMode {
		out = os.Stderr
	}

	printBuildStart(out, spec.Name)

	wd, err := os.Getwd()
	if err != nil {
		return flaterrors.Join(err, errBuildingContainer)
	}

	// Build image tags
	imageWithVersion := fmt.Sprintf("%s:%s", spec.Name, version)
	imageLatest := fmt.Sprintf("%s:latest", spec.Name)

	// Build using podman build
	cmd := exec.Command("podman", "build",
		"-f", spec.Src,
		"-t", imageWithVersion,
		"-t", imageLatest,
		wd,
	)

	// Add build args if provided
	for _, buildArg := range envs.BuildArgs {
		cmd.Args = append(cmd.Args, "--build-arg", buildArg)
	}

	if err := runCmd(cmd, isMCPMode); err != nil {
		return flaterrors.Join(err, errBuildingContainer)
	}

	// Add to artifact store
	addArtifactToStore(store, spec.Name, version, timestamp)

	printBuildSuccess(out, spec.Name, version)
	return nil
}

// buildContainerKaniko builds a container using Kaniko (rootless container builds).
func buildContainerKaniko(
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

	printBuildStart(out, spec.Name)

	wd, err := os.Getwd()
	if err != nil {
		return flaterrors.Join(err, errBuildingContainer)
	}

	// Expand cache directory path (handle ~ for home directory)
	cacheDir := expandPath(envs.KanikoCacheDir)

	// Create cache directory if it doesn't exist
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return flaterrors.Join(err, errBuildingContainer)
	}

	// Build image tags
	imageBase := spec.Name
	imageWithVersion := fmt.Sprintf("%s:%s", imageBase, version)
	imageLatest := fmt.Sprintf("%s:latest", imageBase)

	// Prepare kaniko command
	// Note: We use "docker" here to run the Kaniko executor container itself.
	// This is separate from BuildEngine which specifies we want to use Kaniko for building.
	containerRuntime := "docker"
	args := []string{
		"run", "-i",
		"-v", fmt.Sprintf("%s:/workspace", wd),
		"-v", fmt.Sprintf("%s:/cache", cacheDir),
		"gcr.io/kaniko-project/executor:latest",
		"-f", spec.Src,
		"--context", "/workspace",
		"--no-push",
		"--cache=true",
		"--cache-dir=/cache",
		"--cache-repo=oci:/cache/repo",
		"--tarPath", fmt.Sprintf("/workspace/.ignore.%s.tar", spec.Name),
	}

	// Add build args if provided
	for _, buildArg := range envs.BuildArgs {
		args = append(args, "--build-arg", buildArg)
	}

	// Execute build
	buildCmd := exec.Command(containerRuntime, args...)
	if err := runCmd(buildCmd, isMCPMode); err != nil {
		return flaterrors.Join(err, errBuildingContainer)
	}

	// Load the tar into the container engine
	tarPath := fmt.Sprintf(".ignore.%s.tar", spec.Name)
	loadCmd := exec.Command(containerRuntime, "load", "-i", tarPath)
	if err := runCmd(loadCmd, isMCPMode); err != nil {
		return flaterrors.Join(err, errBuildingContainer)
	}

	// Tag with version and latest
	// First, get the image ID from the tar
	imageID, err := getImageIDFromTar(containerRuntime, tarPath)
	if err != nil {
		return flaterrors.Join(err, errBuildingContainer)
	}

	// Tag with version
	if err := tagImage(containerRuntime, imageID, imageWithVersion, isMCPMode); err != nil {
		return flaterrors.Join(err, errBuildingContainer)
	}

	// Tag with latest
	if err := tagImage(containerRuntime, imageID, imageLatest, isMCPMode); err != nil {
		return flaterrors.Join(err, errBuildingContainer)
	}

	// Clean up tar file
	if err := os.Remove(tarPath); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "⚠️  Warning: failed to remove tar file: %s\n", err)
	}

	// Add to artifact store
	addArtifactToStore(store, spec.Name, version, timestamp)

	printBuildSuccess(out, spec.Name, version)

	return nil
}

// buildContainer dispatches to the appropriate build function based on container engine.
func buildContainer(
	envs Envs,
	spec forge.BuildSpec,
	version, timestamp string,
	store *forge.ArtifactStore,
	isMCPMode bool,
) error {
	// Dispatch based on container engine
	switch envs.BuildEngine {
	case "docker":
		return buildContainerDocker(envs, spec, version, timestamp, store, isMCPMode)
	case "kaniko":
		return buildContainerKaniko(envs, spec, version, timestamp, store, isMCPMode)
	case "podman":
		return buildContainerPodman(envs, spec, version, timestamp, store, isMCPMode)
	default:
		// Should be unreachable due to validation, but defensive programming
		return flaterrors.Join(
			fmt.Errorf("unsupported container engine: %s", envs.BuildEngine),
			errBuildingContainer,
		)
	}
}

// expandPath expands a path with ~ to the user's home directory.
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		if homeDir, err := os.UserHomeDir(); err == nil {
			return strings.Replace(path, "~", homeDir, 1)
		}
	}
	return path
}

var errGettingImageID = errors.New("getting image ID from tar")

// getImageIDFromTar loads a tar and extracts the image ID.
func getImageIDFromTar(containerEngine, tarPath string) (string, error) {
	cmd := exec.Command(containerEngine, "load", "-i", tarPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", flaterrors.Join(err, errGettingImageID)
	}

	// Parse output like: "Loaded image ID: sha256:abc123..."
	// or "Loaded image: <image>:latest"
	outputStr := string(output)
	lines := strings.Split(outputStr, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Loaded image") {
			// Extract image reference or ID
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				// Get everything after the first colon
				imageRef := strings.TrimSpace(strings.Join(parts[1:], ":"))
				return imageRef, nil
			}
		}
	}

	return "", flaterrors.Join(
		errors.New("could not parse image ID from load output: "+outputStr),
		errGettingImageID,
	)
}

// ----------------------------------------------------- ENVS ------------------------------------------------------- //

// Envs holds the environment variables required by the container-build tool.
type Envs struct {
	// BuildEngine specifies which container build engine to use: docker, kaniko, or podman.
	// Note: This is different from CONTAINER_ENGINE which may be used internally to run containers.
	BuildEngine string `env:"CONTAINER_BUILD_ENGINE,required"`
	// BuildArgs is a list of build arguments to pass to the container build command.
	BuildArgs []string `env:"BUILD_ARGS"`
	// KanikoCacheDir is the local directory to use for kaniko layer caching.
	// Defaults to ~/.kaniko-cache
	KanikoCacheDir string `env:"KANIKO_CACHE_DIR"          envDefault:"~/.kaniko-cache"`
}

// ----------------------------------------------------- PRINT HELPERS ----------------------------------------------- //

const usage = `USAGE

CONTAINER_BUILD_ENGINE=%q %s

Required environment variables:
    CONTAINER_BUILD_ENGINE    string    Container build engine: docker, kaniko, or podman.

Optional environment variables:
    BUILD_ARGS                []string  List of build args (e.g. "GO_BUILD_LDFLAGS=\"-X main.BuildTimestamp=$(TIMESTAMP)\"").
    KANIKO_CACHE_DIR          string    Local directory for kaniko layer caching (default: ~/.kaniko-cache).

Modes:
    docker  - Native docker build (fast, requires Docker daemon)
    kaniko  - Rootless Kaniko builds (runs in container via docker, secure)
    podman  - Native podman build (rootless, requires Podman)

Configuration:
    The tool reads container build specifications from forge.yaml
    Artifacts are written to the path specified in build.artifactStorePath

Note:
    CONTAINER_BUILD_ENGINE specifies which build mode to use.
    Internally, docker is used to run the Kaniko executor container in kaniko mode.
`

// runCmd runs a command, redirecting output to stderr in MCP mode to avoid corrupting JSON-RPC.
func runCmd(cmd *exec.Cmd, isMCPMode bool) error {
	if isMCPMode {
		// MCP mode: redirect all output to stderr (safe for JSON-RPC on stdout)
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	// Normal mode: show all output
	return util.RunCmdWithStdPipes(cmd)
}

func printUsage() {
	fmt.Printf(
		usage,
		os.Getenv("CONTAINER_BUILD_ENGINE"),
		os.Args[0],
	)
}

func printSuccess() {
	fmt.Printf("✅ All containers built successfully\n")
}

func printFailure(err error) {
	fmt.Printf("❌ Error building containers\n%s\n", err.Error())
}
