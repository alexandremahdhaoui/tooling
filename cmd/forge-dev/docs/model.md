# The forge-dev model

forge-dev generates programs from typed sources along two axes. Kind says
what the program is. Language says what emits it. A generator fills one
kind and language cell. The defaults ship here. A custom generator is any
forge engine, so a new cell never touches forge-dev.

## What core owns and generators never do

forge-dev core parses `forge-dev.yaml` and the OpenAPI spec, validates
them, computes the source checksum, skips fresh outputs, emits
`zz_generated.runnable.yaml` and the shared docs, and writes files. A
generator only answers file contents. This split is the contract: a
generator cannot weaken freshness, escape the engine directory, or
redefine the runnable manifest.

## Kind, axis one

| Kind | Layout | The generated skeleton |
|---|---|---|
| mcp-server | `layout.tools` | stdio JSON-RPC server, config-validate derived from the Spec schema |
| rest-api | the OpenAPI paths | HTTP server: typed handler per operation, mux from the paths verbatim, LISTENING line on bind. A nil handler answers 501, never a 404 |
| cli | `layout.commands` | argv dispatcher, exit codes pass through, unknown command fails loud |
| binary | none | nothing. The runnable manifest and the docs are the output |

A custom kind is any other name plus whatever layout its generator
defines. Core passes the layout through opaquely and the generator
validates it. Every kind's runnable manifest has the same `inputs:`
shape, so forge-factory checks inputs without knowing kinds.

### The contract kinds

A contract kind names the contract the engine implements. Its tools,
their inputs and their outputs belong to that contract, so the file
declares none of them: a `layout.tools` list beside a contract kind is a
second authority over the same list and is refused.

| Kind | Handler the author writes |
|---|---|
| builder | `Build(ctx, BuildInput, *Spec) (*Artifact, error)` |
| test-runner | `Run(ctx, RunInput, *Spec) (*TestReport, error)` |
| testenv-subengine | `Create` and `Delete` |
| dependency-detector | the detect tool registration |
| mcp-server | one handler per `layout.tools` entry |

## Language, axis two

Builtin cells today. mcp-server in go, rust, python and typescript.
The contract kinds are go only. cli and rest-api in go. binary in every
language. Every other cell is external.

## Generator, the extension door

```yaml
name: my-gui
kind: gui
language: rust
generator: forge://github.com/x/gui-gen/engines/gui-gen
layout:
  windows:
    - name: main-window
```

A generator is an MCP engine with one tool, `generate`. It receives the
normalized model and answers files:

```json
// input
{"name","version","description","kind","language","packageName",
 "layout","runtime","openapiSpec","protoSpec","wiringSpec","checksum","srcDir"}
// output
{"files":[{"path":"zz_generated_gui.rs","content":"..."}],"manifest":false}
```

Paths are relative to the engine directory and never escape it. The
generator resolves like every engine: `forge://` through the factory and
register, so a generator is versioned, proven and cached like anything
else it generates for.

## The wiring spec

A cell may name a `wiring.specPath:` beside `openapi:` and `proto:`. Its
text travels to the generator as `wiringSpec`, and its path joins the
source checksum, so an edit to the wiring file regenerates the cell.

```yaml
wiring:
  specPath: ./wiring.yaml
```

`config-validate` warns when the file does not exist yet, the same way it
warns for an unresolved OpenAPI or proto spec. A cell that declares
`wiring.specPath:` and a `generator:` needs no `openapi.specPath:`.

## What core checks after a generator answers

Three rules run on every answer, before the build calls it a success.

1. Every returned path stays inside the engine directory and is named
   `zz_generated`. A path that is not fails with the path and the
   generator URI, and the whole answer is refused before anything is
   written. A module root is the one exception, because a language names
   it: `lib.rs`, `main.rs`, `mod.rs`, `__init__.py` and `index.ts` are
   accepted when line one carries the generated header. One that misses
   the header fails with the path and says the header is missing.
2. The returned list lands in `zz_generated.runnable.yaml` under `files`.
   On the next run a file the previous list held and the new answer does
   not is removed, and the removal is logged. A recorded entry that
   escapes the engine directory is skipped and logged instead of removed.
   So is one that is neither named `zz_generated` nor a module root. A
   recorded module root is removed only when the file on disk still
   carries the generated header, or when it is already gone. A module
   root without the header is hand written, so it is skipped and logged.
3. An answer with `manifest: true` must hold `zz_generated_cell.yaml`. An
   answer without it fails naming the generator.

## The config keys: the spec is the source of truth

Any cell may name a `configGenerator:`. It speaks the same `generate`
contract, but fills only the config keys: a Spec schema decides the keys,
and the generator answers a typed loader for them. Flag beats env beats
default, a required key with no value is an error, an unknown flag is an
error. Nothing declares a key twice: the schema that validates the spec
is the one that grows its flags and env.

```yaml
kind: cli
configGenerator: forge://github.com/alexandremahdhaoui/golden-configgen/cmd/configgen-gen
```

The block form names the directory the answer lands in. Every path the
config generator answers is prefixed with it. An `outputDir:` that is
absolute or escapes the engine directory fails validation naming it.

```yaml
configGenerator:
  engine: forge://github.com/alexandremahdhaoui/golden-configgen/cmd/configgen-gen
  outputDir: src/config
```

A `generator:` and a `configGenerator:` sit together. The main generator
runs first, then the config generator, then the docs. A main generator
that writes `zz_generated_config_spec.yaml` hands the config generator
that schema instead of the cell's `openapi.specPath`, so a cell whose
keys are derived rather than declared still gets a loader.

## Migration from type: and profile:

Both were the same axis under two names, and both are `kind:` now.

- `type: builder` and the other three, and `profile: X` beside
  `kind: mcp-server`: write `kind: X`.
- `type: generic`: `kind: mcp-server` and move `generate.tools` to
  `layout.tools`.

`type:` fails validation naming its replacement. `profile:` is read for
now, so a checkout written before the fold on 2026-09-08 still
generates; it is refused once every repo has moved.

## Migration from surface:

`surface:` is the old name of `layout:`. A file that still carries it
fails to read with one line: surface was renamed to layout on 2026-09-04,
rename the block.
