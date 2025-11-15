# forge MCP Server

MCP server for build orchestration and test environment management.

## Purpose

The forge CLI itself runs as an MCP server, providing AI agents direct access to build orchestration capabilities. When invoked with `--mcp`, forge exposes tools to build artifacts from forge.yaml configuration.

## Invocation

```bash
forge --mcp
```

Or configure in your AI agent's MCP settings:
```json
{
  "mcpServers": {
    "forge": {
      "command": "forge",
      "args": ["--mcp"]
    }
  }
}
```

## Available Tools

### `build`

Build artifacts defined in forge.yaml configuration. Can build all artifacts or a specific artifact by name.

**Input Schema:**
```json
{
  "name": "string (optional)",           // Specific artifact name to build
  "artifactName": "string (optional)"    // Alternative to "name"
}
```

**Behavior:**
- If `name` or `artifactName` is provided: builds only that specific artifact
- If neither is provided: builds all artifacts defined in forge.yaml
- Reads forge.yaml from current directory
- Updates artifact store with build results
- Invokes appropriate build engines via MCP

**Output:**

Returns both a text message and a structured artifact containing an array of `Artifact` objects:

```json
{
  "content": [{
    "type": "text",
    "text": "Successfully built N artifact(s)"
  }],
  "artifact": [
    {
      "name": "myapp",
      "type": "binary",
      "location": "./build/bin/myapp",
      "timestamp": "2025-01-15T10:30:00Z",
      "version": "abc123def"
    }
  ]
}
```

**Artifact Schema:**
Each artifact in the array contains:
- `name` (string): Artifact name
- `type` (string): Artifact type (e.g., "binary", "container")
- `location` (string): File path or URL to the artifact
- `timestamp` (string): Build timestamp (RFC3339 format)
- `version` (string): Git commit hash or version identifier

Or on error:
```text
Build failed: <error details>
Build completed with errors: <error list>. Successfully built N artifact(s)
```

**Example (build all):**
```json
{
  "method": "tools/call",
  "params": {
    "name": "build",
    "arguments": {}
  }
}
```

**Example (build specific artifact):**
```json
{
  "method": "tools/call",
  "params": {
    "name": "build",
    "arguments": {
      "name": "myapp"
    }
  }
}
```

---

### `test-create`

Create a test environment for a specific test stage.

**Input Schema:**
```json
{
  "stage": "string (required)"  // Test stage name (e.g., "integration", "e2e")
}
```

**Output:**

Returns a structured `TestEnvironment` object with complete environment details:

```json
{
  "content": [{
    "type": "text",
    "text": "Successfully created test environment for stage: integration"
  }],
  "artifact": {
    "id": "test-uuid-123",
    "name": "integration",
    "status": "created",
    "createdAt": "2025-01-15T10:30:00Z",
    "updatedAt": "2025-01-15T10:30:00Z",
    "tmpDir": "/tmp/forge-test-integration-test-uuid-123",
    "files": {
      "testenv-kind.kubeconfig": "kubeconfig",
      "testenv-lcr.credentials": "credentials.json"
    },
    "managedResources": [
      "/tmp/forge-test-integration-test-uuid-123"
    ],
    "metadata": {
      "testenv-kind.clusterName": "forge-integration-test-uuid-123",
      "testenv-lcr.registryURL": "https://localhost:5000"
    }
  }
}
```

**TestEnvironment Schema:**
- `id` (string): Unique test environment identifier
- `name` (string): Test stage name
- `status` (string): Environment status ("created", "running", "passed", "failed", "partially_deleted")
- `createdAt` (string): Creation timestamp (RFC3339)
- `updatedAt` (string): Last update timestamp (RFC3339)
- `tmpDir` (string): Temporary directory path for this environment
- `files` (object): Map of file keys to relative paths (relative to tmpDir)
- `managedResources` (array): List of files/directories managed by this environment
- `metadata` (object): Engine-specific metadata (namespaced by engine name)

---

### `test-get`

Retrieve details of a specific test environment.

**Input Schema:**
```json
{
  "stage": "string (required)",   // Test stage name
  "testID": "string (required)",  // Test environment ID
  "format": "string (optional)"   // Output format: "json", "yaml", or "table" (default)
}
```

**Output:**

Returns the same `TestEnvironment` structure as `test-create`.

---

### `test-list`

List all test reports for a specific test stage.

**Input Schema:**
```json
{
  "stage": "string (required)",  // Test stage name
  "format": "string (optional)"  // Output format: "json", "yaml", or "table" (default)
}
```

**Output:**

Returns an array of `TestReport` objects:

```json
{
  "content": [{
    "type": "text",
    "text": "Successfully listed 2 test report(s) for stage: unit"
  }],
  "artifact": [
    {
      "id": "test-report-unit-20251109-abc123",
      "stage": "unit",
      "status": "passed",
      "testStats": {
        "total": 42,
        "passed": 42,
        "failed": 0,
        "skipped": 0
      },
      "coverage": {
        "percentage": 85.5
      },
      ...
    },
    {
      "id": "test-report-unit-20251109-def456",
      "stage": "unit",
      "status": "failed",
      "testStats": {
        "total": 42,
        "passed": 40,
        "failed": 2,
        "skipped": 0
      },
      ...
    }
  ]
}
```

---

### `test-run`

Run tests for a specific test stage.

**Input Schema:**
```json
{
  "stage": "string (required)",   // Test stage name
  "testID": "string (optional)"   // Existing test environment ID (auto-creates if not provided)
}
```

**Output:**

Returns a structured `TestReport` object with detailed test results:

```json
{
  "content": [{
    "type": "text",
    "text": "Successfully ran tests for stage: unit"
  }],
  "isError": false,
  "artifact": {
    "id": "report-uuid-789",
    "stage": "unit",
    "status": "passed",
    "startTime": "2025-01-15T10:30:00Z",
    "duration": 12.5,
    "testStats": {
      "total": 42,
      "passed": 42,
      "failed": 0,
      "skipped": 0
    },
    "coverage": {
      "percentage": 85.5,
      "filePath": ".forge/tmp/coverage.out"
    },
    "artifactFiles": [
      ".forge/tmp/test-report.xml"
    ],
    "outputPath": ".forge/tmp/test-output.log",
    "errorMessage": "",
    "createdAt": "2025-01-15T10:30:12Z",
    "updatedAt": "2025-01-15T10:30:12Z"
  }
}
```

**TestReport Schema:**
- `id` (string): Unique test report identifier
- `stage` (string): Test stage name
- `status` (string): Test result ("passed" or "failed")
- `startTime` (string): Test start timestamp (RFC3339)
- `duration` (number): Test duration in seconds
- `testStats` (object):
  - `total` (number): Total number of tests
  - `passed` (number): Number of passed tests
  - `failed` (number): Number of failed tests
  - `skipped` (number): Number of skipped tests
- `coverage` (object):
  - `percentage` (number): Code coverage percentage (0-100)
  - `filePath` (string): Path to coverage file
- `artifactFiles` (array): List of artifact files generated (e.g., XML reports)
- `outputPath` (string): Path to detailed test output
- `errorMessage` (string): Error message if tests failed
- `createdAt` (string): Report creation timestamp (RFC3339)
- `updatedAt` (string): Last update timestamp (RFC3339)

**Note:** If tests fail, `isError` is set to `true` but the artifact still contains the full `TestReport`.

---

### `test-delete`

Delete a test environment.

**Input Schema:**
```json
{
  "stage": "string (required)",  // Test stage name
  "testID": "string (required)"  // Test environment ID to delete
}
```

**Output:**
```text
Successfully deleted test environment: test-uuid-123
```

---

### `test-all`

Build all artifacts and run all test stages sequentially.

**Input Schema:**
```json
{}  // No parameters required
```

**Output:**

Returns an aggregated `TestAllResult` object containing all build artifacts and test reports:

```json
{
  "content": [{
    "type": "text",
    "text": "Successfully completed test-all: 3 artifact(s) built, 4 test stage(s) run, 4 passed, 0 failed"
  }],
  "artifact": {
    "buildArtifacts": [
      {
        "name": "myapp",
        "type": "binary",
        "location": "./build/bin/myapp",
        "timestamp": "2025-01-15T10:30:00Z",
        "version": "abc123def"
      }
    ],
    "testReports": [
      {
        "id": "report-uuid-1",
        "stage": "verify-tags",
        "status": "passed",
        ...
      },
      {
        "id": "report-uuid-2",
        "stage": "unit",
        "status": "passed",
        ...
      }
    ],
    "summary": "3 artifact(s) built, 4 test stage(s) run, 4 passed, 0 failed"
  }
}
```

**TestAllResult Schema:**
- `buildArtifacts` (array): Array of `Artifact` objects (see `build` tool schema)
- `testReports` (array): Array of `TestReport` objects (see `test-run` tool schema)
- `summary` (string): Human-readable summary of results

**Note:** If any tests fail, `isError` is set to `true` but the artifact still contains all results.

---

### `config-validate`

Validate forge.yaml configuration file.

**Input Schema:**
```json
{
  "configPath": "string (optional)"  // Path to config file (defaults to "forge.yaml")
}
```

**Output:**
```text
Configuration is valid
```

Or on error:
```text
Configuration validation failed: <error details>
```

---

### `prompt-list`

List all available AI assistant prompts.

**Input Schema:**
```json
{}  // No parameters
```

**Output:**
Lists available prompts for AI-assisted development tasks.

---

### `prompt-get`

Retrieve a specific AI assistant prompt.

**Input Schema:**
```json
{
  "name": "string (required)"  // Prompt name
}
```

**Output:**
Returns the requested prompt content.

---

## How It Works

1. Loads forge.yaml configuration from current directory
2. Reads existing artifact store
3. Filters build specs by artifact name (if provided)
4. Groups specs by build engine
5. Invokes each engine via MCP:
   - Single spec: calls engine's `build` tool
   - Multiple specs: calls engine's `buildBatch` tool
6. Updates artifact store with build results
7. Returns summary of build operations

## Integration with Forge

The forge MCP server orchestrates other MCP build engines:

```yaml
# forge.yaml
name: my-project
artifactStorePath: .forge/artifacts.yaml

build:
  - name: myapp
    src: ./cmd/myapp
    engine: go://go-build      # Invokes go-build MCP server

  - name: myimage
    src: ./Containerfile
    engine: go://container-build  # Invokes container-build MCP server
```

When you call the forge `build` tool, it:
1. Parses the engine URIs (e.g., `go://go-build`)
2. Launches the corresponding MCP server binary
3. Calls the appropriate tool on that server
4. Aggregates results

## CLI Usage

The forge CLI also supports traditional command-line usage:

```bash
# Build all artifacts
forge build

# Build specific artifact
forge build myapp

# Test operations (new command structure)
forge test run unit                 # Run tests
forge test list unit                # List test reports
forge test get unit <TEST_ID>       # Get test report details
forge test delete unit <TEST_ID>    # Delete test report

# Test environment management
forge test list-env integration     # List test environments
forge test get-env integration <ENV_ID>    # Get environment details
forge test create-env integration   # Create test environment
forge test delete-env integration <ENV_ID> # Delete test environment
```

See [forge-usage.md](../../docs/forge-usage.md) for complete CLI documentation.

## Architecture

The forge MCP server acts as an orchestrator, coordinating multiple specialized MCP servers:

```
┌─────────────┐
│   AI Agent  │
│   or User   │
└──────┬──────┘
       │ MCP
┌──────▼──────┐
│    forge    │ MCP Server (orchestrator)
│  --mcp mode │
└──────┬──────┘
       │ Spawns and coordinates
       ├──────────────┬─────────────┐
       │              │             │
┌──────▼──────┐ ┌────▼────┐  ┌─────▼─────┐
│  go-build   │ │ testenv │  │test-runner│
│ MCP Server  │ │   MCP   │  │    MCP    │
└─────────────┘ └─────────┘  └───────────┘
```

## See Also

- [go-build MCP Server](../go-build/MCP.md)
- [container-build MCP Server](../container-build/MCP.md)
- [testenv MCP Server](../testenv/MCP.md)
- [Forge CLI Documentation](../../docs/forge-usage.md)
- [Forge Architecture](../../ARCHITECTURE.md)
