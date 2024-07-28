# local-container-registry

## What are we trying to achieve?

- Let pods in kubernetes test environment access container images (and/or helm charts).

## What has to be done?

- Create a local registry in kubernetes.
- Add TLS cert support.
- Add Basic auth support.
- Think about helm chart support.

## Curl manually after set up

### Pre-requisites

#### Open a port forward

```bash
kubectl port-forward -nlocal-container-registry svc/local-container-registry 5000:5000
```

#### Optional: Open the logs

```bash
kubectl logs -f -nlocal-container-registry deploy/local-container-registry
```

### Login to the registry and push image

```bash
LCR_ENDPOINT="local-container-registry.local-container-registry.svc.cluster.local:5000"
LCR_CONFIG=".ignore.local-container-registry.yaml"

read -rp "container engine? (docker, podman) " CONTAINER_ENGINE

curl -k -u"$(yq '"\(.username):\(.password)"' "${LCR_CONFIG}")" \
    "https://${LCR_ENDPOINT}/v2/"

yq '.password' "${LCR_CONFIG}" \
    | ${CONTAINER_ENGINE} login \
        "${LCR_ENDPOINT}" \
        -u="$(yq '.username' "${LCR_CONFIG}")" \
        --password-stdin \
        --tls-verify=false

NEW_IMAGE="${LCR_ENDPOINT}/registry"

${CONTAINER_ENGINE} tag registry:2 "${NEW_IMAGE}"
${CONTAINER_ENGINE} push "${NEW_IMAGE}" --tls-verify=false
${CONTAINER_ENGINE} pull "${NEW_IMAGE}" --tls-verify=false
```
