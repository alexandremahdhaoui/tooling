# testenv-lcr MCP Server

MCP server for deploying local container registry with TLS in Kind clusters.

## Purpose

Creates TLS-enabled container registry inside Kind clusters with cert-manager, self-signed certificates, and htpasswd authentication. Manages complete registry lifecycle.

## Invocation

```bash
testenv-lcr --mcp
```

Called by testenv orchestrator automatically.

## Available Tools

### `create`

Create local container registry in Kind cluster.

**Input Schema:**
```json
{
  "testID": "string (required)",     // Test environment ID
  "stage": "string (required)",      // Test stage name
  "tmpDir": "string (required)",     // Temporary directory for files
  "metadata": {                      // Metadata from testenv-kind
    "testenv-kind.kubeconfigPath": "string"
  }
}
```

**Output:**
```json
{
  "testID": "string",
  "files": {
    "testenv-lcr.ca.crt": "ca.crt",
    "testenv-lcr.credentials.yaml": "registry-credentials.yaml"
  },
  "metadata": {
    "testenv-lcr.registryFQDN": "testenv-lcr.testenv-lcr.svc.cluster.local:5000",
    "testenv-lcr.namespace": "testenv-lcr",
    "testenv-lcr.caCrtPath": "/abs/path/to/tmpDir/ca.crt",
    "testenv-lcr.credentialPath": "/abs/path/to/tmpDir/registry-credentials.yaml"
  },
  "managedResources": [
    "/abs/path/to/tmpDir/ca.crt",
    "/abs/path/to/tmpDir/registry-credentials.yaml"
  ]
}
```

**What It Does:**
1. Checks if registry is enabled in forge.yaml
2. Uses kubeconfig from testenv-kind metadata
3. Installs cert-manager via Helm
4. Creates self-signed certificate issuer
5. Generates TLS certificates for registry
6. Creates htpasswd credentials
7. Deploys registry:2 with TLS and auth
8. Exports CA cert and credentials to tmpDir
9. Updates /etc/hosts for registry FQDN

**Example:**
```json
{
  "method": "tools/call",
  "params": {
    "name": "create",
    "arguments": {
      "testID": "test-int-20250106-xyz789",
      "stage": "integration",
      "tmpDir": ".forge/tmp/test-int-20250106-xyz789",
      "metadata": {
        "testenv-kind.kubeconfigPath": ".forge/tmp/.../kubeconfig"
      }
    }
  }
}
```

### `delete`

Delete local container registry from Kind cluster.

**Input Schema:**
```json
{
  "testID": "string (required)",
  "metadata": {
    "testenv-kind.kubeconfigPath": "string"
  }
}
```

**Output:**
```json
{
  "success": true,
  "message": "Deleted local container registry"
}
```

**What It Does:**
1. Uses kubeconfig from metadata
2. Deletes Kubernetes namespace
3. Removes /etc/hosts entry
4. Best-effort cleanup (doesn't fail on errors)

## Integration

Called by testenv MCP server during test environment creation/deletion.

## Configuration

Reads from `forge.yaml`:
```yaml
localContainerRegistry:
  enabled: true
  namespace: testenv-lcr
  credentialPath: .forge/registry-credentials.yaml  # Overridden by tmpDir
  caCrtPath: .forge/ca.crt                          # Overridden by tmpDir
```

## Registry Details

- **Image**: registry:2
- **Port**: 5000 (HTTPS)
- **FQDN**: `testenv-lcr.testenv-lcr.svc.cluster.local:5000`
- **Auth**: htpasswd (random 32-char username/password)
- **TLS**: Self-signed via cert-manager
- **Storage**: emptyDir (ephemeral)

## Credential Format

Generated in tmpDir as `registry-credentials.yaml`:
```yaml
username: <random-32-chars>
password: <random-32-chars>
```

## Implementation Details

- Uses eventualconfig for setup phase coordination
- Runs setup phases concurrently where possible
- Waits for cert-manager and registry deployment readiness
- Manages certificates, secrets, configmaps, services, deployments

## See Also

- [testenv MCP Server](../testenv/MCP.md)
- [testenv-kind MCP Server](../testenv-kind/MCP.md)
- [Test Environment Architecture](../../docs/testenv-architecture.md)
