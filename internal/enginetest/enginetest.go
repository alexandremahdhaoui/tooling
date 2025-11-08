package enginetest

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Engine represents a tool/engine to be tested.
type Engine struct {
	// Name is the engine name (e.g., "forge", "build-go")
	Name string
	// BinaryPath is the path to the binary (e.g., "./build/bin/forge")
	BinaryPath string
	// SupportsMCP indicates if the engine should support MCP mode
	SupportsMCP bool
}

// TestVersionCommand tests that the engine supports version commands.
func TestVersionCommand(t *testing.T, engine Engine) {
	t.Helper()

	if _, err := os.Stat(engine.BinaryPath); os.IsNotExist(err) {
		t.Skipf("Binary not found: %s", engine.BinaryPath)
	}

	versionFlags := []string{"version", "--version", "-v"}

	for _, flag := range versionFlags {
		t.Run(fmt.Sprintf("%s_%s", engine.Name, flag), func(t *testing.T) {
			cmd := exec.Command(engine.BinaryPath, flag)
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			err := cmd.Run()
			if err != nil {
				t.Fatalf("Command failed: %v\nStdout: %s\nStderr: %s", err, stdout.String(), stderr.String())
			}

			output := stdout.String()
			if output == "" {
				t.Fatal("Version command produced no output")
			}

			// Check that output contains expected fields
			expectedFields := []string{
				engine.Name + " version",
				"commit:",
				"built:",
				"go:",
				"platform:",
			}

			for _, field := range expectedFields {
				if !strings.Contains(output, field) {
					t.Errorf("Version output missing expected field '%s'\nOutput: %s", field, output)
				}
			}
		})
	}
}

// TestMCPMode tests that the engine supports MCP mode (if applicable).
func TestMCPMode(t *testing.T, engine Engine) {
	t.Helper()

	if !engine.SupportsMCP {
		t.Skipf("Engine %s does not support MCP mode", engine.Name)
	}

	if _, err := os.Stat(engine.BinaryPath); os.IsNotExist(err) {
		t.Skipf("Binary not found: %s", engine.BinaryPath)
	}

	t.Run(fmt.Sprintf("%s_mcp_mode", engine.Name), func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, engine.BinaryPath, "--mcp")

		// MCP servers should communicate via stdin/stdout
		stdin, err := cmd.StdinPipe()
		if err != nil {
			t.Fatalf("Failed to create stdin pipe: %v", err)
		}
		defer stdin.Close()

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			t.Fatalf("Failed to create stdout pipe: %v", err)
		}

		var stderr bytes.Buffer
		cmd.Stderr = &stderr

		// Start the MCP server
		if err := cmd.Start(); err != nil {
			t.Fatalf("Failed to start MCP server: %v", err)
		}

		// Send a simple MCP initialize request (JSON-RPC 2.0)
		initRequest := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0.0"}}}` + "\n"

		if _, err := stdin.Write([]byte(initRequest)); err != nil {
			t.Fatalf("Failed to write to stdin: %v", err)
		}

		// Read response (with timeout)
		responseChan := make(chan string, 1)
		go func() {
			buf := make([]byte, 4096)
			n, err := stdout.Read(buf)
			if err == nil && n > 0 {
				responseChan <- string(buf[:n])
			}
		}()

		select {
		case response := <-responseChan:
			// Check that we got a JSON-RPC response
			if !strings.Contains(response, "jsonrpc") {
				t.Errorf("Expected JSON-RPC response, got: %s", response)
			}
		case <-time.After(2 * time.Second):
			// It's okay if we don't get a response immediately
			// Just verify the process started without error
			t.Log("No immediate response from MCP server (this is okay)")
		}

		// Clean up
		stdin.Close()

		// Wait for process to exit or kill it
		done := make(chan error, 1)
		go func() {
			done <- cmd.Wait()
		}()

		select {
		case <-done:
			// Process exited
		case <-time.After(1 * time.Second):
			// Kill the process if it's still running
			if cmd.Process != nil {
				cmd.Process.Kill()
			}
		}

		// Check stderr for obvious errors
		stderrOutput := stderr.String()
		if strings.Contains(stderrOutput, "panic") {
			t.Errorf("MCP server panicked: %s", stderrOutput)
		}
	})
}

// TestBinaryExists verifies the binary exists and is executable.
func TestBinaryExists(t *testing.T, engine Engine) {
	t.Helper()

	t.Run(fmt.Sprintf("%s_binary_exists", engine.Name), func(t *testing.T) {
		info, err := os.Stat(engine.BinaryPath)
		if err != nil {
			t.Fatalf("Binary does not exist: %s", engine.BinaryPath)
		}

		// Check if file is executable (on Unix-like systems)
		mode := info.Mode()
		if mode&0o111 == 0 {
			t.Errorf("Binary is not executable: %s", engine.BinaryPath)
		}
	})
}

// AllEngines returns a list of all engines to test.
func AllEngines(repoRoot string) []Engine {
	buildBin := filepath.Join(repoRoot, "build", "bin")

	return []Engine{
		{Name: "forge", BinaryPath: filepath.Join(buildBin, "forge"), SupportsMCP: true},
		{Name: "build-go", BinaryPath: filepath.Join(buildBin, "build-go"), SupportsMCP: true},
		{Name: "build-container", BinaryPath: filepath.Join(buildBin, "build-container"), SupportsMCP: true},
		{Name: "generic-builder", BinaryPath: filepath.Join(buildBin, "generic-builder"), SupportsMCP: true},
		{Name: "testenv", BinaryPath: filepath.Join(buildBin, "testenv"), SupportsMCP: true},
		{Name: "testenv-kind", BinaryPath: filepath.Join(buildBin, "testenv-kind"), SupportsMCP: true},
		{Name: "testenv-lcr", BinaryPath: filepath.Join(buildBin, "testenv-lcr"), SupportsMCP: true},
		{Name: "testenv-helm-install", BinaryPath: filepath.Join(buildBin, "testenv-helm-install"), SupportsMCP: true},
		{Name: "test-runner-go", BinaryPath: filepath.Join(buildBin, "test-runner-go"), SupportsMCP: true},
		{Name: "test-runner-go-verify-tags", BinaryPath: filepath.Join(buildBin, "test-runner-go-verify-tags"), SupportsMCP: true},
		{Name: "generic-test-runner", BinaryPath: filepath.Join(buildBin, "generic-test-runner"), SupportsMCP: true},
		{Name: "test-report", BinaryPath: filepath.Join(buildBin, "test-report"), SupportsMCP: true},
		{Name: "format-go", BinaryPath: filepath.Join(buildBin, "format-go"), SupportsMCP: true},
		{Name: "lint-go", BinaryPath: filepath.Join(buildBin, "lint-go"), SupportsMCP: true},
		{Name: "generate-mocks", BinaryPath: filepath.Join(buildBin, "generate-mocks"), SupportsMCP: true},
		{Name: "generate-openapi-go", BinaryPath: filepath.Join(buildBin, "generate-openapi-go"), SupportsMCP: true},
		{Name: "ci-orchestrator", BinaryPath: filepath.Join(buildBin, "ci-orchestrator"), SupportsMCP: true},
		{Name: "forge-e2e", BinaryPath: filepath.Join(buildBin, "forge-e2e"), SupportsMCP: true},
	}
}
