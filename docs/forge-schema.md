# forge.yaml Schema Documentation

This document provides comprehensive documentation for the `forge.yaml` configuration file, which is the central configuration for the forge build orchestrator.

## Table of Contents

- [Overview](#overview)
- [File Location](#file-location)
- [Root Schema](#root-schema)
- [Build Configuration](#build-configuration)
- [BuildSpec Specification](#buildspec-specification)
- [Engine Protocol](#engine-protocol)
- [Test Configuration](#test-configuration)
- [TestSpec Specification](#testspec-specification)
- [Complete Example](#complete-example)
- [Artifact Store Schema](#artifact-store-schema)

## Overview

The `forge.yaml` file defines:
- **Build artifacts** to be created (binaries, containers)
- **Build engines** to use for each artifact
- **Test stages** and environments (unit, integration, e2e)
- **Artifact and test environment tracking** configuration

## File Location

**Path:** `forge.yaml` (repository root)

**Format:** YAML

**Version:** 1.0

## Root Schema

```yaml
name: string                              # Project name
artifactStorePath: string                 # Artifact store path
engines: []EngineConfig                   # Engine configurations (optional)
build: []BuildSpec                        # Build configuration
test: []TestSpec                          # Test stages configuration
oapiCodegenHelper: OAPICodegenHelper      # OpenAPI codegen configuration (optional)
```

### Root Fields

#### `name` (string, required)

Project name used for identification.

**Example:**
```yaml
name: tooling
```

#### `artifactStorePath` (string, required)

Path to the artifact store YAML file where forge tracks built artifacts, test environments, and metadata.

**Default:** `.ignore.artifact-store.yaml`

**Example:**
```yaml
artifactStorePath: .ignore.artifact-store.yaml
```

#### `engines` (array of EngineConfig, optional)

Custom engine configurations with aliases. Allows you to create reusable engine configurations with custom parameters.

**Engine Types:**
- `builder` - Multi-step build orchestration
- `test-runner` - Multi-suite test orchestration
- `testenv` - Test environment setup

**Multi-Engine Orchestration:**

Forge supports executing multiple engines sequentially within a single alias. This enables:
- **Sequential Execution**: Engines run in order, one after another
- **Fail-Fast**: Stops on first failure
- **Result Aggregation**: Combines outputs (artifacts for builders, test reports for test-runners)
- **Config Injection**: Each engine gets its own `spec` configuration

**Example: Multi-Step Builder**
```yaml
engines:
  - alias: generate-all
    type: builder
    builder:
      - engine: "go://generic-builder"
        spec:
          command: "go"
          args: ["mod", "tidy"]
      - engine: "go://generic-builder"
        spec:
          command: "go"
          args: ["generate", "./..."]
      - engine: "go://generic-builder"
        spec:
          command: "controller-gen"
          args: ["object:headerFile=./hack/boilerplate.go.txt", "paths=./..."]
```

**Example: Multi-Suite Test Runner**
```yaml
engines:
  - alias: comprehensive-tests
    type: test-runner
    testRunner:
      - engine: "go://test-runner-go"
        spec:
          args: ["-tags=unit"]
      - engine: "go://test-runner-go-verify-tags"
      - engine: "go://lint-go"
```

**Example: Multi-Step Test Environment**
```yaml
engines:
  - alias: setup-integration
    type: testenv
    testenv:
      - engine: "go://testenv-kind"
      - engine: "go://testenv-lcr"
        spec:
          enabled: true
          autoPushImages: true
```

**Usage:**
```yaml
build:
  - name: generated-code
    src: .
    dest: .
    engine: alias://generate-all

test:
  - name: comprehensive
    runner: alias://comprehensive-tests
  - name: integration
    testenv: alias://setup-integration
    runner: go://test-runner-go
```

#### `build` (array of BuildSpec, required)

Build configuration defining all artifacts to build. See [Build Configuration](#build-configuration).

#### `test` (array of TestSpec, optional)

Test stages configuration. See [Test Configuration](#test-configuration).

**Example:**
```yaml
test:
  - name: unit
    runner: "go://test-runner-go"
  - name: integration
    testenv: "alias://setup-integration"
    runner: "go://test-runner-go"
```

#### `oapiCodegenHelper` (OAPICodegenHelper, optional)

OpenAPI code generation helper configuration.

**Example:**
```yaml
oapiCodegenHelper: {}
```

## Build Configuration

The `build` section defines which artifacts to build and how to track them.

### Schema

```yaml
build:
  artifactStorePath: string    # Path to artifact store file
  specs:                       # Array of BuildSpec objects
    - name: string
      src: string
      dest: string
      builder: string
```

### Fields

#### `artifactStorePath` (string, required)

Path to the artifact store YAML file where forge tracks built artifacts with metadata.

**Default:** `.ignore.artifact-store.yaml`

**Example:**
```yaml
build:
  artifactStorePath: .ignore.artifact-store.yaml
```

The artifact store file is automatically created and managed by forge. It contains:
- Artifact name and type
- Build version (from git)
- Build timestamp
- Artifact location

**Automatic Pruning:** The artifact store automatically retains only the 3 most recent build artifacts for each unique `type:name` combination. Older artifacts are automatically removed when the store is updated. This prevents unbounded growth while maintaining recent build history. Test environments are NOT pruned and retain all historical data.

#### `specs` (array of BuildSpec, required)

List of artifacts to build. Each entry follows the [BuildSpec specification](#buildspec-specification).

**Example:**
```yaml
build:
  artifactStorePath: .ignore.artifact-store.yaml
  specs:
    - name: my-app
      src: ./cmd/my-app
      dest: ./build/bin
      builder: go://build-go

    - name: my-container
      src: ./containers/my-app/Containerfile
      builder: go://build-container
```

## BuildSpec Specification

The `BuildSpec` defines a single artifact to build.

### Schema

```yaml
name: string      # Artifact identifier
src: string       # Source path
dest: string      # Destination path (optional for containers)
builder: string   # Engine URI
```

### Fields

#### `name` (string, required)

Unique identifier for the artifact. Used as:
- Binary filename (for Go binaries)
- Image name (for containers)
- Artifact store key

**Naming Rules:**
- Must be unique within the forge.yaml
- Should be lowercase with hyphens (e.g., `my-app`, `build-container`)
- No spaces or special characters

**Examples:**
```yaml
name: forge              # Binary: ./build/bin/forge
name: build-go           # Binary: ./build/bin/build-go
name: my-api-server      # Image: localhost:5000/my-api-server:v1.0.0
```

#### `src` (string, required)

Source location for the artifact.

**For Go Binaries:**
- Path to Go package/directory containing `main.go`
- Relative to repository root
- Must start with `./`

**For Container Images:**
- Path to Containerfile/Dockerfile
- Must end with `Containerfile` or `Dockerfile`
- Relative to repository root

**Examples:**
```yaml
# Go binary
src: ./cmd/my-app           # Directory containing main.go

# Container image
src: ./containers/my-app/Containerfile   # Containerfile path
```

#### `dest` (string, optional)

Destination directory for the built artifact.

**For Go Binaries (required):**
- Directory where binary will be placed
- Binary name will be the `name` field value
- Relative to repository root

**For Container Images (optional):**
- Can be omitted (images are tagged and pushed)
- If provided, used as registry prefix

**Examples:**
```yaml
# Go binary
dest: ./build/bin           # Creates: ./build/bin/<name>

# Container image (optional)
dest: localhost:5000        # Tags: localhost:5000/<name>:<version>
```

#### `builder` (string, required)

Engine URI specifying which build engine to use.

**Format:** `<protocol>://<engine-name>`

**Supported Engines:**
- `go://build-go` - Build Go binaries
- `go://build-container` - Build container images

See [Engine Protocol](#engine-protocol) for details.

**Examples:**
```yaml
# Build Go binary
builder: go://build-go

# Build container image
builder: go://build-container
```

### Complete BuildSpec Examples

#### Go Binary

```yaml
- name: my-cli-tool
  src: ./cmd/my-cli-tool
  dest: ./build/bin
  builder: go://build-go
```

**Results in:**
- Binary: `./build/bin/my-cli-tool`
- Artifact type: `binary`
- Tracked in artifact store

#### Container Image

```yaml
- name: my-api
  src: ./containers/my-api/Containerfile
  builder: go://build-container
```

**Results in:**
- Image: Tagged with project version
- Artifact type: `container`
- Tracked in artifact store
- Available for push to registry

## Engine Protocol

Build engines use the `go://` protocol to reference MCP servers.

### Protocol Format

```
go://<binary-name>
```

**Components:**
- `go://` - Protocol identifier (indicates MCP server)
- `<binary-name>` - Name of the MCP server binary

### Engine Resolution

When forge encounters an engine URI like `go://build-go@v1.0.0`:

1. **URI Parsing:** Extracts engine name and version from `go://<name>[@<version>]`
2. **Short Name Expansion:** Expands short names to full paths
   - `go://build-go@v1.0.0` → `github.com/alexandremahdhaoui/forge/cmd/build-go@v1.0.0`
   - `go://build-container` → `github.com/alexandremahdhaoui/forge/cmd/build-container@latest`
3. **Binary Check:** Looks for binary in PATH (from previous `go install`)
4. **Auto-Install:** If not found, runs `go install <full-path@version>`
5. **MCP Mode:** Invokes with `--mcp` flag
6. **Communication:** Uses stdio JSON-RPC 2.0 protocol

**Note:** Engines are automatically installed on first use. No manual installation required.

### Available Engines

#### build-go

**URI:** `go://build-go`

**Purpose:** Build Go binaries with version metadata injection

**Required BuildSpec Fields:**
- `name` - Binary name
- `src` - Go package path
- `dest` - Output directory
- `builder: go://build-go`

**Environment Variables:**
- `GO_BUILD_LDFLAGS` - Additional linker flags

**Example:**
```yaml
- name: my-app
  src: ./cmd/my-app
  dest: ./build/bin
  builder: go://build-go
```

**Build Command:**
```bash
GO_BUILD_LDFLAGS="-X main.Version=v1.0.0" forge build
```

#### build-container

**URI:** `go://build-container`

**Purpose:** Build container images using Kaniko (rootless, secure)

**Required BuildSpec Fields:**
- `name` - Image name
- `src` - Path to Containerfile
- `builder: go://build-container`

**Optional BuildSpec Fields:**
- `dest` - Registry prefix (default: uses local tagging)

**Environment Variables:**
- `CONTAINER_ENGINE` - Container engine (docker/podman)
- `PREPEND_CMD` - Command prefix (e.g., `sudo`)

**Example:**
```yaml
- name: my-api
  src: ./containers/my-api/Containerfile
  dest: localhost:5000
  builder: go://build-container
```

**Build Command:**
```bash
CONTAINER_ENGINE=docker forge build
```

### Custom Engines

To create a custom build engine:

1. **Implement MCP Server:**
   - Accept `--mcp` flag
   - Implement stdio JSON-RPC 2.0 protocol
   - Register `build` tool with BuildSpec schema

2. **Tool Registration:**
```go
tool := mcp.Tool{
    Name: "build",
    InputSchema: mcp.ToolInputSchema{
        Type: "object",
        Properties: map[string]interface{}{
            "name":    map[string]string{"type": "string"},
            "src":     map[string]string{"type": "string"},
            "dest":    map[string]string{"type": "string"},
            "builder": map[string]string{"type": "string"},
        },
        Required: []string{"name", "src", "builder"},
    },
}
```

3. **Update forge.yaml:**
```yaml
- name: my-artifact
  src: ./source
  dest: ./output
  builder: go://my-custom-engine
```

## Test Configuration

The `test` section defines test stages with their environments and runners.

### Schema

```yaml
test:
  - name: string       # Test stage name
    testenv: string    # Test environment engine (optional)
    runner: string     # Test runner engine
```

### Fields

#### Test Array (array of TestSpec, optional)

List of test stages. Each stage can have its own environment and runner.

**Example:**
```yaml
test:
  - name: unit
    runner: "go://test-runner-go"

  - name: integration
    testenv: "alias://setup-integration"
    runner: "go://test-runner-go"

  - name: e2e
    runner: "go://forge-e2e"

  - name: lint
    runner: "go://lint-go"
```

## TestSpec Specification

The `TestSpec` defines a single test stage.

### Schema

```yaml
name: string      # Stage identifier
testenv: string   # Environment engine URI (optional)
runner: string    # Test runner engine URI
```

### Fields

#### `name` (string, required)

Test stage identifier. Used in commands like `forge test <name> run`.

**Common Names:**
- `unit` - Unit tests
- `integration` - Integration tests requiring test environment
- `e2e` - End-to-end tests
- `lint` - Code linting

**Example:**
```yaml
name: integration
```

#### `testenv` (string, optional)

Test environment engine URI. Omit this field for tests that don't need an environment (like unit tests and linting).

**Format:** `<protocol>://<engine-name>` or `alias://<alias-name>`

**Available Engines:**
- `"go://testenv"` - Complete test environment (Kind cluster + registry + helm)
- `"go://testenv-kind"` - Kind cluster only
- `"go://testenv-lcr"` - Local container registry only
- `"alias://<name>"` - Custom engine alias from engines section

**Example:**
```yaml
# No environment needed (omit testenv field)
name: unit
runner: "go://test-runner-go"

# Full test environment with cluster
name: integration
testenv: "go://testenv"
runner: "go://test-runner-go"

# Custom environment alias
name: integration
testenv: "alias://setup-integration"
runner: "go://test-runner-go"
```

#### `runner` (string, required)

Test runner engine URI specifying which test runner to use.

**Format:** `<protocol>://<runner-name>`

**Available Runners:**
- `"go://test-runner-go"` - Go test runner with coverage and JUnit reports
- `"go://test-runner-go-verify-tags"` - Verify all test files have build tags
- `"go://generic-test-runner"` - Execute arbitrary commands as tests
- `"go://lint-go"` - Golangci-lint runner
- `"go://forge-e2e"` - Forge end-to-end test runner

**Example:**
```yaml
# Run Go tests
runner: "go://test-runner-go"

# Verify build tags
runner: "go://test-runner-go-verify-tags"

# Run linter
runner: "go://lint-go"

# Execute custom commands
runner: "go://generic-test-runner"
```

### Complete TestSpec Examples

#### Unit Tests (No Environment)

```yaml
- name: unit
  runner: "go://test-runner-go"
```

**Usage:**
```bash
forge test unit run
```

**What happens:**
- No environment created
- Runs Go tests with `-tags=unit`
- Generates JUnit XML and coverage report

#### Integration Tests (With Environment)

```yaml
- name: integration
  testenv: "alias://setup-integration"
  runner: "go://test-runner-go"
```

**Usage:**
```bash
forge test integration create  # Create environment
forge test integration run     # Run tests
forge test integration delete  # Delete environment
```

**What happens:**
- Creates Kind cluster via testenv-kind
- Sets up local registry via testenv-lcr (if configured)
- Runs Go tests with `-tags=integration`
- Environment persists until deleted

#### Linting Stage

```yaml
- name: lint
  runner: "go://lint-go"
```

**Usage:**
```bash
forge test lint run
```

**What happens:**
- No environment created
- Runs golangci-lint with --fix flag
- Returns test report with pass/fail

## Complete Example

Here's a complete `forge.yaml` example with all sections:

```yaml
# Project name
name: my-project

# Path to artifact store
artifactStorePath: .ignore.artifact-store.yaml

# Custom engine configurations (optional)
engines:
  - alias: setup-integration
    type: testenv
    testenv:
      - engine: "go://testenv-kind"
      - engine: "go://testenv-lcr"
        spec:
          enabled: true
          autoPushImages: true

# Build configuration
build:
  # CLI tools
  - name: my-cli
    src: ./cmd/my-cli
    dest: ./build/bin
    engine: go://build-go

  - name: api-server
    src: ./cmd/api-server
    dest: ./build/bin
    engine: go://build-go

  # Build tools (self-hosting)
  - name: build-go
    src: ./cmd/build-go
    dest: ./build/bin
    engine: go://build-go

  - name: build-container
    src: ./cmd/build-container
    dest: ./build/bin
    engine: go://build-go

  # Container images
  - name: api-server-image
    src: ./containers/api-server/Containerfile
    dest: localhost:5000
    engine: go://build-container

  - name: worker
    src: ./containers/worker/Containerfile
    engine: go://build-container

# Test stages configuration
test:
  # Verify build tags
  - name: verify-tags
    runner: "go://test-runner-go-verify-tags"

  # Unit tests - no environment needed
  - name: unit
    runner: "go://test-runner-go"

  # Integration tests - full test environment
  - name: integration
    testenv: "alias://setup-integration"
    runner: "go://test-runner-go"

  # E2E tests
  - name: e2e
    runner: "go://forge-e2e"

  # Linting
  - name: lint
    runner: "go://lint-go"

# OpenAPI code generation (optional)
oapiCodegenHelper: {}
```

## Artifact Store Schema

The artifact store file is automatically managed by forge. Here's the schema for reference.

### Location

Defined by `build.artifactStorePath` in forge.yaml

**Default:** `.ignore.artifact-store.yaml`

### Schema

```yaml
version: string       # Artifact store version (always "1.0")
lastUpdated: string   # ISO 8601 timestamp of last update
artifacts:
  - name: string        # Artifact identifier
    type: string        # "binary", "container", or "formatted"
    location: string    # File path or image reference
    timestamp: string   # ISO 8601 timestamp
    version: string     # Git version (commit hash + dirty flag)
testEnvironments:     # Test environment tracking (not pruned)
  <env-id>:
    id: string
    name: string
    status: string
    createdAt: string
    updatedAt: string
```

### Automatic Pruning

The artifact store implements automatic pruning to prevent unbounded growth:

- **Build Artifacts:** Only the **3 most recent** artifacts are retained for each unique `type:name` combination
- **Pruning Trigger:** Automatic on every `WriteArtifactStore()` call
- **Sorting:** By timestamp (RFC3339 format), newest first
- **Test Data:** Test environments are **NOT pruned** - all test history is retained

**Example:**
- If you build `binary:forge` 5 times, only the 3 most recent builds are kept
- Each unique `type:name` pair (e.g., `binary:forge`, `container:api-server`) is pruned independently
- Invalid timestamps are handled gracefully (kept at end of list)

### Example

```yaml
artifacts:
  - name: my-cli
    type: binary
    location: file://./build/bin/my-cli
    timestamp: "2025-01-03T10:30:00Z"
    version: v1.2.3-abc1234

  - name: api-server
    type: container
    location: localhost:5000/api-server:v1.2.3-abc1234
    timestamp: "2025-01-03T10:31:00Z"
    version: v1.2.3-abc1234
```

### Fields

#### `name` (string)

Artifact identifier matching the BuildSpec name.

#### `type` (string)

Artifact type:
- `binary` - Go executable
- `container` - Container image

#### `location` (string)

Artifact location:
- **For binaries:** `file://<path>` (e.g., `file://./build/bin/my-cli`)
- **For containers:** Image reference (e.g., `localhost:5000/my-api:v1.0.0`)

#### `timestamp` (string)

ISO 8601 formatted build timestamp.

#### `version` (string)

Git-based version string:
- Format: `v<tag>-<commit>` or `<commit>-dirty`
- Includes dirty flag if uncommitted changes exist
- Used for image tags and version metadata

## Best Practices

### 1. Naming Conventions

```yaml
# Good: lowercase with hyphens
- name: api-server
- name: worker-process
- name: cli-tool

# Avoid: mixed case, underscores
- name: APIServer       # Bad
- name: worker_process  # Bad
```

### 2. Source Paths

```yaml
# Good: relative paths from repo root
src: ./cmd/my-app
src: ./containers/my-app/Containerfile

# Avoid: absolute paths
src: /home/user/project/cmd/my-app  # Bad
```

### 3. Destination Consistency

```yaml
# Good: consistent output directory
dest: ./build/bin

# Good: consistent registry prefix
dest: localhost:5000
```

### 4. Engine Selection

```yaml
# Go binaries: use build-go
- name: my-binary
  builder: go://build-go

# Container images: use build-container
- name: my-image
  builder: go://build-container
```

### 5. Self-Hosting

Build tools should build themselves:

```yaml
build:
  specs:
    # forge builds itself
    - name: forge
      src: ./cmd/forge
      dest: ./build/bin
      builder: go://build-go

    # build-go builds itself
    - name: build-go
      src: ./cmd/build-go
      dest: ./build/bin
      builder: go://build-go
```

### 6. File Ignoring

Add forge directory and build outputs to `.gitignore`:

```gitignore
# Forge artifacts and test environments
.forge/

# Build outputs
build/
```

## Validation

To validate your `forge.yaml`:

```bash
# Try building
forge build

# Check for syntax errors
yq eval . forge.yaml

# Verify all source paths exist
# (forge will validate this during build)
```

## Migration from .project.yaml

If migrating from `.project.yaml`:

1. **Rename file:**
   ```bash
   mv .project.yaml forge.yaml
   ```

2. **Update structure:**
   - Add `build.specs` array
   - Convert old build config to BuildSpec format
   - Update engine references to use `go://` protocol

3. **Update references:**
   - Update documentation
   - Update CI/CD scripts
   - Update README

## Troubleshooting

### Build Fails: "engine not found"

**Problem:** Forge cannot find the specified engine binary.

**Solution:**

Engines are automatically installed on first use. If you still encounter issues:
```bash
# Ensure GOBIN is in your PATH
GOBIN_PATH=$(go env GOBIN)
if [ -z "$GOBIN_PATH" ]; then
  GOBIN_PATH=$(go env GOPATH)/bin
fi
export PATH="$GOBIN_PATH:$PATH"

# Run forge build again
forge build
```

**Note:** Forge automatically runs `go install` for missing engines.

### Artifact Store Errors

**Problem:** Artifact store file is corrupted.

**Solution:**
```bash
# Delete and rebuild
rm .forge/artifacts.yaml
forge build
```

### Test Environment Issues

**Problem:** Cannot create test environment.

**Solution:**
```bash
# Check if Kind is installed
kind version

# Check Docker/Podman is running
docker info

# Try creating test environment
forge test integration create

# View environment details
forge test integration list
```

## References

- [forge CLI Usage Guide](./forge-usage.md)
- [ARCHITECTURE.md - Forge Architecture](../ARCHITECTURE.md#forge-architecture)
- [Model Context Protocol Specification](https://modelcontextprotocol.io)
