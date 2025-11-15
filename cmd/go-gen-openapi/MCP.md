# go-gen-openapi MCP Server

MCP server for generating OpenAPI client and server code from specifications.

## Purpose

Provides MCP tools for generating Go client and server code from OpenAPI specifications using oapi-codegen, enabling type-safe API implementations.

## Invocation

```bash
go-gen-openapi --mcp
```

Forge invokes this automatically via:
```yaml
engine: go://go-gen-openapi
```

## Available Tools

### `build`

Generate OpenAPI client and server code from specifications.

**Input Schema:**
```json
{
  "name": "string (required)",        // Generation task name
  "engine": "string (optional)",      // Engine reference
  "tmpDir": "string (optional)",      // Temporary directory (injected by forge)
  "buildDir": "string (optional)",    // Build directory (injected by forge)
  "rootDir": "string (optional)"      // Root directory (injected by forge)
}
```

**Output:**
```json
{
  "name": "openapi-generated-code",
  "type": "generated",
  "location": "pkg/generated",       // Generated code directory
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
      "name": "generate-openapi"
    }
  }
}
```

## Integration with Forge

In `forge.yaml`:
```yaml
build:
  - name: generate-openapi
    engine: go://go-gen-openapi

generateOpenAPI:
  defaults:
    sourceDir: "./api/openapi"
    destinationDir: "./pkg/generated"

  specs:
    - name: "myapi"
      versions: ["v1", "v2"]
      client:
        enabled: true
        packageName: "myapi_client"
      server:
        enabled: true
        packageName: "myapi_server"
```

Run with:
```bash
forge build
```

## Environment Variables

- **OAPI_CODEGEN_VERSION**: Version of oapi-codegen to use (default: `v2.3.0`)
- **OPENAPI_CONFIG_PATH**: Path to forge.yaml configuration (default: `./forge.yaml`)

## Implementation Details

- Reads `generateOpenAPI` configuration from forge.yaml
- Runs `go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@{version}`
- Generates client and/or server code based on configuration
- Supports multiple API specifications and versions
- Parallel generation for better performance
- Creates temporary oapi-codegen config files for each package

## Configuration Structure

The `generateOpenAPI` section in forge.yaml supports:

```yaml
generateOpenAPI:
  defaults:
    sourceDir: "./api/openapi"      # Where .yaml specs are located
    destinationDir: "./pkg/generated"

  specs:
    - name: "api-name"
      source: "./path/to/spec.yaml" # Optional: override source file
      versions: ["v1", "v2"]        # API versions

      client:
        enabled: true
        packageName: "client_pkg"

      server:
        enabled: true
        packageName: "server_pkg"
```

## Generated Code

- **Client**: HTTP client with typed methods
- **Server**: HTTP server interfaces and strict handlers
- **Models**: Go structs for request/response types
- **Embedded Spec**: OpenAPI specification embedded in code

## Naming Convention

Source files should follow: `{name}.{version}.yaml`

Example: `myapi.v1.yaml`, `myapi.v2.yaml`

## See Also

- [go-build MCP Server](../go-build/MCP.md)
- [go-gen-mocks MCP Server](../go-gen-mocks/MCP.md)
- [oapi-codegen Documentation](https://github.com/oapi-codegen/oapi-codegen)
