# Using Generic Test Runner

You are helping a user integrate test commands (linters, security scanners, custom test frameworks) into their forge test workflow using the generic-test-runner, without writing any Go code.

## What is generic-test-runner?

Generic-test-runner is a flexible test execution wrapper that:
- Executes any command as a test runner
- Determines pass/fail based on exit code (0 = pass, non-zero = fail)
- Generates structured TestReport JSON
- Perfect for linters, security scanners, compliance checks, custom test tools

## When to Use generic-test-runner vs Built-in Test Runners

**Use Built-in Test Runners When Available:**
- **Go tests**: Use `go://test-runner-go` for go test (supports coverage, tags, race detector)
- **Build tag verification**: Use `go://test-runner-go-verify-tags` to verify test files have build tags
- **Go linting**: Use `go://lint-go` for golangci-lint
- **E2E tests**: Use `go://forge-e2e` for forge's own e2e testing framework

**Use generic-test-runner When:**
- No built-in runner exists for your test tool
- Running custom linters or security scanners
- Executing compliance checks or validation scripts
- Integrating third-party test frameworks (jest, pytest, etc.)
- Quick prototyping before creating a custom test runner

**Example - Use Built-in:**
```yaml
# ✅ Good: Use built-in for Go tests
test:
  - name: unit
    runner: go://test-runner-go
```

**Example - Use generic-test-runner:**
```yaml
# ✅ Good: No built-in for shellcheck
engines:
  - alias: shellcheck
    type: test-runner
    testRunner:
      - engine: go://generic-test-runner
        spec:
          command: "shellcheck"
          args: ["scripts/*.sh"]

test:
  - name: shell-lint
    runner: alias://shellcheck
```

## Key Difference from generic-builder

| Feature | generic-builder | generic-test-runner |
|---------|----------------|---------------------|
| Purpose | Build operations | Test operations |
| Used in | `build:` section | `test:` section (as runner) |
| Output | Artifact | TestReport |
| Exit code 0 | Success | Test passed |
| Exit code ≠ 0 | Build failed | Test failed (still valid report) |

## Quick Start

### Step 1: Ensure generic-test-runner is Built

Add to your forge.yaml if not present:

```yaml
build:
  - name: generic-test-runner
    src: ./cmd/generic-test-runner
    dest: ./build/bin
    engine: go://build-go
```

Build it: `forge build generic-test-runner`

### Step 2: Define a Test Runner Alias

In your `forge.yaml`, add to `engines:` section:

```yaml
engines:
  - alias: my-test-tool
    type: test-runner
    testRunner:
      - engine: go://generic-test-runner
            spec:
      command: "<command-name>"
      args: ["arg1", "arg2"]
      env:
        VAR_NAME: "value"
      envFile: ".envrc"        # Optional
      workDir: "./subdir"      # Optional
```

### Step 3: Use the Alias as a Test Runner

```yaml
test:
  - name: my-test-stage
    runner: alias://my-test-tool
```

### Step 4: Run

```bash
forge test my-test-stage run
```

## Configuration Reference

Same as generic-builder:

### alias (required)
Unique name for this test runner.

**Example**: `golangci-linter`, `security-scanner`, `custom-validator`

### engine (required)
Must be: `go://generic-test-runner`

### spec.command (required)
The test command to execute.

### spec.args (optional)
Command arguments.

### spec.env (optional)
Environment variables.

### spec.envFile (optional)
Path to environment file.

### spec.workDir (optional)
Working directory.

## Common Patterns

### Pattern 1: Linter as Test

```yaml
engines:
  - alias: golangci-lint
    type: test-runner
    testRunner:
      - engine: go://generic-test-runner
            spec:
      command: "golangci-lint"
      args: ["run", "--timeout=5m", "./..."]
      env:
        GOLANGCI_LINT_CACHE: "/tmp/golangci-cache"

test:
  - name: lint
    runner: alias://golangci-lint
```

**Usage**: `forge test lint run`

### Pattern 2: Security Scanner

```yaml
engines:
  - alias: gosec-scanner
    type: test-runner
    testRunner:
      - engine: go://generic-test-runner
            spec:
      command: "gosec"
      args: ["-fmt=json", "-out=security-report.json", "./..."]

test:
  - name: security
    runner: alias://gosec-scanner
```

**Usage**: `forge test security run`

### Pattern 3: Custom Test Framework (pytest)

```yaml
engines:
  - alias: pytest-runner
    type: test-runner
    testRunner:
      - engine: go://generic-test-runner
            spec:
      command: "pytest"
      args:
        - "--verbose"
        - "--cov=src"
        - "--cov-report=xml"
        - "tests/"
      workDir: "./python-service"
      env:
        PYTHONPATH: "src"

test:
  - name: python-tests
    runner: alias://pytest-runner
```

**Usage**: `forge test python-tests run`

### Pattern 4: Shell Check

```yaml
engines:
  - alias: shellcheck-lint
    type: test-runner
    testRunner:
      - engine: go://generic-test-runner
            spec:
      command: "shellcheck"
      args: ["scripts/*.sh"]

test:
  - name: shell-lint
    runner: alias://shellcheck-lint
```

### Pattern 5: Custom Validation Script

```yaml
engines:
  - alias: custom-validator
    type: test-runner
    testRunner:
      - engine: go://generic-test-runner
            spec:
      command: "./scripts/validate-config.sh"
      args: ["--strict"]
      envFile: ".env.test"

test:
  - name: config-validation
    runner: alias://custom-validator
```

### Pattern 6: Multiple Linters

```yaml
engines:
  - alias: golangci
    type: test-runner
    testRunner:
      - engine: go://generic-test-runner
            spec:
      command: "golangci-lint"
      args: ["run", "./..."]

  - alias: staticcheck
    type: test-runner
    testRunner:
      - engine: go://generic-test-runner
            spec:
      command: "staticcheck"
      args: ["./..."]

  - alias: gosec
    type: test-runner
    testRunner:
      - engine: go://generic-test-runner
            spec:
      command: "gosec"
      args: ["./..."]

test:
  - name: lint-golangci
    runner: alias://golangci

  - name: lint-staticcheck
    runner: alias://staticcheck

  - name: security-scan
    runner: alias://gosec
```

**Usage**:
```bash
# Run all
forge test lint-golangci run
forge test lint-staticcheck run
forge test security-scan run
```

## How It Works

### Exit Code Interpretation

```bash
# Command exits 0 → Test PASSED
golangci-lint run ./...
# Exit code: 0
# Result: Test passed ✅

# Command exits non-zero → Test FAILED
golangci-lint run ./...
# Exit code: 1 (linting issues found)
# Result: Test failed ❌
```

### Test Report Structure

Generic-test-runner generates a TestReport:

```json
{
  "stage": "lint",
  "name": "lint-20251105-123456",
  "status": "passed",
  "timestamp": "2025-11-05T12:34:56Z",
  "testStats": {
    "total": 1,
    "passed": 1,
    "failed": 0,
    "skipped": 0
  },
  "coverage": {
    "percentage": 0.0
  }
}
```

**Note**: generic-test-runner doesn't parse detailed test statistics - it only knows pass/fail based on exit code.

## vs. Custom Test Runners

| Runner | Purpose | Features |
|--------|---------|----------|
| `go://generic-test-runner` | Any command | Simple pass/fail, no parsing |
| Custom runner | Complex logic | Full control, custom parsing |

**Use generic-test-runner when**:
- You just need pass/fail (exit code is enough)
- Tool is a CLI command
- You don't need detailed test statistics

**Use custom runner when**:
- You need to parse test output
- You want detailed statistics
- You need complex logic

## Complete Example

Here's a comprehensive test setup:

```yaml
name: my-project
artifactStorePath: .forge/artifacts.yaml

engines:
  # Formatters (for build)
  - alias: go-fmt
    engine: go://generic-builder
    spec:
      command: "gofmt"
      args: ["-l", "-w", "."]

  # Test runners
  - alias: golangci-lint
    type: test-runner
    testRunner:
      - engine: go://generic-test-runner
            spec:
      command: "golangci-lint"
      args: ["run", "--timeout=5m", "./..."]

  - alias: gosec
    type: test-runner
    testRunner:
      - engine: go://generic-test-runner
            spec:
      command: "gosec"
      args: ["-quiet", "./..."]

  - alias: staticcheck
    type: test-runner
    testRunner:
      - engine: go://generic-test-runner
            spec:
      command: "staticcheck"
      args: ["./..."]

  - alias: shellcheck
    type: test-runner
    testRunner:
      - engine: go://generic-test-runner
            spec:
      command: "shellcheck"
      args: ["scripts/*.sh"]

build:
  - name: format-code
    src: .
    engine: alias://go-fmt

  - name: myapp
    src: ./cmd/myapp
    dest: ./build/bin
    engine: go://build-go

test:
  # Unit tests with generic runner
  - name: unit
    runner: "go://generic-test-runner"

  # Linting
  - name: lint
    runner: alias://golangci-lint

  # Security scanning
  - name: security
    runner: alias://gosec

  # Static analysis
  - name: static-analysis
    runner: alias://staticcheck

  # Shell script linting
  - name: shell-lint
    runner: alias://shellcheck
```

**Usage**:
```bash
# Build
forge build

# Run all tests
forge test unit run
forge test lint run
forge test security run
forge test static-analysis run
forge test shell-lint run
```

## Debugging

### Test Command Manually

```bash
# Extract command from config and run it
golangci-lint run --timeout=5m ./...

# Check exit code
echo $?
# 0 = would pass
# non-zero = would fail
```

### Verbose Output

Add verbose flags to your command:

```yaml
spec:
  command: "golangci-lint"
  args: ["run", "--verbose", "./..."]
```

### Environment Issues

Test with same environment:

```bash
# If using envFile
source .envrc

# Then run command
your-command
```

## Integration with CI/CD

### GitHub Actions

```yaml
name: CI

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Setup Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'

      - name: Install forge
        run: go install github.com/alexandremahdhaoui/forge/cmd/forge@latest

      - name: Install tools
        run: |
          go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
          go install github.com/securego/gosec/v2/cmd/gosec@latest

      - name: Run tests
        run: |
          forge test unit run
          forge test lint run
          forge test security run
```

## Common Tools to Wrap

### Go Ecosystem

- `golangci-lint` - Comprehensive linter
- `staticcheck` - Static analysis
- `gosec` - Security scanner
- `go vet` - Go's built-in checker
- `revive` - Fast linter
- `ineffassign` - Dead code detector

### Other Languages

- `eslint` - JavaScript/TypeScript
- `pylint` / `flake8` - Python
- `rubocop` - Ruby
- `shellcheck` - Shell scripts
- `hadolint` - Dockerfile
- `yamllint` - YAML files

### Security Tools

- `trivy` - Vulnerability scanner
- `snyk` - Dependency scanner
- `semgrep` - SAST tool
- `bandit` - Python security

## Advanced: Combining with Test Engines

For integration tests that need environments:

```yaml
test:
  # Integration tests with environment
  - name: integration
    testenv: "go://testenv"  # Creates test environment
    runner: "go://generic-test-runner"    # Runs tests in that environment

  # Linting (no environment needed)
  - name: lint
    engine: "noop"  # No environment
    runner: alias://golangci-lint
```

## Troubleshooting

| Issue | Solution |
|-------|----------|
| Command not found | Install tool or use absolute path |
| Tests fail unexpectedly | Run command manually to debug |
| Environment vars not working | Check .envrc format and precedence |
| Slow execution | Add caching or timeout flags |

## When to Write Custom Test Runner

Write a custom test runner when you need:
- Parse detailed test output (JUnit XML, TAP, etc.)
- Extract coverage data
- Generate rich test statistics
- Complex test orchestration
- Custom reporting formats

For simple pass/fail checks, generic-test-runner is perfect!

## Related Prompts

- `forge prompt get use-generic-builder` - For build operations
- `forge prompt get create-test-runner` - Custom test runners
- `forge prompt get migrate-makefile` - Migrate from Makefile

## Reference

Full documentation:
```
https://raw.githubusercontent.com/alexandremahdhaoui/forge/refs/heads/main/docs/generic-builder-guide.md
```

## Summary

1. Define test runner alias with command + args
2. Use `engine: go://generic-test-runner`
3. Reference as `runner: alias://your-alias` in test specs
4. Run with `forge test <stage> run`
5. Pass/fail determined by exit code

Generic-test-runner makes it trivial to turn any command into a forge test!
