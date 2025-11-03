# forge.yaml Schema Documentation

This document provides comprehensive documentation for the `forge.yaml` configuration file, which is the central configuration for the forge build orchestrator.

## Table of Contents

- [Overview](#overview)
- [File Location](#file-location)
- [Root Schema](#root-schema)
- [Build Configuration](#build-configuration)
- [BuildSpec Specification](#buildspec-specification)
- [Engine Protocol](#engine-protocol)
- [Kindenv Configuration](#kindenv-configuration)
- [Local Container Registry Configuration](#local-container-registry-configuration)
- [Complete Example](#complete-example)
- [Artifact Store Schema](#artifact-store-schema)

## Overview

The `forge.yaml` file defines:
- **Build artifacts** to be created (binaries, containers)
- **Build engines** to use for each artifact
- **Integration environment** components (kindenv, local-container-registry)
- **Artifact tracking** configuration

## File Location

**Path:** `forge.yaml` (repository root)

**Format:** YAML

**Version:** 1.0

## Root Schema

```yaml
name: string                              # Project name
build: Build                              # Build configuration
kindenv: Kindenv                          # Kind cluster configuration
localContainerRegistry: LocalContainerRegistry  # Local registry configuration
oapiCodegenHelper: OAPICodegenHelper      # OpenAPI codegen configuration
```

### Root Fields

#### `name` (string, required)

Project name used for identification.

**Example:**
```yaml
name: tooling
```

#### `build` (Build, required)

Build configuration defining all artifacts to build. See [Build Configuration](#build-configuration).

#### `kindenv` (Kindenv, optional)

Kind cluster configuration for integration environments. See [Kindenv Configuration](#kindenv-configuration).

#### `localContainerRegistry` (LocalContainerRegistry, optional)

Local container registry configuration. See [Local Container Registry Configuration](#local-container-registry-configuration).

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

## Kindenv Configuration

Configuration for Kind (Kubernetes in Docker) clusters.

### Schema

```yaml
kindenv:
  kubeconfigPath: string    # Path to generated kubeconfig
```

### Fields

#### `kubeconfigPath` (string, required)

Path where the Kind cluster kubeconfig will be written.

**Default:** `.ignore.kindenv.kubeconfig.yaml`

**Example:**
```yaml
kindenv:
  kubeconfigPath: .ignore.kindenv.kubeconfig.yaml
```

**Usage:**
```bash
# Create integration environment with kind cluster
forge integration create my-dev-env

# Use the kubeconfig
export KUBECONFIG=.ignore.kindenv.kubeconfig.yaml
kubectl cluster-info
```

## Local Container Registry Configuration

Configuration for the in-cluster container registry with TLS and authentication.

### Schema

```yaml
localContainerRegistry:
  enabled: boolean           # Enable/disable registry
  autoPushImages: boolean    # Auto-push artifacts on setup
  credentialPath: string     # Path to credentials file
  caCrtPath: string         # Path to CA certificate
  namespace: string         # Kubernetes namespace
```

### Fields

#### `enabled` (boolean, required)

Enable or disable the local container registry component.

**Default:** `false`

**Example:**
```yaml
localContainerRegistry:
  enabled: true
```

#### `autoPushImages` (boolean, optional)

Automatically push container artifacts from the artifact store when setting up the registry.

**Default:** `false`

**Example:**
```yaml
localContainerRegistry:
  autoPushImages: true
```

When `true`, forge will:
1. Read artifact store
2. Find all container artifacts
3. Push them to the local registry on setup

#### `credentialPath` (string, required if enabled)

Path where registry credentials will be written.

**Default:** `.ignore.local-container-registry.yaml`

**Format:**
```yaml
username: <random-32-chars>
password: <random-32-chars>
registry: local-container-registry.local-container-registry.svc.cluster.local:5000
```

**Example:**
```yaml
localContainerRegistry:
  credentialPath: .ignore.local-container-registry.yaml
```

#### `caCrtPath` (string, required if enabled)

Path where the registry CA certificate will be written.

**Default:** `.ignore.ca.crt`

**Usage:** Configure container engine to trust this CA certificate for TLS connections.

**Example:**
```yaml
localContainerRegistry:
  caCrtPath: .ignore.ca.crt
```

#### `namespace` (string, required if enabled)

Kubernetes namespace where the registry will be deployed.

**Default:** `local-container-registry`

**Example:**
```yaml
localContainerRegistry:
  namespace: local-container-registry
```

### Complete Registry Example

```yaml
localContainerRegistry:
  enabled: true
  autoPushImages: true
  credentialPath: .ignore.local-container-registry.yaml
  caCrtPath: .ignore.ca.crt
  namespace: local-container-registry
```

**Creates:**
- TLS-enabled registry on port 5000
- htpasswd authentication
- Self-signed CA certificate
- Persistent credentials
- Kubernetes Deployment, Service, and ConfigMap

## Complete Example

Here's a complete `forge.yaml` example with all sections:

```yaml
# Project name
name: my-project

# Build configuration
build:
  # Path to artifact store
  artifactStorePath: .ignore.artifact-store.yaml

  # Artifacts to build
  specs:
    # CLI tools
    - name: my-cli
      src: ./cmd/my-cli
      dest: ./build/bin
      builder: go://build-go

    - name: api-server
      src: ./cmd/api-server
      dest: ./build/bin
      builder: go://build-go

    # Build tools (self-hosting)
    - name: build-go
      src: ./cmd/build-go
      dest: ./build/bin
      builder: go://build-go

    - name: build-container
      src: ./cmd/build-container
      dest: ./build/bin
      builder: go://build-go

    # Container images
    - name: api-server
      src: ./containers/api-server/Containerfile
      dest: localhost:5000
      builder: go://build-container

    - name: worker
      src: ./containers/worker/Containerfile
      builder: go://build-container

# Kind cluster configuration
kindenv:
  kubeconfigPath: .ignore.kindenv.kubeconfig.yaml

# Local container registry configuration
localContainerRegistry:
  enabled: true
  autoPushImages: true
  credentialPath: .ignore.local-container-registry.yaml
  caCrtPath: .ignore.ca.crt
  namespace: local-container-registry

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
artifacts:
  - name: string        # Artifact identifier
    type: string        # "binary" or "container"
    location: string    # File path or image reference
    timestamp: string   # ISO 8601 timestamp
    version: string     # Git version (commit hash + dirty flag)
```

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

Add artifact store and generated files to `.gitignore`:

```gitignore
.ignore.artifact-store.yaml
.ignore.kindenv.kubeconfig.yaml
.ignore.local-container-registry.yaml
.ignore.ca.crt
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
rm .ignore.artifact-store.yaml
forge build
```

### Registry Connection Issues

**Problem:** Cannot connect to local container registry.

**Solution:**
```bash
# Verify registry is running
kubectl get pods -n local-container-registry

# Check port-forward
kubectl port-forward -n local-container-registry svc/local-container-registry 5000:5000

# Test connection
curl -k https://localhost:5000/v2/
```

## References

- [forge CLI Usage Guide](./forge-usage.md)
- [ARCHITECTURE.md - Forge Architecture](../ARCHITECTURE.md#forge-architecture)
- [Model Context Protocol Specification](https://modelcontextprotocol.io)
