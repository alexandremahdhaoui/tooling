# Engine Test Package

This package provides standardized unit tests for all forge engines/tools.

## Overview

The `enginetest` package ensures that all engines in the forge repository implement:
1. **Version commands**: All engines must support `version`, `--version`, and `-v` flags
2. **MCP mode**: Build engines (build-go, build-container) must support `--mcp` flag for MCP server mode
3. **Binary existence**: All binaries must exist and be executable

## Usage

### Running All Tests

```bash
# Build all tools first
go build -o ./build/bin/forge ./cmd/forge
go build -o ./build/bin/build-go ./cmd/build-go
go build -o ./build/bin/build-container ./cmd/build-container
go build -o ./build/bin/kindenv ./cmd/kindenv
go build -o ./build/bin/local-container-registry ./cmd/local-container-registry
go build -o ./build/bin/test-runner-go ./cmd/test-runner-go

# Run tests
go test ./internal/enginetest -v
```

### Test Coverage

The package tests all engines:

| Engine | Version Support | MCP Support |
|--------|----------------|-------------|
| forge | ✅ | ❌ |
| build-go | ✅ | ✅ |
| build-container | ✅ | ✅ |
| kindenv | ✅ | ❌ |
| local-container-registry | ✅ | ❌ |
| test-runner-go | ✅ | ✅ |

## Test Functions

### `TestAllEnginesHaveVersionSupport`
Tests that all engines support version commands:
- `tool version`
- `tool --version`
- `tool -v`

Verifies that the output contains:
- Tool name and version
- Commit SHA
- Build timestamp
- Go version
- Platform information

### `TestAllMCPEnginesHaveMCPSupport`
Tests that MCP engines (build-go, build-container) support MCP server mode:
- Can be started with `--mcp` flag
- Accept JSON-RPC requests on stdin
- Respond with JSON-RPC on stdout

### `TestEnginesList`
Verifies that the expected number of engines are configured in the test suite.

### `TestMCPEnginesConfiguration`
Verifies that the correct engines are configured to support MCP mode.

## Adding a New Engine

To add a new engine to the test suite:

1. Build the engine binary in `./build/bin/`
2. Add it to the `AllEngines()` function in `enginetest.go`:
   ```go
   {Name: "new-engine", BinaryPath: filepath.Join(buildBin, "new-engine"), SupportsMCP: false},
   ```
3. Run the tests to verify:
   ```bash
   go test ./internal/enginetest -v
   ```

## Engine Structure

All engines should:
1. Implement version support using the `internal/version` package
2. Support MCP mode (if it's a build engine) with `--mcp` flag
3. Be built to `./build/bin/<engine-name>`

## Related Packages

- `internal/version`: Common version information handling
- `cmd/*/main.go`: Individual engine implementations
