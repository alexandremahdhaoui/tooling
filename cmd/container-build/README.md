# container-build

A tool for building container images from declarative configuration with support for multiple build engines and artifact tracking.

## Overview

`container-build` reads container build specifications from `forge.yaml`, builds all defined containers using docker, kaniko, or podman, and writes artifact metadata to an artifact store for version tracking.

## Features

- **Multi-mode support** - Choose between docker, kaniko, or podman build engines
- **Declarative configuration** - Define container builds in `forge.yaml`
- **Git-based versioning** - Uses git commit hash for artifact versions
- **Artifact tracking** - Maintains artifact store with timestamps and versions
- **Rootless builds** - Support for rootless builds via kaniko or podman
- **Automatic tagging** - Tags images with both version and `latest`

## Prerequisites

- Go 1.22.5 or later
- One of: Docker, Podman, or Docker (for kaniko mode)
- Git repository (for version tracking)
- For kaniko mode: Kaniko executor image available

## Build Modes

### docker
Native Docker builds. Fast, requires Docker daemon.

```bash
CONTAINER_BUILD_ENGINE=docker go run ./cmd/container-build
```

### kaniko
Rootless builds using Kaniko executor (runs in container via docker). Secure, supports layer caching.

```bash
CONTAINER_BUILD_ENGINE=kaniko go run ./cmd/container-build
```

### podman
Native Podman builds. Rootless, requires Podman.

```bash
CONTAINER_BUILD_ENGINE=podman go run ./cmd/container-build
```

## Configuration

### forge.yaml

Define container build specifications:

```yaml
build:
  artifactStorePath: .ignore.artifact-store.yaml
  specs:
    - container:
        name: container-build
        file: ./containers/container-build/Containerfile
    - container:
        name: my-app
        file: ./containers/my-app/Containerfile
```

### Environment Variables

| Variable | Description | Required | Default |
|----------|-------------|----------|---------|
| `CONTAINER_BUILD_ENGINE` | Build engine (docker/kaniko/podman) | Yes | - |
| `BUILD_ARGS` | List of build arguments | No | [] |
| `KANIKO_CACHE_DIR` | Cache directory for kaniko mode | No | ~/.kaniko-cache |

## Usage

### Basic Build

```bash
# Build all containers defined in forge.yaml using docker
CONTAINER_BUILD_ENGINE=docker go run ./cmd/container-build
```

### With Build Arguments

```bash
# Pass build arguments to the build engine
CONTAINER_BUILD_ENGINE=docker \
  BUILD_ARGS="GO_BUILD_LDFLAGS=-X main.Version=1.0.0" \
  go run ./cmd/container-build
```

### Kaniko Mode with Custom Cache

```bash
# Use kaniko with custom cache directory
CONTAINER_BUILD_ENGINE=kaniko \
  KANIKO_CACHE_DIR=/custom/cache \
  go run ./cmd/container-build
```

## How It Works

1. **Read Configuration** - Loads `forge.yaml` to get container specs
2. **Get Git Version** - Runs `git rev-parse HEAD` to get current commit hash
3. **Build Each Container** (mode-specific):
   - **docker mode**: Runs `docker build` with tags
   - **kaniko mode**:
     - Runs Kaniko executor in container via docker
     - Exports build to tar file
     - Loads tar into docker
     - Tags with `{name}:{version}` and `{name}:latest`
     - Cleans up tar file
   - **podman mode**: Runs `podman build` with tags
4. **Update Artifact Store** - Writes artifact metadata to store file

## Artifact Store

The artifact store (`. ignore.artifact-store.yaml`) contains metadata for all built artifacts:

```yaml
artifacts:
  - name: container-build
    type: container
    location: container-build:2be2494abc123...
    timestamp: "2025-11-02T21:00:00Z"
    version: 2be2494abc123...
  - name: my-app
    type: container
    location: my-app:2be2494abc123...
    timestamp: "2025-11-02T21:00:05Z"
    version: 2be2494abc123...
```

### Artifact Fields

- `name` - Container name from config
- `type` - Always "container" for this tool
- `location` - Local image reference
- `timestamp` - Build time in RFC3339 format
- `version` - Git commit hash
- Each build creates/updates artifact with current version

## Example Workflow

```bash
# 1. Define containers in forge.yaml
cat >> forge.yaml <<EOF
build:
  artifactStorePath: .ignore.artifact-store.yaml
  specs:
    - container:
        name: my-app
        file: ./containers/my-app/Containerfile
EOF

# 2. Create Containerfile
mkdir -p containers/my-app
cat > containers/my-app/Containerfile <<EOF
FROM alpine:3.20
CMD ["echo", "Hello from my-app"]
EOF

# 3. Build (choose your mode)
CONTAINER_BUILD_ENGINE=docker go run ./cmd/container-build
# or
CONTAINER_BUILD_ENGINE=kaniko go run ./cmd/container-build
# or
CONTAINER_BUILD_ENGINE=podman go run ./cmd/container-build

# 4. Verify images
docker images | grep my-app
# my-app    2be2494abc...    ...
# my-app    latest           ...

# 5. Check artifact store
cat .ignore.artifact-store.yaml
```

## Integration with local-container-registry

Built images can be pushed to the local container registry:

```bash
# Build containers
CONTAINER_BUILD_ENGINE=docker go run ./cmd/container-build

# Setup registry (auto-pushes if autoPushImages: true)
CONTAINER_ENGINE=docker PREPEND_CMD=sudo go run ./cmd/local-container-registry

# Or manually push all
CONTAINER_ENGINE=docker go run ./cmd/local-container-registry push-all

# Or push single image
CONTAINER_ENGINE=docker go run ./cmd/local-container-registry push my-app:latest
```

**Note**: The local-container-registry tool uses `CONTAINER_ENGINE` (not `CONTAINER_BUILD_ENGINE`) to specify which container runtime to use for pushing images.

## Troubleshooting

### Git version error

```
Error: getting git version: exit status 128
```

**Solution**: Ensure you're in a git repository with commits:
```bash
git init
git add .
git commit -m "Initial commit"
```

### Kaniko pull error

```
Error: failed to pull image gcr.io/kaniko-project/executor:latest
```

**Solution**: Pull the Kaniko image first:
```bash
docker pull gcr.io/kaniko-project/executor:latest
```

### Container build fails

```
Error: building container
```

**Solution**: Check:
- Containerfile path is correct
- Containerfile syntax is valid
- All referenced files exist in context

### Artifact store not found

The tool creates the artifact store automatically if it doesn't exist. If you see errors reading it, ensure:
- Parent directory exists
- You have write permissions
- File is valid YAML (if it exists)

## Architecture

See [ARCHITECTURE.md](../../ARCHITECTURE.md#container-build) for detailed architecture documentation.

## Related Tools

- **local-container-registry** - Push built images to local registry
- **kindenv** - Local Kubernetes environment
- **e2e** - End-to-end testing
