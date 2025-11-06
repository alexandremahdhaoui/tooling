# Testenv Architecture

This document describes the testenv architecture for composable test environment management in Forge.

## Overview

The testenv system provides a composable, extensible way to create and manage test environments. It replaces the monolithic `test-integration` approach with a modular architecture based on:

- **testenv**: Orchestration engine that composes testenv-subengines
- **testenv-subengines**: Independent components (testenv-kind, testenv-lcr, etc.)
- **MCP Protocol**: Communication between components
- **Artifact Store**: Centralized storage for test environment metadata

## Core Components

### 1. testenv (Orchestrator)

**Purpose**: Orchestrate multiple testenv-subengines to create/delete test environments.

**Responsibilities**:
- Create tmpDir for file isolation (`/tmp/forge-test-{stage}-{testID}/`)
- Call testenv-subengines in order (create) or reverse order (delete)
- Aggregate files, metadata, and managed resources
- Store test environment in artifact store

**MCP Tools**:
- `create`: Create test environment by orchestrating subengines
- `delete`: Delete test environment and cleanup resources

**Does NOT provide**: get/list operations (handled by forge directly)

**Example Usage**:
```bash
# Via MCP
./build/bin/testenv --mcp

# CLI (for debugging)
./build/bin/testenv create integration
./build/bin/testenv delete test-integration-20250106-abc123
```

### 2. testenv-kind (Subengine)

**Purpose**: Create/delete Kubernetes clusters using kind.

**Responsibilities**:
- Create kind cluster with unique name
- Generate kubeconfig in tmpDir
- Return kubeconfig path and cluster metadata
- Delete cluster and cleanup kubeconfig

**MCP Tools**:
- `create`: Create kind cluster
- `delete`: Delete kind cluster

**CLI Commands** (for debugging):
```bash
./build/bin/testenv-kind setup
./build/bin/testenv-kind teardown
./build/bin/testenv-kind --version
```

**MCP Response** (create):
```json
{
  "testID": "test-integration-20250106-abc123",
  "files": {
    "testenv-kind.kubeconfig": "kubeconfig"
  },
  "metadata": {
    "testenv-kind.clusterName": "forge-test-integration-20250106-abc123",
    "testenv-kind.kubeconfigPath": "/tmp/forge-test-integration-20250106-abc123/kubeconfig"
  },
  "managedResources": [
    "/tmp/forge-test-integration-20250106-abc123/kubeconfig"
  ]
}
```

### 3. testenv-lcr (Subengine)

**Purpose**: Deploy local container registry in Kubernetes cluster.

**Responsibilities**:
- Deploy registry to existing kind cluster (from testenv-kind)
- Generate registry credentials and TLS certificates
- Configure /etc/hosts entry
- Return registry FQDN and credential paths
- Cleanup registry deployment

**MCP Tools**:
- `create`: Deploy container registry
- `delete`: Remove container registry

**CLI Commands** (for debugging):
```bash
./build/bin/testenv-lcr setup
./build/bin/testenv-lcr teardown
./build/bin/testenv-lcr push <image>
./build/bin/testenv-lcr push-all
```

**MCP Response** (create):
```json
{
  "testID": "test-integration-20250106-abc123",
  "files": {
    "testenv-lcr.ca.crt": "ca.crt",
    "testenv-lcr.credentials.yaml": "registry-credentials.yaml"
  },
  "metadata": {
    "testenv-lcr.registryFQDN": "testenv-lcr.local-container-registry.svc.cluster.local:5000",
    "testenv-lcr.namespace": "local-container-registry",
    "testenv-lcr.caCrtPath": "/tmp/forge-test-integration-20250106-abc123/ca.crt",
    "testenv-lcr.credentialPath": "/tmp/forge-test-integration-20250106-abc123/registry-credentials.yaml"
  },
  "managedResources": [
    "/tmp/forge-test-integration-20250106-abc123/ca.crt",
    "/tmp/forge-test-integration-20250106-abc123/registry-credentials.yaml"
  ]
}
```

### 4. forge (Coordinator)

**Purpose**: High-level test orchestration and artifact store management.

**Responsibilities**:
- Read forge.yaml configuration
- Resolve engine aliases
- Call testenv for create/delete
- Read artifact store DIRECTLY for get/list (NO MCP)
- Pass artifactFiles to test-runner

**Commands**:
```bash
# Test environment management (calls testenv via MCP)
forge test <stage> create          # Create test environment
forge test <stage> delete <testID> # Delete test environment

# Artifact store queries (direct access, no MCP)
forge test <stage> get <testID>    # Get test environment details
forge test <stage> list            # List test environments

# Test execution (passes artifactFiles to test-runner)
forge test <stage> run [testID]    # Run tests
```

## Data Flow

### Create Flow

```
forge test integration create
  ↓
forge reads forge.yaml → finds testenv: "alias://setup-integration"
  ↓
forge calls: testenv create (stage="integration")
  ↓
testenv creates tmpDir: /tmp/forge-test-integration-20250106-abc123/
  ↓
testenv orchestrates subengines:
  1. testenv-kind create
     Input: {testID, stage, tmpDir, metadata: {}}
     Output: {files: {kubeconfig}, metadata: {clusterName, kubeconfigPath}}
  ↓
  2. testenv-lcr create
     Input: {testID, stage, tmpDir, metadata: {from testenv-kind}}
     Output: {files: {ca.crt, credentials}, metadata: {registryFQDN, ...}}
  ↓
testenv aggregates results → writes TestEnvironment to artifact store
  ↓
Returns testID: "test-integration-20250106-abc123"
```

### Delete Flow

```
forge test integration delete test-integration-20250106-abc123
  ↓
forge calls: testenv delete (testID="test-integration-20250106-abc123")
  ↓
testenv reads TestEnvironment from artifact store
  ↓
testenv orchestrates subengines in REVERSE order:
  1. testenv-lcr delete (best effort)
     Input: {testID, metadata}
  ↓
  2. testenv-kind delete (best effort)
     Input: {testID, metadata}
  ↓
testenv removes tmpDir and managedResources
  ↓
testenv removes TestEnvironment from artifact store
```

### Run Flow

```
forge test integration run test-integration-20250106-abc123
  ↓
forge reads TestEnvironment from artifact store
  ↓
forge extracts artifactFiles:
  {
    "testenv-kind.kubeconfig": "/tmp/forge-test-.../kubeconfig",
    "testenv-lcr.ca.crt": "/tmp/forge-test-.../ca.crt",
    "testenv-lcr.credentials.yaml": "/tmp/forge-test-.../registry-credentials.yaml"
  }
  ↓
forge calls test-runner-go via MCP:
  {
    id, stage, name,
    artifactFiles: {...}  ← Tests can access these files
  }
  ↓
test-runner-go executes tests with full environment access
```

## Configuration

### forge.yaml

```yaml
# Define engine aliases with type
engines:
  - alias: setup-integration
    type: testenv
    testenv:
      - engine: "go://testenv-kind"
        # spec: optional subengine-specific config
      - engine: "go://testenv-lcr"
        spec:
          autoPushImages: true
          enabled: true

# Reference alias in test stages
test:
  - name: integration
    runner: "go://test-runner-go"
    testenv: "alias://setup-integration"  # Uses the alias
```

## Artifact Store

### TestEnvironment Structure

```go
type TestEnvironment struct {
    ID               string            // Unique test ID
    Name             string            // Test stage name
    Status           string            // created, running, passed, failed
    CreatedAt        time.Time         // Creation timestamp
    UpdatedAt        time.Time         // Last update timestamp
    TmpDir           string            // Temporary directory path
    Files            map[string]string // Namespaced file paths
    ManagedResources []string          // Resources to cleanup
    Metadata         map[string]string // Namespaced metadata
}
```

### Namespacing Convention

All keys in `Files` and `Metadata` are prefixed with the engine name:

- Files: `"testenv-kind.kubeconfig"`, `"testenv-lcr.ca.crt"`
- Metadata: `"testenv-kind.clusterName"`, `"testenv-lcr.registryFQDN"`

This prevents collisions and makes ownership clear.

## Extensibility

### Adding a New testenv-subengine

Example: testenv-helm-install

1. **Create the component**:
   ```bash
   mkdir cmd/testenv-helm-install
   ```

2. **Implement MCP server**:
   ```go
   // cmd/testenv-helm-install/mcp.go
   type CreateInput struct {
       TestID   string            `json:"testID"`
       Stage    string            `json:"stage"`
       TmpDir   string            `json:"tmpDir"`
       Metadata map[string]string `json:"metadata"`
       Spec     map[string]any    `json:"spec,omitempty"`
   }

   func handleCreateTool(...) {
       // 1. Get kubeconfig from metadata["testenv-kind.kubeconfigPath"]
       // 2. Install Helm chart
       // 3. Return files/metadata
   }
   ```

3. **Add CLI commands** (for debugging):
   ```go
   // cmd/testenv-helm-install/main.go
   switch os.Args[1] {
   case "--mcp":
       runMCPServer()
   case "install":
       installChart()
   case "uninstall":
       uninstallChart()
   }
   ```

4. **Update forge.yaml**:
   ```yaml
   engines:
     - alias: k8s-with-helm
       type: testenv
       testenv:
         - engine: "go://testenv-kind"
         - engine: "go://testenv-lcr"
         - engine: "go://testenv-helm-install"
           spec:
             chart: "my-chart"
             values: "values.yaml"
   ```

## Key Design Principles

### 1. Separation of Concerns
- **testenv**: ONLY orchestration (create/delete)
- **forge**: Queries (get/list) and coordination
- **testenv-subengines**: Specific infrastructure setup

### 2. Composability
- Mix and match testenv-subengines
- Order matters (kind before lcr)
- Metadata flows between subengines

### 3. File Isolation
- Each test environment has unique tmpDir
- Files written with relative paths
- Absolute paths passed to test-runner

### 4. Debuggability
- All subengines have CLI commands
- Direct binary execution for troubleshooting
- Stderr forwarded from MCP servers

### 5. Best-Effort Cleanup
- Delete operations continue on errors
- Logs warnings but doesn't fail
- Ensures partial cleanup doesn't block

## Migration from test-integration

### Old Approach
```yaml
# Old forge.yaml
test:
  - name: integration
    runner: "go://test-runner-go"
    setup: "go://test-integration"
```

### New Approach
```yaml
# New forge.yaml
engines:
  - alias: setup-integration
    type: testenv
    testenv:
      - engine: "go://testenv-kind"
      - engine: "go://testenv-lcr"

test:
  - name: integration
    runner: "go://test-runner-go"
    testenv: "alias://setup-integration"
```

### Breaking Changes
1. Field rename: `setup` → `testenv`
2. Component renames:
   - `test-integration` → `testenv`
   - `kindenv` → `testenv-kind`
   - `local-container-registry` → `testenv-lcr`
3. testenv no longer provides get/list (forge handles these)
4. TestEnvironment structure changes (removed deprecated fields)

## Troubleshooting

### Check test environment status
```bash
forge test integration list
forge test integration get <testID>
```

### Debug subengine directly
```bash
# Test kind cluster creation
./build/bin/testenv-kind setup
./build/bin/testenv-kind teardown

# Test registry deployment
./build/bin/testenv-lcr setup
./build/bin/testenv-lcr teardown
```

### Inspect artifact store
```bash
cat .ignore.artifact-store.yaml
```

### View tmpDir contents
```bash
ls -la /tmp/forge-test-integration-*/
```

### Check MCP communication
```bash
# Enable MCP logging (stderr)
forge test integration create 2>&1 | grep -i error
```

## Performance Considerations

### Parallel Subengine Execution
Currently, testenv-subengines are called sequentially because:
- Metadata dependencies (lcr needs kind's kubeconfig)
- Order matters for setup/teardown

Future optimization: Parallel execution where dependencies allow.

### Artifact Store Caching
forge reads artifact store directly (no MCP overhead) for queries.

### tmpDir Cleanup
- Automatic cleanup on delete
- Old tmpDirs cleaned on new runs (keeps last 10)

## Security Considerations

### Credentials
- Stored in tmpDir (restricted permissions: 0755)
- Cleaned up on delete
- Never logged or exposed

### Network Isolation
- Each test environment is isolated
- Kind cluster has unique name
- Registry has unique namespace

### Resource Limits
- No automatic limits (user responsibility)
- Consider quotas for CI/CD environments

## Future Enhancements

1. **Parallel Subengine Execution**: Where dependencies allow
2. **Health Checks**: Verify subengine setup before continuing
3. **Rollback on Failure**: Automatic cleanup if any subengine fails
4. **Resource Quotas**: Limit cluster resources, disk usage
5. **Shared Environments**: Reuse environments across multiple test runs
6. **Remote Backends**: Store artifact store in database instead of file

## References

- [MCP Protocol](https://modelcontextprotocol.io/)
- [Kind Documentation](https://kind.sigs.k8s.io/)
- [Forge Architecture](./architecture.md)
