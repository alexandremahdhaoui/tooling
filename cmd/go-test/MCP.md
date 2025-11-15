# go-test MCP Server

MCP server for running Go tests with JUnit XML and coverage reporting.

## Purpose

Runs Go tests for specific build tags (unit, integration, e2e), generates JUnit XML reports, collects coverage data, and stores results in artifact store.

## Invocation

```bash
go-test --mcp
```

Forge invokes this via:
```yaml
runner: go://go-test
```

## Available Tools

### `run`

Run Go tests and generate TestReport.

**Input Schema:**
```json
{
  "stage": "string (required)",      // Test stage: "unit", "integration", "e2e"
  "name": "string (required)",       // Test name
  "tmpDir": "string (optional)"      // Temp directory for output files (default: ".")
}
```

**Output:**
```json
{
  "id": "string",                    // Generated UUID
  "stage": "string",
  "status": "passed|failed",
  "startTime": "2025-01-06T10:00:00Z",
  "duration": 5.432,
  "testStats": {
    "total": 42,
    "passed": 40,
    "failed": 2,
    "skipped": 0
  },
  "coverage": {
    "percentage": 85.3,
    "coveredLines": 1024,
    "totalLines": 1200
  },
  "artifactFiles": [
    "junit.xml",
    "coverage.out"
  ],
  "errorMessage": "string"
}
```

**Example:**
```json
{
  "method": "tools/call",
  "params": {
    "name": "run",
    "arguments": {
      "stage": "unit",
      "name": "unit-tests",
      "tmpDir": ".forge/tmp/test-unit-20250106-abc123"
    }
  }
}
```

## Integration with Forge

In `forge.yaml`:
```yaml
test:
  - name: unit
    stage: unit
    engine: go://testenv
    runner: go://go-test

  - name: integration
    stage: integration
    engine: go://testenv
    runner: go://go-test
```

Run with:
```bash
forge test run unit
forge test run integration
```

## Go Test Execution

Runs:
```bash
go test -tags={stage} \
  -v \
  -coverprofile={tmpDir}/coverage.out \
  -covermode=atomic \
  ./...
```

Generates:
- JUnit XML using go-junit-report
- Coverage profile in {tmpDir}/coverage.out

## Build Tags

Maps stage to Go build tag:
- `stage: unit` → `-tags=unit`
- `stage: integration` → `-tags=integration`
- `stage: e2e` → `-tags=e2e`

Tests must have corresponding build tags:
```go
//go:build unit

package myapp_test
```

## Coverage Calculation

Parses coverage.out to compute:
- Percentage coverage
- Covered lines count
- Total lines count

## Artifact Storage

TestReport is automatically stored in artifact store at:
- Path: Defined in forge.yaml `artifactStorePath`
- Files: junit.xml and coverage.out in tmpDir

## Implementation Details

- Uses gotestsum for better output formatting
- Parses JUnit XML to extract test statistics
- Stores report with artifact files for later retrieval
- Returns failure status if any tests fail

## See Also

- [go-lint-tags MCP Server](../go-lint-tags/MCP.md)
- [generic-test-runner MCP Server](../generic-test-runner/MCP.md)
- [Test Documentation](../../docs/forge-test-usage.md)
