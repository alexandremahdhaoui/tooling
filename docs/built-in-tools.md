# Built-in Tools Reference

This document provides a comprehensive reference for all built-in forge tools/engines. These tools are available out-of-the-box and don't require any configuration beyond specifying their URI in your `forge.yaml`.

## Table of Contents

- [Overview](#overview)
- [Build Engines](#build-engines)
- [Test Runners](#test-runners)
- [Test Environments](#test-environments)
- [Utility Tools](#utility-tools)
- [Quick Reference Table](#quick-reference-table)

## Overview

Forge includes 17 built-in tools/engines organized into categories:
- **4 Build Engines** - For building binaries and containers
- **5 Test Runners** - For executing tests
- **4 Test Environment Tools** - For managing test infrastructure
- **4 Utility Tools** - For code quality, generation, and management

All tools are MCP servers and can be used directly via their `go://` URI or wrapped in engine aliases for customization.

**Note:** This document covers the 17 built-in engines that forge orchestrates. The forge CLI itself (the 18th tool) is the orchestrator and is documented separately in [forge-usage.md](./forge-usage.md) and [cmd/forge/MCP.md](../cmd/forge/MCP.md).

## Build Engines

### go-build

**Purpose:** Build Go binaries with automatic version injection from git

**URI:** `go://go-build`

**Features:**
- Automatic version metadata injection via ldflags
- Git-based versioning (commit SHA, tags, dirty flag)
- Parallel build support
- Artifact tracking in artifact store

**Usage:**
```yaml
build:
  - name: myapp
    src: ./cmd/myapp
    dest: ./build/bin
    engine: go://go-build
```

**Version Injection:**
Automatically injects:
- `Version` - Git tag or commit SHA
- `CommitSHA` - Full commit hash
- `BuildTimestamp` - RFC3339 timestamp

**When to use:** For all Go binary builds. This is the preferred way to build Go applications.

---

### container-build

**Purpose:** Build container images with support for docker, kaniko, or podman

**URI:** `go://container-build`

**Features:**
- Multi-mode support: docker (native), kaniko (rootless), or podman (rootless)
- Supports both Dockerfile and Containerfile
- Automatic image tagging with git versions
- Build caching
- Multi-stage build support

**Usage:**
```yaml
build:
  - name: myapp-image
    src: ./Containerfile
    engine: go://container-build
```

**Environment Variables:**
- `CONTAINER_BUILD_ENGINE` - Build mode: docker, kaniko, or podman (required)
- `BUILD_ARGS` - Build arguments to pass to the build engine (optional)
- `KANIKO_CACHE_DIR` - Cache directory for kaniko mode (optional, default: ~/.kaniko-cache)

**Build Modes:**
- **docker**: Native Docker builds (fast, requires Docker daemon)
- **kaniko**: Rootless builds using Kaniko executor (runs in container via docker, secure)
- **podman**: Native Podman builds (rootless, requires Podman)

**When to use:** For building container images from Dockerfiles/Containerfiles with flexible backend selection.

---

### generic-builder

**Purpose:** Execute arbitrary commands as build steps

**URI:** `go://generic-builder`

**Features:**
- Run any CLI tool as a build engine
- Environment variable support
- Working directory control
- envFile support for secrets

**Usage:**
```yaml
engines:
  - alias: protoc
    type: builder
    builder:
      - engine: go://generic-builder
        spec:
          command: "protoc"
          args: ["--go_out=.", "api/service.proto"]
          workDir: "."

build:
  - name: generate-proto
    src: ./api
    engine: alias://protoc
```

**When to use:** When no built-in builder exists for your tool (protoc, npm, custom scripts, etc.)

**See Also:** [docs/prompts/use-generic-builder.md](./prompts/use-generic-builder.md)

---

### go-format

**Purpose:** Format Go code using gofmt and goimports

**URI:** `go://go-format`

**Features:**
- Runs gofmt -s -w
- Runs goimports -w
- Formats all .go files recursively
- Can be used as a build step

**Usage:**
```yaml
build:
  - name: format-code
    src: .
    engine: go://go-format
```

**When to use:** To ensure consistent Go code formatting before builds.

---

## Test Runners

### go-test

**Purpose:** Run Go tests with coverage and reporting

**URI:** `go://go-test`

**Features:**
- Uses gotestsum for better test output
- Generates JUnit XML reports
- Generates coverage profiles
- Supports build tags (unit, integration, functional, e2e)
- Race detector enabled
- Stores reports in artifact store

**Usage:**
```yaml
test:
  - name: unit
    runner: go://go-test

  - name: integration
    testenv: "alias://my-testenv"
    runner: go://go-test
```

**Build Tags:** Automatically uses `-tags=<stage-name>` (e.g., `-tags=unit`, `-tags=integration`)

**Environment Variables Passed to Tests:**
- `FORGE_TESTENV_TMPDIR` - Test environment temporary directory
- `FORGE_ARTIFACT_*` - Artifact file paths from testenv
- `FORGE_METADATA_*` - Metadata from testenv

**When to use:** For all Go test execution. This is the standard test runner.

---

### go-lint-tags

**Purpose:** Verify all test files have proper build tags

**URI:** `go://go-lint-tags`

**Features:**
- Scans all *_test.go files
- Ensures each has a `//go:build` tag
- Prevents tests from running in wrong stages
- Returns detailed error messages for violations

**Usage:**
```yaml
test:
  - name: verify-tags
    runner: go://go-lint-tags
```

**When to use:** As a pre-test validation step to ensure test isolation.

---

### generic-test-runner

**Purpose:** Execute arbitrary commands as test runners

**URI:** `go://generic-test-runner`

**Features:**
- Run any command as a test
- Pass/fail based on exit code (0 = pass)
- Generates structured TestReport
- Environment variable support

**Usage:**
```yaml
engines:
  - alias: shellcheck
    type: test-runner
    testRunner:
      - engine: go://generic-test-runner
        spec:
          command: "shellcheck"
          args: ["scripts/*.sh"]

test:
  - name: shell-lint
    runner: alias://shellcheck
```

**When to use:** When no built-in runner exists for your test tool.

**See Also:** [docs/prompts/use-generic-test-runner.md](./prompts/use-generic-test-runner.md)

---

### go-lint

**Purpose:** Run golangci-lint with auto-fix

**URI:** `go://go-lint`

**Features:**
- Runs golangci-lint run --fix ./...
- Automatically fixes issues where possible
- Returns pass/fail as test report
- Works with your .golangci.yml config

**Usage:**
```yaml
test:
  - name: lint
    runner: go://go-lint
```

**When to use:** For Go code linting. Prefer this over wrapping golangci-lint manually.

---

### forge-e2e

**Purpose:** Forge's end-to-end test framework

**URI:** `go://forge-e2e`

**Features:**
- Tests entire forge workflows
- Validates MCP protocol compliance
- Tests build and test orchestration
- Comprehensive forge integration tests

**Usage:**
```yaml
test:
  - name: e2e
    runner: go://forge-e2e
```

**When to use:** For comprehensive forge system tests (primarily for forge development).

---

## Test Environments

### testenv

**Purpose:** Complete test environment orchestrator

**URI:** `go://testenv`

**Features:**
- Orchestrates multiple testenv sub-engines
- Creates Kind clusters via testenv-kind
- Sets up local registries via testenv-lcr
- Installs Helm charts via testenv-helm-install
- Manages environment lifecycle (create, get, list, delete)
- Tracks environments in artifact store

**Usage:**
```yaml
# Option 1: Use default (creates Kind + registry)
test:
  - name: integration
    testenv: "go://testenv"
    runner: "go://go-test"

# Option 2: Use custom alias
engines:
  - alias: my-testenv
    type: testenv
    testenv:
      - engine: "go://testenv-kind"
      - engine: "go://testenv-lcr"
        spec:
          enabled: true

test:
  - name: integration
    testenv: "alias://my-testenv"
    runner: "go://go-test"
```

**When to use:** For integration tests requiring Kubernetes clusters and container registries.

---

### testenv-kind

**Purpose:** Create and manage Kind (Kubernetes in Docker) clusters

**URI:** `go://testenv-kind`

**Features:**
- Creates isolated Kind clusters
- Unique cluster names (forge-test-{stage}-{timestamp}-{random})
- Generates kubeconfig files
- Automatic cleanup on delete
- Stores cluster metadata

**Environment Variables Required:**
- `KIND_BINARY` - Path to kind binary (e.g., "kind")
- `KIND_BINARY_PREFIX` - Optional prefix (e.g., "sudo")

**Outputs:**
- `kubeconfig` file in testenv tmpDir
- Cluster name in metadata

**When to use:** When you need just a Kubernetes cluster (no registry).

---

### testenv-lcr

**Purpose:** Local Container Registry with TLS

**URI:** `go://testenv-lcr`

**Features:**
- Creates TLS-enabled container registry in Kind
- Generates CA certificates
- Auto-pushes images from artifact store
- Stores credentials and certs in testenv tmpDir

**Configuration:**
```yaml
engines:
  - alias: my-testenv
    type: testenv
    testenv:
      - engine: "go://testenv-kind"
      - engine: "go://testenv-lcr"
        spec:
          enabled: true
          autoPushImages: true
          namespace: "local-container-registry"
```

**Outputs:**
- Registry credentials
- CA certificate
- Registry endpoint

**When to use:** When tests need to push/pull container images.

---

### testenv-helm-install

**Purpose:** Install Helm charts into test environments

**URI:** `go://testenv-helm-install`

**Features:**
- Installs Helm charts from repos or local paths
- Waits for deployments to be ready
- Supports multiple charts
- Namespace creation
- Stores chart metadata

**Configuration:**
```yaml
engines:
  - alias: my-testenv
    type: testenv
    testenv:
      - engine: "go://testenv-kind"
      - engine: "go://testenv-helm-install"
        spec:
          charts:
            - name: podinfo/podinfo
              repo: https://stefanprodan.github.io/podinfo
              namespace: test-app
              releaseName: test-podinfo
```

**When to use:** When tests require specific applications/services in the cluster.

---

## Utility Tools

### test-report

**Purpose:** Manage test reports and artifacts

**URI:** `go://test-report`

**Features:**
- Query test reports from artifact store
- List reports by stage
- Get detailed report information
- Delete old reports and artifacts

**Commands:**
```bash
forge test report get <report-id>
forge test report list --stage=unit
forge test report delete <report-id>
```

**When to use:** For CI/CD pipelines to retrieve test results, or cleanup old reports.

---

### go-gen-mocks

**Purpose:** Generate Go mocks using mockery

**URI:** `go://go-gen-mocks`

**Features:**
- Generates mocks for Go interfaces
- Uses mockery under the hood
- Configurable output directories

**Usage:**
```yaml
build:
  - name: go-gen-mocks
    src: ./pkg
    dest: ./mocks
    engine: go://go-gen-mocks
```

**When to use:** For automated mock generation in Go projects.

---

### go-gen-openapi

**Purpose:** Generate Go client/server code from OpenAPI specs

**URI:** `go://go-gen-openapi`

**Features:**
- Generates Go code from OpenAPI 3.0 specs
- Creates both client and server stubs
- Version-aware generation

**Usage:** See `generateOpenAPI` section in forge.yaml schema

**When to use:** For projects using OpenAPI/Swagger specifications.

---

### ci-orchestrator

**Purpose:** CI pipeline orchestration (placeholder)

**URI:** `go://ci-orchestrator`

**Status:** Not yet implemented - returns "not yet implemented" error

**Planned Features:**
- Orchestrate multi-stage CI pipelines
- Parallel job execution
- Dependency management

**When to use:** Reserved for future CI/CD orchestration features.

---

## Quick Reference Table

| Tool | Category | URI | Primary Use |
|------|----------|-----|-------------|
| go-build | Build | `go://go-build` | Build Go binaries |
| container-build | Build | `go://container-build` | Build container images |
| generic-builder | Build | `go://generic-builder` | Wrap custom build tools |
| go-format | Build | `go://go-format` | Format Go code |
| go-test | Test Runner | `go://go-test` | Run Go tests |
| go-lint-tags | Test Runner | `go://go-lint-tags` | Verify build tags |
| generic-test-runner | Test Runner | `go://generic-test-runner` | Wrap custom test tools |
| go-lint | Test Runner | `go://go-lint` | Run golangci-lint |
| forge-e2e | Test Runner | `go://forge-e2e` | Forge system tests |
| testenv | Testenv | `go://testenv` | Full test environment |
| testenv-kind | Testenv | `go://testenv-kind` | Kind clusters |
| testenv-lcr | Testenv | `go://testenv-lcr` | Local container registry |
| testenv-helm-install | Testenv | `go://testenv-helm-install` | Helm chart installation |
| test-report | Utility | `go://test-report` | Test report management |
| go-gen-mocks | Utility | `go://go-gen-mocks` | Mock generation |
| go-gen-openapi | Utility | `go://go-gen-openapi` | OpenAPI code gen |
| ci-orchestrator | Utility | `go://ci-orchestrator` | CI orchestration (NYI) |

## Usage Patterns

### Standard Go Project

```yaml
build:
  - name: format-code
    src: .
    engine: go://go-format

  - name: myapp
    src: ./cmd/myapp
    dest: ./build/bin
    engine: go://go-build

test:
  - name: verify-tags
    runner: go://go-lint-tags

  - name: unit
    runner: go://go-test

  - name: lint
    runner: go://go-lint

  - name: integration
    testenv: "go://testenv"
    runner: go://go-test
```

### With Container Builds

```yaml
build:
  - name: myapp
    src: ./cmd/myapp
    dest: ./build/bin
    engine: go://go-build

  - name: myapp-image
    src: ./Containerfile
    engine: go://container-build

test:
  - name: integration
    testenv: "go://testenv"  # Includes registry
    runner: go://go-test
```

### Custom Tools Integration

```yaml
engines:
  - alias: npm-build
    type: builder
    builder:
      - engine: go://generic-builder
        spec:
          command: "npm"
          args: ["run", "build"]
          workDir: "./frontend"

  - alias: shellcheck
    type: test-runner
    testRunner:
      - engine: go://generic-test-runner
        spec:
          command: "shellcheck"
          args: ["scripts/*.sh"]

build:
  - name: frontend
    src: ./frontend
    engine: alias://npm-build

test:
  - name: shell-lint
    runner: alias://shellcheck
```

## Best Practices

1. **Prefer built-in tools over generic wrappers**
   - Use `go://go-build` instead of wrapping `go build`
   - Use `go://go-test` instead of wrapping `go test`

2. **Use generic-* tools for third-party integrations**
   - `generic-builder` for npm, protoc, custom scripts
   - `generic-test-runner` for shellcheck, custom validators

3. **Always verify build tags**
   - Add `verify-tags` as first test stage
   - Prevents tests running in wrong contexts

4. **Use testenv for integration tests**
   - Creates isolated, reproducible environments
   - Automatic cleanup
   - Consistent across developers and CI

5. **Format before building**
   - Add `go-format` as first build step
   - Ensures consistent code style

## Related Documentation

- [forge.yaml Schema](./forge-schema.md)
- [Using Generic Builder](./prompts/use-generic-builder.md)
- [Using Generic Test Runner](./prompts/use-generic-test-runner.md)
- [Test Environment Architecture](./testenv-architecture.md)
- [Forge Usage Guide](./forge-usage.md)

## MCP Documentation

Each tool has detailed MCP documentation in its source directory:
- See `cmd/<tool-name>/MCP.md` for tool-specific MCP protocol documentation
- See `cmd/<tool-name>/README.md` for implementation details
