//go:build integration

package main

import (
	"encoding/json"
	"testing"

	"github.com/alexandremahdhaoui/forge/pkg/forge"
	"github.com/alexandremahdhaoui/forge/pkg/mcputil"
)

// TestStructuredOutputSchemas verifies that all data structures can be marshaled to JSON
// This ensures the structured output will work correctly when returned via MCP.
func TestStructuredOutputSchemas(t *testing.T) {
	tests := []struct {
		name string
		data interface{}
	}{
		{
			name: "Artifact",
			data: forge.Artifact{
				Name:      "test-app",
				Type:      "binary",
				Location:  "./build/bin/test-app",
				Timestamp: "2025-01-15T10:30:00Z",
				Version:   "abc123def",
			},
		},
		{
			name: "Artifact Array",
			data: []forge.Artifact{
				{
					Name:      "app1",
					Type:      "binary",
					Location:  "./build/bin/app1",
					Timestamp: "2025-01-15T10:30:00Z",
					Version:   "abc123",
				},
				{
					Name:      "app2",
					Type:      "binary",
					Location:  "./build/bin/app2",
					Timestamp: "2025-01-15T10:30:00Z",
					Version:   "def456",
				},
			},
		},
		{
			name: "TestEnvironment",
			data: forge.TestEnvironment{
				ID:       "test-uuid-123",
				Name:     "integration",
				Status:   "created",
				TmpDir:   "/tmp/forge-test-integration-test-uuid-123",
				Files:    map[string]string{"testenv-kind.kubeconfig": "kubeconfig"},
				Metadata: map[string]string{"testenv-kind.clusterName": "forge-integration"},
			},
		},
		{
			name: "TestEnvironment Array",
			data: []*forge.TestEnvironment{
				{
					ID:     "test-uuid-123",
					Name:   "integration",
					Status: "created",
				},
				{
					ID:     "test-uuid-456",
					Name:   "integration",
					Status: "passed",
				},
			},
		},
		{
			name: "TestReport",
			data: forge.TestReport{
				ID:     "report-uuid-789",
				Stage:  "unit",
				Status: "passed",
				TestStats: forge.TestStats{
					Total:   42,
					Passed:  42,
					Failed:  0,
					Skipped: 0,
				},
				Coverage: forge.Coverage{
					Percentage: 85.5,
					FilePath:   ".forge/tmp/coverage.out",
				},
			},
		},
		{
			name: "TestReport Array",
			data: []forge.TestReport{
				{
					ID:     "report-1",
					Stage:  "unit",
					Status: "passed",
				},
				{
					ID:     "report-2",
					Stage:  "integration",
					Status: "failed",
				},
			},
		},
		{
			name: "TestAllResult",
			data: TestAllResult{
				BuildArtifacts: []forge.Artifact{
					{
						Name:     "app1",
						Type:     "binary",
						Location: "./build/bin/app1",
					},
				},
				TestReports: []forge.TestReport{
					{
						ID:     "report-1",
						Stage:  "unit",
						Status: "passed",
					},
				},
				Summary: "1 artifact(s) built, 1 test stage(s) run, 1 passed, 0 failed",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal to JSON
			jsonBytes, err := json.Marshal(tt.data)
			if err != nil {
				t.Fatalf("Failed to marshal %s to JSON: %v", tt.name, err)
			}

			// Verify we got valid JSON
			if len(jsonBytes) == 0 {
				t.Fatalf("%s marshaled to empty JSON", tt.name)
			}

			// Verify it's valid JSON by unmarshaling to map
			var result map[string]interface{}
			if err := json.Unmarshal(jsonBytes, &result); err != nil {
				// For arrays, try unmarshaling to array
				var arrayResult []interface{}
				if err := json.Unmarshal(jsonBytes, &arrayResult); err != nil {
					t.Fatalf("Failed to unmarshal %s JSON: %v\nJSON: %s", tt.name, err, string(jsonBytes))
				}
			}

			t.Logf("%s JSON: %s", tt.name, string(jsonBytes))
		})
	}
}

// TestMCPUtilHelpers verifies that the mcputil helper functions work correctly
func TestMCPUtilHelpers(t *testing.T) {
	t.Run("SuccessResultWithArtifact", func(t *testing.T) {
		artifact := forge.Artifact{
			Name:     "test-app",
			Type:     "binary",
			Location: "./build/bin/test-app",
		}

		result, returnedArtifact := mcputil.SuccessResultWithArtifact("Test message", artifact)

		if result == nil {
			t.Fatal("Expected non-nil result")
		}

		if result.IsError {
			t.Error("Expected IsError to be false for success result")
		}

		if len(result.Content) == 0 {
			t.Fatal("Expected content in result")
		}

		if returnedArtifact == nil {
			t.Fatal("Expected non-nil artifact")
		}
	})

	t.Run("ErrorResultWithArtifact", func(t *testing.T) {
		report := forge.TestReport{
			ID:     "report-1",
			Stage:  "unit",
			Status: "failed",
		}

		result, returnedArtifact := mcputil.ErrorResultWithArtifact("Test failed", report)

		if result == nil {
			t.Fatal("Expected non-nil result")
		}

		if !result.IsError {
			t.Error("Expected IsError to be true for error result")
		}

		if len(result.Content) == 0 {
			t.Fatal("Expected content in result")
		}

		if returnedArtifact == nil {
			t.Fatal("Expected non-nil artifact even on error")
		}
	})
}
