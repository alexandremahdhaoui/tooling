# container-build MCP Server

MCP server for building container images with multiple backend engines.

## Purpose

Provides MCP tools for building container images with support for docker, kaniko (rootless), and podman. Features automatic git versioning and artifact tracking.

## Invocation

```bash
container-build --mcp
```

Forge invokes this automatically via:
```yaml
builder: go://container-build
```

## Available Tools

### `build`

Build a single container image.

**Input Schema:**
```json
{
  "name": "string (required)",        // Container name
  "src": "string (required)",         // Path to Containerfile
  "dest": "string (optional)",        // Destination (default: local)
  "engine": "string (optional)"       // Builder engine reference
}
```

**Output:**
```json
{
  "name": "string",
  "type": "container",
  "location": "string",              // e.g., "my-image:abc123def"
  "timestamp": "string",             // RFC3339 format
  "version": "string"                // Git commit SHA
}
```

**Example:**
```json
{
  "method": "tools/call",
  "params": {
    "name": "build",
    "arguments": {
      "name": "my-app",
      "src": "./Containerfile"
    }
  }
}
```

### `buildBatch`

Build multiple container images in sequence.

**Input Schema:**
```json
{
  "specs": [
    {
      "name": "string",
      "src": "string",
      "dest": "string",
      "engine": "string"
    }
  ]
}
```

**Output:**
Array of Artifacts with summary of successes/failures.

## Integration with Forge

In `forge.yaml`:
```yaml
build:
  specs:
    - name: my-app-image
      src: ./Containerfile
      dest: localhost:5000
      builder: go://container-build
```

Run with:
```bash
forge build
```

## Implementation Details

- Supports three build modes: docker, kaniko, and podman
- Automatically tags with git commit SHA
- Tags both `<name>:<version>` and `<name>:latest`
- Stores artifacts in artifact store
- Kaniko mode: exports to tar, loads into container engine (requires docker to run Kaniko executor)
- Docker/Podman modes: native builds (faster, direct integration)

## Build Modes

### docker
Native Docker builds using `docker build`. Fast and requires Docker daemon.

### kaniko
Rootless builds using Kaniko executor (runs in container via docker). Secure, supports layer caching.

### podman
Native Podman builds using `podman build`. Rootless and requires Podman.

## Environment Variables

- `CONTAINER_BUILD_ENGINE` - Build mode: docker, kaniko, or podman (required)
- `BUILD_ARGS` - Additional build arguments (optional)
- `KANIKO_CACHE_DIR` - Cache directory for kaniko mode (default: ~/.kaniko-cache)

## Mode Comparison

| Feature | docker | kaniko | podman |
|---------|--------|--------|--------|
| Requires Daemon | Yes (Docker) | Yes (Docker to run Kaniko) | Yes (Podman) |
| Rootless | No | Yes | Yes |
| Build Speed | Fast | Moderate | Fast |
| Layer Caching | Native | Via cache dir | Native |

## See Also

- [go-build MCP Server](../go-build/MCP.md)
- [Forge Build Documentation](../../docs/forge-usage.md#building-artifacts)
