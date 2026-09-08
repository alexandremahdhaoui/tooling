# forge.yaml Schema Documentation

This document provides comprehensive documentation for the `forge.yaml` configuration file, which is the central configuration for the forge build orchestrator.

## Table of Contents

- [Overview](#overview)
- [File Location](#file-location)
- [Root Schema](#root-schema)
- [Build Configuration](#build-configuration)
- [BuildSpec Specification](#buildspec-specification)
- [Lazy Rebuild](#lazy-rebuild)
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

#### `frozen` (bool, optional)

Whether a build reads the recorded dependency locks strictly and never
repairs them. Absent means `true`: a build is a real build unless the repo
says otherwise, and a stale lock fails it instead of self-healing into bytes
nobody can reproduce. Regenerating a lock is `forge-factory lock`, never a
build. Every build engine holds the invariant that a build writes no
lockfile; the setting reaches only the engines that declare
`capabilities.frozen` in their `forge-dev.yaml` (go-build, generic-builder,
parallel-builder), where it decides whether the lock is proved before
compiling. There is no flag: the repo declares it once.

**Example:**
```yaml
frozen: false   # a scratch repo whose go.sum is allowed to lag
```

#### Engines have no top-level keys

Every engine is configured under the entry that names it. `forge://testenv-lcr`
and `forge://testenv-kind` once had first-class keys at the top of the file
(`localContainerRegistry`, `kindenv`); both are refused by name now, and the
same settings live on the testenv entry's `spec`.

**Usage in Test Environments:**

Typically, you override these settings in your testenv engine configuration rather than setting them at the root level:

```yaml
engines:
  - alias: setup-integration
    type: testenv
    testenv:
      - engine: "forge://testenv-kind"
      - engine: "forge://testenv-lcr"
        spec:
          enabled: true  # Enable for this test environment
          autoPushImages: true
          imagePullSecretNamespaces:
            - default
            - my-app-namespace
```

**See also:** `cmd/testenv-lcr/MCP.md` for detailed testenv-lcr engine documentation.

#### `engines` (array of EngineConfig, optional)

Custom engine configurations with aliases. Allows you to create reusable engine configurations with custom parameters.

**Engine Types:**
- `builder` - Multi-step build orchestration
- `test-runner` - Multi-suite test orchestration
- `testenv` - Test environment setup
- `dependency-detector` - Dependency detection for lazy rebuild

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
      - engine: "forge://generic-builder"
        spec:
          command: "go"
          args: ["mod", "tidy"]
      - engine: "forge://generic-builder"
        spec:
          command: "go"
          args: ["generate", "./..."]
      - engine: "forge://generic-builder"
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
      - engine: "forge://go-test"
        spec:
          args: ["-tags=unit"]
      - engine: "forge://go-lint-tags"
      - engine: "forge://go-lint"
```

**Example: Multi-Step Test Environment**
```yaml
engines:
  - alias: setup-integration
    type: testenv
    testenv:
      - engine: "forge://testenv-kind"
      - engine: "forge://testenv-lcr"
        spec:
          enabled: true
          autoPushImages: true
```

**TestenvEngineSpec Fields:**

Each testenv sub-engine configuration supports these fields:

- `engine` (string, required) - Engine URI (e.g., `forge://testenv-kind`)
- `deferTemplates` (boolean, optional, default: `false`) - When `true`, forge skips template expansion for this engine's `spec`. The spec is passed verbatim to the sub-engine, allowing it to perform its own template expansion with a richer context (e.g., access to `.Networks`, `.Keys`, or other sub-engine-specific variables).
- `spec` (map, optional) - Engine-specific configuration

**Example: Mixed Template Handling**
```yaml
engines:
  - alias: setup-integration
    type: testenv
    testenv:
      # Engine using forge template expansion (default)
      - engine: "forge://testenv-kind"
        spec:
          clusterName: "{{.Env.CLUSTER_NAME}}"  # Expanded by forge

      # Engine handling its own templates
      - engine: "forge://testenv-vm"
        deferTemplates: true  # Skip forge expansion
        spec:
          cloudInit: |
            {{- range .Networks }}  # Expanded by testenv-vm, not forge
            network: {{ .Name }}
            {{- end }}
```

**When to use `deferTemplates: true`:**
- Sub-engine has its own template system with variables forge does not know about
- Template syntax conflicts with forge's Go template expansion
- Sub-engine needs access to runtime context not available during forge execution

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
    runner: forge://go-test
```

#### `build` (array of BuildSpec, required)

Build configuration defining all artifacts to build. See [Build Configuration](#build-configuration).

#### `test` (array of TestSpec, optional)

Test stages configuration. See [Test Configuration](#test-configuration).

**Example:**
```yaml
test:
  - name: unit
    runner: "forge://go-test"
  - name: integration
    testenv: "alias://setup-integration"
    runner: "forge://go-test"
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
      builder: forge://go-build

    - name: my-container
      src: ./containers/my-app/Containerfile
      builder: forge://container-build
```

## BuildSpec Specification

The `BuildSpec` defines a single artifact to build.

### Schema

```yaml
name: string                     # Artifact identifier
src: string                      # Source path
dest: string                     # Destination path (optional for containers)
engine: string                   # Engine URI
spec:                            # Engine-specific configuration (optional)
  args: []string                 # Custom build arguments
  env: map[string]string         # Environment variables
  # ... other engine-specific fields
```

### Fields

#### `name` (string, required)

Unique identifier for the artifact. Used as:
- Binary filename (for Go binaries)
- Image name (for containers)
- Artifact store key

**Naming Rules:**
- Must be unique within the forge.yaml
- Should be lowercase with hyphens (e.g., `my-app`, `container-build`)
- No spaces or special characters

**Examples:**
```yaml
name: forge              # Binary: ./build/bin/forge
name: go-build           # Binary: ./build/bin/go-build
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

#### `engine` (string, required)

Engine URI specifying which build engine to use.

**Format:** `<protocol>://<engine-name>`

**Supported Engines:**
- `forge://go-build` - Build Go binaries with automatic dependency tracking
- `forge://container-build` - Build container images
- `forge://go-dependency-detector` - Detect Go dependencies (used internally)
- `forge://generic-builder` - Execute any command as a build step
- `alias://<alias-name>` - Custom engine alias defined in `engines` section

See [Engine Protocol](#engine-protocol) for details.

**Examples:**
```yaml
# Build Go binary
engine: forge://go-build

# Build container image
engine: forge://container-build

# Use custom engine alias
engine: alias://my-custom-builder
```

#### `platforms` (array of string, optional)

The os/arch pairs this artifact builds for, e.g. `["linux/amd64",
"linux/arm64"]`. Every build builds all of them, each recorded as its own
artifact carrying its `os` and `arch`; absent means the machine forge runs
on. Nothing narrows or widens the list - there is no flag - so the
declaration is the whole of the policy. Declaring platforms is also what
makes an artifact public: a release ships what carries a platform, and a
repo's own tool that declares none stays home. The engine refuses a platform
it cannot build, at `forge config validate` and again at build time.

**Example:**
```yaml
build:
  - name: forge
    src: ./cmd/forge
    dest: ./build/bin
    engine: forge://go-build
    platforms: [linux/amd64, linux/arm64]
```

#### `spec` (map, optional)

Engine-specific configuration that is passed to the build engine. The supported fields depend on the engine being used.

**Common Fields for go-build:**
- `args` ([]string) - Additional arguments to pass to `go build`
- `env` (map[string]string) - Environment variables to set during build

**Common Fields for container-build:**
- `dependsOn` (array) - List of dependency detectors for lazy rebuild (see [Lazy Rebuild](#lazy-rebuild))

**Example - Custom Build Flags:**
```yaml
build:
  - name: static-binary
    src: ./cmd/myapp
    dest: ./build/bin
    engine: forge://go-build
    spec:
      args:
        - "-tags=netgo"
        - "-ldflags=-w -s"
      env:
        GOOS: "linux"
        GOARCH: "amd64"
        CGO_ENABLED: "0"
```

**Example - Cross-Compilation:**
```yaml
build:
  - name: myapp-darwin-arm64
    src: ./cmd/myapp
    dest: ./build/bin
    engine: forge://go-build
    spec:
      env:
        GOOS: "darwin"
        GOARCH: "arm64"
        CGO_ENABLED: "0"
```

**Example - Container with Dependency Tracking:**
```yaml
build:
  - name: api-server-image
    src: ./containers/api-server/Containerfile
    engine: forge://container-build
    spec:
      dependsOn:
        - engine: forge://go-dependency-detector
          spec:
            filePath: ./cmd/api-server/main.go
            funcName: main
```

**See also:**
- [cmd/go-build/MCP.md](../../cmd/go-build/MCP.md) for go-build specific configuration
- [cmd/container-build/MCP.md](../../cmd/container-build/MCP.md) for container-build specific configuration
- [cmd/generic-builder/MCP.md](../../cmd/generic-builder/MCP.md) for generic-builder configuration

### Complete BuildSpec Examples

#### Go Binary

```yaml
- name: my-cli-tool
  src: ./cmd/my-cli-tool
  dest: ./build/bin
  builder: forge://go-build
```

**Results in:**
- Binary: `./build/bin/my-cli-tool`
- Artifact type: `binary`
- Tracked in artifact store

#### Container Image

```yaml
- name: my-api
  src: ./containers/my-api/Containerfile
  builder: forge://container-build
```

**Results in:**
- Image: Tagged with project version
- Artifact type: `container`
- Tracked in artifact store
- Available for push to registry

## Lazy Rebuild

Forge automatically tracks dependencies for build artifacts and skips rebuilding unchanged artifacts to improve build performance.

### How It Works

When you run `forge build`:

1. **First Build**: All artifacts are built, and forge tracks their dependencies in the artifact store
2. **Subsequent Builds**: Forge compares the content digest of every
   recorded dependency, and of the output when the engine recorded one,
   with what is on disk:
   - **Skips** when every digest matches
   - **Rebuilds** when any recorded path is missing or its digest differs,
     or when a digested output is missing or was edited
   - A modification time is never read: a `touch` rebuilds nothing

### Dependency Tracking

**Go Binaries (go-build):**
- Automatically tracked when building main packages
- Includes every file of every local package the entry reaches (transitive)
- Includes `go.mod` and `go.sum`, so a changed module closure rebuilds
- No configuration required

**Container Images (container-build):**
- Requires explicit `dependsOn` configuration in `spec`
- Use `forge://go-dependency-detector` for Go-based containers
- Example:
  ```yaml
  - name: api-image
    src: ./containers/api/Containerfile
    engine: forge://container-build
    spec:
      dependsOn:
        - engine: forge://go-dependency-detector
          spec:
            filePath: ./cmd/api/main.go
            funcName: main
  ```

### Rebuilding by Hand

There is no force flag. A build is skipped only when every recorded digest
still matches, so an edit to any input rebuilds on its own and a hand-edited
output rebuilds too. To rebuild anyway, delete the output:

```bash
rm build/bin/my-app
forge build my-app
```

### Rebuild Reasons

When an artifact is rebuilt, forge shows the reason:

- `no previous build for <platform>` - First time building this artifact for that platform
- `dependencies not tracked` - The record carries no dependencies (nothing recorded, or a record from before digests)
- `dependency /path/to/file missing` - A recorded input was deleted
- `dependency /path/to/file changed` - A recorded input's content changed
- `artifact /path missing for <platform>` - The built output was deleted
- `artifact /path changed since it was built` - The output was edited by hand

### Performance Benefits

Lazy rebuild provides significant performance improvements:

- **Skip unchanged binaries**: Rebuild only what changed
- **Incremental builds**: Large projects rebuild faster
- **CI/CD optimization**: Only rebuild affected artifacts in pull requests

### Limitations

- What counts as an input is the detector's answer; an engine that records
  no dependencies rebuilds every time
- Container images require explicit `dependsOn` configuration
- A record written before digests carries none and rebuilds once, which
  rewrites it

## Engine Protocol

Build engines use the `forge://` protocol to reference MCP servers.

### Protocol Format

```
forge://<binary-name>
```

**Components:**
- `forge://` - Protocol identifier (indicates MCP server)
- `<binary-name>` - Name of the MCP server binary

### Engine Resolution

When forge encounters an engine URI:

1. **URI Parsing:** `forge://<name>[@<version>]` for forge's own engines,
   `forge://<module-path>[@rev]` for a factory member.
2. **Own engines** (`forge://go-build`): run at the running forge's own
   version - an embedded `@version` is ignored so every engine matches the
   CLI. When the enclosing `go.work` lists the forge module the engine runs
   via `go run github.com/alexandremahdhaoui/forge/cmd/<name>` with no
   version, from the caller's directory. Otherwise it runs via
   `go run github.com/alexandremahdhaoui/forge/cmd/<name>@<forge-version>`.
   `FORGE_RUN_LOCAL_ENABLED=true` is a forced override that builds the engine
   from the checkout named by `FORGE_RUN_LOCAL_BASEDIR` into
   `build/local-engines/`.
3. **Member engines** (`forge://github.com/x/repo/cmd/tool`): the enclosing
   Go workspace wins when it carries the module; otherwise
   `forge-factory run` materialises it and the version comes from the
   member's register. Versions are always pinned - `latest` is never a
   fallback.
4. **MCP Mode:** the resolved command is invoked with `--mcp`.
5. **Communication:** stdio JSON-RPC 2.0.

**Note:** Nothing is installed into PATH by engine resolution; a workspace's
pinned tooling arrives through `forge-factory sync`.

### Available Engines

#### go-build

**URI:** `forge://go-build`

**Purpose:** Build Go binaries with version metadata injection and automatic dependency tracking

**Required BuildSpec Fields:**
- `name` - Binary name
- `src` - Go package path
- `dest` - Output directory
- `builder: forge://go-build`

**Features:**
- Automatic dependency detection for Go main packages
- Lazy rebuild based on dependency changes
- Git version metadata injection

**Environment Variables:**
- `GO_BUILD_LDFLAGS` - Additional linker flags

**Example:**
```yaml
- name: my-app
  src: ./cmd/my-app
  dest: ./build/bin
  builder: forge://go-build
```

**Build Command:**
```bash
GO_BUILD_LDFLAGS="-X main.Version=v1.0.0" forge build
```

**Lazy Rebuild:** Go binaries automatically track dependencies. Subsequent `forge build` commands skip an artifact whose recorded digests all still match.

#### go-dependency-detector

**URI:** `forge://go-dependency-detector`

**Purpose:** Detect Go code dependencies for lazy rebuild optimization

**Usage:** Primarily used internally by `go-build` and `container-build` engines. Can also be used explicitly in `spec.dependsOn` for containers.

**Example (explicit usage in container):**
```yaml
- name: api-image
  src: ./containers/api/Containerfile
  engine: forge://container-build
  spec:
    dependsOn:
      - engine: forge://go-dependency-detector
        spec:
          filePath: ./cmd/api/main.go
          funcName: main
```

**See also:** [cmd/go-dependency-detector/MCP.md](../../cmd/go-dependency-detector/MCP.md)

#### container-build

**URI:** `forge://container-build`

**Purpose:** Build container images using Kaniko (rootless, secure)

**Required BuildSpec Fields:**
- `name` - Image name
- `src` - Path to Containerfile
- `builder: forge://container-build`

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
  builder: forge://container-build
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
  builder: forge://my-custom-engine
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
    runner: "forge://go-test"

  - name: integration
    testenv: "alias://setup-integration"
    runner: "forge://go-test"

  - name: e2e
    runner: "forge://forge-e2e"

  - name: lint
    runner: "forge://go-lint"
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
- `"forge://testenv"` - Complete test environment (Kind cluster + registry + helm)
- `"forge://testenv-kind"` - Kind cluster only
- `"forge://testenv-lcr"` - Local container registry only
- `"alias://<name>"` - Custom engine alias from engines section

**Example:**
```yaml
# No environment needed (omit testenv field)
name: unit
runner: "forge://go-test"

# Full test environment with cluster
name: integration
testenv: "forge://testenv"
runner: "forge://go-test"

# Custom environment alias
name: integration
testenv: "alias://setup-integration"
runner: "forge://go-test"
```

#### `runner` (string, required)

Test runner engine URI specifying which test runner to use.

**Format:** `<protocol>://<runner-name>`

**Available Runners:**
- `"forge://go-test"` - Go test runner with coverage and JUnit reports
- `"forge://go-lint-tags"` - Verify all test files have build tags
- `"forge://generic-test-runner"` - Execute arbitrary commands as tests
- `"forge://go-lint"` - Golangci-lint runner
- `"forge://forge-e2e"` - Forge end-to-end test runner

**Example:**
```yaml
# Run Go tests
runner: "forge://go-test"

# Verify build tags
runner: "forge://go-lint-tags"

# Run linter
runner: "forge://go-lint"

# Execute custom commands
runner: "forge://generic-test-runner"
```

#### `manual` (bool, optional)

A stage `forge test-all` skips. It runs only by name, through
`forge test run <name>`: a dev-machine step that a normal gate must not
trigger.

#### `needs` (array of string, optional)

Build entries this stage needs built before its environment is created and
its runner runs: a fixture image the testenv pushes, a binary the suite
executes. An entry named here is owned by this stage - a bare `forge build`
leaves it alone and says which stage builds it, and only `forge build <name>`
or the owning stage builds it. That is what keeps a fixture that wants a
daemon out of every build that never asked for it, with no flag deciding
anything. One stage per entry; a name no build entry declares is refused.

**Example:**
```yaml
test:
  - name: integration
    runner: forge://go-test
    testenv: alias://setup-integration
    needs: [for-testing-purposes]
```

### Complete TestSpec Examples

#### Unit Tests (No Environment)

```yaml
- name: unit
  runner: "forge://go-test"
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
  runner: "forge://go-test"
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
  runner: "forge://go-lint"
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
      - engine: "forge://testenv-kind"
      - engine: "forge://testenv-lcr"
        spec:
          enabled: true
          autoPushImages: true

# Build configuration
build:
  # CLI tools
  - name: my-cli
    src: ./cmd/my-cli
    dest: ./build/bin
    engine: forge://go-build

  - name: api-server
    src: ./cmd/api-server
    dest: ./build/bin
    engine: forge://go-build

  # Build tools (self-hosting)
  - name: go-build
    src: ./cmd/go-build
    dest: ./build/bin
    engine: forge://go-build

  - name: container-build
    src: ./cmd/container-build
    dest: ./build/bin
    engine: forge://go-build

  # Container images
  - name: api-server-image
    src: ./containers/api-server/Containerfile
    dest: localhost:5000
    engine: forge://container-build

  - name: worker
    src: ./containers/worker/Containerfile
    engine: forge://container-build

# Test stages configuration
test:
  # Verify build tags
  - name: verify-tags
    runner: "forge://go-lint-tags"

  # Unit tests - no environment needed
  - name: unit
    runner: "forge://go-test"

  # Integration tests - full test environment
  - name: integration
    testenv: "alias://setup-integration"
    runner: "forge://go-test"

  # E2E tests
  - name: e2e
    runner: "forge://forge-e2e"

  # Linting
  - name: lint
    runner: "forge://go-lint"

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
# Go binaries: use go-build
- name: my-binary
  builder: forge://go-build

# Container images: use container-build
- name: my-image
  builder: forge://container-build
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
      builder: forge://go-build

    # go-build builds itself
    - name: go-build
      src: ./cmd/go-build
      dest: ./build/bin
      builder: forge://go-build
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
   - Update engine references to use `forge://` protocol

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

- [Forge CLI Usage Guide](./forge-cli.md)
- [DESIGN.md - Forge Architecture](../../DESIGN.md#high-level-architecture)
- [Model Context Protocol Specification](https://modelcontextprotocol.io)
