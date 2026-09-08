# Lazy Rebuild

**Skip rebuilding what its digests say is fresh.**

## What is lazy rebuild?

A build records what it read and what it wrote, each with the sha256 of
its content. The next build compares those digests with the files on disk
and skips the artifact when every one still matches. Nothing is configured
and no clock is consulted.

## How does forge know when to rebuild?

**Forge rebuilds if ANY of:**
- No record exists for the artifact on this platform
- The record carries no dependencies
- A recorded input is missing, or its content digest differs
- The output carries a recorded digest and is missing or differs

**Forge skips the rebuild if ALL of:**
- A record exists with dependencies
- Every recorded input still has its recorded digest
- The output, when digested, still has its recorded digest

A modification time is never read. A `touch` rebuilds nothing; a fresh
clone that dates every file today is exactly as fresh as its content says;
a one-byte edit rebuilds.

## How do I see it in action?

```bash
# First build
forge build
# Output: Building my-app (no previous build for linux/amd64)

# Second build - nothing changed
forge build
# Output: Skipping my-app (unchanged)

# A touch is not a change
touch cmd/my-app/main.go
forge build
# Output: Skipping my-app (unchanged)

# An edit is
echo '// note' >> cmd/my-app/main.go
forge build
# Output: Building my-app (dependency /abs/cmd/my-app/main.go changed)
```

## How do I force a rebuild?

There is no flag. Delete the output and build:

```bash
rm build/bin/my-app
forge build my-app
# Output: Building my-app (artifact build/bin/my-app missing for linux/amd64)
```

A changed toolchain (a new Go version, different `ldflags`) is not an
input the detector records; deleting the output is how that rebuild is
asked for.

## What is recorded?

- `go-build` asks `go-dependency-detector`, which records `go.mod`,
  `go.sum` and every file of every local package the entry reaches. The
  module closure is `go.sum`, so an external module is never recorded on
  its own. The output binary's digest is recorded too.
- A generator (`forge-dev`, mocks, openapi) records what it read and what
  it wrote, so a hand-edited generated file is stale by the same rule as
  an edited source and regenerates with no flag.
- An engine that records no dependencies rebuilds every time.

## How do I debug rebuild issues?

```bash
# See the record: paths and digests
grep -A 30 "name: my-app" .forge/artifact-store.yaml

# Reset every record
rm -rf build/ .forge/artifact-store.yaml
forge build
```

## What are the limitations?

- **Inputs are what the detector says**: an engine without a detector
  rebuilds every time.
- **The toolchain is not an input**: a new compiler with the same sources
  does not rebuild on its own.
- **Containers need explicit config**: images require `dependsOn`
  configuration, and an image the engine cannot digest is judged on its
  inputs alone.

## What's next?

- [Schema Reference](./forge-yaml-schema.md) - `dependsOn` configuration for containers
- [Getting Started](./getting-started.md) - Basic forge setup
