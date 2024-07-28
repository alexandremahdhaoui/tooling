# local-container-registry

Let pods in kindenv access container images (and/or helm charts).

## Left to be done

- Support for mirror images declaratively.
- Support for mirroring helm charts declaratively.

## Test the registry

### Pre-requisites

#### Open a port forward

```bash
kubectl port-forward -nlocal-container-registry svc/local-container-registry 5000:5000
```

#### Optional: Open the logs

```bash
kubectl logs -f -nlocal-container-registry deploy/local-container-registry
```

### Login to the registry, push and pull image

```bash
LCR_ENDPOINT="local-container-registry.local-container-registry.svc.cluster.local:5000"
LCR_CONFIG=".ignore.local-container-registry.yaml"

read -rp "container engine? (docker, podman) " CONTAINER_ENGINE

curl -k -u"$(yq '"\(.username):\(.password)"' "${LCR_CONFIG}")" \
    "https://${LCR_ENDPOINT}/v2/"

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
```

### Simply curl the registry

```bash
LCR_ENDPOINT="local-container-registry.local-container-registry.svc.cluster.local:5000"
LCR_CONFIG=".ignore.local-container-registry.yaml"

curl -k -u"$(yq '"\(.username):\(.password)"' "${LCR_CONFIG}")" \
    "https://${LCR_ENDPOINT}/v2/"
```
