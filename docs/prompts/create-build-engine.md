# Creating a Custom Build Engine

You are helping a user create a custom build engine for forge. A build engine transforms source code into artifacts (binaries, containers, formatted code, generated files, etc.).

## What is a Build Engine?

A **build engine** is a component that performs build operations:
- Compiles source code into binaries
- Builds container images
- Formats or transforms code
- Generates files from templates/specs
- Packages artifacts

Build engines communicate with forge via the Model Context Protocol (MCP) and report results as Artifacts.

## When to Create a Custom Build Engine

**Write a custom engine when**:
- ✅ Generic engines are too limited for your needs
- ✅ You need complex build logic or decision-making
- ✅ You need to interact with APIs or services
- ✅ You need rich artifact metadata
- ✅ You need advanced error handling
- ✅ You need build caching or optimization

**Use generic engines when**:
- ✅ You're just wrapping a CLI tool
- ✅ Build logic is simple
- ✅ Exit code is sufficient for error handling
- ✅ See `forge prompt get use-generic-engine`

## API Contract

### CLI Interface (Optional)

You can optionally support direct CLI usage:

```bash
# Build operation
<engine-binary> build [options]
# Output: Human-readable progress, exit code 0=success

# Version information
<engine-binary> version

# MCP server mode (required)
<engine-binary> --mcp
```

### MCP Interface (Required)

Your engine **must** implement these MCP tools:

**build** - Build a single artifact
- Input: `BuildInput { name, src, dest, engine, ... }`
- Output: `Artifact` object with metadata

**buildBatch** - Build multiple artifacts (optional but recommended)
- Input: `BatchBuildInput { specs: [BuildInput...] }`
- Output: Array of `Artifact` objects

### Artifact Structure

Your build must return an Artifact:

```json
{
  "name": "my-app",
  "type": "binary",
  "location": "./build/bin/my-app",
  "timestamp": "2024-01-01T12:00:00Z",
  "version": "abc123"
}
```

## Implementation Steps

### Step 1: Choose a Template

Start from an existing engine:

```bash
# For building binaries
cp -r cmd/build-go cmd/<your-engine-name>

# For building containers
cp -r cmd/build-container cmd/<your-engine-name>

# For formatters/generators
cp -r cmd/format-go cmd/<your-engine-name>
```

Update the Name constant:
```go
const Name = "<your-engine-name>"
```

### Step 2: Define Your BuildSpec

The BuildSpec comes from `forge.yaml`:

```yaml
build:
  - name: my-artifact
    src: ./src/path
    dest: ./build/output
    engine: go://<your-engine-name>
    # Add custom fields as needed
```

You can extend BuildInput in your engine:

```go
type CustomBuildInput struct {
    mcptypes.BuildInput
    // Add custom fields
    CustomOption string `json:"customOption"`
    Flags        []string `json:"flags"`
}
```

### Step 3: Implement Build Logic

Create the core build function:

```go
func buildArtifact(input mcptypes.BuildInput, version string) (*forge.Artifact, error) {
    startTime := time.Now()

    // 1. Validate inputs
    if input.Src == "" {
        return nil, fmt.Errorf("source path required")
    }

    // 2. Prepare build environment
    if err := prepareBuildEnv(input); err != nil {
        return nil, fmt.Errorf("failed to prepare: %w", err)
    }

    // 3. Execute build operation
    output, err := executeBuild(input)
    if err != nil {
        return nil, fmt.Errorf("build failed: %w", err)
    }

    // 4. Create artifact
    artifact := &forge.Artifact{
        Name:      input.Name,
        Type:      "binary", // or "container", "formatted", etc.
        Location:  filepath.Join(input.Dest, input.Name),
        Timestamp: startTime.Format(time.RFC3339),
        Version:   version,
    }

    return artifact, nil
}
```

### Step 4: Implement MCP Server

Follow this pattern:

```go
func runMCPServer() error {
    server := mcpserver.New(Name, Version)

    // Register build tool
    mcpserver.RegisterTool(server, &mcp.Tool{
        Name:        "build",
        Description: "Build a single artifact",
    }, handleBuildTool)

    // Register buildBatch tool
    mcpserver.RegisterTool(server, &mcp.Tool{
        Name:        "buildBatch",
        Description: "Build multiple artifacts",
    }, handleBuildBatchTool)

    return server.RunDefault()
}

func handleBuildTool(
    ctx context.Context,
    req *mcp.CallToolRequest,
    input mcptypes.BuildInput,
) (*mcp.CallToolResult, any, error) {
    // 1. Validate input
    if result := mcputil.ValidateRequiredWithPrefix("Build failed", map[string]string{
        "name": input.Name,
        "src":  input.Src,
    }); result != nil {
        return result, nil, nil
    }

    // 2. Get version
    version, err := gitutil.GetCurrentCommitSHA()
    if err != nil {
        return mcputil.ErrorResult(fmt.Sprintf("Build failed: %v", err)), nil, nil
    }

    // 3. Build artifact
    artifact, err := buildArtifact(input, version)
    if err != nil {
        return mcputil.ErrorResult(fmt.Sprintf("Build failed: %v", err)), nil, nil
    }

    // 4. Return result
    result, returnedArtifact := mcputil.SuccessResultWithArtifact(
        fmt.Sprintf("Built %s successfully", input.Name),
        artifact,
    )
    return result, returnedArtifact, nil
}
```

### Step 5: Implement Batch Building

```go
func handleBuildBatchTool(
    ctx context.Context,
    req *mcp.CallToolRequest,
    input mcptypes.BatchBuildInput,
) (*mcp.CallToolResult, any, error) {
    log.Printf("Building %d artifacts in batch", len(input.Specs))

    // Use the generic batch handler
    artifacts, errorMsgs := mcputil.HandleBatchBuild(ctx, input.Specs,
        func(ctx context.Context, spec mcptypes.BuildInput) (*mcp.CallToolResult, any, error) {
            return handleBuildTool(ctx, req, spec)
        },
    )

    // Format and return
    result, returnedArtifacts := mcputil.FormatBatchResult("artifacts", artifacts, errorMsgs)
    return result, returnedArtifacts, nil
}
```

### Step 6: Add CLI Mode (Optional)

If you want direct CLI usage:

```go
func run() error {
    // Read forge.yaml
    config, err := forge.ReadSpec()
    if err != nil {
        return err
    }

    // Build all artifacts defined in config
    for _, spec := range config.Build {
        if spec.Engine != "go://"+Name {
            continue
        }

        artifact, err := buildArtifact(spec, version)
        if err != nil {
            return err
        }

        // Store in artifact store
        storeArtifact(artifact)
    }

    return nil
}
```

### Step 7: Store Artifacts

```go
func storeArtifact(artifact *forge.Artifact) error {
    artifactStorePath, _ := forge.GetArtifactStorePath(".forge/artifacts.yaml")
    store, _ := forge.ReadOrCreateArtifactStore(artifactStorePath)

    forge.AddOrUpdateArtifact(&store, *artifact)

    return forge.WriteArtifactStore(artifactStorePath, store)
}
```

### Step 8: Configure in forge.yaml

```yaml
build:
  - name: my-app
    src: ./cmd/my-app
    dest: ./build/bin
    engine: go://<your-engine-name>
```

## Common Patterns

### Pattern 1: Building Go Binaries

```go
func buildGoBinary(input mcptypes.BuildInput) error {
    cmd := exec.Command("go", "build",
        "-o", filepath.Join(input.Dest, input.Name),
        input.Src,
    )

    output, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("go build failed: %s", output)
    }

    return nil
}
```

### Pattern 2: Building Containers

```go
func buildContainer(input mcptypes.BuildInput, tag string) error {
    cmd := exec.Command("docker", "build",
        "-t", fmt.Sprintf("%s:%s", input.Name, tag),
        "-f", filepath.Join(input.Src, "Dockerfile"),
        input.Src,
    )

    return cmd.Run()
}
```

### Pattern 3: Code Generation

```go
func generateCode(input mcptypes.BuildInput) error {
    // Read template
    tmpl, err := template.ParseFiles(input.Src)

    // Generate output
    output, err := os.Create(filepath.Join(input.Dest, input.Name))
    defer output.Close()

    // Execute template
    return tmpl.Execute(output, data)
}
```

### Pattern 4: Code Formatting

```go
func formatCode(input mcptypes.BuildInput) error {
    cmd := exec.Command("gofmt", "-w", input.Src)
    return cmd.Run()
}
```

## Best Practices

1. **Version Everything**: Use git SHA for version field
2. **Validate Early**: Check inputs before starting expensive operations
3. **Error Context**: Provide detailed error messages
4. **Idempotency**: Builds should be repeatable
5. **Clean Artifacts**: Place outputs in predictable locations
6. **Progress Logging**: Use log.Printf for progress (goes to stderr in MCP mode)
7. **Artifact Metadata**: Include accurate location, type, and timestamp

## Testing Your Engine

```bash
# Build the engine
forge build <your-engine-name>

# Test via MCP (recommended)
# Forge will call your engine via MCP
forge build my-artifact

# Test directly (if you implemented CLI mode)
./build/bin/<your-engine-name> build
```

## Debugging

Enable MCP debug logging:

```bash
# Set environment variable
export MCP_DEBUG=1

# Run build
forge build my-artifact

# Check MCP communication in stderr
```

## Examples

- **build-go**: Builds Go binaries using `go build`
- **build-container**: Builds containers using Kaniko or Docker
- **format-go**: Formats Go code using `gofmt`
- **generate-mocks**: Generates mocks using mockgen

## Integration with Forge

Once your engine is ready:

```bash
# Build using your engine
forge build my-artifact

# Build all artifacts
forge build
```

## Need Help?

- Review `cmd/build-go` for a complete working example
- Check the Artifact structure in `pkg/forge/artifact_store.go`
- Use `pkg/mcputil` helpers for common MCP patterns
- The forge CLI handles MCP communication - focus on your build logic
