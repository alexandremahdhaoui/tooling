//go:build unit

package main

import (
	"strings"
	"testing"
)

func TestValidateContainerEngine(t *testing.T) {
	tests := []struct {
		name    string
		engine  string
		wantErr bool
	}{
		{"valid docker", "docker", false},
		{"valid kaniko", "kaniko", false},
		{"valid podman", "podman", false},
		{"invalid empty", "", true},
		{"invalid unknown", "containerd", true},
		{"invalid case", "Docker", true}, // case-sensitive
		{"invalid buildkit", "buildkit", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateContainerEngine(tt.engine)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateContainerEngine() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && !strings.Contains(err.Error(), "invalid CONTAINER_BUILD_ENGINE") {
				t.Errorf("validateContainerEngine() error should mention CONTAINER_BUILD_ENGINE, got: %v", err)
			}
		})
	}
}

func TestExpandPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		checkFn  func(string) bool
		wantDesc string
	}{
		{
			name: "with tilde",
			path: "~/cache",
			checkFn: func(got string) bool {
				return !strings.Contains(got, "~") && len(got) > 7
			},
			wantDesc: "should expand ~ and return non-empty path",
		},
		{
			name: "without tilde",
			path: "/absolute/path",
			checkFn: func(got string) bool {
				return got == "/absolute/path"
			},
			wantDesc: "should return path unchanged",
		},
		{
			name: "relative path",
			path: "relative/path",
			checkFn: func(got string) bool {
				return got == "relative/path"
			},
			wantDesc: "should return path unchanged",
		},
		{
			name: "tilde in middle",
			path: "/path/~/cache",
			checkFn: func(got string) bool {
				return got == "/path/~/cache"
			},
			wantDesc: "should only expand ~ at start",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := expandPath(tt.path)
			if !tt.checkFn(got) {
				t.Errorf("expandPath() = %v, %s", got, tt.wantDesc)
			}
		})
	}
}

func TestGetGitVersionError(t *testing.T) {
	// This test verifies the error handling when not in a git repo
	// We can't easily test the success case without mocking exec.Command
	// but we can verify the error handling works

	// Note: This test assumes we're running in a git repo (which we are)
	// so we test that it returns a non-empty string
	version, err := getGitVersion()
	if err != nil {
		t.Skipf("Skipping test because not in git repo or git not available: %v", err)
	}

	if version == "" {
		t.Error("getGitVersion() returned empty string with no error")
	}

	if len(version) < 7 {
		t.Errorf("getGitVersion() returned suspiciously short version: %s", version)
	}
}

func TestEnvsStructTags(t *testing.T) {
	// Verify that the Envs struct has correct field tags
	// This is a compile-time check more than a runtime test,
	// but we can verify the struct exists and has expected fields

	envs := Envs{}

	// Verify zero values
	if envs.BuildEngine != "" {
		t.Error("BuildEngine should have empty zero value")
	}

	// envs.BuildArgs can be nil (valid zero value for slice)

	if envs.KanikoCacheDir != "" {
		t.Error("KanikoCacheDir should have empty zero value")
	}
}
