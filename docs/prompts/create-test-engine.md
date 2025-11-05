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

Use the built-in `test-integration` engine for simple needs.

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

Start from the `test-integration` template:

```bash
# Copy the template
cp -r cmd/test-integration cmd/<your-engine-name>

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

Follow the pattern in `test-integration/mcp.go`:

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
# Should output: test-integration-20240101-abcd1234

# Test get
./build/bin/<your-engine-name> get test-integration-20240101-abcd1234

# Test list
./build/bin/<your-engine-name> list

# Test delete
./build/bin/<your-engine-name> delete test-integration-20240101-abcd1234
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

- **test-integration**: Reference implementation in `cmd/test-integration`
- **kindenv**: Manages Kind Kubernetes clusters in `cmd/kindenv`

## Need Help?

- Review `cmd/test-integration` for a complete working example
- Check existing test engine implementations
- The forge CLI handles MCP communication - focus on your infrastructure logic
