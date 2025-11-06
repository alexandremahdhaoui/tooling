# build-go MCP Server

MCP server for building Go binaries with automatic git versioning.

## Purpose

Provides MCP tools for building Go binaries with consistent build flags, version injection, and artifact tracking.

## Invocation

```bash
build-go --mcp
```

Forge invokes this automatically via:
```yaml
builder: go://build-go
```

## Available Tools

### `build`

Build a single Go binary.

**Input Schema:**
```json
{
  "name": "string (required)",        // Binary name
  "src": "string (required)",         // Source directory (e.g., "./cmd/myapp")
  "dest": "string (optional)",        // Output directory (default: "./build/bin")
  "engine": "string (optional)"       // Builder engine reference
}
```

**Output:**
```json
{
  "name": "string",
  "type": "binary",
  "location": "string",              // e.g., "./build/bin/myapp"
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
      "name": "myapp",
      "src": "./cmd/myapp",
      "dest": "./build/bin"
    }
  }
}
```

### `buildBatch`

Build multiple Go binaries in sequence.

**Input Schema:**
```json
{
  "specs": [
    {
      "name": "string",
      "src": "string",
      "dest": "string"
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
    - name: myapp
      src: ./cmd/myapp
      dest: ./build/bin
      builder: go://build-go
```

Run with:
```bash
forge build
```

## Implementation Details

- Runs `go build` with optimized flags
- Injects version via ldflags (git commit SHA)
- Outputs binary to `{dest}/{name}`
- Stores artifact metadata in artifact store
- Uses current git HEAD for versioning

## Build Flags

Standard flags used:
```bash
go build -ldflags="-X main.version={gitsha}" -o {dest}/{name} {src}
```

## See Also

- [build-container MCP Server](../build-container/MCP.md)
- [Forge Build Documentation](../../docs/forge-usage.md#building-artifacts)
