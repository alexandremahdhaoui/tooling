# Forge Design Document

**Forge is an MCP-native build orchestration system where every component -- CLI, build engines, test runners, environment managers -- is an MCP server composable via declarative YAML.**

## Problem Statement

Go teams build services with Makefiles, shell scripts, and ad-hoc tooling. These scripts are imperative and fragile: they lack dependency tracking, produce unreproducible builds, and require manual test environment setup. AI coding agents cannot parse or invoke Makefile targets, so builds remain a human-only workflow. No single tool manages the full build-test lifecycle -- from binary compilation through container packaging, test environment provisioning, test execution, and artifact tracking -- in a way that is both declarative and machine-accessible.

Forge solves this with an MCP-first architecture. Every component speaks JSON-RPC 2.0 over stdio. A single `forge.yaml` file declares builds, tests, and engine aliases. AI agents invoke builds, create test environments, and run test suites through the same protocol that human operators use via the CLI.

## Tenets

1. **AI-native over human-convenient.** When in conflict, optimize for machine readability. Every operation is an MCP tool call.
2. **Declarative over imperative.** `forge.yaml` replaces shell scripts. Configuration declares intent; engines handle execution.
3. **Composable over monolithic.** MCP servers compose via stdio. Each engine is independent, testable, and replaceable.
4. **Convention over configuration.** Sensible defaults reduce boilerplate. Override when needed.
5. **Dogfooding.** Forge builds and tests itself using its own engines and test infrastructure.
6. **Fail fast and loud.** Surface errors immediately. Never silently skip a failing stage.

## Requirements

1. Define builds and tests in a single YAML file.
2. Build Go binaries and container images with automatic dependency tracking.
3. Create and teardown test environments (Kind clusters, TLS registries, Helm charts) in one command.
4. Run all test stages sequentially with fail-fast behavior.
5. Track build artifacts with git SHAs and timestamps.
6. Allow AI agents to invoke all operations via MCP protocol.
7. Support parallel builds and parallel test execution.
8. Support custom build engines and test runners via generic wrappers.

## Out of Scope

- Multi-language package managers (npm, pip) -- `generic-builder` handles these but no native engines exist.
- Remote/distributed builds -- all engines run locally.
- GUI or web dashboard.
- Container orchestration beyond Kind (no EKS, GKE provisioning).

## Success Criteria

- `forge test-all` validates the entire system end-to-end.
- AI agent (Claude Code) builds, tests, and manages environments using only MCP protocol.
- New engine can be scaffolded with `forge-dev` in under 30 minutes.
- Lazy rebuild skips unchanged artifacts, verified by timestamp comparison.

## Proposed Design

### High-Level Architecture

```
+-------------------+
|  AI Agent / User  |
+---------+---------+
          |
     forge.yaml
          |
+---------+---------+
|    forge CLI/MCP  |  Presentation layer (thin)
|                   |  CLI: flags, stdout formatting
| +---------------+ |  MCP: JSON-RPC 2.0 handlers
| | Shared Logic  | |  buildAll(), runTestAll()
| | Config Parser | |  Reads forge.yaml
| | Engine Mgr    | |  Resolves forge:// and alias:// URIs
| | Artifact Store| |  Tracks builds in .forge/artifact-store.yaml
| | Test Orchestr | |  Manages test lifecycle
| +---------------+ |
+---------+---------+
          | MCP over stdio (JSON-RPC 2.0)
          |
    +-----+------+----------+--------------+
    |            |           |              |
+---+----+ +----+----+ +----+-----+ +------+------+
| Build  | | Test    | | TestEnv  | | Code Gen    |
| Engines| | Runners | | Managers | | Tools       |
+--------+ +---------+ +----+-----+ +-------------+
                             |
                       +-----+------+
                       |            |
                  +----+---+  +----+----+
                  |testenv |  |testenv  |
                  |kind    |  |lcr      |
                  +--------+  +---------+
```

### Build Context Resolution

Forge uses two distinct directory concepts:

- **CWD (process-level).** Set by `--cwd` flag (CLI) or `cwd` field (MCP). Applied once at startup. Affects config discovery and all relative paths.
- **Context (per-target).** Set by `build[].context` in `forge.yaml` or `EngineSpec.Context`. Resolved per build target via `resolveContextDir()`. Engines receive an absolute path.

The `context` field in `BuildSpec` specifies where source files live. Forge resolves context before dispatching to engines:

```
forge.yaml                           go.work
  |                                    |
  | build[].context                    | use directives
  | (git URL, path, or empty)         | (./forge, ./forge-station, ...)
  |                                    |
  v                                    v
+------------------------------------------------------------------+
|                     forge CLI (cmd/forge/build.go)                |
|                                                                   |
|  resolveContextDir(context) -> (absDir, cleanup, err)            |
|                                                                   |
|  1. Empty or "." -> CWD                                          |
|  2. Absolute path -> use as-is                                   |
|  3. Relative path -> filepath.Abs                                |
|  4. Git URL -> parse module path -> check go.work -> local dir   |
|                                     or clone to temp dir         |
|                                                                   |
|  Then: resolve src relative to context dir                       |
|  Then: pass absolute paths to engine via MCP                     |
+------------------------------------------------------------------+
        |
        | params["context"] = "/abs/path/to/resolved/dir"
        | params["src"]     = "/abs/path/to/resolved/dir/file"
        v
   MCP Engine (receives absolute paths, no URL parsing)
```

**Git URL formats supported:**

| Format | Example | Module Path |
|--------|---------|-------------|
| SSH | `git@github.com:user/repo.git` | `github.com/user/repo` |
| HTTPS | `https://github.com/user/repo` | `github.com/user/repo` |
| SSH protocol | `ssh://git@github.com/user/repo.git` | `github.com/user/repo` |

**Workspace-first resolution:** When context is a git URL, Forge parses the module path, then checks `go.work` use directives. If a workspace member's `go.mod` module path matches, Forge resolves to the local directory. This avoids cloning repos that already exist locally in the workspace. If no match, Forge clones to a temp directory and cleans up after the build.

**forge.yaml example:**

```yaml
build:
  - name: forge-ws-controller-image
    src: ./containers/forge-ws-controller/Containerfile
    context: git@github.com:alexandremahdhaoui/forge-station.git
    engine: forge://container-build
```

### Workspace Resolution

When forge detects a `go.work` file in the directory tree above CWD, it enables workspace mode and may adjust CWD:

```
resolveWorkspace():

  1. Walk up from CWD looking for go.work
  2. If not found: return (no-op)
  3. Parse use directives (e.g., ./forge, ./forge-station)
  4. If CWD is inside a use directory:
     - Set FORGE_RUN_LOCAL_ENABLED=true
     - Set FORGE_RUN_LOCAL_BASEDIR to forge repo directory
     - Return (CWD already correct, workspace mode enabled)
  5. If CWD is workspace root: find forge repo member, chdir to it
  6. Otherwise (CWD in workspace tree but not in a member):
     find forge repo member, chdir to it
  7. Set FORGE_RUN_LOCAL_ENABLED=true
  8. Set FORGE_RUN_LOCAL_BASEDIR to forge repo directory
```

In all cases where `go.work` is found, workspace mode env vars are set. This covers three scenarios: CWD is the workspace root, CWD is inside a member repo listed in `use` directives, or CWD is elsewhere in the workspace tree. The `--skip-workspace-resolution` flag disables this behavior. Both CLI and MCP call the same `resolveWorkspace()` function.

**CLI startup sequence:**

```
parseGlobalFlags -> apply --cwd -> resolveWorkspace -> changeToProjectDir -> source envFile -> dispatch
```

### Engine Resolution

Forge resolves engine URIs to executable MCP server processes:

- `forge://engine-name` is one of forge's own engines at the running forge
  version, executed via `go run github.com/alexandremahdhaoui/forge/cmd/engine-name@version --mcp`
- `forge://<module-path>[@rev]` is a factory member. The enclosing Go
  workspace wins when it carries the module and `go run` uses the local
  copy. Otherwise forge-factory materialises it and the engine starts as
  `forge-factory run --quiet <module-path> -- --mcp`. The run cache makes a
  warm engine start one exec.
- `alias://name` looks up the `engines` section in `forge.yaml`, then resolves each entry to `forge://` engines
- Local development mode (`FORGE_RUN_LOCAL_ENABLED=true`) builds the engine
  from the forge checkout into `build/local-engines/` and execs the binary,
  because `go run` can neither cross module boundaries nor keep the
  caller's working directory
- `go://` is removed. It fails with one line naming `forge://`.

### Run

`forge run` executes a repo's declarative run targets beside build and
test. forge-factory decides WHAT context a run resolves in; forge decides
HOW the target executes. The boundary is one exec: forge-factory
materialises a directory and calls `forge run <name> -- args` inside it,
marked by `FORGE_RUN_MATERIALIZED=1`.

```yaml
run:
  - name: my-tool
    src: ./cmd/my-tool
    engine: forge://generic-runner   # omitted: the built artifact execs
    factory: git@github.com:x/some-factory.git
    spec: { command: sh, args: [run.sh] }
```

`name` and `src` and `factory` are required. The factory is the trust
boundary in every mode: a repo outside its repos list never runs. The
generated half is `zz_generated.runnable.yaml`, emitted by forge-dev from
the engine's typed sources; its `inputs` name the env vars and files a run
needs, checked before any build, with no defaults.

Address forms:

```
forge run my-tool [-- args]                       name form
forge run ./cmd/my-tool [-- args]                 path form
forge run github.com/x/repo my-tool [-- args]     remote form
forge run github.com/x/repo/cmd/tool@REV [...]    remote dev form
forge clone <factory-url>[@rev] [dir]             bootstrap
```

No revision is typed outside the dev form. Dependency context rules,
ordered, first match wins, one stderr line names the rule:

1. `--factory url[@rev]` overrides everything.
2. The enclosing workspace, when its factory claims the repo.
3. The runnable's own `factory:`. Mandatory, so nothing falls through.

The remote sequence, one stderr line per step: clone (full clone, one per
repo URL, worktrees per resolved tuple, never shallow), resolve-target,
resolve-factory, trust gate, resolve-version (the register's internal
track), pin (the provenance revision record beats tags), checkout, sync
`--only`, inputs, build, exec. A warm cache enters at inputs. A dev `@rev`
skips resolve-version and is marked UNPROVEN on stderr. Exit codes and
stdio pass through untouched; the built-in `forge://generic-runner` runs
in process because `go run` swallows exit codes.

### Testenv Chain Composition

```
forge test integration create
  |
  v
testenv (orchestrator)
  |
  +--> testenv-kind (create Kind cluster)
  |      |
  |      +--> sets KUBECONFIG env var
  |
  +--> testenv-lcr (create TLS registry)
  |      |
  |      +--> reads KUBECONFIG from env
  |      +--> sets TESTENV_LCR_FQDN env var
  |
  +--> testenv-helm-install (install charts)
         |
         +--> reads KUBECONFIG from env
         +--> uses {{.Env.TESTENV_LCR_FQDN}} in values
```

Sub-engines run sequentially. Each propagates environment variables to the next via `envPropagation` config. Template expansion (`{{.Env.VAR}}`) enables dynamic configuration between stages.

### Lazy Rebuild

```
forge build <name>  /  forge test-all
  |
  v
shouldRebuild(artifact, platform)?
  |
  +-- No record for this platform?               --> YES, rebuild
  +-- Record carries no dependencies?            --> YES, rebuild
  +-- A recorded path is missing or its content
  |   digest differs from the recorded one?      --> YES, rebuild
  +-- The record carries an output digest and the
  |   output is missing or its digest differs?   --> YES, rebuild
  +-- None of the above?                         --> SKIP

After build:
  the engine's dependency detector records every file it read
  (go-dependency-detector: go.mod, go.sum and every file of every
  local package the entry reaches; a generator: what it read and what
  it wrote) as {path, sha256}; the engine records the output's sha256
  when it has one. Stored in artifact-store.yaml.
```

The rule knows no clock, no language and no filename. A `touch` changes
nothing; a one-byte edit changes everything; a hand-edited generated file
is stale by the same rule as an edited source; a fresh clone that dates
every file today is exactly as fresh as its content says. There is no
force flag: deleting an output is how a rebuild is asked for by hand.

### Parallel Execution

- **parallel-builder**: Wraps N build engines, runs concurrently, collects results.
- **parallel-test-runner**: Wraps N test runners, runs concurrently, merges TestReports.
  - `primaryCoverageRunner` selects which runner provides coverage data.
  - `TestStats` are summed across all runners.
  - Any failure produces overall failure.

## Technical Design

### Data Model

Key types from `pkg/forge/`:

```go
// BuildSpec represents a single artifact to build
type BuildSpec struct {
    Name    string                 `json:"name"`
    Src     string                 `json:"src"`
    Dest    string                 `json:"dest,omitempty"`
    Context string                 `json:"context,omitempty"`
    Engine  string                 `json:"engine"`
    Spec    map[string]interface{} `json:"spec,omitempty"`
}

// TestSpec defines a test stage configuration
type TestSpec struct {
    Name           string                 `json:"name"`
    Testenv        string                 `json:"testenv,omitempty"`
    Runner         string                 `json:"runner"`
    Spec           map[string]interface{} `json:"spec,omitempty"`
    EnvPropagation *EnvPropagation        `json:"envPropagation,omitempty"`
}

// TestReport represents a test execution report
type TestReport struct {
    ID           string    `json:"id"`
    Stage        string    `json:"stage"`
    Status       string    `json:"status"`
    StartTime    time.Time `json:"startTime"`
    Duration     float64   `json:"duration"`
    TestStats    TestStats `json:"testStats"`
    Coverage     Coverage  `json:"coverage"`
    ErrorMessage string    `json:"errorMessage,omitempty"`
}

// Artifact tracks a built artifact with dependency information
type Artifact struct {
    Name                     string                 `json:"name"`
    Type                     string                 `json:"type"`
    Location                 string                 `json:"location"`
    Timestamp                string                 `json:"timestamp"`
    Version                  string                 `json:"version"`
    Dependencies             []ArtifactDependency   `json:"dependencies,omitempty"`
    DependencyDetectorEngine string                 `json:"dependencyDetectorEngine,omitempty"`
    DependencyDetectorSpec   map[string]interface{} `json:"dependencyDetectorSpec,omitempty"`
}

// TestEnvironment represents a test environment instance
type TestEnvironment struct {
    ID               string            `json:"id"`
    Name             string            `json:"name"`
    Status           string            `json:"status"`
    CreatedAt        time.Time         `json:"createdAt"`
    UpdatedAt        time.Time         `json:"updatedAt"`
    TmpDir           string            `json:"tmpDir,omitempty"`
    Files            map[string]string `json:"files,omitempty"`
    ManagedResources []string          `json:"managedResources"`
    Metadata         map[string]string `json:"metadata,omitempty"`
    Env              map[string]string `json:"env,omitempty"`
}

// ArtifactStore is the top-level storage structure
type ArtifactStore struct {
    Version          string                      `json:"version"`
    LastUpdated      time.Time                   `json:"lastUpdated"`
    Artifacts        []Artifact                  `json:"artifacts"`
    TestEnvironments map[string]*TestEnvironment `json:"testEnvironments,omitempty"`
    TestReports      map[string]*TestReport      `json:"testReports,omitempty"`
}
```

Key types from `pkg/forge/` (engine configuration):

```go
// EngineSpec defines per-engine runtime configuration in alias definitions
type EngineSpec struct {
    Command string            `json:"command,omitempty"`
    Args    []string          `json:"args,omitempty"`
    Env     map[string]string `json:"env,omitempty"`
    EnvFile string            `json:"envFile,omitempty"`
    Context string            `json:"context,omitempty"` // Per-engine working directory
}
```

Key types from `pkg/mcptypes/`:

```go
// BuildInput and RunInput use CWD (process-level) set by forge before dispatch.
// They do not carry a WorkDir field. Engines inherit the process CWD.
//
// DetectMockDependenciesInput uses RootDir for project root discovery.
type DetectMockDependenciesInput struct {
    RootDir string `json:"rootDir"` // Project root for .mockery.yaml discovery
}
```

### MCP Protocol

All engines communicate via JSON-RPC 2.0 over stdio. Forge spawns each engine as a child process with `--mcp` flag, then sends tool calls over stdin and reads responses from stdout.

All 12 MCP tool inputs accept an optional `cwd` field (JSON tag `"cwd"`) that overrides the server's working directory for the duration of that request. The MCP handler acquires a mutex, calls `os.Chdir(cwd)`, and restores the original directory on completion.

**Request:**

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "build",
    "arguments": {
      "name": "forge",
      "src": "./cmd/forge",
      "dest": "./build/bin",
      "context": "/abs/path/to/project",
      "engine": "forge://go-build"
    }
  }
}
```

**Response:**

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "{\"name\":\"forge\",\"type\":\"binary\",\"location\":\"./build/bin/forge\"}"
      }
    ]
  }
}
```

**Tool names by engine category:**

| Category | MCP Tool Names |
|----------|---------------|
| Build engines | `build` |
| Test runners | `run` |
| Testenv managers | `create`, `get`, `list`, `delete` |
| Dependency detectors | `detectDependencies` |
| Test report management | `get`, `list`, `delete` |

### Component Catalog

28 CLI tools in `cmd/`, built with Go 1.25.0:

| Name | Location | Category | MCP Tools |
|------|----------|----------|-----------|
| forge | `cmd/forge` | Orchestration | `build`, `test-all`, `test-create`, `test-run`, `test-get`, `test-list`, `test-delete`, `config-validate` |
| forge-dev | `cmd/forge-dev` | Orchestration | `build` |
| forge-e2e | `cmd/forge-e2e` | Orchestration | `run` |
| go-build | `cmd/go-build` | Build Engine | `build` |
| container-build | `cmd/container-build` | Build Engine | `build` |
| generic-builder | `cmd/generic-builder` | Build Engine | `build` |
| parallel-builder | `cmd/parallel-builder` | Build Engine | `build` |
| go-dependency-detector | `cmd/go-dependency-detector` | Dependency Detection | `detectDependencies` |
| go-gen-mocks-dep-detector | `cmd/go-gen-mocks-dep-detector` | Dependency Detection | `detectDependencies` |
| go-gen-openapi-dep-detector | `cmd/go-gen-openapi-dep-detector` | Dependency Detection | `detectDependencies` |
| testenv | `cmd/testenv` | Test Environment | `create`, `get`, `list`, `delete` |
| testenv-kind | `cmd/testenv-kind` | Test Environment | `create`, `get`, `list`, `delete` |
| testenv-lcr | `cmd/testenv-lcr` | Test Environment | `create`, `get`, `list`, `delete` |
| testenv-helm-install | `cmd/testenv-helm-install` | Test Environment | `create`, `get`, `list`, `delete` |
| testenv-stub | `cmd/testenv-stub` | Test Environment | `create`, `get`, `list`, `delete` |
| go-test | `cmd/go-test` | Test Runner | `run` |
| go-lint-tags | `cmd/go-lint-tags` | Test Runner | `run` |
| generic-test-runner | `cmd/generic-test-runner` | Test Runner | `run` |
| parallel-test-runner | `cmd/parallel-test-runner` | Test Runner | `run` |
| test-report | `cmd/test-report` | Test Management | `get`, `list`, `delete` |
| go-format | `cmd/go-format` | Code Quality | `build` |
| go-lint | `cmd/go-lint` | Code Quality | `run` |
| go-lint-licenses | `cmd/go-lint-licenses` | Code Quality | `run` |
| go-gen-mocks | `cmd/go-gen-mocks` | Code Generation | `build` |
| go-gen-openapi | `cmd/go-gen-openapi` | Code Generation | `build` |
| go-gen-protobuf | `cmd/go-gen-protobuf` | Code Generation | `build` |
| go-gen-bpf | `cmd/go-gen-bpf` | Code Generation | `build` |

### Public Package Catalog

13 packages in `pkg/`:

| Package | Location | Purpose |
|---------|----------|---------|
| enginecli | `pkg/enginecli` | Common CLI bootstrapping for forge engine binaries |
| enginedocs | `pkg/enginedocs` | Distributed documentation management across engines |
| engineframework | `pkg/engineframework` | MCP tool registration utilities for type-safe engines |
| engineversion | `pkg/engineversion` | Engine version management and reporting |
| eventualconfig | `pkg/eventualconfig` | Channel-based eventual consistency for async config coordination |
| flaterrors | `pkg/flaterrors` | Error tree flattening compatible with errors.Is/As |
| forge | `pkg/forge` | Core types: Spec, BuildSpec, TestSpec, Artifact, ArtifactStore, TestEnvironment, TestReport |
| mcpserver | `pkg/mcpserver` | MCP server framework for stdio-based JSON-RPC 2.0 |
| mcptypes | `pkg/mcptypes` | MCP wire types: BuildInput, RunInput, DetectDependenciesInput |
| mcputil | `pkg/mcputil` | MCP utilities: validation, batch handling, result formatting |
| portalloc | `pkg/portalloc` | Dynamic port allocation for test environments |
| templateutil | `pkg/templateutil` | Template expansion for environment variable interpolation |
| testenvutil | `pkg/testenvutil` | Test environment utilities: environment variable merging |

### Internal Package Catalog

10 packages in `internal/`:

| Package | Location | Purpose |
|---------|----------|---------|
| cmdutil | `internal/cmdutil` | Command execution utilities |
| engineresolver | `internal/engineresolver` | Engine URI resolution (forge://, alias://) |
| enginetest | `internal/enginetest` | Test helpers for engine development |
| forgepath | `internal/forgepath` | Forge path resolution and directory utilities |
| gitutil | `internal/gitutil` | Git operations: commit SHA, version, dirty state |
| integration | `internal/integration` | Integration test utilities and helpers |
| mcpcaller | `internal/mcpcaller` | MCP client caller for engine invocation |
| orchestrate | `internal/orchestrate` | Build and test orchestration logic |
| testutil | `internal/testutil` | Test utilities and assertions |
| util | `internal/util` | General-purpose utilities |

## Design Patterns

1. **MCP-First.** Every engine is an MCP server communicating via JSON-RPC 2.0 over stdio. This makes all tooling directly accessible to AI agents without adapters or wrappers.

2. **Dogfooding.** Forge builds and tests itself. The `forge.yaml` in the repository root defines 28 build targets and 7 test stages that exercise every engine.

3. **Adapter Pattern.** testenv-lcr uses four adapters -- K8s (namespace), TLS (certificates), Credentials (htpasswd), Registry (deployment) -- coordinated via eventualconfig. Each adapter manages one concern.

4. **Eventual Consistency.** The `eventualconfig` package enables concurrent setup phases that depend on each other's outputs. Producers set values; consumers block until values are available. This eliminates polling and race conditions.

5. **Error Aggregation.** `flaterrors.Join` collects errors from multi-step operations (e.g., teardown) instead of failing on the first error. All errors surface to the caller.

6. **Engine URI Convention.** `forge://name` references built-in engines. `alias://name` references user-defined engine chains in `forge.yaml`. This two-tier scheme separates distribution from composition.

7. **Code Generation.** `forge-dev` scaffolds new engines from OpenAPI specs using `engineframework` for type-safe MCP tool registration. Generated code follows `zz_generated` naming convention.

## Alternatives Considered

1. **Do nothing (keep Makefiles and shell scripts).** Rejected: Makefiles are imperative, fragile, and invisible to AI agents. No dependency tracking, no artifact versioning, no reproducible test environments.
2. **Makefile-based orchestration with AI wrappers.** Rejected: wrapping Makefiles in MCP adapters preserves the underlying fragility. Dependency tracking and artifact management still require custom tooling.
3. **Bazel/Buck.** Rejected: heavyweight, poor AI integration, steep learning curve for Go-centric teams.
4. **REST API instead of MCP/stdio.** Rejected: stdio requires no network stack, no server lifecycle management, and provides direct AI agent compatibility via MCP protocol.
5. **Single monolithic binary.** Rejected: composability requires independent engines. Each engine can be developed, tested, and versioned independently.

## Risks and Mitigations

| Risk | Mitigation |
|------|-----------|
| MCP protocol immaturity | Keep JSON-RPC 2.0 core simple; avoid protocol extensions |
| Go-only ecosystem perception | generic-builder and generic-test-runner wrap any CLI command |
| Local-only execution | Acceptable for current scope. Multi-repo and remote execution live in forge-ci. |

## Testing Strategy

`forge test-all` runs all stages sequentially with fail-fast behavior:

| Stage | Tool | Purpose |
|-------|------|---------|
| lint-tags | go-lint-tags | Verify all test files have build tags |
| lint-license | go-lint-licenses | Verify license headers |
| lint | go-lint | golangci-lint |
| unit | go-test | Fast Go tests, no external dependencies |
| integration | go-test + testenv | Kind cluster + TLS registry + Helm charts |
| e2e | forge-e2e | Full system validation |
| e2e-decl | go-test | Declarative YAML-based e2e tests for CLI and MCP tools |
| e2e-stub | go-test + testenv-stub | Lightweight testenv create/list/get/delete workflow |

The e2e-decl stage runs declarative tests defined as YAML files in `test/e2e/testdata/`. The `test/e2e/testrunner` package provides CLI, MCP, and harness executors with an assertion engine and template system for cross-step data flow. Adding a test means adding a YAML file — no Go code changes required.

## FAQ

**Why MCP over gRPC?**
MCP uses stdio transport. No server lifecycle management, no port allocation, no TLS configuration. AI coding agents (Claude Code, Cursor) speak MCP natively. gRPC would require a separate integration layer.

**Why sequential testenv sub-engines?**
Environment variables propagate between stages. testenv-kind sets KUBECONFIG; testenv-lcr reads it and sets TESTENV_LCR_FQDN; testenv-helm-install reads both. Parallel execution would break this dependency chain.

**Why not content-addressable caching?**
Timestamp comparison is simpler and sufficient for local builds. Content hashing adds complexity (large binary hashing, cache invalidation) without proportional benefit for the local-only use case.

## Appendix

### forge.yaml example (forge self-build, abbreviated)

```yaml
name: forge
envFile: .envrc
artifactStorePath: .forge/artifact-store.yaml

engines:
  - alias: setup-e2e-stub
    type: testenv
    testenv:
      - engine: "forge://testenv-stub"

  - alias: setup-integration
    type: testenv
    testenv:
      - engine: "forge://testenv-kind"
      - engine: "forge://testenv-lcr"
        spec:
          enabled: true
          images:
            - name: local://for-testing-purposes:latest
          imagePullSecretNamespaces:
            - default
            - test-podinfo
      - engine: "forge://testenv-helm-install"
        spec:
          charts:
            - name: podinfo-release
              sourceType: helm-repo
              url: https://stefanprodan.github.io/podinfo
              chartName: podinfo
              namespace: test-podinfo
              releaseName: test-podinfo
              createNamespace: true

test:
  - name: lint-tags
    runner: "forge://go-lint-tags"
  - name: lint-license
    runner: "forge://go-lint-licenses"
  - name: lint
    runner: "forge://go-lint"
  - name: unit
    runner: "forge://go-test"
  - name: integration
    runner: "forge://go-test"
    testenv: "alias://setup-integration"
  - name: e2e
    runner: "forge://forge-e2e"
  - name: e2e-decl
    runner: "forge://go-test"
    spec:
      tags:
        - e2e
      packages:
        - ./test/e2e/...
      timeout: "20m"
      args:
        - "-run"
        - "TestE2EDeclarative"
  - name: e2e-stub
    runner: "forge://go-test"
    testenv: "alias://setup-e2e-stub"

build:
  - name: forge
    src: ./cmd/forge
    dest: ./build/bin
    engine: forge://go-build
  - name: go-build
    src: ./cmd/go-build
    dest: ./build/bin
    engine: forge://go-build
  - name: container-build
    src: ./cmd/container-build
    dest: ./build/bin
    engine: forge://go-build
  - name: testenv
    src: ./cmd/testenv
    dest: ./build/bin
    engine: forge://go-build
  - name: forge-dev
    src: ./cmd/forge-dev
    dest: ./build/bin
    engine: forge://go-build
    spec:
      ldflags: "-X main.Version={{.GitVersion}}"
  # Cross-repo container build with context
  - name: forge-ws-controller-image
    src: ./containers/forge-ws-controller/Containerfile
    context: git@github.com:alexandremahdhaoui/forge-station.git
    engine: forge://container-build
  # ... 23 more build targets
```
