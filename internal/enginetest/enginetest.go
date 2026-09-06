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

package enginetest

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
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
	// Name is the engine name (e.g., "forge", "go-build")
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
		defer func() { _ = stdin.Close() }()

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
		_ = stdin.Close()

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
				_ = cmd.Process.Kill()
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
		{Name: "go-build", BinaryPath: filepath.Join(buildBin, "go-build"), SupportsMCP: true},
		{Name: "container-build", BinaryPath: filepath.Join(buildBin, "container-build"), SupportsMCP: true},
		{Name: "generic-builder", BinaryPath: filepath.Join(buildBin, "generic-builder"), SupportsMCP: true},
		{Name: "testenv", BinaryPath: filepath.Join(buildBin, "testenv"), SupportsMCP: true},
		{Name: "testenv-kind", BinaryPath: filepath.Join(buildBin, "testenv-kind"), SupportsMCP: true},
		{Name: "testenv-lcr", BinaryPath: filepath.Join(buildBin, "testenv-lcr"), SupportsMCP: true},
		{Name: "testenv-helm-install", BinaryPath: filepath.Join(buildBin, "testenv-helm-install"), SupportsMCP: true},
		{Name: "go-test", BinaryPath: filepath.Join(buildBin, "go-test"), SupportsMCP: true},
		{Name: "go-lint-licenses", BinaryPath: filepath.Join(buildBin, "go-lint-licenses"), SupportsMCP: true},
		{Name: "go-lint-tags", BinaryPath: filepath.Join(buildBin, "go-lint-tags"), SupportsMCP: true},
		{Name: "generic-test-runner", BinaryPath: filepath.Join(buildBin, "generic-test-runner"), SupportsMCP: true},
		{Name: "test-report", BinaryPath: filepath.Join(buildBin, "test-report"), SupportsMCP: true},
		{Name: "go-format", BinaryPath: filepath.Join(buildBin, "go-format"), SupportsMCP: true},
		{Name: "go-lint", BinaryPath: filepath.Join(buildBin, "go-lint"), SupportsMCP: true},
		{Name: "go-gen-mocks", BinaryPath: filepath.Join(buildBin, "go-gen-mocks"), SupportsMCP: true},
		{Name: "go-gen-openapi", BinaryPath: filepath.Join(buildBin, "go-gen-openapi"), SupportsMCP: true},
		{Name: "forge-e2e", BinaryPath: filepath.Join(buildBin, "forge-e2e"), SupportsMCP: true},
		{Name: "forge-dev", BinaryPath: filepath.Join(buildBin, "forge-dev"), SupportsMCP: true},
		{Name: "container-build-simple", BinaryPath: filepath.Join(buildBin, "container-build-simple"), SupportsMCP: true},
		{Name: "parallel-builder", BinaryPath: filepath.Join(buildBin, "parallel-builder"), SupportsMCP: true},
		{Name: "go-gen-bpf", BinaryPath: filepath.Join(buildBin, "go-gen-bpf"), SupportsMCP: true},
		{Name: "go-gen-protobuf", BinaryPath: filepath.Join(buildBin, "go-gen-protobuf"), SupportsMCP: true},
		{Name: "go-license-header", BinaryPath: filepath.Join(buildBin, "go-license-header"), SupportsMCP: true},
		{Name: "rust-license-header", BinaryPath: filepath.Join(buildBin, "rust-license-header"), SupportsMCP: true},
	}
}

// TestBuildEngineTools verifies that a build engine implements both "build" and "buildBatch" tools.
// This prevents issues where engines might be missing batch support, which would cause errors when
// forge tries to build multiple artifacts with the same engine.
func TestBuildEngineTools(t *testing.T, engine Engine) {
	t.Helper()

	if !engine.SupportsMCP {
		t.Skipf("Engine %s does not support MCP mode", engine.Name)
	}

	if _, err := os.Stat(engine.BinaryPath); os.IsNotExist(err) {
		t.Skipf("Binary not found: %s", engine.BinaryPath)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, engine.BinaryPath, "--mcp")

	// MCP servers communicate via stdin/stdout
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("Failed to create stdin pipe: %v", err)
	}
	defer func() { _ = stdin.Close() }()

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
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}()

	// Send initialize request
	initRequest := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0.0"}}}` + "\n"
	if _, err := stdin.Write([]byte(initRequest)); err != nil {
		t.Fatalf("Failed to send initialize request: %v", err)
	}

	// Read initialize response
	scanner := bufio.NewScanner(stdout)
	if !scanner.Scan() {
		t.Fatalf("Failed to read initialize response: %v", scanner.Err())
	}

	// Send initialized notification (required by MCP protocol before other requests)
	initializedNotification := `{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n"
	if _, err := stdin.Write([]byte(initializedNotification)); err != nil {
		t.Fatalf("Failed to send initialized notification: %v", err)
	}

	// Send tools/list request
	toolsListRequest := `{"jsonrpc":"2.0","id":2,"method":"tools/list"}` + "\n"
	if _, err := stdin.Write([]byte(toolsListRequest)); err != nil {
		t.Fatalf("Failed to send tools/list request: %v", err)
	}

	// Read tools/list response (skip any notifications)
	var response string
	for scanner.Scan() {
		line := scanner.Text()
		// Skip notifications (no "id" field with our expected id)
		if strings.Contains(line, `"id":2`) {
			response = line
			break
		}
	}
	if response == "" {
		t.Fatalf("Failed to read tools/list response: %v", scanner.Err())
	}

	// Parse JSON-RPC response
	var jsonRPCResponse struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Result  struct {
			Tools []struct {
				Name        string `json:"name"`
				Description string `json:"description"`
			} `json:"tools"`
		} `json:"result"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal([]byte(response), &jsonRPCResponse); err != nil {
		t.Fatalf("Failed to parse tools/list response: %v\nResponse: %s", err, response)
	}

	if jsonRPCResponse.Error != nil {
		t.Fatalf("tools/list returned error: %s", jsonRPCResponse.Error.Message)
	}

	// Check for required tools
	tools := make(map[string]bool)
	for _, tool := range jsonRPCResponse.Result.Tools {
		tools[tool.Name] = true
	}

	// Verify both "build" and "buildBatch" tools are present
	requiredTools := []string{"build", "buildBatch"}
	var missingTools []string

	for _, toolName := range requiredTools {
		if !tools[toolName] {
			missingTools = append(missingTools, toolName)
		}
	}

	if len(missingTools) > 0 {
		availableTools := make([]string, 0, len(tools))
		for name := range tools {
			availableTools = append(availableTools, name)
		}
		t.Errorf("Engine %s is missing required tools: %v\nAvailable tools: %v",
			engine.Name, missingTools, availableTools)
	}

	// Verify tool names are correct
	if !tools["build"] {
		t.Errorf("Engine %s does not implement 'build' tool (required for single artifact builds)", engine.Name)
	}
	if !tools["buildBatch"] {
		t.Errorf("Engine %s does not implement 'buildBatch' tool (required for batch builds)", engine.Name)
	}
}

// TestPlatformRefusal is the platform contract, held against a real engine
// over real MCP. A malformed platform is refused naming it. A platform no
// toolchain targets is refused by name when the engine declares the host,
// and admitted when it declares any - the engine's own tooling then decides,
// so the only thing asserted is that the declaration was honoured. Every
// builder passes this or it is not holding the contract.
func TestPlatformRefusal(t *testing.T, engine Engine, declaresAny bool) {
	t.Helper()

	if !engine.SupportsMCP {
		t.Skipf("Engine %s does not support MCP mode", engine.Name)
	}

	if _, err := os.Stat(engine.BinaryPath); os.IsNotExist(err) {
		t.Skipf("Binary not found: %s", engine.BinaryPath)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, engine.BinaryPath, "--mcp")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("Failed to create stdin pipe: %v", err)
	}
	defer func() { _ = stdin.Close() }()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("Failed to create stdout pipe: %v", err)
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start MCP server: %v", err)
	}
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}()

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)

	send := func(line string) {
		if _, err := stdin.Write([]byte(line + "\n")); err != nil {
			t.Fatalf("Failed to write to the engine: %v", err)
		}
	}

	send(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0.0"}}}`)

	if !scanner.Scan() {
		t.Fatalf("Failed to read initialize response: %v", scanner.Err())
	}

	send(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)

	id := 2

	probe := func(platform string) (isError bool, text string) {
		id++

		send(fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"tools/call","params":{"name":"build","arguments":{"name":"probe","engine":"forge://%s","src":".","platforms":[%q],"spec":{}}}}`, id, engine.Name, platform))

		marker := fmt.Sprintf(`"id":%d`, id)

		var response string

		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, marker) {
				response = line

				break
			}
		}

		if response == "" {
			t.Fatalf("Failed to read the build response: %v\nstderr: %s", scanner.Err(), stderr.String())
		}

		var rpc struct {
			Result struct {
				IsError bool `json:"isError"`
				Content []struct {
					Text string `json:"text"`
				} `json:"content"`
			} `json:"result"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}

		if err := json.Unmarshal([]byte(response), &rpc); err != nil {
			t.Fatalf("Failed to parse the build response: %v\n%s", err, response)
		}

		if rpc.Error != nil {
			t.Fatalf("the build must answer a result, not a protocol error: %s", rpc.Error.Message)
		}

		for _, c := range rpc.Result.Content {
			text += c.Text
		}

		return rpc.Result.IsError, text
	}

	// A platform with no arch is malformed for every engine.
	isError, text := probe("plan9")
	if !isError || !strings.Contains(text, "plan9") {
		t.Fatalf("Engine %s must refuse a malformed platform by name, got isError=%v: %s", engine.Name, isError, text)
	}

	// A platform no toolchain on earth targets.
	isError, text = probe("plan9/mips")

	if declaresAny {
		// Admitted by declaration: the engine's own tooling decides, and
		// whatever it answers, it is not the framework's refusal.
		if strings.Contains(text, "it declares platforms") {
			t.Fatalf("Engine %s declares any and must not refuse by declaration: %s", engine.Name, text)
		}

		return
	}

	if !isError || !strings.Contains(text, "plan9/mips") || !strings.Contains(text, "cannot build") {
		t.Fatalf("Engine %s must refuse a platform it did not declare, naming it, got isError=%v: %s", engine.Name, isError, text)
	}
}
