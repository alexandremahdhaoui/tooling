# generic-builder MCP Server

MCP server for executing arbitrary shell commands as build steps.

## Purpose

Provides generic command execution for build workflows. Use when you need to integrate any CLI tool into forge builds without writing custom Go code.

## Invocation

```bash
generic-builder --mcp
```

Forge invokes this via:
```yaml
builder: go://generic-builder
```

## Available Tools

### `build`

Execute a shell command and return structured output.

**Input Schema:**
```json
{
  "name": "string (required)",       // Build name
  "command": "string (required)",    // Shell command to execute
  "args": ["string"],                // Command arguments (supports templates)
  "env": {"key": "value"},           // Environment variables
  "envFile": "string",               // Path to env file
  "workDir": "string",               // Working directory
  "src": "string",                   // Source directory (for templates)
  "dest": "string",                  // Destination (for templates)
  "version": "string"                // Version (for templates)
}
```

**Template Support:**

Arguments support Go template syntax with these fields:
- `{{ .Name }}` - Build name
- `{{ .Src }}` - Source directory
- `{{ .Dest }}` - Destination directory
- `{{ .Version }}` - Version string

**Output:**
```json
{
  "name": "string",
  "type": "command-output",
  "location": "string",              // workDir or src or "."
  "timestamp": "string",
  "version": "string"                // "{command}-exit{code}"
}
```

**Example - Run formatter:**
```json
{
  "method": "tools/call",
  "params": {
    "name": "build",
    "arguments": {
      "name": "format-code",
      "command": "gofumpt",
      "args": ["-w", "{{ .Src }}"],
      "src": "./cmd/myapp"
    }
  }
}
```

**Example - Generate code:**
```json
{
  "method": "tools/call",
  "params": {
    "name": "build",
    "arguments": {
      "name": "generate-proto",
      "command": "protoc",
      "args": [
        "--go_out={{ .Dest }}",
        "--go_opt=paths=source_relative",
        "{{ .Src }}/api.proto"
      ],
      "src": "./proto",
      "dest": "./pkg/api"
    }
  }
}
```

## Integration with Forge

In `forge.yaml`:
```yaml
build:
  specs:
    - name: format-code
      command: gofumpt
      args: ["-w", "./..."]
      workDir: .
      builder: go://generic-builder

    - name: generate-mocks
      command: mockery
      args: ["--all", "--output", "{{ .Dest }}"]
      dest: ./mocks
      builder: go://generic-builder
```

## Use Cases

- Code formatting (gofmt, prettier, black)
- Code generation (protoc, mockgen, swagger-codegen)
- Linting (in build phase, though test phase is better)
- Asset compilation
- Any CLI tool that produces build artifacts

## Error Handling

- Exit code 0: Success → Returns Artifact
- Exit code != 0: Failure → Returns error with stdout/stderr

## Implementation Details

- Executes commands via exec.Command
- Captures stdout, stderr, and exit code
- Processes template arguments before execution
- Working directory defaults to current directory

## See Also

- [go-build MCP Server](../go-build/MCP.md)
- [container-build MCP Server](../container-build/MCP.md)
- [generic-test-runner MCP Server](../generic-test-runner/MCP.md)
