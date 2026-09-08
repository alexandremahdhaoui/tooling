# Testing with Forge

**Run tests, manage environments, and get reports with a unified interface.**

> "I needed a way to run unit tests quickly but also spin up full Kubernetes clusters for integration tests. Forge lets me do both with the same commands, and the test environments clean themselves up."

## What problem does forge test solve?

Testing modern applications requires different environments: unit tests need nothing, integration tests need clusters. Forge provides a single interface (`forge test`) that handles all cases, automatically managing test environments and storing reports.

## Table of Contents

- [How do I run tests?](#how-do-i-run-tests)
- [How do I create test environments?](#how-do-i-create-test-environments)
- [How do I manage test stages?](#how-do-i-manage-test-stages)
- [How do I get test reports?](#how-do-i-get-test-reports)

## How do I run tests?

```bash
# Run all tests (builds first, fails fast, auto-cleans environments)
forge test-all

# Run a specific stage
forge test run unit
forge test run integration
forge test run lint
```

Configure stages in `forge.yaml`:

```yaml
test:
  - name: unit
    runner: "forge://go-test"
  - name: integration
    testenv: "forge://testenv"
    runner: "forge://go-test"
  - name: lint
    runner: "forge://go-lint"
```

**Key fields:** `runner` (test executor), `testenv` (environment manager - omit for unit tests).

## How do I create test environments?

**Automatic** (recommended):
```bash
forge test run integration  # Creates env, runs tests, keeps env for inspection
```

**Manual** (for debugging):
```bash
forge test create-env integration           # Create environment
forge test run integration <ENV_ID>         # Run tests in existing env
forge test delete-env integration <ENV_ID>  # Clean up when done
```

For stages with `testenv: "forge://testenv"`, forge creates:
- Kind cluster with unique name
- Local container registry (TLS-enabled)
- Kubeconfig at `.forge/<env-id>/kubeconfig`
- Registry credentials and CA certificate

**Use the environment:**
```bash
ENV_ID=$(forge test list-env integration -ojson | jq -r '.[0].id')
export KUBECONFIG=$(forge test get-env integration $ENV_ID -oyaml | yq .files.kubeconfig)
kubectl get nodes
```

## How do I manage test stages?

| Stage Type | testenv | Use Case |
|------------|---------|----------|
| Unit tests | (omit) | Fast, isolated tests |
| Lint | (omit) | Code quality checks |
| Integration | `forge://testenv` | Tests needing Kubernetes |

**Custom testenv alias** for more control:

```yaml
engines:
  - alias: setup-integration
    type: testenv
    testenv:
      - engine: "forge://testenv-kind"
      - engine: "forge://testenv-lcr"
        spec:
          enabled: true
          autoPushImages: true

test:
  - name: integration
    testenv: "alias://setup-integration"
    runner: "forge://go-test"
```

## What does a stage declare about itself?

Three keys say what a stage is, and nothing is inferred from its name:

| Key | Meaning |
|-----|---------|
| `testenv:` | The stage needs an environment, created before its runner and deleted after. |
| `manual: true` | Not a gate. `forge test-all` skips it; it runs only through `forge test run <name>`. |
| `needs: [entries]` | What the stage needs built first. The named build entries are owned by the stage: a bare `forge build` leaves them alone and the stage builds them before creating its environment. |

```yaml
test:
  - name: integration
    runner: forge://go-test
    testenv: alias://setup-integration
    needs: [for-testing-purposes]   # the fixture image the testenv pushes
```

## How do I get test reports?

```bash
forge test list unit                    # List all reports for a stage
forge test get unit <TEST_ID>           # Get detailed report (YAML)
forge test get unit <TEST_ID> -ojson    # Get as JSON
forge test delete unit <TEST_ID>        # Delete old report
```

Reports include: status, test statistics, coverage percentage, duration, timestamps.

## Quick Reference

```bash
# Run tests
forge test-all                    # Build + all stages (fail-fast)
forge test run <stage>            # Run single stage
forge test run <stage> <ENV_ID>   # Run in existing environment

# Test reports
forge test list <stage>           # List reports
forge test get <stage> <ID>       # Get report details
forge test delete <stage> <ID>    # Delete report

# Test environments
forge test list-env <stage>       # List environments
forge test get-env <stage> <ID>   # Get environment details
forge test create-env <stage>     # Create environment
forge test delete-env <stage> <ID># Delete environment
```

## Related Documentation

- [Forge CLI Reference](./forge-cli.md) - Complete CLI documentation
- [Built-in Tools](./built-in-tools.md) - Test runner and testenv engine details
- [forge.yaml Schema](./forge-yaml-schema.md) - Test configuration options
- [Test Environment Architecture](../architecture/testenv-architecture.md) - Deep dive into testenv design
