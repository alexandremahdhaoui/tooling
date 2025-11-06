# generic-test-runner MCP Server

MCP server for executing arbitrary commands as test runners.

## Purpose

Provides generic test execution for any CLI tool. Returns structured TestReport with pass/fail status based on exit code.

## Invocation

```bash
generic-test-runner --mcp
```

Forge invokes this via:
```yaml
runner: go://generic-test-runner
```

## Available Tools

### `run`

Execute command as test and generate TestReport.

**Input Schema:**
```json
{
  "stage": "string (required)",      // Test stage name
  "name": "string (required)",       // Test name
  "command": "string (required)",    // Shell command to execute
  "args": ["string"],                // Command arguments
  "env": {"key": "value"},           // Environment variables
  "envFile": "string",               // Path to env file
  "workDir": "string",               // Working directory
  "tmpDir": "string",                // Temporary directory
  "buildDir": "string",              // Build directory
  "rootDir": "string"                // Root directory
}
```

**Output:**
```json
{
  "id": "string",                    // Generated UUID
  "stage": "string",
  "status": "passed|failed",
  "startTime": "2025-01-06T10:00:00Z",
  "duration": 1.234,                 // Seconds
  "testStats": {
    "total": 1,
    "passed": 1,                     // If exit code 0
    "failed": 0,                     // If exit code != 0
    "skipped": 0
  },
  "errorMessage": "string"           // Populated on failure
}
```

**Example - Run linter:**
```json
{
  "method": "tools/call",
  "params": {
    "name": "run",
    "arguments": {
      "stage": "lint",
      "name": "golangci-lint",
      "command": "golangci-lint",
      "args": ["run", "./..."]
    }
  }
}
```

**Example - Run security scanner:**
```json
{
  "method": "tools/call",
  "params": {
    "name": "run",
    "arguments": {
      "stage": "security",
      "name": "gosec",
      "command": "gosec",
      "args": ["-fmt=json", "./..."]
    }
  }
}
```

## Integration with Forge

In `forge.yaml`:
```yaml
test:
  - name: lint
    stage: lint
    runner: go://generic-test-runner
    command: golangci-lint
    args: ["run", "./..."]

  - name: security
    stage: security
    runner: go://generic-test-runner
    command: gosec
    args: ["-fmt=json", "./..."]
```

Run with:
```bash
forge test run lint
forge test run security
```

## Use Cases

- Linters (golangci-lint, eslint, shellcheck)
- Security scanners (gosec, trivy, snyk)
- Custom test frameworks
- Compliance checkers
- Any pass/fail validation

## Status Determination

- **Exit code 0** → status: "passed"
- **Exit code != 0** → status: "failed"

TestReport.errorMessage contains stdout/stderr on failure.

## Implementation Details

- Executes command via exec.Command
- Captures stdout, stderr, exit code
- Measures execution duration
- Generates UUID for report ID
- Returns TestReport regardless of pass/fail

## See Also

- [test-runner-go MCP Server](../test-runner-go/MCP.md)
- [test-runner-go-verify-tags MCP Server](../test-runner-go-verify-tags/MCP.md)
- [generic-builder MCP Server](../generic-builder/MCP.md)
