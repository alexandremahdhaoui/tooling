# Go Development Tooling

This repository provides a collection of tools and utilities designed to streamline Go development workflows, particularly for projects involving containers and Kubernetes.

**Key Features:**
- **Forge CLI**: Make-like build orchestrator using MCP (Model Context Protocol) servers
- **Unified Build System**: Single configuration file (`forge.yaml`) for all artifacts
- **Integration Environments**: Managed Kind clusters with local container registries
- **Artifact Tracking**: Automatic versioning and metadata tracking

All tools are configured via a central `forge.yaml` file, allowing for consistent and reproducible builds, tests, and deployments.

## Table of Contents

- [Go Development Tooling](#go-development-tooling)
  - [Quick Start](#quick-start)
  - [Forge CLI](#forge-cli)
  - [Available Tools](#available-tools)
  - [Project Configuration (`forge.yaml`)](#project-configuration-forgeyaml)
    - [Example `forge.yaml`](#example-forgeyaml)
  - [Usage](#usage)
    - [`forge`](#forge)
    - [`build-go`](#build-go)
    - [`build-container`](#build-container)
    - [`kindenv`](#kindenv)
    - [`local-container-registry`](#local-container-registry)
    - [`oapi-codegen-helper`](#oapi-codegen-helper)
    - [`test-go`](#test-go)
  - [Examples](#examples)
    - [Containerfile](#containerfile)
      - [Go](#go)
    - [Makefile](#makefile)
  - [Documentation](#documentation)

## Quick Start

```bash
# 1. Create forge.yaml (or use existing)
cat > forge.yaml <<EOF
name: my-project

build:
  artifactStorePath: .ignore.artifact-store.yaml
  specs:
    - name: my-app
      src: ./cmd/my-app
      dest: ./build/bin
      builder: go://build-go

kindenv:
  kubeconfigPath: .ignore.kindenv.kubeconfig.yaml

localContainerRegistry:
  enabled: true
  autoPushImages: true
  credentialPath: .ignore.local-container-registry.yaml
  caCrtPath: .ignore.ca.crt
  namespace: local-container-registry
EOF

# 2. Build all artifacts
go run ./cmd/forge build

# 3. Create integration environment
go run ./cmd/forge integration create dev

# 4. Use the environment
export KUBECONFIG=.ignore.kindenv.kubeconfig.yaml
kubectl get nodes
```

See [docs/forge-usage.md](./docs/forge-usage.md) for complete usage guide.

## Forge CLI

**Forge** is a make-like build orchestrator that provides a unified interface for building artifacts and managing integration environments.

**Key Concepts:**
- **BuildSpec**: Unified specification for building any artifact (binaries, containers)
- **MCP Servers**: Build engines that communicate via Model Context Protocol
- **Artifact Store**: Automatic tracking of built artifacts with metadata
- **Integration Environments**: Managed Kind clusters with optional components

**Commands:**
- `forge build` - Build all artifacts defined in forge.yaml
- `forge integration create <name>` - Create integration environment
- `forge integration list` - List environments
- `forge integration get <id>` - Get environment details
- `forge integration delete <id>` - Delete environment

**Documentation:**
- [Forge CLI Usage Guide](./docs/forge-usage.md)
- [forge.yaml Schema Documentation](./docs/forge-schema.md)
- [Architecture Documentation](./ARCHITECTURE.md#forge-architecture)

## Available Tools

| Name                       | Description                                                                                                                                                                                                     |
|----------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `forge` | **Primary build orchestrator.** Builds all artifacts and manages integration environments using MCP servers. Configured via `forge.yaml`. |
| `build-go` | MCP server for building Go binaries. Used by forge as a build engine. Can also be invoked directly. |
| `build-container` | MCP server for building container images using Kaniko. Used by forge as a build engine. Can also be invoked directly. |
| `kindenv`                  | Manages Kind (Kubernetes in Docker) clusters for local development and testing. Outputs kubeconfig to path specified in `forge.yaml`. |
| `local-container-registry` | Creates a TLS-enabled container registry within Kind clusters. Configured via `forge.yaml`. |
| `oapi-codegen-helper`      | Wrapper for `oapi-codegen` that generates server and client code from OpenAPI specifications. Reads configuration from `forge.yaml` and parallelizes code generation. |
| `test-go` | Wrapper around `gotestsum` for executing scoped tests. Supports test tags (unit, integration, functional, e2e). |
| `chart-prereq` | Helper tool to install necessary Helm charts in Kubernetes clusters for testing. |
| `ci-orchestrator` | Tool for orchestrating CI jobs (work in progress). |
| `e2e` | End-to-end test script for `local-container-registry`. Now uses forge for building artifacts. |

## Project Configuration (`forge.yaml`)

The `forge.yaml` file is the central configuration file for all the tools in this repository. It allows you to declare the intent of your project and configure the behavior of the tools, including:
- **Build artifacts** (binaries and containers)
- **Integration environment** components
- **Artifact tracking** configuration

### Example `forge.yaml`

```yaml
name: my-project

# Build configuration
build:
  artifactStorePath: .ignore.artifact-store.yaml
  specs:
    # Go binaries
    - name: my-cli
      src: ./cmd/my-cli
      dest: ./build/bin
      builder: go://build-go

    - name: api-server
      src: ./cmd/api-server
      dest: ./build/bin
      builder: go://build-go

    # Container images
    - name: api-server
      src: ./containers/api-server/Containerfile
      dest: localhost:5000
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

# OpenAPI code generation
oapiCodegenHelper:
  defaults:
    sourceDir: "api"
    destinationDir: "pkg/api"
  specs:
    - name: "my-api"
      versions: ["v1"]
      client:
        enabled: true
        packageName: "myapiv1"
      server:
        enabled: true
        packageName: "myapiv1"
```

See [docs/forge-schema.md](./docs/forge-schema.md) for complete schema documentation.

## Usage

### `forge`

**Primary build orchestrator** - builds all artifacts and manages integration environments.

**Build all artifacts:**

```sh
# Build everything defined in forge.yaml
forge build

# With custom flags
GO_BUILD_LDFLAGS="-X main.Version=v1.0.0" CONTAINER_ENGINE=docker forge build
```

**Manage integration environments:**

```sh
# Create environment
forge integration create my-dev-env

# List environments
forge integration list

# Get environment details
forge integration get my-dev-env

# Delete environment
forge integration delete my-dev-env
```

**See:** [docs/forge-usage.md](./docs/forge-usage.md) for complete usage guide.

### `build-go`

This tool builds Go binaries. It can be used as an MCP server by forge or invoked directly.

**Environment Variables:**

* `BINARY_NAME`: The name of the binary to build (direct invocation only)
* `GO_BUILD_LDFLAGS`: The linker flags to pass to the `go build` command

**Direct invocation example:**

```sh
BINARY_NAME="my-app" GO_BUILD_LDFLAGS="-X main.Version=1.0.0" go run ./cmd/build-go
```

**Recommended:** Use forge instead for consistent builds.

### `build-container`

This tool builds container images using Kaniko. It can be used as an MCP server by forge or invoked directly.

**Environment Variables:**

* `CONTAINER_ENGINE`: The container engine to use (e.g., `docker`, `podman`)
* `CONTAINER_NAME`: The name of the container to build (direct invocation only)
* `BUILD_ARGS`: A list of build arguments to pass to the container build command
* `DESTINATIONS`: A list of destinations to push the container image to

**Direct invocation example:**

```sh
CONTAINER_ENGINE="docker" \
CONTAINER_NAME="my-app" \
BUILD_ARGS="VERSION=1.0.0" \
DESTINATIONS="docker.io/my-user/my-app:latest" \
go run ./cmd/build-container
```

**Recommended:** Use forge instead for consistent builds.

### `kindenv`

This tool manages a local Kubernetes cluster using Kind.

**Commands:**

* `setup`: Creates a Kind cluster.
* `teardown`: Deletes the Kind cluster.

**Example:**

```sh
go run github.com/alexandremahdhaoui/tooling/cmd/kindenv setup
go run github.com/alexandremahdhaoui/tooling/cmd/kindenv teardown
```

### `local-container-registry`

This tool sets up a local container registry in the Kind cluster.

**Commands:**

* `setup`: Sets up the local container registry.
* `teardown`: Tears down the local container registry.

**Example:**

```sh
go run github.com/alexandremahdhaoui/tooling/cmd/local-container-registry setup
go run github.com/alexandremahdhaoui/tooling/cmd/local-container-registry teardown
```

### `oapi-codegen-helper`

This tool generates Go code from OpenAPI specifications.

**Environment Variables:**

* `OAPI_CODEGEN`: The `oapi-codegen` command to use (e.g., `go run github.com/deepmap/oapi-codegen/cmd/oapi-codegen`).

**Example:**

```sh
OAPI_CODEGEN="go run github.com/deepmap/oapi-codegen/cmd/oapi-codegen" go run github.com/alexandremahdhaoui/tooling/cmd/oapi-codegen-helper
```

### `test-go`

This tool runs Go tests using `gotestsum`.

**Environment Variables:**

* `TEST_TAG`: The build tag to use for the tests (e.g., `unit`, `integration`).
* `GOTESTSUM`: The `gotestsum` command to use (e.g., `go run gotest.tools/gotestsum`).

**Example:**

```sh
TEST_TAG="unit" GOTESTSUM="go run gotest.tools/gotestsum" go run github.com/alexandremahdhaoui/tooling/cmd/test-go
```

## Examples

### Containerfile

#### Go

```Dockerfile
FROM docker.io/golang:1.23 as downloader

WORKDIR /workdir
COPY ./go.* ./
RUN go mod download

FROM downloader as builder

ARG GO_BUILD_LDFLAGS
ARG NAME=your-cmd
ARG INPUT_CMD="./cmd/${NAME}"
ARG OUTPUT_BIN="/bin/${NAME}"
WORKDIR /workdir
COPY . ./
RUN CG0_ENABLED=0 \
    GOOS=linux \
    go build \
      -ldflags "${GO_BUILD_LDFLAGS}" \
      -o "${OUTPUT_BIN}" \
      "${INPUT_CMD}"

FROM docker.io/alpine:3.20.1

ARG NAME=your-cmd
ARG OUTPUT_BIN="/bin/${NAME}"
COPY --from=builder ${OUTPUT_BIN} ${OUTPUT_BIN}
CMD [ "your-cmd" ]
```

### Makefile

```Makefile
# ------------------------------------------------------- ENVS ------------------------------------------------------- #

PROJECT    := <YOUR PROJECT NAME>

COMMIT_SHA := $(shell git rev-parse --short HEAD)
TIMESTAMP  := $(shell date --utc --iso-8601=seconds)
VERSION    ?= $(shell git describe --tags --always --dirty)

GO_BUILD_LDFLAGS ?= -X main.BuildTimestamp=$(TIMESTAMP) -X main.CommitSHA=$(COMMIT_SHA) -X main.Version=$(VERSION)

# ------------------------------------------------------- VERSIONS --------------------------------------------------- #

# renovate: datasource=github-release depName=kubernetes-sigs/controller-tools
CONTROLLER_GEN_VERSION := v0.14.0
# renovate: datasource=github-release depName=mvdan/gofumpt
GOFUMPT_VERSION        := v0.6.0
# renovate: datasource=github-release depName=golangci/golangci-lint
GOLANGCI_LINT_VERSION  := v1.59.1
# renovate: datasource=github-release depName=gotestyourself/gotestsum
GOTESTSUM_VERSION      := v1.12.0
# renovate: datasource=github-release depName=vektra/mockery
MOCKERY_VERSION        := v2.42.0
# renovate: datasource=github-release depName=oapi-codegen/oapi-codegen
OAPI_CODEGEN_VERSION   := v2.3.0
# renovate: datasource=github-release depName=alexandremahdhaoui/tooling
TOOLING_VERSION        := v0.1.4

# ------------------------------------------------------- TOOLS ------------------------------------------------------ #

CONTAINER_ENGINE   ?= docker
KIND_BINARY        ?= kind
KIND_BINARY_PREFIX ?= sudo

KINDENV_ENVS := KIND_BINARY_PREFIX="$(KIND_BINARY_PREFIX)" KIND_BINARY="$(KIND_BINARY)"

CONTROLLER_GEN      := go run sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_GEN_VERSION)
KINDENV             := KIND_BINARY="$(KIND_BINARY)" $(TOOLING)/kindenv@$(TOOLING_VERSION)
GO_GEN              := go generate
GOFUMPT             := go run mvdan.cc/gofumpt@$(GOFUMPT_VERSION)
GOLANGCI_LINT       := go run github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
GOTESTSUM           := go run gotest.tools/gotestsum@$(GOTESTSUM_VERSION) --format pkgname
MOCKERY             := go run github.com/vektra/mockery/v2@$(MOCKERY_VERSION)
OAPI_CODEGEN        := go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@$(OAPI_CODEGEN_VERSION)

TOOLING := go run github.com/alexandremahdhaoui/tooling/cmd

BUILD_BINARY        := GO_BUILD_LDFLAGS="$(GO_BUILD_LDFLAGS)" $(TOOLING)/build-binary@$(TOOLING_VERSION)
BUILD_CONTAINER     := CONTAINER_ENGINE="$(CONTAINER_ENGINE)" BUILD_ARGS="GO_BUILD_LDFLAGS=$(GO_BUILD_LDFLAGS)" $(TOOLING)/build-container@$(TOOLING_VERSION)
KINDENV             := KINDENV_ENVS="$(KINDENV_ENVS)" $(TOOLING)/kindenv@$(TOOLING_VERSION)
LOCAL_CONTAINER_REG := $(TOOLING)/local-container-registry@$(TOOLING_VERSION)
OAPI_CODEGEN_HELPER := OAPI_CODEGEN="$(OAPI_CODEGEN)" $(TOOLING)/oapi-codegen-helper@$(TOOLING_VERSION)
TEST_GO             := GOTESTSUM="$(GOTESTSUM)" $(TOOLING)/test-go@$(TOOLING_VERSION)

CLEAN_MOCKS := rm -rf ./internal/util/mocks

# ------------------------------------------------------- GENERATE --------------------------------------------------- #

.PHONY: sync-tooling
sync-tooling: ## Synchronize tooling scripts into this repository.
	echo TODO: implement 'make `sync-tooling`'

PROTO_FILES := $(shell find . ! -path '.*/\.*' -name "*.proto")
PROTOC_GEN_GO_OUT=--go_out=. --go_opt=paths=source_relative
PROTOC_GEN_GO_GRPC_OUT=--go-grpc_out=. --go-grpc_opt=paths=source_relative
COMPILE_PROTO_CMD = protoc $(PROTOC_GEN_GO_OUT) $(PROTOC_GEN_GO_GRPC_OUT) $<

FORCE_REBUILD:
	@:

# Rule to compile .proto files
%.pb.go: %.proto FORCE_REBUILD
	$(COMPILE_PROTO_CMD)

.PHONY: generate
generate: $(PROTO_FILES:.proto=.pb.go) ## Generate REST API server/client code, CRDs and other go generators.
	$(OAPI_CODEGEN_HELPER)
	$(GO_GEN) "./..."

	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."
	$(CONTROLLER_GEN) paths="./..." \
		crd:generateEmbeddedObjectMeta=true \
		output:crd:artifacts:config=charts/$(PROJECT)/templates/crds

	$(CONTROLLER_GEN) paths="./..." \
		rbac:roleName=$(PROJECT) \
		webhook \
		output:rbac:dir=charts/$(PROJECT)/templates/rbac \
		output:webhook:dir=charts/$(PROJECT)/templates/webhook

	$(CLEAN_MOCKS)
	$(MOCKERY)

# ------------------------------------------------------- BUILD BINARIES --------------------------------------------- #

.PHONY: build-binary
build-binary:
	$(BUILD_BINARY)

# ------------------------------------------------------- BUILD CONTAINERS -------------------------------------------- #

.PHONY: build-container
build-container:
	$(BUILD_CONTAINER)

# ------------------------------------------------------- FMT -------------------------------------------------------- #

.PHONY: fmt
fmt:
	$(GOFUMPT) -w .

# ------------------------------------------------------- LINT ------------------------------------------------------- #

.PHONY: lint
lint:
	$(GOLANGCI_LINT) run --fix

# ------------------------------------------------------- TEST ------------------------------------------------------- #

.PHONY: test-chart
test-chart:
	echo TODO: implement 'make `test-chart`'.

.PHONY: test-unit
test-unit:
	TEST_TAG=unit $(TEST_GO)

.PHONY: test-integration
test-integration:
	TEST_TAG=integration $(TEST_GO)

.PHONY: test-functional
test-functional:
	TEST_TAG=functional $(TEST_GO)

.PHONY: test-e2e
test-e2e:
	echo TODO: implement 'make `test-e2e`'

.PHONY: test-setup
test-setup:
	$(KINDENV) setup

.PHONY: test-teardown
test-teardown:
	$(KINDENV) teardown

.PHONY: test
test: test-unit test-setup test-integration test-functional test-teardown

# ------------------------------------------------------- PRE-PUSH --------------------------------------------------- #

.PHONY: githooks
githooks: ## Set up git hooks to run before a push.
	git config core.hooksPath .githooks

.PHONY: pre-push
pre-push: generate fmt lint test
	git status --porcelain

```

## Documentation

### Forge Documentation

- **[Forge CLI Usage Guide](./docs/forge-usage.md)** - Comprehensive usage guide with examples and workflows
- **[forge.yaml Schema Documentation](./docs/forge-schema.md)** - Complete schema reference for forge.yaml
- **[Architecture - Forge Section](./ARCHITECTURE.md#forge-architecture)** - Technical architecture and design

### Architecture

- **[ARCHITECTURE.md](./ARCHITECTURE.md)** - Complete architecture documentation
  - Core packages (eventualconfig, flaterrors, project)
  - Command-line tools
  - Forge architecture
  - Local container registry
  - Configuration management
  - Design patterns

### Additional Resources

- **[Model Context Protocol](https://modelcontextprotocol.io)** - MCP specification
- **[Makefile](./Makefile)** - Build automation and tool orchestration
- **[.project.yaml → forge.yaml Migration](./docs/forge-schema.md#migration-from-projectyaml)** - Migration guide

## Contributing

1. **Install pre-push hooks:**
   ```bash
   make githooks
   ```

2. **Run pre-push validation:**
   ```bash
   make pre-push
   ```

3. **Build with forge:**
   ```bash
   forge build
   ```

## License

Apache 2.0 - See LICENSE file for details.
