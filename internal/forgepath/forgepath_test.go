//go:build unit

package forgepath

import (
	"os"
	"path/filepath"
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

// TestFindForgeRepo_FromEnvironment tests finding forge repo via environment variable
func TestFindForgeRepo_FromEnvironment(t *testing.T) {
	// NOTE: Cannot use t.Parallel() with t.Setenv()

	// Create a temp forge repo
	tmpDir := t.TempDir()
	setupFakeForgeRepo(t, tmpDir)

	// Set environment variable
	t.Setenv("FORGE_REPO_PATH", tmpDir)

	// Note: We can't easily reset the cache in tests, but this test should still work
	// because the environment variable is checked first before using the cache
	repoPath, err := findForgeRepoUncached()
	if err != nil {
		t.Fatalf("FindForgeRepo() error = %v, want nil", err)
	}

	// Compare absolute paths
	wantPath, _ := filepath.Abs(tmpDir)
	gotPath, _ := filepath.Abs(repoPath)

	if gotPath != wantPath {
		t.Errorf("FindForgeRepo() = %s, want %s", gotPath, wantPath)
	}
}

// TestBuildGoRunCommand_Success tests successful command building
func TestBuildGoRunCommand_Success(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		packageName string
		wantCmd     []string
	}{
		{
			name:        "testenv-kind",
			packageName: "testenv-kind",
			wantCmd:     []string{"run", "github.com/alexandremahdhaoui/forge/cmd/testenv-kind"},
		},
		{
			name:        "build-go",
			packageName: "build-go",
			wantCmd:     []string{"run", "github.com/alexandremahdhaoui/forge/cmd/build-go"},
		},
		{
			name:        "testenv",
			packageName: "testenv",
			wantCmd:     []string{"run", "github.com/alexandremahdhaoui/forge/cmd/testenv"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := BuildGoRunCommand(tt.packageName)
			if err != nil {
				t.Fatalf("BuildGoRunCommand(%q) error = %v, want nil", tt.packageName, err)
			}

			if len(got) != len(tt.wantCmd) {
				t.Fatalf("BuildGoRunCommand(%q) length = %d, want %d", tt.packageName, len(got), len(tt.wantCmd))
			}

			for i := range got {
				if got[i] != tt.wantCmd[i] {
					t.Errorf("BuildGoRunCommand(%q)[%d] = %q, want %q", tt.packageName, i, got[i], tt.wantCmd[i])
				}
			}
		})
	}
}

// TestBuildGoRunCommand_EmptyPackageName tests error handling for empty package name
func TestBuildGoRunCommand_EmptyPackageName(t *testing.T) {
	t.Parallel()

	_, err := BuildGoRunCommand("")
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

// TestFindForgeRepo_ErrorWhenInvalidEnvPath tests error when FORGE_REPO_PATH points to invalid directory
func TestFindForgeRepo_ErrorWhenInvalidEnvPath(t *testing.T) {
	// Don't use t.Parallel() since we're setting environment variable

	// Create a temp directory that is NOT a forge repo
	tmpDir := t.TempDir()

	// Set environment variable to invalid path
	t.Setenv("FORGE_REPO_PATH", tmpDir)

	// Should return error since tmpDir is not a forge repo
	_, err := findForgeRepoUncached()
	if err == nil {
		t.Error("findForgeRepoUncached() with invalid FORGE_REPO_PATH should return error, got nil")
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
