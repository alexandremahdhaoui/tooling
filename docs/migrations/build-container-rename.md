# Migration Guide: build-container → container-build

## Summary

The `build-container` engine has been renamed to `container-build` to follow naming conventions where the action comes first (e.g., `go-build`, `container-build` instead of `build-go`, `build-container`). This makes the tool naming more consistent across the forge ecosystem.

## What Changed

### Engine URI
- **Before:** `go://build-container`
- **After:** `go://container-build`

### Binary Name
- **Before:** `build-container`
- **After:** `container-build`

### Directory Structure
- **Before:** `cmd/build-container/`
- **After:** `cmd/container-build/`

### Environment Variable
- **Before:** `CONTAINER_ENGINE`
- **After:** `CONTAINER_BUILD_ENGINE`

## New Features in container-build

The renamed tool also adds multi-mode support for different container build engines:

- **docker mode:** Native `docker build` (NEW)
  - Fast builds using Docker daemon
  - Requires Docker installed and running
  - Best for local development

- **kaniko mode:** Existing Kaniko-based builds (IMPROVED)
  - Rootless container builds
  - Runs in a container (requires Docker or Podman to run Kaniko)
  - Secure and reproducible
  - Best for CI/CD environments

- **podman mode:** Native `podman build` (NEW)
  - Rootless builds using Podman
  - Requires Podman installed
  - Best for rootless workflows

Set the build mode via the `CONTAINER_BUILD_ENGINE` environment variable.

## How to Migrate

### 1. Update forge.yaml

Find all build specs using the old engine URI:

```yaml
build:
  - name: my-app
    src: ./containers/my-app/Containerfile
    engine: go://build-container  # OLD
```

Change to:

```yaml
build:
  - name: my-app
    src: ./containers/my-app/Containerfile
    engine: go://container-build  # NEW
```

### 2. Update Environment Variables

**Before:**
```bash
export CONTAINER_ENGINE=kaniko
```

**After:**
```bash
export CONTAINER_BUILD_ENGINE=kaniko
```

### 3. Update Scripts

If you have scripts invoking the binary directly:

```bash
# Before
go run ./cmd/build-container

# After
go run ./cmd/container-build
```

Or if using the built binary:

```bash
# Before
build/bin/build-container

# After
build/bin/container-build
```

### 4. Update Documentation

Update any project-specific documentation referencing `build-container` to use `container-build`.

## Backward Compatibility

**The old URI `go://build-container` still works** via automatic aliasing in the forge orchestrator. When the old URI is used:
- A deprecation warning is printed to stderr
- The build proceeds using the new `container-build` engine
- Your builds will NOT break

This allows gradual migration across teams and projects without immediate breaking changes.

### Example Deprecation Warning

```
⚠️  DEPRECATED: go://build-container is deprecated, use go://container-build instead (in spec: my-app)
```

## Deprecation Timeline

| Version | Status | Description |
|---------|--------|-------------|
| v0.x.x (current) | **Alias Active** | `go://build-container` works with deprecation warning |
| v0.y.0 (next minor) | **Deprecation Notice** | Warning includes version when alias will be removed |
| v1.0.0 (planned) | **Alias Removed** | Must use `go://container-build`, old URI returns error |

### Important Dates
- **Now - v1.0.0:** Migrate at your convenience, builds won't break
- **v1.0.0 Release:** Estimated Q2 2025 - must migrate before this version
- **Grace Period:** Approximately 6 months to migrate

### What Happens After v1.0.0?

If you use `go://build-container` after v1.0.0:
- Build will FAIL with error: "Unknown engine: go://build-container (renamed to container-build in v0.x.0)"
- Migration is simple: Update `forge.yaml` to use `go://container-build` and set `CONTAINER_BUILD_ENGINE` environment variable

## Migration Checklist

Use this checklist to ensure complete migration:

- [ ] Update all `forge.yaml` files to use `go://container-build`
- [ ] Update environment variable from `CONTAINER_ENGINE` to `CONTAINER_BUILD_ENGINE`
- [ ] Choose a build mode: `docker`, `kaniko`, or `podman`
- [ ] Update CI/CD scripts if they invoke the binary directly
- [ ] Update project documentation and README files
- [ ] Test builds work: `forge build`
- [ ] Verify no deprecation warnings in stderr output
- [ ] Update team documentation and notify team members

## Mode Selection Guide

Choose the appropriate build mode for your use case:

### Use `docker` mode when:
- You have Docker installed locally
- You want the fastest build times
- You're building on a development machine
- You need native Docker features

### Use `kaniko` mode when:
- You need rootless builds
- You're running in CI/CD environments
- You want reproducible builds
- Security is a high priority

### Use `podman` mode when:
- You prefer Podman over Docker
- You need rootless builds
- You're running in environments without Docker
- You want daemonless container builds

## Troubleshooting

### Error: "CONTAINER_BUILD_ENGINE environment variable required"

You need to set the `CONTAINER_BUILD_ENGINE` environment variable. Example:

```bash
export CONTAINER_BUILD_ENGINE=docker
forge build
```

### Error: "invalid CONTAINER_BUILD_ENGINE: must be one of [docker kaniko podman]"

You provided an invalid value for `CONTAINER_BUILD_ENGINE`. Valid values are:
- `docker`
- `kaniko`
- `podman`

### Builds still reference old URI

If you see deprecation warnings after updating `forge.yaml`, check:
1. You saved the `forge.yaml` file
2. You're in the correct directory
3. You're not using a cached copy of the config
4. Search your project for any remaining `go://build-container` references

## Questions?

See the following documentation for more information:
- [docs/built-in-tools.md](../built-in-tools.md) - Complete `container-build` documentation
- [docs/forge-usage.md](../forge-usage.md) - General forge usage guide
- [cmd/container-build/MCP.md](../../cmd/container-build/MCP.md) - MCP server documentation

## Feedback

If you encounter issues during migration or have suggestions for improving this process, please open an issue in the forge repository.
