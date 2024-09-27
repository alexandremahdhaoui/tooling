#!/usr/bin/env bash

set -o errexit
set -o nounset

__usage() {
  cat <<EOF
USAGE:

GO_BUILD_LDFLAGS="" ${0} [BINARY_NAME]

Required environment variables:
    GO_BUILD_LDFLAGS    Go linker flags.
EOF
  exit 1
}

trap __usage EXIT

BINARY_NAME="${1}"

export CGO_ENABLED=0

go build \
  -ldflags "${GO_BUILD_LDFLAGS}" \
  -o "build/bin/${BINARY_NAME}" \
  "./cmd/${BINARY_NAME}"

trap 'echo "✅ Binary \"${BINARY_NAME}\" built successfully"' EXIT
