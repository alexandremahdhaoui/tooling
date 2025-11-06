# Testenv Quick Start Guide

Get started with composable test environments in 5 minutes.

## Prerequisites

```bash
# Required tools
- Go 1.21+
- kind (Kubernetes in Docker)
- kubectl
- Docker or Podman

# Optional
- Helm (for future testenv-helm-install)
```

## Installation

### 1. Build Forge and Components

```bash
# Navigate to project root
cd /path/to/forge

# Build all binaries
go build -o ./build/bin/forge ./cmd/forge
go build -o ./build/bin/testenv ./cmd/testenv
go build -o ./build/bin/testenv-kind ./cmd/testenv-kind
go build -o ./build/bin/testenv-lcr ./cmd/testenv-lcr
go build -o ./build/bin/test-runner-go ./cmd/test-runner-go

# Verify
./build/bin/forge --version
./build/bin/testenv --version
./build/bin/testenv-kind --version
./build/bin/testenv-lcr --version
```

### 2. Configure forge.yaml

Create or update `forge.yaml` in your project root:

```yaml
name: my-project

# Build configuration
build:
  artifactStorePath: .forge/artifacts.yaml
  specs:
    - name: my-app
      src: ./cmd/my-app
      dest: ./build/bin
      builder: go://build-go

# Define test stages
test:
  # Unit tests - no environment needed
  - name: unit
    engine: "noop"
    runner: "go://test-runner-go"

  # Integration tests - full test environment with Kind cluster and registry
  - name: integration
    engine: "go://testenv"
    runner: "go://test-runner-go"

  # E2E tests - same environment as integration
  - name: e2e
    engine: "go://testenv"
    runner: "go://test-runner-go"
```

**Note**: The `testenv` engine automatically orchestrates `testenv-kind` (Kind cluster) and `testenv-lcr` (local container registry with TLS) when creating the test environment.

### 3. Set Environment Variables

```bash
# Required for testenv-kind
export KIND_BINARY="kind"

# Optional: Use sudo if needed
export KIND_BINARY_PREFIX=""  # or "sudo"

# Required for testenv-lcr
export CONTAINER_ENGINE="docker"  # or "podman"
export PREPEND_CMD=""  # or "sudo" if needed
```

## Usage

### Create Test Environment

```bash
# Create a new test environment for integration tests
./build/bin/forge test integration create

# Output: integration-20250106-143000 (JSON format)

# View all test environments
./build/bin/forge test integration list
```

**What happens**:
1. testenv creates tmpDir: `/tmp/forge-test-integration-20250106-abc123/`
2. testenv-kind creates Kubernetes cluster
3. testenv-lcr deploys container registry
4. Files and metadata stored in artifact store

### List Test Environments

```bash
# List all environments for integration stage
./build/bin/forge test integration list

# Sample output:
# === Test Environments ===
# ID                                      NAME            STATUS     CREATED
# -----------------------------------------------------------------------------------------
# test-integration-20250106-abc123        integration     created    2025-01-06T10:30:00Z
```

### Get Environment Details

```bash
# Get full details of specific environment
./build/bin/forge test integration get test-integration-20250106-abc123

# Output (example):
# === Test Environment ===
# ID:          test-integration-20250106-abc123
# Name:        integration
# Status:      created
# Created:     2025-01-06T10:30:00Z
# Updated:     2025-01-06T10:30:00Z
# TmpDir:      /tmp/forge-test-integration-20250106-abc123
#
# Files:
#   testenv-kind.kubeconfig: kubeconfig
#   testenv-lcr.ca.crt: ca.crt
#   testenv-lcr.credentials.yaml: registry-credentials.yaml
#
# Metadata:
#   testenv-kind.clusterName: forge-test-integration-20250106-abc123
#   testenv-kind.kubeconfigPath: /tmp/forge-test-integration-20250106-abc123/kubeconfig
#   testenv-lcr.registryFQDN: testenv-lcr.local-container-registry.svc.cluster.local:5000
```

### Run Tests

```bash
# Run tests using the environment
./build/bin/forge test integration run test-integration-20250106-abc123

# Or auto-create environment and run
./build/bin/forge test integration run
```

**What happens**:
1. forge reads TestEnvironment from artifact store
2. Extracts artifactFiles (kubeconfig, credentials, etc.)
3. Passes files to test-runner-go via MCP
4. Tests execute with full environment access

### Delete Test Environment

```bash
# Clean up environment when done
./build/bin/forge test integration delete test-integration-20250106-abc123

# Output: Deleted test environment: test-integration-20250106-abc123
```

**What happens**:
1. testenv-lcr removes container registry (reverse order)
2. testenv-kind deletes Kubernetes cluster
3. tmpDir and all managed resources cleaned up
4. TestEnvironment removed from artifact store

## Examples

### Example 1: Basic Integration Test Workflow

```bash
# 1. Create environment
testID=$(./build/bin/forge test integration create)

# 2. Inspect what was created
ls -la /tmp/forge-test-integration-*/
# kubeconfig  ca.crt  registry-credentials.yaml

# 3. Manually test cluster access
export KUBECONFIG=/tmp/forge-test-integration-$testID/kubeconfig
kubectl get nodes
kubectl get pods -n local-container-registry

# 4. Run tests
./build/bin/forge test integration run $testID

# 5. Clean up
./build/bin/forge test integration delete $testID
```

### Example 2: Multiple Test Environments

```bash
# Create multiple isolated environments
testID1=$(./build/bin/forge test integration create)
testID2=$(./build/bin/forge test integration create)
testID3=$(./build/bin/forge test integration create)

# List all
./build/bin/forge test integration list

# Run tests in parallel (different terminals)
./build/bin/forge test integration run $testID1 &
./build/bin/forge test integration run $testID2 &
./build/bin/forge test integration run $testID3 &
wait

# Clean up all
./build/bin/forge test integration delete $testID1
./build/bin/forge test integration delete $testID2
./build/bin/forge test integration delete $testID3
```

### Example 3: Debug Subengine Directly

```bash
# Test kind cluster alone (without orchestration)
./build/bin/testenv-kind setup
# Creates cluster: forge-{project-name}

# Verify
kind get clusters
export KUBECONFIG=.forge/<test-id>/kubeconfig.yaml
kubectl get nodes

# Clean up
./build/bin/testenv-kind teardown

# Test registry alone (requires existing cluster)
./build/bin/testenv-lcr setup
kubectl get pods -n local-container-registry

# Push image
docker pull nginx:latest
./build/bin/testenv-lcr push nginx:latest

# Clean up
./build/bin/testenv-lcr teardown
```

### Example 4: Multiple Test Stages

```yaml
# forge.yaml - Different test stages

test:
  # Unit tests: no environment
  - name: unit
    engine: "noop"
    runner: "go://test-runner-go"

  # Integration tests: full environment (Kind + registry)
  - name: integration
    engine: "go://testenv"
    runner: "go://test-runner-go"

  # E2E tests: same full environment
  - name: e2e
    engine: "go://testenv"
    runner: "go://test-runner-go"

  # Linting: no environment
  - name: lint
    engine: "noop"
    runner: "go://lint-go"
```

**Note**: The `testenv` engine automatically composes `testenv-kind` and `testenv-lcr` subengines.

## Debugging

### Check Component Versions

```bash
./build/bin/forge --version
./build/bin/testenv --version
./build/bin/testenv-kind --version
./build/bin/testenv-lcr --version
```

### Inspect Artifact Store

```bash
# View artifact store contents
cat .forge/artifacts.yaml

# Or use yq for better formatting
yq eval . .forge/artifacts.yaml
```

### Check tmpDir Contents

```bash
# List all test environment directories
ls -la /tmp/forge-test-*/

# Inspect specific environment
testID="test-integration-20250106-abc123"
tree /tmp/forge-test-$testID/
```

### Verify Kubernetes Cluster

```bash
# List kind clusters
kind get clusters

# Should see: forge-test-integration-20250106-abc123

# Check cluster
export KUBECONFIG=/tmp/forge-test-.../kubeconfig
kubectl cluster-info
kubectl get pods -A
```

### Verify Container Registry

```bash
# Check registry pod
kubectl get pods -n local-container-registry

# Test registry connectivity
curl https://testenv-lcr.local-container-registry.svc.cluster.local:5000/v2/_catalog \
  --cacert /tmp/forge-test-.../ca.crt
```

### Enable Debug Logging

```bash
# Capture all output (including stderr from MCP servers)
./build/bin/forge test integration create 2>&1 | tee debug.log

# Check for errors
grep -i error debug.log
```

## Tips & Best Practices

### 1. Reuse Test Environment Across Stages

```yaml
# Use same engine for multiple stages
test:
  - name: integration
    engine: "go://testenv"
    runner: "go://test-runner-go"
  - name: e2e
    engine: "go://testenv"
    runner: "go://test-runner-go"
```

### 2. Clean Up Old Environments

```bash
# List environments
./build/bin/forge test integration list

# Delete old ones
for testID in $(./build/bin/forge test integration list -o json | jq -r '.[].id'); do
  ./build/bin/forge test integration delete $testID
done
```

### 3. Use tmpDir for Test Artifacts

```bash
# Tests can write to tmpDir
echo "test-output" > $FORGE_TMP_DIR/results.txt

# Files automatically cleaned up on delete
```

### 4. Check Status Before Cleanup

```bash
# Get current status
status=$(./build/bin/forge test integration get $testID -o json | jq -r '.status')

# Only delete if passed/failed (not running)
if [ "$status" != "running" ]; then
  ./build/bin/forge test integration delete $testID
fi
```

### 5. Use Make for Automation

```makefile
# Makefile

.PHONY: test-integration-create
test-integration-create:
	@./build/bin/forge test integration create

.PHONY: test-integration-run
test-integration-run:
	@testID=$$(./build/bin/forge test integration create) && \
	./build/bin/forge test integration run $$testID && \
	./build/bin/forge test integration delete $$testID

.PHONY: test-integration-cleanup
test-integration-cleanup:
	@for id in $$(./build/bin/forge test integration list -o json | jq -r '.[].id'); do \
		./build/bin/forge test integration delete $$id; \
	done
```

## Troubleshooting

### "binary not found: ./build/bin/testenv-kind"

**Solution**: Build all binaries
```bash
go build -o ./build/bin/testenv-kind ./cmd/testenv-kind
go build -o ./build/bin/testenv-lcr ./cmd/testenv-lcr
```

### "kind cluster already exists"

**Solution**: Delete existing cluster
```bash
kind delete cluster --name forge-{project-name}
```

### "failed to connect to MCP server"

**Solution**: Verify binary has execute permissions
```bash
chmod +x ./build/bin/*
```

### "test environment not found"

**Solution**: Check artifact store
```bash
cat .forge/artifacts.yaml
./build/bin/forge test integration list
```

## Next Steps

1. **Read Architecture**: [testenv-architecture.md](./testenv-architecture.md)
2. **Forge CLI Usage**: [forge-usage.md](./forge-usage.md)
3. **Create Custom Test Engine**: [docs/prompts/create-test-engine.md](./prompts/create-test-engine.md)
4. **Create Custom Test Runner**: [docs/prompts/create-test-runner.md](./prompts/create-test-runner.md)
5. **CI/CD Integration**: Automate test environment management

## Quick Reference

```bash
# Create
./build/bin/forge test <stage> create

# List
./build/bin/forge test <stage> list

# Get
./build/bin/forge test <stage> get <testID>

# Run
./build/bin/forge test <stage> run [testID]

# Delete
./build/bin/forge test <stage> delete <testID>

# Debug subengines directly
./build/bin/testenv-kind setup|teardown
./build/bin/testenv-lcr setup|teardown|push|push-all
```

## Resources

- **Architecture**: [testenv-architecture.md](./testenv-architecture.md)
- **Forge Usage**: [forge-usage.md](./forge-usage.md)
- **Test Usage**: [forge-test-usage.md](./forge-test-usage.md)
- **MCP Protocol**: https://modelcontextprotocol.io/
- **Kind**: https://kind.sigs.k8s.io/

Happy testing! 🚀
