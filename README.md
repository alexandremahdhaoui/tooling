# Go Development Tooling

This repository provides a collection of tools and utilities designed to streamline Go development workflows, particularly for projects involving containers and Kubernetes. These tools are configured via a central `.project.yaml` file, allowing for consistent and reproducible builds, tests, and deployments.

## Table of Contents

- [Go Development Tooling](#go-development-tooling)
  - [Available Tools](#available-tools)
  - [Project Configuration (`.project.yaml`)](#project-configuration-projectyaml)
    - [Example `.project.yaml`](#example-projectyaml)
  - [Usage](#usage)
    - [`build-binary`](#build-binary)
    - [`build-container`](#build-container)
    - [`kindenv`](#kindenv)
    - [`local-container-registry`](#local-container-registry)
    - [`oapi-codegen-helper`](#oapi-codegen-helper)
    - [`test-go`](#test-go)
  - [Examples](#examples)
    - [Containerfile](#containerfile)
      - [Go](#go)
    - [Makefile](#makefile)

## Available Tools

| Name                       | Description                                                                                                                                                                                                     |
|----------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `build-binary` | A wrapper around `go build` that simplifies building Go binaries. It uses environment variables for configuration. |
| `build-container` | A wrapper script around `kaniko` to build container images from a `Containerfile`. It is configured using environment variables. |
| `chart-prereq` | A helper tool to install necessary Helm charts in a Kubernetes cluster dedicated for tests. |
| `ci-orchestrator` | The `ci-orchestrator` is a tool responsible for orchestrating CI jobs. |
| `e2e` | A script to execute end-to-end tests for the `local-container-registry`. |
| `kindenv`                  | This tool wraps `kind` to create a Kubernetes cluster for local development and testing. It outputs the kubeconfig to a local path specified in the `.project.yaml` file.                                                                                        |
| `local-container-registry` | This tool creates a container registry within the kind cluster created by `kindenv`. It reads its configuration from the `.project.yaml` file.                                                                                     |
| `oapi-codegen-helper`      | A wrapper for `oapi-codegen` that simplifies the generation of server and client code from OpenAPI specifications. It reads its configuration from the `.project.yaml` file and parallelizes code generation. |
| `test-go` | A wrapper around `gotestsum` for executing scoped tests. It uses environment variables for configuration and supports test tags. |

## Project Configuration (`.project.yaml`)

The `.project.yaml` file is the central configuration file for all the tools in this repository. It allows you to declare the intent of your project and configure the behavior of the tools.

### Example `.project.yaml`

```yaml
name: my-project

kindenv:
  kubeconfigPath: .ignore.kindenv.kubeconfig.yaml

localContainerRegistry:
  enabled: true
  credentialPath: .ignore.local-container-registry.yaml
  caCrtPath: .ignore.ca.crt
  namespace: local-container-registry

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

## Usage

### `build-binary`

This tool builds a Go binary.

**Environment Variables:**

* `BINARY_NAME`: The name of the binary to build.
* `GO_BUILD_LDFLAGS`: The linker flags to pass to the `go build` command.

**Example:**

```sh
BINARY_NAME="my-app" GO_BUILD_LDFLAGS="-X main.Version=1.0.0" go run github.com/alexandremahdhaoui/tooling/cmd/build-binary
```

### `build-container`

This tool builds a container image using Kaniko.

**Environment Variables:**

* `CONTAINER_ENGINE`: The container engine to use (e.g., `docker`, `podman`).
* `CONTAINER_NAME`: The name of the container to build.
* `BUILD_ARGS`: A list of build arguments to pass to the container build command.
* `DESTINATIONS`: A list of destinations to push the container image to.

**Example:**

```sh
CONTAINER_ENGINE="docker" \
CONTAINER_NAME="my-app" \
BUILD_ARGS="VERSION=1.0.0" \
DESTINATIONS="docker.io/my-user/my-app:latest" \
go run github.com/alexandremahdhaoui/tooling/cmd/build-container
```

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

