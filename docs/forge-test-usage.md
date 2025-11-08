# Forge Test Command Usage Guide

## Overview

The `forge test` command provides a unified interface for managing test environments and executing tests across different stages (unit, integration, e2e, etc.). It replaces the hardcoded `forge integration` command with a flexible, configuration-driven approach.

**MCP Integration:** All components in the test system (forge CLI, test engines, test runners) are MCP servers, allowing AI coding agents to programmatically create test environments, run tests, and retrieve results.

## Quick Start

### 1. Configure forge.yaml

```yaml
name: my-project
artifactStorePath: .forge/artifacts.json

test:
  # Unit tests - no environment needed
  - name: unit
    engine: "noop"
    runner: "go://test-runner-go"

  # Integration tests - requires test environment
  - name: integration
    engine: "go://testenv"
    runner: "go://test-runner-go"

  # E2E tests - custom environment
  - name: e2e
    engine: "go://my-e2e-engine"
    runner: "go://test-runner-go"
```

### 2. Run Tests

```bash
# Run unit tests (no environment needed)
forge test unit run

# Run integration tests (auto-creates environment)
forge test integration run

# Create environment, run tests, keep environment
TEST_ID=$(forge test integration create)
forge test integration run $TEST_ID
forge test integration delete $TEST_ID
```

## Commands

### `forge test <stage> create`

Creates a test environment for the specified stage.

**Usage:**
```bash
forge test <stage> create
```

**Example:**
```bash
TEST_ID=$(forge test integration create)
echo "Created: $TEST_ID"
# Output: test-integration-20241103-abc12345
```

**Behavior:**
- Calls the test engine specified in forge.yaml
- Generates a unique test environment ID
- Stores environment in `.forge/artifacts.json`
- Returns the test ID to stdout

**Errors:**
- Stage not found in forge.yaml
- Engine is "noop" (no environment management)
- Engine execution failed

### `forge test <stage> get <test-id>`

Retrieves and displays test environment details.

**Usage:**
```bash
forge test <stage> get <TEST-ID>
```

**Example:**
```bash
forge test integration get test-integration-20241103-abc12345
```

**Output:**
```json
{
  "testID": "integration-20241103-143000",
  "createdAt": "2024-11-03T14:30:00Z",
  "files": {
    "kubeconfig": ".forge/integration-20241103-143000/kubeconfig.yaml",
    "ca.crt": ".forge/integration-20241103-143000/ca.crt",
    "credentials.yaml": ".forge/integration-20241103-143000/credentials.yaml"
  },
  "metadata": {
    "clusterName": "test-integration-20241103-143000",
    "registryURL": "registry.local:5000"
  }
}
```

**Status Values:**
- `created` - Environment created but not used
- `running` - Tests currently executing
- `passed` - Tests completed successfully
- `failed` - Tests failed
- `partially_deleted` - Cleanup incomplete

### `forge test <stage> delete <test-id>`

Deletes a test environment and cleans up all resources.

**Usage:**
```bash
forge test <stage> delete <TEST-ID>
```

**Example:**
```bash
forge test integration delete test-integration-20241103-abc12345
# Output: Deleted test environment: test-integration-20241103-abc12345
```

**Cleanup:**
- Calls engine delete operation
- Removes Kubernetes clusters (if applicable)
- Cleans up registry resources (if applicable)
- Deletes managed files and directories
- Removes environment from artifact store

### `forge test <stage> list`

Lists all test environments for a stage.

**Usage:**
```bash
forge test <stage> list
```

**Example:**
```bash
forge test integration list
```

**Output:**
```
Test environments for stage: integration

ID                                        STATUS      CREATED
----------------------------------------  ----------  --------------------
test-integration-20241103-abc12345        passed      2024-11-03 14:30
test-integration-20241103-def67890        created     2024-11-03 15:45
```

### `forge test <stage> run [test-id]`

Runs tests for the specified stage.

**Usage:**
```bash
# Auto-create environment and run
forge test <stage> run

# Run in existing environment
forge test <stage> run <TEST-ID>
```

**Examples:**
```bash
# Run unit tests (no environment)
forge test unit run

# Run integration tests (auto-creates environment)
forge test integration run

# Run in specific environment
forge test integration run test-integration-20241103-abc12345
```

**Behavior:**
1. Creates test environment (if not provided and engine != "noop")
2. Calls test runner with stage and unique name
3. Updates environment status based on results
4. Displays test report summary

**Output:**
```
Running tests: stage=integration, name=integration-20241103-143000

Test Results:
Status: passed
Total: 150
Passed: 148
Failed: 2
Coverage: 85.3%
```

**Exit Codes:**
- 0: Tests passed
- Non-zero: Tests failed

## Configuration

### Test Stages

Define test stages in `forge.yaml`:

```yaml
test:
  - name: <stage-name>
    engine: <engine-uri>
    runner: <runner-uri>
```

**Fields:**
- `name`: Stage identifier (e.g., "unit", "integration", "e2e")
- `engine`: Test environment manager
  - `"noop"` or `""` - No environment management
  - `"go://<package>"` - Go package implementing test engine
  - `"go://<package>@<version>"` - Specific version
- `runner`: Test executor
  - `"go://<package>"` - Go package implementing test runner
  - `"shell://bash <script>"` - Shell script runner

### Engine URIs

Engines use the `go://` protocol:

```yaml
# Short name (expands to github.com/alexandremahdhaoui/forge/cmd/...)
engine: "go://testenv"

# With version
engine: "go://testenv@v1.0.0"

# Full package path
engine: "go://github.com/myorg/my-engine"

# Full path with version
engine: "go://github.com/myorg/my-engine@v2.0.0"
```

**Auto-installation:**
- Forge checks `./build/bin/<binary>` first
- Then checks PATH
- If not found, runs `go install <package>@<version>`
- Verifies version compatibility for forge tools

### Artifact Store

The artifact store (`.forge/artifacts.json`) contains:
- Build artifacts
- Test environments
- Test results metadata

**Structure:**
```json
{
  "version": "1.0",
  "lastUpdated": "2024-11-03T20:40:16Z",
  "artifacts": [
    {
      "name": "myapp",
      "type": "binary",
      "location": "./build/bin/myapp",
      "timestamp": "2024-11-03T20:30:00Z",
      "version": "abc123"
    }
  ],
  "testEnvironments": {
    "test-integration-20241103-abc12345": {
      "id": "test-integration-20241103-abc12345",
      "name": "integration",
      "status": "passed",
      "createdAt": "2024-11-03T14:30:00Z",
      "updatedAt": "2024-11-03T14:35:00Z",
      "kubeconfigPath": ".forge/kindenvs/test-integration-20241103-abc12345/kubeconfig",
      "managedResources": [
        ".forge/kindenvs/test-integration-20241103-abc12345"
      ],
      "metadata": {}
    }
  }
}
```

## Common Workflows

### Unit Tests (No Environment)

```bash
# Simple run
forge test unit run

# With forge.yaml
test:
  - name: unit
    engine: "noop"
    runner: "go://test-runner-go"
```

### Integration Tests (Managed Environment)

```bash
# Auto-create and run
forge test integration run

# Manual lifecycle
TEST_ID=$(forge test integration create)
forge test integration run $TEST_ID
forge test integration delete $TEST_ID

# With forge.yaml
test:
  - name: integration
    engine: "go://testenv"
    runner: "go://test-runner-go"
```

### E2E Tests (Long-lived Environment)

```bash
# Create environment once
TEST_ID=$(forge test e2e create)

# Run tests multiple times
forge test e2e run $TEST_ID
forge test e2e run $TEST_ID

# Inspect environment
forge test e2e get $TEST_ID

# Cleanup when done
forge test e2e delete $TEST_ID
```

### CI/CD Integration

```bash
#!/bin/bash
set -e

# Run all test stages
forge test unit run
forge test integration run
forge test e2e run

# Or with cleanup
TEST_ID=$(forge test integration create)
trap "forge test integration delete $TEST_ID" EXIT
forge test integration run $TEST_ID
```

## Implementing Custom Tools

### Custom Test Engine

See [`docs/prompts/create-test-engine.md`](./prompts/create-test-engine.md) for detailed implementation guide.

**Minimal engine:**
```go
// Implement 4 commands: create, get, delete, list
// Support MCP mode with --mcp flag
// Persist state to artifact store
```

**Usage:**
```yaml
test:
  - name: custom
    engine: "go://github.com/myorg/my-engine"
    runner: "go://test-runner-go"
```

### Custom Test Runner

See [`docs/prompts/create-test-runner.md`](./prompts/create-test-runner.md) for detailed implementation guide.

**Minimal runner:**
```go
// Accept <STAGE> <NAME> arguments
// Execute tests
// Output JSON report to stdout
// Support MCP mode with --mcp flag
```

**Usage:**
```yaml
test:
  - name: integration
    engine: "go://testenv"
    runner: "go://github.com/myorg/my-runner"
```

## Troubleshooting

### Environment Not Found

```
Error: test environment not found: test-integration-20241103-abc12345
```

**Solution:**
- Check `forge test integration list` for valid IDs
- Verify `.forge/artifacts.json` exists
- Ensure you're in the project root directory

### Engine Not Found

```
Error: failed to parse engine URI: unsupported engine protocol
```

**Solution:**
- Check forge.yaml has valid engine URI
- Ensure engine format is `go://package-name`
- Verify engine is installed or in `./build/bin/`

### Tests Fail to Run

```
Error: no test runner configured for stage: unit
```

**Solution:**
- Add `runner` field to test stage in forge.yaml
- Ensure runner binary exists or can be installed
- Check runner supports MCP mode

### Permission Denied

```
Error: failed to write artifact store: permission denied
```

**Solution:**
- Check file permissions on `.forge/artifacts.json`
- Ensure directory `.forge/` exists and is writable
- Run with appropriate permissions

### Version Mismatch

```
Warning: engine testenv version mismatch: forge=v0.2.2, engine=v0.1.0
```

**Solution:**
- Update engine: `go install github.com/alexandremahdhaoui/forge/cmd/testenv@v0.2.2`
- Or specify version in forge.yaml: `engine: "go://testenv@v0.2.2"`

## Best Practices

### 1. Use Descriptive Stage Names

```yaml
test:
  - name: unit           # ✅ Clear
  - name: integration    # ✅ Clear
  - name: e2e           # ✅ Clear
  - name: test1         # ❌ Unclear
```

### 2. Pin Engine Versions in CI

```yaml
test:
  - name: integration
    engine: "go://testenv@v1.0.0"  # ✅ Explicit
    runner: "go://test-runner-go@v1.0.0"
```

### 3. Clean Up Test Environments

```bash
# Auto-cleanup pattern
TEST_ID=$(forge test integration create)
trap "forge test integration delete $TEST_ID" EXIT
forge test integration run $TEST_ID
```

### 4. Use Artifact Store for State

Query artifact store for environment details:

```bash
# Get all integration environments
jq '.testEnvironments | to_entries | map(select(.value.name == "integration"))' .forge/artifacts.json
```

### 5. Document Custom Engines

If you create custom engines/runners, document:
- Required configuration
- Environment setup
- Resource requirements
- Cleanup behavior

## Examples

See `.ai/plan/test-command-refactor/examples.md` for more comprehensive examples and patterns.

## Reference

- **Test Engine Guide**: [`docs/prompts/create-test-engine.md`](./prompts/create-test-engine.md)
- **Generic Test Runner Guide**: [`docs/prompts/use-generic-test-runner.md`](./prompts/use-generic-test-runner.md)
- **Architecture**: [ARCHITECTURE.md](../ARCHITECTURE.md#test-infrastructure)
- **Reference Implementation**: `cmd/testenv`, `cmd/generic-test-runner`
