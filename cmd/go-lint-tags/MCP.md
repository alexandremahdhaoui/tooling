# go-lint-tags MCP Server

MCP server for verifying that all Go test files have valid build tags.

## Purpose

Scans repository for test files and verifies each has one of the required build tags: `unit`, `integration`, or `e2e`. Prevents tests from being silently skipped due to missing tags.

## Invocation

```bash
go-lint-tags --mcp
```

Forge invokes this via:
```yaml
runner: go://go-lint-tags
```

## Available Tools

### `run`

Verify all test files have valid build tags.

**Input Schema:**
```json
{
  "stage": "string (required)",      // Test stage name
  "rootDir": "string (optional)"     // Root directory to scan (default: ".")
}
```

**Output:**
```json
{
  "id": "string",
  "stage": "string",
  "status": "passed|failed",
  "startTime": "2025-01-06T10:00:00Z",
  "duration": 0.123,
  "testStats": {
    "total": 42,                     // Total test files found
    "passed": 42,                    // Files with valid tags
    "failed": 0,                     // Files without tags
    "skipped": 0
  },
  "errorMessage": "string"           // Details of files without tags
}
```

**Example:**
```json
{
  "method": "tools/call",
  "params": {
    "name": "run",
    "arguments": {
      "stage": "verify-tags",
      "rootDir": "."
    }
  }
}
```

## Integration with Forge

In `forge.yaml`:
```yaml
test:
  - name: verify-tags
    stage: verify-tags
    runner: go://go-lint-tags
```

Run with:
```bash
forge test run verify-tags
```

## Validation Rules

**Valid Build Tags:**
- `//go:build unit`
- `//go:build integration`
- `//go:build e2e`

**Files Checked:**
- All `*_test.go` files
- Recursively scans rootDir

**Passes If:**
- All test files have one of the valid build tags

**Fails If:**
- Any test file missing build tag
- Error message lists all files without tags

## Error Message Format

On failure:
```
Found 3 test file(s) without build tags out of 45 total files

Files missing build tags:
  - pkg/myapp/handler_test.go
  - pkg/utils/helper_test.go
  - cmd/server/main_test.go

Test files must have one of these build tags:
  //go:build unit
  //go:build integration
  //go:build e2e
```

## Use Case

Run as pre-commit check or in CI to ensure:
- Tests are properly tagged
- Tests won't be accidentally skipped
- Build tag conventions are enforced

## Implementation Details

- Walks directory tree recursively
- Parses Go files to check for build tags
- Returns detailed error with file list on failure
- Fast execution (no test compilation)

## See Also

- [go-test MCP Server](../go-test/MCP.md)
- [Test Documentation](../../docs/test-runner-guide.md)
