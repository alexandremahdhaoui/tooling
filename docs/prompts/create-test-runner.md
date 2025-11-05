# Creating a Custom Test Runner

You are helping a user create a custom test runner for forge. A test runner executes tests and generates structured JSON reports.

## What is a Test Runner?

A **test runner** is a component that:
- Executes test frameworks with appropriate flags
- Captures test output (stdout/stderr)
- Parses test results and coverage data
- Generates structured JSON reports (`TestReport`)

Test runners do NOT manage test environments - that's the test engine's job.

## When to Create a Custom Test Runner

Create a custom test runner when you need to:
- ✅ Integrate a test framework not covered by existing runners
- ✅ Parse custom test output formats
- ✅ Generate specialized test reports
- ✅ Implement custom test execution logic
- ✅ Add test result enrichment or metrics

Use `generic-test-runner` for simple CLI tools where exit code determines pass/fail.

## API Contract

### CLI Interface

Your test runner must support this command:

```bash
# Run tests
<runner-binary> <STAGE> <NAME>
# Output: JSON report to stdout, test output to stderr

# Version information
<runner-binary> version

# MCP server mode (required)
<runner-binary> --mcp
```

### Input Parameters

- **STAGE**: Test stage name (e.g., "unit", "integration", "e2e")
- **NAME**: Unique identifier for this test run

### Output Channels

**CRITICAL**: Proper output channel usage is essential:
- **stdout**: ONLY for JSON report (one line, valid JSON)
- **stderr**: All test output, progress messages, errors

This separation allows forge to:
1. Display test output in real-time (stderr)
2. Capture the structured report (stdout)
3. Store the report in the artifact store

### TestReport Structure

Your JSON output must match this structure:

```json
{
  "id": "unique-report-id",
  "stage": "unit",
  "status": "passed",
  "startTime": "2024-01-01T12:00:00Z",
  "duration": 45.2,
  "testStats": {
    "total": 150,
    "passed": 148,
    "failed": 2,
    "skipped": 0
  },
  "coverage": {
    "percentage": 85.5,
    "filePath": ".forge/coverage/unit-coverage.out"
  },
  "artifactFiles": [
    ".forge/test-results/unit-report.xml",
    ".forge/coverage/unit-coverage.out"
  ],
  "outputPath": ".forge/test-results/unit-output.log",
  "errorMessage": "",
  "createdAt": "2024-01-01T12:00:00Z",
  "updatedAt": "2024-01-01T12:00:45Z"
}
```

## Implementation Steps

### Step 1: Set Up Project Structure

Start from the `test-runner-go` template:

```bash
# Copy the template
cp -r cmd/test-runner-go cmd/<your-runner-name>

# Update the Name constant in main.go
const Name = "<your-runner-name>"
```

### Step 2: Implement Test Execution

Create a `runner.go` file with the main test execution logic:

```go
type TestRunner struct {
    stage      string
    name       string
    config     *forge.TestSpec
    startTime  time.Time
}

func (r *TestRunner) Run() (*forge.TestReport, error) {
    r.startTime = time.Now()

    // 1. Prepare test command
    cmd := r.buildTestCommand()

    // 2. Capture output
    var stderr bytes.Buffer
    cmd.Stderr = &stderr

    // 3. Execute tests
    err := cmd.Run()
    duration := time.Since(r.startTime).Seconds()

    // 4. Parse results
    stats, coverage := r.parseResults()

    // 5. Build report
    report := &forge.TestReport{
        ID:         generateReportID(r.stage, r.name),
        Stage:      r.stage,
        Status:     determineStatus(err, stats),
        StartTime:  r.startTime,
        Duration:   duration,
        TestStats:  stats,
        Coverage:   coverage,
        // ... other fields
    }

    return report, nil
}
```

### Step 3: Parse Test Framework Output

Different test frameworks have different output formats. Implement parsers for your framework:

```go
// Example: Parse JUnit XML
func parseJUnitXML(xmlPath string) (forge.TestStats, error) {
    data, err := os.ReadFile(xmlPath)
    if err != nil {
        return forge.TestStats{}, err
    }

    var suites JUnitTestSuites
    if err := xml.Unmarshal(data, &suites); err != nil {
        return forge.TestStats{}, err
    }

    stats := forge.TestStats{
        Total:   suites.Tests,
        Passed:  suites.Tests - suites.Failures - suites.Errors - suites.Skipped,
        Failed:  suites.Failures + suites.Errors,
        Skipped: suites.Skipped,
    }

    return stats, nil
}

// Example: Parse coverage report
func parseCoverageProfile(profilePath string) (forge.Coverage, error) {
    profiles, err := cover.ParseProfiles(profilePath)
    if err != nil {
        return forge.Coverage{}, err
    }

    total, covered := 0, 0
    for _, profile := range profiles {
        for _, block := range profile.Blocks {
            total += block.NumStmt
            if block.Count > 0 {
                covered += block.NumStmt
            }
        }
    }

    percentage := 0.0
    if total > 0 {
        percentage = float64(covered) / float64(total) * 100
    }

    return forge.Coverage{
        Percentage: percentage,
        FilePath:   profilePath,
    }, nil
}
```

### Step 4: Handle Output Channels Correctly

```go
func runTests(stage, name string) error {
    // Create test runner
    runner := NewTestRunner(stage, name)

    // Run tests (outputs to stderr)
    report, err := runner.Run()

    // Store report in artifact store
    storeTestReport(report)

    // Output JSON report to stdout (ONLY THIS goes to stdout)
    reportJSON, _ := json.Marshal(report)
    fmt.Println(string(reportJSON))

    // Return error if tests failed
    if report.Status == "failed" {
        return fmt.Errorf("tests failed")
    }

    return nil
}
```

### Step 5: Implement Artifact File Management

```go
func (r *TestRunner) createArtifactFiles() ([]string, error) {
    var files []string

    // Create output directory
    outputDir := ".forge/test-results"
    os.MkdirAll(outputDir, 0755)

    // Save test output
    outputPath := filepath.Join(outputDir, fmt.Sprintf("%s-output.log", r.name))
    if err := os.WriteFile(outputPath, r.output, 0644); err != nil {
        return nil, err
    }
    files = append(files, outputPath)

    // Save JUnit XML (if applicable)
    if r.junitXML != "" {
        junitPath := filepath.Join(outputDir, fmt.Sprintf("%s-report.xml", r.name))
        if err := os.WriteFile(junitPath, []byte(r.junitXML), 0644); err != nil {
            return nil, err
        }
        files = append(files, junitPath)
    }

    // Save coverage report (if applicable)
    if r.coverageFile != "" {
        files = append(files, r.coverageFile)
    }

    return files, nil
}
```

### Step 6: Implement MCP Server

Follow the pattern in `test-runner-go/mcp.go`:

```go
func handleRunTool(ctx context.Context, req *mcp.CallToolRequest, input RunInput) (*mcp.CallToolResult, any, error) {
    // 1. Run tests
    report, err := runTests(input.Stage, input.Name)

    // 2. Return report as artifact
    if err != nil {
        return mcputil.ErrorResult(fmt.Sprintf("Tests failed: %v", err)), report, nil
    }

    result, returnedReport := mcputil.SuccessResultWithArtifact(
        fmt.Sprintf("Tests passed: %s", input.Stage),
        report,
    )
    return result, returnedReport, nil
}
```

### Step 7: Store Reports in Artifact Store

```go
func storeTestReport(report *forge.TestReport) error {
    artifactStorePath, _ := forge.GetArtifactStorePath(".forge/artifacts.yaml")
    store, _ := forge.ReadOrCreateArtifactStore(artifactStorePath)

    forge.AddOrUpdateTestReport(&store, report)

    return forge.WriteArtifactStore(artifactStorePath, store)
}
```

### Step 8: Configure in forge.yaml

```yaml
test:
  - name: unit
    engine: go://test-integration
    runner: go://<your-runner-name>
```

## Best Practices

1. **Output Separation**: ONLY JSON goes to stdout, everything else to stderr
2. **Status Accuracy**: Set status based on actual test results, not just exit code
3. **Error Context**: Include helpful error messages in `errorMessage` field
4. **Artifact Files**: List all created files in `artifactFiles` array
5. **Coverage Data**: Include coverage when available
6. **Timestamps**: Use UTC time and RFC3339 format
7. **ID Generation**: Use consistent format: `test-<stage>-<timestamp>-<random>`

## Testing Your Runner

```bash
# Build the runner
forge build <your-runner-name>

# Test directly
./build/bin/<your-runner-name> unit test-run-1 2>test-output.log 1>report.json

# View JSON report
cat report.json | jq .

# View test output
cat test-output.log
```

## Integration with Forge

Once your runner is ready:

```bash
# Use with forge test
forge test run unit
forge test run integration

# View reports
forge test report list
forge test report get <report-id>
```

## Common Patterns

### Pattern 1: Go Tests

```go
func runGoTests(stage string) (forge.TestStats, forge.Coverage, error) {
    // Run tests with coverage
    cmd := exec.Command("go", "test", "-v", "-tags="+stage,
        "-coverprofile=coverage.out", "-json", "./...")

    // Parse JSON output
    // Return stats and coverage
}
```

### Pattern 2: Python Tests (pytest)

```go
func runPytestTests(stage string) (forge.TestStats, error) {
    // Run pytest with JUnit XML output
    cmd := exec.Command("pytest", "--junitxml=results.xml")

    // Parse XML
    // Return stats
}
```

### Pattern 3: JavaScript Tests (Jest)

```go
func runJestTests(stage string) (forge.TestStats, forge.Coverage, error) {
    // Run jest with JSON output
    cmd := exec.Command("npm", "test", "--", "--json")

    // Parse JSON
    // Return stats and coverage
}
```

## Examples

- **test-runner-go**: Reference implementation in `cmd/test-runner-go`
- **generic-test-runner**: Simple exit-code-based runner in `cmd/generic-test-runner`

## Need Help?

- Review `cmd/test-runner-go` for a complete working example
- Check the TestReport structure in `pkg/forge/artifact_store.go`
- The forge CLI handles MCP communication - focus on test execution and parsing
