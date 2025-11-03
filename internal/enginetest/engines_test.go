package enginetest_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alexandremahdhaoui/forge/internal/enginetest"
)

// getRepoRoot returns the repository root directory.
func getRepoRoot(t *testing.T) string {
	t.Helper()

	// Try to find the repo root by looking for go.mod
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("Could not find repository root (no go.mod found)")
		}
		dir = parent
	}
}

func TestAllEnginesHaveVersionSupport(t *testing.T) {
	repoRoot := getRepoRoot(t)
	engines := enginetest.AllEngines(repoRoot)

	for _, engine := range engines {
		t.Run(engine.Name, func(t *testing.T) {
			enginetest.TestBinaryExists(t, engine)
			enginetest.TestVersionCommand(t, engine)
		})
	}
}

func TestAllMCPEnginesHaveMCPSupport(t *testing.T) {
	repoRoot := getRepoRoot(t)
	engines := enginetest.AllEngines(repoRoot)

	for _, engine := range engines {
		if !engine.SupportsMCP {
			continue
		}

		t.Run(engine.Name, func(t *testing.T) {
			enginetest.TestMCPMode(t, engine)
		})
	}
}

func TestEnginesList(t *testing.T) {
	repoRoot := getRepoRoot(t)
	engines := enginetest.AllEngines(repoRoot)

	if len(engines) != 9 {
		t.Errorf("Expected 9 engines, got %d", len(engines))
	}

	expectedEngines := map[string]bool{
		"forge":                    true,
		"build-go":                 true,
		"build-container":          true,
		"kindenv":                  true,
		"local-container-registry": true,
		"test-go":                  true,
		"oapi-codegen-helper":      true,
		"test-runner-go":           true,
		"test-integration":         true,
	}

	for _, engine := range engines {
		if !expectedEngines[engine.Name] {
			t.Errorf("Unexpected engine in list: %s", engine.Name)
		}
		delete(expectedEngines, engine.Name)
	}

	if len(expectedEngines) > 0 {
		for name := range expectedEngines {
			t.Errorf("Missing engine from list: %s", name)
		}
	}
}

func TestMCPEnginesConfiguration(t *testing.T) {
	repoRoot := getRepoRoot(t)
	engines := enginetest.AllEngines(repoRoot)

	// Verify which engines should support MCP
	expectedMCPEngines := map[string]bool{
		"forge":            true,
		"build-go":         true,
		"build-container":  true,
		"test-runner-go":   true,
		"test-integration": true,
	}

	for _, engine := range engines {
		expected := expectedMCPEngines[engine.Name]
		if engine.SupportsMCP != expected {
			t.Errorf("Engine %s: expected SupportsMCP=%v, got %v",
				engine.Name, expected, engine.SupportsMCP)
		}
	}
}
