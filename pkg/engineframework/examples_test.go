//go:build unit

// Copyright 2024 Alexandre Mahdhaoui
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package engineframework_test

import (
	"context"
	"fmt"

	"github.com/alexandremahdhaoui/forge/pkg/engineframework"
	"github.com/alexandremahdhaoui/forge/pkg/forge"
	"github.com/alexandremahdhaoui/forge/pkg/mcpserver"
	"github.com/alexandremahdhaoui/forge/pkg/mcptypes"
)

// Example_builder demonstrates creating a simple builder using the framework.
//
// This example shows:
//   - Using engineframework for MCP tool registration
//   - Extracting spec configuration
//   - Creating artifacts
func Example_builder() {
	// runMCPServer sets up the MCP server with builder tools
	runMCPServer := func() error {
		server := mcpserver.New("example-builder", "1.0.0")

		// Configure builder with framework
		config := engineframework.BuilderConfig{
			Name:    "example-builder",
			Version: "1.0.0",
			BuildFunc: func(ctx context.Context, input mcptypes.BuildInput) ([]forge.Artifact, error) {
				// Extract spec configuration
				outputDir := engineframework.ExtractStringWithDefault(input.Spec, "outputDir", "./build")

				fmt.Printf("Building %s in %s\n", input.Name, outputDir)

				// Create versioned artifact (uses git commit SHA)
				artifact := engineframework.CreateArtifact(input.Name, forge.TypeBinary, outputDir+"/"+input.Name)

				return engineframework.One(artifact), nil
			},
		}

		// Register builder tools (registers both 'build' and 'buildBatch')
		if err := engineframework.RegisterBuilderTools(server, config); err != nil {
			return err
		}

		fmt.Println("Builder tools registered successfully")
		return nil
	}

	// In a real engine, main.go would use cli.Bootstrap()
	// For this example, we just call runMCPServer directly
	if err := runMCPServer(); err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Output:
	// Builder tools registered successfully
}

// Example_testRunner demonstrates creating a test runner using the framework.
//
// This example shows:
//   - Using TestRunnerConfig for test execution
//   - Handling test failures vs execution errors
//   - Returning TestReport even when tests fail
func Example_testRunner() {
	runMCPServer := func() error {
		server := mcpserver.New("example-test-runner", "1.0.0")

		// Configure test runner with framework
		config := engineframework.TestRunnerConfig{
			Name:    "example-test-runner",
			Version: "1.0.0",
			RunTestFunc: func(ctx context.Context, input mcptypes.RunInput) (*forge.TestReport, error) {
				fmt.Printf("Running tests for stage %s\n", input.Stage)

				// Create test report
				// CRITICAL: Return report even if tests fail
				report := &forge.TestReport{
					Stage:  input.Stage,
					Status: "passed",
					TestStats: forge.TestStats{
						Total:  10,
						Passed: 10,
						Failed: 0,
					},
				}

				return report, nil
			},
		}

		// Register test runner tools (registers 'run')
		if err := engineframework.RegisterTestRunnerTools(server, config); err != nil {
			return err
		}

		fmt.Println("Test runner tools registered successfully")
		return nil
	}

	if err := runMCPServer(); err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Output:
	// Test runner tools registered successfully
}

// Example_testEnvSubengine demonstrates creating a test environment subengine using the framework.
//
// This example shows:
//   - Using TestEnvSubengineConfig for resource provisioning
//   - Creating TestEnvArtifact with files, metadata, and managed resources
//   - Best-effort cleanup in DeleteFunc
func Example_testEnvSubengine() {
	runMCPServer := func() error {
		server := mcpserver.New("example-testenv", "1.0.0")

		// Configure testenv subengine with framework
		config := engineframework.TestEnvSubengineConfig{
			Name:    "example-testenv",
			Version: "1.0.0",
			CreateFunc: func(ctx context.Context, input engineframework.CreateInput) (*engineframework.TestEnvArtifact, error) {
				// Extract spec configuration
				version := engineframework.ExtractStringWithDefault(input.Spec, "version", "latest")

				resourceName := fmt.Sprintf("example-%s", input.TestID)
				fmt.Printf("Creating resource %s (version: %s)\n", resourceName, version)

				// Return artifact with files, metadata, and managed resources
				return &engineframework.TestEnvArtifact{
					TestID: input.TestID,
					Files: map[string]string{
						"example.config": "config.yaml",
					},
					Metadata: map[string]string{
						"example.resourceName": resourceName,
						"example.version":      version,
					},
					ManagedResources: []string{input.TmpDir + "/config.yaml"},
				}, nil
			},
			DeleteFunc: func(ctx context.Context, input engineframework.DeleteInput) error {
				// Best-effort cleanup
				resourceName := input.Metadata["example.resourceName"]
				fmt.Printf("Deleting resource %s\n", resourceName)
				return nil
			},
		}

		// Register testenv subengine tools (registers 'create' and 'delete')
		if err := engineframework.RegisterTestEnvSubengineTools(server, config); err != nil {
			return err
		}

		fmt.Println("TestEnv subengine tools registered successfully")
		return nil
	}

	if err := runMCPServer(); err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Output:
	// TestEnv subengine tools registered successfully
}

// Example_specExtraction demonstrates using spec extraction utilities.
//
// This example shows:
//   - Extracting typed values from map[string]any
//   - Using defaults for missing values
//   - Handling JSON unmarshal edge cases
func Example_specExtraction() {
	// Example spec from MCP input
	spec := map[string]any{
		"outputDir": "/tmp/build",
		"timeout":   30,
		"enabled":   true,
		"tags":      []any{"test", "integration"},  // JSON unmarshal gives []any
		"labels":    map[string]any{"env": "prod"}, // JSON unmarshal gives map[string]any
	}

	// Extract string
	outputDir := engineframework.ExtractStringWithDefault(spec, "outputDir", "./build")
	fmt.Printf("outputDir: %s\n", outputDir)

	// Extract int
	timeout := engineframework.ExtractIntWithDefault(spec, "timeout", 60)
	fmt.Printf("timeout: %d\n", timeout)

	// Extract bool
	enabled, _ := engineframework.ExtractBool(spec, "enabled")
	fmt.Printf("enabled: %t\n", enabled)

	// Extract string slice (handles JSON unmarshal edge case)
	tags, _ := engineframework.ExtractStringSlice(spec, "tags")
	fmt.Printf("tags: %v\n", tags)

	// Extract string map (handles JSON unmarshal edge case)
	labels, _ := engineframework.ExtractStringMap(spec, "labels")
	fmt.Printf("labels: %v\n", labels)

	// Extract with default when key is missing
	buildDir := engineframework.ExtractStringWithDefault(spec, "buildDir", "./dist")
	fmt.Printf("buildDir (using default): %s\n", buildDir)

	// Output:
	// outputDir: /tmp/build
	// timeout: 30
	// enabled: true
	// tags: [test integration]
	// labels: map[env:prod]
	// buildDir (using default): ./dist
}

// Example_versionUtilities demonstrates using git versioning utilities.
//
// This example shows:
//   - Creating versioned artifacts (with git commit SHA)
//   - Creating artifacts without version (for generated code)
//   - Creating artifacts with custom version
func Example_versionUtilities() {
	// Create artifact without version (for generated code)
	generatedArtifact := engineframework.CreateArtifact(
		"openapi-client",
		"generated",
		"./pkg/generated",
	)
	fmt.Printf("Generated artifact version: %q\n", generatedArtifact.Version)

	// Create artifact with custom version
	containerArtifact := engineframework.CreateCustomArtifact(
		"my-app",
		"container",
		"localhost:5000/my-app:v1.2.3",
		"v1.2.3",
	)
	fmt.Printf("Container artifact version: %s\n", containerArtifact.Version)

	// Note: CreateVersionedArtifact would use git commit SHA
	// artifact, err := engineframework.CreateVersionedArtifact("my-app", "binary", "./build/bin/my-app")
	// In this example environment, git may not be available, so we skip this

	// Output:
	// Generated artifact version: ""
	// Container artifact version: v1.2.3
}

// Example_builderWithSpecExtraction demonstrates a realistic builder implementation.
//
// This example shows:
//   - Complete builder setup
//   - Spec extraction for configuration
//   - Creating artifacts
func Example_builderWithSpecExtraction() {
	buildFunc := func(ctx context.Context, input mcptypes.BuildInput) ([]forge.Artifact, error) {
		// Extract configuration from spec
		outputDir := engineframework.ExtractStringWithDefault(input.Spec, "outputDir", "./build")
		stripDebug := engineframework.ExtractBoolWithDefault(input.Spec, "stripDebug", false)
		buildTags, _ := engineframework.ExtractStringSlice(input.Spec, "buildTags")

		fmt.Printf("Building %s:\n", input.Name)
		fmt.Printf("  Output: %s\n", outputDir)
		fmt.Printf("  Strip debug: %t\n", stripDebug)
		fmt.Printf("  Build tags: %v\n", buildTags)

		// Simulate build
		artifactPath := outputDir + "/" + input.Name

		// Create artifact (without version for this example)
		return engineframework.One(engineframework.CreateArtifact(input.Name, forge.TypeBinary, artifactPath)), nil
	}

	runMCPServer := func() error {
		server := mcpserver.New("advanced-builder", "1.0.0")

		config := engineframework.BuilderConfig{
			Name:      "advanced-builder",
			Version:   "1.0.0",
			BuildFunc: buildFunc,
		}

		if err := engineframework.RegisterBuilderTools(server, config); err != nil {
			return err
		}

		return nil
	}

	// Simulate calling buildFunc directly
	spec := map[string]any{
		"outputDir":  "/tmp/output",
		"stripDebug": true,
		"buildTags":  []any{"integration", "postgres"},
	}

	_, _ = buildFunc(context.Background(), mcptypes.BuildInput{
		Name:   "my-app",
		Engine: "forge://advanced-builder",
		Spec:   spec,
	})

	if err := runMCPServer(); err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Output:
	// Building my-app:
	//   Output: /tmp/output
	//   Strip debug: true
	//   Build tags: [integration postgres]
}

// Example_testRunnerWithFailures demonstrates handling test failures correctly.
//
// This example shows:
//   - Distinguishing between test failures and execution errors
//   - Returning TestReport with Status="failed" for test failures
//   - Only returning error for execution failures
func Example_testRunnerWithFailures() {
	runTestFunc := func(ctx context.Context, input mcptypes.RunInput) (*forge.TestReport, error) {
		// Simulate running tests
		fmt.Printf("Running tests for stage: %s\n", input.Stage)

		// Simulate parsing test output
		totalTests := 100
		failedTests := 5

		// CRITICAL: Even if tests failed, return a report (not an error)
		report := &forge.TestReport{
			Stage: input.Stage,
			TestStats: forge.TestStats{
				Total:  totalTests,
				Passed: totalTests - failedTests,
				Failed: failedTests,
			},
		}

		// Set status based on test results
		if failedTests > 0 {
			report.Status = "failed"
			report.ErrorMessage = fmt.Sprintf("%d tests failed", failedTests)
			fmt.Printf("Tests failed: %s\n", report.ErrorMessage)
		} else {
			report.Status = "passed"
			fmt.Println("All tests passed")
		}

		// Return report even if tests failed
		// Framework will use ErrorResultWithArtifact for failed tests
		return report, nil

		// Only return error if we couldn't execute tests at all:
		// return nil, fmt.Errorf("failed to execute tests: %w", executionErr)
	}

	// Simulate running tests
	report, err := runTestFunc(context.Background(), mcptypes.RunInput{
		Stage: "integration",
		Name:  "example-test-runner",
	})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Report status: %s\n", report.Status)

	// Output:
	// Running tests for stage: integration
	// Tests failed: 5 tests failed
	// Report status: failed
}
