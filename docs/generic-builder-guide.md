# Generic Engine Implementation Guide

## Overview

**Generic engines** (`generic-builder` and `generic-test-runner`) are flexible, configuration-driven engines that allow you to execute arbitrary shell commands without writing custom Go code. They bridge the gap between shell scripts and full-featured forge engines, enabling you to integrate any command-line tool into your build and test workflows.

## Why Use Generic Engines?

Use generic engines when you need to:
- **Execute formatters**: gofmt, prettier, black, rustfmt
- **Run code generators**: protoc, mockgen, swagger-codegen
- **Invoke linters**: golangci-lint, eslint, shellcheck
- **Execute test tools**: custom test frameworks, security scanners
- **Run any CLI tool** that produces artifacts or test results

You don't need to write Go code or implement MCP servers - just configure the command in `forge.yaml`.

## The Two Generic Engines

### generic-builder

**Purpose**: Execute shell commands as build steps

**Use cases**:
- Code formatting
- Code generation
- Asset compilation
- Custom build steps
- Tool orchestration

**Output**: Creates an `Artifact` record in the artifact store

### generic-test-runner

**Purpose**: Execute shell commands as test runners

**Use cases**:
- Running linters as tests
- Custom test frameworks
- Security scanning
- Compliance checks
- Any pass/fail validation

**Output**: Creates a `TestReport` with pass/fail status based on exit code

## Quick Start

### 1. Install the Generic Engines

Add to your `forge.yaml` build section:

```yaml
build:
  - name: generic-builder
    src: ./cmd/generic-builder
    dest: ./build/bin
    engine: go://build-go

  - name: generic-test-runner
    src: ./cmd/generic-test-runner
    dest: ./build/bin
    engine: go://build-go
```

Build them:

```bash
go run ./cmd/forge build generic-builder generic-test-runner
```

### 2. Define an Engine Alias

In `forge.yaml`, add an `engines:` section:

```yaml
engines:
  - alias: my-formatter
    engine: go://generic-builder
    config:
      command: "gofmt"
      args: ["-l", "-w", "."]
      env:
        GOFMT_STYLE: "compact"
```

### 3. Use the Alias

**In build specs:**

```yaml
build:
  - name: format-code
    src: .
    engine: alias://my-formatter
```

**In test specs:**

```yaml
test:
  - name: lint
    engine: "noop"
    runner: alias://my-linter
```

## Engine Configuration

### Full Configuration Schema

```yaml
engines:
  - alias: <alias-name>              # Required: Unique identifier
    engine: <engine-uri>             # Required: go://generic-builder or go://generic-test-runner
    config:
      command: <executable>          # Required: Command to execute
      args: [<arg1>, <arg2>, ...]   # Optional: Command arguments
      env:                           # Optional: Environment variables
        KEY1: value1
        KEY2: value2
      envFile: <path-to-file>        # Optional: Load env vars from file
      workDir: <directory>           # Optional: Working directory
```

### Configuration Fields

#### alias (string, required)
The name you'll use to reference this engine with `alias://` protocol.

**Example**: `my-formatter`, `security-scanner`, `code-generator`

#### engine (string, required)
The underlying generic engine to use.

**Values**:
- `go://generic-builder` - For build operations
- `go://generic-test-runner` - For test operations

#### config.command (string, required)
The executable to run. Can be:
- Bare command: `gofmt`, `docker`, `npm`
- Absolute path: `/usr/bin/python3`
- Relative path: `./scripts/build.sh`

The command must be in PATH or be an accessible path.

#### config.args (array of strings, optional)
Arguments passed to the command.

**Examples**:
```yaml
# Formatting
args: ["-l", "-w", "."]

# Testing with coverage
args: ["test", "-cover", "-v", "./..."]

# Docker build
args: ["build", "-t", "myimage:latest", "."]
```

**Note**: Arguments are passed as separate items, not as a single shell string.

#### config.env (map of strings, optional)
Environment variables to set for the command.

**Example**:
```yaml
env:
  GO111MODULE: "on"
  GOCACHE: "/tmp/gocache"
  CGO_ENABLED: "0"
```

These variables are added to the system environment. They override system vars with the same name.

#### config.envFile (string, optional)
Path to a file containing environment variables (`.envrc` format).

**File format**:
```bash
# Comments start with #
export MY_VAR=value

# Quotes are supported
QUOTED_VAR="value with spaces"
SINGLE_QUOTE='another value'

# Export keyword is optional
ANOTHER_VAR=value
```

**Precedence** (later overrides earlier):
1. System environment
2. Variables from `envFile`
3. Variables from `env` map

#### config.workDir (string, optional)
Working directory for command execution.

**Default**: Current directory (where forge was invoked)

**Example**:
```yaml
workDir: "./frontend"  # Run npm commands in frontend directory
```

## Usage Patterns

### Pattern 1: Code Formatting

**Use Case**: Format Go code before building

```yaml
engines:
  - alias: go-formatter
    engine: go://generic-builder
    config:
      command: "gofmt"
      args: ["-l", "-w", "."]

build:
  - name: format-code
    src: .
    engine: alias://go-formatter
```

**How it works**:
1. Forge resolves `alias://go-formatter` to `go://generic-builder`
2. Injects config (command, args) into MCP call
3. generic-builder executes: `gofmt -l -w .`
4. Creates artifact record with exit code and output

### Pattern 2: Linting as Tests

**Use Case**: Run golangci-lint as a test stage

```yaml
engines:
  - alias: go-linter
    engine: go://generic-test-runner
    config:
      command: "golangci-lint"
      args: ["run", "./..."]

test:
  - name: lint
    engine: "noop"
    runner: alias://go-linter
```

**How it works**:
1. `forge test lint run` resolves runner alias
2. generic-test-runner executes: `golangci-lint run ./...`
3. Exit code 0 → Test passed
4. Exit code ≠ 0 → Test failed
5. Creates TestReport with status

### Pattern 3: Code Generation

**Use Case**: Generate mocks with mockgen

```yaml
engines:
  - alias: mock-generator
    engine: go://generic-builder
    config:
      command: "mockgen"
      args:
        - "-source=pkg/interfaces.go"
        - "-destination=pkg/mocks/mock_interfaces.go"
      workDir: "."

build:
  - name: generate-mocks
    src: ./pkg/interfaces.go
    dest: ./pkg/mocks
    engine: alias://mock-generator
```

### Pattern 4: Multi-Tool Pipeline

**Use Case**: Run multiple formatters in sequence

```yaml
engines:
  - alias: go-formatter
    engine: go://generic-builder
    config:
      command: "gofmt"
      args: ["-l", "-w", "."]

  - alias: import-formatter
    engine: go://generic-builder
    config:
      command: "goimports"
      args: ["-l", "-w", "."]

build:
  - name: format-go-code
    src: .
    engine: alias://go-formatter

  - name: format-imports
    src: .
    engine: alias://import-formatter
```

### Pattern 5: Environment-Specific Builds

**Use Case**: Build with different configurations

```yaml
engines:
  - alias: build-prod
    engine: go://generic-builder
    config:
      command: "go"
      args: ["build", "-o", "app", "-tags", "prod"]
      env:
        CGO_ENABLED: "0"
        GOOS: "linux"

  - alias: build-dev
    engine: go://generic-builder
    config:
      command: "go"
      args: ["build", "-o", "app-dev", "-tags", "dev"]
      env:
        CGO_ENABLED: "1"

build:
  - name: app-production
    src: ./cmd/app
    dest: ./build/prod
    engine: alias://build-prod

  - name: app-development
    src: ./cmd/app
    dest: ./build/dev
    engine: alias://build-dev
```

### Pattern 6: Using .envrc Files

**Use Case**: Load environment from file

```bash
# .envrc.prod
export AWS_REGION=us-west-2
export LOG_LEVEL=info
export DATABASE_URL=postgres://prod-db:5432/myapp
```

```yaml
engines:
  - alias: deploy-prod
    engine: go://generic-builder
    config:
      command: "./scripts/deploy.sh"
      envFile: ".envrc.prod"
```

### Pattern 7: Docker Operations

**Use Case**: Build container images

```yaml
engines:
  - alias: docker-builder
    engine: go://generic-builder
    config:
      command: "docker"
      args:
        - "build"
        - "-t"
        - "myapp:latest"
        - "."
      env:
        DOCKER_BUILDKIT: "1"

build:
  - name: container-image
    src: ./Dockerfile
    engine: alias://docker-builder
```

### Pattern 8: Custom Test Frameworks

**Use Case**: Run pytest with custom flags

```yaml
engines:
  - alias: pytest-runner
    engine: go://generic-test-runner
    config:
      command: "pytest"
      args:
        - "--verbose"
        - "--cov=src"
        - "--cov-report=xml"
        - "tests/"
      workDir: "./python-service"

test:
  - name: python-tests
    engine: "noop"
    runner: alias://pytest-runner
```

## Command Execution Details

### How Commands Are Executed

When generic-builder/generic-test-runner executes your command:

1. **Working Directory**: Changes to `workDir` if specified
2. **Environment Setup**:
   - Starts with system environment
   - Loads variables from `envFile` if specified
   - Overlays variables from `env` map
3. **Command Execution**: Runs command with args
4. **Output Capture**: Captures stdout and stderr
5. **Exit Code**: Records success/failure based on exit code

### Exit Code Handling

**generic-builder**:
- Exit code is recorded in artifact metadata
- Non-zero exit code causes build failure
- Error message includes stderr output

**generic-test-runner**:
- Exit code 0 → Test passed (status: "passed")
- Exit code ≠ 0 → Test failed (status: "failed")
- TestReport shows pass/fail in structured format

### Environment Variable Precedence

Given this config:
```yaml
config:
  envFile: ".envrc"
  env:
    MY_VAR: "from-config"
```

And system environment: `MY_VAR=from-system`

And `.envrc`: `MY_VAR=from-file`

**Result**: `MY_VAR=from-config` (env map has highest priority)

**Precedence order** (highest to lowest):
1. `config.env` - Inline environment variables
2. `config.envFile` - File-based variables
3. System environment

## Advanced Topics

### Runtime Parameter Overrides

When calling engines via MCP (internally), build/test specs can override config:

```yaml
engines:
  - alias: flexible-builder
    engine: go://generic-builder
    config:
      command: "go"
      args: ["build"]

# This works, but args come from engine config
build:
  - name: my-build
    src: ./cmd/app
    engine: alias://flexible-builder
```

**Note**: Currently, build specs don't support overriding engine config. The config from the engine alias is used as-is.

### Debugging Commands

To see what command is actually being executed:

1. **Check MCP logs**: Look at stderr output when running forge
2. **Test manually**: Extract command from engine config and run it directly
3. **Add echo**: Wrap command in a shell script that echoes before executing

Example wrapper script:
```bash
#!/bin/bash
# debug-wrapper.sh
echo "Command: $@" >&2
exec "$@"
```

```yaml
config:
  command: "./debug-wrapper.sh"
  args: ["go", "build", "./..."]
```

### Working with Paths

**Relative paths** are relative to forge's working directory:
```yaml
config:
  command: "./scripts/build.sh"  # Looks in ./scripts/ from forge root
  workDir: "./subproject"        # Changes to ./subproject
```

**Absolute paths** work as expected:
```yaml
config:
  command: "/usr/local/bin/custom-tool"
```

**PATH lookup**: Commands without slashes are looked up in PATH:
```yaml
config:
  command: "go"  # Finds go in PATH
```

### Error Handling

**Common errors and solutions**:

| Error | Cause | Solution |
|-------|-------|----------|
| `command not found` | Command not in PATH | Use absolute path or install tool |
| `permission denied` | Missing execute permission | `chmod +x <file>` |
| `no such file or directory` | Wrong workDir or path | Check paths are relative to forge root |
| `alias not found` | Typo in alias name | Verify alias name in engines section |

### Security Considerations

⚠️ **Important**: Generic engines execute arbitrary commands with your user permissions.

**Best practices**:
1. **Review commands**: Always inspect engine configs before running
2. **Limit scope**: Use `workDir` to restrict command execution location
3. **Validate inputs**: Don't use untrusted data in commands
4. **Use .envrc carefully**: Don't commit secrets to environment files
5. **Prefer explicit over magic**: Use explicit paths and arguments

**Never do this**:
```yaml
# ❌ BAD: Executing arbitrary user input
config:
  command: "sh"
  args: ["-c", "$(cat user-input.txt)"]
```

**Do this instead**:
```yaml
# ✅ GOOD: Explicit, controlled command
config:
  command: "./scripts/safe-operation.sh"
  args: ["--input", "validated-file.txt"]
```

## Testing Your Generic Engine

### Manual Testing

```bash
# 1. Build the generic engines
go run ./cmd/forge build generic-builder generic-test-runner

# 2. Test generic-builder directly
./build/bin/generic-builder --help

# 3. Test via forge with an alias
go run ./cmd/forge build my-alias-name

# 4. Test generic-test-runner
go run ./cmd/forge test my-test-stage run
```

### Verification Checklist

- [ ] Engine alias defined in `engines:` section
- [ ] Command exists and is executable
- [ ] Arguments are correct (test manually first)
- [ ] Environment variables are set correctly
- [ ] WorkDir points to valid directory
- [ ] Exit code 0 for success scenarios
- [ ] Error messages are clear and helpful

## Complete Example

Here's a complete `forge.yaml` demonstrating generic engines:

```yaml
name: my-project
artifactStorePath: .ignore.artifact-store.yaml

# Define engine aliases
engines:
  # Formatter using gofmt
  - alias: go-formatter
    engine: go://generic-builder
    config:
      command: "gofmt"
      args: ["-l", "-w", "."]

  # Linter using golangci-lint
  - alias: go-linter
    engine: go://generic-test-runner
    config:
      command: "golangci-lint"
      args: ["run", "--timeout=5m", "./..."]
      env:
        GOLANGCI_LINT_CACHE: "/tmp/golangci-cache"

  # Mock generator
  - alias: mock-gen
    engine: go://generic-builder
    config:
      command: "go"
      args: ["generate", "./..."]

  # Security scanner
  - alias: security-scan
    engine: go://generic-test-runner
    config:
      command: "gosec"
      args: ["-fmt=json", "-out=security-report.json", "./..."]

# Build artifacts
build:
  # Format before building
  - name: format-code
    src: .
    engine: alias://go-formatter

  # Generate mocks
  - name: generate-mocks
    src: ./pkg
    engine: alias://mock-gen

  # Build main binary
  - name: myapp
    src: ./cmd/myapp
    dest: ./build/bin
    engine: go://build-go

# Test stages
test:
  # Unit tests (uses built-in runner)
  - name: unit
    engine: "noop"
    runner: go://test-runner-go

  # Linting as test
  - name: lint
    engine: "noop"
    runner: alias://go-linter

  # Security scan as test
  - name: security
    engine: "noop"
    runner: alias://security-scan
```

Usage:
```bash
# Build everything
forge build

# Run linting
forge test lint run

# Run security scan
forge test security run
```

## Comparison with Custom Engines

| Feature | Generic Engine | Custom Engine |
|---------|---------------|---------------|
| Development time | Seconds (just config) | Hours (write Go code) |
| Flexibility | Medium (shell commands) | High (full Go access) |
| Type safety | None | Full Go type checking |
| MCP integration | Automatic | Manual implementation |
| Testing | Test command directly | Unit + integration tests |
| Error handling | Basic (exit codes) | Rich (structured errors) |
| Best for | Simple tools, scripts | Complex logic, API integration |

**When to use generic engines**:
- ✅ Wrapping existing CLI tools
- ✅ Running scripts
- ✅ Simple pass/fail checks
- ✅ Prototyping quickly

**When to write custom engines**:
- ✅ Complex business logic
- ✅ API integrations
- ✅ Advanced error handling
- ✅ Rich artifact metadata
- ✅ Performance-critical operations

## Troubleshooting

### Engine Not Found

```
Error: failed to resolve engine alias://my-engine
```

**Solution**: Check that alias is defined in `engines:` section with exact name.

### Command Execution Fails

```
Error: command not found: my-command
```

**Solutions**:
1. Install the command: `which my-command`
2. Use absolute path: `command: "/usr/local/bin/my-command"`
3. Check PATH: `echo $PATH`

### Environment Variables Not Working

**Problem**: Variables from `envFile` not being used

**Solutions**:
1. Verify file exists and is readable
2. Check file format (KEY=VALUE per line)
3. Use absolute path for `envFile`
4. Check precedence (inline `env` overrides `envFile`)

### WorkDir Not Found

```
Error: no such file or directory
```

**Solution**: Ensure `workDir` is relative to forge root, not current directory.

### Generic-Engine vs Generic-Test-Runner

| Aspect | generic-builder | generic-test-runner |
|--------|----------------|---------------------|
| Purpose | Build operations | Test execution |
| Output | Artifact | TestReport |
| Used in | `build:` specs | `test:` specs |
| Exit code ≠ 0 | Build fails | Test fails (but execution succeeds) |
| MCP tool | `build` | `run` |

## Reference

### forge.yaml Schema for Engines

```yaml
engines:
  - alias: string                    # Required, unique
    engine: string                   # Required, go://generic-builder or go://generic-test-runner
    config:
      command: string                # Required, executable path
      args: array<string>            # Optional, arguments
      env: map<string,string>        # Optional, environment variables
      envFile: string                # Optional, path to env file
      workDir: string                # Optional, working directory
```

### Using Aliases

**In build specs**:
```yaml
build:
  - name: my-artifact
    engine: alias://my-engine-alias
```

**In test specs**:
```yaml
test:
  - name: my-test
    runner: alias://my-runner-alias
```

### CLI Commands

```bash
# Build generic engines
go run ./cmd/forge build generic-builder generic-test-runner

# Test with MCP mode
./build/bin/generic-builder --mcp
./build/bin/generic-test-runner --mcp

# Run directly (useful for debugging)
./build/bin/generic-builder build --help
./build/bin/generic-test-runner run --help
```

## Best Practices

1. **📛 Name aliases clearly**: Use descriptive names like `go-formatter`, not `fmt`

2. **📁 Use workDir**: Isolate operations to specific directories when possible

3. **🔐 Manage secrets carefully**: Use `.envrc` for secrets, keep out of git

4. **✅ Test manually first**: Run commands directly before adding to forge.yaml

5. **📝 Document your engines**: Add comments explaining what each alias does

6. **🔄 Version your tools**: Document required tool versions in README

7. **⚠️ Handle errors**: Ensure commands fail with non-zero exit code on error

8. **🎯 Keep it simple**: If logic gets complex, consider writing a custom engine

## Next Steps

- Read [General Engine Implementation Guide](./engine-implementation-guide.md) for custom engines
- Read [Test Runner Guide](./test-runner-guide.md) for advanced test patterns
- See [Test Engine Guide](./test-engine-guide.md) for environment management
- Check `cmd/generic-builder` and `cmd/generic-test-runner` source code for implementation details

## Summary

Generic engines provide a powerful way to integrate any CLI tool into forge without writing Go code. By defining engine aliases in `forge.yaml`, you can:

✅ Execute arbitrary commands
✅ Configure environments
✅ Run formatters, linters, generators
✅ Create build and test workflows
✅ Prototype quickly

Start with generic engines for simple integrations, then graduate to custom engines when you need advanced features.
