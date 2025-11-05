# Engine Implementation Guide

## Overview

An **engine** in forge is a component that performs build operations. Engines are responsible for transforming source code into artifacts - whether that's compiling binaries, building containers, formatting code, or generating files. This guide covers how to implement custom engines from scratch.

## What is an Engine?

Engines are executables that implement a specific interface to perform build operations. They communicate with forge via the Model Context Protocol (MCP), receive build specifications, execute operations, and report results back as artifacts.

### Engine vs Test Engine vs Test Runner

| Type | Purpose | Used In | Output |
|------|---------|---------|--------|
| **Engine** | Build operations | `build:` specs | Artifact |
| **Test Engine** | Manage test environments | `test:` specs (engine field) | TestEnvironment |
| **Test Runner** | Execute tests | `test:` specs (runner field) | TestReport |

**This guide** covers **engines** (build operations). For test-related components, see:
- [Test Engine Guide](./test-engine-guide.md) - Environment management
- [Test Runner Guide](./test-runner-guide.md) - Test execution

## When to Write a Custom Engine

**Write a custom engine when**:
- ✅ Generic engines are too limited
- ✅ You need complex build logic
- ✅ You need to interact with APIs
- ✅ You need rich artifact metadata
- ✅ You need advanced error handling
- ✅ You need build caching or optimization

**Use generic engines when**:
- ✅ You're wrapping a CLI tool
- ✅ Build logic is simple
- ✅ Exit code is sufficient for error handling
- ✅ See [Generic Engine Guide](./generic-engine-guide.md)

## API Contract

### CLI Interface

Engines must support the following command-line interface:

```bash
# Build operation (optional, for direct CLI use)
<engine-binary> build [options]
# Output: Human-readable progress to stderr, success/failure exit code

# Version information
<engine-binary> version
<engine-binary> --version
<engine-binary> -v

# Help text
<engine-binary> help
<engine-binary> --help
<engine-binary> -h

# MCP server mode (REQUIRED)
<engine-binary> --mcp
```

**Note**: Only `--mcp` mode is required for forge integration. CLI commands are optional for convenience.

### MCP Interface

All engines MUST support MCP mode via the `--mcp` flag. This is how forge communicates with engines.

#### Required MCP Tool: `build`

```json
{
  "name": "build",
  "description": "Build an artifact from source",
  "inputSchema": {
    "type": "object",
    "properties": {
      "name": {
        "type": "string",
        "description": "Artifact name"
      },
      "src": {
        "type": "string",
        "description": "Source path or directory"
      },
      "dest": {
        "type": "string",
        "description": "Destination path for output"
      }
    },
    "required": ["name", "src"]
  }
}
```

**Additional Fields**: Engines can accept additional fields in the input schema for engine-specific configuration.

**Response**:
- **Success**:
  - Content: Success message (text)
  - Meta: `Artifact` object (see Data Structures below)
  - IsError: false

- **Error**:
  - Content: Error message (text)
  - IsError: true

#### Optional MCP Tool: `buildBatch`

For performance, engines can optionally implement batch building:

```json
{
  "name": "buildBatch",
  "description": "Build multiple artifacts in one operation",
  "inputSchema": {
    "type": "object",
    "properties": {
      "specs": {
        "type": "array",
        "items": {
          "type": "object",
          "properties": {
            "name": { "type": "string" },
            "src": { "type": "string" },
            "dest": { "type": "string" }
          }
        }
      }
    },
    "required": ["specs"]
  }
}
```

**Response**: Array of `Artifact` objects in Meta

## Data Structures

### Artifact

The standard artifact structure defined in `pkg/forge/artifact_store.go`:

```go
type Artifact struct {
    // Name is the artifact identifier
    Name string `json:"name"`

    // Type describes the artifact kind
    // Examples: "binary", "container", "formatted-code", "generated-code"
    Type string `json:"type"`

    // Location is the path or identifier where artifact can be found
    // For files: absolute or relative path
    // For containers: image name/tag
    Location string `json:"location"`

    // Timestamp when artifact was created (RFC3339 format)
    Timestamp string `json:"timestamp"`

    // Version identifier (git commit, semantic version, etc.)
    Version string `json:"version"`

    // Metadata holds engine-specific data
    Metadata map[string]string `json:"metadata,omitempty"`
}
```

### BuildSpec

The input from forge.yaml (defined in `pkg/forge/spec_build.go`):

```go
type BuildSpec struct {
    // Name of the artifact to build
    Name string `json:"name"`

    // Src is the source path (e.g., "./cmd/myapp", "./Containerfile")
    Src string `json:"src"`

    // Dest is the output path (e.g., "./build/bin")
    Dest string `json:"dest,omitempty"`

    // Engine is the engine URI (e.g., "go://build-go")
    Engine string `json:"engine"`
}
```

**Note**: When using generic engines with aliases, additional fields (command, args, env, etc.) are injected by forge.

## Implementation Pattern

### Directory Structure

```
cmd/my-engine/
├── main.go           # Entry point, CLI routing
├── build.go          # Build operation logic
├── mcp.go            # MCP server implementation
├── main_test.go      # Unit tests
└── README.md         # Documentation
```

### Step 1: Create main.go

```go
package main

import (
    "fmt"
    "log"
    "os"

    "github.com/alexandremahdhaoui/forge/internal/version"
)

// Version information (set via ldflags during build)
var (
    Version        = "dev"
    CommitSHA      = "unknown"
    BuildTimestamp = "unknown"
)

var versionInfo *version.Info

func init() {
    versionInfo = version.New("my-engine")
    versionInfo.Version = Version
    versionInfo.CommitSHA = CommitSHA
    versionInfo.BuildTimestamp = BuildTimestamp
}

func main() {
    if len(os.Args) < 2 {
        printUsage()
        os.Exit(1)
    }

    command := os.Args[1]

    switch command {
    case "--mcp":
        if err := runMCPServer(); err != nil {
            log.Printf("MCP server error: %v", err)
            os.Exit(1)
        }
    case "build":
        // Optional: implement direct CLI build
        if err := cmdBuild(os.Args[2:]); err != nil {
            fmt.Fprintf(os.Stderr, "Error: %v\n", err)
            os.Exit(1)
        }
    case "version", "--version", "-v":
        versionInfo.Print()
    case "help", "--help", "-h":
        printUsage()
    default:
        fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
        printUsage()
        os.Exit(1)
    }
}

func printUsage() {
    fmt.Print(`my-engine - Custom build engine

Usage:
  my-engine --mcp         Run as MCP server
  my-engine build         Build artifacts (if implementing direct CLI)
  my-engine version       Show version information
  my-engine help          Show this help message

Description:
  my-engine is a custom forge engine that [describe what it does].

Examples:
  # Run as MCP server (used by forge)
  my-engine --mcp

  # Show version
  my-engine version
`)
}
```

### Step 2: Implement build.go

```go
package main

import (
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "time"

    "github.com/alexandremahdhaoui/forge/pkg/forge"
)

// BuildInput represents the parameters for building
type BuildInput struct {
    Name string `json:"name"`
    Src  string `json:"src"`
    Dest string `json:"dest,omitempty"`

    // Add engine-specific fields here
    // Example: BuildTags []string `json:"buildTags,omitempty"`
}

// performBuild executes the actual build operation
func performBuild(input BuildInput) (*forge.Artifact, error) {
    // 1. Validate inputs
    if input.Name == "" {
        return nil, fmt.Errorf("artifact name is required")
    }
    if input.Src == "" {
        return nil, fmt.Errorf("source path is required")
    }

    // 2. Prepare build environment
    // Example: Create temp directories, set up paths, etc.

    // 3. Execute build operation
    // This is where your custom logic goes

    // Example: Compile a Go binary
    outputPath := filepath.Join(input.Dest, input.Name)
    cmd := exec.Command("go", "build", "-o", outputPath, input.Src)
    cmd.Stdout = os.Stderr  // Send output to stderr for logging
    cmd.Stderr = os.Stderr

    if err := cmd.Run(); err != nil {
        return nil, fmt.Errorf("build failed: %w", err)
    }

    // 4. Determine artifact properties
    artifactType := "binary"  // Or "container", "formatted-code", etc.
    artifactLocation := outputPath

    // Get version information (example: from git)
    version := getVersionString()

    // 5. Create artifact record
    artifact := &forge.Artifact{
        Name:      input.Name,
        Type:      artifactType,
        Location:  artifactLocation,
        Timestamp: time.Now().UTC().Format(time.RFC3339),
        Version:   version,
        Metadata:  make(map[string]string),
    }

    // 6. Add engine-specific metadata
    artifact.Metadata["source"] = input.Src
    artifact.Metadata["engine"] = "my-engine"
    // Add more metadata as needed

    return artifact, nil
}

// getVersionString determines the version for the artifact
func getVersionString() string {
    // Example: Use git commit hash
    cmd := exec.Command("git", "rev-parse", "HEAD")
    output, err := cmd.Output()
    if err != nil {
        return "unknown"
    }
    return string(output[:8])  // First 8 chars of commit hash
}

// cmdBuild implements optional direct CLI build
func cmdBuild(args []string) error {
    // Parse arguments, create BuildInput, call performBuild
    // This is optional - only needed if you want direct CLI usage
    return fmt.Errorf("direct CLI build not implemented (use --mcp mode)")
}
```

### Step 3: Implement mcp.go

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/alexandremahdhaoui/forge/internal/mcpserver"
    "github.com/alexandremahdhaoui/forge/pkg/forge"
    "github.com/modelcontextprotocol/go-sdk/mcp"
)

// runMCPServer starts the MCP server
func runMCPServer() error {
    v, _, _ := versionInfo.Get()
    server := mcpserver.New("my-engine", v)

    // Register build tool
    mcpserver.RegisterTool(server, &mcp.Tool{
        Name:        "build",
        Description: "Build an artifact from source",
    }, handleBuildTool)

    // Optional: Register buildBatch tool for performance
    mcpserver.RegisterTool(server, &mcp.Tool{
        Name:        "buildBatch",
        Description: "Build multiple artifacts in batch",
    }, handleBuildBatchTool)

    return server.RunDefault()
}

// handleBuildTool handles the "build" MCP tool
func handleBuildTool(
    ctx context.Context,
    req *mcp.CallToolRequest,
    input BuildInput,
) (*mcp.CallToolResult, any, error) {
    log.Printf("Building: name=%s src=%s", input.Name, input.Src)

    // Perform the build
    artifact, err := performBuild(input)
    if err != nil {
        return &mcp.CallToolResult{
            Content: []mcp.Content{
                &mcp.TextContent{Text: fmt.Sprintf("Build failed: %v", err)},
            },
            IsError: true,
        }, nil, nil
    }

    // Return success with artifact
    return &mcp.CallToolResult{
        Content: []mcp.Content{
            &mcp.TextContent{
                Text: fmt.Sprintf("✅ Built %s: %s", artifact.Type, artifact.Name),
            },
        },
    }, artifact, nil
}

// BatchBuildInput for batch operations
type BatchBuildInput struct {
    Specs []BuildInput `json:"specs"`
}

// handleBuildBatchTool handles batch building (optional)
func handleBuildBatchTool(
    ctx context.Context,
    req *mcp.CallToolRequest,
    input BatchBuildInput,
) (*mcp.CallToolResult, any, error) {
    log.Printf("Building %d artifacts in batch", len(input.Specs))

    artifacts := []forge.Artifact{}

    for _, spec := range input.Specs {
        artifact, err := performBuild(spec)
        if err != nil {
            return &mcp.CallToolResult{
                Content: []mcp.Content{
                    &mcp.TextContent{Text: fmt.Sprintf("Batch build failed on %s: %v", spec.Name, err)},
                },
                IsError: true,
            }, nil, nil
        }
        artifacts = append(artifacts, *artifact)
    }

    return &mcp.CallToolResult{
        Content: []mcp.Content{
            &mcp.TextContent{Text: fmt.Sprintf("✅ Built %d artifacts", len(artifacts))},
        },
    }, artifacts, nil
}
```

### Step 4: Add to forge.yaml

```yaml
build:
  # Add your engine to the build list
  - name: my-engine
    src: ./cmd/my-engine
    dest: ./build/bin
    engine: go://build-go

  # Use your engine
  - name: my-artifact
    src: ./cmd/my-app
    dest: ./build/bin
    engine: go://my-engine
```

### Step 5: Test Your Engine

```bash
# Build your engine
go run ./cmd/forge build my-engine

# Test MCP mode
./build/bin/my-engine --mcp &
MCP_PID=$!

# Send test request (using forge)
go run ./cmd/forge build my-artifact

# Kill MCP server
kill $MCP_PID

# Test version
./build/bin/my-engine version
```

## Advanced Topics

### Batch Building

Batch building improves performance by building multiple artifacts in one operation:

```go
func handleBuildBatchTool(
    ctx context.Context,
    req *mcp.CallToolRequest,
    input BatchBuildInput,
) (*mcp.CallToolResult, any, error) {
    // Shared setup (once for all builds)
    setupSharedResources()
    defer cleanupSharedResources()

    artifacts := []forge.Artifact{}

    for _, spec := range input.Specs {
        artifact, err := performBuild(spec)
        if err != nil {
            // Decide: fail fast or continue?
            return failedResult(err)
        }
        artifacts = append(artifacts, *artifact)
    }

    return successResult(artifacts)
}
```

**Benefits**:
- Amortize startup costs
- Shared resource usage
- Faster overall build times

### Artifact Metadata

Use metadata to store engine-specific information:

```go
artifact.Metadata = map[string]string{
    "compiler":     "go1.21",
    "build-tags":   "prod,linux",
    "optimization": "O2",
    "source-hash":  computeHash(input.Src),
    "dependencies": strings.Join(deps, ","),
}
```

This metadata appears in the artifact store and can be queried later.

### Error Handling

**Return structured errors**:

```go
if err := validateInput(input); err != nil {
    return &mcp.CallToolResult{
        Content: []mcp.Content{
            &mcp.TextContent{Text: fmt.Sprintf("Validation failed: %v", err)},
        },
        IsError: true,
    }, nil, nil
}
```

**Include context in errors**:

```go
if err := cmd.Run(); err != nil {
    return nil, fmt.Errorf("compilation failed for %s: %w\nOutput: %s",
        input.Name, err, stderr.String())
}
```

### Logging Best Practices

**DO**:
- ✅ Log to stderr (stdout is for MCP JSON-RPC)
- ✅ Use structured logging: `log.Printf("Building: name=%s src=%s", name, src)`
- ✅ Log important milestones
- ✅ Log warnings and errors

**DON'T**:
- ❌ Write to stdout (breaks MCP protocol)
- ❌ Log sensitive information (passwords, keys)
- ❌ Log excessively (slows down builds)

### Handling Build Failures

```go
func performBuild(input BuildInput) (*forge.Artifact, error) {
    cmd := exec.Command("my-compiler", args...)

    var stderr bytes.Buffer
    cmd.Stderr = &stderr

    if err := cmd.Run(); err != nil {
        // Include compilation errors in message
        return nil, fmt.Errorf("build failed: %w\nCompiler output:\n%s",
            err, stderr.String())
    }

    // Success path...
}
```

### Incremental Builds

Implement caching for faster rebuilds:

```go
func performBuild(input BuildInput) (*forge.Artifact, error) {
    cacheKey := computeCacheKey(input)

    if cached := checkCache(cacheKey); cached != nil {
        log.Printf("Using cached artifact: %s", input.Name)
        return cached, nil
    }

    // Perform actual build
    artifact, err := actualBuild(input)
    if err != nil {
        return nil, err
    }

    // Save to cache
    saveCache(cacheKey, artifact)

    return artifact, nil
}
```

### Working with External Tools

**Example: Calling external compiler**

```go
func compileWithExternalTool(input BuildInput) error {
    // Find tool in PATH
    toolPath, err := exec.LookPath("my-tool")
    if err != nil {
        return fmt.Errorf("my-tool not found in PATH: %w", err)
    }

    // Prepare command
    cmd := exec.Command(toolPath,
        "--input", input.Src,
        "--output", input.Dest,
    )

    // Set environment
    cmd.Env = append(os.Environ(),
        "TOOL_OPTION=value",
    )

    // Capture output
    output, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("compilation failed: %w\nOutput: %s", err, output)
    }

    return nil
}
```

## Testing Your Engine

### Unit Tests

Create `main_test.go`:

```go
package main

import (
    "testing"
)

func TestPerformBuild(t *testing.T) {
    input := BuildInput{
        Name: "test-artifact",
        Src:  "./testdata/src",
        Dest: "./testdata/dest",
    }

    artifact, err := performBuild(input)
    if err != nil {
        t.Fatalf("Build failed: %v", err)
    }

    if artifact.Name != input.Name {
        t.Errorf("Expected name %s, got %s", input.Name, artifact.Name)
    }

    if artifact.Type == "" {
        t.Error("Artifact type should not be empty")
    }
}

func TestBuildValidation(t *testing.T) {
    tests := []struct {
        name    string
        input   BuildInput
        wantErr bool
    }{
        {
            name: "missing name",
            input: BuildInput{
                Src: "./src",
            },
            wantErr: true,
        },
        {
            name: "missing src",
            input: BuildInput{
                Name: "artifact",
            },
            wantErr: true,
        },
        {
            name: "valid input",
            input: BuildInput{
                Name: "artifact",
                Src:  "./src",
            },
            wantErr: false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, err := performBuild(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("wantErr=%v, got err=%v", tt.wantErr, err)
            }
        })
    }
}
```

### Integration Tests

Test via forge:

```bash
# Create test forge.yaml
cat > test-forge.yaml <<EOF
name: test
artifactStorePath: .test-artifacts.json

build:
  - name: test-build
    src: ./testdata
    dest: ./test-output
    engine: go://my-engine
EOF

# Run test
forge build --config=test-forge.yaml

# Verify artifacts
test -f ./test-output/test-build || exit 1
```

## Best Practices

### 1. Input Validation

Always validate inputs before processing:

```go
func validateInput(input BuildInput) error {
    if input.Name == "" {
        return fmt.Errorf("name is required")
    }
    if input.Src == "" {
        return fmt.Errorf("src is required")
    }
    if !fileExists(input.Src) {
        return fmt.Errorf("source not found: %s", input.Src)
    }
    return nil
}
```

### 2. Deterministic Builds

Produce the same output for the same input:

```go
// ✅ Good: Deterministic timestamp
artifact.Timestamp = time.Now().UTC().Format(time.RFC3339)

// ✅ Good: Reproducible version
artifact.Version = getGitCommit()

// ❌ Bad: Random elements
artifact.Version = generateRandomID()
```

### 3. Resource Cleanup

Always clean up temporary resources:

```go
func performBuild(input BuildInput) (*forge.Artifact, error) {
    tmpDir, err := os.MkdirTemp("", "build-*")
    if err != nil {
        return nil, err
    }
    defer os.RemoveAll(tmpDir)  // Always cleanup

    // Build logic...
}
```

### 4. Artifact Locations

Use absolute paths for artifacts:

```go
outputPath := filepath.Join(input.Dest, input.Name)
absPath, err := filepath.Abs(outputPath)
if err != nil {
    return nil, err
}

artifact.Location = absPath  // Absolute path
```

### 5. Version Information

Use `internal/version` for consistency:

```go
import "github.com/alexandremahdhaoui/forge/internal/version"

var versionInfo = version.New("my-engine")

func init() {
    versionInfo.Version = Version
    versionInfo.CommitSHA = CommitSHA
    versionInfo.BuildTimestamp = BuildTimestamp
}
```

### 6. Error Messages

Provide actionable error messages:

```go
// ❌ Bad: Vague
return fmt.Errorf("build failed")

// ✅ Good: Specific with context
return fmt.Errorf("build failed for %s: compilation error at line 42: undefined variable 'foo'", input.Name)
```

### 7. Documentation

Document your engine:

```go
// BuildInput represents the parameters for building.
//
// Fields:
//   - Name: Required. The artifact name.
//   - Src: Required. Path to source directory or file.
//   - Dest: Optional. Output directory. Defaults to "./build".
//   - BuildTags: Optional. Go build tags to use.
//
// Example:
//   input := BuildInput{
//       Name: "myapp",
//       Src:  "./cmd/myapp",
//       Dest: "./build/bin",
//       BuildTags: []string{"prod", "linux"},
//   }
type BuildInput struct {
    Name      string   `json:"name"`
    Src       string   `json:"src"`
    Dest      string   `json:"dest,omitempty"`
    BuildTags []string `json:"buildTags,omitempty"`
}
```

## Common Patterns

### Pattern 1: Multi-Stage Builds

```go
func performBuild(input BuildInput) (*forge.Artifact, error) {
    // Stage 1: Prepare
    if err := prepareSource(input.Src); err != nil {
        return nil, fmt.Errorf("prepare failed: %w", err)
    }

    // Stage 2: Compile
    compiled, err := compile(input)
    if err != nil {
        return nil, fmt.Errorf("compile failed: %w", err)
    }

    // Stage 3: Post-process
    final, err := postProcess(compiled)
    if err != nil {
        return nil, fmt.Errorf("post-process failed: %w", err)
    }

    return createArtifact(final), nil
}
```

### Pattern 2: Conditional Building

```go
func performBuild(input BuildInput) (*forge.Artifact, error) {
    // Check if rebuild is needed
    if !needsRebuild(input) {
        log.Printf("Skipping %s: up to date", input.Name)
        return loadExistingArtifact(input.Name)
    }

    // Perform build
    return actualBuild(input)
}

func needsRebuild(input BuildInput) bool {
    // Check timestamps, hashes, etc.
    sourceTime := getModTime(input.Src)
    artifactTime := getModTime(input.Dest)
    return sourceTime.After(artifactTime)
}
```

### Pattern 3: Parallel Building

```go
func handleBuildBatchTool(..., input BatchBuildInput) (*mcp.CallToolResult, any, error) {
    type result struct {
        artifact *forge.Artifact
        err      error
    }

    results := make(chan result, len(input.Specs))

    // Build in parallel
    for _, spec := range input.Specs {
        go func(s BuildInput) {
            artifact, err := performBuild(s)
            results <- result{artifact, err}
        }(spec)
    }

    // Collect results
    artifacts := []forge.Artifact{}
    for range input.Specs {
        r := <-results
        if r.err != nil {
            return errorResult(r.err)
        }
        artifacts = append(artifacts, *r.artifact)
    }

    return successResult(artifacts)
}
```

## Troubleshooting

### MCP Server Not Starting

**Symptoms**: `forge build` hangs or times out

**Solutions**:
1. Test MCP mode manually: `./my-engine --mcp`
2. Check logs: stderr should show "MCP server started"
3. Verify tool registration: Tool name must be "build"

### Artifacts Not Appearing

**Symptoms**: Build succeeds but artifacts missing from store

**Cause**: Engine not returning artifact in Meta field

**Solution**:
```go
// ✅ Correct: Return artifact in Meta
return &mcp.CallToolResult{
    Content: []mcp.Content{...},
}, artifact, nil

// ❌ Wrong: Missing artifact
return &mcp.CallToolResult{
    Content: []mcp.Content{...},
}, nil, nil
```

### Build Failures Silent

**Symptoms**: Build fails but no error shown

**Cause**: Not setting IsError: true

**Solution**:
```go
return &mcp.CallToolResult{
    Content: []mcp.Content{
        &mcp.TextContent{Text: errorMsg},
    },
    IsError: true,  // Must set this!
}, nil, nil
```

## Reference Implementations

Study these engines for examples:

1. **cmd/build-go** - Compiles Go binaries
   - Shows: Basic compilation, batch building, versioning

2. **cmd/build-container** - Builds container images
   - Shows: External tool integration (kaniko), complex metadata

3. **cmd/format-go** - Formats Go code
   - Shows: In-place modifications, code formatting

4. **cmd/generic-engine** - Generic command executor
   - Shows: Configuration-driven, flexible execution

## Summary Checklist

Before shipping your engine, verify:

- [ ] Implements `--mcp` flag
- [ ] Registers "build" MCP tool
- [ ] Returns `Artifact` in Meta field
- [ ] Sets `IsError: true` on failures
- [ ] Logs to stderr only (never stdout)
- [ ] Validates all inputs
- [ ] Handles errors gracefully
- [ ] Includes version information
- [ ] Has unit tests
- [ ] Has usage documentation
- [ ] Cleans up temporary resources
- [ ] Returns absolute paths in Location
- [ ] Sets appropriate Artifact.Type
- [ ] Includes useful metadata

## Next Steps

- Read [Generic Engine Guide](./generic-engine-guide.md) for simpler alternatives
- Read [Test Engine Guide](./test-engine-guide.md) for environment management
- Read [Test Runner Guide](./test-runner-guide.md) for test execution
- Study reference implementations in `cmd/build-go` and `cmd/generic-engine`

## Conclusion

Writing a custom engine gives you full control over build operations. While more complex than generic engines, custom engines enable:

✅ Advanced build logic
✅ API integrations
✅ Rich artifact metadata
✅ Build caching and optimization
✅ Complex error handling
✅ Tool orchestration

Start with the template in this guide, study reference implementations, and iterate based on your needs.
