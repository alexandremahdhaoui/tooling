# Using Generic Engine

You are helping a user integrate a command-line tool into their forge build process using the generic-engine, without writing any Go code.

## What is generic-engine?

Generic-engine is a flexible, configuration-driven build engine that executes arbitrary shell commands. It's perfect for:
- Wrapping formatters (gofmt, prettier, rustfmt)
- Running code generators (protoc, mockgen, swagger-codegen)
- Executing build scripts
- Calling external tools (docker, npm, make)

## Quick Start

### Step 1: Ensure generic-engine is Built

Add to your forge.yaml if not present:

```yaml
build:
  - name: generic-engine
    src: ./cmd/generic-engine
    dest: ./build/bin
    engine: go://build-go
```

Build it: `forge build generic-engine`

### Step 2: Define an Engine Alias

In your `forge.yaml`, add an `engines:` section:

```yaml
engines:
  - alias: my-tool
    engine: go://generic-engine
    config:
      command: "<command-name>"
      args: ["arg1", "arg2"]
      env:
        VAR_NAME: "value"
      envFile: ".envrc"        # Optional
      workDir: "./subdir"      # Optional
```

### Step 3: Use the Alias in Build Specs

```yaml
build:
  - name: my-artifact
    src: ./source
    dest: ./output
    engine: alias://my-tool
```

### Step 4: Run

```bash
forge build my-artifact
```

## Configuration Reference

### alias (required)
Unique name for this engine configuration.

**Example**: `go-formatter`, `protoc-generator`, `npm-builder`

### engine (required)
Must be: `go://generic-engine`

### config.command (required)
The executable to run. Can be:
- Command in PATH: `"go"`, `"docker"`, `"npm"`
- Relative path: `"./scripts/build.sh"`
- Absolute path: `"/usr/local/bin/tool"`

### config.args (optional)
Array of arguments passed to command.

**Example**:
```yaml
args: ["build", "-o", "output.bin", "input.go"]
```

### config.env (optional)
Environment variables to set.

**Example**:
```yaml
env:
  GO111MODULE: "on"
  CGO_ENABLED: "0"
```

### config.envFile (optional)
Path to file with environment variables (`.envrc` format).

**File format**:
```bash
# .envrc
export VAR1=value1
VAR2=value2
QUOTED="value with spaces"
```

**Precedence**: System env < envFile < inline env

### config.workDir (optional)
Working directory for command execution.

**Example**:
```yaml
workDir: "./frontend"
```

## Common Patterns

### Pattern 1: Code Formatter

```yaml
engines:
  - alias: go-fmt
    engine: go://generic-engine
    config:
      command: "gofmt"
      args: ["-l", "-w", "."]

build:
  - name: format-code
    src: .
    engine: alias://go-fmt
```

### Pattern 2: Code Generator

```yaml
engines:
  - alias: proto-gen
    engine: go://generic-engine
    config:
      command: "protoc"
      args:
        - "--go_out=."
        - "--go-grpc_out=."
        - "api/service.proto"
      workDir: "."

build:
  - name: generate-grpc
    src: ./api
    dest: ./pkg/generated
    engine: alias://proto-gen
```

### Pattern 3: npm Build

```yaml
engines:
  - alias: npm-build
    engine: go://generic-engine
    config:
      command: "npm"
      args: ["run", "build"]
      workDir: "./frontend"
      env:
        NODE_ENV: "production"

build:
  - name: frontend-assets
    src: ./frontend
    dest: ./frontend/dist
    engine: alias://npm-build
```

### Pattern 4: Docker Build

```yaml
engines:
  - alias: docker-build
    engine: go://generic-engine
    config:
      command: "docker"
      args: ["build", "-t", "myapp:latest", "."]
      env:
        DOCKER_BUILDKIT: "1"

build:
  - name: container-image
    src: ./Dockerfile
    engine: alias://docker-build
```

### Pattern 5: Custom Script

```yaml
engines:
  - alias: custom-build
    engine: go://generic-engine
    config:
      command: "./scripts/build.sh"
      args: ["--env=prod", "--verbose"]
      envFile: ".env.prod"

build:
  - name: custom-artifact
    src: ./src
    engine: alias://custom-build
```

### Pattern 6: Multi-Step Pipeline

```yaml
engines:
  - alias: step1-generate
    engine: go://generic-engine
    config:
      command: "./generate.sh"

  - alias: step2-compile
    engine: go://generic-engine
    config:
      command: "gcc"
      args: ["-o", "output", "generated.c"]

build:
  # Run in sequence
  - name: generate-source
    src: ./templates
    engine: alias://step1-generate

  - name: compile-binary
    src: ./generated.c
    dest: ./bin
    engine: alias://step2-compile
```

## Debugging

### Check What's Being Executed

1. **Manual test**: Extract command and run it manually
   ```bash
   # If your config has:
   # command: "go"
   # args: ["build", "./..."]

   # Test manually:
   go build ./...
   ```

2. **Add logging**: Wrap in a script that echoes
   ```bash
   #!/bin/bash
   # debug-wrapper.sh
   echo "Running: $@" >&2
   exec "$@"
   ```

   ```yaml
   config:
     command: "./debug-wrapper.sh"
     args: ["go", "build", "./..."]
   ```

3. **Check environment**: Print env vars
   ```yaml
   config:
     command: "env"
     args: []
   ```

### Common Errors

| Error | Cause | Solution |
|-------|-------|----------|
| `command not found` | Command not in PATH | Use absolute path or install tool |
| `permission denied` | File not executable | `chmod +x <file>` |
| `no such file or directory` | Wrong workDir | Check paths relative to forge root |
| `alias not found` | Typo in alias name | Verify engines section |

## Environment Variables

### Loading from File (.envrc)

Create `.envrc`:
```bash
export AWS_REGION=us-west-2
export DB_HOST=localhost
export DB_PORT=5432
```

Reference in config:
```yaml
config:
  command: "./deploy.sh"
  envFile: ".envrc"
```

⚠️ Add `.envrc` to `.gitignore` for secrets!

### Inline Variables

```yaml
config:
  command: "go"
  args: ["build"]
  env:
    CGO_ENABLED: "0"
    GOOS: "linux"
    GOARCH: "amd64"
```

### Precedence

Given:
- System: `MY_VAR=system`
- .envrc: `MY_VAR=file`
- config.env: `MY_VAR=inline`

Result: `MY_VAR=inline` (inline wins)

## Security Considerations

⚠️ **Generic engines execute with your user permissions**

**Best practices**:
1. Review commands before running
2. Use `workDir` to limit scope
3. Never use untrusted input in commands
4. Don't commit secrets (.envrc files)
5. Use explicit paths over dynamic construction

**Bad**:
```yaml
# ❌ DANGEROUS: Executing arbitrary input
config:
  command: "sh"
  args: ["-c", "$(cat user-input.txt)"]
```

**Good**:
```yaml
# ✅ SAFE: Explicit, controlled command
config:
  command: "./scripts/safe-build.sh"
  args: ["--input", "validated-file.txt"]
```

## When to Use Custom Engine Instead

Use a custom Go engine when you need:
- Complex conditional logic
- API integrations
- Advanced error handling
- Performance-critical operations
- Rich artifact metadata

For simple tool wrapping, generic-engine is perfect!

## Testing Your Configuration

```bash
# 1. Build generic-engine if needed
forge build generic-engine

# 2. Test your command manually first
<your-command> <your-args>

# 3. Add to forge.yaml with alias

# 4. Test via forge
forge build <your-artifact-name>

# 5. Check output and artifacts
ls -la <output-location>
```

## Complete Example

Here's a real-world example combining multiple tools:

```yaml
name: my-project
artifactStorePath: .forge/artifacts.yaml

engines:
  # Formatter
  - alias: go-fmt
    engine: go://generic-engine
    config:
      command: "gofmt"
      args: ["-l", "-w", "."]

  # Linter (as test runner - see use-generic-test-runner prompt)
  - alias: golangci
    engine: go://generic-test-runner
    config:
      command: "golangci-lint"
      args: ["run", "./..."]

  # Mock generator
  - alias: mockgen
    engine: go://generic-engine
    config:
      command: "go"
      args: ["generate", "./..."]

  # Docker builder
  - alias: docker-builder
    engine: go://generic-engine
    config:
      command: "docker"
      args: ["build", "-t", "myapp:latest", "."]
      env:
        DOCKER_BUILDKIT: "1"

build:
  # Format first
  - name: format-code
    src: .
    engine: alias://go-fmt

  # Generate mocks
  - name: generate-mocks
    src: ./pkg
    engine: alias://mockgen

  # Build main app
  - name: myapp
    src: ./cmd/myapp
    dest: ./build/bin
    engine: go://build-go  # Use built-in Go builder

  # Build container
  - name: myapp-container
    src: ./Dockerfile
    engine: alias://docker-builder

test:
  - name: lint
    engine: "noop"
    runner: alias://golangci
```

## Related Prompts

- `forge prompt get use-generic-test-runner` - For test commands
- `forge prompt get create-build-engine` - For custom engines
- `forge prompt get migrate-makefile` - Migrate from Makefile

## Reference

Full documentation:
```
https://raw.githubusercontent.com/alexandremahdhaoui/forge/refs/heads/main/docs/generic-engine-guide.md
```

## Summary

1. Define engine alias with command + args
2. Use alias in build specs
3. Run `forge build`
4. That's it!

Generic-engine makes integrating any CLI tool into forge trivial. No Go code needed!
