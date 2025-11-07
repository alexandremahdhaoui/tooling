# Forge

## Why Forge?

Modern software development faces common orchestration challenges:
- **Verbose build scripts**: Traditional Makefiles and build scripts become complex maintenance burdens
- **Inconsistent tooling**: Different projects use different conventions, making builds hard to reproduce
- **Poor AI agent integration**: Existing build tools weren't designed for AI-driven development workflows
- **No artifact tracking**: Teams lose track of what was built, when, and with what version
- **Manual environment setup**: Creating reproducible test environments is error-prone and time-consuming
- **Tool fragmentation**: Build, test, and development tools don't work together cohesively

Forge solves these with a **modern, declarative, AI-native approach** to build and development orchestration:

- **AI-Driven**: Deeply integrated with AI coding agents like Claude Code, enabling seamless AI-assisted development
- **Declarative & Simple**: Single `forge.yaml` configuration—no verbose scripts or complex Make syntax
- **Extensible**: MCP-based architecture makes adding new capabilities straightforward
- **Consistent**: Same commands, same configuration format across all projects and languages
- **Artifact Tracking**: Automatic versioning and tracking of all build and test artifacts
- **Test-Driven**: First-class support for test-driven development with automated environment management
- **Language-Agnostic**: While optimized for Go, the architecture supports any language or build system

## How It Works

Forge is built on **Model Context Protocol (MCP)**, the same protocol that powers AI coding agents like Claude Code. This architectural choice makes Forge uniquely suited for AI-driven development:

```
┌──────────────────────────────────────────────┐
│  AI Agent (e.g., Claude Code) or Developer  │
└────────────────┬─────────────────────────────┘
                 │
         ┌───────▼────────┐
         │  forge.yaml    │  Declarative configuration
         │  (your intent) │  What to build, test, deploy
         └───────┬────────┘
                 │
         ┌───────▼────────┐
         │   forge CLI    │  Orchestrator
         │  (understands  │  Interprets configuration
         │   your needs)  │  Manages execution flow
         └───────┬────────┘
                 │
          MCP protocol (stdio)
                 │
    ┌────────────┼────────────┐
    │            │            │
┌───▼───┐   ┌───▼───┐   ┌───▼────┐
│ build │   │ test  │   │testenv │  Specialized engines
│engines│   │runners│   │managers│  Composable via MCP
└───────┘   └───────┘   └────────┘
```

**Key Principles:**

1. **Declarative Configuration**: Define *what* you want in `forge.yaml`, not *how* to do it
2. **MCP Communication**: All engines speak MCP, making them composable and AI-accessible
3. **Artifact Tracking**: Every build, test, and deployment is tracked with git SHAs and timestamps
4. **AI-Native Design**: AI agents can read configurations, invoke engines, and interpret results naturally
5. **Test-Driven Workflow**: Automated environment creation, test execution, and cleanup in a single command

Configure once in `forge.yaml`, then use it from command line, CI/CD pipelines, or AI coding agents—all with the same consistent interface.

## Quick Start

### Installation

```bash
# Install from source
git clone https://github.com/alexandremahdhaoui/forge
cd forge
go build -o ./build/bin/forge ./cmd/forge

# Or install with go install
go install github.com/alexandremahdhaoui/forge/cmd/forge@latest
```

### Basic Usage

```bash
# Create forge.yaml
cat > forge.yaml <<EOF
name: my-project
artifactStorePath: .forge/artifacts.yaml

build:
  - name: my-app
    src: ./cmd/my-app
    dest: ./build/bin
    engine: go://build-go

test:
  - name: unit
    runner: go://test-runner-go
EOF

# Build all artifacts
forge build

# Run tests
forge test unit run

# Get help
forge --help
```

## Core Features

- **Unified Build System**: One `forge.yaml` for all artifacts (binaries, containers)
- **MCP-Based Architecture**: 11 specialized MCP server engines for extensibility
- **Test Environment Management**: Automated Kind clusters with TLS-enabled registries
- **Artifact Tracking**: Automatic versioning with git commit SHAs
- **18 CLI Tools**: From code generation to E2E testing

## Available Tools

All 18 tools categorized by function. Tools marked ⚡ provide MCP servers.

### Build Tools (3)
- ⚡ `build-go` - Go binary builder with git versioning
- ⚡ `build-container` - Container image builder using Kaniko
- ⚡ `generic-builder` - Execute any command as build step

### Test Tools (8)
- ⚡ `testenv` - Test environment orchestrator
- ⚡ `testenv-kind` - Kind cluster manager
- ⚡ `testenv-lcr` - Local container registry with TLS
- ⚡ `testenv-helm-install` - Helm chart installer for test environments
- ⚡ `test-runner-go` - Go test runner with JUnit/coverage
- ⚡ `test-runner-go-verify-tags` - Build tag verifier
- ⚡ `generic-test-runner` - Execute any command as test
- ⚡ `test-report` - Test report management

### Code Quality (2)
- `format-go` - Go code formatter (gofumpt)
- `lint-go` - Go linter (golangci-lint)

### Code Generation (2)
- `generate-mocks` - Mock generator (mockery)
- `generate-openapi-go` - OpenAPI code generator

### Orchestration (2)
- `forge` - Main CLI orchestrator
- `ci-orchestrator` - CI/CD orchestration (planning)

## Configuration: forge.yaml

Central declarative configuration file.

```yaml
name: my-project
artifactStorePath: .forge/artifacts.yaml

# Build specifications
build:
  - name: my-app
    src: ./cmd/my-app
    dest: ./build/bin
    engine: go://build-go

  - name: my-app-image
    src: ./containers/my-app/Containerfile
    engine: go://build-container

# Test specifications
test:
  - name: unit
    runner: go://test-runner-go

  - name: integration
    runner: go://test-runner-go
    testenv: alias://setup-integration

# Custom engine configurations
engines:
  - alias: setup-integration
    type: testenv
    testenv:
      - engine: go://testenv-kind
      - engine: go://testenv-lcr
        spec:
          enabled: true
          autoPushImages: true
```

For complete schema documentation, see [docs/forge-schema.md](./docs/forge-schema.md).

## Usage Examples

### Building Artifacts

```bash
# Build all artifacts defined in forge.yaml
forge build

# Build specific artifact
forge build my-app

# Artifacts are tracked in artifact store
cat .forge/artifacts.yaml
```

### Managing Test Environments

```bash
# Create test environment for integration tests
forge test integration create

# List all test environments
forge test integration list

# Get test environment details
forge test integration get <test-id>

# Run tests in the environment
forge test integration run

# Cleanup when done
forge test integration delete <test-id>
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

## Architecture

Forge uses the Model Context Protocol (MCP) to orchestrate specialized engines. Each tool (builder, test runner, environment manager) implements the MCP server interface, allowing them to be composed declaratively via `forge.yaml`.

```
┌─────────────┐
│    forge    │ Orchestrator (client)
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

**Key Architectural Principles:**
- **MCP Communication**: All engines use stdio-based MCP protocol
- **Composability**: Engines can be combined via `forge.yaml` configuration
- **Extensibility**: Add new engines by implementing MCP server interface
- **Dogfooding**: Forge builds and tests itself using its own tools

For complete architecture details, design patterns, and component descriptions, see [ARCHITECTURE.md](./ARCHITECTURE.md).

## Documentation

### User Documentation
- **[Forge CLI Usage](./docs/forge-usage.md)** - Complete forge command reference
- **[Forge Schema](./docs/forge-schema.md)** - forge.yaml field documentation
- **[Test Usage Guide](./docs/forge-test-usage.md)** - Test system usage patterns
- **[Test Environment Guide](./docs/testenv-quick-start.md)** - Quick start for test environments
- **[Generic Builder Guide](./docs/generic-builder-guide.md)** - Custom build commands

### Architecture Documentation
- **[Architecture Overview](./ARCHITECTURE.md)** - System architecture and design patterns
- **[Test Environment Architecture](./docs/testenv-architecture.md)** - Testenv system design

### Maintenance Documentation
- **[Documentation Conventions](./docs/doc-convention.md)** - How to write documentation
- **[Keeping Docs Updated](./docs/keeping-docs-up-to-date.md)** - Documentation maintenance guide

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
forge test verify-tags run
forge test unit run
forge test integration run
forge test e2e run

# Or run specific tests
go test ./...
```

### Project Statistics

- **18 CLI tools** across build, test, and code generation
- **11 MCP server** implementations
- **5 public packages** for reusable functionality
- **123 Go source files** with comprehensive tests
- **Go 1.24.1** with modern dependency management

## Contributing

Issues and pull requests welcome at https://github.com/alexandremahdhaoui/forge

When contributing:
1. Follow existing code patterns and conventions
2. Add tests for new functionality
3. Update documentation for user-facing changes
4. Ensure `forge test unit run` passes

## License

Apache 2.0
