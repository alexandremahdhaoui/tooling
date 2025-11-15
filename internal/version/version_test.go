//go:build unit

package version_test

import (
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge/internal/version"
)

func TestNew(t *testing.T) {
	info := version.New("test-tool")
	if info.ToolName != "test-tool" {
		t.Errorf("Expected ToolName 'test-tool', got '%s'", info.ToolName)
	}
	if info.Version != "dev" {
		t.Errorf("Expected Version 'dev', got '%s'", info.Version)
	}
	if info.CommitSHA != "unknown" {
		t.Errorf("Expected CommitSHA 'unknown', got '%s'", info.CommitSHA)
	}
	if info.BuildTimestamp != "unknown" {
		t.Errorf("Expected BuildTimestamp 'unknown', got '%s'", info.BuildTimestamp)
	}
}

func TestGet(t *testing.T) {
	info := version.New("test-tool")
	info.Version = "v1.0.0"
	info.CommitSHA = "abc1234"
	info.BuildTimestamp = "2025-01-01T00:00:00Z"

	v, c, ts := info.Get()
	if v != "v1.0.0" {
		t.Errorf("Expected version 'v1.0.0', got '%s'", v)
	}
	if c != "abc1234" {
		t.Errorf("Expected commit 'abc1234', got '%s'", c)
	}
	if ts != "2025-01-01T00:00:00Z" {
		t.Errorf("Expected timestamp '2025-01-01T00:00:00Z', got '%s'", ts)
	}
}

func TestString(t *testing.T) {
	info := version.New("test-tool")
	info.Version = "v1.2.3"

	str := info.String()
	expected := "test-tool version v1.2.3"
	if str != expected {
		t.Errorf("Expected '%s', got '%s'", expected, str)
	}
}

func TestGetWithBuildInfo(t *testing.T) {
	// This test verifies that Get() works with default values
	// and attempts to read from build info
	info := version.New("test-tool")

	v, c, ts := info.Get()

	// Should have some value (either "dev" or from build info)
	if v == "" {
		t.Error("Expected non-empty version")
	}
	if c == "" {
		t.Error("Expected non-empty commit")
	}
	if ts == "" {
		t.Error("Expected non-empty timestamp")
	}
}

func TestPrint(t *testing.T) {
	// This is a basic test that Print() doesn't panic
	// Actual output verification would require capturing stdout
	info := version.New("test-tool")
	info.Version = "v1.0.0"
	info.CommitSHA = "abc1234"
	info.BuildTimestamp = "2025-01-01T00:00:00Z"

	// Should not panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Print() panicked: %v", r)
		}
	}()

	info.Print()
}

func TestStringContainsToolName(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		version  string
	}{
		{"forge", "forge", "v1.0.0"},
		{"go-build", "go-build", "v2.0.0"},
		{"test-tool", "test-tool", "dev"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := version.New(tt.toolName)
			info.Version = tt.version

			str := info.String()
			if !strings.Contains(str, tt.toolName) {
				t.Errorf("String() should contain tool name '%s', got '%s'", tt.toolName, str)
			}
			if !strings.Contains(str, tt.version) {
				t.Errorf("String() should contain version '%s', got '%s'", tt.version, str)
			}
		})
	}
}
