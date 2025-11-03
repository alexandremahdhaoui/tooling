# Architecture

This document provides a comprehensive overview of the tooling repository architecture, design patterns, and components.

## Table of Contents

- [Overview](#overview)
- [Project Structure](#project-structure)
- [Core Packages](#core-packages)
- [Command-Line Tools](#command-line-tools)
- [Build System](#build-system)
- [Testing Infrastructure](#testing-infrastructure)
- [Local Container Registry](#local-container-registry)
- [Forge Architecture](#forge-architecture)
- [Configuration Management](#configuration-management)
- [Design Patterns](#design-patterns)
- [Dependencies](#dependencies)

## Overview

This is a Go development tooling repository that provides a comprehensive set of command-line tools for streamlining Go development workflows, with a particular focus on:

- Container and Kubernetes development
- Local development environments
- CI/CD operations
- Code generation

**Key Statistics:**

- 25 Go source files
- 3,373 lines of Go code
- Go version: 1.24.1
- License: Apache 2.0

**Philosophy:** The repository follows a "dogfooding" approach where the tools are used to build and test themselves, ensuring they work in real-world scenarios.

## Project Structure

```
/
├── cmd/                    # Command-line tools (11 tools)
├── pkg/                    # Public reusable packages
├── internal/              # Internal utilities and mocks
├── containers/            # Container definitions
├── docs/                  # Documentation
├── hack/                  # Build and generation scripts
├── .githooks/            # Git hooks for quality control
├── .project.yaml         # Central project configuration
└── Makefile              # Build automation
```

### Directory Responsibilities

#### `/cmd` - Command-Line Tools

Each subdirectory contains a standalone CLI tool. Tools are designed to be:

- Environment-variable driven for CI/CD compatibility
- Self-contained with minimal dependencies
- Composable and scriptable

#### `/pkg` - Public Packages

Reusable library code that can be imported by other Go projects:

- `eventualconfig` - Async configuration management
- `flaterrors` - Error flattening utilities
- `project` - Project configuration management

#### `/internal` - Internal Utilities

Private implementation details not exposed as public API:

- `util` - Command execution and environment formatting
- `util/mocks` - Generated mocks for testing

## Core Packages

### eventualconfig

**Location:** `pkg/eventualconfig/`

**Purpose:** Manages configuration values that may be set asynchronously across different goroutines or setup phases.

**Design Pattern:** Channel-based eventual consistency with type-safe value retrieval.

**Key Features:**

- Thread-safe value setting and getting
- Generic `AwaitValue[T]` function for type-safe retrieval
- Pre-declared keys at initialization time
- Blocks until value is available (ensures coordination)

**Use Case:** The local-container-registry uses this to coordinate between different setup phases (TLS, credentials, registry) that run concurrently but depend on each other's outputs.

**Example:**

```go
cfg := eventualconfig.New([]string{"TLSCert", "TLSKey"})

// In one goroutine
cfg.Set("TLSCert", certPath)

// In another goroutine (blocks until value is available)
cert := eventualconfig.AwaitValue[string](cfg, "TLSCert")
```

**Test Coverage:** Comprehensive unit tests in `eventualconfig_test.go` covering:

- Concurrent operations
- Error conditions (unknown keys, type mismatches)
- Blocking behavior

### flaterrors

**Location:** `pkg/flaterrors/`

**Purpose:** Flattens nested error trees into a single-level error list for cleaner error messages.

**Design Pattern:** Custom error unwrapping implementing Go's error unwrapping interface.

**Key Features:**

- Recursively flattens error trees
- Compatible with `errors.Is()` and `errors.As()`
- Provides cleaner error output in multi-step operations

**Use Case:** Throughout the codebase where multiple operations may fail independently (e.g., cleanup operations in local-container-registry teardown).

**Example:**

```go
err1 := errors.New("failed to delete namespace")
err2 := errors.New("failed to delete certificate")
flatErr := flaterrors.Join(err1, err2)
// Result: single error with both messages
```

### project

**Location:** `pkg/project/`

**Purpose:** Central configuration management for the entire project.

**Configuration File:** `.project.yaml`

**Key Structures:**

```go
type Config struct {
    Name                   string
    Kindenv                Kindenv
    LocalContainerRegistry LocalContainerRegistry
    OAPICodegenHelper      OAPICodegenHelper
}
```

**Configuration Sources:**

1. `.project.yaml` file (primary)
2. Environment variables (override)

## Command-Line Tools

### Forge CLI

**Location:** `cmd/forge/`

**Purpose:** Make-like build orchestrator that uses MCP (Model Context Protocol) servers to build artifacts and manage integration environments.

**Architecture:** See [Forge Architecture](#forge-architecture) section below.

### Local Container Registry

**Location:** `cmd/local-container-registry/`

**Purpose:** Creates a fully functional, TLS-enabled container registry inside a Kind cluster for local development.

**Architecture:** See [Local Container Registry](#local-container-registry) section below.

### kindenv

**Location:** `cmd/kindenv/`

**Purpose:** Manages Kind (Kubernetes in Docker) cluster lifecycle.

**Features:**

- Creates Kind clusters with custom configuration
- Generates kubeconfig at specified path
- Teardown and cleanup

### e2e

**Location:** `cmd/e2e/`

**Purpose:** End-to-end test runner that validates the entire toolchain.

**Test Flow:**

1. Setup local-container-registry
2. Port-forward registry service (5000:5000)
3. Login to registry with generated credentials
4. Tag and push test image
5. Pull image back
6. Teardown infrastructure
7. Report results

**Container Engine Support:** Docker and Podman

### build-binary

**Location:** `cmd/build-binary/`

**Purpose:** Wrapper around `go build` with standardized build flags.

**Features:**

- Injects version, commit SHA, and timestamp via ldflags
- Consistent build metadata across all binaries

### build-container

**Location:** `cmd/build-container/`

**Purpose:** Builds container images using Kaniko (rootless, secure).

**Features:**

- Supports multiple Containerfiles
- Configurable context directory
- Build arguments support

### test-go

**Location:** `cmd/test-go/`

**Purpose:** Go test runner with enhanced output formatting.

**Features:**

- Uses gotestsum for pretty output
- Generates JUnit XML reports
- Coverage reporting
- Support for test tags (unit, integration, functional, e2e)

### oapi-codegen-helper

**Location:** `cmd/oapi-codegen-helper/`

**Purpose:** Helper for generating Go code from OpenAPI specifications.

**Features:**

- Wraps oapi-codegen with project conventions
- Configurable via `.project.yaml`

### chart-prereq

**Location:** `cmd/chart-prereq/`

**Purpose:** Manages Helm chart dependencies and prerequisites.

**Status:** Minimal implementation

### ci-orchestrator

**Location:** `cmd/ci-orchestrator/`

**Purpose:** Vision for vendor-agnostic CI/CD orchestration.

**Goals:**

- Accessibility (run CI/CD anywhere)
- Security (proper secret management)
- Reproducibility (local = CI)
- Quality gates and artifact management

**Status:** Design/brainstorming phase

## Build System

### Makefile

The Makefile is the central orchestration point for all build, test, and generation tasks.

**Key Variables:**

```makefile
PROJECT          # Project name
VERSION          # Git-based semantic version
COMMIT_SHA       # Short git commit hash
TIMESTAMP        # ISO 8601 build timestamp
GO_BUILD_LDFLAGS # Linker flags for build metadata
```

**Tool Versions (managed by Renovate):**

- controller-tools: v0.14.0
- gofumpt: v0.6.0
- golangci-lint: v1.59.1
- gotestsum: v1.12.0
- mockery: v2.42.0
- oapi-codegen: v2.3.0

**Primary Targets:**

| Target | Description |
|--------|-------------|
| `generate` | Generates code (OpenAPI, CRDs, mocks, protobuf) |
| `build` | Builds all artifacts using forge |
| `build-go` | Builds Go binaries using forge |
| `build-container` | Builds container images using forge |
| `fmt` | Formats code with gofumpt |
| `lint` | Runs golangci-lint |
| `test-unit` | Runs unit tests |
| `test-integration` | Runs integration tests |
| `test-functional` | Runs functional tests |
| `test-e2e` | Runs end-to-end tests (uses forge) |
| `test-setup` | Creates Kind cluster and local registry |
| `test-teardown` | Destroys Kind cluster and local registry |
| `test` | Runs all tests |
| `pre-push` | Pre-push validation (generate, fmt, lint, test) |

**Special Features:**

- Protobuf support with auto-generation
- Controller-gen for Kubernetes CRDs
- Parallel test execution
- Self-referencing (uses own tools)

## Testing Infrastructure

### Test Organization

**Test Tags (Go build tags):**

- `unit` - Unit tests (fast, no external dependencies)
- `integration` - Integration tests (requires test cluster)
- `functional` - Functional tests (end-to-end scenarios)
- `e2e` - End-to-end tests (full system validation)

**Test Outputs:**

- JUnit XML: `.ignore.test-{tag}.xml`
- Coverage: `.ignore.test-{tag}-coverage.out`
- Pretty output via gotestsum

### Test Execution Flow

```
make test-setup          # Create Kind cluster
├── make test-unit       # Fast unit tests
├── make test-integration # Integration tests (requires cluster)
├── make test-functional  # Functional tests
└── make test-e2e        # Full system validation
make test-teardown       # Destroy Kind cluster
```

### E2E Test Architecture

**Location:** `cmd/e2e/main.sh`

**Test Sequence:**

1. **Setup Phase:**
   - Create local-container-registry
   - Port-forward registry service (5000:5000)
   - Extract credentials from generated file

2. **Validation Phase:**
   - Login to registry (with TLS verification handling)
   - Pull test image (registry:2)
   - Tag for local registry
   - Push to local registry
   - Pull from local registry

3. **Teardown Phase:**
   - Kill port-forward process
   - Teardown local-container-registry
   - Clean up

**Error Handling:**

- Automatic cleanup on failure
- Process management for background port-forwarding
- Supports both Docker and Podman with different TLS configurations

## Local Container Registry

The local-container-registry is the most sophisticated component in the repository.

### Architecture Overview

**Design Pattern:** Adapter pattern with eventual consistency coordination.

**Purpose:** Create a production-like container registry in a Kind cluster with:

- TLS encryption (via cert-manager)
- htpasswd authentication
- Persistent storage
- Service exposure

### Components (Setup Adapters)

#### 1. K8s Adapter (`setup_k8s.go`)

**Responsibilities:**

- Create/manage namespace
- Set KUBECONFIG environment variable
- Namespace lifecycle management

```go
type SetupK8s struct {
    client    client.Client
    namespace string
}
```

#### 2. TLS Adapter (`setup_tls.go`)

**Responsibilities:**

- Install cert-manager via Helm
- Create self-signed certificate issuer
- Generate TLS certificates for registry
- Export CA certificate
- Manage certificate lifecycle

**Certificate Configuration:**

- Issuer: Self-signed
- Certificate: Server cert for registry service
- DNS names: `local-container-registry.local-container-registry.svc.cluster.local`
- CA cert exported to: `.ignore.ca.crt`

**EventualConfig Keys Set:**

- `TLSCACert` - CA certificate mount info
- `TLSCert` - Server certificate mount info
- `TLSKey` - Server key mount info
- `TLSSecretName` - Kubernetes secret name

#### 3. Credentials Adapter (`setup_credentials.go`)

**Responsibilities:**

- Generate random username/password (32 characters each)
- Create htpasswd hash using httpd:2 container
- Store credentials in Kubernetes Secret
- Write credentials to local file (`.ignore.local-container-registry.yaml`)

**Process:**

1. Generate random credentials
2. Run `htpasswd` in container to create hash
3. Create Kubernetes Secret with htpasswd file
4. Write credentials to local YAML file

**EventualConfig Keys Set:**

- `CredentialMount` - Credential file mount info
- `CredentialSecretName` - Kubernetes secret name

#### 4. Container Registry Adapter (`setup_container_registry.go`)

**Responsibilities:**

- Template registry configuration
- Create ConfigMap with registry config
- Create Service (ClusterIP on port 5000)
- Create Deployment (registry:2 image)
- Wait for deployment readiness
- Mount credentials, TLS certs, and config

**Registry Configuration Template:**

```yaml
version: 0.1
log:
  fields:
    service: registry
storage:
  filesystem:
    rootdirectory: /var/lib/registry
http:
  addr: :5000
  tls:
    certificate: /certs/tls.crt
    key: /certs/tls.key
auth:
  htpasswd:
    realm: basic-realm
    path: /auth/htpasswd
```

**Deployment Specification:**

- Image: `registry:2`
- Port: 5000 (HTTPS)
- Volume mounts:
  - `/auth` - htpasswd credentials
  - `/certs` - TLS certificates
  - `/etc/docker/registry` - registry config
- Storage: emptyDir (ephemeral)

### Configuration Management

**EventualConfig Keys:**

```go
const (
    TLSCACert             = "TLSCACert"
    TLSCert               = "TLSCert"
    TLSKey                = "TLSKey"
    TLSSecretName         = "TLSSecretName"
    CredentialMount       = "CredentialMount"
    CredentialSecretName  = "CredentialSecretName"
)
```

**Mount Structure:**

```go
type Mount struct {
    Dir      string  // Mount directory in container
    Filename string  // Filename
}
```

### Setup Sequence

```
main()
├── Read .project.yaml
├── Create Kubernetes client
├── Initialize EventualConfig
├── Initialize all adapters
├── Setup K8s (namespace) ──────────────┐
├── Setup Credentials (parallel) ───────┼──> All adapters run concurrently
└── Setup TLS (parallel) ───────────────┤
    └── Setup Registry (waits on EventualConfig) ─┘
        └── Wait for deployment readiness
```

**Concurrency Model:**

- K8s setup runs first (creates namespace)
- Credentials and TLS setup run in parallel
- Registry setup waits for EventualConfig values from TLS and Credentials
- EventualConfig ensures proper coordination

### Teardown Sequence

```
teardown()
├── Delete namespace (cascade deletes all resources)
├── Delete cert-manager resources
└── Clean up local files
```

### Registry FQDN

**Service FQDN:**

```
local-container-registry.local-container-registry.svc.cluster.local:5000
```

**Access Methods:**

1. **From within cluster:** Use service FQDN directly
2. **From host:** Port-forward to localhost:5000

   ```bash
   kubectl port-forward -n local-container-registry svc/local-container-registry 5000:5000
   ```

## Forge Architecture

Forge is a make-like build orchestrator that provides a unified interface for building artifacts and managing integration environments using the Model Context Protocol (MCP).

### Overview

**Design Philosophy:**
- Unified build specification across all artifact types
- Engine-based architecture using MCP servers
- Artifact tracking and versioning
- Integration environment lifecycle management

**Key Features:**
- Build Go binaries and container images through a single interface
- Track built artifacts with metadata (version, timestamp, type)
- Manage integration environments (kind clusters with optional components)
- Support for MCP-based build engines

### Core Components

#### 1. BuildSpec API

The `BuildSpec` is a unified specification for building any type of artifact:

```go
type BuildSpec struct {
    Name   string `yaml:"name"`   // Artifact name
    Src    string `yaml:"src"`    // Source directory/file
    Dest   string `yaml:"dest"`   // Destination directory
    Engine string `yaml:"engine"` // Engine URI (e.g., "go://build-go")
}
```

**Engine URI Format:** `<protocol>://<engine-name>`

Supported engines:
- `go://build-go` - Go binary builder
- `container://build-container` - Container image builder

#### 2. Build Engines (MCP Servers)

Build engines are MCP servers that implement the build protocol via stdio communication.

**MCP Server Architecture:**

```
forge CLI (client)
    |
    | JSON-RPC 2.0 over stdio
    |
    v
MCP Server (--mcp flag)
    |
    | Executes build
    |
    v
Build Artifact
```

**Available Engines:**

1. **build-go** (`cmd/build-go/`)
   - Builds Go binaries
   - Supports ldflags injection
   - MCP tool: `build`

2. **build-container** (`cmd/build-container/`)
   - Builds container images using Kaniko
   - Supports custom Containerfiles
   - MCP tool: `build`

**MCP Communication Example:**

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "build",
    "arguments": {
      "name": "my-binary",
      "src": "./cmd/my-app",
      "dest": "./build/bin",
      "engine": "go://build-go"
    }
  }
}
```

#### 3. Artifact Store

**Location:** `.ignore.artifact-store.yaml`

**Purpose:** Tracks all built artifacts with metadata for version control and reproducibility.

**Structure:**

```yaml
artifacts:
  - name: build-binary
    type: go-binary
    version: v1.2.3-abc1234-dirty
    timestamp: "2025-01-03T10:30:00Z"
    src: ./cmd/build-binary
    dest: ./build/bin

  - name: build-container
    type: container
    version: v1.2.3-abc1234
    timestamp: "2025-01-03T10:31:00Z"
    containerfile: ./containers/build-container/Containerfile
    context: .
    image: localhost:5000/build-container:v1.2.3-abc1234
```

**Artifact Types:**
- `go-binary` - Go executable binaries
- `container` - Container images

**Metadata Fields:**
- `name` - Artifact identifier
- `type` - Artifact type
- `version` - Semantic version with git metadata
- `timestamp` - ISO 8601 build timestamp
- `src` - Source location
- `dest` - Output location

#### 4. Integration Environment Management

**Location:** `.ignore.integration-envs.yaml`

**Purpose:** Manage integration testing environments with kind clusters and optional components.

**Environment Structure:**

```go
type IntegrationEnvironment struct {
    ID         string                 `yaml:"id"`
    Name       string                 `yaml:"name"`
    Created    string                 `yaml:"created"`
    Components map[string]Component   `yaml:"components"`
}

type Component struct {
    Enabled        bool              `yaml:"enabled"`
    Ready          bool              `yaml:"ready"`
    ConnectionInfo map[string]string `yaml:"connectionInfo,omitempty"`
}
```

**Supported Components:**
- `kindenv` - Kind cluster
- `local-container-registry` - Local registry with TLS

**Environment Store Example:**

```yaml
environments:
  - id: abc123-def456
    name: my-dev-env
    created: "2025-01-03T10:00:00Z"
    components:
      kindenv:
        enabled: true
        ready: true
        connectionInfo:
          kubeconfig: .ignore.kindenv.kubeconfig.yaml
      local-container-registry:
        enabled: true
        ready: true
        connectionInfo:
          namespace: local-container-registry
          credentialsFile: .ignore.local-container-registry.yaml
```

### Configuration: forge.yaml

**Location:** `forge.yaml` (root of repository)

**Purpose:** Defines all buildable artifacts for the project.

**Structure:**

```yaml
build:
  - name: build-binary
    src: ./cmd/build-binary
    dest: ./build/bin
    engine: go://build-go

  - name: build-container
    src: ./cmd/build-container
    dest: ./build/bin
    engine: go://build-go

  - name: build-container-image
    src: ./containers/build-container/Containerfile
    dest: localhost:5000
    engine: container://build-container
```

**Configuration Fields:**
- `build` - Array of BuildSpec objects
- Each BuildSpec defines one buildable artifact

### CLI Commands

#### Build Commands

**forge build** - Build all artifacts defined in forge.yaml

```bash
forge build
```

**Environment Variables:**
- `CONTAINER_ENGINE` - Container engine (docker/podman)
- `GO_BUILD_LDFLAGS` - Go linker flags
- `PREPEND_CMD` - Command prefix (e.g., sudo)

**Outputs:**
- Built artifacts in specified destinations
- Updated artifact store (`.ignore.artifact-store.yaml`)

**Build Flow:**

```
1. Read forge.yaml
2. For each BuildSpec:
   a. Parse engine URI
   b. Locate engine binary
   c. Start MCP server (--mcp flag)
   d. Send JSON-RPC build request
   e. Capture build output
   f. Record artifact metadata
3. Write artifact store
```

#### Integration Environment Commands

**forge integration create** - Create a new integration environment

```bash
forge integration create <name>
```

**Outputs:**
- Kind cluster
- Optional: local container registry
- Environment record in `.ignore.integration-envs.yaml`
- Environment ID for reference

**forge integration list** - List all integration environments

```bash
forge integration list
```

**Output Format:**
```
Integration Environments:
- my-dev-env (ID: abc123-def456)
  Created: 2025-01-03T10:00:00Z
  Components:
    - kindenv: enabled, ready
    - local-container-registry: enabled, ready
```

**forge integration get** - Get details about an environment

```bash
forge integration get <id-or-name>
```

**forge integration delete** - Delete an integration environment

```bash
forge integration delete <id-or-name>
```

**Operations:**
- Teardown kind cluster
- Teardown local container registry
- Remove from environment store

### MCP Server Protocol

#### Initialization

1. Start engine with `--mcp` flag
2. Engine sends `initialize` request
3. Client responds with capabilities
4. Engine sends `initialized` notification

#### Tool Invocation

**Request Format:**

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "build",
    "arguments": {
      "name": "artifact-name",
      "src": "./source/path",
      "dest": "./dest/path",
      "engine": "go://engine-name"
    }
  }
}
```

**Response Format:**

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "Build completed successfully"
      }
    ]
  }
}
```

**Error Response:**

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32603,
    "message": "Build failed: compilation error"
  }
}
```

### Build Engine Implementation

#### Engine Interface

Each build engine must:
1. Accept `--mcp` flag to enable MCP mode
2. Implement stdio-based JSON-RPC 2.0 protocol
3. Provide `build` tool with BuildSpec parameters
4. Return success/failure with detailed messages

#### Example: build-go Engine

**Location:** `cmd/build-go/main.go`

**Capabilities:**

```go
// MCP tool definition
tool := mcp.Tool{
    Name: "build",
    Description: "Build a Go binary",
    InputSchema: mcp.ToolInputSchema{
        Type: "object",
        Properties: map[string]interface{}{
            "name":   map[string]string{"type": "string"},
            "src":    map[string]string{"type": "string"},
            "dest":   map[string]string{"type": "string"},
            "engine": map[string]string{"type": "string"},
        },
        Required: []string{"name", "src", "dest", "engine"},
    },
}
```

**Build Execution:**

```go
func executeBuild(args BuildSpec) error {
    // Construct go build command
    cmd := exec.Command("go", "build",
        "-o", filepath.Join(args.Dest, args.Name),
        "-ldflags", os.Getenv("GO_BUILD_LDFLAGS"),
        args.Src,
    )

    // Execute and capture output
    output, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("build failed: %w\n%s", err, output)
    }

    return nil
}
```

### Integration with Makefile

**Installation:**

```bash
# Install forge (engines are auto-installed on first use)
go install github.com/alexandremahdhaoui/forge/cmd/forge@latest

# Add to PATH
GOBIN_PATH=$(go env GOBIN)
if [ -z "$GOBIN_PATH" ]; then
  GOBIN_PATH=$(go env GOPATH)/bin
fi
export PATH="$GOBIN_PATH:$PATH"
```

**Makefile Variables:**

```makefile
FORGE := GO_BUILD_LDFLAGS="$(GO_BUILD_LDFLAGS)" \
         $(KINDENV_ENVS) \
         CONTAINER_ENGINE="$(CONTAINER_ENGINE)" \
         forge

BUILD_GO := $(FORGE) build
BUILD_CONTAINER := $(FORGE) build
```

**Makefile Targets:**

```makefile
.PHONY: build
build: ## Build all artifacts using forge
	$(FORGE) build

.PHONY: build-go
build-go: ## Build Go binaries using forge
	$(BUILD_GO)

.PHONY: build-container
build-container: ## Build container images using forge
	$(BUILD_CONTAINER)
```

### Testing

#### Integration Tests

**Location:** `cmd/forge/build_test.go`, `cmd/forge/integration_test.go`

**Test Coverage:**
- Build lifecycle (all artifacts)
- Single artifact builds
- Error handling (nonexistent artifacts)
- Integration environment lifecycle (create, list, get, delete)
- Artifact store operations

**Example Test:**

```go
func TestBuildIntegration(t *testing.T) {
    // Change to repository root
    os.Chdir("../..")

    // Build forge binary
    exec.Command("go", "build", "-o", "./build/bin/forge", "./cmd/forge").Run()

    // Run forge build
    cmd := exec.Command("./build/bin/forge", "build")
    output, err := cmd.CombinedOutput()

    // Verify artifacts
    store, _ := forge.ReadArtifactStore(".ignore.artifact-store.yaml")
    // Assert artifact properties
}
```

#### MCP Server Tests

**Location:** `cmd/test-mcp-servers.sh`

**Purpose:** Direct MCP server invocation testing

**Test Flow:**
1. Build MCP server binaries
2. Send JSON-RPC requests via stdin
3. Capture JSON-RPC responses via stdout
4. Verify artifacts were created

#### E2E Tests

**Location:** `cmd/e2e/main.sh`

**Integration:** E2E tests now use forge for building containers:

```bash
# Build containers using forge
CONTAINER_ENGINE="${CONTAINER_ENGINE}" \
GO_BUILD_LDFLAGS="${GO_BUILD_LDFLAGS:-}" \
forge build
```

**Note:** E2E tests use `go run ./cmd/forge` in the actual test script to ensure they test the current source code, not an installed version.

### Data Structures

#### Artifact

```go
type Artifact struct {
    Name          string            `yaml:"name"`
    Type          ArtifactType      `yaml:"type"`
    Version       string            `yaml:"version"`
    Timestamp     string            `yaml:"timestamp"`
    Src           string            `yaml:"src,omitempty"`
    Dest          string            `yaml:"dest,omitempty"`
    Containerfile string            `yaml:"containerfile,omitempty"`
    Context       string            `yaml:"context,omitempty"`
    Image         string            `yaml:"image,omitempty"`
}

type ArtifactType string

const (
    ArtifactTypeGoBinary  ArtifactType = "go-binary"
    ArtifactTypeContainer ArtifactType = "container"
)
```

#### ArtifactStore

```go
type ArtifactStore struct {
    Artifacts []Artifact `yaml:"artifacts"`
}

// Read artifact store from file
func ReadArtifactStore(path string) (ArtifactStore, error)

// Write artifact store to file
func WriteArtifactStore(path string, store ArtifactStore) error

// Add or update artifact
func AddOrUpdateArtifact(store *ArtifactStore, artifact Artifact)
```

#### IntegrationEnvStore

```go
type IntegrationEnvStore struct {
    Environments []IntegrationEnvironment `yaml:"environments"`
}

// Store operations
func ReadIntegrationEnvStore(path string) (IntegrationEnvStore, error)
func WriteIntegrationEnvStore(path string, store IntegrationEnvStore) error
func AddEnvironment(store *IntegrationEnvStore, env IntegrationEnvironment)
func GetEnvironment(store IntegrationEnvStore, idOrName string) (IntegrationEnvironment, error)
func DeleteEnvironment(store *IntegrationEnvStore, idOrName string) error
```

### Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                         Forge CLI                                │
│                                                                   │
│  ┌──────────────┐  ┌──────────────┐  ┌────────────────────┐   │
│  │ Build Command│  │ Integration  │  │  Artifact Store    │   │
│  │              │  │  Command     │  │  Management        │   │
│  └──────┬───────┘  └──────┬───────┘  └────────┬───────────┘   │
│         │                  │                    │                │
│         v                  v                    v                │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │              Engine Manager (MCP Client)                  │  │
│  └──────────────────────────────────────────────────────────┘  │
└─────────────────────────────┬───────────────────────────────────┘
                               │
                               │ stdio + JSON-RPC 2.0
                               │
         ┌─────────────────────┴─────────────────────┐
         │                                             │
         v                                             v
┌────────────────────┐                      ┌────────────────────┐
│  build-go          │                      │ build-container    │
│  (MCP Server)      │                      │ (MCP Server)       │
│                    │                      │                    │
│  ┌──────────────┐  │                      │ ┌──────────────┐  │
│  │ MCP Protocol │  │                      │ │ MCP Protocol │  │
│  └──────┬───────┘  │                      │ └──────┬───────┘  │
│         v          │                      │        v          │
│  ┌──────────────┐  │                      │ ┌──────────────┐  │
│  │ go build     │  │                      │ │ kaniko       │  │
│  └──────────────┘  │                      │ └──────────────┘  │
└────────────────────┘                      └────────────────────┘
```

### Future Enhancements

1. **Additional Build Engines:**
   - `helm://package` - Helm chart packaging
   - `protoc://compile` - Protocol buffer compilation
   - `npm://build` - Node.js builds

2. **Parallel Builds:**
   - Build multiple artifacts concurrently
   - Dependency graph resolution
   - Smart caching

3. **Remote Engines:**
   - Network-based MCP servers
   - Distributed builds
   - Build farms

4. **Build Caching:**
   - Content-addressable storage
   - Skip unchanged artifacts
   - Cache invalidation strategies

5. **Enhanced Integration Environments:**
   - Custom component plugins
   - Environment templates
   - Resource quotas

### Known Limitations

1. **Sequential Builds:** Currently builds artifacts sequentially
2. **No Dependency Graph:** Cannot express build dependencies between artifacts
3. **Local Only:** MCP servers must be local binaries
4. **No Caching:** Rebuilds all artifacts every time
5. **Basic Error Recovery:** Limited retry or rollback capabilities

## Configuration Management

### .project.yaml

Central configuration file for the entire project.

**Structure:**

```yaml
name: tooling

kindenv:
  kubeconfigPath: .ignore.kindenv.kubeconfig.yaml

localContainerRegistry:
  enabled: true
  credentialPath: .ignore.local-container-registry.yaml
  caCrtPath: .ignore.ca.crt
  namespace: local-container-registry

oapiCodegenHelper: {}
```

**Configuration Loading:**

1. Parse `.project.yaml` file
2. Override with environment variables (using `github.com/caarlos0/env`)
3. Validate configuration

### Environment Variables

All tools support environment variable configuration with standardized naming:

**Format:** `{TOOL_NAME}_{FIELD_NAME}`

**Example:**

```bash
KINDENV_KUBECONFIG_PATH=./my-kubeconfig.yaml
LOCAL_CONTAINER_REGISTRY_NAMESPACE=my-registry
```

**Benefits:**

- CI/CD compatibility
- Docker/container-friendly
- Override configuration without modifying files

## Design Patterns

### 1. Dogfooding (Self-Hosting)

The repository uses its own tools for building and testing itself.

**Examples:**

- `make build-binary` uses `cmd/build-binary`
- `make test-go` uses `cmd/test-go`
- `make test-e2e` uses `cmd/e2e` and `cmd/local-container-registry`

**Benefits:**

- Ensures tools work in real-world scenarios
- Catches bugs early
- Demonstrates tool usage

### 2. Adapter Pattern

Used extensively in local-container-registry for separation of concerns.

**Adapters:**

- K8s Adapter (namespace management)
- TLS Adapter (certificate management)
- Credentials Adapter (authentication management)
- Registry Adapter (registry deployment)

**Benefits:**

- Clear separation of concerns
- Easy to test individual components
- Flexible and extensible

### 3. Eventual Consistency

The `eventualconfig` package implements eventual consistency for async operations.

**Pattern:**

```go
// Producer
cfg.Set("key", value)

// Consumer (blocks until value is available)
value := eventualconfig.AwaitValue[T](cfg, "key")
```

**Benefits:**

- Coordinates concurrent operations
- Type-safe value retrieval
- Prevents race conditions

### 4. Error Aggregation

Using `flaterrors` for collecting multiple errors.

**Pattern:**

```go
var errs []error
if err := operation1(); err != nil {
    errs = append(errs, err)
}
if err := operation2(); err != nil {
    errs = append(errs, err)
}
return flaterrors.Join(errs...)
```

**Benefits:**

- Provides complete error context
- Doesn't fail fast (collects all errors)
- Useful for cleanup operations

### 5. Environment-Driven Configuration

All configuration is driven by environment variables.

**Benefits:**

- 12-factor app compliance
- CI/CD friendly
- Container-native
- Easy to override

### 6. Code Generation Convention

All generated code uses `zz_generated` prefix.

**Examples:**

- `zz_generated.deepcopy.go` (controller-gen)
- `zz_generated.mock.go` (mockery)

**Benefits:**

- Easy to identify generated code
- Easy to exclude from linting
- Clear .gitignore patterns

### 7. Test Tag Hierarchy

Build tags separate test types for selective execution.

**Hierarchy:**

```
unit < integration < functional < e2e
```

**Usage:**

```go
//go:build integration

package mypackage_test
```

**Benefits:**

- Fast feedback loop (run unit tests first)
- Selective CI execution
- Clear test classification

## Dependencies

### Go Modules

**Core Dependencies:**

- `github.com/caarlos0/env/v11` - Environment variable parsing
- `github.com/cert-manager/cert-manager` - Certificate management APIs
- `k8s.io/client-go` - Kubernetes client
- `k8s.io/api` - Kubernetes API types
- `sigs.k8s.io/controller-runtime` - Kubernetes controller framework
- `sigs.k8s.io/yaml` - YAML marshaling

**Test Dependencies:**

- `github.com/stretchr/testify` - Testing assertions and test suites

### External Tools

**Code Generation:**

- `controller-gen` - Kubernetes CRD/RBAC/webhook generation
- `oapi-codegen` - OpenAPI client/server generation
- `mockery` - Mock generation for interfaces
- `protoc` - Protocol buffer compilation

**Code Quality:**

- `gofumpt` - Stricter gofmt (stricter formatting rules)
- `golangci-lint` - Meta-linter (42+ linters)
- `gotestsum` - Test runner with enhanced output

**Container & Kubernetes:**

- `kaniko` - Container image builder (rootless, secure)
- `kind` - Kubernetes in Docker
- `helm` - Kubernetes package manager
- `kubectl` - Kubernetes CLI

**Container Engines:**

- Docker (default)
- Podman (supported)

### Dependency Management

**Renovate Configuration:** `.renovaterc`

**Features:**

- Automated dependency updates
- Separates major/minor/patch releases
- 3-day stability period before auto-merge
- Auto-merge strategy: rebase
- Post-update: `go mod tidy`
- Custom regex manager for Makefile tool versions

**Benefits:**

- Always up-to-date dependencies
- Automated security patches
- Consistent versioning

## Quality Assurance

### Linting

**Configuration:** `.golangci-lint.yml`

**Enabled Linters:** 42+ including:

- `gofmt`, `gofumpt` (formatting)
- `govet`, `staticcheck` (correctness)
- `gosec` (security)
- `errcheck` (error handling)
- `misspell` (spelling)

**Custom Rules:**

- Kubernetes import aliases enforced
- Generated code excluded (`zz_generated.*\.go`)
- 10-minute timeout
- Parallel execution

### Git Hooks

**Pre-push Hook:** `.githooks/pre-push`

**Checks:**

1. Run code generation
2. Format code
3. Run linter
4. Run tests
5. Verify generated files are committed

**Benefits:**

- Prevents pushing broken code
- Ensures generated code is up-to-date
- Maintains code quality standards

**Installation:**

```bash
make githooks
```

## Future Roadmap

### ci-orchestrator

**Vision:** Vendor-agnostic CI/CD orchestration platform

**Inspired by:** Kubernetes Prow (PR-based CI)

**Goals:**

- **Accessibility:** Run CI/CD anywhere (local, cloud, on-prem)
- **Security:** Proper secret management and isolation
- **Reproducibility:** Local environment = CI environment
- **Observability:** Quality gates, metrics, artifact management

**Status:** Design/brainstorming phase

**Potential Features:**

- Plugin architecture for different CI vendors
- Local CI execution for testing
- Quality gate definitions
- Artifact versioning and promotion
- Integration with local-container-registry

## Best Practices

### For Contributors

1. **Run Pre-push Validation:**

   ```bash
   make pre-push
   ```

2. **Use Test Tags Appropriately:**
   - `unit` - No external dependencies
   - `integration` - Requires test cluster
   - `functional` - End-to-end scenarios
   - `e2e` - Full system validation

3. **Document Packages:**
   - Every package should have a README.md
   - GoDoc comments for exported symbols

4. **Generated Code:**
   - Always commit generated code
   - Use `zz_generated` prefix
   - Run `make generate` after API changes

5. **Error Handling:**
   - Use `flaterrors.Join()` for multiple errors
   - Provide context in error messages
   - Don't silently ignore errors

6. **Configuration:**
   - Use `.project.yaml` for project configuration
   - Support environment variable overrides
   - Document all configuration options

### For Tool Development

1. **Environment-Driven:**
   - All configuration via environment variables
   - Use `github.com/caarlos0/env` for parsing
   - Support both ENV and config file

2. **Self-Contained:**
   - Minimal dependencies
   - Clear error messages
   - Proper exit codes

3. **Composable:**
   - Tools should work together
   - Use standard input/output
   - Follow Unix philosophy

4. **Testable:**
   - Write tests with appropriate tags
   - Mock external dependencies
   - Test error paths

## Conclusion

This tooling repository represents a well-architected, production-grade Go development toolkit with:

- **Strong separation of concerns** via adapters and packages
- **Modern Kubernetes patterns** with controller-runtime and cert-manager
- **Comprehensive testing** with multiple test tiers
- **Automated quality control** via linting, formatting, and git hooks
- **Reproducible environments** between local and CI
- **Unified build orchestration** via forge and MCP servers

The **forge CLI** is particularly noteworthy as a modern build orchestrator that demonstrates:
- Protocol-based extensibility (MCP)
- Unified artifact specification (BuildSpec)
- Comprehensive artifact tracking
- Integration environment lifecycle management

The **local-container-registry** component demonstrates advanced Kubernetes concepts including custom resource generation, secret management, TLS automation, and event-driven coordination.

This toolkit would be valuable for any organization building Go microservices on Kubernetes, especially those prioritizing reproducible local development environments, infrastructure-as-code principles, and unified build tooling.
