# go-build MCP Server

MCP server for building Go binaries with automatic git versioning.

## Purpose

Provides MCP tools for building Go binaries with consistent build flags, version injection, and artifact tracking.

## Invocation

```bash
go-build --mcp
```

Forge invokes this automatically via:
```yaml
builder: go://go-build
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
  "engine": "string (optional)",      // Builder engine reference
  "args": ["string"],                 // Additional go build arguments (e.g., ["-tags=netgo"])
  "env": {"key": "value"}             // Environment variables (e.g., {"GOOS": "linux", "GOARCH": "amd64"})
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

### Basic Usage

In `forge.yaml`:
```yaml
build:
  - name: myapp
    src: ./cmd/myapp
    dest: ./build/bin
    engine: go://go-build
```

Run with:
```bash
forge build
```

### With Custom Configuration

**Example 1: Static Binary with Build Tags**
```yaml
build:
  - name: static-binary
    src: ./cmd/myapp
    dest: ./build/bin
    engine: go://go-build
    spec:
      args:
        - "-tags=netgo"
        - "-ldflags=-w -s"
      env:
        CGO_ENABLED: "0"
```

**Example 2: Cross-Compilation**
```yaml
build:
  - name: myapp-linux-amd64
    src: ./cmd/myapp
    dest: ./build/bin
    engine: go://go-build
    spec:
      env:
        GOOS: "linux"
        GOARCH: "amd64"
        CGO_ENABLED: "0"

  - name: myapp-darwin-arm64
    src: ./cmd/myapp
    dest: ./build/bin
    engine: go://go-build
    spec:
      env:
        GOOS: "darwin"
        GOARCH: "arm64"
        CGO_ENABLED: "0"
```

**Example 3: Custom Linker Flags**
```yaml
build:
  - name: myapp-optimized
    src: ./cmd/myapp
    dest: ./build/bin
    engine: go://go-build
    spec:
      args:
        - "-ldflags=-w -s -X main.buildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
      env:
        CGO_ENABLED: "0"
```

## Implementation Details

- Runs `go build` with optimized flags
- Injects version via ldflags (git commit SHA)
- Outputs binary to `{dest}/{name}`
- Stores artifact metadata in artifact store
- Uses current git HEAD for versioning
- Supports custom build arguments via `args` field
- Supports custom environment variables via `env` field
- Sets `CGO_ENABLED=0` by default (can be overridden)

## Build Flags

**Default build command:**
```bash
CGO_ENABLED=0 go build -o {dest}/{name} {src}
```

**With custom args and env:**
```bash
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -tags=netgo -ldflags="-w -s" -o {dest}/{name} {src}
```

**Notes:**
- Custom `args` are inserted before the source path
- Custom `env` variables override defaults
- `CGO_ENABLED=0` is set by default but can be overridden via `env`

## See Also

- [container-build MCP Server](../container-build/MCP.md)
- [Forge Build Documentation](../../docs/forge-usage.md#building-artifacts)
