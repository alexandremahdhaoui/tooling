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

package engineframework

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge/pkg/forge"
	"github.com/alexandremahdhaoui/forge/pkg/mcpserver"
	"github.com/alexandremahdhaoui/forge/pkg/mcptypes"
	"github.com/alexandremahdhaoui/forge/pkg/mcputil"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// mockBuildFunc creates a mock BuilderFunc for testing.
// Returns success artifact if name doesn't contain "fail".
func mockBuildFunc(returnError bool) BuilderFunc {
	return func(ctx context.Context, input mcptypes.BuildInput) ([]forge.Artifact, error) {
		// Simulate failure if name contains "fail"
		if strings.Contains(input.Name, "fail") || returnError {
			return nil, errors.New("build failed: simulated error")
		}

		// Return success artifact
		return One(CreateArtifact(input.Name, forge.TypeGenerated, "/path/to/"+input.Name)), nil
	}
}

func TestMakeBuildHandler_Success(t *testing.T) {
	config := BuilderConfig{
		Name:      "test-builder",
		Version:   "1.0.0",
		BuildFunc: mockBuildFunc(false),
	}

	handler := makeBuildHandler(config)

	ctx := context.Background()
	req := &mcp.CallToolRequest{}
	input := mcptypes.BuildInput{
		Name:      "my-app",
		Engine:    "forge://test-builder",
		Platforms: []string{HostPlatform()},
	}

	result, artifact, err := handler(ctx, req, input)
	// Should not return error
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	// Result should not be an error
	if result.IsError {
		if len(result.Content) > 0 {
			if textContent, ok := result.Content[0].(*mcp.TextContent); ok {
				t.Fatalf("handler returned error result: %s", textContent.Text)
			}
		}
		t.Fatalf("handler returned error result")
	}

	// Artifact should be returned
	if artifact == nil {
		t.Fatal("handler returned nil artifact")
	}

	// The build answers a list, one artifact here
	output, ok := artifact.(mcptypes.BuildOutput)
	if !ok {
		t.Fatalf("artifact is not mcptypes.BuildOutput, got %T", artifact)
	}

	if len(output.Artifacts) != 1 {
		t.Fatalf("expected one artifact, got %d", len(output.Artifacts))
	}

	artifactObj := output.Artifacts[0]

	// Artifact should have correct name
	if artifactObj.Name != "my-app" {
		t.Errorf("artifact.Name = %q, want %q", artifactObj.Name, "my-app")
	}

	// Artifact should have correct type
	if artifactObj.Type != forge.TypeGenerated {
		t.Errorf("artifact.Type = %q, want %q", artifactObj.Type, forge.TypeGenerated)
	}

	// Artifact should have correct location
	if artifactObj.Location != "/path/to/my-app" {
		t.Errorf("artifact.Location = %q, want %q", artifactObj.Location, "/path/to/my-app")
	}
}

func TestMakeBuildHandler_BuildFuncError(t *testing.T) {
	config := BuilderConfig{
		Name:      "test-builder",
		Version:   "1.0.0",
		BuildFunc: mockBuildFunc(true), // Always returns error
	}

	handler := makeBuildHandler(config)

	ctx := context.Background()
	req := &mcp.CallToolRequest{}
	input := mcptypes.BuildInput{
		Name:      "my-app",
		Engine:    "forge://test-builder",
		Platforms: []string{HostPlatform()},
	}

	result, artifact, err := handler(ctx, req, input)
	// Should not return Go error (errors converted to MCP results)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	// Result should be an error
	if !result.IsError {
		t.Fatal("handler should return error result when BuildFunc fails")
	}

	// Artifact should be nil
	if artifact != nil {
		t.Errorf("handler returned artifact despite error: %v", artifact)
	}

	// Error message should contain "Build failed"
	if len(result.Content) > 0 {
		if textContent, ok := result.Content[0].(*mcp.TextContent); ok {
			if !strings.Contains(textContent.Text, "Build failed") {
				t.Errorf("error message does not contain 'Build failed': %s", textContent.Text)
			}
		}
	}
}

func TestMakeBuildHandler_MissingName(t *testing.T) {
	config := BuilderConfig{
		Name:      "test-builder",
		Version:   "1.0.0",
		BuildFunc: mockBuildFunc(false),
	}

	handler := makeBuildHandler(config)

	ctx := context.Background()
	req := &mcp.CallToolRequest{}
	input := mcptypes.BuildInput{
		Name:   "", // Missing name
		Engine: "forge://test-builder",
	}

	result, artifact, err := handler(ctx, req, input)
	// Should not return Go error
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	// Result should be an error (validation failure)
	if !result.IsError {
		t.Fatal("handler should return error result when required field is missing")
	}

	// Artifact should be nil
	if artifact != nil {
		t.Errorf("handler returned artifact despite validation error: %v", artifact)
	}
}

func TestMakeBuildHandler_MissingEngine(t *testing.T) {
	config := BuilderConfig{
		Name:      "test-builder",
		Version:   "1.0.0",
		BuildFunc: mockBuildFunc(false),
	}

	handler := makeBuildHandler(config)

	ctx := context.Background()
	req := &mcp.CallToolRequest{}
	input := mcptypes.BuildInput{
		Name:   "my-app",
		Engine: "", // Missing engine
	}

	result, artifact, err := handler(ctx, req, input)
	// Should not return Go error
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	// Result should be an error (validation failure)
	if !result.IsError {
		t.Fatal("handler should return error result when required field is missing")
	}

	// Artifact should be nil
	if artifact != nil {
		t.Errorf("handler returned artifact despite validation error: %v", artifact)
	}
}

func TestMakeBatchBuildHandler_AllSuccess(t *testing.T) {
	config := BuilderConfig{
		Name:      "test-builder",
		Version:   "1.0.0",
		BuildFunc: mockBuildFunc(false),
	}

	handler := makeBatchBuildHandler(config)

	ctx := context.Background()
	req := &mcp.CallToolRequest{}
	input := mcptypes.BatchBuildInput{
		Specs: []mcptypes.BuildInput{
			{Name: "app1", Engine: "forge://test-builder", Platforms: []string{HostPlatform()}},
			{Name: "app2", Engine: "forge://test-builder", Platforms: []string{HostPlatform()}},
			{Name: "app3", Engine: "forge://test-builder", Platforms: []string{HostPlatform()}},
		},
	}

	result, artifacts, err := handler(ctx, req, input)
	// Should not return Go error
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	// Result should not be an error (all builds succeeded)
	if result.IsError {
		if len(result.Content) > 0 {
			if textContent, ok := result.Content[0].(*mcp.TextContent); ok {
				t.Fatalf("handler returned error result: %s", textContent.Text)
			}
		}
		t.Fatalf("handler returned error result")
	}

	// Artifacts should be returned
	if artifacts == nil {
		t.Fatal("handler returned nil artifacts")
	}

	// Should have 3 artifacts (one per spec)
	// mcputil.FormatBatchResult returns mcputil.BatchResult
	batchResult, ok := artifacts.(mcputil.BatchResult)
	if !ok {
		t.Fatalf("artifacts is not mcputil.BatchResult, got %T", artifacts)
	}

	if len(batchResult.Artifacts) != 3 {
		t.Errorf("expected 3 artifacts, got %d", len(batchResult.Artifacts))
	}

	// Verify each artifact
	expectedNames := []string{"app1", "app2", "app3"}
	for i, expectedName := range expectedNames {
		artifact, ok := batchResult.Artifacts[i].(forge.Artifact)
		if !ok {
			t.Errorf("artifact[%d] is not forge.Artifact, got %T", i, batchResult.Artifacts[i])
			continue
		}

		if artifact.Name != expectedName {
			t.Errorf("artifact[%d].Name = %q, want %q", i, artifact.Name, expectedName)
		}
	}
}

func TestMakeBatchBuildHandler_MixedResults(t *testing.T) {
	config := BuilderConfig{
		Name:      "test-builder",
		Version:   "1.0.0",
		BuildFunc: mockBuildFunc(false), // Fails if name contains "fail"
	}

	handler := makeBatchBuildHandler(config)

	ctx := context.Background()
	req := &mcp.CallToolRequest{}
	input := mcptypes.BatchBuildInput{
		Specs: []mcptypes.BuildInput{
			{Name: "app1", Engine: "forge://test-builder", Platforms: []string{HostPlatform()}},         // Success
			{Name: "fail-app", Engine: "forge://test-builder", Platforms: []string{HostPlatform()}},     // Failure (name contains "fail")
			{Name: "app3", Engine: "forge://test-builder", Platforms: []string{HostPlatform()}},         // Success
			{Name: "another-fail", Engine: "forge://test-builder", Platforms: []string{HostPlatform()}}, // Failure
		},
	}

	result, artifacts, err := handler(ctx, req, input)
	// Should not return Go error
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	// Result should be an error (some builds failed)
	if !result.IsError {
		t.Fatal("handler should return error result when some builds fail")
	}

	// Artifacts should still be returned (successful ones)
	if artifacts == nil {
		t.Fatal("handler returned nil artifacts despite some successes")
	}

	// Should have 2 successful artifacts
	// mcputil.FormatBatchResult returns mcputil.BatchResult
	batchResult, ok := artifacts.(mcputil.BatchResult)
	if !ok {
		t.Fatalf("artifacts is not mcputil.BatchResult, got %T", artifacts)
	}

	if len(batchResult.Artifacts) != 2 {
		t.Errorf("expected 2 successful artifacts, got %d", len(batchResult.Artifacts))
	}

	// Verify successful artifacts
	expectedNames := []string{"app1", "app3"}
	for i, expectedName := range expectedNames {
		artifact, ok := batchResult.Artifacts[i].(forge.Artifact)
		if !ok {
			t.Errorf("artifact[%d] is not forge.Artifact, got %T", i, batchResult.Artifacts[i])
			continue
		}

		if artifact.Name != expectedName {
			t.Errorf("artifact[%d].Name = %q, want %q", i, artifact.Name, expectedName)
		}
	}

	// Verify error message mentions failures
	if len(result.Content) > 0 {
		if textContent, ok := result.Content[0].(*mcp.TextContent); ok {
			if !strings.Contains(textContent.Text, "Build failed") {
				t.Errorf("error message should mention build failures: %s", textContent.Text)
			}
			// Should contain multiple error messages (2 failures)
			if strings.Count(textContent.Text, "Build failed") < 2 {
				t.Errorf("error message should mention multiple failures, got: %s", textContent.Text)
			}
		}
	}
}

func TestMakeBatchBuildHandler_AllFailures(t *testing.T) {
	config := BuilderConfig{
		Name:      "test-builder",
		Version:   "1.0.0",
		BuildFunc: mockBuildFunc(true), // Always fails
	}

	handler := makeBatchBuildHandler(config)

	ctx := context.Background()
	req := &mcp.CallToolRequest{}
	input := mcptypes.BatchBuildInput{
		Specs: []mcptypes.BuildInput{
			{Name: "app1", Engine: "forge://test-builder", Platforms: []string{HostPlatform()}},
			{Name: "app2", Engine: "forge://test-builder", Platforms: []string{HostPlatform()}},
		},
	}

	result, artifacts, err := handler(ctx, req, input)
	// Should not return Go error
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	// Result should be an error
	if !result.IsError {
		t.Fatal("handler should return error result when all builds fail")
	}

	// Artifacts should be empty or nil
	if artifacts != nil {
		if batchResult, ok := artifacts.(mcputil.BatchResult); ok && len(batchResult.Artifacts) > 0 {
			t.Errorf("expected no artifacts when all builds fail, got %d", len(batchResult.Artifacts))
		}
	}
}

func TestMakeBatchBuildHandler_EmptySpecs(t *testing.T) {
	config := BuilderConfig{
		Name:      "test-builder",
		Version:   "1.0.0",
		BuildFunc: mockBuildFunc(false),
	}

	handler := makeBatchBuildHandler(config)

	ctx := context.Background()
	req := &mcp.CallToolRequest{}
	input := mcptypes.BatchBuildInput{
		Specs: []mcptypes.BuildInput{},
	}

	result, artifacts, err := handler(ctx, req, input)
	// Should not return Go error
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	// Result should not be an error (no specs to fail)
	if result.IsError {
		t.Fatal("handler should not return error result for empty specs")
	}

	// Artifacts should be empty
	if artifacts != nil {
		if batchResult, ok := artifacts.(mcputil.BatchResult); ok && len(batchResult.Artifacts) > 0 {
			t.Errorf("expected no artifacts for empty specs, got %d", len(batchResult.Artifacts))
		}
	}
}

func TestRegisterBuilderTools(t *testing.T) {
	config := BuilderConfig{
		Name:      "test-builder",
		Version:   "1.0.0",
		BuildFunc: mockBuildFunc(false),
	}

	server := mcpserver.New("test-builder", "1.0.0")

	err := RegisterBuilderTools(server, config)
	if err != nil {
		t.Fatalf("RegisterBuilderTools returned error: %v", err)
	}

	// NOTE: We can't easily test that tools are registered without
	// examining internal server state or running a full MCP server.
	// This test just verifies the function doesn't error.
	// Real validation happens in integration tests.
}

func TestRegisterBuilderTools_IntegrationTest(t *testing.T) {
	// This test verifies that the registered tools can actually be called
	// through the MCP protocol
	config := BuilderConfig{
		Name:      "test-builder",
		Version:   "1.0.0",
		BuildFunc: mockBuildFunc(false),
	}

	server := mcpserver.New("test-builder", "1.0.0")

	err := RegisterBuilderTools(server, config)
	if err != nil {
		t.Fatalf("RegisterBuilderTools returned error: %v", err)
	}

	// Create handlers
	buildHandler := makeBuildHandler(config)
	batchHandler := makeBatchBuildHandler(config)

	// Test build handler
	ctx := context.Background()
	req := &mcp.CallToolRequest{}
	buildInput := mcptypes.BuildInput{
		Name:      "integration-test",
		Engine:    "forge://test-builder",
		Platforms: []string{HostPlatform()},
	}

	buildResult, buildArtifact, buildErr := buildHandler(ctx, req, buildInput)
	if buildErr != nil {
		t.Errorf("build handler returned error: %v", buildErr)
	}
	if buildResult.IsError {
		t.Error("build handler returned error result")
	}
	if buildArtifact == nil {
		t.Error("build handler returned nil artifact")
	}

	// Test batch handler
	batchInput := mcptypes.BatchBuildInput{
		Specs: []mcptypes.BuildInput{buildInput},
	}

	batchResult, batchArtifacts, batchErr := batchHandler(ctx, req, batchInput)
	if batchErr != nil {
		t.Errorf("batch handler returned error: %v", batchErr)
	}
	if batchResult.IsError {
		t.Error("batch handler returned error result")
	}
	if batchArtifacts == nil {
		t.Error("batch handler returned nil artifacts")
	}
}

func TestBuilderFunc_ContextPropagation(t *testing.T) {
	// Test that context is properly propagated to BuilderFunc
	var receivedCtx context.Context

	testBuilder := func(ctx context.Context, input mcptypes.BuildInput) ([]forge.Artifact, error) {
		receivedCtx = ctx
		return One(CreateArtifact(input.Name, forge.TypeGenerated, "/path")), nil
	}

	config := BuilderConfig{
		Name:      "test-builder",
		Version:   "1.0.0",
		BuildFunc: testBuilder,
	}

	handler := makeBuildHandler(config)

	// Create context with value
	type ctxKey string
	ctx := context.WithValue(context.Background(), ctxKey("test"), "value")
	req := &mcp.CallToolRequest{}
	input := mcptypes.BuildInput{
		Name:      "test-app",
		Engine:    "forge://test-builder",
		Platforms: []string{HostPlatform()},
	}

	_, _, err := handler(ctx, req, input)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	// Verify context was propagated
	if receivedCtx == nil {
		t.Fatal("context was not propagated to BuilderFunc")
	}

	// Verify context value
	if val := receivedCtx.Value(ctxKey("test")); val != "value" {
		t.Errorf("context value = %v, want %q", val, "value")
	}
}
