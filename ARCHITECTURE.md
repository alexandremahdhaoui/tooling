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

Go development tooling repository providing CLI tools for streamlined workflows with focus on:

- Container and Kubernetes development
- Local test environments with MCP orchestration
- CI/CD operations
- Code generation

**Key Statistics:**

- **123 Go source files**
- **21,829 lines of Go code**
- **20 command-line tools**
- **5 public packages**
- **10 MCP servers**
- Go version: 1.24.1
- License: Apache 2.0

**Philosophy:** Dogfooding approach - tools build and test themselves, ensuring real-world reliability.

## Project Structure

```
/
├── cmd/                    # Command-line tools (20 tools)
│   ├── forge/             # Main CLI orchestrator
│   ├── build-*/           # Build engines (3)
│   ├── test*/             # Test engines and runners (7)
│   ├── generic-*/         # Generic engines (2)
│   └── */                 # Other tools (8)
├── pkg/                    # Public packages (5)
│   ├── forge/             # Core types and specs
│   ├── mcptypes/          # MCP protocol types
│   ├── mcputil/           # MCP utilities
│   ├── eventualconfig/    # Async config management
│   └── flaterrors/        # Error handling
├── internal/              # Internal utilities
├── docs/                  # Documentation
├── forge.yaml             # Central configuration
└── .ai/                   # AI assistant context
```

### Directory Responsibilities

#### `/cmd` - Command-Line Tools (20 tools)

Standalone CLI tools, each in its own subdirectory:
- Environment-variable driven for CI/CD
- Self-contained with minimal dependencies
- Composable via MCP protocol

See [Command-Line Tools](#command-line-tools) section for complete list.

#### `/pkg` - Public Packages (5 packages)

Reusable libraries importable by other Go projects:

- `forge` - Core types, BuildSpec, TestReport, artifact store
- `mcptypes` - MCP protocol types (BuildInput, RunInput, etc.)
- `mcputil` - MCP utilities (validation, batch handling, result formatting)
- `eventualconfig` - Async configuration with eventual consistency
- `flaterrors` - Error tree flattening

#### `/internal` - Internal Utilities

Private implementation details:

- `cmdutil` - Command execution utilities
- `gitutil` - Git operations (commit SHA, etc.)
- `mcpserver` - MCP server framework
- `enginetest` - Test helpers for engine development

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

**Use Case:** testenv-lcr uses this to coordinate between different setup phases (TLS, credentials, registry) that run concurrently but depend on each other's outputs.

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

**Use Case:** Throughout the codebase where multiple operations may fail independently (e.g., cleanup operations in testenv-lcr teardown).

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

**Configuration File:** `forge.yaml`

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

1. `forge.yaml` file (primary)
2. Environment variables (override)

## Command-Line Tools

All 20 tools are standalone binaries in `cmd/`. Tools marked with ⚡ provide MCP servers.

### Tool Categories

```
Build Engines (3):
  ⚡ build-go              - Go binary builder
  ⚡ build-container       - Container image builder (Kaniko)
  ⚡ generic-builder       - Generic command executor

Test Engines (3):
  ⚡ testenv              - Test environment orchestrator
  ⚡ testenv-kind         - Kind cluster manager
  ⚡ testenv-lcr          - Local container registry manager

Test Runners (3):
  ⚡ test-runner-go       - Go test runner with JUnit/coverage
  ⚡ test-runner-go-verify-tags - Build tag verifier
  ⚡ generic-test-runner  - Generic test command executor

Test Management (1):
  ⚡ test-report          - Test report aggregator

Code Quality (3):
  format-go              - Go code formatter
  lint-go                - Go linter wrapper
  test-go                - Legacy Go test runner

Code Generation (3):
  generate-mocks         - Mock generator
  generate-openapi-go    - OpenAPI code generator
  oapi-codegen-helper    - OpenAPI codegen helper

Orchestration (2):
  forge                  - Main CLI orchestrator
  forge-e2e              - Forge E2E tests

Planning (2):
  chart-prereq           - Helm chart dependencies
  ci-orchestrator        - CI/CD orchestration (planning phase)
```

### Core Tools

**forge** (`cmd/forge/`)
- Make-like build orchestrator using MCP protocol
- Manages builds, tests, and test environments
- See [Forge Architecture](#forge-architecture) section

**testenv** (`cmd/testenv/`)
- Orchestrates test environment creation/deletion
- Coordinates testenv-kind and testenv-lcr via MCP
- Manages TestEnvironment lifecycle in artifact store

**testenv-kind** (`cmd/testenv-kind/`)
- Creates/deletes Kind clusters for test environments
- Generates kubeconfig files per test
- MCP server for forge integration

**testenv-lcr** (`cmd/testenv-lcr/`)
- Deploys TLS-enabled container registry in Kind clusters
- Generates certificates and credentials
- Manages host file entries and registry access

See [Testing Infrastructure](#testing-infrastructure) for testenv architecture diagram.

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

## MCP Architecture

Forge uses Model Context Protocol (MCP) for communication between the orchestrator and tool engines.

### Communication Flow

```
┌─────────────┐
│    forge    │  Main orchestrator
│  (client)   │
└──────┬──────┘
       │ MCP over stdio
       ├────────────────┬────────────────┬────────────────┐
       │                │                │                │
┌──────▼──────┐  ┌──────▼──────┐  ┌──────▼──────┐  ┌──────▼──────┐
│  build-go   │  │   testenv   │  │ test-runner │  │   generic   │
│  (server)   │  │  (server)   │  │   (server)  │  │  (server)   │
└─────────────┘  └──────┬──────┘  └─────────────┘  └─────────────┘
                        │ Orchestrates sub-engines
                 ┌──────┴──────┐
                 │             │
          ┌──────▼──────┐ ┌────▼────────┐
          │ testenv-kind│ │ testenv-lcr │
          │  (server)   │ │  (server)   │
          └─────────────┘ └─────────────┘
```

### MCP Servers (10 total)

**Build Engines** (communicate via `build` tool):
- `build-go --mcp` - Returns Artifact
- `build-container --mcp` - Returns Artifact
- `generic-builder --mcp` - Returns Artifact

**Test Runners** (communicate via `run` tool):
- `test-runner-go --mcp` - Returns TestReport
- `test-runner-go-verify-tags --mcp` - Returns TestReport
- `generic-test-runner --mcp` - Returns TestReport

**Test Engines** (complex orchestration):
- `testenv --mcp` - Orchestrates testenv-kind + testenv-lcr
- `testenv-kind --mcp` - Manages Kind clusters
- `testenv-lcr --mcp` - Manages container registry

**Test Management**:
- `test-report --mcp` - Manages test reports (get/list/delete)

### Tool Registration in forge.yaml

```yaml
build:
  specs:
    - name: my-app
      src: ./cmd/my-app
      builder: go://build-go      # References MCP server

test:
  - name: unit
    engine: go://testenv          # References MCP server
    runner: go://test-runner-go   # References MCP server
```

The `go://` protocol indicates forge should spawn the binary with `--mcp` flag and communicate via stdio.

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

### Test Environment Architecture

```
forge test create <stage>
       │
       ▼
   ┌──────────┐
   │ testenv  │ Main orchestrator
   └─────┬────┘
         │
    ┌────┴────┐
    │         │
    ▼         ▼
┌─────────┐ ┌──────────┐
│testenv- │ │testenv-  │
│kind     │ │lcr       │
└─────────┘ └──────────┘
Creates:      Creates:
- Kind        - Registry
  cluster     - TLS certs
- Kubeconfig  - Credentials
```

**TestEnvironment Lifecycle:**

1. **Create**: `forge test create <stage>` → Returns testID
2. **Get**: `forge test get <testID>` → Returns TestEnvironment details
3. **List**: `forge test list [--stage=<stage>]` → Lists all environments
4. **Delete**: `forge test delete <testID>` → Cleans up all resources

**TestEnvironment Storage:**

Stored in artifact store (`.forge/artifacts.yaml`):

```go
type TestEnvironment struct {
    ID        string            // e.g., "test-unit-20250106-abc123"
    Name      string            // Stage name
    Status    string            // "created", "running", "failed"
    CreatedAt time.Time
    UpdatedAt time.Time
    TmpDir    string            // Temp directory for files
    Files     map[string]string // Relative paths in tmpDir
    Metadata  map[string]string // Component metadata
    ManagedResources []string   // Resources to clean up
}
```

### Test Execution Flow

```
forge test create unit      # Create test environment with testenv
forge test run unit         # Run tests with test-runner-go
forge test delete <testID>  # Clean up test environment
```

## Test Environment Components (testenv-*)

### testenv-lcr Architecture

**Location:** `cmd/testenv-lcr/`

**Design Pattern:** Adapter pattern with eventual consistency coordination.

**Purpose:** Deploys production-like container registry in Kind clusters with:

- TLS encryption (via cert-manager)
- htpasswd authentication
- Persistent storage
- Service exposure

See `docs/testenv-architecture.md` for complete testenv system architecture.

### testenv-lcr Components (Setup Adapters)

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
- DNS names: `testenv-lcr.testenv-lcr.svc.cluster.local`
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
- Write credentials to test environment tmpDir

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
├── Read forge.yaml
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

**Automatic Pruning:** The artifact store automatically retains only the 3 most recent build artifacts for each unique `type:name` combination. Test environments are not pruned and retain all historical data.

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
- `testenv-kind` - Kind cluster
- `testenv-lcr` - Local registry with TLS

**TestEnvironment Example:**

```yaml
testEnvironments:
  - id: test-unit-20250106-abc123
    name: unit
    stage: unit
    status: created
    created: "2025-01-06T10:00:00Z"
    tmpDir: .forge/tmp/test-unit-20250106-abc123
    files:
      testenv-kind.kubeconfig: kubeconfig
      testenv-lcr.ca.crt: ca.crt
      testenv-lcr.credentials.yaml: registry-credentials.yaml
    metadata:
      testenv-kind.clusterName: forge-test-unit-20250106-abc123
      testenv-kind.kubeconfigPath: .forge/tmp/test-unit-20250106-abc123/kubeconfig
      testenv-lcr.registryFQDN: testenv-lcr.testenv-lcr.svc.cluster.local:5000
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
Test Environments:
- test-unit-20250106-abc123
  Stage: unit
  Status: created
  Created: 2025-01-06T10:00:00Z
  Components:
    - testenv-kind: enabled
    - testenv-lcr: enabled
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
    Version          string                      `json:"version"`
    LastUpdated      time.Time                   `json:"lastUpdated"`
    Artifacts        []Artifact                  `json:"artifacts"`
    TestEnvironments map[string]*TestEnvironment `json:"testEnvironments,omitempty"`
}

// Read artifact store from file
func ReadArtifactStore(path string) (ArtifactStore, error)

// Read artifact store or create empty one if not exists
func ReadOrCreateArtifactStore(path string) (ArtifactStore, error)

// Write artifact store to file (automatically prunes old artifacts)
func WriteArtifactStore(path string, store ArtifactStore) error

// Add or update artifact
func AddOrUpdateArtifact(store *ArtifactStore, artifact Artifact)

// Prune old build artifacts (keeps 3 most recent per type:name)
func PruneBuildArtifacts(store *ArtifactStore, keepCount int)

// Get latest artifact by name
func GetLatestArtifact(store ArtifactStore, name string) (Artifact, error)

// Get artifacts by type
func GetArtifactsByType(store ArtifactStore, artifactType string) []Artifact
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

### forge.yaml

Central configuration file for the entire project.

**Structure:**

```yaml
name: tooling

# Build specifications
build:
  artifactStorePath: .forge/artifacts.yaml
  specs:
    - name: forge
      src: ./cmd/forge
      dest: ./build/bin
      builder: go://build-go

# Test specifications
test:
  - name: unit
    stage: unit
    engine: go://testenv
    runner: go://test-runner-go

# Kind cluster configuration
kindenv:
  kubeconfigPath: .forge/kubeconfig

# Local container registry configuration
localContainerRegistry:
  enabled: true
  namespace: testenv-lcr
  credentialPath: .forge/registry-credentials.yaml
  caCrtPath: .forge/ca.crt
```

**Configuration Loading:**

1. Parse `forge.yaml` file
2. Override with environment variables
3. Validate configuration

### Environment Variables

Tools support environment variable configuration with standardized naming:

**Format:** `{TOOL_NAME}_{FIELD_NAME}` or direct overrides

**Common Variables:**

```bash
# Forge
FORGE_ARTIFACT_STORE_PATH=.forge/artifacts.yaml

# Container engine
CONTAINER_ENGINE=docker  # or podman

# Test environment
KUBECONFIG=.forge/kubeconfig
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
   - Use `forge.yaml` for project configuration
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
