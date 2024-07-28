#!/usr/bin/env bash

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

__setup() {
  go run ./cmd/local-container-registry
  kubectl port-forward -nlocal-container-registry svc/local-container-registry 5000:5000 &
  sleep 3
}

__teardown() {
  go run ./cmd/local-container-registry teardown
  PID="$(netstat -ntulp 2>/dev/null | grep -E 'tcp.*127.0.0.1:5000.*LISTEN.*kubectl' | awk '{print $7}' | sed 's@/kubectl@@')"
  kill -9 "${PID}"
}

# Verify required envs
trap __usage EXIT
echo "${CONTAINER_ENGINE} ${KUBECONFIG}" &>/dev/null

# Run the test
trap '__teardown && echo "❌ [FAILED] local-container-registry e2e test failed"' EXIT
__setup

LCR_ENDPOINT="local-container-registry.local-container-registry.svc.cluster.local:5000"
LCR_CONFIG=".ignore.local-container-registry.yaml"

ADDITIONAL_FLAGS=""

if [ "${CONTAINER_ENGINE}" == "podman" ]; then
  ADDITIONAL_FLAGS="--tls-verify=false"
fi

yq '.password' "${LCR_CONFIG}" \
    | ${CONTAINER_ENGINE} login \
        "${LCR_ENDPOINT}" \
        -u="$(yq '.username' "${LCR_CONFIG}")" \
        --password-stdin \
        ${ADDITIONAL_FLAGS}

NEW_IMAGE="${LCR_ENDPOINT}/registry"

${CONTAINER_ENGINE} tag registry:2 "${NEW_IMAGE}"
${CONTAINER_ENGINE} push "${NEW_IMAGE}" ${ADDITIONAL_FLAGS}
${CONTAINER_ENGINE} pull "${NEW_IMAGE}" ${ADDITIONAL_FLAGS}

trap '__teardown && echo "✅ [PASS] local-container-registry e2e test passed successfully"' EXIT
