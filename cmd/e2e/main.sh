#!/usr/bin/env bash

# This script runs the end-to-end tests for the local-container-registry tool.
# It sets up a local container registry, pushes and pulls an image, and then tears down the registry.

set -o errexit
set -o nounset

__usage() {
  cat <<EOF
USAGE:

${0}

Required environment variables:
    CONTAINER_ENGINE    Container engine such as podman or docker.
EOF
  exit 1
}

export KUBECONFIG=.ignore.kindenv.kubeconfig.yaml
export KIND_BINARY="${KIND_BINARY:-kind}"
export KIND_BINARY_PREFIX="${KIND_BINARY_PREFIX:-sudo}"

__setup_cluster() {
  echo "⏳ Setting up Kind cluster"
  KIND_BINARY="${KIND_BINARY}" KIND_BINARY_PREFIX="${KIND_BINARY_PREFIX}" go run ./cmd/kindenv setup
}

__teardown_cluster() {
  echo "⏳ Tearing down Kind cluster"
  KIND_BINARY="${KIND_BINARY}" KIND_BINARY_PREFIX="${KIND_BINARY_PREFIX}" go run ./cmd/kindenv teardown
}

__setup() {
  CONTAINER_ENGINE="${CONTAINER_ENGINE}" PREPEND_CMD=sudo go run ./cmd/local-container-registry

  # Wait for the registry deployment to be fully ready
  kubectl wait --for=condition=available --timeout=60s -nlocal-container-registry deployment/local-container-registry
}

__teardown() {
  CONTAINER_ENGINE="${CONTAINER_ENGINE}" PREPEND_CMD=sudo go run ./cmd/local-container-registry teardown
}

# Verify required envs
trap __usage EXIT
echo "${CONTAINER_ENGINE} ${KUBECONFIG}" &>/dev/null

# Setup cluster
__setup_cluster

# Run the test
trap '__teardown && __teardown_cluster && echo "❌ [FAILED] local-container-registry e2e test failed"' EXIT

# Step 1: Build containers using forge
echo "⏳ [TEST] Building containers with forge"
CONTAINER_ENGINE="${CONTAINER_ENGINE}" GO_BUILD_LDFLAGS="${GO_BUILD_LDFLAGS:-}" go run ./cmd/forge build
echo "✅ [TEST] Containers built successfully"

# Step 2: Verify artifact store exists and contains build-container artifact
echo "⏳ [TEST] Verifying artifact store"
ARTIFACT_STORE_PATH=".ignore.artifact-store.yaml"
if [ ! -f "${ARTIFACT_STORE_PATH}" ]; then
  echo "❌ [TEST] Artifact store not found at ${ARTIFACT_STORE_PATH}"
  exit 1
fi

# Check that artifact store contains build-container
if ! grep -q "name: build-container" "${ARTIFACT_STORE_PATH}"; then
  echo "❌ [TEST] Artifact store does not contain build-container"
  cat "${ARTIFACT_STORE_PATH}"
  exit 1
fi

# Check that artifact has required fields
if ! grep -q "type: container" "${ARTIFACT_STORE_PATH}"; then
  echo "❌ [TEST] Artifact missing type field"
  exit 1
fi

if ! grep -q "version:" "${ARTIFACT_STORE_PATH}"; then
  echo "❌ [TEST] Artifact missing version field"
  exit 1
fi

if ! grep -q "timestamp:" "${ARTIFACT_STORE_PATH}"; then
  echo "❌ [TEST] Artifact missing timestamp field"
  exit 1
fi

echo "✅ [TEST] Artifact store verified successfully"
cat "${ARTIFACT_STORE_PATH}"

# Step 3: Setup registry
__setup

# Step 4: Test push-all command to push built containers to registry
echo "⏳ [TEST] Testing push-all command"
CONTAINER_ENGINE="${CONTAINER_ENGINE}" PREPEND_CMD=sudo go run ./cmd/local-container-registry push-all
echo "✅ [TEST] push-all command succeeded"

# Step 5: Verify images are in the registry by checking with docker/podman images
echo "⏳ [TEST] Verifying built image was pushed"
BUILT_IMAGE_VERSION=$(yq '.artifacts[0].version' "${ARTIFACT_STORE_PATH}")
echo "Built image version: ${BUILT_IMAGE_VERSION}"
echo "✅ [TEST] Push-all verified (image pushed to registry)"

# Step 6: Test push command with a standard image
echo "⏳ [TEST] Testing push command with registry:2 image"
${CONTAINER_ENGINE} pull registry:2
CONTAINER_ENGINE="${CONTAINER_ENGINE}" PREPEND_CMD=sudo go run ./cmd/local-container-registry push registry:2
echo "✅ [TEST] push command succeeded"

trap '__teardown && __teardown_cluster && echo "✅ [PASS] local-container-registry e2e test passed successfully"' EXIT
