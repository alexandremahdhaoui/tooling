# Creating a Custom Testenv Orchestrator

You are helping a user create a custom testenv orchestrator for forge. A testenv orchestrator composes multiple testenv subengines to create and manage complete test environments.

## What is a Testenv Orchestrator?

A **testenv orchestrator** is a component that:
- Composes multiple testenv subengines to create complete test environments
- Manages a shared tmpDir for file isolation across subengines
- Coordinates create/delete operations across subengines
- Aggregates metadata, files, and managed resources from all subengines
- Stores the final TestEnvironment in the artifact store

The default `testenv` orchestrator is sufficient for most use cases. Create a custom orchestrator only when you need specialized composition logic.

## When to Create a Custom Testenv Orchestrator

Create a custom testenv orchestrator when you need:
- ✅ Complex conditional logic for subengine selection
- ✅ Dynamic subengine ordering based on configuration
- ✅ Advanced error handling or retry logic
- ✅ Custom resource allocation or cleanup strategies
- ✅ Integration with external provisioning systems
- ✅ Multi-cloud or multi-environment orchestration

## When to Use the Default Testenv

Use the built-in `testenv` orchestrator when:
- ✅ You can express your environment as a list of subengines
- ✅ Subengines run in a fixed order
- ✅ No conditional logic is needed
- ✅ Standard cleanup (reverse order) is sufficient

**Most users should use the default testenv and create custom subengines instead.**

## API Contract

### MCP Interface (Required)

Your testenv orchestrator **must** implement these MCP tools:

#### `create` Tool

Create a complete test environment by orchestrating subengines.

**Input Schema:**
```json
{
  "stage": "string (required)"       // Test stage name (e.g., "integration", "e2e")
}
```

**Output Schema:**
```json
{
  "testID": "string"                 // Unique test environment ID
}
```

**Responsibilities:**
1. Generate unique test ID (format: `test-<stage>-YYYYMMDD-XXXXXXXX`)
2. Create tmpDir at `.forge/tmp/<testID>/`
3. Call subengines in order with (testID, stage, tmpDir)
4. Aggregate files, metadata, and managedResources from all subengines
5. Create TestEnvironment and store in artifact store
6. Return testID

#### `delete` Tool

Delete a test environment and clean up all resources.

**Input Schema:**
```json
{
  "testID": "string (required)"      // Test environment ID to delete
}
```

**Output Schema:**
```json
{
  "success": true,
  "message": "Deleted test environment: test-integration-20250106-abc123"
}
```

**Responsibilities:**
1. Read TestEnvironment from artifact store
2. Call subengines in **reverse order** for cleanup
3. Remove tmpDir
4. Delete TestEnvironment from artifact store

## TestEnvironment Structure

Your orchestrator creates and manages this structure:

```go
type TestEnvironment struct {
    ID               string            `json:"id"`               // test-stage-20250106-abc123
    Name             string            `json:"name"`             // Test stage name
    Status           string            `json:"status"`           // "created", "running", etc.
    CreatedAt        time.Time         `json:"createdAt"`
    UpdatedAt        time.Time         `json:"updatedAt"`
    TmpDir           string            `json:"tmpDir"`           // Shared tmpDir path
    Files            map[string]string `json:"files"`            // Logical name -> filename
    Metadata         map[string]string `json:"metadata"`         // Key-value pairs
    ManagedResources []string          `json:"managedResources"` // Resources to clean up
}
```

## Implementation Steps

### Step 1: Choose a Template

Start from the existing testenv:

```bash
# Copy the reference implementation
cp -r cmd/testenv cmd/testenv-<your-name>

# Update the Name constant in main.go
const Name = "testenv-<your-name>"
```

### Step 2: Define Subengine Configuration

In your `forge.yaml`:

```yaml
test:
  - name: integration
    testenv:
      engine: go://testenv-<your-name>
      subengines:
        - go://testenv-kind
        - go://testenv-lcr
        - go://testenv-postgres
```

Or implement custom logic to dynamically select subengines.

### Step 3: Implement Create Operation

```go
package main

import (
    "crypto/rand"
    "encoding/hex"
    "fmt"
    "os"
    "path/filepath"
    "time"

    "github.com/alexandremahdhaoui/forge/pkg/forge"
)

type CreateInput struct {
    Stage string `json:"stage"`
}

type CreateOutput struct {
    TestID string `json:"testID"`
}

func handleCreate(input CreateInput) (*CreateOutput, error) {
    // 1. Validate input
    if input.Stage == "" {
        return nil, fmt.Errorf("stage is required")
    }

    // 2. Generate unique test ID
    testID := generateTestID(input.Stage)

    // 3. Create tmpDir
    tmpDir := filepath.Join(".forge", "tmp", testID)
    if err := os.MkdirAll(tmpDir, 0755); err != nil {
        return nil, fmt.Errorf("failed to create tmpDir: %w", err)
    }

    // 4. Initialize TestEnvironment
    testEnv := &forge.TestEnvironment{
        ID:               testID,
        Name:             input.Stage,
        Status:           "created",
        CreatedAt:        time.Now().UTC(),
        UpdatedAt:        time.Now().UTC(),
        TmpDir:           tmpDir,
        Files:            make(map[string]string),
        Metadata:         make(map[string]string),
        ManagedResources: []string{tmpDir},
    }

    // 5. Get subengine list (from config or custom logic)
    subengines, err := getSubengines(input.Stage)
    if err != nil {
        return nil, fmt.Errorf("failed to get subengines: %w", err)
    }

    // 6. Call each subengine in order
    for _, subengine := range subengines {
        result, err := callSubengineCreate(subengine, testID, input.Stage, tmpDir)
        if err != nil {
            // Cleanup on failure
            cleanup(testEnv, subengines[:indexOf(subengine)])
            return nil, fmt.Errorf("subengine %s failed: %w", subengine, err)
        }

        // 7. Aggregate results
        for fileKey, fileName := range result.Files {
            testEnv.Files[fileKey] = fileName
        }
        for metaKey, metaValue := range result.Metadata {
            testEnv.Metadata[metaKey] = metaValue
        }
        testEnv.ManagedResources = append(testEnv.ManagedResources, result.ManagedResources...)
    }

    // 8. Store in artifact store
    if err := storeTestEnvironment(testEnv); err != nil {
        cleanup(testEnv, subengines)
        return nil, fmt.Errorf("failed to store test environment: %w", err)
    }

    return &CreateOutput{TestID: testID}, nil
}

func generateTestID(stage string) string {
    randBytes := make([]byte, 4)
    rand.Read(randBytes)
    suffix := hex.EncodeToString(randBytes)
    dateStr := time.Now().Format("20060102")
    return fmt.Sprintf("test-%s-%s-%s", stage, dateStr, suffix)
}
```

### Step 4: Implement Delete Operation

```go
type DeleteInput struct {
    TestID string `json:"testID"`
}

type DeleteOutput struct {
    Success bool   `json:"success"`
    Message string `json:"message"`
}

func handleDelete(input DeleteInput) (*DeleteOutput, error) {
    // 1. Validate input
    if input.TestID == "" {
        return nil, fmt.Errorf("testID is required")
    }

    // 2. Retrieve TestEnvironment from artifact store
    testEnv, err := getTestEnvironment(input.TestID)
    if err != nil {
        return nil, fmt.Errorf("test environment not found: %w", err)
    }

    // 3. Get subengine list
    subengines, err := getSubengines(testEnv.Name)
    if err != nil {
        return nil, fmt.Errorf("failed to get subengines: %w", err)
    }

    // 4. Call subengines in REVERSE order for cleanup
    for i := len(subengines) - 1; i >= 0; i-- {
        subengine := subengines[i]
        if err := callSubengineDelete(subengine, input.TestID); err != nil {
            // Log but continue - best effort cleanup
            log.Printf("Warning: subengine %s delete failed: %v", subengine, err)
        }
    }

    // 5. Remove tmpDir
    if testEnv.TmpDir != "" {
        if err := os.RemoveAll(testEnv.TmpDir); err != nil {
            log.Printf("Warning: failed to remove tmpDir %s: %v", testEnv.TmpDir, err)
        }
    }

    // 6. Remove from artifact store
    if err := deleteTestEnvironmentFromStore(input.TestID); err != nil {
        log.Printf("Warning: failed to remove from artifact store: %v", err)
    }

    return &DeleteOutput{
        Success: true,
        Message: fmt.Sprintf("Deleted test environment: %s", input.TestID),
    }, nil
}
```

### Step 5: Implement Subengine Communication

```go
type SubengineResult struct {
    TestID           string            `json:"testID"`
    Files            map[string]string `json:"files"`
    Metadata         map[string]string `json:"metadata"`
    ManagedResources []string          `json:"managedResources"`
}

func callSubengineCreate(subengine, testID, stage, tmpDir string) (*SubengineResult, error) {
    // Build MCP input
    input := map[string]any{
        "testID": testID,
        "stage":  stage,
        "tmpDir": tmpDir,
    }

    // Call subengine via MCP
    result, err := callMCPTool(subengine, "create", input)
    if err != nil {
        return nil, fmt.Errorf("MCP call failed: %w", err)
    }

    // Parse result
    var subengineResult SubengineResult
    if err := parseResult(result, &subengineResult); err != nil {
        return nil, fmt.Errorf("failed to parse result: %w", err)
    }

    return &subengineResult, nil
}

func callSubengineDelete(subengine, testID string) error {
    input := map[string]any{
        "testID": testID,
    }

    _, err := callMCPTool(subengine, "delete", input)
    return err
}

// Helper: Call MCP tool on a subengine
func callMCPTool(engine, tool string, args map[string]any) (any, error) {
    // Resolve engine path
    enginePath, err := resolveEngine(engine)
    if err != nil {
        return nil, err
    }

    // Start MCP server
    cmd := exec.Command(enginePath, "--mcp")
    // ... setup stdin/stdout pipes
    // ... send MCP request
    // ... read MCP response

    return response, nil
}
```

### Step 6: Implement Artifact Store Integration

```go
func storeTestEnvironment(testEnv *forge.TestEnvironment) error {
    // Read forge.yaml to get artifact store path
    config, err := forge.ReadSpec()
    if err != nil {
        return err
    }

    artifactStorePath := config.ArtifactStorePath
    if artifactStorePath == "" {
        artifactStorePath = ".ignore.artifact-store.yaml"
    }

    // Read or create artifact store
    store, err := forge.ReadOrCreateArtifactStore(artifactStorePath)
    if err != nil {
        return err
    }

    // Add or update test environment
    forge.AddOrUpdateTestEnvironment(&store, testEnv)

    // Write back to disk
    return forge.WriteArtifactStore(artifactStorePath, store)
}

func getTestEnvironment(testID string) (*forge.TestEnvironment, error) {
    config, err := forge.ReadSpec()
    if err != nil {
        return nil, err
    }

    artifactStorePath := config.ArtifactStorePath
    if artifactStorePath == "" {
        artifactStorePath = ".ignore.artifact-store.yaml"
    }

    store, err := forge.ReadArtifactStore(artifactStorePath)
    if err != nil {
        return nil, err
    }

    return forge.GetTestEnvironment(&store, testID)
}

func deleteTestEnvironmentFromStore(testID string) error {
    config, err := forge.ReadSpec()
    if err != nil {
        return err
    }

    artifactStorePath := config.ArtifactStorePath
    if artifactStorePath == "" {
        artifactStorePath = ".ignore.artifact-store.yaml"
    }

    store, err := forge.ReadArtifactStore(artifactStorePath)
    if err != nil {
        return err
    }

    if err := forge.DeleteTestEnvironment(&store, testID); err != nil {
        return err
    }

    return forge.WriteArtifactStore(artifactStorePath, store)
}
```

### Step 7: Implement MCP Server

```go
package main

import (
    "context"
    "log"

    "github.com/alexandremahdhaoui/forge/internal/mcpserver"
    "github.com/modelcontextprotocol/go-sdk/mcp"
)

func runMCPServer() error {
    v, _, _ := versionInfo.Get()
    server := mcpserver.New("testenv-myorchestrator", v)

    mcpserver.RegisterTool(server, &mcp.Tool{
        Name:        "create",
        Description: "Create a test environment by orchestrating subengines",
    }, handleCreateTool)

    mcpserver.RegisterTool(server, &mcp.Tool{
        Name:        "delete",
        Description: "Delete a test environment",
    }, handleDeleteTool)

    return server.RunDefault()
}

func handleCreateTool(
    ctx context.Context,
    req *mcp.CallToolRequest,
    input CreateInput,
) (*mcp.CallToolResult, any, error) {
    log.Printf("Creating test environment: stage=%s", input.Stage)

    output, err := handleCreate(input)
    if err != nil {
        return &mcp.CallToolResult{
            Content: []mcp.Content{
                &mcp.TextContent{Text: fmt.Sprintf("Create failed: %v", err)},
            },
            IsError: true,
        }, nil, nil
    }

    return &mcp.CallToolResult{
        Content: []mcp.Content{
            &mcp.TextContent{Text: fmt.Sprintf("Created test environment: %s", output.TestID)},
        },
    }, output, nil
}

func handleDeleteTool(
    ctx context.Context,
    req *mcp.CallToolRequest,
    input DeleteInput,
) (*mcp.CallToolResult, any, error) {
    log.Printf("Deleting test environment: testID=%s", input.TestID)

    output, err := handleDelete(input)
    if err != nil {
        return &mcp.CallToolResult{
            Content: []mcp.Content{
                &mcp.TextContent{Text: fmt.Sprintf("Delete failed: %v", err)},
            },
            IsError: true,
        }, nil, nil
    }

    return &mcp.CallToolResult{
        Content: []mcp.Content{
            &mcp.TextContent{Text: output.Message},
        },
    }, output, nil
}
```

### Step 8: Configuration

Add to `forge.yaml`:

```yaml
name: my-project

test:
  - name: integration
    testenv:
      engine: go://testenv-<your-name>
      subengines:
        - go://testenv-kind
        - go://testenv-postgres
        - go://testenv-redis
    runner: go://test-runner-go
```

## Best Practices

### 1. Subengine Order Matters

Subengines run in order during create, **reverse order** during delete:

```go
// Create order
subengines := []string{
    "go://testenv-kind",      // 1. Create cluster
    "go://testenv-postgres",  // 2. Deploy database (needs cluster)
    "go://testenv-redis",     // 3. Deploy cache (needs cluster)
}

// Delete order (reverse)
for i := len(subengines) - 1; i >= 0; i-- {
    deleteSubengine(subengines[i])
}
```

### 2. Cleanup on Failure

If any subengine fails during create, clean up in reverse order:

```go
for i, subengine := range subengines {
    if err := createSubengine(subengine); err != nil {
        // Cleanup already-created subengines
        for j := i - 1; j >= 0; j-- {
            deleteSubengine(subengines[j])
        }
        return err
    }
}
```

### 3. Shared tmpDir

All subengines share the same tmpDir for file isolation:

```go
tmpDir := filepath.Join(".forge", "tmp", testID)
os.MkdirAll(tmpDir, 0755)

// Pass to all subengines
for _, subengine := range subengines {
    callSubengine(subengine, testID, stage, tmpDir)
}
```

### 4. Aggregate Metadata

Combine metadata from all subengines:

```go
testEnv := &forge.TestEnvironment{
    Files:            make(map[string]string),
    Metadata:         make(map[string]string),
    ManagedResources: []string{tmpDir},
}

for _, result := range subengineResults {
    // Merge files
    for key, val := range result.Files {
        testEnv.Files[key] = val
    }
    // Merge metadata
    for key, val := range result.Metadata {
        testEnv.Metadata[key] = val
    }
    // Append managed resources
    testEnv.ManagedResources = append(testEnv.ManagedResources, result.ManagedResources...)
}
```

### 5. Error Handling

Be resilient during delete:

```go
func handleDelete(input DeleteInput) (*DeleteOutput, error) {
    var errors []error

    // Best-effort cleanup - don't stop on errors
    for i := len(subengines) - 1; i >= 0; i-- {
        if err := deleteSubengine(subengines[i]); err != nil {
            errors = append(errors, err)
            log.Printf("Warning: %v", err)
            // Continue cleanup
        }
    }

    // Always try to remove tmpDir
    os.RemoveAll(tmpDir)

    // Always try to remove from artifact store
    deleteFromStore(testID)

    // Return success even if some cleanup failed
    return &DeleteOutput{Success: true}, nil
}
```

## Advanced Patterns

### Pattern 1: Conditional Subengines

Select subengines based on configuration:

```go
func getSubengines(stage string) ([]string, error) {
    config, err := forge.ReadSpec()
    if err != nil {
        return nil, err
    }

    testSpec := findTestSpec(config, stage)

    subengines := []string{"go://testenv-kind"}  // Always include kind

    // Conditionally add database
    if testSpec.NeedsDatabase {
        subengines = append(subengines, "go://testenv-postgres")
    }

    // Conditionally add registry
    if testSpec.NeedsRegistry {
        subengines = append(subengines, "go://testenv-lcr")
    }

    return subengines, nil
}
```

### Pattern 2: Parallel Subengines

Run independent subengines in parallel:

```go
func createIndependentSubengines(testID, stage, tmpDir string) error {
    // These don't depend on each other
    subengines := []string{
        "go://testenv-postgres",
        "go://testenv-redis",
        "go://testenv-s3mock",
    }

    var wg sync.WaitGroup
    errChan := make(chan error, len(subengines))

    for _, se := range subengines {
        wg.Add(1)
        go func(subengine string) {
            defer wg.Done()
            if err := callSubengineCreate(subengine, testID, stage, tmpDir); err != nil {
                errChan <- err
            }
        }(se)
    }

    wg.Wait()
    close(errChan)

    // Check for errors
    for err := range errChan {
        if err != nil {
            return err
        }
    }

    return nil
}
```

### Pattern 3: Retry Logic

Retry transient failures:

```go
func callSubengineWithRetry(subengine, testID, stage, tmpDir string, retries int) error {
    var err error
    for i := 0; i < retries; i++ {
        err = callSubengineCreate(subengine, testID, stage, tmpDir)
        if err == nil {
            return nil
        }

        if !isTransientError(err) {
            return err  // Don't retry permanent errors
        }

        log.Printf("Retry %d/%d for %s: %v", i+1, retries, subengine, err)
        time.Sleep(time.Duration(i+1) * time.Second)
    }
    return fmt.Errorf("failed after %d retries: %w", retries, err)
}
```

## Testing Your Orchestrator

```bash
# Build
forge build testenv-<your-name>

# Test via forge
forge test create-env integration
forge test list-env integration
forge test get-env integration <ENV_ID>
forge test delete-env integration <ENV_ID>

# Verify tmpDir was created
ls -la .forge/tmp/

# Verify artifact store
cat .ignore.artifact-store.yaml
```

## Integration with Forge

Forge calls your orchestrator like this:

```go
// forge calls your orchestrator via MCP
result := callMCPTool("go://testenv-<your-name>", "create", map[string]any{
    "stage": "integration",
})

testID := result["testID"]  // test-integration-20250106-abc123
```

## Documentation

Create `cmd/testenv-<your-name>/MCP.md`:

```markdown
# testenv-<your-name> MCP Server

Custom test environment orchestrator for [your use case].

## Purpose

[Description]

## Available Tools

### `create`
[Details]

### `delete`
[Details]

## Subengines

This orchestrator composes:
- testenv-kind
- testenv-postgres
- ...

## See Also

- [testenv MCP Server](../testenv/MCP.md)
```

## Reference Implementation

See `cmd/testenv` for the reference orchestrator implementation.

## When NOT to Create a Custom Orchestrator

**Don't create a custom orchestrator if:**
- ✅ You just need a new type of resource → Create a **subengine** instead
- ✅ Subengines always run in a fixed order → Use default testenv
- ✅ No complex conditional logic needed → Use default testenv

**Most users should create subengines, not orchestrators.**

## Summary

A testenv orchestrator must:
1. ✅ Implement MCP `create` and `delete` tools
2. ✅ Generate unique test IDs
3. ✅ Create and manage shared tmpDir
4. ✅ Call subengines in order (create) and reverse order (delete)
5. ✅ Aggregate files, metadata, and managedResources
6. ✅ Store TestEnvironment in artifact store
7. ✅ Handle cleanup on failures
8. ✅ Support best-effort deletion

Following this guide ensures your orchestrator integrates seamlessly with forge and can compose multiple subengines into complete test environments.
