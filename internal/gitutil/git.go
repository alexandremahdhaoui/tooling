package gitutil

import (
	"fmt"
	"os/exec"
	"strings"
)

// GetCurrentCommitSHA returns the current Git commit SHA (full 40-character hash).
//
// Returns an error if:
//   - Git command fails to execute
//   - Not in a Git repository
//   - The returned SHA is empty
//
// Example usage:
//
//	sha, err := gitutil.GetCurrentCommitSHA()
//	if err != nil {
//	    return fmt.Errorf("failed to get git version: %w", err)
//	}
func GetCurrentCommitSHA() (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	sha := strings.TrimSpace(string(output))
	if sha == "" {
		return "", fmt.Errorf("empty git commit SHA")
	}

	return sha, nil
}
