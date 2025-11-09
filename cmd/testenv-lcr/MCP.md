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
    "testenv-lcr.credentialPath": "/abs/path/to/tmpDir/registry-credentials.yaml",
    "testenv-lcr.imagePullSecretCount": "2",
    "testenv-lcr.imagePullSecret.0.namespace": "default",
    "testenv-lcr.imagePullSecret.0.secretName": "local-container-registry-credentials",
    "testenv-lcr.imagePullSecret.1.namespace": "my-app",
    "testenv-lcr.imagePullSecret.1.secretName": "local-container-registry-credentials"
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
10. Auto-pushes images from artifact store (if autoPushImages: true)
11. Creates image pull secrets in configured namespaces (if imagePullSecretNamespaces specified)

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
2. Deletes image pull secrets in all namespaces
3. Deletes Kubernetes namespace
4. Removes /etc/hosts entry
5. Best-effort cleanup (doesn't fail on errors)

### `create-image-pull-secret`

Create an image pull secret in a specific namespace for the local container registry.

**Input Schema:**
```json
{
  "testID": "string (required)",     // Test environment ID
  "namespace": "string (required)",  // Kubernetes namespace for secret
  "secretName": "string (optional)", // Secret name (defaults to config or "local-container-registry-credentials")
  "metadata": {                      // Metadata from testenv
    "testenv-kind.kubeconfigPath": "string",
    "testenv-lcr.registryFQDN": "string",
    "testenv-lcr.caCrtPath": "string",
    "testenv-lcr.credentialPath": "string"
  }
}
```

**Output:**
```json
{
  "success": true,
  "message": "Created image pull secret: namespace/secret-name"
}
```

**What It Does:**
1. Validates inputs (testID and namespace required)
2. Reads registry credentials from file
3. Reads CA certificate
4. Creates namespace if it doesn't exist
5. Generates .dockerconfigjson with registry auth
6. Creates Kubernetes secret with type kubernetes.io/dockerconfigjson
7. Labels secret with app.kubernetes.io/managed-by=testenv-lcr

**Example:**
```json
{
  "method": "tools/call",
  "params": {
    "name": "create-image-pull-secret",
    "arguments": {
      "testID": "test-int-20250106-xyz789",
      "namespace": "my-app",
      "metadata": {
        "testenv-lcr.registryFQDN": "testenv-lcr.testenv-lcr.svc.cluster.local:5000",
        "testenv-lcr.caCrtPath": ".forge/tmp/.../ca.crt",
        "testenv-lcr.credentialPath": ".forge/tmp/.../registry-credentials.yaml"
      }
    }
  }
}
```

### `list-image-pull-secrets`

List all image pull secrets created by testenv-lcr across all namespaces or in a specific namespace.

**Input Schema:**
```json
{
  "testID": "string (required)",      // Test environment ID
  "namespace": "string (optional)",   // Optional namespace filter
  "metadata": {                       // Metadata from testenv
    "testenv-kind.kubeconfigPath": "string"
  }
}
```

**Output:**
```json
{
  "testID": "string",
  "secrets": [
    {
      "namespace": "default",
      "secretName": "local-container-registry-credentials",
      "createdAt": "2025-01-06T10:30:00Z"
    },
    {
      "namespace": "test-podinfo",
      "secretName": "local-container-registry-credentials",
      "createdAt": "2025-01-06T10:30:00Z"
    }
  ],
  "count": 2
}
```

**What It Does:**
1. Lists all secrets with label app.kubernetes.io/managed-by=testenv-lcr
2. Filters by namespace if provided
3. Returns secret information (namespace, name, creation time)

**Example:**
```json
{
  "method": "tools/call",
  "params": {
    "name": "list-image-pull-secrets",
    "arguments": {
      "testID": "test-int-20250106-xyz789",
      "namespace": "default",
      "metadata": {
        "testenv-kind.kubeconfigPath": ".forge/tmp/.../kubeconfig"
      }
    }
  }
}
```

## Integration

Called by testenv MCP server during test environment creation/deletion.

## Configuration

Reads configuration from the root-level `localContainerRegistry` section in `forge.yaml`:

```yaml
localContainerRegistry:
  enabled: true                                     # Required: enable/disable the registry
  namespace: testenv-lcr                            # Optional: defaults to "testenv-lcr"
  credentialPath: .forge/registry-credentials.yaml  # Optional: overridden by tmpDir
  caCrtPath: .forge/ca.crt                          # Optional: overridden by tmpDir
  autoPushImages: true                              # Optional: defaults to false
  imagePullSecretNamespaces:                        # Optional: list of namespaces for image pull secrets
    - default
    - my-app
  imagePullSecretName: local-container-registry-credentials  # Optional: defaults to this value
```

**Configuration Fields:**

- `enabled` (boolean, required): Whether to create the local container registry
- `namespace` (string, optional, default: `"testenv-lcr"`): Kubernetes namespace for deployment
- `credentialPath` (string, optional): Path to store registry credentials (overridden by tmpDir in MCP mode)
- `caCrtPath` (string, optional): Path to store CA certificate (overridden by tmpDir in MCP mode)
- `autoPushImages` (boolean, optional, default: `false`): Automatically push images from artifact store on setup
- `imagePullSecretNamespaces` ([]string, optional): List of namespaces where image pull secrets should be created
- `imagePullSecretName` (string, optional, default: `"local-container-registry-credentials"`): Name of the image pull secret

**Override via Spec:**

All configuration fields can be overridden via the `spec` parameter in the testenv engine configuration:

```yaml
engines:
  - alias: my-testenv
    type: testenv
    testenv:
      - engine: go://testenv-lcr
        spec:
          enabled: true
          namespace: custom-namespace  # Override default
          autoPushImages: true
          imagePullSecretNamespaces:
            - default
            - my-app-namespace
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
