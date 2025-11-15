# go-format MCP Server

MCP server for formatting Go code using gofmt.

## Purpose

Provides MCP tools for automatically formatting Go source code to maintain consistent code style across the project.

## Invocation

```bash
go-format --mcp
```

Forge invokes this automatically via:
```yaml
engine: go://go-format
```

## Available Tools

### `build`

Format Go code in the specified directory.

**Input Schema:**
```json
{
  "name": "string (required)",        // Format task name (e.g., "format-code")
  "src": "string (required)",         // Source directory to format (e.g., "." or "./pkg")
  "engine": "string (optional)",      // Engine reference
  "tmpDir": "string (optional)",      // Temporary directory (injected by forge)
  "buildDir": "string (optional)",    // Build directory (injected by forge)
  "rootDir": "string (optional)"      // Root directory (injected by forge)
}
```

**Output:**
```json
{
  "name": "string",
  "type": "formatted",
  "location": "string",              // Directory that was formatted
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
      "name": "format-code",
      "src": "."
    }
  }
}
```

## Integration with Forge

In `forge.yaml`:
```yaml
build:
  - name: format-code
    src: .
    engine: go://go-format
```

Run with:
```bash
forge build
```

## Implementation Details

- Runs `gofmt -s -w` recursively on all Go files
- `-s` flag simplifies code where possible
- `-w` flag writes changes directly to files
- Formats all `.go` files in the specified source directory
- Returns formatted code artifact metadata

## Formatting Behavior

- Applies standard Go formatting rules
- Simplifies code constructs (e.g., slice literals)
- Modifies files in-place
- No output if files are already formatted
- Non-zero exit if formatting changes were needed

## See Also

- [go-lint MCP Server](../go-lint/MCP.md)
- [go-build MCP Server](../go-build/MCP.md)
- [Forge Build Documentation](../../docs/forge-usage.md#building-artifacts)
