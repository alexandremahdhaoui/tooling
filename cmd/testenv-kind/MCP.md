# testenv-kind MCP Server

MCP server for managing Kind (Kubernetes in Docker) clusters for test environments.

## Purpose

Creates and deletes Kind clusters with unique names per test environment. Generates kubeconfig files and manages cluster lifecycle.

## Invocation

```bash
testenv-kind --mcp
```

Called by testenv orchestrator automatically.

## Available Tools

### `create`

Create a Kind cluster for a test environment.

**Input Schema:**
```json
{
  "testID": "string (required)",     // Test environment ID
  "stage": "string (required)",      // Test stage name
  "tmpDir": "string (required)"      // Temporary directory for files
}
```

**Output:**
```json
{
  "testID": "string",
  "files": {
    "testenv-kind.kubeconfig": "kubeconfig"  // Relative path in tmpDir
  },
  "metadata": {
    "testenv-kind.clusterName": "forge-test-unit-20250106-abc123",
    "testenv-kind.kubeconfigPath": "/abs/path/to/tmpDir/kubeconfig"
  },
  "managedResources": [
    "/abs/path/to/tmpDir/kubeconfig"
  ]
}
```

**What It Does:**
1. Generates cluster name: `{projectName}-{testID}`
2. Creates Kind cluster with custom config
3. Generates kubeconfig at `{tmpDir}/kubeconfig`
4. Returns file locations and metadata for testenv

**Example:**
```json
{
  "method": "tools/call",
  "params": {
    "name": "create",
    "arguments": {
      "testID": "test-unit-20250106-abc123",
      "stage": "unit",
      "tmpDir": ".forge/tmp/test-unit-20250106-abc123"
    }
  }
}
```

### `delete`

Delete a Kind cluster.

**Input Schema:**
```json
{
  "testID": "string (required)"      // Test environment ID
}
```

**Output:**
```json
{
  "success": true,
  "message": "Deleted kind cluster: forge-test-unit-20250106-abc123"
}
```

**What It Does:**
1. Reconstructs cluster name from testID
2. Runs `kind delete cluster --name {clusterName}`
3. Best-effort cleanup (doesn't fail if cluster already gone)

## Integration

Called by testenv MCP server during test environment creation/deletion.

## Configuration

Reads from `forge.yaml`:
```yaml
kindenv:
  kubeconfigPath: .forge/kubeconfig  # Ignored in MCP mode, uses tmpDir
```

## Implementation Details

- Uses `kind create cluster` command
- Cluster name format: `{projectName}-{testID}`
- Kubeconfig written to tmpDir for isolation
- Each test environment gets its own cluster

## See Also

- [testenv MCP Server](../testenv/MCP.md)
- [testenv-lcr MCP Server](../testenv-lcr/MCP.md)
