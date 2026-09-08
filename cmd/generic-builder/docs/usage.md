# generic-builder

**Execute any shell command as a forge build step.**

> "I needed to integrate our custom asset compiler into forge without writing Go code. generic-builder let me wrap our existing scripts and get them into the build pipeline in minutes."

## What problem does generic-builder solve?

Not every build step fits into specialized engines like go-build or container-build. generic-builder runs arbitrary shell commands as build steps, letting you integrate any CLI tool into forge workflows.

## How do I use generic-builder?

Add a build target to `forge.yaml`:

```yaml
build:
  - name: format-code
    command: gofumpt
    args: ["-w", "./..."]
    engine: forge://generic-builder
```

Run the build:

```bash
forge build
```

## What configuration options are available?

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Build step name |
| `command` | Yes | Command to execute |
| `args` | No | Command arguments (supports templates) |
| `env` | No | Environment variables |
| `envFile` | No | Path to env file |
| `context` | No | Context directory for command execution |
| `src` | No | Source path (available as template) |
| `dest` | No | Destination path (available as template) |

## What does the command find in its environment?

The engine names no language, so a platform, a mode and a destination reach
the command as plain variables and the command decides what they mean:

| Variable | Value |
|----------|-------|
| `FORGE_PLATFORM` | The os/arch pair being built, e.g. `linux/arm64`. The command runs once per platform the entry declares. |
| `FORGE_OS`, `FORGE_ARCH` | The two halves of `FORGE_PLATFORM`. |
| `FORGE_FROZEN` | `true` or `false`: the repo's `frozen:` setting. A command that reads locks builds strictly against them when true and never repairs them either way. |
| `FORGE_OUT` | Where to write what this run builds: `<dest>/<name>` for the host platform, `<dest>/<name>_<os>_<arch>` for a cross build. Absent when the entry declares no `dest`. A file left at that path is recorded as a binary for that platform. |

Write to `$FORGE_OUT` rather than composing the path in shell: it is the
one place the naming convention lives.

## How do I use template variables?

Arguments support Go template syntax:

| Variable | Description |
|----------|-------------|
| `{{ .Name }}` | Build name |
| `{{ .Src }}` | Source directory |
| `{{ .Dest }}` | Destination directory |
| `{{ .Version }}` | Version string |

Example:

```yaml
build:
  - name: generate-proto
    command: protoc
    args:
      - "--go_out={{ .Dest }}"
      - "{{ .Src }}/api.proto"
    src: ./proto
    dest: ./pkg/api
    engine: forge://generic-builder
```

## How do I run custom scripts?

```yaml
build:
  - name: custom-build
    command: ./scripts/build.sh
    args: ["{{ .Name }}", "{{ .Version }}"]
    env:
      BUILD_MODE: production
    engine: forge://generic-builder
```

## How does it work?

- Executes commands via `exec.Command`
- Captures stdout, stderr, and exit code
- Exit code 0 returns success artifact
- Non-zero exit returns error with output
- Templates are processed before execution

## What's next?

- [schema.md](schema.md) - Configuration reference
- [MCP.md](../MCP.md) - MCP tool documentation
