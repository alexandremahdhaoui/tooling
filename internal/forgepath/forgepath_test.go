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

package forgepath

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestIsForgeRepo tests the IsForgeRepo function
func TestIsForgeRepo_Valid(t *testing.T) {
	t.Parallel()

	// Create a temporary directory structure that looks like a forge repo
	tmpDir := t.TempDir()

	// Create go.mod with forge module
	goModContent := `module github.com/alexandremahdhaoui/forge

go 1.24
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0o644); err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	// Create cmd/forge/main.go
	cmdDir := filepath.Join(tmpDir, "cmd", "forge")
	if err := os.MkdirAll(cmdDir, 0o755); err != nil {
		t.Fatalf("Failed to create cmd/forge directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cmdDir, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("Failed to create main.go: %v", err)
	}

	// Test IsForgeRepo
	if !IsForgeRepo(tmpDir) {
		t.Errorf("IsForgeRepo(%s) = false, want true", tmpDir)
	}
}

func TestIsForgeRepo_Invalid_NoGoMod(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create cmd/forge/main.go but no go.mod
	cmdDir := filepath.Join(tmpDir, "cmd", "forge")
	if err := os.MkdirAll(cmdDir, 0o755); err != nil {
		t.Fatalf("Failed to create cmd/forge directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cmdDir, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("Failed to create main.go: %v", err)
	}

	if IsForgeRepo(tmpDir) {
		t.Errorf("IsForgeRepo(%s) = true, want false (no go.mod)", tmpDir)
	}
}

func TestIsForgeRepo_Invalid_WrongModule(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create go.mod with wrong module
	goModContent := `module github.com/other/repo

go 1.24
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0o644); err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	// Create cmd/forge/main.go
	cmdDir := filepath.Join(tmpDir, "cmd", "forge")
	if err := os.MkdirAll(cmdDir, 0o755); err != nil {
		t.Fatalf("Failed to create cmd/forge directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cmdDir, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("Failed to create main.go: %v", err)
	}

	if IsForgeRepo(tmpDir) {
		t.Errorf("IsForgeRepo(%s) = true, want false (wrong module)", tmpDir)
	}
}

func TestIsForgeRepo_Invalid_NoMainGo(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create go.mod with forge module
	goModContent := `module github.com/alexandremahdhaoui/forge

go 1.24
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0o644); err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	// Don't create cmd/forge/main.go

	if IsForgeRepo(tmpDir) {
		t.Errorf("IsForgeRepo(%s) = true, want false (no cmd/forge/main.go)", tmpDir)
	}
}

// LocalCheckout is the one owner of run-local: off, it refuses; on with a
// basedir, it answers that basedir and nothing else is consulted.
func TestLocalCheckoutIsTheOneOwnerOfRunLocal(t *testing.T) {
	t.Setenv("FORGE_RUN_LOCAL_ENABLED", "")
	t.Setenv("FORGE_RUN_LOCAL_BASEDIR", "/nowhere")

	if RunLocal() {
		t.Fatal("run-local must be off until FORGE_RUN_LOCAL_ENABLED=true")
	}

	if _, err := LocalCheckout(); err == nil {
		t.Fatal("asking for the checkout outside run-local mode must refuse")
	}

	t.Setenv("FORGE_RUN_LOCAL_ENABLED", "true")

	dir, err := LocalCheckout()
	if err != nil {
		t.Fatal(err)
	}

	if dir != "/nowhere" {
		t.Fatalf("the basedir names the checkout, got %q", dir)
	}
}

// TestBuildGoRunCommand_Success tests successful command building
func TestBuildGoRunCommand_Success(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		packageName string
	}{
		{
			name:        "testenv-kind",
			packageName: "testenv-kind",
		},
		{
			name:        "go-build",
			packageName: "go-build",
		},
		{
			name:        "testenv",
			packageName: "testenv",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := BuildGoRunCommand(tt.packageName, "v0.9.0")
			if err != nil {
				t.Fatalf("BuildGoRunCommand(%q) error = %v, want nil", tt.packageName, err)
			}

			// Verify we got a command with at least 2 parts: ["run", "<path>"]
			if len(got) < 2 {
				t.Fatalf("BuildGoRunCommand(%q) length = %d, want at least 2", tt.packageName, len(got))
			}

			// Find the "run" argument (could be at index 0 or after -C flag)
			runIndex := -1
			for i, arg := range got {
				if arg == "run" {
					runIndex = i
					break
				}
			}

			if runIndex == -1 {
				t.Errorf("BuildGoRunCommand(%q) = %v, want 'run' argument", tt.packageName, got)
			}

			// The path after "run" should contain the package name
			if runIndex >= 0 && runIndex+1 < len(got) {
				if !strings.Contains(got[runIndex+1], tt.packageName) {
					t.Errorf("BuildGoRunCommand(%q) path = %q, should contain %q", tt.packageName, got[runIndex+1], tt.packageName)
				}
			}
		})
	}
}

// TestBuildGoRunCommand_EmptyPackageName tests error handling for empty package name
func TestBuildGoRunCommand_EmptyPackageName(t *testing.T) {
	t.Parallel()

	_, err := BuildGoRunCommand("", "v0.9.0")
	if err == nil {
		t.Error("BuildGoRunCommand(\"\") error = nil, want error")
	}
}

// TestFindForgeRepo_FromGoList tests finding forge repo via go list
// This test actually runs `go list` so it will find the real forge module
func TestFindForgeRepo_FromGoList(t *testing.T) {
	// Don't use t.Parallel() to avoid interference with other tests

	// This test relies on the actual forge module being available
	// which should be the case since we're running inside the forge repo
	repoPath, err := FindForgeRepo()
	if err != nil {
		t.Skipf("Skipping test: forge module not found in go list: %v", err)
	}

	// Verify the result is a valid forge repo
	if !IsForgeRepo(repoPath) {
		t.Errorf("FindForgeRepo() returned %s which is not a valid forge repo", repoPath)
	}
}

// TestFindForgeRepo_Caching tests that FindForgeRepo caches its result
func TestFindForgeRepo_Caching(t *testing.T) {
	// Don't use t.Parallel() to ensure consistent cache state

	// Call FindForgeRepo twice
	path1, err1 := FindForgeRepo()
	path2, err2 := FindForgeRepo()

	// Both calls should return the same result (due to caching)
	if err1 != err2 {
		t.Errorf("FindForgeRepo() cache inconsistency: first call error = %v, second call error = %v", err1, err2)
	}

	if path1 != path2 {
		t.Errorf("FindForgeRepo() cache inconsistency: first call = %s, second call = %s", path1, path2)
	}
}

// Helper function to set up a fake forge repo for testing
func setupFakeForgeRepo(t *testing.T, dir string) {
	t.Helper()

	// Create go.mod
	goModContent := `module github.com/alexandremahdhaoui/forge

go 1.24
`
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goModContent), 0o644); err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	// Create cmd/forge/main.go
	cmdDir := filepath.Join(dir, "cmd", "forge")
	if err := os.MkdirAll(cmdDir, 0o755); err != nil {
		t.Fatalf("Failed to create cmd/forge directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cmdDir, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("Failed to create main.go: %v", err)
	}
}

func TestIsExternalModule(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		want bool
	}{
		// Internal short names
		{name: "short name testenv-kind", path: "testenv-kind", want: false},
		{name: "short name go-build", path: "go-build", want: false},
		{name: "short name testenv", path: "testenv", want: false},
		{name: "short name with numbers", path: "go-test-v2", want: false},

		// External modules (domain with dot in first segment)
		{name: "github.com module", path: "github.com/user/repo/cmd/tool", want: true},
		{name: "gitlab.com module", path: "gitlab.com/org/project/pkg/util", want: true},
		{name: "bitbucket.org module", path: "bitbucket.org/team/repo", want: true},
		{name: "custom domain", path: "my.company.com/internal/tool", want: true},
		{name: "gopkg.in module", path: "gopkg.in/yaml.v3", want: true},

		// Local paths (not external)
		{name: "relative current dir", path: "./cmd/tool", want: false},
		{name: "relative parent dir", path: "../pkg/util", want: false},
		{name: "relative nested", path: "./internal/pkg/foo", want: false},

		// Edge cases
		{name: "empty path", path: "", want: false},
		{name: "internal sub-path no dot", path: "cmd/tool", want: false},
		{name: "internal sub-path multiple segments", path: "internal/pkg/util", want: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := IsExternalModule(tt.path)
			if got != tt.want {
				t.Errorf("IsExternalModule(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}
