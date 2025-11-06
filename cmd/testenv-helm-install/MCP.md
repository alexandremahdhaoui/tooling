# testenv-helm-install MCP Server

MCP server for installing Helm charts into Kubernetes clusters for test environments.

## Purpose

Installs and manages Helm charts as part of test environment setup. Works with kubeconfig files provided by testenv-kind to deploy charts into test clusters.

## Invocation

```bash
testenv-helm-install --mcp
```

Called by testenv orchestrator automatically.

## Available Tools

### `create`

Install Helm charts into a Kubernetes cluster.

**Input Schema:**
```json
{
  "testID": "string (required)",     // Test environment ID
  "stage": "string (required)",      // Test stage name
  "tmpDir": "string (required)",     // Temporary directory for files
  "metadata": {                      // Metadata from previous testenv-subengines
    "testenv-kind.kubeconfigPath": "/path/to/kubeconfig"
  },
  "spec": {
    "charts": [                      // Array of charts to install
      {
        "name": "string (required)",           // Chart name
        "repo": "string (optional)",           // Helm repository URL
        "version": "string (optional)",        // Chart version
        "namespace": "string (optional)",      // K8s namespace
        "releaseName": "string (optional)",    // Custom release name
        "values": {                            // Helm values to override
          "key": "value"
        }
      }
    ]
  }
}
```

**Output:**
```json
{
  "testID": "string",
  "files": {},
  "metadata": {
    "testenv-helm-install.chartCount": "2",
    "testenv-helm-install.chart.0.name": "cert-manager",
    "testenv-helm-install.chart.0.releaseName": "cert-manager",
    "testenv-helm-install.chart.0.namespace": "cert-manager",
    "testenv-helm-install.chart.1.name": "nginx-ingress",
    "testenv-helm-install.chart.1.releaseName": "nginx-ingress"
  },
  "managedResources": []
}
```

**What It Does:**
1. Locates kubeconfig from metadata (provided by testenv-kind)
2. For each chart in spec.charts:
   - Adds Helm repository if specified
   - Runs `helm install` with provided configuration
   - Stores chart metadata for cleanup
3. Returns metadata with installed chart information

**Example:**
```json
{
  "method": "tools/call",
  "params": {
    "name": "create",
    "arguments": {
      "testID": "test-integration-20250106-abc123",
      "stage": "integration",
      "tmpDir": ".forge/tmp/test-integration-20250106-abc123",
      "metadata": {
        "testenv-kind.kubeconfigPath": "/tmp/kubeconfig"
      },
      "spec": {
        "charts": [
          {
            "name": "cert-manager",
            "repo": "https://charts.jetstack.io",
            "version": "v1.13.0",
            "namespace": "cert-manager",
            "values": {
              "installCRDs": "true"
            }
          }
        ]
      }
    }
  }
}
```

### `delete`

Uninstall Helm charts from a Kubernetes cluster.

**Input Schema:**
```json
{
  "testID": "string (required)",     // Test environment ID
  "metadata": {                       // Metadata from test environment
    "testenv-helm-install.chartCount": "2",
    "testenv-helm-install.chart.0.releaseName": "cert-manager",
    "testenv-helm-install.chart.0.namespace": "cert-manager",
    "testenv-kind.kubeconfigPath": "/path/to/kubeconfig"
  }
}
```

**Output:**
```json
{
  "success": true,
  "message": "Uninstalled 2 Helm chart(s)"
}
```

**What It Does:**
1. Extracts chart information from metadata
2. Uninstalls charts in reverse order (last installed, first removed)
3. Best-effort cleanup (logs warnings but continues on errors)

## Integration

Called by testenv MCP server during test environment creation/deletion. Must be positioned after testenv-kind in the testenv subengine list to ensure kubeconfig is available.

## Configuration

Example in `forge.yaml`:
```yaml
engines:
  - alias: k8s-with-helm
    type: testenv
    testenv:
      - engine: "go://testenv-kind"
      - engine: "go://testenv-lcr"
        spec:
          enabled: true
          autoPushImages: true
      - engine: "go://testenv-helm-install"
        spec:
          charts:
            - name: cert-manager
              repo: https://charts.jetstack.io
              version: v1.13.0
              namespace: cert-manager
              values:
                installCRDs: "true"
            - name: nginx-ingress
              repo: https://kubernetes.github.io/ingress-nginx
              namespace: ingress-nginx
```

## Implementation Details

- Uses `helm` CLI commands (requires helm to be installed)
- Finds kubeconfig from testenv-kind metadata
- Charts are installed sequentially in order
- Charts are uninstalled in reverse order during cleanup
- Supports custom release names, namespaces, and values
- Creates namespaces automatically if specified

## Requirements

- Helm CLI must be installed and available in PATH
- Kubeconfig must be provided by testenv-kind
- Charts must be accessible (public repos or pre-configured repos)

## See Also

- [testenv MCP Server](../testenv/MCP.md)
- [testenv-kind MCP Server](../testenv-kind/MCP.md)
- [testenv-lcr MCP Server](../testenv-lcr/MCP.md)
