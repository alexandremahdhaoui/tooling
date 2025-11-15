# forge-e2e MCP Server

MCP server for running end-to-end tests for the forge project.

## Purpose

Provides MCP tools for executing comprehensive end-to-end tests of the forge build and test orchestration system. Tests are categorized by functionality (build, testenv, test-runner, etc.) and can be filtered and run in parallel.

## Invocation

```bash
forge-e2e --mcp
```

Or configure in your AI agent's MCP settings:
```json
{
  "mcpServers": {
    "forge-e2e": {
      "command": "forge-e2e",
      "args": ["--mcp"]
    }
  }
}
```

## Available Tools

### `run`

Execute end-to-end tests for forge. Returns a structured test report with results, duration, and per-category statistics.

**Input Schema:**
```json
{
  "stage": "string (required)",         // Test stage name (e.g., "e2e")
  "name": "string (required)",          // Test run identifier
  "id": "string (optional)",            // Custom test ID
  "tmpDir": "string (optional)",        // Temporary directory path
  "buildDir": "string (optional)",      // Build output directory
  "rootDir": "string (optional)"        // Root directory for tests
}
```

**Output:**
Returns a JSON test report:
```json
{
  "status": "passed|failed",
  "errorMessage": "string",
  "duration": 123.45,
  "total": 42,
  "passed": 40,
  "failed": 2,
  "skipped": 0,
  "results": [
    {
      "name": "test name",
      "category": "build",
      "status": "passed",
      "duration": 1.23,
      "error": ""
    }
  ],
  "categories": {
    "build": {
      "total": 5,
      "passed": 5,
      "failed": 0,
      "skipped": 0,
      "duration": 10.5
    }
  }
}
```

**Example:**
```json
{
  "method": "tools/call",
  "params": {
    "name": "run",
    "arguments": {
      "stage": "e2e",
      "name": "test-20241106"
    }
  }
}
```

## Test Categories

forge-e2e organizes tests into the following categories:

- **build** - Build system tests (forge build commands)
- **testenv** - Test environment lifecycle tests (create, list, get, delete)
- **test-runner** - Test runner integration tests (unit, integration, lint)
- **prompt** - Prompt system tests (list, get prompts)
- **artifact-store** - Artifact store validation tests
- **system** - System command tests (version, help)
- **error-handling** - Error handling tests
- **cleanup** - Resource cleanup tests
- **mcp** - MCP integration tests
- **performance** - Performance tests

## Environment Variables

Control test execution with environment variables:

```bash
# Filter tests by category
TEST_CATEGORY=build forge-e2e e2e test-run

# Filter tests by name pattern
TEST_NAME_PATTERN=environment forge-e2e e2e test-run

# Required for testenv tests
KIND_BINARY=kind
CONTAINER_ENGINE=docker

# Skip cleanup for debugging
SKIP_CLEANUP=1 forge-e2e e2e test-run

# Enable verbose output
FORGE_E2E_VERBOSE=1 forge-e2e e2e test-run
```

## CLI Usage

The forge-e2e tool also supports traditional command-line usage:

```bash
# Run all e2e tests
forge-e2e e2e test-20241106

# Run only build tests
TEST_CATEGORY=build forge-e2e e2e test-20241106

# Run tests matching pattern
TEST_NAME_PATTERN=environment forge-e2e e2e test-20241106

# Keep test resources for debugging
SKIP_CLEANUP=1 forge-e2e e2e test-20241106
```

## Test Execution Strategy

### Parallel vs Sequential

- **Parallel tests**: Independent tests that can run concurrently (marked `Parallel: true`)
- **Sequential tests**: Tests that modify shared state or create/destroy resources (marked `Parallel: false`)

### Shared Test Environments

Some tests use a shared test environment to avoid repeated setup/teardown:
- Created once during test suite setup
- Reused across multiple tests
- Cleaned up during test suite teardown
- Skipped if `KIND_BINARY` not available

## Integration with Forge

forge-e2e is invoked by forge's test infrastructure:

```yaml
# forge.yaml
test:
  - name: e2e
    runner: go://forge-e2e
    tags: ["e2e"]
```

Run with:
```bash
forge test e2e run
```

## Output

- **stderr**: Test progress, status updates, and summary
- **stdout**: Structured JSON test report
- **Exit code**: 0 on success, 1 on failure

## See Also

- [forge MCP Server](../forge/MCP.md)
- [go-test MCP Server](../go-test/MCP.md)
- [Forge Test Documentation](../../docs/forge-test-usage.md)
- [Forge Architecture](../../ARCHITECTURE.md)
