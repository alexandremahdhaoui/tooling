#!/usr/bin/env bash

set -o errexit
set -o nounset

__usage() {
  cat <<EOF
USAGE:

${0} [BINARY_NAME]

Required environment variables:
    CONTAINER_ENGINE    Container engine such as podman or docker.
    GO_BUILD_LDFLAGS    Go linker flags.
    VERSION             Semver tag.
EOF
  exit 1
}

trap __usage EXIT

BINARY_NAME="${1}"

"${CONTAINER_ENGINE}" \
  build \
  . \
  --build-arg "GO_BUILD_LDFLAGS=${GO_BUILD_LDFLAGS}" \
  -t "${BINARY_NAME}:${VERSION}" \
  -f "./containers/${BINARY_NAME}/Containerfile"

trap 'echo "✅ Container image \"${BINARY_NAME}\" built successfully"' EXIT
