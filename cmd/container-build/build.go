// Copyright 2024 Alexandre Mahdhaoui
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/alexandremahdhaoui/forge/internal/util"
	"github.com/alexandremahdhaoui/forge/pkg/engineframework"
	"github.com/alexandremahdhaoui/forge/pkg/engineversion"
	"github.com/alexandremahdhaoui/forge/pkg/flaterrors"
	"github.com/alexandremahdhaoui/forge/pkg/forge"
	"github.com/alexandremahdhaoui/forge/pkg/mcptypes"
	"github.com/caarlos0/env/v11"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

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

// ----------------------------------------------------- BUILD ------------------------------------------------------- //

// Build implements the BuildFunc for building container images
func Build(ctx context.Context, input mcptypes.BuildInput, spec *Spec) ([]forge.Artifact, error) {
	log.Printf("Building container: %s from %s", input.Name, input.Src)

	// Note: spec contains typed fields like Dockerfile, Context, BuildArgs, Tags, Target, Push, Registry
	// These can be used in future iterations to enhance build functionality
	// For now, the build uses input.Src as the Dockerfile path and input.Spec for dependsOn parsing
	_ = spec // TODO: Use spec fields for enhanced build configuration

	// Parse environment variables (CONTAINER_BUILD_ENGINE is required)
	envs := Envs{} //nolint:exhaustruct
	if err := env.Parse(&envs); err != nil {
		return nil, fmt.Errorf("environment parse failed: %w (CONTAINER_BUILD_ENGINE required)", err)
	}

	// Validate container engine
	if err := validateContainerEngine(envs.BuildEngine); err != nil {
		return nil, err
	}

	// Resolve build context directory
	// Prefer input.Context (set by forge CLI), fall back to CWD for backward compat
	buildContextDir := input.Context
	if buildContextDir == "" {
		var err error
		buildContextDir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get working directory: %w", err)
		}
	}

	// Create BuildSpec from input (include Spec for dependsOn support)
	buildSpec := forge.BuildSpec{
		Name:   input.Name,
		Src:    input.Src,
		Dest:   input.Dest,
		Engine: input.Engine,
		Spec:   input.Spec,
	}

	// Get git version
	version, err := engineframework.GetGitVersion()
	if err != nil {
		return nil, fmt.Errorf("could not get git version: %w", err)
	}

	// Build the container (isMCPMode=true)
	var store forge.ArtifactStore
	if err := buildContainer(envs, buildSpec, version, "", &store, true, buildContextDir); err != nil {
		return nil, err
	}

	// Retrieve the artifact from store to get dependencies
	location := fmt.Sprintf("%s:%s", input.Name, version)
	artifact := engineframework.CreateCustomArtifact(
		input.Name,
		forge.TypeContainer,
		location,
		version,
	)

	// Find artifact in store to get dependencies
	for _, a := range store.Artifacts {
		if a.Name == input.Name {
			artifact.Dependencies = a.Dependencies
			artifact.DependencyDetectorEngine = a.DependencyDetectorEngine
			artifact.DependencyDetectorSpec = a.DependencyDetectorSpec
			break
		}
	}

	return engineframework.One(artifact), nil
}

// ----------------------------------------------------- CONTAINER BUILD --------------------------------------------- //

var (
	errBuildingContainer      = errors.New("building container")
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

// buildContainer dispatches to the appropriate build function based on container engine.
func buildContainer(
	envs Envs,
	spec forge.BuildSpec,
	version, timestamp string,
	store *forge.ArtifactStore,
	isMCPMode bool,
	contextDir string,
) error {
	// Dispatch based on container engine
	switch envs.BuildEngine {
	case "docker":
		return buildContainerDocker(envs, spec, version, timestamp, store, isMCPMode, contextDir)
	case "kaniko":
		return buildContainerKaniko(envs, spec, version, timestamp, store, isMCPMode, contextDir)
	case "podman":
		return buildContainerPodman(envs, spec, version, timestamp, store, isMCPMode, contextDir)
	default:
		// Should be unreachable due to validation, but defensive programming
		return flaterrors.Join(
			fmt.Errorf("unsupported container engine: %s", envs.BuildEngine),
			errBuildingContainer,
		)
	}
}

// buildContainerDocker builds a container using native docker build.
func buildContainerDocker(
	envs Envs,
	spec forge.BuildSpec,
	version, timestamp string,
	store *forge.ArtifactStore,
	isMCPMode bool,
	contextDir string,
) error {
	out := os.Stdout
	if isMCPMode {
		out = os.Stderr
	}

	printBuildStart(out, spec.Name)

	wd := contextDir

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

	// Detect dependencies after successful build
	var dependencies []forge.ArtifactDependency
	var detectorEngines []string
	var dependsOnSpec []forge.DependsOnSpec

	dependsOn, err := forge.ParseDependsOn(spec.Spec)
	if err != nil {
		// ParseDependsOn error: log and skip detection (don't fail build)
		_, _ = fmt.Fprintf(out, "Warning: failed to parse dependsOn: %v\n", err)
		_, _ = fmt.Fprintf(out, "   Skipping dependency detection\n")
	} else if len(dependsOn) > 0 {
		// Call dependency detection
		dependencies, detectorEngines, err = detectDependenciesFromSpec(dependsOn, spec)
		if err != nil {
			// Detection failed - FAIL the build
			return flaterrors.Join(err, errBuildingContainer)
		}
		dependsOnSpec = dependsOn
	}

	// Add to artifact store
	addArtifactToStore(store, spec.Name, version, timestamp, dependencies, detectorEngines, dependsOnSpec)

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
	contextDir string,
) error {
	out := os.Stdout
	if isMCPMode {
		out = os.Stderr
	}

	printBuildStart(out, spec.Name)

	wd := contextDir

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

	// Detect dependencies after successful build
	var dependencies []forge.ArtifactDependency
	var detectorEngines []string
	var dependsOnSpec []forge.DependsOnSpec

	dependsOn, err := forge.ParseDependsOn(spec.Spec)
	if err != nil {
		// ParseDependsOn error: log and skip detection (don't fail build)
		_, _ = fmt.Fprintf(out, "Warning: failed to parse dependsOn: %v\n", err)
		_, _ = fmt.Fprintf(out, "   Skipping dependency detection\n")
	} else if len(dependsOn) > 0 {
		// Call dependency detection
		dependencies, detectorEngines, err = detectDependenciesFromSpec(dependsOn, spec)
		if err != nil {
			// Detection failed - FAIL the build
			return flaterrors.Join(err, errBuildingContainer)
		}
		dependsOnSpec = dependsOn
	}

	// Add to artifact store
	addArtifactToStore(store, spec.Name, version, timestamp, dependencies, detectorEngines, dependsOnSpec)

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
	contextDir string,
) error {
	// In MCP mode, write to stderr; in normal mode, write to stdout
	out := os.Stdout
	if isMCPMode {
		out = os.Stderr
	}

	printBuildStart(out, spec.Name)

	wd := contextDir

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

	// Load the tar and get the image ID
	tarPath := fmt.Sprintf(".ignore.%s.tar", spec.Name)
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
		_, _ = fmt.Fprintf(os.Stderr, "Warning: failed to remove tar file: %s\n", err)
	}

	// Detect dependencies after successful build
	var dependencies []forge.ArtifactDependency
	var detectorEngines []string
	var dependsOnSpec []forge.DependsOnSpec

	dependsOn, err := forge.ParseDependsOn(spec.Spec)
	if err != nil {
		// ParseDependsOn error: log and skip detection (don't fail build)
		_, _ = fmt.Fprintf(out, "Warning: failed to parse dependsOn: %v\n", err)
		_, _ = fmt.Fprintf(out, "   Skipping dependency detection\n")
	} else if len(dependsOn) > 0 {
		// Call dependency detection
		dependencies, detectorEngines, err = detectDependenciesFromSpec(dependsOn, spec)
		if err != nil {
			// Detection failed - FAIL the build
			return flaterrors.Join(err, errBuildingContainer)
		}
		dependsOnSpec = dependsOn
	}

	// Add to artifact store
	addArtifactToStore(store, spec.Name, version, timestamp, dependencies, detectorEngines, dependsOnSpec)

	printBuildSuccess(out, spec.Name, version)

	return nil
}

// ----------------------------------------------------- HELPERS ----------------------------------------------------- //

// tagImage tags an image with a specific tag.
func tagImage(containerEngine, imageID, tag string, isMCPMode bool) error {
	cmd := exec.Command(containerEngine, "tag", imageID, tag)
	return runCmd(cmd, isMCPMode)
}

// addArtifactToStore adds a container artifact to the store.
func addArtifactToStore(
	store *forge.ArtifactStore,
	name, version, timestamp string,
	dependencies []forge.ArtifactDependency,
	detectorEngines []string,
	dependsOnSpec []forge.DependsOnSpec,
) {
	artifact := forge.Artifact{
		Name:         name,
		Type:         forge.TypeContainer,
		Location:     fmt.Sprintf("%s:%s", name, version),
		Timestamp:    timestamp,
		Version:      version,
		Dependencies: dependencies,
	}

	// Store detector engines as comma-separated string
	if len(detectorEngines) > 0 {
		detectorEngineStr := ""
		for i, engine := range detectorEngines {
			if i > 0 {
				detectorEngineStr += ","
			}
			detectorEngineStr += engine
		}
		artifact.DependencyDetectorEngine = detectorEngineStr
	}

	// Store dependsOn configuration as spec
	if len(dependsOnSpec) > 0 {
		artifact.DependencyDetectorSpec = make(map[string]interface{})
		artifact.DependencyDetectorSpec["dependsOn"] = dependsOnSpec
	}

	forge.AddOrUpdateArtifact(store, artifact)
}

// printBuildStart prints build start message.
func printBuildStart(out io.Writer, name string) {
	_, _ = fmt.Fprintf(out, "Building container: %s\n", name)
}

// printBuildSuccess prints build success message.
func printBuildSuccess(out io.Writer, name, version string) {
	_, _ = fmt.Fprintf(out, "Built container: %s (version: %s)\n", name, version)
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

// ----------------------------------------------------- DEPENDENCY DETECTION ---------------------------------------- //

// detectDependenciesFromSpec detects dependencies by calling all configured dependency detectors.
//
// Error handling strategy:
//   - Detector not found: logs warning, skips that detector (graceful degradation)
//   - Detector found but fails: returns error after 1 retry (fail build)
//   - ParseDependsOn error: caller should log and skip detection
//   - Empty dependsOn: returns (nil, nil)
//
// Returns aggregated and deduplicated dependencies from all detectors.
func detectDependenciesFromSpec(dependsOn []forge.DependsOnSpec, spec forge.BuildSpec) ([]forge.ArtifactDependency, []string, error) {
	if len(dependsOn) == 0 {
		// No detectors configured - not an error
		return nil, nil, nil
	}

	var allDeps []forge.ArtifactDependency
	var detectorEngines []string

	for i, detectorSpec := range dependsOn {
		log.Printf("Running dependency detector %d/%d: %s", i+1, len(dependsOn), detectorSpec.Engine)

		// Step 1: Resolve engine URI to command and args using go run pattern
		// Use GetEffectiveVersion to handle both ldflags version and go run @version
		cmd, args, err := engineframework.ResolveDetector(detectorSpec.Engine, engineversion.GetEffectiveVersion(Version))
		if err != nil {
			// Detector resolution failed - graceful degradation for this detector
			log.Printf("Dependency detector resolution failed: %v", err)
			log.Printf("   Skipping detector %s (continuing with others)", detectorSpec.Engine)
			continue
		}

		log.Printf("Resolved dependency detector: %s %v", cmd, args)

		// Step 2: Call detector with retry logic
		dependencies, err := callDependencyDetectorForContainer(cmd, args, spec, detectorSpec.Spec)
		if err != nil {
			// First retry
			log.Printf("Dependency detection failed (attempt 1/2): %v", err)
			log.Printf("   Retrying after 100ms...")
			time.Sleep(100 * time.Millisecond)

			dependencies, err = callDependencyDetectorForContainer(cmd, args, spec, detectorSpec.Spec)
			if err != nil {
				// Second failure - FAIL the build
				return nil, nil, fmt.Errorf("dependency detection failed after retry for %s: %w", detectorSpec.Engine, err)
			}
		}

		// Step 3: Convert mcptypes.Dependency to forge.ArtifactDependency
		for _, dep := range dependencies {
			artifactDep := forge.ArtifactDependency{
				Type:            dep.Type,
				FilePath:        dep.FilePath,
				ExternalPackage: dep.ExternalPackage,
				Timestamp:       dep.Timestamp,
				Semver:          dep.Semver,
			}
			allDeps = append(allDeps, artifactDep)
		}

		// Track this detector engine
		detectorEngines = append(detectorEngines, detectorSpec.Engine)

		log.Printf("Detected %d dependencies from %s", len(dependencies), detectorSpec.Engine)
	}

	// Step 4: Deduplicate dependencies
	deduplicated := deduplicateDependencies(allDeps)

	log.Printf("Total: %d unique dependencies from %d detectors", len(deduplicated), len(detectorEngines))

	return deduplicated, detectorEngines, nil
}

// getStringField extracts a string field from a map, checking multiple key names in order.
// Returns empty string if none of the keys are present or if the value is not a string.
func getStringField(m map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if v, ok := m[key].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

// deduplicateDependencies removes duplicate dependencies based on (Type, FilePath, ExternalPackage).
func deduplicateDependencies(deps []forge.ArtifactDependency) []forge.ArtifactDependency {
	seen := make(map[string]bool)
	result := make([]forge.ArtifactDependency, 0, len(deps))

	for _, dep := range deps {
		// Create unique key based on type and identifier
		var key string
		if dep.Type == "file" {
			key = fmt.Sprintf("file:%s", dep.FilePath)
		} else {
			key = fmt.Sprintf("external:%s", dep.ExternalPackage)
		}

		if !seen[key] {
			seen[key] = true
			result = append(result, dep)
		}
	}

	return result
}

// callDependencyDetectorForContainer calls a dependency detector MCP server for container builds.
// For containers, we pass the Containerfile path as the file to analyze.
func callDependencyDetectorForContainer(cmdName string, cmdArgs []string, spec forge.BuildSpec, detectorSpec map[string]interface{}) ([]mcptypes.Dependency, error) {
	// Create command to spawn MCP server (append --mcp flag)
	execCmd := exec.Command(cmdName, append(cmdArgs, "--mcp")...)
	execCmd.Env = os.Environ()
	execCmd.Stderr = os.Stderr // Forward logs

	// Create MCP client
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "container-build-client",
		Version: "v1.0.0",
	}, nil)

	// Create command transport
	transport := &mcp.CommandTransport{
		Command: execCmd,
	}

	// Connect to the MCP server
	ctx := context.Background()
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to dependency detector: %w", err)
	}
	defer func() { _ = session.Close() }()

	// Extract detector configuration from detectorSpec
	// The detectorSpec contains the user's dependsOn configuration with the actual
	// source file and function to analyze (e.g., a Go main.go file, not the Containerfile)
	//
	// Note: Check both camelCase (MCP protocol) and lowercase (YAML convention) for compatibility
	filePath := getStringField(detectorSpec, "filePath", "filepath")
	if filePath == "" {
		return nil, fmt.Errorf("detector spec missing required 'filepath' or 'filePath' field")
	}
	funcName := getStringField(detectorSpec, "funcName")
	if funcName == "" {
		return nil, fmt.Errorf("detector spec missing required 'funcName' field")
	}

	input := map[string]any{
		"filePath": filePath,
		"funcName": funcName,
		"spec":     detectorSpec,
	}

	// Call the detectDependencies tool
	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "detectDependencies",
		Arguments: input,
	})
	if err != nil {
		return nil, fmt.Errorf("MCP tool call failed: %w", err)
	}

	// Check if result indicates an error
	if result.IsError {
		errMsg := "unknown error"
		if len(result.Content) > 0 {
			if textContent, ok := result.Content[0].(*mcp.TextContent); ok {
				errMsg = textContent.Text
			}
		}
		return nil, fmt.Errorf("dependency detection failed: %s", errMsg)
	}

	// Parse structured content
	if result.StructuredContent == nil {
		return nil, fmt.Errorf("no structured content returned from detector")
	}

	// Convert structured content to DetectDependenciesOutput
	var output mcptypes.DetectDependenciesOutput
	jsonBytes, err := json.Marshal(result.StructuredContent)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal detector output: %w", err)
	}

	if err := json.Unmarshal(jsonBytes, &output); err != nil {
		return nil, fmt.Errorf("failed to unmarshal detector output: %w", err)
	}

	return output.Dependencies, nil
}
