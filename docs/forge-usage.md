# Forge CLI Usage Guide

This guide provides practical examples and workflows for using the forge CLI tool.

## Table of Contents

- [Quick Start](#quick-start)
- [Building Artifacts](#building-artifacts)
- [Code Quality](#code-quality)
  - [Code Formatting](#code-formatting)
  - [Linting](#linting)
- [Testing](#testing)
- [Integration Environments](#integration-environments)
- [Common Workflows](#common-workflows)
- [Environment Variables](#environment-variables)
- [Advanced Usage](#advanced-usage)
- [Troubleshooting](#troubleshooting)
- [Best Practices](#best-practices)

## Quick Start

### Installation

Install forge using `go install`:

```bash
# Install forge
go install github.com/alexandremahdhaoui/forge/cmd/forge@latest

# Add Go bin directory to PATH (if not already in your shell profile)
GOBIN_PATH=$(go env GOBIN)
if [ -z "$GOBIN_PATH" ]; then
  GOBIN_PATH=$(go env GOPATH)/bin
fi
export PATH="$GOBIN_PATH:$PATH"

# Verify installation
forge --help

# Check version
forge version
```

**Note:** Add the PATH export to your `~/.bashrc`, `~/.zshrc`, or equivalent shell profile to make it permanent.

### Version Information

Check the installed version of forge:

```bash
forge version
# Or use short flags
forge --version
forge -v
```

**Output example:**
```
forge version v0.2.1
  commit:    f42cc14
  built:     2025-11-03T19:30:40Z
  go:        go1.24.1
  platform:  linux/amd64
```

**Version Sources:**
- **When installed via `go install`**: Version info comes automatically from Go's build system (module version, VCS commit, build time)
- **When built with forge or make**: Version info comes from git tags and ldflags (`-X main.Version=...`)
- **Development builds**: Shows "dev" for version with available build info

### Basic Build

```bash
# Build all artifacts defined in forge.yaml
# Note: Automatically formats code if format-code is in build specs
forge build
```

### View Help

```bash
# General help
forge --help

# Command-specific help
forge build --help
forge integration --help
```

## Building Artifacts

### Build All Artifacts

The most common command - builds everything defined in `forge.yaml`:

```bash
forge build
```

**What it does:**
1. Reads `forge.yaml`
2. For each BuildSpec (in order):
   - If `format-code` is first, formats all Go code using gofumpt
   - Locates the appropriate build engine
   - Invokes engine via MCP protocol
   - Builds the artifact
   - Records metadata in artifact store
3. Updates `.ignore.artifact-store.yaml`

**Note:** Build specs are processed in the order they appear in `forge.yaml`. To ensure code is formatted before building binaries, place the `format-code` spec first.

**Output:**
```
🔨 Building artifacts from forge.yaml...
✅ Built: forge (go-binary)
✅ Built: build-go (go-binary)
✅ Built: build-container (go-binary)
✅ Built: my-api (container)
📦 Artifact store updated: .ignore.artifact-store.yaml
```

### Build with Custom Flags

#### Go Build Flags

Pass custom linker flags for Go builds:

```bash
# Add version information
GO_BUILD_LDFLAGS="-X main.Version=v1.0.0 -X main.Commit=abc123" forge build
```

**Common ldflags patterns:**

```bash
# Version and commit
VERSION=$(git describe --tags --always)
COMMIT=$(git rev-parse --short HEAD)
GO_BUILD_LDFLAGS="-X main.Version=$VERSION -X main.CommitSHA=$COMMIT" forge build

# Build timestamp
TIMESTAMP=$(date --utc --iso-8601=seconds)
GO_BUILD_LDFLAGS="-X main.BuildTimestamp=$TIMESTAMP" forge build
```

#### Container Engine Selection

Choose between Docker and Podman:

```bash
# Use Docker (default)
CONTAINER_ENGINE=docker forge build

# Use Podman
CONTAINER_ENGINE=podman forge build
```

#### Privileged Commands

Run container operations with elevated privileges:

```bash
# Use sudo for container operations
PREPEND_CMD=sudo CONTAINER_ENGINE=docker forge build
```

### Build Single Artifact

Currently, forge builds all artifacts. To build selectively, modify `forge.yaml` temporarily or use the underlying tools directly:

```bash
# Build just one binary
go run ./cmd/build-go --mcp <<EOF
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "build",
    "arguments": {
      "name": "my-cli",
      "src": "./cmd/my-cli",
      "dest": "./build/bin",
      "builder": "go://build-go"
    }
  }
}
EOF
```

### Verify Builds

Check the artifact store to see what was built:

```bash
# View artifact store
cat .ignore.artifact-store.yaml
```

**Example output:**
```yaml
artifacts:
  - name: forge
    type: binary
    location: file://./build/bin/forge
    timestamp: "2025-01-03T10:30:00Z"
    version: v1.0.0-abc123

  - name: my-api
    type: container
    location: localhost:5000/my-api:v1.0.0-abc123
    timestamp: "2025-01-03T10:31:00Z"
    version: v1.0.0-abc123
```

## Code Quality

Forge provides integrated code formatting and linting capabilities to maintain code quality.

### Code Formatting

Forge can automatically format your Go code using gofumpt (stricter gofmt) as part of the build process.

#### Configure Formatting in forge.yaml

Add `format-code` as the first build spec to ensure code is formatted before building:

```yaml
# forge.yaml
build:
  specs:
    # Format code first
    - name: format-code
      src: .
      builder: go://format-go

    # Then build binaries
    - name: my-app
      src: ./cmd/my-app
      dest: ./build/bin
      builder: go://build-go
```

#### How It Works

When you run `forge build`:

1. The `format-code` spec runs first
2. gofumpt v0.6.0 formats all `.go` files in the project
3. Binary builds proceed with formatted code

**Example output:**
```bash
$ forge build
Building 1 artifact(s) with go://format-go...
✅ Formatted Go code at .
Building 13 artifact(s) with go://build-go...
✅ Built binary: my-app (version: abc123)
```

#### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `GOFUMPT_VERSION` | gofumpt version to use | `v0.6.0` |

**Custom version:**
```bash
GOFUMPT_VERSION=v0.7.0 forge build
```

#### Manual Formatting

You can also format code manually without building:

```bash
# Direct invocation
go run ./cmd/format-go

# Or if built locally
./build/bin/format-go
```

### Linting

Forge integrates golangci-lint as a test stage for comprehensive code linting.

#### Configure Linting in forge.yaml

Add a lint test stage:

```yaml
# forge.yaml
test:
  - name: lint
    engine: "noop"  # No environment needed
    runner: "go://lint-go"
```

#### Run Linter

Execute the linter using the test command:

```bash
# Run linter
forge test lint run

# The linter will:
# - Install golangci-lint v2.6.0 if needed
# - Run all configured linters
# - Apply automatic fixes with --fix flag
# - Report issues
```

**Example output:**
```bash
$ forge test lint run
Running tests: stage=lint, name=lint-20251104-011133
✅ Linting passed

Test Results:
Status: passed
Total: 0
Passed: 1
Failed: 0
```

#### Golangci-lint Configuration

Golangci-lint uses:
- **Version:** v2.6.0 (from `github.com/golangci/golangci-lint/v2`)
- **Flags:** `--fix` (automatically fixes issues when possible)
- **Config:** Reads `.golangci.yml` or `.golangci.yaml` in your project root

**Create .golangci.yml:**
```yaml
# .golangci.yml
linters:
  enable:
    - gofmt
    - goimports
    - govet
    - errcheck
    - staticcheck
    - unused
    - gosimple
    - ineffassign

linters-settings:
  gofmt:
    simplify: true
```

#### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `GOLANGCI_LINT_VERSION` | golangci-lint version | `v2.6.0` |

**Custom version:**
```bash
GOLANGCI_LINT_VERSION=v2.7.0 forge test lint run
```

#### Integration with Makefile

```makefile
# Makefile
GOLANGCI_LINT_VERSION := v2.6.0
GOFUMPT_VERSION := v0.6.0

export GOLANGCI_LINT_VERSION
export GOFUMPT_VERSION

.PHONY: fmt
fmt:
	forge build format-code

.PHONY: lint
lint:
	forge test lint run

.PHONY: pre-commit
pre-commit: fmt lint
	git status
```

## Testing

Forge provides a unified test management system that supports multiple test stages including unit, integration, e2e, and linting.

### Test Command Structure

```bash
forge test <stage> <operation> [args...]
```

**Operations:**
- `run [test-id]` - Run tests for the stage
- `create` - Create test environment (for stages with engines)
- `list` - List test environments
- `get <test-id>` - Get environment details
- `delete <test-id>` - Delete test environment

### Configure Test Stages

Define test stages in `forge.yaml`:

```yaml
# forge.yaml
test:
  # Unit tests - no environment needed
  - name: unit
    engine: "noop"
    runner: "go://test-runner-go"

  # Integration tests - creates Kind cluster automatically
  - name: integration
    engine: "go://test-integration"
    runner: "go://test-runner-go"

  # E2E tests
  - name: e2e
    engine: "noop"
    runner: "go://forge-e2e"

  # Linting as a test stage
  - name: lint
    engine: "noop"
    runner: "go://lint-go"
```

### Run Tests

#### Unit Tests

Run fast, isolated unit tests:

```bash
# Run unit tests
forge test unit run
```

**Output:**
```
Running tests: stage=unit, name=unit-20251104-012345
✅ Unit tests passed

Test Results:
Status: passed
Total: 42
Passed: 42
Failed: 0
Coverage: 85.3%
```

#### Integration Tests

Run integration tests with automatic environment creation:

```bash
# Run integration tests (creates environment automatically)
forge test integration run

# Or use existing environment
forge test integration run <test-id>
```

**What it does:**
1. Creates Kind cluster with local registry
2. Runs integration tests
3. Returns results
4. Environment persists for inspection

**Clean up environment:**
```bash
forge test integration delete <test-id>
```

#### Linting

Run linter as a test:

```bash
forge test lint run
```

### Manage Test Environments

For test stages with engines (like integration tests):

```bash
# Create environment manually
forge test integration create

# List all integration test environments
forge test integration list

# Get environment details
forge test integration get <test-id>

# Delete when done
forge test integration delete <test-id>
```

### Test Workflow Example

Complete testing workflow:

```bash
# 1. Format code
forge build format-code

# 2. Run unit tests
forge test unit run

# 3. Lint code
forge test lint run

# 4. Run integration tests
forge test integration run

# 5. Clean up
forge test integration list
forge test integration delete <test-id>
```

## Integration Environments

Integration environments are complete development environments with Kind clusters and optional components like local container registries.

### Create Environment

Create a new integration environment:

```bash
forge integration create my-dev-env
```

**What it does:**
1. Generates unique environment ID
2. Creates Kind cluster (if kindenv is configured)
3. Sets up local container registry (if enabled)
4. Generates credentials and certificates
5. Records environment in `.ignore.integration-envs.yaml`

**Output:**
```
🔧 Creating integration environment: my-dev-env
📦 Setting up kindenv...
✅ Kind cluster created
📦 Setting up local-container-registry...
✅ Registry deployed
✅ Environment created (ID: abc123-def456)

Environment Details:
- Name: my-dev-env
- ID: abc123-def456
- Kubeconfig: .ignore.kindenv.kubeconfig.yaml
- Registry: local-container-registry.local-container-registry.svc.cluster.local:5000
- Credentials: .ignore.local-container-registry.yaml
```

### List Environments

View all integration environments:

```bash
forge integration list
```

**Output:**
```
Integration Environments:

1. my-dev-env (ID: abc123-def456)
   Created: 2025-01-03T10:00:00Z
   Components:
     - kindenv: enabled, ready
     - local-container-registry: enabled, ready

2. testing-env (ID: xyz789-uvw012)
   Created: 2025-01-03T11:00:00Z
   Components:
     - kindenv: enabled, ready
     - local-container-registry: disabled
```

### Get Environment Details

Get detailed information about a specific environment:

```bash
# By ID
forge integration get abc123-def456

# By name
forge integration get my-dev-env
```

**Output:**
```
Environment: my-dev-env
ID: abc123-def456
Created: 2025-01-03T10:00:00Z

Components:
  kindenv:
    Enabled: true
    Ready: true
    Connection Info:
      - kubeconfig: .ignore.kindenv.kubeconfig.yaml

  local-container-registry:
    Enabled: true
    Ready: true
    Connection Info:
      - namespace: local-container-registry
      - credentialsFile: .ignore.local-container-registry.yaml
      - caCertFile: .ignore.ca.crt
```

### Delete Environment

Tear down an integration environment:

```bash
# By ID
forge integration delete abc123-def456

# By name
forge integration delete my-dev-env
```

**What it does:**
1. Tears down Kind cluster
2. Tears down local container registry
3. Deletes generated files (kubeconfig, credentials, CA cert)
4. Removes entry from environment store

**Output:**
```
🗑️  Deleting integration environment: my-dev-env
🗑️  Tearing down local-container-registry...
✅ Registry removed
🗑️  Tearing down kindenv...
✅ Kind cluster deleted
🧹 Cleaning up files...
✅ Environment deleted
```

### Use Integration Environment

Once created, use the environment for development and testing:

```bash
# Set kubeconfig
export KUBECONFIG=.ignore.kindenv.kubeconfig.yaml

# Verify cluster
kubectl cluster-info
kubectl get nodes

# Port-forward to registry (for pushing from host)
kubectl port-forward -n local-container-registry svc/local-container-registry 5000:5000 &

# Load credentials
REGISTRY_USER=$(yq .username .ignore.local-container-registry.yaml)
REGISTRY_PASS=$(yq .password .ignore.local-container-registry.yaml)

# Login to registry
docker login localhost:5000 -u "$REGISTRY_USER" -p "$REGISTRY_PASS"

# Push images
docker push localhost:5000/my-api:v1.0.0
```

## Common Workflows

### Workflow 1: Fresh Build and Test

Complete workflow from build to testing with code quality checks:

```bash
# 1. Build all artifacts (automatically formats code)
forge build

# 2. Run linter
forge test lint run

# 3. Run unit tests
forge test unit run

# 4. Run integration tests (creates environment automatically)
forge test integration run

# 5. Clean up test environments
forge test integration list
forge test integration delete <test-id>
```

### Workflow 2: Iterative Development

Quick iteration during development:

```bash
# Development loop:
# 1. Make code changes
vim cmd/my-app/main.go

# 2. Format and build (format happens automatically)
forge build

# 3. Quick lint check
forge test lint run

# 4. Run relevant tests
forge test unit run

# When ready to commit:
forge test integration run
```

### Workflow 3: Container Image Development

Build and push container images:

```bash
# 1. Create environment with registry
forge integration create dev

# 2. Build containers
CONTAINER_ENGINE=docker forge build

# 3. Port-forward registry
export KUBECONFIG=.ignore.kindenv.kubeconfig.yaml
kubectl port-forward -n local-container-registry svc/local-container-registry 5000:5000 &

# 4. Login to registry
REGISTRY_USER=$(yq .username .ignore.local-container-registry.yaml)
REGISTRY_PASS=$(yq .password .ignore.local-container-registry.yaml)
docker login localhost:5000 -u "$REGISTRY_USER" -p "$REGISTRY_PASS"

# 5. Tag and push
docker tag my-api:latest localhost:5000/my-api:dev
docker push localhost:5000/my-api:dev

# 6. Deploy
kubectl apply -f k8s/deployment.yaml
```

### Workflow 4: CI/CD Pipeline

Automate builds in CI/CD:

```bash
#!/bin/bash
set -e

# Environment setup
export GO_BUILD_LDFLAGS="-X main.Version=$CI_COMMIT_TAG -X main.Commit=$CI_COMMIT_SHA"
export CONTAINER_ENGINE=docker

# Build phase
echo "Building artifacts..."
forge build

# Test phase
echo "Creating test environment..."
forge integration create ci-test-$CI_JOB_ID

export KUBECONFIG=.ignore.kindenv.kubeconfig.yaml

echo "Running tests..."
make test-integration

# Cleanup
echo "Cleaning up..."
forge integration delete ci-test-$CI_JOB_ID
```

### Workflow 5: Multi-Environment Testing

Test across different configurations:

```bash
# Create multiple environments
forge integration create env-basic
forge integration create env-advanced
forge integration create env-minimal

# Run tests in each
for env in env-basic env-advanced env-minimal; do
    echo "Testing in $env..."
    forge integration get $env
    # Run your tests here
done

# Clean up all
for env in env-basic env-advanced env-minimal; do
    forge integration delete $env
done
```

## Environment Variables

Forge respects several environment variables:

### Build-Related

| Variable | Description | Default | Example |
|----------|-------------|---------|---------|
| `GO_BUILD_LDFLAGS` | Go linker flags | None | `-X main.Version=v1.0.0` |
| `CONTAINER_ENGINE` | Container engine | `docker` | `podman` |
| `PREPEND_CMD` | Command prefix | None | `sudo` |

### Code Quality-Related

| Variable | Description | Default | Example |
|----------|-------------|---------|---------|
| `GOFUMPT_VERSION` | gofumpt version for formatting | `v0.6.0` | `v0.7.0` |
| `GOLANGCI_LINT_VERSION` | golangci-lint version for linting | `v2.6.0` | `v2.7.0` |

### Test-Related

| Variable | Description | Default | Example |
|----------|-------------|---------|---------|
| `GOTESTSUM_VERSION` | gotestsum version for test runner | `v1.12.0` | `v1.13.0` |

### Environment-Related

| Variable | Description | Default | Example |
|----------|-------------|---------|---------|
| `KIND_BINARY` | Kind binary name | `kind` | `/usr/local/bin/kind` |
| `KIND_BINARY_PREFIX` | Kind command prefix | None | `sudo` |
| `KUBECONFIG` | Kubernetes config | `~/.kube/config` | `.ignore.kindenv.kubeconfig.yaml` |

### Example with All Variables

```bash
# Build with all options
GO_BUILD_LDFLAGS="-X main.Version=v1.0.0" \
CONTAINER_ENGINE=podman \
PREPEND_CMD=sudo \
GOFUMPT_VERSION=v0.7.0 \
forge build

# Test with custom versions
GOLANGCI_LINT_VERSION=v2.7.0 \
GOTESTSUM_VERSION=v1.13.0 \
KIND_BINARY=kind \
KIND_BINARY_PREFIX=sudo \
forge test integration run
```

## Advanced Usage

### Custom Artifact Store Location

Override the default artifact store path:

```yaml
# forge.yaml
build:
  artifactStorePath: ./custom-path/artifacts.yaml
  specs:
    - name: my-app
      src: ./cmd/my-app
      dest: ./build/bin
      builder: go://build-go
```

### Auto-Push Images on Environment Setup

Configure automatic image pushing:

```yaml
# forge.yaml
localContainerRegistry:
  enabled: true
  autoPushImages: true  # Automatically push artifacts on create
```

**Workflow:**
```bash
# 1. Build images
forge build

# 2. Create environment (automatically pushes images)
forge integration create dev

# 3. Images are already in registry
kubectl run my-app --image=localhost:5000/my-api:v1.0.0
```

### Makefile Integration

Integrate forge into your Makefile:

```makefile
# Makefile
FORGE := GO_BUILD_LDFLAGS="$(GO_BUILD_LDFLAGS)" \
         CONTAINER_ENGINE="$(CONTAINER_ENGINE)" \
         go run ./cmd/forge

.PHONY: build
build:
	$(FORGE) build

.PHONY: test-setup
test-setup: build
	$(FORGE) integration create test
	@echo "Run: export KUBECONFIG=.ignore.kindenv.kubeconfig.yaml"

.PHONY: test-teardown
test-teardown:
	$(FORGE) integration delete test
```

**Usage:**
```bash
make build
make test-setup
# Run your tests
make test-teardown
```

### JSON Output (Future Feature)

For programmatic access:

```bash
# Future feature - not yet implemented
forge integration list --output=json | jq .
```

## Troubleshooting

### Build Issues

#### "engine not found"

**Problem:** Forge cannot locate the build engine binary.

**Solution:**

Forge automatically installs missing engines. If you encounter this error:
```bash
# Forge will automatically install missing engines on first use
forge build
```

**Note:** Engines are automatically installed from the forge repository when needed. No manual installation is required.

#### "go build failed"

**Problem:** Go compilation errors.

**Solution:**
```bash
# Check syntax errors
go build ./cmd/my-app

# Check for missing dependencies
go mod tidy
go mod download

# Try again
forge build
```

#### "container build failed"

**Problem:** Container image build failure.

**Solution:**
```bash
# Verify Containerfile exists
ls -la containers/my-app/Containerfile

# Test Containerfile manually
docker build -f containers/my-app/Containerfile -t test:latest .

# Check CONTAINER_ENGINE
echo $CONTAINER_ENGINE  # Should be 'docker' or 'podman'

# Try with explicit engine
CONTAINER_ENGINE=docker forge build
```

### Integration Environment Issues

#### "kind cluster creation failed"

**Problem:** Cannot create Kind cluster.

**Solution:**
```bash
# Check if kind is installed
kind version

# Check if Docker/Podman is running
docker info   # or: podman info

# Check existing clusters
kind get clusters

# Try creating manually
kind create cluster --name test-cluster

# If successful, try forge again
forge integration create test-env
```

#### "local-container-registry deployment failed"

**Problem:** Registry pod not starting.

**Solution:**
```bash
# Check cluster
export KUBECONFIG=.ignore.kindenv.kubeconfig.yaml
kubectl cluster-info

# Check registry namespace
kubectl get all -n local-container-registry

# Check registry pod logs
kubectl logs -n local-container-registry deployment/local-container-registry

# Check cert-manager (required for TLS)
kubectl get pods -n cert-manager

# If cert-manager is missing, it will be installed automatically
# Wait longer or check cert-manager logs
```

#### "cannot connect to registry"

**Problem:** Registry is running but unreachable.

**Solution:**
```bash
# Port-forward to registry
kubectl port-forward -n local-container-registry svc/local-container-registry 5000:5000 &

# Test connection
curl -k https://localhost:5000/v2/

# Check credentials
cat .ignore.local-container-registry.yaml

# Login
docker login localhost:5000 \
    -u $(yq .username .ignore.local-container-registry.yaml) \
    -p $(yq .password .ignore.local-container-registry.yaml)
```

### File and Permission Issues

#### "permission denied"

**Problem:** Cannot write files or execute binaries.

**Solution:**
```bash
# Check write permissions
ls -la .ignore.artifact-store.yaml

# Fix permissions
chmod 644 .ignore.artifact-store.yaml

# For container operations, use PREPEND_CMD
PREPEND_CMD=sudo CONTAINER_ENGINE=docker forge build
```

#### "artifact store corrupted"

**Problem:** Artifact store file has invalid YAML.

**Solution:**
```bash
# Backup current file
cp .ignore.artifact-store.yaml .ignore.artifact-store.yaml.backup

# Delete and rebuild
rm .ignore.artifact-store.yaml
forge build

# Or manually fix YAML syntax
vim .ignore.artifact-store.yaml
```

### Environment Variable Issues

#### "KUBECONFIG not set"

**Problem:** kubectl cannot find the cluster.

**Solution:**
```bash
# Set KUBECONFIG
export KUBECONFIG=.ignore.kindenv.kubeconfig.yaml

# Verify
kubectl cluster-info

# Or use inline
kubectl --kubeconfig=.ignore.kindenv.kubeconfig.yaml get nodes
```

#### "GO_BUILD_LDFLAGS not working"

**Problem:** Version info not injected.

**Solution:**
```bash
# Check your main.go has variables
cat cmd/my-app/main.go | grep "var Version"

# Should have:
# var Version string
# var CommitSHA string

# Use correct flag format
GO_BUILD_LDFLAGS="-X main.Version=v1.0.0" forge build

# Verify in binary
./build/bin/my-app --version
```

## Best Practices

### 1. Version Control

**Add to .gitignore:**
```gitignore
# Forge artifacts
.ignore.artifact-store.yaml
.ignore.kindenv.kubeconfig.yaml
.ignore.local-container-registry.yaml
.ignore.ca.crt
.ignore.integration-envs.yaml

# Build outputs
build/
```

**Commit forge.yaml:**
```bash
git add forge.yaml
git commit -m "Add forge configuration"
```

### 2. Environment Lifecycle

**Always clean up environments:**
```bash
# List before deleting
forge integration list

# Delete all test environments
for env in $(forge integration list | grep test- | awk '{print $1}'); do
    forge integration delete $env
done
```

### 3. Build Reproducibility

**Use consistent build flags:**
```bash
# Define in Makefile
VERSION := $(shell git describe --tags --always)
COMMIT := $(shell git rev-parse --short HEAD)
TIMESTAMP := $(shell date --utc --iso-8601=seconds)

GO_BUILD_LDFLAGS := -X main.Version=$(VERSION) \
                    -X main.CommitSHA=$(COMMIT) \
                    -X main.BuildTimestamp=$(TIMESTAMP)

export GO_BUILD_LDFLAGS
```

### 4. CI/CD Integration

**Create dedicated CI environment:**
```bash
#!/bin/bash
# ci-build.sh
set -e

ENV_NAME="ci-${CI_JOB_ID:-local}"

# Cleanup on exit
trap "forge integration delete $ENV_NAME" EXIT

# Create environment
forge integration create $ENV_NAME

# Run tests
export KUBECONFIG=.ignore.kindenv.kubeconfig.yaml
make test-integration
```

### 5. Development Workflow

**Use persistent dev environment:**
```bash
# Setup once
forge integration create dev
export KUBECONFIG=.ignore.kindenv.kubeconfig.yaml

# Add to ~/.bashrc or ~/.zshrc
alias dev-env='export KUBECONFIG=/path/to/repo/.ignore.kindenv.kubeconfig.yaml'

# Use throughout development
dev-env
kubectl get pods
```

### 6. Documentation

**Document custom build requirements in README.md:**
```markdown
## Building

```bash
# Build all components
forge build

# Build with version info
VERSION=v1.0.0 GO_BUILD_LDFLAGS="-X main.Version=$VERSION" forge build
```
```

## See Also

- [forge.yaml Schema Documentation](./forge-schema.md) - Complete schema reference
- [ARCHITECTURE.md - Forge Architecture](../ARCHITECTURE.md#forge-architecture) - Technical architecture
- [Model Context Protocol](https://modelcontextprotocol.io) - MCP specification

## Getting Help

- **Issues:** Report bugs at `github.com/your-org/tooling/issues`
- **Questions:** Ask in discussions or team chat
- **Documentation:** Check `docs/` directory for detailed guides
