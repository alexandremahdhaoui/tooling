# Engine Test Package

This package provides standardized unit tests for all forge engines/tools.

## Overview

The `enginetest` package ensures that all engines in the forge repository implement:
1. **Version commands**: All engines must support `version`, `--version`, and `-v` flags
2. **MCP mode**: Build engines (go-build, container-build) must support `--mcp` flag for MCP server mode
3. **Binary existence**: All binaries must exist and be executable

## Usage

### Running All Tests

```bash
# Build all tools first (or use forge build)
forge build

# Run tests
go test ./internal/enginetest -v
```

### Test Coverage

The package tests all 18 engines:

| Engine | Version Support | MCP Support |
|--------|----------------|-------------|
| forge | ✅ | ✅ |
| go-build | ✅ | ✅ |
| container-build | ✅ | ✅ |
| generic-builder | ✅ | ✅ |
| testenv | ✅ | ✅ |
| testenv-kind | ✅ | ✅ |
| testenv-lcr | ✅ | ✅ |
| testenv-helm-install | ✅ | ✅ |
| go-test | ✅ | ✅ |
| go-lint-tags | ✅ | ✅ |
| generic-test-runner | ✅ | ✅ |
| test-report | ✅ | ✅ |
| go-format | ✅ | ✅ |
| go-lint | ✅ | ✅ |
| go-gen-mocks | ✅ | ✅ |
| go-gen-openapi | ✅ | ✅ |
| ci-orchestrator | ✅ | ✅ |
| forge-e2e | ✅ | ✅ |

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
Tests that MCP engines (go-build, container-build) support MCP server mode:
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
