# Creating a Testenv Subengine

You are helping a user create a custom testenv subengine for forge. A testenv subengine is a component that handles one specific aspect of test environment setup (e.g., creating a database, configuring a service, setting up mock APIs).

## What is a Testenv Subengine?

A **testenv subengine** is an independent component that:
- Creates a specific resource or service for a test environment
- Generates configuration files in a shared tmpDir
- Returns metadata about created resources
- Cleans up resources on delete

Testenv subengines are **composed** by the testenv orchestrator to build complete test environments.

## When to Create a Testenv Subengine

Create a testenv subengine when you need to:
- ✅ Provision a specific service (database, cache, message queue)
- ✅ Configure cloud resources for testing
- ✅ Deploy applications to a test cluster
- ✅ Generate test data or fixtures
- ✅ Set up mock APIs or external services
- ✅ Configure networking or security components

## Examples of Testenv Subengines

**Built-in subengines:**
- `testenv-kind`: Creates Kind Kubernetes clusters
- `testenv-lcr`: Deploys local container registry
- `testenv-helm-install`: Installs Helm charts

**Custom subengine ideas:**
- `testenv-postgres`: Provision PostgreSQL database
- `testenv-redis`: Deploy Redis cache
- `testenv-s3-mock`: Setup S3-compatible mock storage
- `testenv-kafka`: Deploy Kafka cluster
- `testenv-oauth-mock`: Mock OAuth provider

## API Contract

### MCP Interface (Required)

Your subengine **must** implement these MCP tools:

#### `create` Tool

Create resources for the test environment.

**Input Schema:**
```json
{
  "testID": "string (required)",     // Unique test environment ID
  "stage": "string (required)",      // Test stage name (e.g., "integration")
  "tmpDir": "string (required)"      // Shared temporary directory for files
}
```

**Output Schema:**
```json
{
  "testID": "string",
  "files": {
    "my-subengine.config": "config.yaml"  // Relative path in tmpDir
  },
  "metadata": {
    "my-subengine.endpoint": "http://localhost:5432",
    "my-subengine.configPath": "/abs/path/to/tmpDir/config.yaml"
  },
  "managedResources": [
    "/abs/path/to/tmpDir/config.yaml",
    "docker-container-id"
  ]
}
```

**Fields:**
- `testID`: Echo back the test ID
- `files`: Map of logical names to filenames in tmpDir (relative paths)
- `metadata`: Key-value pairs accessible to test runners
- `managedResources`: Paths/IDs for cleanup (absolute paths for files)

#### `delete` Tool

Clean up resources created by this subengine.

**Input Schema:**
```json
{
  "testID": "string (required)"      // Test environment ID
}
```

**Output:**
```json
{
  "success": true,
  "message": "Deleted resources for test-integration-20250106-abc123"
}
```

**Important:** Delete should be best-effort. Don't fail if resources are already gone.

### CLI Interface (Optional)

You can optionally support direct CLI usage for debugging:

```bash
# Setup operation (for debugging)
<subengine-binary> setup

# Teardown operation (for debugging)
<subengine-binary> teardown

# Version information
<subengine-binary> version

# MCP server mode (required)
<subengine-binary> --mcp
```

## Implementation Steps

### Step 1: Choose a Template

Start from an existing subengine:

```bash
# Copy a similar subengine
cp -r cmd/testenv-kind cmd/testenv-<your-name>

# Update the Name constant in main.go
const Name = "testenv-<your-name>"
```

**Template choices:**
- `testenv-kind`: For cluster/VM creation
- `testenv-lcr`: For service deployment
- `testenv-helm-install`: For Helm-based deployments

### Step 2: Implement the Create Operation

```go
package main

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"

    "github.com/alexandremahdhaoui/forge/pkg/forge"
)

type CreateInput struct {
    TestID string `json:"testID"`
    Stage  string `json:"stage"`
    TmpDir string `json:"tmpDir"`
}

type CreateOutput struct {
    TestID           string            `json:"testID"`
    Files            map[string]string `json:"files"`
    Metadata         map[string]string `json:"metadata"`
    ManagedResources []string          `json:"managedResources"`
}

func handleCreate(input CreateInput) (*CreateOutput, error) {
    // 1. Validate inputs
    if input.TestID == "" || input.Stage == "" || input.TmpDir == "" {
        return nil, fmt.Errorf("testID, stage, and tmpDir are required")
    }

    // 2. Create your resource (database, service, etc.)
    resourceID, err := createMyResource(input.TestID, input.Stage)
    if err != nil {
        return nil, fmt.Errorf("failed to create resource: %w", err)
    }

    // 3. Generate configuration files in tmpDir
    configPath := filepath.Join(input.TmpDir, "my-subengine-config.yaml")
    config := generateConfig(resourceID)
    if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
        return nil, fmt.Errorf("failed to write config: %w", err)
    }

    // 4. Build output
    output := &CreateOutput{
        TestID: input.TestID,
        Files: map[string]string{
            "testenv-mysubengine.config": "my-subengine-config.yaml", // Relative
        },
        Metadata: map[string]string{
            "testenv-mysubengine.resourceID":  resourceID,
            "testenv-mysubengine.endpoint":    getEndpoint(resourceID),
            "testenv-mysubengine.configPath":  configPath, // Absolute
        },
        ManagedResources: []string{
            configPath,      // Files (absolute paths)
            resourceID,      // External resource IDs
        },
    }

    return output, nil
}
```

### Step 3: Implement the Delete Operation

```go
type DeleteInput struct {
    TestID string `json:"testID"`
}

type DeleteOutput struct {
    Success bool   `json:"success"`
    Message string `json:"message"`
}

func handleDelete(input DeleteInput) (*DeleteOutput, error) {
    // 1. Reconstruct resource ID from testID or metadata
    resourceID := getResourceIDFromTestID(input.TestID)

    // 2. Delete the resource (best-effort)
    if err := deleteMyResource(resourceID); err != nil {
        // Log but don't fail - resource might be already gone
        log.Printf("Warning: failed to delete resource %s: %v", resourceID, err)
    }

    // 3. Return success
    return &DeleteOutput{
        Success: true,
        Message: fmt.Sprintf("Deleted resources for %s", input.TestID),
    }, nil
}
```

### Step 4: Implement MCP Server

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
    server := mcpserver.New("testenv-mysubengine", v)

    // Register create tool
    mcpserver.RegisterTool(server, &mcp.Tool{
        Name:        "create",
        Description: "Create resources for test environment",
    }, handleCreateTool)

    // Register delete tool
    mcpserver.RegisterTool(server, &mcp.Tool{
        Name:        "delete",
        Description: "Delete resources for test environment",
    }, handleDeleteTool)

    return server.RunDefault()
}

func handleCreateTool(
    ctx context.Context,
    req *mcp.CallToolRequest,
    input CreateInput,
) (*mcp.CallToolResult, any, error) {
    log.Printf("Creating resources: testID=%s stage=%s", input.TestID, input.Stage)

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
            &mcp.TextContent{Text: fmt.Sprintf("Created resources for %s", input.TestID)},
        },
    }, output, nil
}

func handleDeleteTool(
    ctx context.Context,
    req *mcp.CallToolRequest,
    input DeleteInput,
) (*mcp.CallToolResult, any, error) {
    log.Printf("Deleting resources: testID=%s", input.TestID)

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

### Step 5: Create main.go

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
    versionInfo = version.New("testenv-mysubengine")
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
    case "version", "--version", "-v":
        versionInfo.Print()
    case "setup":
        // Optional: CLI mode for debugging
        fmt.Println("Setup not implemented in CLI mode. Use --mcp.")
    case "teardown":
        // Optional: CLI mode for debugging
        fmt.Println("Teardown not implemented in CLI mode. Use --mcp.")
    default:
        fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
        printUsage()
        os.Exit(1)
    }
}

func printUsage() {
    fmt.Println(`testenv-mysubengine - Manage test environment resources

Usage:
  testenv-mysubengine --mcp
  testenv-mysubengine version`)
}
```

### Step 6: Add to forge.yaml

Configure your subengine in the testenv spec:

```yaml
name: my-project

test:
  - name: integration
    testenv:
      engine: go://testenv
      subengines:
        - go://testenv-kind
        - go://testenv-mysubengine  # Your new subengine
        - go://testenv-lcr
    runner: go://test-runner-go
```

### Step 7: Add Build Configuration

Add your subengine to the build spec in `forge.yaml`:

```yaml
build:
  - name: testenv-mysubengine
    src: ./cmd/testenv-mysubengine
    dest: ./build/bin
    engine: go://build-go
```

## Best Practices

### 1. Naming Conventions

**Subengine names:** `testenv-<component>`
- Examples: `testenv-postgres`, `testenv-redis`, `testenv-s3mock`

**File keys:** `testenv-<component>.<purpose>`
- Examples: `testenv-postgres.config`, `testenv-redis.credentials`

**Metadata keys:** `testenv-<component>.<property>`
- Examples: `testenv-postgres.connectionString`, `testenv-redis.port`

### 2. File Management

**Always use relative paths in `files` map:**
```go
Files: map[string]string{
    "testenv-mysubengine.config": "config.yaml",  // Relative to tmpDir
}
```

**Use absolute paths in `metadata` and `managedResources`:**
```go
Metadata: map[string]string{
    "testenv-mysubengine.configPath": filepath.Join(tmpDir, "config.yaml"),
}
ManagedResources: []string{
    filepath.Join(tmpDir, "config.yaml"),
}
```

### 3. Resource Cleanup

**Be idempotent and best-effort:**
```go
func handleDelete(input DeleteInput) (*DeleteOutput, error) {
    // Don't fail if resource doesn't exist
    if !resourceExists(resourceID) {
        return &DeleteOutput{Success: true, Message: "Already deleted"}, nil
    }

    // Log errors but don't fail
    if err := deleteResource(resourceID); err != nil {
        log.Printf("Warning: %v", err)
    }

    return &DeleteOutput{Success: true}, nil
}
```

### 4. Error Handling

**Validate inputs:**
```go
if input.TestID == "" || input.TmpDir == "" {
    return nil, fmt.Errorf("testID and tmpDir are required")
}
```

**Provide helpful error messages:**
```go
return nil, fmt.Errorf("failed to create database: connection timeout after 30s")
```

### 5. Dependencies Between Subengines

If your subengine depends on another (e.g., needs a Kubernetes cluster):

```go
func handleCreate(input CreateInput) (*CreateOutput, error) {
    // Access metadata from previous subengines via environment or tmpDir files
    kubeconfigPath := filepath.Join(input.TmpDir, "kubeconfig")
    if _, err := os.Stat(kubeconfigPath); err != nil {
        return nil, fmt.Errorf("kubeconfig not found - ensure testenv-kind runs first")
    }

    // Use kubeconfig to deploy to cluster
    // ...
}
```

**Order matters:** Declare subengines in dependency order in `forge.yaml`:
```yaml
subengines:
  - go://testenv-kind           # Creates cluster
  - go://testenv-mysubengine    # Uses cluster
```

### 6. Testing Your Subengine

**Build:**
```bash
forge build testenv-mysubengine
```

**Test via testenv orchestrator:**
```bash
forge test create-env integration
forge test list-env integration
forge test delete-env integration <ENV_ID>
```

**Verify files in tmpDir:**
```bash
ls -la /tmp/forge-test-integration-<testID>/
```

**Check metadata:**
```bash
forge test get-env integration <ENV_ID>
```

## Common Patterns

### Pattern 1: Database Provisioning

```go
func handleCreate(input CreateInput) (*CreateOutput, error) {
    // Start database container
    containerID, err := startPostgresContainer(input.TestID)

    // Generate connection string
    connString := fmt.Sprintf("postgresql://user:pass@localhost:5432/%s", input.TestID)

    // Write credentials file
    credsPath := filepath.Join(input.TmpDir, "db-credentials.yaml")
    creds := fmt.Sprintf("connectionString: %s\n", connString)
    os.WriteFile(credsPath, []byte(creds), 0644)

    return &CreateOutput{
        Files: map[string]string{
            "testenv-postgres.credentials": "db-credentials.yaml",
        },
        Metadata: map[string]string{
            "testenv-postgres.connectionString": connString,
            "testenv-postgres.containerID":      containerID,
        },
        ManagedResources: []string{credsPath, containerID},
    }, nil
}
```

### Pattern 2: Service Deployment (Kubernetes)

```go
func handleCreate(input CreateInput) (*CreateOutput, error) {
    // Read kubeconfig from tmpDir (created by testenv-kind)
    kubeconfigPath := filepath.Join(input.TmpDir, "kubeconfig")

    // Deploy service using kubectl or k8s client
    svcEndpoint, err := deployService(kubeconfigPath, input.TestID)

    return &CreateOutput{
        Metadata: map[string]string{
            "testenv-myservice.endpoint":  svcEndpoint,
            "testenv-myservice.namespace": input.TestID,
        },
        ManagedResources: []string{},  // Kubernetes handles cleanup
    }, nil
}
```

### Pattern 3: Mock Service

```go
func handleCreate(input CreateInput) (*CreateOutput, error) {
    // Start mock server on random port
    port, server := startMockServer()

    // Save PID for cleanup
    pidFile := filepath.Join(input.TmpDir, "mock-server.pid")
    os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", server.PID)), 0644)

    return &CreateOutput{
        Files: map[string]string{
            "testenv-mockserver.pid": "mock-server.pid",
        },
        Metadata: map[string]string{
            "testenv-mockserver.url": fmt.Sprintf("http://localhost:%d", port),
            "testenv-mockserver.pid": fmt.Sprintf("%d", server.PID),
        },
        ManagedResources: []string{pidFile},
    }, nil
}

func handleDelete(input DeleteInput) (*DeleteOutput, error) {
    // Read PID and kill process
    pidFile := getPIDFile(input.TestID)
    pid := readPID(pidFile)
    killProcess(pid)
    os.Remove(pidFile)

    return &DeleteOutput{Success: true}, nil
}
```

## Integration with Test Runners

Test runners receive all files and metadata via the testenv:

```go
// In your test code
func TestMyFeature(t *testing.T) {
    // Access files from tmpDir
    dbCreds := os.Getenv("TESTENV_POSTGRES_CREDENTIALS")  // Path to credentials file

    // Access metadata
    endpoint := os.Getenv("TESTENV_MYSERVICE_ENDPOINT")

    // Run tests
    // ...
}
```

## Documentation

Create `cmd/testenv-mysubengine/MCP.md`:

```markdown
# testenv-mysubengine MCP Server

MCP server for managing [your resource] in test environments.

## Purpose

[Description of what this subengine does]

## Available Tools

### `create`

Create [resource] for a test environment.

**Input Schema:**
[Schema]

**Output:**
[Schema]

### `delete`

Delete [resource] for a test environment.

[Details]

## See Also

- [testenv MCP Server](../testenv/MCP.md)
- [testenv-kind MCP Server](../testenv-kind/MCP.md)
```

## Reference Implementations

See these subengines for examples:
- **testenv-kind** (`cmd/testenv-kind`): Cluster creation
- **testenv-lcr** (`cmd/testenv-lcr`): Service deployment
- **testenv-helm-install** (`cmd/testenv-helm-install`): Helm charts

## Troubleshooting

### Files Not Found

Ensure you're using the correct tmpDir:
```go
// ✅ Correct
configPath := filepath.Join(input.TmpDir, "config.yaml")

// ❌ Wrong - don't hardcode paths
configPath := "/tmp/config.yaml"
```

### Subengine Order Issues

If your subengine depends on another, ensure correct order in `forge.yaml`:
```yaml
subengines:
  - go://testenv-kind      # Must run first
  - go://testenv-mydb      # Depends on kind
```

### Resource Cleanup Fails

Make delete operations idempotent:
```go
if !exists(resourceID) {
    return &DeleteOutput{Success: true}, nil  // Already deleted
}
```

## Summary

A testenv subengine must:
1. ✅ Implement MCP `create` and `delete` tools
2. ✅ Write files to shared tmpDir (relative paths in output)
3. ✅ Return metadata for test runners
4. ✅ Track managed resources for cleanup
5. ✅ Handle errors gracefully
6. ✅ Support best-effort deletion
7. ✅ Use consistent naming conventions

Following this guide ensures your subengine integrates seamlessly with the testenv orchestrator and can be composed with other subengines to build complex test environments.
