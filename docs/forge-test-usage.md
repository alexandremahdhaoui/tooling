# Forge Test Command Usage Guide

## Overview

The `forge test` command provides a unified interface for managing test reports, test environments, and executing tests across different stages (unit, integration, e2e, lint, etc.).

**Key Concepts:**
- **Test Reports**: Results from test runs, stored in the artifact store
- **Test Environments**: Infrastructure needed to run tests (Kind clusters, registries, etc.)
- **Test Stages**: Configured in forge.yaml (unit, integration, e2e, lint, etc.)
- **Test-Report Stages**: Stages that only store reports, no persistent environment

**MCP Integration:** All test components (forge CLI, testenv orchestrators, test runners) are MCP servers, enabling AI coding agents to programmatically create environments, run tests, and retrieve results.

## Command Structure

```bash
forge test <SUBCOMMAND> <STAGE> [args...]
```

### Subcommands

**Test Reports** (work with all stages):
- `run <STAGE> [ENV_ID]` - Run tests, optionally in existing environment
- `list <STAGE> [-o json|yaml|table]` - List test reports for a stage
- `get <STAGE> <TEST_ID> [-o json|yaml|table]` - Get test report details
- `delete <STAGE> <TEST_ID>` - Delete a test report

**Test Environments** (only for stages with testenv orchestrators):
- `list-env <STAGE> [-o json|yaml|table]` - List test environments
- `get-env <STAGE> <ENV_ID> [-o json|yaml|table]` - Get environment details
- `create-env <STAGE>` - Create a test environment
- `delete-env <STAGE> <ENV_ID>` - Delete a test environment

## Quick Start

### 1. Configure forge.yaml

```yaml
name: my-project
artifactStorePath: .ignore.artifact-store.yaml

test:
  # Unit tests - test-report only (no environment)
  - name: unit
    testenv: "go://test-report"
    runner: "go://test-runner-go"

  # Integration tests - full test environment
  - name: integration
    testenv: "go://testenv"
    runner: "go://test-runner-go"

  # E2E tests - test-report only
  - name: e2e
    testenv: "go://test-report"
    runner: "go://forge-e2e"

  # Linting - test-report only
  - name: lint
    testenv: "go://test-report"
    runner: "go://lint-go"
```

### 2. Run Tests

```bash
# Run unit tests
forge test run unit

# Run integration tests (auto-creates environment)
forge test run integration

# Run tests in existing environment
forge test run integration <ENV_ID>
```

### 3. View Test Reports

```bash
# List all unit test reports
forge test list unit

# Get specific test report
forge test get unit test-report-unit-20251109-123456

# Delete old test report
forge test delete unit test-report-unit-20251109-123456
```

### 4. Manage Test Environments

```bash
# Create test environment
forge test create-env integration

# List test environments
forge test list-env integration

# Get environment details
forge test get-env integration env-integration-20251109-123456

# Delete environment
forge test delete-env integration env-integration-20251109-123456
```

## Test Reports vs Test Environments

### Test Reports

**Test reports** contain the results of test runs:
- Test statistics (passed, failed, skipped)
- Code coverage
- Test duration
- Error messages
- Output logs

**All stages** produce test reports, regardless of whether they have test environments.

**Commands:**
```bash
# List test reports for a stage
forge test list <STAGE>

# Get test report details
forge test get <STAGE> <TEST_ID>

# Delete test report
forge test delete <STAGE> <TEST_ID>
```

### Test Environments

**Test environments** are the infrastructure needed to run tests:
- Kind Kubernetes clusters
- Local container registries
- Test databases
- Other test infrastructure

**Only stages with testenv orchestrators** (like `go://testenv`) create real test environments.

**Stages with `go://test-report`** show a synthetic "default" environment and reject create/delete operations.

**Commands:**
```bash
# List test environments
forge test list-env <STAGE>

# Get environment details
forge test get-env <STAGE> <ENV_ID>

# Create environment
forge test create-env <STAGE>

# Delete environment
forge test delete-env <STAGE> <ENV_ID>
```

## Test Environment Types

### Test-Report Stages

Stages configured with `testenv: "go://test-report"` only store test reports, they don't create persistent environments.

**Example:**
```yaml
test:
  - name: unit
    testenv: "go://test-report"
    runner: "go://test-runner-go"
```

**Behavior:**
- `run` - Runs tests, stores report
- `list` - Lists test reports
- `get <TEST_ID>` - Shows test report details
- `delete <TEST_ID>` - Deletes test report
- `list-env` - Shows synthetic "default" environment
- `get-env default` - Shows details about test-report behavior
- `create-env` - **Rejected** with helpful error message
- `delete-env` - **Rejected** with helpful error message

**Example interaction:**
```bash
$ forge test list-env unit

=== Test Environments ===
ENV_ID     TYPE            RUNTIME    DESCRIPTION
----------------------------------------------------------------------------------------------------
default    test-report     none       Test report storage only (no persistent environment)

$ forge test create-env unit
Error: stage 'unit' uses test-report for test result storage only.
No environment creation is needed or supported.
To run tests: forge test run unit
To list test reports: forge test list unit
```

### Full Test Environment Stages

Stages configured with `testenv: "go://testenv"` or other testenv orchestrators create real, persistent test environments.

**Example:**
```yaml
test:
  - name: integration
    testenv: "go://testenv"
    runner: "go://test-runner-go"
```

**Behavior:**
- `run [ENV_ID]` - Runs tests, auto-creates environment if not provided
- `list` - Lists test reports
- `get <TEST_ID>` - Shows test report details
- `delete <TEST_ID>` - Deletes test report
- `list-env` - Lists actual test environments
- `get-env <ENV_ID>` - Shows environment configuration and status
- `create-env` - Creates new test environment
- `delete-env <ENV_ID>` - Deletes test environment

## Command Reference

### `forge test run <STAGE> [ENV_ID]`

Run tests for the specified stage.

**Usage:**
```bash
# Auto-create environment and run (for testenv stages)
forge test run <STAGE>

# Run in existing environment
forge test run <STAGE> <ENV_ID>

# Run test-report stage (no environment needed)
forge test run unit
```

**Examples:**
```bash
# Run unit tests
forge test run unit

# Run integration tests (auto-creates environment)
forge test run integration

# Run in specific environment
forge test run integration env-integration-20251109-123456
```

**Output:**
```
Running tests: stage=unit, name=test-report-unit-20251109-143000

Test Results:
Status: passed
Total: 42
Passed: 42
Failed: 0
Coverage: 85.3%
```

**Exit Codes:**
- 0: Tests passed
- Non-zero: Tests failed

---

### `forge test list <STAGE> [-o FORMAT]`

List all test reports for a stage.

**Usage:**
```bash
forge test list <STAGE> [-o json|yaml|table]
```

**Examples:**
```bash
# List with table output (default)
forge test list unit

# List with JSON output
forge test list unit -ojson

# List with YAML output
forge test list unit -oyaml
```

**Output (table format):**
```
=== Test Reports ===
TEST_ID                                  STATUS     PASSED/TOTAL    COVERAGE   TIMESTAMP
----------------------------------------------------------------------------------------------------
test-report-unit-20251109-143000         passed     42/42           85.3%      2025-11-09 14:30:00
test-report-unit-20251109-120000         failed     40/42           80.1%      2025-11-09 12:00:00
```

**Output (JSON format):**
```json
[
  {
    "id": "test-report-unit-20251109-143000",
    "stage": "unit",
    "status": "passed",
    "testStats": {
      "total": 42,
      "passed": 42,
      "failed": 0,
      "skipped": 0
    },
    "coverage": {
      "percentage": 85.3
    },
    "startTime": "2025-11-09T14:30:00Z",
    "duration": 12.5,
    "createdAt": "2025-11-09T14:30:12Z",
    "updatedAt": "2025-11-09T14:30:12Z"
  }
]
```

---

### `forge test get <STAGE> <TEST_ID> [-o FORMAT]`

Get detailed information about a test report.

**Usage:**
```bash
forge test get <STAGE> <TEST_ID> [-o json|yaml|table]
```

**Examples:**
```bash
# Get with YAML output (default for get commands)
forge test get unit test-report-unit-20251109-143000

# Get with JSON output
forge test get unit test-report-unit-20251109-143000 -ojson
```

**Output (YAML format - default):**
```yaml
id: test-report-unit-20251109-143000
stage: unit
status: passed
startTime: "2025-11-09T14:30:00Z"
duration: 12.5
testStats:
  total: 42
  passed: 42
  failed: 0
  skipped: 0
coverage:
  percentage: 85.3
  filePath: /tmp/coverage.out
createdAt: "2025-11-09T14:30:12Z"
updatedAt: "2025-11-09T14:30:12Z"
```

---

### `forge test delete <STAGE> <TEST_ID>`

Delete a test report.

**Usage:**
```bash
forge test delete <STAGE> <TEST_ID>
```

**Examples:**
```bash
# Delete test report
forge test delete unit test-report-unit-20251109-143000
```

**Output:**
```
Deleted test report: test-report-unit-20251109-143000
```

---

### `forge test list-env <STAGE> [-o FORMAT]`

List all test environments for a stage.

**Usage:**
```bash
forge test list-env <STAGE> [-o json|yaml|table]
```

**Examples:**
```bash
# List environments (table format - default)
forge test list-env integration

# List with JSON output
forge test list-env integration -ojson

# List for test-report stage (shows synthetic "default")
forge test list-env unit
```

**Output (for testenv stage):**
```
=== Test Environments ===
ENV_ID                              STATUS      CREATED
----------------------------------------------------------------
env-integration-20251109-123456     created     2025-11-09 12:34
env-integration-20251109-143000     running     2025-11-09 14:30
```

**Output (for test-report stage):**
```
=== Test Environments ===
ENV_ID     TYPE            RUNTIME    DESCRIPTION
----------------------------------------------------------------------------------------------------
default    test-report     none       Test report storage only (no persistent environment)
```

---

### `forge test get-env <STAGE> <ENV_ID> [-o FORMAT]`

Get detailed information about a test environment.

**Usage:**
```bash
forge test get-env <STAGE> <ENV_ID> [-o json|yaml|table]
```

**Examples:**
```bash
# Get environment details (YAML - default)
forge test get-env integration env-integration-20251109-123456

# Get with JSON output
forge test get-env integration env-integration-20251109-123456 -ojson

# Get synthetic "default" for test-report stage
forge test get-env unit default
```

**Output (for testenv stage):**
```yaml
id: env-integration-20251109-123456
stage: integration
status: created
createdAt: "2025-11-09T12:34:56Z"
updatedAt: "2025-11-09T12:34:56Z"
tmpDir: /tmp/forge-test-integration-env-integration-20251109-123456
files:
  testenv-kind.kubeconfig: kubeconfig
  testenv-lcr.credentials: credentials.json
  testenv-lcr.ca-cert: ca.crt
managedResources:
  - /tmp/forge-test-integration-env-integration-20251109-123456
metadata:
  testenv-kind.clusterName: forge-integration-env-integration-20251109-123456
  testenv-lcr.registryURL: https://localhost:5000
```

**Output (for test-report stage):**
```yaml
id: default
stage: unit
type: test-report
runtime: none
description: |-
  This stage uses test-report for test result storage only.
  No persistent test environment is created.
  Test reports are stored in the artifact store and can be listed with:
    forge test list unit
```

**Environment Status Values:**
- `created` - Environment created but not used
- `running` - Tests currently executing
- `passed` - Tests completed successfully
- `failed` - Tests failed
- `partially_deleted` - Cleanup incomplete

---

### `forge test create-env <STAGE>`

Create a new test environment for a stage.

**Usage:**
```bash
forge test create-env <STAGE>
```

**Examples:**
```bash
# Create integration test environment
forge test create-env integration

# Attempting to create for test-report stage (fails with helpful message)
forge test create-env unit
```

**Output (success):**
```
✅ Test environment created: env-integration-20251109-123456
```

**Output (test-report stage):**
```
Error: stage 'unit' uses test-report for test result storage only.
No environment creation is needed or supported.
To run tests: forge test run unit
To list test reports: forge test list unit
```

**What it does:**
1. Generates unique environment ID
2. Invokes testenv orchestrator via MCP
3. Creates infrastructure (Kind cluster, registry, etc.)
4. Generates configuration files (kubeconfig, credentials)
5. Records environment in artifact store

---

### `forge test delete-env <STAGE> <ENV_ID>`

Delete a test environment.

**Usage:**
```bash
forge test delete-env <STAGE> <ENV_ID>
```

**Examples:**
```bash
# Delete integration test environment
forge test delete-env integration env-integration-20251109-123456

# Attempting to delete for test-report stage (fails with helpful message)
forge test delete-env unit default
```

**Output (success):**
```
✅ Test environment deleted: env-integration-20251109-123456
```

**Output (test-report stage):**
```
Error: stage 'unit' uses test-report for test result storage only.
No environment exists to delete.
To delete test reports: forge test delete unit <TEST_ID>
```

**What it does:**
1. Invokes testenv orchestrator delete operation
2. Tears down infrastructure (Kind cluster, registry, etc.)
3. Deletes configuration files
4. Removes managed resources
5. Removes entry from artifact store

---

## Common Workflows

### Unit Tests (Test-Report Only)

```bash
# Run unit tests
forge test run unit

# List test reports
forge test list unit

# Get specific test report details
TEST_ID=$(forge test list unit -ojson | jq -r '.[0].id')
forge test get unit $TEST_ID

# Delete old test report
forge test delete unit $TEST_ID
```

**forge.yaml:**
```yaml
test:
  - name: unit
    testenv: "go://test-report"
    runner: "go://test-runner-go"
```

---

### Integration Tests (Managed Environment)

```bash
# Auto-create environment and run
forge test run integration

# Manual lifecycle
ENV_ID=$(forge test create-env integration)
forge test run integration $ENV_ID
forge test delete-env integration $ENV_ID

# List test reports
forge test list integration

# List test environments
forge test list-env integration
```

**forge.yaml:**
```yaml
test:
  - name: integration
    testenv: "go://testenv"
    runner: "go://test-runner-go"
```

---

### E2E Tests (Long-lived Environment)

```bash
# Create environment once
ENV_ID=$(forge test create-env e2e)

# Run tests multiple times
forge test run e2e $ENV_ID
forge test run e2e $ENV_ID

# Inspect environment
forge test get-env e2e $ENV_ID

# List test reports
forge test list e2e

# Cleanup when done
forge test delete-env e2e $ENV_ID
```

---

### CI/CD Integration

```bash
#!/bin/bash
set -e

# Run unit tests (no environment)
forge test run unit

# Run integration tests (auto-creates environment)
forge test run integration

# Or with manual cleanup
ENV_ID=$(forge test create-env integration)
trap "forge test delete-env integration $ENV_ID" EXIT
forge test run integration $ENV_ID
```

---

## Configuration

### Test Stage Configuration

Define test stages in `forge.yaml`:

```yaml
test:
  - name: <stage-name>
    testenv: <testenv-uri>
    runner: <runner-uri>
```

**Fields:**
- `name`: Stage identifier (e.g., "unit", "integration", "e2e")
- `testenv`: Test environment manager
  - `"go://test-report"` - Test report storage only (no environment)
  - `"go://testenv"` - Full testenv orchestrator
  - `"go://<package>"` - Custom testenv orchestrator
- `runner`: Test executor
  - `"go://test-runner-go"` - Go test runner
  - `"go://lint-go"` - Linter as test runner
  - `"go://<package>"` - Custom test runner

### Testenv URIs

Testenv orchestrators use the `go://` protocol:

```yaml
# Built-in test-report (no environment)
testenv: "go://test-report"

# Built-in testenv orchestrator
testenv: "go://testenv"

# Custom testenv orchestrator
testenv: "go://github.com/myorg/my-testenv"

# With version
testenv: "go://testenv@v1.0.0"
```

**Auto-installation:**
- Forge checks `./build/bin/<binary>` first
- Then checks PATH
- If not found, runs `go install <package>@<version>`
- Verifies version compatibility for forge tools

---

## Troubleshooting

### Test Report Not Found

```
Error: test report not found: test-report-unit-20251109-143000
```

**Solution:**
- Check `forge test list <STAGE>` for valid IDs
- Verify `.ignore.artifact-store.yaml` exists
- Ensure you're in the project root directory

---

### Environment Not Found

```
Error: test environment not found: env-integration-20251109-123456
```

**Solution:**
- Check `forge test list-env <STAGE>` for valid IDs
- Verify `.ignore.artifact-store.yaml` exists
- Ensure environment wasn't deleted

---

### Cannot Create Environment for Test-Report Stage

```
Error: stage 'unit' uses test-report for test result storage only.
No environment creation is needed or supported.
```

**Solution:**
- This is expected behavior for test-report stages
- Use `forge test run unit` to run tests
- Use `forge test list unit` to list test reports
- Test-report stages don't support persistent environments

---

### Testenv Not Found

```
Error: failed to parse engine URI: unsupported engine protocol
```

**Solution:**
- Check forge.yaml has valid testenv URI
- Ensure testenv format is `go://package-name`
- Verify testenv is installed or in `./build/bin/`

---

### Tests Fail to Run

```
Error: no test runner configured for stage: unit
```

**Solution:**
- Add `runner` field to test stage in forge.yaml
- Ensure runner binary exists or can be installed
- Check runner supports MCP mode

---

## Best Practices

### 1. Use Test-Report for Simple Tests

```yaml
test:
  - name: unit           # ✅ Use test-report
    testenv: "go://test-report"
  - name: lint           # ✅ Use test-report
    testenv: "go://test-report"
```

### 2. Use Testenv for Infrastructure-Heavy Tests

```yaml
test:
  - name: integration    # ✅ Use testenv
    testenv: "go://testenv"
  - name: e2e           # ✅ Use testenv if needs cluster
    testenv: "go://testenv"
```

### 3. Clean Up Test Environments

```bash
# Auto-cleanup pattern
ENV_ID=$(forge test create-env integration)
trap "forge test delete-env integration $ENV_ID" EXIT
forge test run integration $ENV_ID
```

### 4. Pin Testenv Versions in CI

```yaml
test:
  - name: integration
    testenv: "go://testenv@v1.0.0"  # ✅ Explicit version
    runner: "go://test-runner-go@v1.0.0"
```

### 5. Use Descriptive Stage Names

```yaml
test:
  - name: unit                  # ✅ Clear
  - name: integration          # ✅ Clear
  - name: integration-kind     # ✅ Specific
  - name: test1                # ❌ Unclear
```

---

## Reference

- **Forge Usage Guide**: [docs/forge-usage.md](./forge-usage.md)
- **Forge Schema**: [docs/forge-schema.md](./forge-schema.md)
- **Architecture**: [ARCHITECTURE.md](../ARCHITECTURE.md#test-infrastructure)
- **Testenv Orchestrator Guide**: [docs/prompts/create-testenv.md](./prompts/create-testenv.md)
- **Testenv Subengine Guide**: [docs/prompts/create-testenv-subengine.md](./prompts/create-testenv-subengine.md)
- **Generic Test Runner Guide**: [docs/prompts/use-generic-test-runner.md](./prompts/use-generic-test-runner.md)
- **Reference Implementation**: `cmd/testenv`, `cmd/test-runner-go`, `cmd/test-report`
