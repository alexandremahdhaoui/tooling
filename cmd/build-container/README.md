# build-container

A tool for building container images from declarative configuration and tracking them in an artifact store.

## Overview

`build-container` reads container build specifications from `.project.yaml`, builds all defined containers using Kaniko, and writes artifact metadata to an artifact store for version tracking.

## Features

- **Declarative configuration** - Define container builds in `.project.yaml`
- **Git-based versioning** - Uses git commit hash for artifact versions
- **Artifact tracking** - Maintains artifact store with timestamps and versions
- **Kaniko-based builds** - Rootless container builds
- **Automatic tagging** - Tags images with both version and `latest`

## Prerequisites

- Go 1.22.5 or later
- Docker or Podman
- Git repository (for version tracking)
- Kaniko executor image available

## Configuration

### .project.yaml

Define container build specifications:

```yaml
build:
  artifactStorePath: .ignore.artifact-store.yaml
  specs:
    - container:
        name: build-container
        file: ./containers/build-container/Containerfile
    - container:
        name: my-app
        file: ./containers/my-app/Containerfile
```

### Environment Variables

| Variable | Description | Required | Default |
|----------|-------------|----------|---------|
| `CONTAINER_ENGINE` | Container engine (docker/podman) | Yes | - |
| `BUILD_ARGS` | List of build arguments for kaniko | No | [] |

## Usage

### Basic Build

```bash
# Build all containers defined in .project.yaml
CONTAINER_ENGINE=docker go run ./cmd/build-container
```

### With Build Arguments

```bash
# Pass build arguments to kaniko
CONTAINER_ENGINE=docker \
  BUILD_ARGS="GO_BUILD_LDFLAGS=-X main.Version=1.0.0" \
  go run ./cmd/build-container
```

## How It Works

1. **Read Configuration** - Loads `.project.yaml` to get container specs
2. **Get Git Version** - Runs `git rev-parse HEAD` to get current commit hash
3. **Build Each Container**:
   - Runs Kaniko to build container from specified Containerfile
   - Exports build to tar file
   - Loads tar into container engine
   - Tags with `{name}:{version}` and `{name}:latest`
   - Cleans up tar file
4. **Update Artifact Store** - Writes artifact metadata to store file

## Artifact Store

The artifact store (`. ignore.artifact-store.yaml`) contains metadata for all built artifacts:

```yaml
artifacts:
  - name: build-container
    type: container
    location: build-container:2be2494abc123...
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
# 1. Define containers in .project.yaml
cat >> .project.yaml <<EOF
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

# 3. Build
CONTAINER_ENGINE=docker go run ./cmd/build-container

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
CONTAINER_ENGINE=docker go run ./cmd/build-container

# Setup registry (auto-pushes if autoPushImages: true)
CONTAINER_ENGINE=docker PREPEND_CMD=sudo go run ./cmd/local-container-registry

# Or manually push all
CONTAINER_ENGINE=docker go run ./cmd/local-container-registry push-all

# Or push single image
CONTAINER_ENGINE=docker go run ./cmd/local-container-registry push my-app:latest
```

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

See [ARCHITECTURE.md](../../ARCHITECTURE.md#build-container) for detailed architecture documentation.

## Related Tools

- **local-container-registry** - Push built images to local registry
- **kindenv** - Local Kubernetes environment
- **e2e** - End-to-end testing
