# go-gen-mocks MCP Server

MCP server for generating Go mocks using mockery.

## Purpose

Provides MCP tools for generating mock implementations of Go interfaces using mockery, enabling easier unit testing with dependency injection.

## Invocation

```bash
go-gen-mocks --mcp
```

Forge invokes this automatically via:
```yaml
engine: go://go-gen-mocks
```

## Available Tools

### `build`

Generate Go mocks using mockery.

**Input Schema:**
```json
{
  "name": "string (required)",        // Generation task name (e.g., "go-gen-mocks")
  "engine": "string (optional)",      // Engine reference
  "tmpDir": "string (optional)",      // Temporary directory (injected by forge)
  "buildDir": "string (optional)",    // Build directory (injected by forge)
  "rootDir": "string (optional)"      // Root directory (injected by forge)
}
```

**Output:**
```json
{
  "name": "mocks",
  "type": "generated",
  "location": "string",              // Directory where mocks were generated
  "timestamp": "string"              // RFC3339 format
}
```

**Example:**
```json
{
  "method": "tools/call",
  "params": {
    "name": "build",
    "arguments": {
      "name": "go-gen-mocks"
    }
  }
}
```

## Integration with Forge

In `forge.yaml`:
```yaml
build:
  - name: go-gen-mocks
    engine: go://go-gen-mocks
```

Run with:
```bash
forge build
```

## Environment Variables

- **MOCKERY_VERSION**: Version of mockery to use (default: `v3.5.5`)
- **MOCKS_DIR**: Directory to clean/generate mocks (default: `./internal/util/mocks`)

## Implementation Details

- Cleans existing mocks directory before generating
- Runs `go run github.com/vektra/mockery/v3@{version}`
- Discovers interfaces automatically via mockery configuration
- Generates mock implementations in specified output directory
- Returns generated artifact metadata

## Mockery Configuration

Uses `.mockery.yaml` or `mockery.yaml` in project root for configuration. Example:
```yaml
with-expecter: true
dir: "./internal/util/mocks"
packages:
  github.com/myorg/myproject/pkg/interfaces:
    interfaces:
      MyInterface:
```

## Behavior

- **Cleans target directory**: Removes all existing files in mocks directory
- **Generates mocks**: Creates mock implementations for configured interfaces
- **Output location**: Controlled by `MOCKS_DIR` environment variable or default
- **In-place generation**: Mocks are written directly to the output directory

## See Also

- [go-build MCP Server](../go-build/MCP.md)
- [go-gen-openapi MCP Server](../go-gen-openapi/MCP.md)
- [Mockery Documentation](https://vektra.github.io/mockery/)
