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

---

# COMPREHENSIVE TEST RUNNER IMPLEMENTATION REFERENCE GUIDE

The following sections provide the complete, detailed test runner implementation guide. Use this as reference when helping users implement custom test runners.

---

# Test Runner Implementation Guide

## Overview

A **test runner** executes tests and generates structured reports. Test runners are responsible for invoking test frameworks, capturing output, parsing results, and producing standardized JSON reports that can be consumed by CI/CD systems and the forge CLI.

## Responsibilities

Test runners handle:
- **Executing** test frameworks with appropriate flags
- **Capturing** test output (stdout/stderr)
- **Parsing** test results and coverage data
- **Generating** structured JSON reports

Test runners do NOT:
- Create or manage test environments (that's the engine's job)
- Store test state persistently
- Manage test lifecycle beyond execution

## API Contract

### CLI Interface

Test runners must support the following command-line interface:

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
- **NAME**: Unique identifier for this test run (used for output files)

### Output Channels

**CRITICAL**: Proper output channel usage is essential:
- **stdout**: ONLY for JSON report (one line, valid JSON)
- **stderr**: All test output, progress messages, errors

This separation allows the forge CLI to:
1. Display test output in real-time (stderr)
2. Parse the structured report programmatically (stdout)

### MCP Interface

Test runners MUST support MCP mode via the `--mcp` flag.

**Required MCP Tool:**

#### `run` Tool
```json
{
  "name": "run",
  "description": "Run tests for a given stage and generate structured report",
  "inputSchema": {
    "type": "object",
    "properties": {
      "stage": {
        "type": "string",
        "description": "Test stage name (e.g., 'unit', 'integration')"
      },
      "name": {
        "type": "string",
        "description": "Test run identifier (used for output files)"
      }
    },
    "required": ["stage", "name"]
  }
}
```

**Response:**
- Success: TestReport object in Meta
- Error: `IsError: true` with error message

The runner may still set `IsError: true` if tests fail while execution succeeds. Check the `status` field in the report.

## Data Structures

### TestReport

The standardized test report structure:

```go
type TestReport struct {
    // Stage is the test stage name
    Stage string `json:"stage"`

    // Name is the test run identifier
    Name string `json:"name"`

    // Status is the overall result ("passed" or "failed")
    Status string `json:"status"`

    // StartTime is when execution began
    StartTime time.Time `json:"startTime"`

    // Duration is total execution time in seconds
    Duration float64 `json:"duration"`

    // TestStats contains execution statistics
    TestStats TestStats `json:"testStats"`

    // Coverage contains code coverage information
    Coverage Coverage `json:"coverage"`

    // OutputPath is the path to detailed output files (optional)
    OutputPath string `json:"outputPath,omitempty"`

    // ErrorMessage contains error details if execution failed (optional)
    ErrorMessage string `json:"errorMessage,omitempty"`
}

type TestStats struct {
    Total   int `json:"total"`
    Passed  int `json:"passed"`
    Failed  int `json:"failed"`
    Skipped int `json:"skipped"`
}

type Coverage struct {
    Percentage float64 `json:"percentage"`
    FilePath   string  `json:"filePath,omitempty"`
}
```

### Exit Codes

- **0**: Tests passed successfully
- **Non-zero**: Tests failed or execution error

## Implementation Pattern

### Directory Structure

```
cmd/my-test-runner/
├── main.go      # Entry point, CLI parsing
├── runner.go    # Test execution logic
├── report.go    # Report generation and parsing
└── mcp.go       # MCP server implementation
```

### main.go Template

```go
package main

import (
    "encoding/json"
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
    versionInfo = version.New("my-test-runner")
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
    case "help", "--help", "-h":
        printUsage()
    default:
        // Assume first arg is stage, second is name
        if len(os.Args) < 3 {
            fmt.Fprintf(os.Stderr, "Error: requires <STAGE> and <NAME>\n")
            printUsage()
            os.Exit(1)
        }

        stage := os.Args[1]
        name := os.Args[2]

        if err := run(stage, name); err != nil {
            fmt.Fprintf(os.Stderr, "Error: %v\n", err)
            os.Exit(1)
        }
    }
}

func printUsage() {
    fmt.Println(`my-test-runner - Run tests and generate reports

Usage:
  my-test-runner <STAGE> <NAME>
  my-test-runner --mcp
  my-test-runner version

Arguments:
  STAGE    Test stage name (e.g., "unit", "integration")
  NAME     Test run identifier

Output:
  - Test output is written to stderr
  - JSON report is written to stdout`)
}

func run(stage, name string) error {
    report, err := runTests(stage, name)
    if err != nil {
        return err
    }

    // Output JSON report to stdout
    if err := json.NewEncoder(os.Stdout).Encode(report); err != nil {
        return fmt.Errorf("failed to encode report: %w", err)
    }

    // Exit with non-zero if tests failed
    if report.Status == "failed" {
        os.Exit(1)
    }

    return nil
}
```

### runner.go Template

```go
package main

import (
    "fmt"
    "os/exec"
    "time"
)

func runTests(stage, name string) (*TestReport, error) {
    startTime := time.Now()

    // Generate output file paths
    junitFile := fmt.Sprintf(".ignore.test-%s-%s.xml", stage, name)
    coverageFile := fmt.Sprintf(".ignore.test-%s-%s-coverage.out", stage, name)

    // Build test command
    cmd := buildTestCommand(stage, name, junitFile, coverageFile)

    // IMPORTANT: Redirect test output to stderr
    cmd.Stdout = os.Stderr
    cmd.Stderr = os.Stderr

    // Execute tests
    err := cmd.Run()
    duration := time.Since(startTime).Seconds()

    // Determine status
    status := "passed"
    errorMessage := ""
    if err != nil {
        status = "failed"
        if exitErr, ok := err.(*exec.ExitError); ok {
            errorMessage = fmt.Sprintf("tests failed with exit code %d", exitErr.ExitCode())
        } else {
            errorMessage = fmt.Sprintf("failed to execute tests: %v", err)
        }
    }

    // Parse test statistics
    testStats, _ := parseTestResults(junitFile)

    // Parse coverage
    coverage, _ := parseCoverage(coverageFile)

    // Create report
    report := &TestReport{
        Stage:        stage,
        Name:         name,
        Status:       status,
        StartTime:    startTime,
        Duration:     duration,
        TestStats:    testStats,
        Coverage:     coverage,
        OutputPath:   junitFile,
        ErrorMessage: errorMessage,
    }

    return report, nil
}

func buildTestCommand(stage, name, junitFile, coverageFile string) *exec.Cmd {
    // Example for Go tests - customize for your framework
    return exec.Command("go", "test",
        "-tags", stage,
        "-race",
        "-count=1",
        "-cover",
        "-coverprofile", coverageFile,
        "./...",
    )
}
```

### report.go - Parsing Test Results

```go
package main

import (
    "encoding/xml"
    "fmt"
    "os"
    "os/exec"
    "strings"
)

// JUnit XML structures
type junitTestSuites struct {
    TestSuites []junitTestSuite `xml:"testsuite"`
}

type junitTestSuite struct {
    Tests    int `xml:"tests,attr"`
    Failures int `xml:"failures,attr"`
    Skipped  int `xml:"skipped,attr"`
}

func parseTestResults(xmlPath string) (TestStats, error) {
    data, err := os.ReadFile(xmlPath)
    if err != nil {
        return TestStats{}, err
    }

    var suites junitTestSuites
    if err := xml.Unmarshal(data, &suites); err != nil {
        return TestStats{}, err
    }

    stats := TestStats{}
    for _, suite := range suites.TestSuites {
        stats.Total += suite.Tests
        stats.Failed += suite.Failures
        stats.Skipped += suite.Skipped
    }
    stats.Passed = stats.Total - stats.Failed - stats.Skipped

    return stats, nil
}

func parseCoverage(coveragePath string) (Coverage, error) {
    if _, err := os.Stat(coveragePath); err != nil {
        return Coverage{}, err
    }

    // Use go tool cover to get percentage
    cmd := exec.Command("go", "tool", "cover", "-func", coveragePath)
    output, err := cmd.Output()
    if err != nil {
        return Coverage{FilePath: coveragePath}, err
    }

    // Parse total coverage from last line
    lines := strings.Split(string(output), "\n")
    for i := len(lines) - 1; i >= 0; i-- {
        line := strings.TrimSpace(lines[i])
        if strings.HasPrefix(line, "total:") {
            parts := strings.Fields(line)
            if len(parts) > 0 {
                percentStr := strings.TrimSuffix(parts[len(parts)-1], "%")
                var percentage float64
                fmt.Sscanf(percentStr, "%f", &percentage)
                return Coverage{
                    Percentage: percentage,
                    FilePath:   coveragePath,
                }, nil
            }
        }
    }

    return Coverage{FilePath: coveragePath}, nil
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
    "github.com/modelcontextprotocol/go-sdk/mcp"
)

type RunInput struct {
    Stage string `json:"stage"`
    Name  string `json:"name"`
}

func runMCPServer() error {
    v, _, _ := versionInfo.Get()
    server := mcpserver.New("my-test-runner", v)

    mcpserver.RegisterTool(server, &mcp.Tool{
        Name:        "run",
        Description: "Run tests and generate structured report",
    }, handleRunTool)

    return server.RunDefault()
}

func handleRunTool(
    ctx context.Context,
    req *mcp.CallToolRequest,
    input RunInput,
) (*mcp.CallToolResult, any, error) {
    log.Printf("Running tests: stage=%s name=%s", input.Stage, input.Name)

    // Validate inputs
    if input.Stage == "" {
        return &mcp.CallToolResult{
            Content: []mcp.Content{
                &mcp.TextContent{Text: "Run failed: missing 'stage'"},
            },
            IsError: true,
        }, nil, nil
    }

    if input.Name == "" {
        return &mcp.CallToolResult{
            Content: []mcp.Content{
                &mcp.TextContent{Text: "Run failed: missing 'name'"},
            },
            IsError: true,
        }, nil, nil
    }

    // Run tests
    report, err := runTests(input.Stage, input.Name)
    if err != nil {
        return &mcp.CallToolResult{
            Content: []mcp.Content{
                &mcp.TextContent{Text: fmt.Sprintf("Run failed: %v", err)},
            },
            IsError: true,
        }, nil, nil
    }

    // Create success message
    msg := fmt.Sprintf("Tests %s: %d/%d passed",
        report.Status,
        report.TestStats.Passed,
        report.TestStats.Total,
    )

    // Return result with report as artifact
    // IsError indicates test failure, not execution failure
    return &mcp.CallToolResult{
        Content: []mcp.Content{
            &mcp.TextContent{Text: msg},
        },
        IsError: report.Status == "failed",
    }, report, nil
}
```

## Framework-Specific Examples

### Go Tests (with gotestsum)

```go
func buildTestCommand(stage, name, junitFile, coverageFile string) *exec.Cmd {
    return exec.Command("go", "run", "gotest.tools/gotestsum@v1.13.0",
        "--format", "pkgname-and-test-fails",
        "--format-hide-empty-pkg",
        "--junitfile", junitFile,
        "--",
        "-tags", stage,
        "-race",
        "-count=1",
        "-short",
        "-cover",
        "-coverprofile", coverageFile,
        "./...",
    )
}
```

### Python Tests (pytest)

```go
func buildTestCommand(stage, name, junitFile, coverageFile string) *exec.Cmd {
    return exec.Command("pytest",
        "-v",
        "--junitxml="+junitFile,
        "--cov=.",
        "--cov-report=xml:"+coverageFile,
        "-m", stage,  // Use markers for stages
    )
}

func parseCoverage(coveragePath string) (Coverage, error) {
    // Parse coverage.xml for Python
    type CoverageXML struct {
        LineRate float64 `xml:"line-rate,attr"`
    }

    data, err := os.ReadFile(coveragePath)
    if err != nil {
        return Coverage{}, err
    }

    var cov CoverageXML
    if err := xml.Unmarshal(data, &cov); err != nil {
        return Coverage{}, err
    }

    return Coverage{
        Percentage: cov.LineRate * 100,
        FilePath:   coveragePath,
    }, nil
}
```

### JavaScript/TypeScript Tests (Jest)

```go
func buildTestCommand(stage, name, junitFile, coverageFile string) *exec.Cmd {
    return exec.Command("npx", "jest",
        "--testNamePattern", stage,
        "--reporters=default",
        "--reporters=jest-junit",
        "--coverage",
        "--coverageDirectory="+filepath.Dir(coverageFile),
    )
}

func parseCoverage(coveragePath string) (Coverage, error) {
    // Parse coverage-summary.json from Jest
    type JestCoverage struct {
        Total struct {
            Lines struct {
                Pct float64 `json:"pct"`
            } `json:"lines"`
        } `json:"total"`
    }

    data, err := os.ReadFile(coveragePath + "/coverage-summary.json")
    if err != nil {
        return Coverage{}, err
    }

    var cov JestCoverage
    if err := json.Unmarshal(data, &cov); err != nil {
        return Coverage{}, err
    }

    return Coverage{
        Percentage: cov.Total.Lines.Pct,
        FilePath:   coveragePath,
    }, nil
}
```

### Shell Script Tests

```go
func buildTestCommand(stage, name, junitFile, coverageFile string) *exec.Cmd {
    return exec.Command("bash", "./scripts/run-tests.sh",
        stage,
        junitFile,
    )
}

// Shell script should generate JUnit XML
func parseTestResults(xmlPath string) (TestStats, error) {
    // Same JUnit XML parsing as above
    // ...
}

func parseCoverage(coveragePath string) (Coverage, error) {
    // Coverage not applicable for shell scripts
    return Coverage{}, nil
}
```

## Best Practices

### 1. Output Separation

**CRITICAL**: Never write to stdout except for the final JSON report:

```go
// ✅ Correct
fmt.Fprintf(os.Stderr, "Running tests...\n")
json.NewEncoder(os.Stdout).Encode(report)

// ❌ Wrong - pollutes stdout
fmt.Println("Running tests...")
```

### 2. Artifact Management

Use consistent naming for test artifacts:

```go
junitFile := fmt.Sprintf(".ignore.test-%s-%s.xml", stage, name)
coverageFile := fmt.Sprintf(".ignore.test-%s-%s-coverage.out", stage, name)
```

The `.ignore.` prefix ensures git ignores these files.

### 3. Error Handling

Distinguish between execution errors and test failures:

```go
// Execution error (can't run tests)
if err != nil && !isTestFailure(err) {
    return nil, fmt.Errorf("execution failed: %w", err)
}

// Test failure (tests ran but failed)
report.Status = "failed"
report.ErrorMessage = "tests failed"
```

### 4. Timeout Handling

Add timeouts for long-running tests:

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
defer cancel()

cmd := exec.CommandContext(ctx, "go", "test", "./...")
```

### 5. Test Isolation

Use unique names to avoid conflicts:

```go
// Each run gets unique artifact files
name := fmt.Sprintf("%s-%s", stage, time.Now().Format("20060102-150405"))
```

## Integration with Forge

### forge.yaml Configuration

```yaml
test:
  - name: unit
    engine: "noop"
    runner: "go://my-test-runner"

  - name: integration
    engine: "go://test-integration"
    runner: "go://my-test-runner"
```

### Invocation

Forge will call your runner via MCP:

```go
result, err := callMCPEngine(runnerPath, "run", map[string]any{
    "stage": "unit",
    "name":  "unit-20241103-123456",
})
```

The runner must:
1. Execute tests
2. Generate test artifacts
3. Parse results
4. Return TestReport in Meta

## Testing Your Runner

### Manual Testing

```bash
# Build
go build -o ./build/bin/my-test-runner ./cmd/my-test-runner

# Run tests (output to stderr, JSON to stdout)
./build/bin/my-test-runner unit test-001 2>test.log | jq .

# Check test output
cat test.log

# Verify JSON report
./build/bin/my-test-runner unit test-002 | jq '.testStats'
```

### Verify MCP Mode

```bash
# Start MCP server (blocks)
./build/bin/my-test-runner --mcp

# In another terminal, test with forge
forge test unit run
```

## Troubleshooting

### No JSON Output

Check that you're writing ONLY the report to stdout:

```go
// Ensure all logging goes to stderr
log.SetOutput(os.Stderr)

// Final line should be JSON to stdout
json.NewEncoder(os.Stdout).Encode(report)
```

### Invalid JSON

Validate the report structure:

```bash
./build/bin/my-test-runner unit test | jq empty
```

If jq errors, your JSON is malformed.

### Test Output Mixed with Report

Ensure test command outputs to stderr:

```go
cmd.Stdout = os.Stderr  // ✅
cmd.Stderr = os.Stderr  // ✅
```

### Coverage Parsing Fails

Make coverage parsing resilient:

```go
coverage, err := parseCoverage(coverageFile)
if err != nil {
    // Don't fail - just return 0% coverage
    coverage = Coverage{FilePath: coverageFile}
}
```

## Summary

A test runner must:
1. ✅ Accept stage and name parameters
2. ✅ Execute tests with appropriate framework
3. ✅ Write test output to stderr
4. ✅ Write JSON report to stdout
5. ✅ Parse test results (JUnit XML or equivalent)
6. ✅ Parse coverage data (if applicable)
7. ✅ Support MCP mode with `run` tool
8. ✅ Return proper exit codes

Following this guide ensures your test runner integrates seamlessly with the forge test infrastructure and works with any test framework.

## Reference Implementation

See `cmd/test-runner-go` for a complete reference implementation using gotestsum for Go tests.
