# forge-dev Configuration Schema

## Overview

This document describes the configuration files required for `forge-dev` code generation. Each engine using forge-dev needs two configuration files in its directory:

1. **forge-dev.yaml** - Engine metadata and generation settings
2. **spec.openapi.yaml** - OpenAPI 3.0 schema defining the Spec structure

## forge-dev.yaml Schema

```yaml
# Required: Engine name (lowercase alphanumeric with hyphens)
name: my-engine

# Required: Engine type
# Values: builder, test-runner, testenv-subengine
kind: builder

# Required: Engine version (semver format: x.y.z)
version: 0.15.0

# Optional: Human-readable description
description: My custom build engine

# Required: OpenAPI configuration
openapi:
  # Required: Relative path to OpenAPI spec file
  specPath: ./spec.openapi.yaml

# Required: Code generation settings
generate:
  # Required: Go package name for generated code
  packageName: main
```

### Field Reference

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Engine name. Must be lowercase alphanumeric with hyphens, starting with a letter. Max 64 characters. |
| `kind` | string | Yes | What the program is. One of `mcp-server`, `rest-api`, `cli`, `binary`, or a custom kind owned by a `generator:` |
| `profile` | enum | No | The contract kinds' old spelling, folded into `kind` on 2026-09-08. Read only so a file written before the fold still generates; refused once every repo has moved |
| `generator` | string | No | A `forge://` engine that emits this kind and language cell instead of a builtin |
| `layout` | object | No | The kind vocabulary: `tools` for mcp-server, `commands` for cli, anything for a custom kind |
| `version` | string | Yes | Semantic version in format `x.y.z` |
| `description` | string | No | Human-readable description of the engine |
| `openapi.specPath` | string | Yes | Relative path to the OpenAPI spec file |
| `proto.specPath` | string | No | Relative path to the proto file. Joins the source checksum and travels to the generator as `protoSpec` |
| `wiring.specPath` | string | No | Relative path to the wiring file. Joins the source checksum and travels to the generator as `wiringSpec` |
| `configGenerator` | string or object | No | A `forge://` engine that emits the config loader. The object form takes `engine` and `outputDir` |
| `generate.packageName` | string | Yes | Go package name for generated files. Must be a valid Go identifier. |

### Engine Types

**builder**: For build engines that produce artifacts.
- Generated function signature: `BuildFunc(ctx, input mcptypes.BuildInput, spec *Spec) (*forge.Artifact, error)`
- Registers: `build`, `buildBatch`, `config-validate` tools

**test-runner**: For test runner engines.
- Generated function signature: `TestRunnerFunc(ctx, input mcptypes.RunInput, spec *Spec) (*forge.TestReport, error)`
- Registers: `run`, `config-validate` tools

**testenv-subengine**: For test environment subengines.
- Generated function signatures:
  - `CreateFunc(ctx, input engineframework.CreateInput, spec *Spec) (*engineframework.TestEnvArtifact, error)`
  - `DeleteFunc(ctx, input engineframework.DeleteInput, spec *Spec) error`
- Registers: `create`, `delete`, `config-validate` tools

## spec.openapi.yaml Schema

The OpenAPI spec file must define a `Spec` schema under `components.schemas`:

```yaml
openapi: 3.0.3
info:
  title: my-engine Spec Schema
  version: 0.15.0
components:
  schemas:
    Spec:
      type: object
      properties:
        # Define your spec fields here
        fieldName:
          type: string
          description: Field description
        # ... more fields
      required:
        - fieldName  # List required fields
```

### Supported Property Types

#### String

```yaml
myString:
  type: string
  description: A string value
```

Go type: `string`

#### Boolean

```yaml
myBool:
  type: boolean
  description: A boolean flag
```

Go type: `bool`

#### Integer

```yaml
myInt:
  type: integer
  description: An integer value
```

Go type: `int`

#### Number (Float)

```yaml
myFloat:
  type: number
  description: A floating-point value
```

Go type: `float64`

#### String Array

```yaml
myStringArray:
  type: array
  items:
    type: string
  description: An array of strings
```

Go type: `[]string`

#### Integer Array

```yaml
myIntArray:
  type: array
  items:
    type: integer
  description: An array of integers
```

Go type: `[]int`

#### String Map

```yaml
myStringMap:
  type: object
  additionalProperties:
    type: string
  description: A map of string to string
```

Go type: `map[string]string`

#### Enum

```yaml
myEnum:
  type: string
  enum:
    - value1
    - value2
    - value3
  description: An enumerated string
```

Go type: `string` (with validation that value is in the allowed set)

### Required Fields

Use the `required` array at the Spec level to mark required fields:

```yaml
Spec:
  type: object
  properties:
    requiredField:
      type: string
    optionalField:
      type: string
  required:
    - requiredField
```

Required fields:
- Must be provided in configuration
- Generate validation errors if missing
- Do not include `omitempty` in JSON tags

### Default Values

```yaml
myField:
  type: string
  default: "default-value"
```

Default values are currently stored in the schema but not automatically applied. Engines should check for zero values and apply defaults manually.

## Generated Files

### zz_generated.spec.go

Contains:
- `Spec` struct with JSON tags
- `FromMap(map[string]interface{}) (*Spec, error)` - Parse map to Spec
- `ToMap() map[string]interface{}` - Convert Spec to map

### zz_generated.validate.go

Contains:
- `Validate(spec *Spec) *mcptypes.ConfigValidateOutput` - Validate Spec struct
- `ValidateMap(m map[string]interface{}) *mcptypes.ConfigValidateOutput` - Parse and validate map

### zz_generated.mcp.go

Contains:
- Type-safe function type (e.g., `BuildFunc`, `TestRunnerFunc`)
- `SetupMCPServer(version string, fn TypedFunc) (*mcpserver.Server, error)`
- Wrapper functions that handle parsing and validation

## Examples

### Minimal Configuration

**forge-dev.yaml:**
```yaml
name: simple-engine
kind: builder
version: 0.1.0
openapi:
  specPath: ./spec.openapi.yaml
generate:
  packageName: main
```

**spec.openapi.yaml:**
```yaml
openapi: 3.0.3
info:
  title: simple-engine
  version: 0.1.0
components:
  schemas:
    Spec:
      type: object
```

### Full Configuration

**forge-dev.yaml:**
```yaml
name: advanced-engine
kind: builder
version: 0.15.0
description: An advanced build engine with many options
openapi:
  specPath: ./spec.openapi.yaml
generate:
  packageName: main
```

**spec.openapi.yaml:**
```yaml
openapi: 3.0.3
info:
  title: advanced-engine Spec Schema
  version: 0.15.0
components:
  schemas:
    Spec:
      type: object
      properties:
        outputDir:
          type: string
          description: Directory for generated output
        verbose:
          type: boolean
          description: Enable verbose logging
          default: false
        logLevel:
          type: string
          enum:
            - debug
            - info
            - warn
            - error
          description: Logging level
          default: info
        tags:
          type: array
          items:
            type: string
          description: Build tags to include
        env:
          type: object
          additionalProperties:
            type: string
          description: Environment variables
      required:
        - outputDir
```

## Limitations

The following OpenAPI features are **not supported** in v1:

- `$ref` references to other schemas
- `oneOf`, `anyOf`, `allOf` combinators
- Arrays of objects (nested object types)
- Deeply nested objects (only one level of nesting)

If your engine requires these features, it cannot use forge-dev and must implement manual Spec handling.

## See Also

- [forge-dev Usage Guide](usage.md)
- [OpenAPI 3.0 Specification](https://spec.openapis.org/oas/v3.0.3)

## generate.docsBaseURL

The raw content base URL used for remote documentation fetching. Optional.

Defaults to forge's own repository. **A sibling repository that generates
engines with forge-dev must set this**, or its engines advertise forge's URLs
for their own documentation.

```yaml
generate:
  packageName: main
  docsBaseURL: https://raw.githubusercontent.com/alexandremahdhaoui/forge-ci/refs/heads/main
```

It must be an absolute http or https URL with no trailing slash, because every
consumer joins it with a slash already.

## kind: mcp-server

Every other engine type has its tools fixed by its family. A builder has
`build`. A test runner has `run`. A generic engine declares its own, so a
sibling repository can generate an engine forge has never heard of.

```yaml
name: ci-state-git
kind: mcp-server
version: 0.1.0
description: Read and write CI state in a git repo.
openapi:
  specPath: ./spec.openapi.yaml
generate:
  packageName: main
  docsBaseURL: https://raw.githubusercontent.com/alexandremahdhaoui/forge-ci/refs/heads/main
  tools:
    - name: get
      description: Read one record.
      input: StateGetInput
      output: StateGetOutput
      useSpec: true
    - name: put
      description: Write one record.
      input: StatePutInput
```

### generate.tools

| Field | Required | Means |
|---|---|---|
| `name` | yes | The MCP tool name callers use. Alphanumeric with hyphens or underscores. |
| `description` | yes | What the tool does. Shown to callers. |
| `input` | yes | A schema name from `components.schemas`. |
| `output` | no | A schema name. Omit it and the handler returns only an error. |
| `useSpec` | no | Parse and validate `Spec` from the input, and hand the handler a typed value. |

`config-validate` is registered automatically and cannot be declared.

### What gets generated

One `<Name>Func` type per tool, one `Handlers` struct with a field per tool, and
one wrapper per tool. `SetupMCPServer(name, version string, handlers Handlers)`
takes the struct rather than positional arguments, because several tools often
share an input type and positional arguments of near identical types are a
silent miswiring hazard. A nil field returns an error rather than registering a
tool that panics when called.

### What you write

A constructor returning `Handlers`, named `NewHandlers` unless you set
`generate.handlersFunc`.

```go
func NewHandlers() Handlers {
	return Handlers{
		Get: func(ctx context.Context, in StateGetInput, spec *Spec) (*StateGetOutput, error) {
			...
		},
		Put: func(ctx context.Context, in StatePutInput) error {
			...
		},
	}
}
```

### A Spec schema is still required

`components.schemas.Spec` must exist even when an engine reads no
configuration, because the generator emits `Validate` and `FromMap`
unconditionally. Declare an empty one, as `go-dependency-detector` does.

```yaml
components:
  schemas:
    Spec:
      type: object
      description: This engine reads no configuration.
```
