# ------------------------------------------------------- ENVS ------------------------------------------------------- #

PROJECT    := ipxer

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
GOLANGCI_LINT_VERSION  := v2.6.0
# renovate: datasource=github-release depName=gotestyourself/gotestsum
GOTESTSUM_VERSION      := v1.12.0
# renovate: datasource=github-release depName=vektra/mockery
MOCKERY_VERSION        := v2.42.0
# renovate: datasource=github-release depName=oapi-codegen/oapi-codegen
OAPI_CODEGEN_VERSION   := v2.3.0

# ------------------------------------------------------- TOOLS ------------------------------------------------------ #

CONTAINER_ENGINE   ?= docker
KIND_BINARY        ?= kind
KIND_BINARY_PREFIX ?= sudo

KINDENV_ENVS := KIND_BINARY_PREFIX="$(KIND_BINARY_PREFIX)" KIND_BINARY="$(KIND_BINARY)"

# Forge - the orchestration tool that runs everything
FORGE := GO_BUILD_LDFLAGS="$(GO_BUILD_LDFLAGS)" $(KINDENV_ENVS) CONTAINER_ENGINE="$(CONTAINER_ENGINE)" go run ./cmd/forge

# Individual tools (for direct invocation if needed)
FORMAT_GO            := GOFUMPT_VERSION="$(GOFUMPT_VERSION)" ./build/bin/format-go
GENERATE_MOCKS       := MOCKERY_VERSION="$(MOCKERY_VERSION)" ./build/bin/generate-mocks
GENERATE_OPENAPI_GO  := OAPI_CODEGEN_VERSION="$(OAPI_CODEGEN_VERSION)" ./build/bin/generate-openapi-go

# ------------------------------------------------------- HELP ------------------------------------------------------- #

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# ------------------------------------------------------- BUILD ------------------------------------------------------ #

.PHONY: build
build: ## Build all artifacts using forge
	$(FORGE) build

.PHONY: build-go
build-go: ## Build Go binaries using forge
	$(FORGE) build

.PHONY: build-container
build-container: ## Build container images using forge
	$(FORGE) build for-testing-purposes

# ------------------------------------------------------- GENERATE --------------------------------------------------- #

.PHONY: generate
generate: generate-oapi generate-mocks ## Generate all code (oapi, mocks)

.PHONY: generate-oapi
generate-oapi: build-tools ## Generate OpenAPI client/server code
	$(GENERATE_OPENAPI_GO)

.PHONY: generate-mocks
generate-mocks: build-tools ## Generate mocks
	$(GENERATE_MOCKS)

# ------------------------------------------------------- FORMAT ----------------------------------------------------- #

.PHONY: fmt
fmt: build-tools ## Format Go code using gofumpt
	$(FORMAT_GO)

# ------------------------------------------------------- LINT ------------------------------------------------------- #

.PHONY: lint
lint: ## Lint Go code using golangci-lint
	$(FORGE) test lint run

# ------------------------------------------------------- TEST ------------------------------------------------------- #

.PHONY: test-unit
test-unit: ## Run unit tests
	$(FORGE) test unit run

.PHONY: test-integration
test-integration: ## Run integration tests
	$(FORGE) test integration run

.PHONY: test-e2e
test-e2e: ## Run end-to-end tests
	@echo "DISCLAIMER: this is still a work in progress"
	CONTAINER_ENGINE=$(CONTAINER_ENGINE) ./cmd/e2e/main.sh

.PHONY: test-chart
test-chart: ## Run chart tests
	@echo "TODO: implement 'make test-chart'."

# ------------------------------------------------------- TEST SETUP/TEARDOWN ---------------------------------------- #

.PHONY: test-setup
test-setup: build-container ## Setup test environment (kindenv + local registry)
	$(KINDENV_ENVS) go run ./cmd/kindenv setup
	CONTAINER_ENGINE="$(CONTAINER_ENGINE)" PREPEND_CMD=sudo go run ./cmd/local-container-registry

.PHONY: test-teardown
test-teardown: ## Teardown test environment
	CONTAINER_ENGINE="$(CONTAINER_ENGINE)" PREPEND_CMD=sudo go run ./cmd/local-container-registry teardown
	$(KINDENV_ENVS) go run ./cmd/kindenv teardown

# ------------------------------------------------------- UTILITIES -------------------------------------------------- #

.PHONY: build-tools
build-tools: ## Build format-go, lint-go, generate-mocks, and generate-openapi-go tools
	@$(FORGE) build format-go lint-go generate-mocks generate-openapi-go > /dev/null 2>&1 || true

.PHONY: clean
clean: ## Clean build artifacts
	rm -rf ./build/bin/*
	rm -f .ignore.*

.PHONY: version
version: ## Show forge version
	$(FORGE) version
