# Forge

## Why Forge?

Go projects face common development challenges:
- **Verbose Makefiles**: Complex build scripts become maintenance burdens
- **Inconsistent builds**: Different tools, different conventions, hard to reproduce
- **Manual test environments**: Setting up Kind + registry + TLS manually is error-prone
- **No artifact tracking**: Lost track of what was built when and with what version

Forge solves these by providing a unified, declarative approach to Go project workflows.

## How It Works

Forge uses **Model Context Protocol (MCP)** to orchestrate specialized build and test engines:

```
forge.yaml (declarative config)
      ↓
  forge CLI (orchestrator)
      ↓
  MCP protocol (stdio)
      ↓
  Engines (build-go, testenv, etc.)
```

All tools communicate via MCP, making them composable and extensible. Configure once in `forge.yaml`, run anywhere.

## What You Get

### Core Features

- **Unified Build System**: One `forge.yaml` for all artifacts (binaries, containers)
- **MCP-Based Engines**: 10 specialized tools for builds, tests, and environments
- **Test Environment Management**: Automated Kind clusters with TLS-enabled registries
- **Artifact Tracking**: Automatic versioning with git commit SHAs
- **20+ CLI Tools**: From code generation to E2E testing

### Quick Start

```bash
# Install forge
go install github.com/alexandremahdhaoui/forge/cmd/forge@latest

# Create forge.yaml
cat > forge.yaml <<EOF
name: my-project

build:
  artifactStorePath: .forge/artifacts.yaml
  specs:
    - name: my-app
      src: ./cmd/my-app
      dest: ./build/bin
      builder: go://build-go
EOF

# Build all artifacts
forge build

# Create test environment
forge test create unit

# Run tests
forge test run unit

# Cleanup
forge test delete <test-id>
```

## Available Tools

All 20 tools categorized by function. Tools marked ⚡ provide MCP servers.

**Build Tools (3)**
- ⚡ `build-go` - Go binary builder with git versioning
- ⚡ `build-container` - Container image builder using Kaniko
- ⚡ `generic-builder` - Execute any command as build step

**Test Tools (7)**
- ⚡ `testenv` - Test environment orchestrator
- ⚡ `testenv-kind` - Kind cluster manager
- ⚡ `testenv-lcr` - Local container registry with TLS
- ⚡ `test-runner-go` - Go test runner with JUnit/coverage
- ⚡ `test-runner-go-verify-tags` - Build tag verifier
- ⚡ `generic-test-runner` - Execute any command as test
- ⚡ `test-report` - Test report management

**Code Quality (3)**
- `format-go` - Go code formatter (gofumpt)
- `lint-go` - Go linter (golangci-lint)
- `test-go` - Legacy Go test runner

**Code Generation (3)**
- `generate-mocks` - Mock generator (mockery)
- `generate-openapi-go` - OpenAPI code generator
- `oapi-codegen-helper` - OpenAPI codegen helper

**Orchestration (4)**
- `forge` - Main CLI orchestrator
- `forge-e2e` - Forge end-to-end tests
- `chart-prereq` - Helm chart prerequisites
- `ci-orchestrator` - CI/CD orchestration (planning)

## Configuration: forge.yaml

Central declarative configuration file.

```yaml
name: my-project

# Build specifications
build:
  artifactStorePath: .forge/artifacts.yaml
  specs:
    - name: my-app
      src: ./cmd/my-app
      dest: ./build/bin
      builder: go://build-go

    - name: my-app-image
      src: ./Containerfile
      dest: registry.local:5000
      builder: go://build-container

# Test specifications
test:
  - name: unit
    stage: unit
    engine: go://testenv
    runner: go://test-runner-go

  - name: integration
    stage: integration
    engine: go://testenv
    runner: go://test-runner-go

# Test environment configuration
kindenv:
  kubeconfigPath: .forge/kubeconfig

localContainerRegistry:
  enabled: true
  namespace: testenv-lcr
  credentialPath: .forge/registry-credentials.yaml
  caCrtPath: .forge/ca.crt
```

## Usage Examples

### Building Artifacts

```bash
# Build all artifacts defined in forge.yaml
forge build

# Artifacts are tracked in artifact store
cat .forge/artifacts.yaml
```

### Managing Test Environments

```bash
# Create test environment for unit tests
TEST_ID=$(forge test create unit)

# List all test environments
forge test list

# Get test environment details
forge test get $TEST_ID

# Run tests in the environment
forge test run unit

# Cleanup when done
forge test delete $TEST_ID
```

### Direct Engine Usage

All MCP engines can be used standalone:

```bash
# Build Go binary directly
build-go --mcp <<EOF
{
  "method": "tools/call",
  "params": {
    "name": "build",
    "arguments": {
      "name": "my-app",
      "src": "./cmd/my-app"
    }
  }
}
EOF
```

## Documentation

- **[Forge CLI Usage](./docs/forge-usage.md)** - Complete forge command reference
- **[Forge Schema](./docs/forge-schema.md)** - forge.yaml field documentation
- **[Architecture](./ARCHITECTURE.md)** - System architecture and design patterns
- **[Test Environment Guide](./docs/testenv-quick-start.md)** - Using testenv system
- **[MCP Documentation](./docs/)** - MCP server documentation per tool

## Development

### Prerequisites

- Go 1.24.1+
- Docker or Podman
- Kind (for test environments)

### Building from Source

```bash
# Clone repository
git clone https://github.com/alexandremahdhaoui/forge
cd forge

# Build all tools using forge
go run ./cmd/forge build

# Binaries in ./build/bin/
ls build/bin/
```

### Running Tests

```bash
# Run all test stages
forge test run unit
forge test run integration
forge test run e2e
```

## Architecture

Forge uses MCP protocol for tool communication:

```
┌─────────────┐
│    forge    │ Orchestrator
│  (client)   │
└──────┬──────┘
       │ MCP over stdio
       ├────────────────┬────────────────┐
       │                │                │
┌──────▼──────┐  ┌──────▼──────┐  ┌──────▼──────┐
│  build-go   │  │   testenv   │  │ test-runner │
│  (server)   │  │  (server)   │  │   (server)  │
└─────────────┘  └──────┬──────┘  └─────────────┘
                        │
                 ┌──────┴──────┐
                 │             │
          ┌──────▼──────┐ ┌────▼────────┐
          │ testenv-kind│ │ testenv-lcr │
          │  (server)   │ │  (server)   │
          └─────────────┘ └─────────────┘
```

See [ARCHITECTURE.md](./ARCHITECTURE.md) for complete details.

## License

Apache 2.0

## Contributing

Issues and pull requests welcome at https://github.com/alexandremahdhaoui/forge
