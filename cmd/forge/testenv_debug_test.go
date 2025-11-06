//go:build integration

package main

import (
	"os"
	"strings"
	"testing"
)

// TestDebugEnvironmentVariables prints all FORGE_* environment variables
// to help debug what's being passed to tests.
func TestDebugEnvironmentVariables(t *testing.T) {
	t.Log("=== All FORGE_* Environment Variables ===")
	found := false
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "FORGE_") {
			t.Log(env)
			found = true
		}
	}
	if !found {
		t.Log("No FORGE_* environment variables found")
	}
	t.Log("=== End Environment Variables ===")
}
