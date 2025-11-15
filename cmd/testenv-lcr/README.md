# local-container-registry

A tool for creating a fully functional, TLS-enabled container registry inside a Kind (Kubernetes in Docker) cluster for local development.

## Overview

`local-container-registry` automates the setup of a production-like container registry in your local Kubernetes cluster with:

- **TLS encryption** via cert-manager with self-signed certificates
- **htpasswd authentication** with randomly generated credentials
- **Automatic DNS configuration** via /etc/hosts entry
- **Docker/Podman compatibility** with CA certificate export
- **Full Kubernetes integration** (Service, Deployment, ConfigMap, Secrets)

## Prerequisites

- Go 1.22.5 or later
- Kind cluster running (use `cmd/kindenv` to create one)
- `kubectl` configured with access to the cluster
- `helm` installed
- `docker` or `podman` installed
- Root/sudo access (for /etc/hosts and Docker cert directory)

## Quick Start

```bash
# Setup Kind cluster
KIND_BINARY=kind KIND_BINARY_PREFIX=sudo go run ./cmd/kindenv setup

# Setup registry
CONTAINER_ENGINE=docker PREPEND_CMD=sudo go run ./cmd/local-container-registry

# The registry is now available at:
# local-container-registry.local-container-registry.svc.cluster.local:5000
```

## Configuration

The tool reads configuration from `forge.yaml`:

```yaml
localContainerRegistry:
  enabled: true
  credentialPath: .ignore.local-container-registry.yaml
  caCrtPath: .ignore.ca.crt
  namespace: local-container-registry
  autoPushImages: true  # Automatically push images from artifact store on setup
  imagePullSecretNamespaces:  # Automatically create image pull secrets in these namespaces
    - default
    - my-app
  imagePullSecretName: local-container-registry-credentials  # Custom secret name (optional)

build:
  artifactStorePath: .ignore.artifact-store.yaml
  specs:
    - container:
        name: container-build
        file: ./containers/container-build/Containerfile
```

### Environment Variables

| Variable | Description | Required | Default |
|----------|-------------|----------|---------|
| `CONTAINER_ENGINE` | Container engine (docker/podman) | Yes | - |
| `PREPEND_CMD` | Command prefix for privileged ops (e.g., "sudo") | No | "" |
| `KUBECONFIG` | Path to kubeconfig file | No | Default kubectl config |

## Usage

### Setup

```bash
# Basic setup
CONTAINER_ENGINE=docker go run ./cmd/local-container-registry

# With sudo for privileged operations
CONTAINER_ENGINE=docker PREPEND_CMD=sudo go run ./cmd/local-container-registry
```

**What happens during setup:**
1. Creates `local-container-registry` namespace
2. Installs cert-manager via Helm
3. Generates random credentials (32 chars each)
4. Creates htpasswd hash for authentication
5. Generates TLS certificates (self-signed)
6. Exports CA certificate to `.ignore.ca.crt`
7. **Adds /etc/hosts entry** for registry FQDN
8. Deploys registry with TLS and auth
9. Writes credentials to `.ignore.local-container-registry.yaml`
10. Auto-pushes images from artifact store (if `autoPushImages: true`)
11. Creates image pull secrets in configured namespaces (if `imagePullSecretNamespaces` specified)

### Teardown

```bash
CONTAINER_ENGINE=docker PREPEND_CMD=sudo go run ./cmd/local-container-registry teardown
```

**What happens during teardown:**
1. Deletes image pull secrets in all namespaces
2. Deletes namespace (cascades to all resources)
3. Uninstalls cert-manager
4. **Removes /etc/hosts entry**
5. Cleans up local files

### Push Single Image

```bash
# Push a specific image to the registry
CONTAINER_ENGINE=docker go run ./cmd/local-container-registry push nginx:latest

# Push a locally built image
CONTAINER_ENGINE=docker go run ./cmd/local-container-registry push container-build:abc123
```

**What happens during push:**
1. Reads registry configuration
2. Logs in to registry
3. Tags image with registry FQDN
4. Pushes image to registry

### Push All Images

```bash
# Push all images from artifact store
CONTAINER_ENGINE=docker go run ./cmd/local-container-registry push-all
```

**What happens during push-all:**
1. Reads `forge.yaml` build configuration
2. Reads artifact store
3. Logs in to registry
4. For each container in config:
   - Finds latest artifact version
   - Tags and pushes image

**Note**: If `autoPushImages: true` in config, this happens automatically during setup.

### Create Image Pull Secret

```bash
# Create an image pull secret in a specific namespace
CONTAINER_ENGINE=docker go run ./cmd/local-container-registry create-image-pull-secret my-app

# Create with a custom secret name
CONTAINER_ENGINE=docker go run ./cmd/local-container-registry create-image-pull-secret my-app my-custom-secret
```

**What happens during create-image-pull-secret:**
1. Reads registry configuration and credentials
2. Creates namespace if it doesn't exist
3. Generates `.dockerconfigjson` with registry auth
4. Creates Kubernetes secret of type `kubernetes.io/dockerconfigjson`
5. Labels secret with `app.kubernetes.io/managed-by=testenv-lcr`

### List Image Pull Secrets

```bash
# List all image pull secrets created by testenv-lcr
CONTAINER_ENGINE=docker go run ./cmd/local-container-registry list-image-pull-secrets

# List secrets in a specific namespace
CONTAINER_ENGINE=docker go run ./cmd/local-container-registry list-image-pull-secrets my-app
```

**What happens during list-image-pull-secrets:**
1. Lists all secrets with label `app.kubernetes.io/managed-by=testenv-lcr`
2. Filters by namespace if provided
3. Displays namespace, secret name, and creation time

## Accessing the Registry

### Registry FQDN

```
local-container-registry.local-container-registry.svc.cluster.local:5000
```

### From Host Machine

```bash
# 1. Port-forward the registry
kubectl port-forward -n local-container-registry svc/local-container-registry 5000:5000 &

# 2. Login
yq '.password' .ignore.local-container-registry.yaml | \
  docker login local-container-registry.local-container-registry.svc.cluster.local:5000 \
    -u "$(yq '.username' .ignore.local-container-registry.yaml)" \
    --password-stdin

# 3. Push an image
docker tag nginx:latest \
  local-container-registry.local-container-registry.svc.cluster.local:5000/nginx:latest
docker push \
  local-container-registry.local-container-registry.svc.cluster.local:5000/nginx:latest

# 4. Pull the image
docker pull \
  local-container-registry.local-container-registry.svc.cluster.local:5000/nginx:latest
```

### From Within Cluster

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
spec:
  containers:
  - name: nginx
    image: local-container-registry.local-container-registry.svc.cluster.local:5000/nginx:latest
  imagePullSecrets:
  - name: local-container-registry-credentials
```

## Output Files

After setup, these files are created:

- **`.ignore.local-container-registry.yaml`** - Credentials
  ```yaml
  username: <random-32-chars>
  password: <random-32-chars>
  ```

- **`.ignore.ca.crt`** - CA certificate for TLS

## Troubleshooting

### Registry not accessible

```bash
# Check deployment
kubectl get deployment -n local-container-registry

# Check service
kubectl get service -n local-container-registry

# Verify /etc/hosts entry
grep local-container-registry /etc/hosts
```

### TLS errors

```bash
# Verify CA cert exists
ls -la .ignore.ca.crt

# Check Docker cert directory
ls -la /etc/docker/certs.d/local-container-registry.local-container-registry.svc.cluster.local:5000/

# For Podman, use --tls-verify=false
podman push --tls-verify=false <image>
```

### Manual cleanup

```bash
# Delete namespace
kubectl delete namespace local-container-registry

# Uninstall cert-manager
helm uninstall cert-manager -n cert-manager

# Remove /etc/hosts entry
sudo sed -i '/local-container-registry.local-container-registry.svc.cluster.local/d' /etc/hosts

# Remove local files
rm -f .ignore.local-container-registry.yaml .ignore.ca.crt
```

## Complete Example

### Basic Workflow

```bash
#!/bin/bash
set -e

# 1. Setup Kind cluster
echo "Setting up Kind cluster..."
KIND_BINARY=kind KIND_BINARY_PREFIX=sudo go run ./cmd/kindenv setup

# 2. Setup registry
echo "Setting up local container registry..."
CONTAINER_ENGINE=docker PREPEND_CMD=sudo go run ./cmd/local-container-registry

# 3. Port-forward
echo "Port-forwarding registry..."
kubectl port-forward -n local-container-registry svc/local-container-registry 5000:5000 &
sleep 5

# 4. Login
echo "Logging in to registry..."
yq '.password' .ignore.local-container-registry.yaml | \
  docker login local-container-registry.local-container-registry.svc.cluster.local:5000 \
    -u "$(yq '.username' .ignore.local-container-registry.yaml)" \
    --password-stdin

# 5. Pull, tag, and push
echo "Pushing test image..."
docker pull nginx:latest
docker tag nginx:latest \
  local-container-registry.local-container-registry.svc.cluster.local:5000/nginx:latest
docker push \
  local-container-registry.local-container-registry.svc.cluster.local:5000/nginx:latest

# 6. Pull from registry
echo "Pulling from registry..."
docker pull \
  local-container-registry.local-container-registry.svc.cluster.local:5000/nginx:latest

echo "✅ Success!"

# Cleanup
read -p "Clean up? (y/n) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
  kill %1  # Kill port-forward
  CONTAINER_ENGINE=docker PREPEND_CMD=sudo go run ./cmd/local-container-registry teardown
  go run ./cmd/kindenv teardown
fi
```

### Workflow with container-build

```bash
#!/bin/bash
set -e

# 1. Setup Kind cluster
echo "Setting up Kind cluster..."
KIND_BINARY=kind KIND_BINARY_PREFIX=sudo go run ./cmd/kindenv setup

# 2. Build containers from config
echo "Building containers..."
CONTAINER_ENGINE=docker go run ./cmd/container-build

# 3. Setup registry (auto-pushes if autoPushImages: true)
echo "Setting up local container registry..."
CONTAINER_ENGINE=docker PREPEND_CMD=sudo go run ./cmd/local-container-registry

# 4. Or manually push all images
echo "Pushing all images to registry..."
CONTAINER_ENGINE=docker go run ./cmd/local-container-registry push-all

# 5. Or push single image
echo "Pushing single image..."
CONTAINER_ENGINE=docker go run ./cmd/local-container-registry push container-build:latest

echo "✅ Success!"

# Cleanup
read -p "Clean up? (y/n) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
  CONTAINER_ENGINE=docker PREPEND_CMD=sudo go run ./cmd/local-container-registry teardown
  KIND_BINARY=kind KIND_BINARY_PREFIX=sudo go run ./cmd/kindenv teardown
fi
```

## Architecture

See [ARCHITECTURE.md](../../ARCHITECTURE.md#local-container-registry) for detailed architecture documentation.

## Future Enhancements

- [ ] Support for mirroring images declaratively
- [ ] Support for mirroring helm charts declaratively
- [ ] Persistent storage option
- [ ] Multi-cluster support

## Related Tools

- **kindenv** - Creates Kind clusters
- **e2e** - End-to-end tests
- **container-build** - Builds container images
