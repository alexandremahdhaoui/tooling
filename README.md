# tooling

This repository contains tooling and utilities to simplify development.

## Available tools

| Name                       | Description                                                                                                                                                                                                     |
|----------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `build-binary` | Wrapper script around `go build` to build go binaries. |
| `build-container` | Wrapper script arounce `kaniko` to build container images. |
| `chart-prereq` | Helper to install necessary helm charts in k8s cluster dedicated for tests. |
| `ci-orchestrator` | The `ci-orchestrator` is a tool responsible for orchestrating CI jobs. |
| `e2e` | Script to execute e2e tests. |
| `kindenv`                  | It wraps `kind` to create a k8s cluster and output the kubeconfig to a local path specified by the `.project.yaml` file.                                                                                        |
| `local-container-registry` | It creates a container registry in the kind cluster created by `kindenv`. It reads it's configuration from `.project.yaml`.                                                                                     | 
| `oapi-codegen-helper`      | It wraps `oapi-codegen` to conveniently generate server and/or client code from a local or remote OpenAPI Specification. It reads its configuration from `.oapi-codegen.yaml`. Code generation is parallelized. | 
| `test-go` | Wrapper script around `gotestsum` to execute scoped tests. |

## Project Config

The project config or `.project.yaml` file is a single configuration file that declares intent about the project and is
used by the tools and utilities defined in this project.

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

.PHONY: generate
generate: ## Generate REST API server/client code, CRDs and other go generators.
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

