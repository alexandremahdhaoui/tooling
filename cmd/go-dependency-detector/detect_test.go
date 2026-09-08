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
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge/pkg/forge"

	"github.com/alexandremahdhaoui/forge/pkg/mcptypes"
)

// TestIsStandardLibrary tests the standard library detection logic.
func TestIsStandardLibrary(t *testing.T) {
	tests := []struct {
		name     string
		pkgPath  string
		expected bool
	}{
		{"fmt is stdlib", "fmt", true},
		{"encoding/json is stdlib", "encoding/json", true},
		{"net/http is stdlib", "net/http", true},
		{"github.com/foo/bar is NOT stdlib", "github.com/foo/bar", false},
		{"golang.org/x/mod is NOT stdlib", "golang.org/x/mod", false},
		{"C is NOT stdlib (cgo)", "C", false},
		{"internal is stdlib", "internal", true},
		{"vendor is stdlib", "vendor", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isStandardLibrary(tt.pkgPath)
			if result != tt.expected {
				t.Errorf("isStandardLibrary(%q) = %v, want %v", tt.pkgPath, result, tt.expected)
			}
		})
	}
}

// TestExtractImports tests import extraction from AST.
func TestExtractImports(t *testing.T) {
	// Create a temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")

	testCode := `package main

import (
	"fmt"
	"encoding/json"
	"github.com/alexandremahdhaoui/forge/pkg/mcptypes"
)

func main() {
	fmt.Println("hello")
}
`

	err := os.WriteFile(testFile, []byte(testCode), 0o644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Parse the file
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, testFile, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse test file: %v", err)
	}

	// Extract imports
	imports := extractImports(file)

	expectedImports := []string{"fmt", "encoding/json", "github.com/alexandremahdhaoui/forge/pkg/mcptypes"}
	if len(imports) != len(expectedImports) {
		t.Errorf("Expected %d imports, got %d", len(expectedImports), len(imports))
	}

	for i, imp := range imports {
		if imp != expectedImports[i] {
			t.Errorf("Import %d: expected %q, got %q", i, expectedImports[i], imp)
		}
	}
}

// TestFindGoMod tests finding go.mod in parent directories.
func TestFindGoMod(t *testing.T) {
	// Use the real forge project structure
	// Start from this file and find go.mod
	thisFile, err := filepath.Abs("detect_test.go")
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	goModPath, err := findGoMod(thisFile)
	if err != nil {
		t.Fatalf("Failed to find go.mod: %v", err)
	}

	// Verify it's actually a go.mod file
	if filepath.Base(goModPath) != "go.mod" {
		t.Errorf("Expected go.mod, got %s", filepath.Base(goModPath))
	}

	// Verify it exists
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		t.Errorf("go.mod not found at %s", goModPath)
	}
}

func TestDetectDependencies_NoImports(t *testing.T) {
	// Create a temporary project structure
	tmpDir := t.TempDir()

	// Create go.mod
	goModContent := `module example.com/test

go 1.23
`
	err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	// Create test file with no imports
	testFile := filepath.Join(tmpDir, "main.go")
	testCode := `package main

func main() {
	x := 42
	_ = x
}
`
	err = os.WriteFile(testFile, []byte(testCode), 0o644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Test dependency detection
	input := mcptypes.DetectDependenciesInput{
		FilePath: testFile,
		FuncName: "main",
	}

	output, err := DetectDependencies(input)
	if err != nil {
		t.Fatalf("DetectDependencies failed: %v", err)
	}

	// go.mod and the entry file itself. The entry file used to be walked for
	// its imports and never recorded, so editing it rebuilt nothing.
	if len(output.Dependencies) != 2 {
		t.Errorf("Expected 2 dependencies (go.mod + main.go), got %d", len(output.Dependencies))
	}

	// Verify it's go.mod
	if len(output.Dependencies) > 0 {
		dep := output.Dependencies[0]
		if !strings.HasPrefix(dep.Digest, forge.DigestPrefix) {
			t.Errorf("Expected a content digest for go.mod, got %q", dep.Digest)
		}
		if filepath.Base(dep.Path) != "go.mod" {
			t.Errorf("Expected go.mod, got %s", filepath.Base(dep.Path))
		}
	}

	if !hasFile(output.Dependencies, "main.go") {
		t.Error("the entry file must be a dependency of the thing built from it")
	}
}

// hasFile reports whether the dependencies carry a file with this base name.
func hasFile(deps []mcptypes.Dependency, base string) bool {
	for _, dep := range deps {
		if filepath.Base(dep.Path) == base {
			return true
		}
	}

	return false
}

// TestDetectDependencies_StdlibOnly tests a function with only stdlib imports.
func TestDetectDependencies_StdlibOnly(t *testing.T) {
	// Create a temporary project structure
	tmpDir := t.TempDir()

	// Create go.mod
	goModContent := `module example.com/test

go 1.23
`
	err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	// Create test file with stdlib imports only
	testFile := filepath.Join(tmpDir, "main.go")
	testCode := `package main

import (
	"fmt"
	"encoding/json"
)

func main() {
	fmt.Println("hello")
	json.Marshal(nil)
}
`
	err = os.WriteFile(testFile, []byte(testCode), 0o644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Test dependency detection
	input := mcptypes.DetectDependenciesInput{
		FilePath: testFile,
		FuncName: "main",
	}

	output, err := DetectDependencies(input)
	if err != nil {
		t.Fatalf("DetectDependencies failed: %v", err)
	}

	// Should have 1 dependency (go.mod, stdlib is excluded)
	if len(output.Dependencies) != 2 {
		t.Errorf("Expected 2 dependencies (go.mod + main.go, stdlib excluded), got %d",
			len(output.Dependencies))
	}

	// Verify it's go.mod
	if len(output.Dependencies) > 0 {
		dep := output.Dependencies[0]
		if !strings.HasPrefix(dep.Digest, forge.DigestPrefix) {
			t.Errorf("Expected a content digest for go.mod, got %q", dep.Digest)
		}
		if filepath.Base(dep.Path) != "go.mod" {
			t.Errorf("Expected go.mod, got %s", filepath.Base(dep.Path))
		}
	}
}

// TestDetectDependencies_ExternalPackage tests a function with external imports.
func TestDetectDependencies_ExternalPackage(t *testing.T) {
	// Create a temporary project structure
	tmpDir := t.TempDir()

	// Create go.mod with external dependency
	goModContent := `module example.com/test

go 1.23

require (
	github.com/stretchr/testify v1.8.4
	golang.org/x/mod v0.30.0
)
`
	err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	// Create test file with external imports
	testFile := filepath.Join(tmpDir, "main.go")
	testCode := `package main

import (
	"github.com/stretchr/testify/assert"
	"golang.org/x/mod/modfile"
)

func main() {
	assert.NotNil(nil, "test")
	modfile.Parse("", nil, nil)
}
`
	err = os.WriteFile(testFile, []byte(testCode), 0o644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Test dependency detection
	input := mcptypes.DetectDependenciesInput{
		FilePath: testFile,
		FuncName: "main",
	}

	output, err := DetectDependencies(input)
	if err != nil {
		t.Fatalf("DetectDependencies failed: %v", err)
	}

	// go.mod and main.go, and nothing for the two external packages: their
	// versions are lines in go.sum, recorded beside go.mod when it exists,
	// and a module is never a record of its own.
	if len(output.Dependencies) != 2 {
		t.Errorf("Expected 2 dependencies (go.mod + main.go), got %d: %+v", len(output.Dependencies), output.Dependencies)
	}

	if len(output.Dependencies) > 0 && filepath.Base(output.Dependencies[0].Path) != "go.mod" {
		t.Errorf("Expected go.mod first, got %s", filepath.Base(output.Dependencies[0].Path))
	}

	for _, dep := range output.Dependencies {
		if !strings.HasPrefix(dep.Digest, forge.DigestPrefix) {
			t.Errorf("Expected a content digest for %s, got %q", dep.Path, dep.Digest)
		}
	}
}

// A lock file beside the manifest is recorded with it: that is where an
// external module's version lives, so a bump is a changed digest here.
func TestDetectDependencies_TheLockIsRecordedBesideTheManifest(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module example.com/test\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "go.sum"), []byte("example.com/dep v1.0.0 h1:abc=\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	mainFile := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(mainFile, []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	output, err := DetectDependencies(mcptypes.DetectDependenciesInput{FilePath: mainFile, FuncName: "main"})
	if err != nil {
		t.Fatal(err)
	}

	var names []string
	for _, dep := range output.Dependencies {
		names = append(names, filepath.Base(dep.Path))
	}

	if strings.Join(names, ",") != "go.mod,go.sum,main.go" {
		t.Fatalf("expected go.mod,go.sum,main.go, got %v", names)
	}
}

// TestDetectDependencies_LocalPackage tests a function with local package imports.
func TestDetectDependencies_LocalPackage(t *testing.T) {
	// Create a temporary project structure
	tmpDir := t.TempDir()

	// Create go.mod
	goModContent := `module example.com/test

go 1.23
`
	err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	// Create a local package
	pkgDir := filepath.Join(tmpDir, "pkg", "helper")
	err = os.MkdirAll(pkgDir, 0o755)
	if err != nil {
		t.Fatalf("Failed to create pkg dir: %v", err)
	}

	helperFile := filepath.Join(pkgDir, "helper.go")
	helperCode := `package helper

func Helper() string {
	return "helper"
}
`
	err = os.WriteFile(helperFile, []byte(helperCode), 0o644)
	if err != nil {
		t.Fatalf("Failed to write helper file: %v", err)
	}

	// Create main file that imports local package
	testFile := filepath.Join(tmpDir, "main.go")
	testCode := `package main

import (
	"example.com/test/pkg/helper"
)

func main() {
	helper.Helper()
}
`
	err = os.WriteFile(testFile, []byte(testCode), 0o644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Test dependency detection
	input := mcptypes.DetectDependenciesInput{
		FilePath: testFile,
		FuncName: "main",
	}

	output, err := DetectDependencies(input)
	if err != nil {
		t.Fatalf("DetectDependencies failed: %v", err)
	}

	// Should have 2 file dependencies: go.mod + helper.go
	if len(output.Dependencies) != 3 {
		t.Errorf("Expected 3 dependencies (go.mod + main.go + helper.go), got %d",
			len(output.Dependencies))
	}

	// First dependency should be go.mod
	if len(output.Dependencies) > 0 {
		dep := output.Dependencies[0]
		if !strings.HasPrefix(dep.Digest, forge.DigestPrefix) {
			t.Errorf("Expected a content digest for go.mod, got %q", dep.Digest)
		}
		if filepath.Base(dep.Path) != "go.mod" {
			t.Errorf("Expected go.mod, got %s", filepath.Base(dep.Path))
		}
	}

	// helper.go is in there, found by name: the entry file is a dependency
	// too now, so nothing here may depend on its position.
	if !hasFile(output.Dependencies, "helper.go") {
		t.Error("Expected helper.go among the dependencies")
	}

	for _, dep := range output.Dependencies {
		if !filepath.IsAbs(dep.Path) {
			t.Errorf("Expected absolute path, got %q", dep.Path)
		}

		if !strings.HasPrefix(dep.Digest, forge.DigestPrefix) {
			t.Errorf("Expected a content digest for %s, got %q", dep.Path, dep.Digest)
		}
	}
}

// TestDetectDependencies_TransitiveDependencies tests transitive dependencies.
func TestDetectDependencies_TransitiveDependencies(t *testing.T) {
	// Create a temporary project structure: A -> B -> C
	tmpDir := t.TempDir()

	// Create go.mod
	goModContent := `module example.com/test

go 1.23
`
	err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	// Create package C
	pkgCDir := filepath.Join(tmpDir, "pkg", "c")
	err = os.MkdirAll(pkgCDir, 0o755)
	if err != nil {
		t.Fatalf("Failed to create pkg/c dir: %v", err)
	}

	cFile := filepath.Join(pkgCDir, "c.go")
	cCode := `package c

func C() string {
	return "c"
}
`
	err = os.WriteFile(cFile, []byte(cCode), 0o644)
	if err != nil {
		t.Fatalf("Failed to write c.go: %v", err)
	}

	// Create package B (imports C)
	pkgBDir := filepath.Join(tmpDir, "pkg", "b")
	err = os.MkdirAll(pkgBDir, 0o755)
	if err != nil {
		t.Fatalf("Failed to create pkg/b dir: %v", err)
	}

	bFile := filepath.Join(pkgBDir, "b.go")
	bCode := `package b

import "example.com/test/pkg/c"

func B() string {
	return c.C()
}
`
	err = os.WriteFile(bFile, []byte(bCode), 0o644)
	if err != nil {
		t.Fatalf("Failed to write b.go: %v", err)
	}

	// Create main file (imports B)
	testFile := filepath.Join(tmpDir, "main.go")
	testCode := `package main

import "example.com/test/pkg/b"

func main() {
	b.B()
}
`
	err = os.WriteFile(testFile, []byte(testCode), 0o644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Test dependency detection
	input := mcptypes.DetectDependenciesInput{
		FilePath: testFile,
		FuncName: "main",
	}

	output, err := DetectDependencies(input)
	if err != nil {
		t.Fatalf("DetectDependencies failed: %v", err)
	}

	// Should have 3 file dependencies: go.mod + b.go + c.go
	if len(output.Dependencies) != 4 {
		t.Errorf("Expected 4 dependencies (go.mod + main.go + transitive), got %d",
			len(output.Dependencies))
	}

	// First dependency should be go.mod
	if len(output.Dependencies) > 0 {
		dep := output.Dependencies[0]
		if !strings.HasPrefix(dep.Digest, forge.DigestPrefix) {
			t.Errorf("Expected a content digest for go.mod, got %q", dep.Digest)
		}
		if filepath.Base(dep.Path) != "go.mod" {
			t.Errorf("Expected go.mod, got %s", filepath.Base(dep.Path))
		}
	}

	// Verify both b.go and c.go are present (skip go.mod at index 0)
	foundB := false
	foundC := false
	for i := 1; i < len(output.Dependencies); i++ {
		dep := output.Dependencies[i]
		if !strings.HasPrefix(dep.Digest, forge.DigestPrefix) {
			t.Errorf("Expected a content digest, got %q", dep.Digest)
		}
		baseName := filepath.Base(dep.Path)
		if baseName == "b.go" {
			foundB = true
		} else if baseName == "c.go" {
			foundC = true
		}
	}

	if !foundB {
		t.Error("Expected b.go in dependencies")
	}
	if !foundC {
		t.Error("Expected c.go in dependencies (transitive)")
	}
}

// TestDetectDependencies_CircularDependency tests circular dependency handling.
func TestDetectDependencies_CircularDependency(t *testing.T) {
	// Create a temporary project structure: A -> B -> A
	tmpDir := t.TempDir()

	// Create go.mod
	goModContent := `module example.com/test

go 1.23
`
	err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	// Create package A (imports B)
	pkgADir := filepath.Join(tmpDir, "pkg", "a")
	err = os.MkdirAll(pkgADir, 0o755)
	if err != nil {
		t.Fatalf("Failed to create pkg/a dir: %v", err)
	}

	aFile := filepath.Join(pkgADir, "a.go")
	aCode := `package a

import "example.com/test/pkg/b"

func A() string {
	return b.B()
}
`
	err = os.WriteFile(aFile, []byte(aCode), 0o644)
	if err != nil {
		t.Fatalf("Failed to write a.go: %v", err)
	}

	// Create package B (imports A - circular!)
	pkgBDir := filepath.Join(tmpDir, "pkg", "b")
	err = os.MkdirAll(pkgBDir, 0o755)
	if err != nil {
		t.Fatalf("Failed to create pkg/b dir: %v", err)
	}

	bFile := filepath.Join(pkgBDir, "b.go")
	bCode := `package b

import "example.com/test/pkg/a"

func B() string {
	return a.A()
}
`
	err = os.WriteFile(bFile, []byte(bCode), 0o644)
	if err != nil {
		t.Fatalf("Failed to write b.go: %v", err)
	}

	// Test dependency detection starting from A
	input := mcptypes.DetectDependenciesInput{
		FilePath: aFile,
		FuncName: "A",
	}

	// Should NOT hang or panic - memoization prevents infinite loop
	output, err := DetectDependencies(input)
	if err != nil {
		t.Fatalf("DetectDependencies failed: %v", err)
	}

	// Should have 2 file dependencies: go.mod + b.go
	// When we start from A, we mark A as visited, then process B
	// B tries to import A, but A is already visited, so we skip it
	// Therefore, we get go.mod + B in dependencies (A is the starting point, not a dependency)
	if len(output.Dependencies) != 3 {
		t.Errorf("Expected 3 dependencies (go.mod + main.go + circular handled), got %d",
			len(output.Dependencies))
		for i, dep := range output.Dependencies {
			t.Logf("Dependency %d: %s", i, filepath.Base(dep.Path))
		}
	}

	// First dependency should be go.mod
	if len(output.Dependencies) > 0 {
		dep := output.Dependencies[0]
		if !strings.HasPrefix(dep.Digest, forge.DigestPrefix) {
			t.Errorf("Expected a content digest for go.mod, got %q", dep.Digest)
		}
		if filepath.Base(dep.Path) != "go.mod" {
			t.Errorf("Expected go.mod, got %s", filepath.Base(dep.Path))
		}
	}

	// b.go is reached through the cycle, found by name rather than position.
	if !hasFile(output.Dependencies, "b.go") {
		t.Error("Expected b.go among the dependencies")
	}

	// Verify no duplicates
	seen := make(map[string]bool)
	for _, dep := range output.Dependencies {
		if seen[dep.Path] {
			t.Errorf("Duplicate dependency: %s", dep.Path)
		}
		seen[dep.Path] = true
	}
}

// TestDetectDependencies_FunctionNotFound tests error when function doesn't exist.
func TestDetectDependencies_FunctionNotFound(t *testing.T) {
	// Create a temporary project structure
	tmpDir := t.TempDir()

	// Create go.mod
	goModContent := `module example.com/test

go 1.23
`
	err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	// Create test file
	testFile := filepath.Join(tmpDir, "main.go")
	testCode := `package main

func main() {
}
`
	err = os.WriteFile(testFile, []byte(testCode), 0o644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Test dependency detection with non-existent function
	input := mcptypes.DetectDependenciesInput{
		FilePath: testFile,
		FuncName: "nonExistent",
	}

	_, err = DetectDependencies(input)
	if err == nil {
		t.Fatal("Expected error for non-existent function, got nil")
	}

	if !strings.Contains(err.Error(), "function nonExistent not found") {
		t.Errorf("Expected 'function nonExistent not found' error, got: %v", err)
	}
}

// TestDetectDependencies_NonExistentFile tests error when file doesn't exist.
func TestDetectDependencies_NonExistentFile(t *testing.T) {
	input := mcptypes.DetectDependenciesInput{
		FilePath: "/this/path/does/not/exist/file.go",
		FuncName: "main",
	}

	_, err := DetectDependencies(input)
	if err == nil {
		t.Fatal("Expected error for non-existent file, got nil")
	}

	// Verify error message mentions file not found
	if !strings.Contains(err.Error(), "no such file") &&
		!strings.Contains(err.Error(), "does not exist") {
		t.Errorf("Expected file not found error, got: %v", err)
	}
}

// TestDetectDependencies_EveryFileOfAPackage is the case the rest of this
// suite cannot express: every other fixture package holds exactly one .go
// file, so taking only the first one looked identical to taking all of them.
//
// Here the package holds three files and only the alphabetically LAST one
// reaches deeper. Resolving a package to its first file stopped at a.go and
// never learned that deep exists - which is how forge-ci's binary came to
// track reconcilecontroller/index.go and miss fourteen packages.
func TestDetectDependencies_EveryFileOfAPackage(t *testing.T) {
	tmpDir := t.TempDir()

	goMod := `module example.com/test

go 1.23
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	deepDir := filepath.Join(tmpDir, "deep")
	if err := os.MkdirAll(deepDir, 0o755); err != nil {
		t.Fatalf("Failed to create deep dir: %v", err)
	}

	deepCode := `package deep

func Deep() string { return "deep" }
`
	if err := os.WriteFile(filepath.Join(deepDir, "deep.go"), []byte(deepCode), 0o644); err != nil {
		t.Fatalf("Failed to write deep.go: %v", err)
	}

	wideDir := filepath.Join(tmpDir, "wide")
	if err := os.MkdirAll(wideDir, 0o755); err != nil {
		t.Fatalf("Failed to create wide dir: %v", err)
	}

	// a.go sorts first and imports nothing. The old walk stopped here.
	aCode := `package wide

func A() string { return "a" }
`
	if err := os.WriteFile(filepath.Join(wideDir, "a.go"), []byte(aCode), 0o644); err != nil {
		t.Fatalf("Failed to write a.go: %v", err)
	}

	bCode := `package wide

func B() string { return "b" }
`
	if err := os.WriteFile(filepath.Join(wideDir, "b.go"), []byte(bCode), 0o644); err != nil {
		t.Fatalf("Failed to write b.go: %v", err)
	}

	// z.go sorts last and is the only door to the deep package.
	zCode := `package wide

import "example.com/test/deep"

func Z() string { return deep.Deep() }
`
	if err := os.WriteFile(filepath.Join(wideDir, "z.go"), []byte(zCode), 0o644); err != nil {
		t.Fatalf("Failed to write z.go: %v", err)
	}

	mainFile := filepath.Join(tmpDir, "main.go")
	mainCode := `package main

import "example.com/test/wide"

func main() {
	_ = wide.A()
}
`
	if err := os.WriteFile(mainFile, []byte(mainCode), 0o644); err != nil {
		t.Fatalf("Failed to write main.go: %v", err)
	}

	output, err := DetectDependencies(mcptypes.DetectDependenciesInput{
		FilePath: mainFile,
		FuncName: "main",
	})
	if err != nil {
		t.Fatalf("DetectDependencies failed: %v", err)
	}

	for _, want := range []string{"go.mod", "main.go", "a.go", "b.go", "z.go", "deep.go"} {
		if !hasFile(output.Dependencies, want) {
			t.Errorf("%s is not tracked, so editing it would rebuild nothing", want)
		}
	}
}

// TestDetectDependencies_PackageSiblingOfTheEntryFile is golden-go's
// config-demo reduced: a main package of two files, where the second is
// generated. Walking imports alone never sees a sibling, because a sibling is
// not imported - so config-demo came out with go.mod as its only dependency
// and no edit to either file ever rebuilt it.
func TestDetectDependencies_PackageSiblingOfTheEntryFile(t *testing.T) {
	tmpDir := t.TempDir()

	goMod := `module example.com/test

go 1.23
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	mainFile := filepath.Join(tmpDir, "main.go")
	mainCode := `package main

func main() {
	_ = Config()
}
`
	if err := os.WriteFile(mainFile, []byte(mainCode), 0o644); err != nil {
		t.Fatalf("Failed to write main.go: %v", err)
	}

	generated := `package main

func Config() string { return "generated" }
`
	if err := os.WriteFile(filepath.Join(tmpDir, "zz_generated.config.go"), []byte(generated), 0o644); err != nil {
		t.Fatalf("Failed to write zz_generated.config.go: %v", err)
	}

	output, err := DetectDependencies(mcptypes.DetectDependenciesInput{
		FilePath: mainFile,
		FuncName: "main",
	})
	if err != nil {
		t.Fatalf("DetectDependencies failed: %v", err)
	}

	if !hasFile(output.Dependencies, "zz_generated.config.go") {
		t.Error("a sibling in the entry package must be tracked; it is compiled into the binary")
	}
}

// TestDetectDependencies_TestFilesAreNotDependencies keeps the walk off test
// files. They are not compiled into the artifact, so an edit to one must not
// invalidate a build that does not contain it.
func TestDetectDependencies_TestFilesAreNotDependencies(t *testing.T) {
	tmpDir := t.TempDir()

	goMod := `module example.com/test

go 1.23
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	mainFile := filepath.Join(tmpDir, "main.go")
	mainCode := `package main

func main() {}
`
	if err := os.WriteFile(mainFile, []byte(mainCode), 0o644); err != nil {
		t.Fatalf("Failed to write main.go: %v", err)
	}

	testCode := `package main

import "testing"

func TestNothing(t *testing.T) {}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "main_test.go"), []byte(testCode), 0o644); err != nil {
		t.Fatalf("Failed to write main_test.go: %v", err)
	}

	output, err := DetectDependencies(mcptypes.DetectDependenciesInput{
		FilePath: mainFile,
		FuncName: "main",
	})
	if err != nil {
		t.Fatalf("DetectDependencies failed: %v", err)
	}

	if hasFile(output.Dependencies, "main_test.go") {
		t.Error("a test file is not compiled into the artifact and must not be a dependency")
	}
}
