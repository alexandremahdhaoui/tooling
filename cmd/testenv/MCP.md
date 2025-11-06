# testenv MCP Server

MCP server for orchestrating test environment creation and deletion.

## Purpose

Main test environment orchestrator that coordinates testenv-kind and testenv-lcr to create complete test environments with Kind clusters and local container registries.

## Invocation

```bash
testenv --mcp
```

Forge invokes this automatically via:
```yaml
engine: go://testenv
```

## Available Tools

### `create`

Create a complete test environment.

**Input Schema:**
```json
{
  "stage": "string (required)"       // Test stage name (e.g., "unit", "integration")
}
```

**Output:**
```json
{
  "testID": "string"                 // e.g., "test-unit-20250106-abc123"
}
```

**What It Does:**
1. Generates unique test ID
2. Creates temporary directory for test files
3. Calls testenv-kind to create Kind cluster
4. Calls testenv-lcr to create registry (if enabled)
5. Stores TestEnvironment in artifact store

**Example:**
```json
{
  "method": "tools/call",
  "params": {
    "name": "create",
    "arguments": {
      "stage": "integration"
    }
  }
}
```

### `delete`

Delete a test environment and clean up all resources.

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
  "message": "Deleted test environment: test-unit-20250106-abc123"
}
```

**What It Does:**
1. Retrieves TestEnvironment from artifact store
2. Calls testenv-lcr delete (if registry was created)
3. Calls testenv-kind delete
4. Removes temporary directory
5. Removes TestEnvironment from artifact store

## Integration with Forge

In `forge.yaml`:
```yaml
test:
  - name: integration
    stage: integration
    engine: go://testenv
    runner: go://test-runner-go
```

Run with:
```bash
# Create environment
forge test create integration

# Delete environment
forge test delete <test-id>
```

## TestEnvironment Structure

Stored in artifact store:
```go
{
  "id": "test-unit-20250106-abc123",
  "name": "unit",
  "stage": "unit",
  "status": "created",
  "createdAt": "2025-01-06T10:00:00Z",
  "updatedAt": "2025-01-06T10:00:00Z",
  "tmpDir": ".forge/tmp/test-unit-20250106-abc123",
  "files": {
    "testenv-kind.kubeconfig": "kubeconfig",
    "testenv-lcr.ca.crt": "ca.crt",
    "testenv-lcr.credentials.yaml": "registry-credentials.yaml"
  },
  "metadata": {
    "testenv-kind.clusterName": "forge-test-unit-20250106-abc123",
    "testenv-kind.kubeconfigPath": ".forge/tmp/.../kubeconfig",
    "testenv-lcr.registryFQDN": "testenv-lcr.testenv-lcr.svc.cluster.local:5000"
  },
  "managedResources": [...]
}
```

## Notes

- `get` and `list` operations are NOT implemented as MCP tools
- Forge reads artifact store directly for get/list operations
- This design reduces complexity and improves performance

## See Also

- [testenv-kind MCP Server](../testenv-kind/MCP.md)
- [testenv-lcr MCP Server](../testenv-lcr/MCP.md)
- [Test Environment Documentation](../../docs/testenv-quick-start.md)
