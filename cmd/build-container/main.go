package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/alexandremahdhaoui/forge/internal/util"
	"github.com/alexandremahdhaoui/forge/pkg/flaterrors"
	"github.com/alexandremahdhaoui/forge/pkg/forge"
	"github.com/caarlos0/env/v11"
)

const Name = "build-container"

// ----------------------------------------------------- MAIN ------------------------------------------------------- //

func main() {
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

var errBuildingContainers = errors.New("building containers")

// run executes the main logic of the build-container tool.
// It reads the project configuration, builds all defined containers, and writes artifacts to the artifact store.
func run() error {
	// I. Read environment variables
	envs := Envs{} //nolint:exhaustruct // unmarshal

	if err := env.Parse(&envs); err != nil {
		printUsage()
		return flaterrors.Join(err, errBuildingContainers)
	}

	// II. Read project configuration
	config, err := forge.ReadSpec()
	if err != nil {
		return flaterrors.Join(err, errBuildingContainers)
	}

	// III. Read artifact store
	store, err := forge.ReadArtifactStore(config.Build.ArtifactStorePath)
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
	for _, spec := range config.Build.Specs {
		// Skip if spec name is empty or engine doesn't match
		if spec.Name == "" || spec.Engine != "go://build-container" {
			continue
		}

		if err := buildContainer(envs, spec, version, timestamp, &store, false); err != nil {
			return flaterrors.Join(err, errBuildingContainers)
		}
	}

	// VI. Write artifact store
	if err := forge.WriteArtifactStore(config.Build.ArtifactStorePath, store); err != nil {
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

// buildContainer builds a single container and adds it to the artifact store.
// The isMCPMode parameter controls output streams (stdout must be reserved for JSON-RPC).
func buildContainer(
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
	_, _ = fmt.Fprintf(out, "⏳ Building container: %s\n", spec.Name)

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
	cmd := envs.ContainerEngine
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
	buildCmd := exec.Command(cmd, args...)
	if err := runCmd(buildCmd, isMCPMode); err != nil {
		return flaterrors.Join(err, errBuildingContainer)
	}

	// Load the tar into the container engine
	tarPath := fmt.Sprintf(".ignore.%s.tar", spec.Name)
	loadCmd := exec.Command(envs.ContainerEngine, "load", "-i", tarPath)
	if err := runCmd(loadCmd, isMCPMode); err != nil {
		return flaterrors.Join(err, errBuildingContainer)
	}

	// Tag with version and latest
	// First, get the image ID from the tar
	imageID, err := getImageIDFromTar(envs.ContainerEngine, tarPath)
	if err != nil {
		return flaterrors.Join(err, errBuildingContainer)
	}

	// Tag with version
	tagCmd := exec.Command(envs.ContainerEngine, "tag", imageID, imageWithVersion)
	if err := runCmd(tagCmd, isMCPMode); err != nil {
		return flaterrors.Join(err, errBuildingContainer)
	}

	// Tag with latest
	tagLatestCmd := exec.Command(envs.ContainerEngine, "tag", imageID, imageLatest)
	if err := runCmd(tagLatestCmd, isMCPMode); err != nil {
		return flaterrors.Join(err, errBuildingContainer)
	}

	// Clean up tar file
	if err := os.Remove(tarPath); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "⚠️  Warning: failed to remove tar file: %s\n", err)
	}

	// Add to artifact store
	artifact := forge.Artifact{
		Name:      spec.Name,
		Type:      "container",
		Location:  imageWithVersion, // Local image reference
		Timestamp: timestamp,
		Version:   version,
	}

	forge.AddOrUpdateArtifact(store, artifact)

	_, _ = fmt.Fprintf(
		out,
		"✅ Built container: %s (version: %s)\n",
		spec.Name,
		version,
	)

	return nil
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

// Envs holds the environment variables required by the build-container tool.
type Envs struct {
	// ContainerEngine is the container engine to use for building the container (e.g., docker, podman).
	ContainerEngine string `env:"CONTAINER_ENGINE,required"`
	// BuildArgs is a list of build arguments to pass to the container build command.
	BuildArgs []string `env:"BUILD_ARGS"`
	// KanikoCacheDir is the local directory to use for kaniko layer caching.
	// Defaults to ~/.kaniko-cache
	KanikoCacheDir string `env:"KANIKO_CACHE_DIR"          envDefault:"~/.kaniko-cache"`
}

// ----------------------------------------------------- PRINT HELPERS ----------------------------------------------- //

const usage = `USAGE

CONTAINER_ENGINE=%q %s

Required environment variables:
    CONTAINER_ENGINE    string    Container engine such as podman or docker.

Optional environment variables:
    BUILD_ARGS          []string  List of build args (e.g. "GO_BUILD_LDFLAGS=\"-X main.BuildTimestamp=$(TIMESTAMP)\"").
    KANIKO_CACHE_DIR    string    Local directory for kaniko layer caching (default: ~/.kaniko-cache).

Configuration:
    The tool reads container build specifications from .project.yaml
    Artifacts are written to the path specified in build.artifactStorePath
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
		os.Getenv("CONTAINER_ENGINE"),
		os.Args[0],
	)
}

func printSuccess() {
	fmt.Printf("✅ All containers built successfully\n")
}

func printFailure(err error) {
	fmt.Printf("❌ Error building containers\n%s\n", err.Error())
}
