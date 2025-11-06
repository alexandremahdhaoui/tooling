# build-container MCP Server

MCP server for building container images using Kaniko.

## Purpose

Provides MCP tools for building container images with rootless Kaniko builder, automatic git versioning, and artifact tracking.

## Invocation

```bash
build-container --mcp
```

Forge invokes this automatically via:
```yaml
builder: go://build-container
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
      builder: go://build-container
```

Run with:
```bash
forge build
```

## Implementation Details

- Uses Kaniko for rootless container builds
- Automatically tags with git commit SHA
- Tags both `<name>:<version>` and `<name>:latest`
- Stores artifacts in artifact store
- Exports to tar, loads into container engine
- Supports Docker and Podman

## Environment Variables

- `CONTAINER_ENGINE` - docker or podman (required)
- `BUILD_ARGS` - Additional build arguments (optional)
- `KANIKO_CACHE_DIR` - Cache directory (default: ~/.kaniko-cache)

## See Also

- [build-go MCP Server](../build-go/MCP.md)
- [Forge Build Documentation](../../docs/forge-usage.md#building-artifacts)
