# go-lint MCP Server

MCP server for running Go linter (golangci-lint) and generating test reports.

## Purpose

Runs golangci-lint on Go code, generates test reports with pass/fail status, and provides structured output for integration with forge test system.

## Invocation

```bash
go-lint --mcp
```

Forge invokes this via:
```yaml
runner: go://go-lint
```

## Available Tools

### `run`

Run golangci-lint and generate TestReport.

**Input Schema:**
```json
{
  "id": "string (optional)",         // Test run ID (injected by forge)
  "stage": "string (required)",      // Test stage (e.g., "lint")
  "name": "string (required)",       // Test name
  "tmpDir": "string (optional)",     // Temp directory (injected by forge)
  "buildDir": "string (optional)",   // Build directory (injected by forge)
  "rootDir": "string (optional)"     // Root directory (injected by forge)
}
```

**Output:**
```json
{
  "status": "passed|failed",
  "error": "string (if failed)",
  "duration": 5.432,
  "total": 1,                        // Issues found (0 if passed, 1+ if failed)
  "passed": 1,                       // 1 if passed, 0 if failed
  "failed": 0                        // 0 if passed, 1 if failed
}
```

**Example:**
```json
{
  "method": "tools/call",
  "params": {
    "name": "run",
    "arguments": {
      "stage": "lint",
      "name": "lint-check",
      "id": "lint-20250106-abc123"
    }
  }
}
```

## Integration with Forge

In `forge.yaml`:
```yaml
test:
  - name: lint
    runner: go://go-lint
```

Run with:
```bash
forge test lint run
```

Or as part of test-all:
```bash
forge test-all
```

## Implementation Details

- Runs `go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@{version} run --fix`
- Version controlled via `GOLANGCI_LINT_VERSION` environment variable (default: v2.6.0)
- Automatically applies fixes where possible (`--fix` flag)
- Returns structured test report for artifact store integration
- Lint output written to stderr
- JSON report written to stdout

## Environment Variables

- **GOLANGCI_LINT_VERSION**: Version of golangci-lint to use (default: `v2.6.0`)

## Exit Behavior

- Exit code 0: Linting passed (no issues)
- Exit code 1: Linting failed (issues found)

## Lint Configuration

Uses `.golangci.yml` in project root if present. Falls back to golangci-lint defaults.

## See Also

- [go-test MCP Server](../go-test/MCP.md)
- [go-format MCP Server](../go-format/MCP.md)
- [Forge Test Documentation](../../docs/forge-test-usage.md)
