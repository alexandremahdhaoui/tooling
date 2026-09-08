//go:build integration

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

package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge/pkg/forge"
	"github.com/alexandremahdhaoui/forge/pkg/mcptypes"
)

// checkClangBPFSupport verifies that clang is available and has BPF target support.
// Returns nil if clang with BPF support is available, otherwise returns an error.
func checkClangBPFSupport() error {
	// Check clang is available
	if _, err := exec.LookPath("clang"); err != nil {
		return err
	}

	// Verify clang can compile for BPF target
	cmd := exec.Command("clang", "-target", "bpf", "-c", "-x", "c", "-o", "/dev/null", "-")
	cmd.Stdin = strings.NewReader("int main() { return 0; }")
	return cmd.Run()
}

// TestBuildIntegration tests the full BPF build flow with real bpf2go execution.
func TestBuildIntegration(t *testing.T) {
	// Check for required tools - FAIL if not available (per project requirements)
	if err := checkClangBPFSupport(); err != nil {
		t.Fatalf("clang with BPF target support is required: %v", err)
	}

	// Create temp directory structure
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "minimal.c")
	destDir := filepath.Join(tmpDir, "generated")

	// Create minimal valid BPF C source file
	// This is the simplest possible BPF program that bpf2go can compile
	bpfContent := `// SPDX-License-Identifier: GPL-2.0
// Minimal BPF program for testing

// Minimal placeholder to satisfy bpf2go
char _license[] __attribute__((section("license"))) = "GPL";

// Empty program section
__attribute__((section("socket")))
int minimal() {
    return 0;
}
`
	if err := os.WriteFile(srcFile, []byte(bpfContent), 0o644); err != nil {
		t.Fatalf("Failed to write BPF source file: %v", err)
	}

	// Call Build() with valid mcptypes.BuildInput
	input := mcptypes.BuildInput{
		Name: "test-bpf",
		Src:  srcFile,
		Dest: destDir,
		Spec: map[string]any{
			"ident": "minimal",
		},
	}

	spec, err := FromMap(input.Spec)
	if err != nil {
		t.Fatalf("FromMap() failed: %v", err)
	}

	artifacts, err := Build(context.Background(), input, spec)
	if err != nil {
		t.Fatalf("Build() failed: %v", err)
	}
	if len(artifacts) != 1 {
		t.Fatalf("Build() answered %d artifacts, want 1", len(artifacts))
	}
	artifact := artifacts[0]

	// Verify artifact has correct name
	if artifact.Name != "test-bpf" {
		t.Errorf("artifact.Name = %q, want %q", artifact.Name, "test-bpf")
	}

	// Verify artifact type is "bpf"
	if artifact.Type != "bpf" {
		t.Errorf("artifact.Type = %q, want %q", artifact.Type, "bpf")
	}

	// Verify artifact has DependencyDetectorEngine set
	if artifact.DependencyDetectorEngine != "forge://go-gen-bpf" {
		t.Errorf("artifact.DependencyDetectorEngine = %q, want %q", artifact.DependencyDetectorEngine, "forge://go-gen-bpf")
	}

	// Verify dependencies populated with .c file
	if len(artifact.Dependencies) == 0 {
		t.Error("artifact.Dependencies is empty, expected at least one dependency")
	} else {
		// Check that the source file is in dependencies
		foundSourceFile := false
		for _, dep := range artifact.Dependencies {
			if strings.HasSuffix(dep.Path, "minimal.c") {
				foundSourceFile = true
			}
			if !strings.HasPrefix(dep.Digest, forge.DigestPrefix) {
				t.Errorf("dependency.Digest = %q, want a content digest", dep.Digest)
			}
		}
		if !foundSourceFile {
			t.Error("minimal.c not found in artifact.Dependencies")
		}
	}

	// Verify generated files exist (bpf2go generates _bpfel.go and _bpfeb.go)
	generatedGoFile := filepath.Join(destDir, "zz_generated_bpfel.go")
	if _, err := os.Stat(generatedGoFile); os.IsNotExist(err) {
		t.Errorf("Generated Go file not found at %s", generatedGoFile)
	}

	// Verify artifact location matches dest
	if artifact.Location != destDir {
		t.Errorf("artifact.Location = %q, want %q", artifact.Location, destDir)
	}

	// Verify artifact timestamp is set
	if artifact.Timestamp == "" {
		t.Error("artifact.Timestamp is empty")
	}
}

// TestBuildIntegrationMissingSrc tests that build fails for missing source file.
func TestBuildIntegrationMissingSrc(t *testing.T) {
	tmpDir := t.TempDir()
	destDir := filepath.Join(tmpDir, "generated")

	input := mcptypes.BuildInput{
		Name: "test-missing-src",
		Src:  "", // Missing source
		Dest: destDir,
		Spec: map[string]any{
			"ident": "test",
		},
	}

	spec, _ := FromMap(input.Spec)
	_, err := Build(context.Background(), input, spec)
	if err == nil {
		t.Error("Build() should fail when src is empty")
	}
	if !strings.Contains(err.Error(), "src is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestBuildIntegrationMissingDest tests that build fails for missing destination.
func TestBuildIntegrationMissingDest(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "test.c")
	if err := os.WriteFile(srcFile, []byte("// test"), 0o644); err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}

	input := mcptypes.BuildInput{
		Name: "test-missing-dest",
		Src:  srcFile,
		Dest: "", // Missing destination
		Spec: map[string]any{
			"ident": "test",
		},
	}

	spec, _ := FromMap(input.Spec)
	_, err := Build(context.Background(), input, spec)
	if err == nil {
		t.Error("Build() should fail when dest is empty")
	}
	if !strings.Contains(err.Error(), "dest is required") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestBuildIntegrationMissingIdent tests that validation fails when ident is missing.
func TestBuildIntegrationMissingIdent(t *testing.T) {
	// Test that the validation layer catches missing ident.
	// The generated MCP wrapper validates before calling Build().
	input := mcptypes.BuildInput{
		Name: "test-missing-ident",
		Spec: map[string]any{}, // Missing ident
	}

	spec, _ := FromMap(input.Spec)
	output := Validate(spec)
	if output.Valid {
		t.Error("validation should fail when ident is missing")
	}
	if len(output.Errors) == 0 {
		t.Error("validation should produce at least one error")
	} else if !strings.Contains(output.Errors[0].Field, "ident") {
		t.Errorf("expected error about ident, got: %v", output.Errors[0])
	}
}

// TestBuildIntegrationSourceNotFound tests that build fails for non-existent source.
func TestBuildIntegrationSourceNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	destDir := filepath.Join(tmpDir, "generated")

	input := mcptypes.BuildInput{
		Name: "test-not-found",
		Src:  filepath.Join(tmpDir, "nonexistent.c"),
		Dest: destDir,
		Spec: map[string]any{
			"ident": "test",
		},
	}

	spec, _ := FromMap(input.Spec)
	_, err := Build(context.Background(), input, spec)
	if err == nil {
		t.Error("Build() should fail when source file doesn't exist")
	}
	if !strings.Contains(err.Error(), "source file not found") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestBuildIntegrationSourceIsDirectory tests that build fails when source is a directory.
func TestBuildIntegrationSourceIsDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "srcdir")
	destDir := filepath.Join(tmpDir, "generated")

	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatalf("Failed to create source directory: %v", err)
	}

	input := mcptypes.BuildInput{
		Name: "test-src-is-dir",
		Src:  srcDir, // Directory instead of file
		Dest: destDir,
		Spec: map[string]any{
			"ident": "test",
		},
	}

	spec, _ := FromMap(input.Spec)
	_, err := Build(context.Background(), input, spec)
	if err == nil {
		t.Error("Build() should fail when source is a directory")
	}
	if !strings.Contains(err.Error(), "must be a file, not directory") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestBuildIntegrationWithAllOptions tests build with all spec options configured.
func TestBuildIntegrationWithAllOptions(t *testing.T) {
	// Check for required tools - FAIL if not available (per project requirements)
	if err := checkClangBPFSupport(); err != nil {
		t.Fatalf("clang with BPF target support is required: %v", err)
	}

	// Create temp directory structure
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "full_options.c")
	destDir := filepath.Join(tmpDir, "output")

	// Create minimal valid BPF C source file
	bpfContent := `// SPDX-License-Identifier: GPL-2.0
char _license[] __attribute__((section("license"))) = "GPL";
__attribute__((section("socket")))
int full_options() { return 0; }
`
	if err := os.WriteFile(srcFile, []byte(bpfContent), 0o644); err != nil {
		t.Fatalf("Failed to write BPF source file: %v", err)
	}

	// Call Build() with all options specified
	input := mcptypes.BuildInput{
		Name: "test-full-options",
		Src:  srcFile,
		Dest: destDir,
		Spec: map[string]any{
			"ident":         "fullopts",
			"bpf2goVersion": "latest",
			"goPackage":     "bpftest",
			"outputStem":    "gen_bpf",
			"tags":          []any{"linux"},
		},
	}

	spec, err := FromMap(input.Spec)
	if err != nil {
		t.Fatalf("FromMap() failed: %v", err)
	}

	artifacts, err := Build(context.Background(), input, spec)
	if err != nil {
		t.Fatalf("Build() failed: %v", err)
	}
	if len(artifacts) != 1 {
		t.Fatalf("Build() answered %d artifacts, want 1", len(artifacts))
	}
	artifact := artifacts[0]

	// Verify artifact
	if artifact.Name != "test-full-options" {
		t.Errorf("artifact.Name = %q, want %q", artifact.Name, "test-full-options")
	}

	// Verify generated files with custom output stem exist
	generatedGoFile := filepath.Join(destDir, "gen_bpf_bpfel.go")
	if _, err := os.Stat(generatedGoFile); os.IsNotExist(err) {
		t.Errorf("Generated Go file not found at %s", generatedGoFile)
	}
}
