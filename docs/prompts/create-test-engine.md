# Creating a Custom Test Engine

You are helping a user create a custom test engine for forge. A test engine manages the lifecycle of test environments (create, get, delete, list operations).

## What is a Test Engine?

A **test engine** is a component that manages test environment infrastructure:
- Creates test environments with unique identifiers
- Retrieves test environment details and status
- Lists test environments, optionally filtered by stage
- Deletes test environments and cleans up all resources

Test engines do NOT execute tests or generate reports - that's the test runner's job.

## When to Create a Custom Test Engine

Create a custom test engine when you need to:
- ✅ Manage Kubernetes clusters (kind, k3d, minikube)
- ✅ Spin up databases or services for integration tests
- ✅ Configure cloud resources for testing
- ✅ Set up mock APIs or test fixtures
- ✅ Manage complex multi-component test environments

Use the built-in `testenv` engine for simple needs.

## API Contract

### CLI Interface

Your test engine must support these commands:

```bash
# Create a test environment
<engine-binary> create <STAGE-NAME>
# Output: test-id (to stdout)

# Get test environment details
<engine-binary> get <TEST-ID>
# Output: formatted environment information

# Delete a test environment
<engine-binary> delete <TEST-ID>
# Output: confirmation message

# List test environments (optional stage filter)
<engine-binary> list [--stage=<STAGE-NAME>]
# Output: table of environments

# Version information
<engine-binary> version

# MCP server mode (required)
<engine-binary> --mcp
```

### MCP Interface

Your engine must implement these MCP tools:

**create** - Create a test environment
- Input: `{ "stage": "string" }`
- Output: `{ "testID": "string" }`

**get** - Get test environment details
- Input: `{ "testID": "string" }`
- Output: `TestEnvironment` object

**delete** - Delete a test environment
- Input: `{ "testID": "string" }`
- Output: Success message

**list** - List test environments
- Input: `{ "stage": "string" }` (optional)
- Output: Array of `TestEnvironment` objects

## Implementation Steps

### Step 1: Set Up Project Structure

Start from the `testenv` template:

```bash
# Copy the template
cp -r cmd/testenv cmd/<your-engine-name>

# Update the Name constant in main.go
const Name = "<your-engine-name>"
```

### Step 2: Implement Create Operation

The create operation should:
1. Generate a unique test ID (format: `test-<stage>-YYYYMMDD-XXXXXXXX`)
2. Set up the required infrastructure
3. Track managed resources for cleanup
4. Store the TestEnvironment in the artifact store
5. Output the test ID to stdout

Example structure:

```go
func cmdCreate(stageName string) error {
    // 1. Read forge.yaml to get test spec
    config, err := forge.ReadSpec()

    // 2. Generate unique test ID
    testID := generateTestID(stageName)

    // 3. Set up infrastructure (your custom logic)
    resources, err := setupInfrastructure(stageName, testID)

    // 4. Create TestEnvironment
    env := &forge.TestEnvironment{
        ID:               testID,
        Name:             stageName,
        Status:           forge.TestStatusCreated,
        CreatedAt:        time.Now().UTC(),
        UpdatedAt:        time.Now().UTC(),
        ManagedResources: resources,
        // Add custom fields like KubeconfigPath, RegistryConfig, etc.
    }

    // 5. Store in artifact store
    artifactStorePath, _ := forge.GetArtifactStorePath(".forge/artifacts.json")
    store, _ := forge.ReadOrCreateArtifactStore(artifactStorePath)
    forge.AddOrUpdateTestEnvironment(&store, env)
    forge.WriteArtifactStore(artifactStorePath, store)

    // 6. Output test ID
    fmt.Println(testID)
    return nil
}
```

### Step 3: Implement Get Operation

Retrieve and display test environment details:

```go
func cmdGet(testID string) error {
    // Read from artifact store
    artifactStorePath, _ := forge.GetArtifactStorePath(".forge/artifacts.json")
    store, _ := forge.ReadArtifactStore(artifactStorePath)

    env, err := forge.GetTestEnvironment(&store, testID)
    if err != nil {
        return fmt.Errorf("test environment not found: %s", testID)
    }

    // Display information
    fmt.Printf("ID: %s\n", env.ID)
    fmt.Printf("Stage: %s\n", env.Name)
    fmt.Printf("Status: %s\n", env.Status)
    // Display custom fields

    return nil
}
```

### Step 4: Implement Delete Operation

Clean up all resources and remove from artifact store:

```go
func cmdDelete(testID string) error {
    // Get environment
    artifactStorePath, _ := forge.GetArtifactStorePath(".forge/artifacts.json")
    store, _ := forge.ReadArtifactStore(artifactStorePath)

    env, err := forge.GetTestEnvironment(&store, testID)
    if err != nil {
        return err
    }

    // Clean up infrastructure (your custom logic)
    for _, resource := range env.ManagedResources {
        cleanupResource(resource)
    }

    // Remove from artifact store
    forge.DeleteTestEnvironment(&store, testID)
    forge.WriteArtifactStore(artifactStorePath, store)

    fmt.Printf("Deleted test environment: %s\n", testID)
    return nil
}
```

### Step 5: Implement List Operation

```go
func cmdList(stageFilter string) error {
    artifactStorePath, _ := forge.GetArtifactStorePath(".forge/artifacts.json")
    store, _ := forge.ReadArtifactStore(artifactStorePath)

    envs := forge.ListTestEnvironments(&store, stageFilter)

    // Display as table
    w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
    fmt.Fprintln(w, "ID\tSTAGE\tSTATUS\tCREATED")
    for _, env := range envs {
        fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
            env.ID, env.Name, env.Status, env.CreatedAt.Format("2006-01-02 15:04"))
    }
    w.Flush()

    return nil
}
```

### Step 6: Implement MCP Server

Follow the pattern in `testenv/mcp.go`:

1. Register tools (create, get, delete, list)
2. Each tool handler should call the corresponding cmd function
3. Return appropriate MCP results

### Step 7: Configure in forge.yaml

```yaml
test:
  - name: integration
    engine: go://<your-engine-name>
    runner: go://test-runner-go
```

## Best Practices

1. **Idempotency**: Create operations should be safe to retry
2. **Resource Tracking**: Store all managed resources in `ManagedResources`
3. **Cleanup**: Always clean up resources in delete, even if some fail
4. **Error Handling**: Provide clear error messages
5. **Status Updates**: Update environment status appropriately
6. **Unique IDs**: Use consistent ID format: `test-<stage>-YYYYMMDD-XXXXXXXX`

## Testing Your Engine

```bash
# Build the engine
forge build <your-engine-name>

# Test create
./build/bin/<your-engine-name> create integration
# Should output: testenv-20240101-abcd1234

# Test get
./build/bin/<your-engine-name> get testenv-20240101-abcd1234

# Test list
./build/bin/<your-engine-name> list

# Test delete
./build/bin/<your-engine-name> delete testenv-20240101-abcd1234
```

## Integration with Forge

Once your engine is ready:

```bash
# Use with forge test
forge test create integration
forge test get <test-id>
forge test delete <test-id>
forge test list
```

## Examples

- **testenv**: Reference implementation in `cmd/testenv`
- **kindenv**: Manages Kind Kubernetes clusters in `cmd/kindenv`

## Need Help?

- Review `cmd/testenv` for a complete working example
- Check existing test engine implementations
- The forge CLI handles MCP communication - focus on your infrastructure logic

---

# COMPREHENSIVE TEST ENGINE IMPLEMENTATION REFERENCE GUIDE

The following sections provide the complete, detailed test engine implementation guide. Use this as reference when helping users implement custom test engines.

---

# Test Engine Implementation Guide

## Overview

A **test engine** is a component that manages the lifecycle of test environments. Test engines implement four core operations: create, get, delete, and list. They are responsible for setting up the necessary infrastructure for running tests (e.g., Kubernetes clusters, databases, mock services) and cleaning up afterwards.

## Responsibilities

Test engines handle:
- **Creating** test environments with unique identifiers
- **Retrieving** test environment details and status
- **Listing** test environments, optionally filtered by stage
- **Deleting** test environments and cleaning up all resources

Test engines do NOT:
- Execute tests (that's the test runner's job)
- Generate test reports
- Manage test code or test data

## API Contract

### CLI Interface

Test engines must support the following command-line interface:

```bash
# Create a test environment
<engine-binary> create <STAGE-NAME>
# Output: test-id (to stdout)

# Get test environment details
<engine-binary> get <TEST-ID>
# Output: formatted environment information

# Delete a test environment
<engine-binary> delete <TEST-ID>
# Output: confirmation message

# List test environments (optional stage filter)
<engine-binary> list [--stage=<STAGE-NAME>]
# Output: table of environments

# Version information
<engine-binary> version

# MCP server mode (required)
<engine-binary> --mcp
```

### MCP Interface

All test engines MUST support MCP (Model Context Protocol) mode via the `--mcp` flag. This enables programmatic access from the forge CLI.

**Required MCP Tools:**

#### 1. `create` Tool
```json
{
  "name": "create",
  "description": "Create a test environment for a given stage",
  "inputSchema": {
    "type": "object",
    "properties": {
      "stage": {
        "type": "string",
        "description": "Test stage name (e.g., 'integration', 'e2e')"
      }
    },
    "required": ["stage"]
  }
}
```

**Response:**
- Success: `{ "testID": "test-<stage>-YYYYMMDD-XXXXXXXX" }`
- Error: `IsError: true` with error message in Content

#### 2. `get` Tool
```json
{
  "name": "get",
  "description": "Get test environment details by ID",
  "inputSchema": {
    "type": "object",
    "properties": {
      "testID": {
        "type": "string",
        "description": "Unique test environment identifier"
      }
    },
    "required": ["testID"]
  }
}
```

**Response:**
- Success: TestEnvironment object in Meta
- Error: `IsError: true` with error message

#### 3. `delete` Tool
```json
{
  "name": "delete",
  "description": "Delete a test environment by ID",
  "inputSchema": {
    "type": "object",
    "properties": {
      "testID": {
        "type": "string",
        "description": "Unique test environment identifier"
      }
    },
    "required": ["testID"]
  }
}
```

**Response:**
- Success: Confirmation message
- Error: `IsError: true` with error message

#### 4. `list` Tool
```json
{
  "name": "list",
  "description": "List test environments, optionally filtered by stage",
  "inputSchema": {
    "type": "object",
    "properties": {
      "stage": {
        "type": "string",
        "description": "Optional stage filter"
      }
    }
  }
}
```

**Response:**
- Success: Array of TestEnvironment objects in Meta
- Error: `IsError: true` with error message

## Data Structures

### TestEnvironment

Test engines work with the `TestEnvironment` structure defined in `pkg/forge/spec_tst.go`:

```go
type TestEnvironment struct {
    // ID is the unique identifier (format: test-<stage>-YYYYMMDD-XXXXXXXX)
    ID string `json:"id"`

    // Name is the test stage name (e.g., "integration", "e2e")
    Name string `json:"name"`

    // Status tracks the current state
    // Values: "created", "running", "passed", "failed", "partially_deleted"
    Status string `json:"status"`

    // CreatedAt is when the environment was created
    CreatedAt time.Time `json:"createdAt"`

    // UpdatedAt is when the environment was last updated
    UpdatedAt time.Time `json:"updatedAt"`

    // ArtifactPath is the root directory for test artifacts
    ArtifactPath string `json:"artifactPath,omitempty"`

    // KubeconfigPath is the path to kubeconfig (for Kubernetes-based tests)
    KubeconfigPath string `json:"kubeconfigPath,omitempty"`

    // RegistryConfig holds local container registry configuration
    RegistryConfig map[string]string `json:"registryConfig,omitempty"`

    // ManagedResources lists all files/directories created
    // Used for cleanup on delete
    ManagedResources []string `json:"managedResources"`

    // Metadata holds engine-specific data
    Metadata map[string]string `json:"metadata,omitempty"`
}
```

### Status Values

```go
const (
    TestStatusCreated          = "created"
    TestStatusRunning          = "running"
    TestStatusPassed           = "passed"
    TestStatusFailed           = "failed"
    TestStatusPartiallyDeleted = "partially_deleted"
)
```

## Artifact Store Integration

Test engines MUST persist test environment state to the unified artifact store at `.forge/artifacts.json` (or the path specified in `forge.yaml`).

### Reading the Artifact Store

```go
import "github.com/alexandremahdhaoui/forge/pkg/forge"

config, err := forge.ReadSpec()
if err != nil {
    return err
}

artifactStorePath := config.ArtifactStorePath
if artifactStorePath == "" {
    artifactStorePath = ".forge/artifacts.json"
}

store, err := forge.ReadArtifactStore(artifactStorePath)
if err != nil {
    return err
}
```

### Writing to the Artifact Store

```go
// Create or update test environment
env := &forge.TestEnvironment{
    ID:               testID,
    Name:             stageName,
    Status:           forge.TestStatusCreated,
    CreatedAt:        time.Now().UTC(),
    UpdatedAt:        time.Now().UTC(),
    ManagedResources: []string{},
    Metadata:         make(map[string]string),
}

forge.AddOrUpdateTestEnvironment(&store, env)

if err := forge.WriteArtifactStore(artifactStorePath, store); err != nil {
    return err
}
```

### Helper Functions

```go
// Add or update a test environment
forge.AddOrUpdateTestEnvironment(store *ArtifactStore, env *TestEnvironment)

// Retrieve a test environment by ID
env, err := forge.GetTestEnvironment(store *ArtifactStore, id string) (*TestEnvironment, error)

// List test environments (optional stage filter)
envs := forge.ListTestEnvironments(store *ArtifactStore, stageName string) []*TestEnvironment

// Delete a test environment
err := forge.DeleteTestEnvironment(store *ArtifactStore, id string) error
```

## Implementation Pattern

### Directory Structure

```
cmd/my-test-engine/
├── main.go       # Entry point, CLI routing
├── create.go     # Create operation
├── get.go        # Get operation
├── delete.go     # Delete operation
├── list.go       # List operation
└── mcp.go        # MCP server implementation
```

### main.go Template

```go
package main

import (
    "fmt"
    "log"
    "os"

    "github.com/alexandremahdhaoui/forge/internal/version"
)

var (
    Version        = "dev"
    CommitSHA      = "unknown"
    BuildTimestamp = "unknown"
)

var versionInfo *version.Info

func init() {
    versionInfo = version.New("my-test-engine")
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
    case "create":
        stageName := ""
        if len(os.Args) >= 3 {
            stageName = os.Args[2]
        }
        if err := cmdCreate(stageName); err != nil {
            fmt.Fprintf(os.Stderr, "Error: %v\n", err)
            os.Exit(1)
        }
    case "get":
        if len(os.Args) < 3 {
            fmt.Fprintf(os.Stderr, "Error: test ID required\n")
            os.Exit(1)
        }
        if err := cmdGet(os.Args[2]); err != nil {
            fmt.Fprintf(os.Stderr, "Error: %v\n", err)
            os.Exit(1)
        }
    case "delete":
        if len(os.Args) < 3 {
            fmt.Fprintf(os.Stderr, "Error: test ID required\n")
            os.Exit(1)
        }
        if err := cmdDelete(os.Args[2]); err != nil {
            fmt.Fprintf(os.Stderr, "Error: %v\n", err)
            os.Exit(1)
        }
    case "list":
        stageFilter := ""
        // Parse --stage flag if present
        if err := cmdList(stageFilter); err != nil {
            fmt.Fprintf(os.Stderr, "Error: %v\n", err)
            os.Exit(1)
        }
    case "version", "--version", "-v":
        versionInfo.Print()
    default:
        fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
        printUsage()
        os.Exit(1)
    }
}

func printUsage() {
    fmt.Println(`my-test-engine - Manage test environments

Usage:
  my-test-engine create <STAGE>
  my-test-engine get <TEST-ID>
  my-test-engine delete <TEST-ID>
  my-test-engine list [--stage=<NAME>]
  my-test-engine --mcp
  my-test-engine version`)
}
```

### create.go Template

```go
package main

import (
    "crypto/rand"
    "encoding/hex"
    "fmt"
    "time"

    "github.com/alexandremahdhaoui/forge/pkg/forge"
)

func cmdCreate(stageName string) error {
    if stageName == "" {
        return fmt.Errorf("stage name is required")
    }

    // Read forge.yaml
    config, err := forge.ReadSpec()
    if err != nil {
        return fmt.Errorf("failed to read forge.yaml: %w", err)
    }

    // Generate unique test ID
    testID := generateTestID(stageName)

    // Setup your test infrastructure here
    // Example: Create Kubernetes cluster, database, etc.
    managedResources := []string{}

    // Example: Setup a resource
    // resourcePath := setupMyResource(testID)
    // managedResources = append(managedResources, resourcePath)

    // Create test environment
    env := &forge.TestEnvironment{
        ID:               testID,
        Name:             stageName,
        Status:           forge.TestStatusCreated,
        CreatedAt:        time.Now().UTC(),
        UpdatedAt:        time.Now().UTC(),
        ManagedResources: managedResources,
        Metadata:         make(map[string]string),
    }

    // Store engine-specific data in Metadata
    // env.Metadata["my-key"] = "my-value"

    // Save to artifact store
    artifactStorePath := config.ArtifactStorePath
    if artifactStorePath == "" {
        artifactStorePath = ".forge/artifacts.json"
    }

    store, err := forge.ReadArtifactStore(artifactStorePath)
    if err != nil {
        return fmt.Errorf("failed to read artifact store: %w", err)
    }

    forge.AddOrUpdateTestEnvironment(&store, env)

    if err := forge.WriteArtifactStore(artifactStorePath, store); err != nil {
        return fmt.Errorf("failed to write artifact store: %w", err)
    }

    // Output test ID (required)
    fmt.Println(testID)
    return nil
}

func generateTestID(stageName string) string {
    randBytes := make([]byte, 4)
    rand.Read(randBytes)
    suffix := hex.EncodeToString(randBytes)
    dateStr := time.Now().Format("20060102")
    return fmt.Sprintf("test-%s-%s-%s", stageName, dateStr, suffix)
}
```

### mcp.go Template

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

type CreateInput struct {
    Stage string `json:"stage"`
}

type GetInput struct {
    TestID string `json:"testID"`
}

type DeleteInput struct {
    TestID string `json:"testID"`
}

type ListInput struct {
    Stage string `json:"stage,omitempty"`
}

func runMCPServer() error {
    v, _, _ := versionInfo.Get()
    server := mcpserver.New("my-test-engine", v)

    mcpserver.RegisterTool(server, &mcp.Tool{
        Name:        "create",
        Description: "Create a test environment",
    }, handleCreateTool)

    mcpserver.RegisterTool(server, &mcp.Tool{
        Name:        "get",
        Description: "Get test environment details",
    }, handleGetTool)

    mcpserver.RegisterTool(server, &mcp.Tool{
        Name:        "delete",
        Description: "Delete a test environment",
    }, handleDeleteTool)

    mcpserver.RegisterTool(server, &mcp.Tool{
        Name:        "list",
        Description: "List test environments",
    }, handleListTool)

    return server.RunDefault()
}

func handleCreateTool(
    ctx context.Context,
    req *mcp.CallToolRequest,
    input CreateInput,
) (*mcp.CallToolResult, any, error) {
    log.Printf("Creating test environment: stage=%s", input.Stage)

    if input.Stage == "" {
        return &mcp.CallToolResult{
            Content: []mcp.Content{
                &mcp.TextContent{Text: "Create failed: missing 'stage'"},
            },
            IsError: true,
        }, nil, nil
    }

    // Implementation similar to cmdCreate
    // Return testID in Meta

    return &mcp.CallToolResult{
        Content: []mcp.Content{
            &mcp.TextContent{Text: "Created test environment: " + testID},
        },
    }, map[string]string{"testID": testID}, nil
}

// Implement other handlers similarly...
```

## Resource Management

### Managed Resources

Track ALL created resources in `ManagedResources` for proper cleanup:

```go
env.ManagedResources = append(env.ManagedResources,
    "/path/to/kubeconfig",
    "/path/to/data-dir",
    "/path/to/temp-file",
)
```

### Cleanup Pattern

```go
func cmdDelete(testID string) error {
    // Get environment
    env, err := forge.GetTestEnvironment(&store, testID)
    if err != nil {
        return err
    }

    // Cleanup managed resources
    for _, resource := range env.ManagedResources {
        if err := os.RemoveAll(resource); err != nil {
            log.Printf("Warning: failed to remove %s: %v", resource, err)
        }
    }

    // Cleanup engine-specific resources
    // Example: Delete Kubernetes cluster, database, etc.

    // Remove from artifact store
    forge.DeleteTestEnvironment(&store, testID)
    forge.WriteArtifactStore(artifactStorePath, store)

    return nil
}
```

## Best Practices

### 1. Unique Identifiers

Always generate unique test IDs using the pattern:
```
test-<stage>-YYYYMMDD-XXXXXXXX
```

This ensures:
- Clear identification of test stage
- Date-based sorting
- Collision resistance (8-char random hex)

### 2. Error Handling

- Return detailed error messages
- Set `IsError: true` in MCP responses
- Log errors to stderr (stdout is for structured data)

### 3. Idempotency

Make operations idempotent where possible:
- Creating an existing environment should return the existing ID
- Deleting a non-existent environment should not error
- Getting a deleted environment should return a clear error

### 4. State Management

- Always update `UpdatedAt` when modifying environments
- Use `Status` to track environment lifecycle
- Store paths as absolute paths in ManagedResources

### 5. Versioning

Use `internal/version` for consistent version reporting:

```go
import "github.com/alexandremahdhaoui/forge/internal/version"

var versionInfo = version.New("my-engine")
```

### 6. Logging

- Use `log.Printf()` for debug information
- Write logs to stderr only
- Never write to stdout except for required outputs (test ID, formatted data)

## Testing Your Engine

### Manual Testing

```bash
# Build your engine
go build -o ./build/bin/my-test-engine ./cmd/my-test-engine

# Test create
TEST_ID=$(./build/bin/my-test-engine create integration)
echo "Created: $TEST_ID"

# Test get
./build/bin/my-test-engine get $TEST_ID

# Test list
./build/bin/my-test-engine list

# Test delete
./build/bin/my-test-engine delete $TEST_ID
```

### Integration with Forge

Add to `forge.yaml`:

```yaml
test:
  - name: integration
    engine: "go://github.com/myorg/my-test-engine"
    runner: "go://test-runner-go"
```

Test via forge:

```bash
forge test integration create
forge test integration list
```

## Reference Implementation

See `cmd/test-integration` for a complete reference implementation that manages integration test environments.

## Common Patterns

### Kubernetes-based Engines

For engines that create Kubernetes clusters:

```go
env.KubeconfigPath = "/path/to/kubeconfig"
env.Metadata["cluster-name"] = clusterName
env.ManagedResources = append(env.ManagedResources, kubeconfigPath)
```

### Database Engines

For engines that provision databases:

```go
env.Metadata["db-host"] = dbHost
env.Metadata["db-port"] = dbPort
env.Metadata["db-name"] = dbName
env.ManagedResources = append(env.ManagedResources, dbDataDir)
```

### Container Registry Engines

For engines that setup container registries:

```go
env.RegistryConfig = map[string]string{
    "host":     registryHost,
    "ca-cert":  caCertPath,
    "username": username,
}
env.ManagedResources = append(env.ManagedResources, certDir, dataDir)
```

## Troubleshooting

### Environment Not Found

Ensure you're reading/writing to the correct artifact store path:

```go
artifactStorePath := config.ArtifactStorePath
if artifactStorePath == "" {
    artifactStorePath = ".forge/artifacts.json"
}
```

### MCP Connection Errors

Verify MCP server is properly initialized:

```go
server := mcpserver.New("my-engine", version)
return server.RunDefault()  // Not server.Run()
```

### Resource Cleanup Failures

Always continue cleanup even if some resources fail:

```go
for _, resource := range env.ManagedResources {
    if err := os.RemoveAll(resource); err != nil {
        log.Printf("Warning: %v", err)  // Log but continue
    }
}
```

## Summary

A test engine must:
1. ✅ Implement 4 CLI commands: create, get, delete, list
2. ✅ Support MCP mode with --mcp flag
3. ✅ Persist state to the artifact store
4. ✅ Track all managed resources for cleanup
5. ✅ Generate unique test IDs
6. ✅ Handle errors gracefully
7. ✅ Support version reporting

Following this guide ensures your test engine integrates seamlessly with the forge test infrastructure.
