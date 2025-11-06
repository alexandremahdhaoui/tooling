# test-report MCP Server

MCP server for managing test reports (get, list, delete).

## Purpose

Provides test report management operations. Test reports are created by test runners (test-runner-go, generic-test-runner), this server manages retrieval and cleanup.

## Invocation

```bash
test-report --mcp
```

Forge uses this for test report management.

## Available Tools

### `create`

No-op operation for interface compatibility.

**Input Schema:**
```json
{
  "stage": "string (required)"
}
```

**Output:**
```json
{
  "message": "No-op: test reports for stage 'unit' are created by test runners during execution"
}
```

**Note:** Test reports are actually created by test runners, not by this server. This tool exists only for interface compatibility.

### `get`

Get test report details by ID.

**Input Schema:**
```json
{
  "reportID": "string (required)"    // Test report UUID
}
```

**Output:**
```json
{
  "id": "string",
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
  "artifactFiles": ["junit.xml", "coverage.out"],
  "errorMessage": "string"
}
```

**Example:**
```json
{
  "method": "tools/call",
  "params": {
    "name": "get",
    "arguments": {
      "reportID": "550e8400-e29b-41d4-a716-446655440000"
    }
  }
}
```

### `delete`

Delete test report and its artifact files.

**Input Schema:**
```json
{
  "reportID": "string (required)"
}
```

**Output:**
```json
{
  "success": true,
  "message": "Deleted test report: 550e8400-e29b-41d4-a716-446655440000"
}
```

**What It Deletes:**
- TestReport from artifact store
- Associated artifact files (junit.xml, coverage.out, etc.)

**Example:**
```json
{
  "method": "tools/call",
  "params": {
    "name": "delete",
    "arguments": {
      "reportID": "550e8400-e29b-41d4-a716-446655440000"
    }
  }
}
```

### `list`

List test reports, optionally filtered by stage.

**Input Schema:**
```json
{
  "stage": "string (optional)"       // Filter by stage (e.g., "unit")
}
```

**Output:**
```json
[
  {
    "id": "string",
    "stage": "string",
    "status": "string",
    "startTime": "2025-01-06T10:00:00Z",
    "duration": 5.432,
    "testStats": {...}
  },
  ...
]
```

**Example - List all:**
```json
{
  "method": "tools/call",
  "params": {
    "name": "list",
    "arguments": {}
  }
}
```

**Example - Filter by stage:**
```json
{
  "method": "tools/call",
  "params": {
    "name": "list",
    "arguments": {
      "stage": "unit"
    }
  }
}
```

## Integration with Forge

Used internally by forge for report management:

```bash
# List reports
forge test report list

# Get report details
forge test report get <report-id>

# Delete report
forge test report delete <report-id>
```

## Storage Location

Reports stored in artifact store defined in forge.yaml:
```yaml
build:
  artifactStorePath: .forge/artifacts.yaml
```

## Implementation Details

- Reads/writes artifact store directly
- Deletes artifact files from filesystem
- Returns TestReport objects as defined in pkg/forge
- No test execution - only management operations

## See Also

- [test-runner-go MCP Server](../test-runner-go/MCP.md)
- [generic-test-runner MCP Server](../generic-test-runner/MCP.md)
- [Test Documentation](../../docs/forge-test-usage.md)
