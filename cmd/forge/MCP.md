# forge MCP Server

MCP server for build orchestration and test environment management.

## Purpose

The forge CLI itself runs as an MCP server, providing AI agents direct access to build orchestration capabilities. When invoked with `--mcp`, forge exposes tools to build artifacts from forge.yaml configuration.

## Invocation

```bash
forge --mcp
```

Or configure in your AI agent's MCP settings:
```json
{
  "mcpServers": {
    "forge": {
      "command": "forge",
      "args": ["--mcp"]
    }
  }
}
```

## Available Tools

### `build`

Build artifacts defined in forge.yaml configuration. Can build all artifacts or a specific artifact by name.

**Input Schema:**
```json
{
  "name": "string (optional)",           // Specific artifact name to build
  "artifactName": "string (optional)"    // Alternative to "name"
}
```

**Behavior:**
- If `name` or `artifactName` is provided: builds only that specific artifact
- If neither is provided: builds all artifacts defined in forge.yaml
- Reads forge.yaml from current directory
- Updates artifact store with build results
- Invokes appropriate build engines via MCP

**Output:**
```text
Successfully built N artifact(s)
```

Or on error:
```text
Build failed: <error details>
Build completed with errors: <error list>. Successfully built N artifact(s)
```

**Example (build all):**
```json
{
  "method": "tools/call",
  "params": {
    "name": "build",
    "arguments": {}
  }
}
```

**Example (build specific artifact):**
```json
{
  "method": "tools/call",
  "params": {
    "name": "build",
    "arguments": {
      "name": "myapp"
    }
  }
}
```

## How It Works

1. Loads forge.yaml configuration from current directory
2. Reads existing artifact store
3. Filters build specs by artifact name (if provided)
4. Groups specs by build engine
5. Invokes each engine via MCP:
   - Single spec: calls engine's `build` tool
   - Multiple specs: calls engine's `buildBatch` tool
6. Updates artifact store with build results
7. Returns summary of build operations

## Integration with Forge

The forge MCP server orchestrates other MCP build engines:

```yaml
# forge.yaml
name: my-project
artifactStorePath: .forge/artifacts.yaml

build:
  - name: myapp
    src: ./cmd/myapp
    engine: go://build-go      # Invokes build-go MCP server

  - name: myimage
    src: ./Containerfile
    engine: go://build-container  # Invokes build-container MCP server
```

When you call the forge `build` tool, it:
1. Parses the engine URIs (e.g., `go://build-go`)
2. Launches the corresponding MCP server binary
3. Calls the appropriate tool on that server
4. Aggregates results

## CLI Usage

The forge CLI also supports traditional command-line usage:

```bash
# Build all artifacts
forge build

# Build specific artifact
forge build myapp

# Test operations
forge test unit run
forge test integration create
```

See [forge-usage.md](../../docs/forge-usage.md) for complete CLI documentation.

## Architecture

The forge MCP server acts as an orchestrator, coordinating multiple specialized MCP servers:

```
┌─────────────┐
│   AI Agent  │
│   or User   │
└──────┬──────┘
       │ MCP
┌──────▼──────┐
│    forge    │ MCP Server (orchestrator)
│  --mcp mode │
└──────┬──────┘
       │ Spawns and coordinates
       ├──────────────┬─────────────┐
       │              │             │
┌──────▼──────┐ ┌────▼────┐  ┌─────▼─────┐
│  build-go   │ │ testenv │  │test-runner│
│ MCP Server  │ │   MCP   │  │    MCP    │
└─────────────┘ └─────────┘  └───────────┘
```

## See Also

- [build-go MCP Server](../build-go/MCP.md)
- [build-container MCP Server](../build-container/MCP.md)
- [testenv MCP Server](../testenv/MCP.md)
- [Forge CLI Documentation](../../docs/forge-usage.md)
- [Forge Architecture](../../ARCHITECTURE.md)
