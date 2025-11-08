# Migrating from Makefile to Forge

You are assisting a user in migrating their existing Makefile-based build system to forge, a modern build orchestration tool for Go projects.

## Overview

Forge is a modern, AI-native build and test orchestration system that:
- **MCP-First Architecture:** Forge CLI and all engines are MCP servers, making every component AI-accessible
- Uses declarative YAML configuration instead of imperative Makefiles
- Provides structured artifact tracking and test environment management
- Supports engine-based extensibility for custom build logic
- Offers built-in integration with Go tooling
- Enables reproducible builds with proper dependency management
- Allows AI coding agents to directly invoke build, test, and environment operations

## Migration Strategy

Follow this systematic approach:

### Phase 1: Understanding Current Makefile

1. **Analyze Existing Targets**
   - Identify all make targets
   - Categorize them: build, test, clean, format, lint, deploy, etc.
   - Note dependencies between targets
   - Document environment variables used
   - Identify tools being called (go, docker, npm, etc.)

2. **Map to Forge Concepts**
   - **Build targets** → `build:` section in forge.yaml
   - **Test targets** → `test:` section in forge.yaml
   - **Format/lint targets** → Can be build artifacts or test stages
   - **Complex scripts** → Custom engines or generic-builder with aliases
   - **Environment setup** → Test engines (for environments) or engine config

### Phase 2: Learn About Forge Engines

Before proceeding, read these documentation files using `forge prompt get` or directly from:

**Essential Reading (in order)**:

1. **Generic Builder Usage** (for simple tool wrapping):
   ```
   docs/generic-builder-guide.md
   ```

2. **Generic Test Runner** (for test/lint commands):
   ```
   docs/prompts/use-generic-test-runner.md
   ```

3. **Custom Engine Implementation** (if you need custom logic):
   ```
   docs/prompts/create-build-engine.md
   ```

4. **Test Engine Guide** (if you manage test environments):
   ```
   docs/prompts/create-test-engine.md
   ```

### Phase 3: Create forge.yaml Structure

Start with this template and customize:

```yaml
name: <project-name>
artifactStorePath: .forge/artifacts.yaml

# Define reusable engine aliases for tools from Makefile
engines:
  # Example: Wrapper for a formatter from Makefile
  - alias: my-formatter
    type: builder
    builder:
      - engine: go://generic-builder
        spec:
          command: "gofmt"
          args: ["-l", "-w", "."]

  # Example: Wrapper for a linter from Makefile
  - alias: my-linter
    type: test-runner
    testRunner:
      - engine: go://generic-test-runner
        spec:
          command: "golangci-lint"
          args: ["run", "./..."]

# Build artifacts (from make build, make compile, etc.)
build:
  # Format code (equivalent to: make fmt)
  - name: format-code
    src: .
    engine: alias://my-formatter

  # Build main binary (equivalent to: make build)
  - name: myapp
    src: ./cmd/myapp
    dest: ./build/bin
    engine: go://build-go

# Test stages (from make test, make lint, etc.)
test:
  # Unit tests (equivalent to: make test)
  - name: unit
    runner: "go://test-runner-go"

  # Linting (equivalent to: make lint)
  - name: lint
    runner: alias://my-linter
```

### Phase 4: Migration Patterns

#### Pattern 1: Simple Go Build

**Makefile**:
```makefile
build:
	go build -o bin/myapp ./cmd/myapp
```

**forge.yaml**:
```yaml
build:
  - name: myapp
    src: ./cmd/myapp
    dest: ./bin
    engine: go://build-go
```

#### Pattern 2: Build with Environment Variables

**Makefile**:
```makefile
build:
	CGO_ENABLED=0 GOOS=linux go build -o bin/myapp ./cmd/myapp
```

**forge.yaml**:
```yaml
engines:
  - alias: linux-builder
    type: builder
    builder:
      - engine: go://generic-builder
        spec:
          command: "go"
          args: ["build", "-o", "bin/myapp", "./cmd/myapp"]
          env:
            CGO_ENABLED: "0"
            GOOS: "linux"

build:
  - name: myapp
    src: ./cmd/myapp
    dest: ./bin
    engine: alias://linux-builder
```

#### Pattern 3: Code Formatting

**Makefile**:
```makefile
fmt:
	gofmt -l -w .
	goimports -l -w .
```

**forge.yaml**:
```yaml
engines:
  - alias: go-fmt
    engine: go://generic-builder
    spec:
      command: "gofmt"
      args: ["-l", "-w", "."]

  - alias: go-imports
    engine: go://generic-builder
    spec:
      command: "goimports"
      args: ["-l", "-w", "."]

build:
  - name: format-gofmt
    src: .
    engine: alias://go-fmt

  - name: format-imports
    src: .
    engine: alias://go-imports
```

#### Pattern 4: Running Tests

**Makefile**:
```makefile
test:
	go test -v -race -cover ./...

test-integration:
	go test -v -tags=integration ./...
```

**forge.yaml**:
```yaml
test:
  - name: unit
    # No testenv needed
    runner: "go://generic-test-runner"

  - name: integration
    testenv: "go://testenv"  # Manages test environment
    runner: "go://generic-test-runner"
```

#### Pattern 5: Linting as Tests

**Makefile**:
```makefile
lint:
	golangci-lint run ./...
```

**forge.yaml**:
```yaml
engines:
  - alias: golangci
    engine: go://generic-test-runner
    spec:
      command: "golangci-lint"
      args: ["run", "./..."]

test:
  - name: lint
    # No testenv needed
    runner: alias://golangci
```

#### Pattern 6: Docker Build

**Makefile**:
```makefile
docker-build:
	docker build -t myapp:latest .
```

**forge.yaml**:
```yaml
build:
  - name: myapp-container
    src: ./Dockerfile
    engine: go://build-container
```

Or with generic-builder:

```yaml
engines:
  - alias: docker-builder
    engine: go://generic-builder
    spec:
      command: "docker"
      args: ["build", "-t", "myapp:latest", "."]

build:
  - name: myapp-container
    src: ./Dockerfile
    engine: alias://docker-builder
```

#### Pattern 7: Code Generation

**Makefile**:
```makefile
generate:
	go generate ./...
	mockgen -source=interfaces.go -destination=mocks/mocks.go
```

**forge.yaml**:
```yaml
engines:
  - alias: go-generate
    engine: go://generic-builder
    spec:
      command: "go"
      args: ["generate", "./..."]

  - alias: mockgen
    engine: go://generic-builder
    spec:
      command: "mockgen"
      args: ["-source=interfaces.go", "-destination=mocks/mocks.go"]

build:
  - name: generate-code
    src: .
    engine: alias://go-generate

  - name: generate-mocks
    src: ./interfaces.go
    dest: ./mocks
    engine: alias://mockgen
```

#### Pattern 8: Phony Targets and Scripts

**Makefile**:
```makefile
.PHONY: setup
setup:
	./scripts/setup.sh
	go mod download
```

**forge.yaml**:
```yaml
engines:
  - alias: setup-script
    engine: go://generic-builder
    spec:
      command: "./scripts/setup.sh"

  - alias: mod-download
    engine: go://generic-builder
    spec:
      command: "go"
      args: ["mod", "download"]

build:
  - name: run-setup
    src: ./scripts/setup.sh
    engine: alias://setup-script

  - name: download-deps
    src: ./go.mod
    engine: alias://mod-download
```

### Phase 5: Handle Complex Scenarios

#### When Makefile Logic is Complex

If your Makefile has:
- Complex shell scripts with conditionals
- Multiple interdependent steps
- Dynamic variable generation
- Advanced error handling

**Options**:

1. **Extract to Shell Script + Generic Engine**:
   ```yaml
   engines:
     - alias: complex-build
       engine: go://generic-builder
       spec:
         command: "./scripts/complex-build.sh"
         args: ["arg1", "arg2"]
   ```

2. **Write a Custom Engine**:
   Read: `docs/prompts/create-build-engine.md`

   Custom engines give you full Go power for complex logic.

#### Environment Variables and Secrets

**Makefile**:
```makefile
deploy:
	export AWS_REGION=us-west-2 && ./deploy.sh
```

**forge.yaml with .envrc**:

Create `.envrc`:
```bash
export AWS_REGION=us-west-2
export AWS_ACCESS_KEY_ID=xxx
```

```yaml
engines:
  - alias: deployer
    engine: go://generic-builder
    spec:
      command: "./deploy.sh"
      envFile: ".envrc"
```

⚠️ **Never commit secrets** - add `.envrc` to `.gitignore`

#### Parallel Execution

Forge automatically parallelizes independent builds. Just list them:

```yaml
build:
  # These will build in parallel
  - name: service-a
    src: ./cmd/service-a
    engine: go://build-go

  - name: service-b
    src: ./cmd/service-b
    engine: go://build-go

  - name: service-c
    src: ./cmd/service-c
    engine: go://build-go
```

### Phase 6: Testing the Migration

1. **Start Small**: Migrate one target at a time
2. **Verify Equivalence**: Ensure forge produces same output as make
3. **Test Incrementally**: Run `forge build` after each migration step
4. **Compare Artifacts**: Check that binaries/outputs match

### Phase 7: Migration Checklist

- [ ] All build targets migrated to `build:` section
- [ ] All test targets migrated to `test:` section
- [ ] Environment variables handled (inline or via .envrc)
- [ ] Custom scripts wrapped with generic-builder
- [ ] Complex logic extracted to custom engines (if needed)
- [ ] forge.yaml tested and working
- [ ] CI/CD updated to use `forge` instead of `make`
- [ ] Documentation updated
- [ ] Team trained on new workflow

## Common Commands Mapping

| Makefile | Forge |
|----------|-------|
| `make build` | `forge build` |
| `make test` | `forge test unit run` |
| `make lint` | `forge test lint run` |
| `make clean` | Remove `./build/` directory |
| `make install` | N/A - use go install or deployment tools |
| `make all` | `forge build` (builds everything) |

## Workflow Comparison

**Makefile Workflow**:
```bash
make clean
make build
make test
make lint
make docker-build
```

**Forge Workflow**:
```bash
# Build everything (runs format, then builds all artifacts)
forge build

# Run all tests
forge test unit run
forge test integration run
forge test lint run
```

## Advantages of Migration

1. **Declarative**: YAML config vs imperative shell scripts
2. **Structured**: Clear separation of concerns (build vs test vs environment)
3. **Traceable**: Artifact store tracks all builds and tests
4. **Extensible**: Write custom engines in Go for complex logic
5. **Type-Safe**: Go engines have compile-time checking
6. **Integrated**: Built-in support for Go tooling and patterns
7. **Reproducible**: Consistent artifact tracking and versioning

## Getting Help

- **List available prompts**: `forge prompt list`
- **Get specific prompt**: `forge prompt get <name>`
- **Read documentation**: Check `docs/` directory
- **Examples**: Study `forge.yaml` in forge repository itself

## Quick Migration Example

Here's a complete before/after for a typical Go project:

**Before (Makefile)**:
```makefile
.PHONY: all build test lint clean

all: build test lint

build:
	go build -o bin/myapp ./cmd/myapp

test:
	go test -v -race -cover ./...

lint:
	golangci-lint run ./...

fmt:
	gofmt -l -w .

clean:
	rm -rf bin/
```

**After (forge.yaml)**:
```yaml
name: myproject
artifactStorePath: .forge/artifacts.yaml

engines:
  - alias: formatter
    engine: go://generic-builder
    spec:
      command: "gofmt"
      args: ["-l", "-w", "."]

  - alias: linter
    engine: go://generic-test-runner
    spec:
      command: "golangci-lint"
      args: ["run", "./..."]

build:
  - name: format-code
    src: .
    engine: alias://formatter

  - name: myapp
    src: ./cmd/myapp
    dest: ./bin
    engine: go://build-go

test:
  - name: unit
    # No testenv needed
    runner: "go://generic-test-runner"

  - name: lint
    # No testenv needed
    runner: alias://linter
```

**Usage**:
```bash
# Build everything
forge build

# Run tests
forge test unit run
forge test lint run

# Clean (just delete bin/)
rm -rf bin/
```

## Advanced: Preserving Make Commands

If your team needs a transition period, create a thin Makefile wrapper:

```makefile
.PHONY: build test lint

build:
	forge build

test:
	forge test unit run

lint:
	forge test lint run

all: build test lint
```

This lets teams gradually adopt `forge` commands while maintaining `make` compatibility.

## Summary

1. Read generic-builder and test-runner documentation
2. Map Makefile targets to forge concepts
3. Create forge.yaml with engines and build/test sections
4. Test incrementally
5. Update CI/CD
6. Train team

The migration is straightforward for most projects. Complex logic can be handled with custom engines or extracted scripts. Start simple and iterate!
