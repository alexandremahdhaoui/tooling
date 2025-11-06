//go:build unit

package gitutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGetCurrentCommitSHA_InGitRepo(t *testing.T) {
	// This test assumes we're running in a git repository
	// which should be true for this project
	sha, err := GetCurrentCommitSHA()
	if err != nil {
		t.Fatalf("GetCurrentCommitSHA failed: %v", err)
	}

	// SHA should be 40 characters (full hash)
	if len(sha) != 40 {
		t.Errorf("Expected SHA length 40, got %d: %s", len(sha), sha)
	}

	// SHA should only contain hex characters
	for _, c := range sha {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("SHA contains invalid character: %c (SHA: %s)", c, sha)
		}
	}
}

func TestGetCurrentCommitSHA_NotInGitRepo(t *testing.T) {
	// Create a temporary directory that's not a git repo
	tmpDir := t.TempDir()

	// Change to temp directory
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Should return error when not in a git repo
	_, err = GetCurrentCommitSHA()
	if err == nil {
		t.Error("Expected error when not in a git repository, got nil")
	}
}

func TestGetCurrentCommitSHA_Consistency(t *testing.T) {
	// Getting the SHA twice should return the same value
	// (assuming no commits happen during the test)
	sha1, err := GetCurrentCommitSHA()
	if err != nil {
		t.Fatalf("First GetCurrentCommitSHA failed: %v", err)
	}

	sha2, err := GetCurrentCommitSHA()
	if err != nil {
		t.Fatalf("Second GetCurrentCommitSHA failed: %v", err)
	}

	if sha1 != sha2 {
		t.Errorf("SHA changed between calls: %s != %s", sha1, sha2)
	}
}

func TestGetCurrentCommitSHA_MatchesGitCommand(t *testing.T) {
	// Compare with direct git command output
	ourSHA, err := GetCurrentCommitSHA()
	if err != nil {
		t.Fatalf("GetCurrentCommitSHA failed: %v", err)
	}

	// Run git command directly
	cmd := exec.Command("git", "rev-parse", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("Direct git command failed: %v", err)
	}

	directSHA := string(output)
	// TrimSpace to match our function's behavior
	directSHA = directSHA[:len(directSHA)-1] // Remove newline

	if ourSHA != directSHA {
		t.Errorf("SHA mismatch: ours=%s, direct=%s", ourSHA, directSHA)
	}
}

func TestGetCurrentCommitSHA_InSubdirectory(t *testing.T) {
	// Create a subdirectory and test from there
	// Git should still work from subdirectories
	tmpSubDir := filepath.Join(t.TempDir(), "subdir")
	if err := os.MkdirAll(tmpSubDir, 0o755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	// Get SHA from current directory
	sha1, err := GetCurrentCommitSHA()
	if err != nil {
		t.Fatalf("GetCurrentCommitSHA from root failed: %v", err)
	}

	// Change to subdirectory within the git repo
	// We need to create a subdirectory in the actual git repo, not temp dir
	actualSubDir := filepath.Join(".", "pkg", "forge") // Use existing directory
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(actualSubDir); err != nil {
		t.Skipf("Could not change to subdirectory %s: %v", actualSubDir, err)
	}

	// Get SHA from subdirectory
	sha2, err := GetCurrentCommitSHA()
	if err != nil {
		t.Fatalf("GetCurrentCommitSHA from subdirectory failed: %v", err)
	}

	// Should be the same SHA
	if sha1 != sha2 {
		t.Errorf("SHA differs when called from subdirectory: root=%s, subdir=%s", sha1, sha2)
	}
}
