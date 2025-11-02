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
GOLANGCI_LINT_VERSION  := v1.59.1
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

CONTROLLER_GEN := go run sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_GEN_VERSION)
GO_GEN         := go generate
GOFUMPT        := go run mvdan.cc/gofumpt@$(GOFUMPT_VERSION)
GOLANGCI_LINT  := go run github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
GOTESTSUM      := go run gotest.tools/gotestsum@$(GOTESTSUM_VERSION) --format pkgname
MOCKERY        := go run github.com/vektra/mockery/v2@$(MOCKERY_VERSION)
OAPI_CODEGEN   := go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@$(OAPI_CODEGEN_VERSION)

BUILD_BINARY                := GO_BUILD_LDFLAGS="$(GO_BUILD_LDFLAGS)" go run ./cmd/build-binary
BUILD_CONTAINER             := CONTAINER_ENGINE="$(CONTAINER_ENGINE)" BUILD_ARGS="GO_BUILD_LDFLAGS=$(GO_BUILD_LDFLAGS)" go run ./cmd/build-container
KINDENV                     := $(KINDENV_ENVS) go run ./cmd/kindenv
LOCAL_CONTAINER_REGISTRY    := CONTAINER_ENGINE="$(CONTAINER_ENGINE)" PREPEND_CMD=sudo go run ./cmd/local-container-registry
OAPI_CODEGEN_HELPER         := OAPI_CODEGEN="$(OAPI_CODEGEN)" go run ./cmd/oapi-codegen-helper
TEST_GO                     := GOTESTSUM="$(GOTESTSUM)" go run ./cmd/test-go

CLEAN_MOCKS := rm -rf ./internal/util/mocks

# ------------------------------------------------------- GENERATE --------------------------------------------------- #


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
	echo "DISCLAIMER: this is still a work in progress"
	CONTAINER_ENGINE=$(CONTAINER_ENGINE) ./cmd/e2e/main.sh

.PHONY: test-setup
test-setup: build-container
	$(KINDENV) setup
	$(LOCAL_CONTAINER_REGISTRY)

.PHONY: test-teardown
test-teardown:
	$(LOCAL_CONTAINER_REGISTRY) teardown
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
