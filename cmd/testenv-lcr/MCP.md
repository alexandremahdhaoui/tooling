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
    "testenv-lcr.registryFQDN": "testenv-lcr.testenv-lcr.svc.cluster.local:31906",
    "testenv-lcr.namespace": "testenv-lcr",
    "testenv-lcr.port": "31906",
    "testenv-lcr.caCrtPath": "/abs/path/to/tmpDir/ca.crt",
    "testenv-lcr.credentialPath": "/abs/path/to/tmpDir/registry-credentials.yaml",
    "testenv-lcr.imagePullSecretCount": "2",
    "testenv-lcr.imagePullSecret.0.namespace": "default",
    "testenv-lcr.imagePullSecret.0.secretName": "local-container-registry-credentials",
    "testenv-lcr.imagePullSecret.1.namespace": "my-app",
    "testenv-lcr.imagePullSecret.1.secretName": "local-container-registry-credentials"
  },
  "env": {
    "TESTENV_LCR_FQDN": "testenv-lcr.testenv-lcr.svc.cluster.local:31906",
    "TESTENV_LCR_HOST": "testenv-lcr.testenv-lcr.svc.cluster.local",
    "TESTENV_LCR_PORT": "31906",
    "TESTENV_LCR_NAMESPACE": "testenv-lcr",
    "TESTENV_LCR_CA_CERT": "/abs/path/to/tmpDir/ca.crt"
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
10. Pushes images specified in spec.images (if configured)
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
        "testenv-lcr.registryFQDN": "testenv-lcr.testenv-lcr.svc.cluster.local:31906",
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

Reads its configuration from the `spec` of the testenv entry that names it, and
nothing from the top of `forge.yaml`:

```yaml
engines:
  - alias: setup-integration
    type: testenv
    testenv:
      - engine: "forge://testenv-kind"
      - engine: "forge://testenv-lcr"
        spec:
          enabled: true                       # Required: enable/disable the registry
          namespace: testenv-lcr              # Optional: defaults to "testenv-lcr"
          imagePullSecretNamespaces:          # Optional: namespaces for image pull secrets
            - default
            - my-app
          imagePullSecretName: local-container-registry-credentials  # Optional
```

The CA certificate and the registry credentials are written under the run's
tmpDir; the kubeconfig comes from the cluster engine before it.

**Configuration Fields:**

- `enabled` (boolean, required): Whether to create the local container registry
- `namespace` (string, optional, default: `"testenv-lcr"`): Kubernetes namespace for deployment
- `credentialPath` (string, optional): Path to store registry credentials (overridden by tmpDir in MCP mode)
- `caCrtPath` (string, optional): Path to store CA certificate (overridden by tmpDir in MCP mode)
- `imagePullSecretNamespaces` ([]string, optional): List of namespaces where image pull secrets should be created
- `imagePullSecretName` (string, optional, default: `"local-container-registry-credentials"`): Name of the image pull secret

**Override via Spec:**

All configuration fields can be overridden via the `spec` parameter in the testenv engine configuration:

```yaml
engines:
  - alias: my-testenv
    type: testenv
    testenv:
      - engine: forge://testenv-lcr
        spec:
          enabled: true
          namespace: custom-namespace  # Override default
          imagePullSecretNamespaces:
            - default
            - my-app-namespace
          images:  # Image configuration (see below)
            - name: local://myapp:latest
            - name: quay.io/example/img:v1.2.3
```

### Image Configuration

The `images` field allows explicit declaration of container images to push to the local registry:

#### Spec Fields

| Field | Type | Description |
|-------|------|-------------|
| images | []ImageSource | List of images to push to local registry |

#### ImageSource

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| name | string | Yes | Image reference (local://name:tag or registry/path:tag) |
| basicAuth | BasicAuth | No | Credentials for private registries |

#### BasicAuth

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| username | ValueFrom | Yes | Username credential |
| password | ValueFrom | Yes | Password credential |

#### ValueFrom

| Field | Type | Description |
|-------|------|-------------|
| envName | string | Environment variable name (mutually exclusive with literal) |
| literal | string | Direct literal value (mutually exclusive with envName) |

#### Image Name Formats

- **Local images**: `local://name:tag` - Image already exists in local Docker daemon
- **Remote images**: `registry/path:tag` - Will be pulled from remote registry
- Tags are **mandatory** (no :latest inference)

#### Example Configuration

```yaml
engines:
  - alias: testenv-integration
    type: testenv
    testenv:
      - engine: forge://testenv-lcr
        spec:
          images:
            - name: local://myapp:latest
            - name: quay.io/example/img:v1.2.3
              basicAuth:
                username:
                  envName: "QUAY_USER"
                password:
                  envName: "QUAY_PASS"
```

## Registry Details

- **Image**: registry:2
- **Port**: Dynamic (30000-32767 NodePort range, allocated per cluster)
- **FQDN**: `testenv-lcr.testenv-lcr.svc.cluster.local:<dynamic-port>`
- **Auth**: htpasswd (random 32-char username/password)
- **TLS**: Self-signed via cert-manager
- **Storage**: emptyDir (ephemeral)

## Environment Variables

testenv-lcr exports environment variables for use in subsequent testenv sub-engines via template expansion.

### Exported Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `TESTENV_LCR_FQDN` | Full registry address with port | `testenv-lcr.testenv-lcr.svc.cluster.local:31906` |
| `TESTENV_LCR_HOST` | Registry hostname (without port) | `testenv-lcr.testenv-lcr.svc.cluster.local` |
| `TESTENV_LCR_PORT` | Dynamic port number | `31906` |
| `TESTENV_LCR_NAMESPACE` | Kubernetes namespace | `testenv-lcr` |
| `TESTENV_LCR_CA_CERT` | Absolute path to CA certificate | `/abs/path/to/tmpDir/ca.crt` |

### Template Expansion

These variables can be referenced in subsequent testenv sub-engine specs using `{{.Env.VARIABLE_NAME}}` syntax:

```yaml
engines:
  - alias: setup-integration
    type: testenv
    testenv:
      - engine: forge://testenv-kind
      - engine: forge://testenv-lcr
        spec:
          enabled: true
          namespace: testenv-lcr
      - engine: forge://testenv-helm-install
        spec:
          charts:
            - name: my-app
              path: ./charts/my-app
              namespace: default
              values:
                image:
                  repository: "{{.Env.TESTENV_LCR_FQDN}}/my-app"
                  tag: latest
```

In the example above, `{{.Env.TESTENV_LCR_FQDN}}` is expanded to the actual registry address (e.g., `testenv-lcr.testenv-lcr.svc.cluster.local:31906`) before testenv-helm-install processes the chart.

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
- [Test Environment Architecture](../../docs/architecture/testenv-architecture.md)
