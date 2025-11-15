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

## What is Forge?

Forge is both a **command-line interface (CLI)** and an **MCP server**:
- **As a CLI:** Run directly from your terminal for builds, tests, and environment management
- **As an MCP server:** AI coding agents can invoke forge's capabilities programmatically
- **Architecture:** All forge components (CLI + engines) are MCP servers, creating a uniform, AI-accessible interface

This guide focuses on CLI usage. For MCP server usage, see [ARCHITECTURE.md](../ARCHITECTURE.md#mcp-architecture).

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
forge test --help
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
✅ Built: go-build (go-binary)
✅ Built: container-build (go-binary)
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
go run ./cmd/go-build --mcp <<EOF
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
      "builder": "go://go-build"
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
      builder: go://go-format

    # Then build binaries
    - name: my-app
      src: ./cmd/my-app
      dest: ./build/bin
      builder: go://go-build
```

#### How It Works

When you run `forge build`:

1. The `format-code` spec runs first
2. gofumpt v0.6.0 formats all `.go` files in the project
3. Binary builds proceed with formatted code

**Example output:**
```bash
$ forge build
Building 1 artifact(s) with go://go-format...
✅ Formatted Go code at .
Building 13 artifact(s) with go://go-build...
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
go run ./cmd/go-format

# Or if built locally
./build/bin/go-format
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
    runner: "go://go-lint"
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
forge test <operation> <stage> [args...]
```

**Operations:**

**Test Reports:**
- `run <stage> [ENV_ID]` - Run tests (optionally using existing environment)
- `list <stage>` - List test reports
- `get <stage> <TEST_ID>` - Get test report details
- `delete <stage> <TEST_ID>` - Delete test report

**Test Environments:**
- `list-env <stage>` - List test environments
- `get-env <stage> <ENV_ID>` - Get environment details
- `create-env <stage>` - Create test environment
- `delete-env <stage> <ENV_ID>` - Delete test environment

### Configure Test Stages

Define test stages in `forge.yaml`:

```yaml
# forge.yaml
test:
  # Unit tests - test-report only (no environment)
  - name: unit
    testenv: "go://test-report"
    runner: "go://go-test"

  # Integration tests - creates Kind cluster automatically
  - name: integration
    testenv: "go://testenv"
    runner: "go://go-test"

  # E2E tests
  - name: e2e
    testenv: "go://test-report"
    runner: "go://forge-e2e"

  # Linting as a test stage
  - name: lint
    testenv: "go://test-report"
    runner: "go://go-lint"
```

**Test Environment Types:**
- `go://test-report` - Test report storage only (no persistent environment)
  - Use for unit tests, linting, and other tests that don't need infrastructure
  - Shows synthetic "default" environment in `list-env`
  - Rejects `create-env` and `delete-env` operations
- `go://testenv` - Full test environment orchestrator
  - Creates Kind clusters, registries, and other infrastructure
  - Supports persistent environments for debugging
- `noop` or empty - Legacy option (equivalent to `go://test-report`)

### Run Tests

#### Unit Tests

Run fast, isolated unit tests:

```bash
# Run unit tests
forge test run unit
```

**Output:**
```
Running tests: stage=unit, name=test-report-unit-20251109-012345
✅ Unit tests passed

Test Results:
Status: passed
Total: 42
Passed: 42
Failed: 0
Coverage: 85.3%
```

**List test reports:**
```bash
# List all unit test reports
forge test list unit

# Get specific test report details
forge test get unit test-report-unit-20251109-012345

# Delete old test report
forge test delete unit test-report-unit-20251109-012345
```

#### Integration Tests

Run integration tests with automatic environment creation:

```bash
# Run integration tests (creates environment automatically)
forge test run integration

# Or use existing environment
forge test run integration <ENV_ID>
```

**What it does:**
1. Creates Kind cluster with local registry
2. Runs integration tests
3. Returns results
4. Environment persists for inspection

**Manage test environments:**
```bash
# List test environments
forge test list-env integration

# Get environment details
forge test get-env integration <ENV_ID>

# Create environment manually
forge test create-env integration

# Delete environment when done
forge test delete-env integration <ENV_ID>
```

#### Linting

Run linter as a test:

```bash
forge test run lint
```

### Manage Test Environments

For test stages with testenv orchestrators (like integration tests):

```bash
# Create environment manually
forge test create-env integration

# List all integration test environments
forge test list-env integration

# Get environment details
forge test get-env integration <ENV_ID>

# Delete when done
forge test delete-env integration <ENV_ID>
```

**Note:** Stages using `go://test-report` (like unit, lint) don't support environment management. They show a synthetic "default" environment and reject create/delete operations.

### Test Workflow Example

Complete testing workflow:

```bash
# 1. Format code
forge build format-code

# 2. Run unit tests
forge test run unit

# 3. Lint code
forge test run lint

# 4. Run integration tests
forge test run integration

# 5. View test reports
forge test list unit
forge test list integration

# 6. Clean up test environments (not test reports)
forge test list-env integration
forge test delete-env integration <ENV_ID>
```

## Integration Environments

Integration environments are complete development environments with Kind clusters and optional components like local container registries.

### Create Environment

Create a new test environment for a stage:

```bash
forge test create-env integration
```

**What it does:**
1. Generates unique environment ID
2. Creates Kind cluster (via testenv-kind)
3. Sets up local container registry with TLS (via testenv-lcr if configured)
4. Generates kubeconfig, credentials, and certificates
5. Records environment in artifact store

**Output:**
```
✅ Test environment created: env-integration-20251109-123456
```

### List Environments

View all test environments for a stage:

```bash
forge test list-env integration
```

**Output:**
```
=== Test Environments ===
ENV_ID                              STATUS      CREATED
----------------------------------------------------------------
env-integration-20251109-123456     created     2025-11-09 12:34
```

**JSON output:**
```bash
forge test list-env integration -ojson
```

### Get Environment Details

Get detailed information about a test environment:

```bash
forge test get-env integration env-integration-20251109-123456
```

**Output (YAML by default):**
```yaml
id: env-integration-20251109-123456
stage: integration
status: created
createdAt: "2025-11-09T12:34:56Z"
files:
  kubeconfig: .forge/integration-20251109-123456/kubeconfig.yaml
  ca.crt: .forge/integration-20251109-123456/ca.crt
  credentials.yaml: .forge/integration-20251109-123456/credentials.yaml
metadata:
  clusterName: test-integration-20251109-123456
  registryURL: registry.local:5000
```

**Status Values:**
- `created` - Environment created but not used
- `running` - Tests currently executing
- `passed` - Tests completed successfully
- `failed` - Tests failed
- `partially_deleted` - Cleanup incomplete

### Delete Environment

Tear down a test environment:

```bash
forge test delete-env integration env-integration-20251109-123456
```

**What it does:**
1. Tears down Kind cluster
2. Tears down local container registry
3. Deletes generated files (kubeconfig, credentials, CA cert)
4. Removes entry from artifact store

**Output:**
```
✅ Test environment deleted: env-integration-20251109-123456
```

### Use Test Environment

Once created, use the environment for development and testing:

```bash
# Get environment details to find kubeconfig
ENV_ID=$(forge test list-env integration -ojson | jq -r '.[0].id')
KUBECONFIG_PATH=$(forge test get-env integration $ENV_ID -oyaml | yq .files.kubeconfig)

# Set kubeconfig
export KUBECONFIG=$KUBECONFIG_PATH

# Verify cluster
kubectl cluster-info
kubectl get nodes

# Port-forward to registry (for pushing from host)
kubectl port-forward -n registry svc/registry 5000:5000 &

# Load credentials from environment details
CREDS_PATH=$(forge test get-env integration $ENV_ID -oyaml | yq '.files["credentials.yaml"]')
REGISTRY_USER=$(yq .username $CREDS_PATH)
REGISTRY_PASS=$(yq .password $CREDS_PATH)

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
forge test run lint

# 3. Run unit tests
forge test run unit

# 4. Run integration tests (creates environment automatically)
forge test run integration

# 5. View test reports
forge test list unit
forge test list integration

# 6. Clean up test environments (not test reports)
forge test list-env integration
forge test delete-env integration <ENV_ID>
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
forge test run lint

# 4. Run relevant tests
forge test run unit

# When ready to commit:
forge test run integration
```

### Workflow 3: Container Image Development

Build and push container images:

```bash
# 1. Create test environment with registry
forge test create-env integration

# 2. Build containers
CONTAINER_ENGINE=docker forge build

# 3. Get environment details
ENV_ID=$(forge test list-env integration -ojson | jq -r '.[0].id')
KUBECONFIG_PATH=$(forge test get-env integration $ENV_ID -oyaml | yq .files.kubeconfig)
CREDS_PATH=$(forge test get-env integration $ENV_ID -oyaml | yq '.files["credentials.yaml"]')

# 4. Port-forward registry
export KUBECONFIG=$KUBECONFIG_PATH
kubectl port-forward -n registry svc/registry 5000:5000 &

# 5. Login to registry
REGISTRY_USER=$(yq .username $CREDS_PATH)
REGISTRY_PASS=$(yq .password $CREDS_PATH)
docker login localhost:5000 -u "$REGISTRY_USER" -p "$REGISTRY_PASS"

# 6. Tag and push
docker tag my-api:latest localhost:5000/my-api:dev
docker push localhost:5000/my-api:dev

# 7. Deploy
kubectl apply -f k8s/deployment.yaml

# 8. Clean up when done
forge test delete-env integration $ENV_ID
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
forge test create-env integration

# Get test environment details
ENV_ID=$(forge test list-env integration -ojson | jq -r '.[0].id')
KUBECONFIG_PATH=$(forge test get-env integration $ENV_ID -oyaml | yq .files.kubeconfig)
export KUBECONFIG=$KUBECONFIG_PATH

echo "Running tests..."
forge test run integration $ENV_ID

# Cleanup
echo "Cleaning up..."
forge test delete-env integration $ENV_ID
```

### Workflow 5: Multi-Environment Testing

Test across different configurations:

```bash
# Create multiple test environments
forge test create-env integration  # Creates env 1
forge test create-env integration  # Creates env 2
forge test create-env integration  # Creates env 3

# List all environments
ENV_IDS=$(forge test list-env integration -ojson | jq -r '.[].id')

# Run tests in each
for env_id in $ENV_IDS; do
    echo "Testing in $env_id..."
    forge test run integration $env_id
done

# Clean up all
for env_id in $ENV_IDS; do
    forge test delete-env integration $env_id
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
| `KUBECONFIG` | Kubernetes config | `~/.kube/config` | `.forge/<test-id>/kubeconfig.yaml` |

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
      builder: go://go-build
```

### Custom Test Environment Configuration

Configure test stages in forge.yaml:

```yaml
# forge.yaml
test:
  - name: integration
    engine: "go://testenv"
    runner: "go://go-test"
    config:
      registry:
        enabled: true
        autoPush: true  # Automatically push images after build
```

**Workflow:**
```bash
# 1. Build images
forge build

# 2. Create environment and run tests
forge test integration run

# 3. Images are automatically available in cluster registry
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

.PHONY: test-unit
test-unit:
	$(FORGE) test unit run

.PHONY: test-integration
test-integration: build
	$(FORGE) test integration run
```

**Usage:**
```bash
make build
make test-unit
make test-integration
```

### JSON Output

Test environment commands return JSON for programmatic access:

```bash
# List test environments (returns JSON)
forge test integration list | jq .

# Get environment details (returns JSON)
TEST_ID=$(forge test integration list | jq -r '.environments[0].testID')
forge test integration get $TEST_ID | jq .
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
forge test integration create
```

#### "registry deployment failed"

**Problem:** Registry pod not starting in test environment.

**Solution:**
```bash
# Get test environment kubeconfig
TEST_ID=$(forge test integration list | jq -r '.environments[0].testID')
KUBECONFIG_PATH=$(forge test integration get $TEST_ID | jq -r '.files.kubeconfig')
export KUBECONFIG=$KUBECONFIG_PATH

# Check cluster
kubectl cluster-info

# Check registry namespace
kubectl get all -n registry

# Check registry pod logs
kubectl logs -n registry deployment/registry

# Check cert-manager (required for TLS)
kubectl get pods -n cert-manager

# If cert-manager is missing, it will be installed automatically
# Wait longer or check cert-manager logs
```

#### "cannot connect to registry"

**Problem:** Registry is running but unreachable.

**Solution:**
```bash
# Get test environment details
TEST_ID=$(forge test integration list | jq -r '.environments[0].testID')
KUBECONFIG_PATH=$(forge test integration get $TEST_ID | jq -r '.files.kubeconfig')
export KUBECONFIG=$KUBECONFIG_PATH

# Port-forward to registry
kubectl port-forward -n registry svc/registry 5000:5000 &

# Test connection
curl -k https://localhost:5000/v2/

# Get credentials from test environment
CREDS_PATH=$(forge test integration get $TEST_ID | jq -r '.files["credentials.yaml"]')

# Login
docker login localhost:5000 \
    -u $(yq .username $CREDS_PATH) \
    -p $(yq .password $CREDS_PATH)
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
# Get kubeconfig path from test environment
TEST_ID=$(forge test integration list | jq -r '.environments[0].testID')
KUBECONFIG_PATH=$(forge test integration get $TEST_ID | jq -r '.files.kubeconfig')

# Set KUBECONFIG
export KUBECONFIG=$KUBECONFIG_PATH

# Verify
kubectl cluster-info

# Or use inline
kubectl --kubeconfig=$KUBECONFIG_PATH get nodes
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
# Forge artifacts and test environments
.forge/

# Build outputs
build/
```

**Commit forge.yaml:**
```bash
git add forge.yaml
git commit -m "Add forge configuration"
```

### 2. Environment Lifecycle

**Always clean up test environments:**
```bash
# List integration test environments
forge test integration list

# Delete all integration test environments
TEST_IDS=$(forge test integration list | jq -r '.environments[].testID')
for test_id in $TEST_IDS; do
    forge test integration delete $test_id
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

**Create dedicated CI test script:**
```bash
#!/bin/bash
# ci-test.sh
set -e

# Create test environment
forge test integration create

# Get environment details
TEST_ID=$(forge test integration list | jq -r '.environments[0].testID')
KUBECONFIG_PATH=$(forge test integration get $TEST_ID | jq -r '.files.kubeconfig')

# Cleanup on exit
trap "forge test integration delete $TEST_ID" EXIT

# Run tests
export KUBECONFIG=$KUBECONFIG_PATH
forge test integration run $TEST_ID
```

### 5. Development Workflow

**Use persistent dev environment:**
```bash
# Create test environment once
forge test integration create

# Get and save environment ID
TEST_ID=$(forge test integration list | jq -r '.environments[0].testID')
echo "export TEST_ID=$TEST_ID" >> ~/.dev-env

# Get kubeconfig path
KUBECONFIG_PATH=$(forge test integration get $TEST_ID | jq -r '.files.kubeconfig')

# Add to ~/.bashrc or ~/.zshrc
echo "export KUBECONFIG=$KUBECONFIG_PATH" >> ~/.bashrc

# Use throughout development
source ~/.bashrc
kubectl get pods

# Clean up when done developing
forge test integration delete $TEST_ID
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
