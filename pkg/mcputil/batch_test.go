//go:build unit

package mcputil

import (
	"context"
	"errors"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type testSpec struct {
	Name      string
	ShouldFail bool
}

func TestHandleBatchBuild_AllSuccess(t *testing.T) {
	specs := []testSpec{
		{Name: "spec1", ShouldFail: false},
		{Name: "spec2", ShouldFail: false},
		{Name: "spec3", ShouldFail: false},
	}

	handler := func(ctx context.Context, spec testSpec) (*mcp.CallToolResult, any, error) {
		if spec.ShouldFail {
			return ErrorResult("failed"), nil, nil
		}
		return SuccessResult("success"), spec.Name + "-artifact", nil
	}

	artifacts, errorMsgs := HandleBatchBuild(context.Background(), specs, handler)

	if len(artifacts) != 3 {
		t.Errorf("Expected 3 artifacts, got %d", len(artifacts))
	}
	if len(errorMsgs) != 0 {
		t.Errorf("Expected 0 errors, got %d: %v", len(errorMsgs), errorMsgs)
	}
}

func TestHandleBatchBuild_AllFailures(t *testing.T) {
	specs := []testSpec{
		{Name: "spec1", ShouldFail: true},
		{Name: "spec2", ShouldFail: true},
	}

	handler := func(ctx context.Context, spec testSpec) (*mcp.CallToolResult, any, error) {
		return ErrorResult("build failed"), nil, nil
	}

	artifacts, errorMsgs := HandleBatchBuild(context.Background(), specs, handler)

	if len(artifacts) != 0 {
		t.Errorf("Expected 0 artifacts, got %d", len(artifacts))
	}
	if len(errorMsgs) != 2 {
		t.Errorf("Expected 2 errors, got %d", len(errorMsgs))
	}
}

func TestHandleBatchBuild_MixedResults(t *testing.T) {
	specs := []testSpec{
		{Name: "spec1", ShouldFail: false},
		{Name: "spec2", ShouldFail: true},
		{Name: "spec3", ShouldFail: false},
		{Name: "spec4", ShouldFail: true},
	}

	handler := func(ctx context.Context, spec testSpec) (*mcp.CallToolResult, any, error) {
		if spec.ShouldFail {
			return ErrorResult("failed"), nil, nil
		}
		return SuccessResult("success"), spec.Name + "-artifact", nil
	}

	artifacts, errorMsgs := HandleBatchBuild(context.Background(), specs, handler)

	if len(artifacts) != 2 {
		t.Errorf("Expected 2 artifacts, got %d", len(artifacts))
	}
	if len(errorMsgs) != 2 {
		t.Errorf("Expected 2 errors, got %d", len(errorMsgs))
	}
}

func TestHandleBatchBuild_EmptySpecs(t *testing.T) {
	specs := []testSpec{}

	handler := func(ctx context.Context, spec testSpec) (*mcp.CallToolResult, any, error) {
		return SuccessResult("success"), "artifact", nil
	}

	artifacts, errorMsgs := HandleBatchBuild(context.Background(), specs, handler)

	if len(artifacts) != 0 {
		t.Errorf("Expected 0 artifacts, got %d", len(artifacts))
	}
	if len(errorMsgs) != 0 {
		t.Errorf("Expected 0 errors, got %d", len(errorMsgs))
	}
}

func TestHandleBatchBuild_HandlerReturnsError(t *testing.T) {
	specs := []testSpec{{Name: "spec1"}}

	handler := func(ctx context.Context, spec testSpec) (*mcp.CallToolResult, any, error) {
		return nil, nil, errors.New("handler error")
	}

	artifacts, errorMsgs := HandleBatchBuild(context.Background(), specs, handler)

	if len(artifacts) != 0 {
		t.Errorf("Expected 0 artifacts, got %d", len(artifacts))
	}
	if len(errorMsgs) != 1 {
		t.Errorf("Expected 1 error, got %d", len(errorMsgs))
	}
	if errorMsgs[0] != "handler error" {
		t.Errorf("Expected error message 'handler error', got '%s'", errorMsgs[0])
	}
}

func TestFormatBatchResult_WithErrors(t *testing.T) {
	artifacts := []any{"artifact1"}
	errorMsgs := []string{"error1", "error2"}

	result, returnedArtifacts := FormatBatchResult("binaries", artifacts, errorMsgs)

	if !result.IsError {
		t.Error("Expected IsError to be true")
	}
	if len(returnedArtifacts.([]any)) != 1 {
		t.Errorf("Expected 1 artifact returned, got %d", len(returnedArtifacts.([]any)))
	}
}

func TestFormatBatchResult_Success(t *testing.T) {
	artifacts := []any{"artifact1", "artifact2"}
	errorMsgs := []string{}

	result, returnedArtifacts := FormatBatchResult("containers", artifacts, errorMsgs)

	if result.IsError {
		t.Error("Expected IsError to be false")
	}
	if len(returnedArtifacts.([]any)) != 2 {
		t.Errorf("Expected 2 artifacts returned, got %d", len(returnedArtifacts.([]any)))
	}
}
