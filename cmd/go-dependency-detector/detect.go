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
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/alexandremahdhaoui/forge/internal/gosource"
	"github.com/alexandremahdhaoui/forge/pkg/forge"
	"github.com/alexandremahdhaoui/forge/pkg/mcptypes"
	"golang.org/x/mod/modfile"
)

// dependencyTracker tracks visited files/packages and accumulates dependencies.
type dependencyTracker struct {
	visitedFiles    map[string]bool // Absolute file paths
	visitedPackages map[string]bool // Import paths
	goModPath       string          // Absolute path to go.mod
	goModData       *modfile.File   // Parsed go.mod
	dependencies    []mcptypes.Dependency
	moduleDir       string // Directory containing go.mod
	modulePath      string // Module name from go.mod
}

// DetectDependencies detects all dependencies for a given Go function.
// It recursively analyzes all transitive dependencies (local and external packages).
func DetectDependencies(input mcptypes.DetectDependenciesInput) (mcptypes.DetectDependenciesOutput, error) {
	// Step 1: Convert input.FilePath to absolute path
	absFilePath, err := filepath.Abs(input.FilePath)
	if err != nil {
		return mcptypes.DetectDependenciesOutput{}, fmt.Errorf("failed to resolve absolute path for %s: %w", input.FilePath, err)
	}

	// Step 2: Find and parse go.mod file
	goModPath, err := findGoMod(absFilePath)
	if err != nil {
		return mcptypes.DetectDependenciesOutput{}, fmt.Errorf("go.mod not found: %w", err)
	}

	goModData, err := parseGoMod(goModPath)
	if err != nil {
		return mcptypes.DetectDependenciesOutput{}, fmt.Errorf("failed to parse go.mod: %w", err)
	}

	// The manifest and the lock are the first two dependencies: an external
	// module's version lives in them, so a bump shows as a changed digest
	// of go.sum and no module is recorded on its own.
	manifest, err := forge.DependencyOf(goModPath)
	if err != nil {
		return mcptypes.DetectDependenciesOutput{}, err
	}

	seed := []mcptypes.Dependency{{Path: manifest.Path, Digest: manifest.Digest}}

	goSumPath := filepath.Join(filepath.Dir(goModPath), "go.sum")
	if _, err := os.Stat(goSumPath); err == nil {
		lock, err := forge.DependencyOf(goSumPath)
		if err != nil {
			return mcptypes.DetectDependenciesOutput{}, err
		}

		seed = append(seed, mcptypes.Dependency{Path: lock.Path, Digest: lock.Digest})
	}

	// Step 3: Parse the Go file at input.FilePath
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, absFilePath, nil, parser.ParseComments)
	if err != nil {
		return mcptypes.DetectDependenciesOutput{}, fmt.Errorf("failed to parse file %s: %w", absFilePath, err)
	}

	// Step 4: Find the function specified by input.FuncName
	funcFound := false
	ast.Inspect(file, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok && fn.Name.Name == input.FuncName {
			funcFound = true
		}
		return true
	})
	if !funcFound {
		return mcptypes.DetectDependenciesOutput{}, fmt.Errorf("function %s not found in %s", input.FuncName, absFilePath)
	}

	// Step 5: Initialize dependencyTracker with go.mod as first dependency
	tracker := &dependencyTracker{
		visitedFiles:    make(map[string]bool),
		visitedPackages: make(map[string]bool),
		goModPath:       goModPath,
		goModData:       goModData,
		dependencies:    seed,
		moduleDir:       filepath.Dir(goModPath),
		modulePath:      goModData.Module.Mod.Path,
	}

	// Step 6: Record the entry package and traverse everything it imports.
	//
	// The entry point is a package and not a file. Starting from the file
	// alone left main.go untracked - it was walked for its imports and never
	// recorded - and left every sibling in its package unseen, because the
	// walk follows imports and a sibling is not imported. golden-go's
	// config-demo came out with go.mod as its ONLY dependency, so editing
	// either of its two source files rebuilt nothing.
	err = tracker.processPackage(filepath.Dir(absFilePath))
	if err != nil {
		return mcptypes.DetectDependenciesOutput{}, err
	}

	// Step 7: Return DependencyDetectorOutput with all collected dependencies
	return mcptypes.DetectDependenciesOutput{
		Dependencies: tracker.dependencies,
	}, nil
}

// processPackage records every file of one package and walks what they
// import. A package is all of its files: reading one of them and calling that
// the package is what made 14 of forge-ci's 24 packages invisible.
func (t *dependencyTracker) processPackage(dir string) error {
	files, err := gosource.ListGoFiles(dir)
	if err != nil {
		return err
	}

	for _, file := range files {
		if err := t.processFile(file); err != nil {
			return err
		}
	}

	return nil
}

// processFile records one file and recursively processes what it imports.
func (t *dependencyTracker) processFile(filePath string) error {
	// Prevent cycles
	if t.visitedFiles[filePath] {
		return nil
	}
	t.visitedFiles[filePath] = true

	// Record the file itself before walking it. A file that is read for its
	// imports but never recorded is a file whose own edits are invisible.
	// go.mod is already in the list, so it is not recorded twice.
	if filePath != t.goModPath {
		digest, err := forge.DigestFile(filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to digest %s: %v\n", filePath, err)
		} else {
			t.dependencies = append(t.dependencies, mcptypes.Dependency{Path: filePath, Digest: digest})
		}
	}

	// Parse the file
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		// Log warning but continue (as per spec)
		fmt.Fprintf(os.Stderr, "Warning: failed to parse %s: %v\n", filePath, err)
		return nil
	}

	// Extract imports
	imports := extractImports(file)

	// Process each import
	for _, importPath := range imports {
		// Skip standard library
		if isStandardLibrary(importPath) {
			continue
		}

		// Check if already visited
		if t.visitedPackages[importPath] {
			continue
		}

		// Determine if this is a local or external package
		isLocal := isLocalPackage(importPath, t.modulePath, t.goModData)

		if isLocal {
			// Local package - resolve to its directory and take all of it
			pkgDir, err := resolveLocalPackage(t.goModData, importPath, t.moduleDir, t.modulePath)
			if err != nil {
				// Log warning but continue
				fmt.Fprintf(os.Stderr, "Warning: failed to resolve local package %s: %v\n", importPath, err)
				continue
			}

			// Mark the package visited before recursing, so a cycle between
			// two packages terminates here rather than on the stack.
			t.visitedPackages[importPath] = true

			// Recurse into every file of the local package
			err = t.processPackage(pkgDir)
			if err != nil {
				// Log warning but continue
				fmt.Fprintf(os.Stderr, "Warning: failed to process package %s: %v\n", importPath, err)
			}
		} else {
			// An external package is recorded by nothing of its own: its
			// version is a line in go.sum, already digested, and a module
			// the manifest does not know is still refused here so a build
			// that would fail says why before it runs.
			if _, err := getPackageVersion(t.goModData, importPath); err != nil {
				return fmt.Errorf("package %s not found in go.mod: %w", importPath, err)
			}

			// Mark package as visited
			t.visitedPackages[importPath] = true
		}
	}

	return nil
}

// extractImports extracts all import paths from a Go file.
func extractImports(file *ast.File) []string {
	var imports []string
	for _, imp := range file.Imports {
		// Remove quotes from import path
		importPath := strings.Trim(imp.Path.Value, `"`)
		imports = append(imports, importPath)
	}
	return imports
}

// isStandardLibrary checks if a package is in the Go standard library.
// Algorithm: Check if import path does NOT contain a dot in first segment.
func isStandardLibrary(pkgPath string) bool {
	// Special case: "C" is NOT stdlib (cgo)
	if pkgPath == "C" {
		return false
	}

	// Check first segment for dot
	firstSegment := pkgPath
	if idx := strings.Index(pkgPath, "/"); idx != -1 {
		firstSegment = pkgPath[:idx]
	}

	// If first segment contains a dot, it's not stdlib
	return !strings.Contains(firstSegment, ".")
}

// isLocalPackage determines if an import is a local package (vs external).
func isLocalPackage(importPath, modulePath string, goModData *modfile.File) bool {
	// Check if it's under the module path
	if strings.HasPrefix(importPath, modulePath) {
		return true
	}

	// Check if there's a replace directive pointing to a local path
	for _, replace := range goModData.Replace {
		if replace.Old.Path == importPath {
			// If new path is a relative path, it's local
			if strings.HasPrefix(replace.New.Path, ".") || strings.HasPrefix(replace.New.Path, "/") {
				return true
			}
			// If new path is under module path, it's local
			if strings.HasPrefix(replace.New.Path, modulePath) {
				return true
			}
		}
	}

	return false
}

// resolveLocalPackage resolves a local import path to the directory holding
// that package. It answers a directory rather than a file because a package
// is every file in it, and the caller reads all of them.
func resolveLocalPackage(goModData *modfile.File, importPath, goModDir, modulePath string) (string, error) {
	// Check for replace directive
	for _, replace := range goModData.Replace {
		if replace.Old.Path == importPath {
			// Use replacement path
			replacePath := replace.New.Path
			if strings.HasPrefix(replacePath, ".") {
				// Relative path
				replacePath = filepath.Join(goModDir, replacePath)
			}

			return replacePath, nil
		}
	}

	// Construct path relative to module root
	if !strings.HasPrefix(importPath, modulePath) {
		return "", fmt.Errorf("import path %s is not under module path %s", importPath, modulePath)
	}

	// Remove module path prefix to get relative path
	relPath := strings.TrimPrefix(importPath, modulePath)
	relPath = strings.TrimPrefix(relPath, "/")

	return filepath.Join(goModDir, relPath), nil
}

// getPackageVersion extracts the version of an external package from go.mod.
func getPackageVersion(goModData *modfile.File, pkgPath string) (string, error) {
	// Search in require directives
	for _, req := range goModData.Require {
		if req.Mod.Path == pkgPath {
			return req.Mod.Version, nil
		}
		// Handle subpackages (e.g., github.com/foo/bar/baz should match github.com/foo/bar)
		if strings.HasPrefix(pkgPath, req.Mod.Path+"/") {
			return req.Mod.Version, nil
		}
	}

	// Check replace directives
	for _, replace := range goModData.Replace {
		if replace.Old.Path == pkgPath {
			if replace.New.Version != "" {
				return replace.New.Version, nil
			}
		}
	}

	return "", fmt.Errorf("package %s not found in go.mod", pkgPath)
}

// findGoMod walks up the directory tree to find go.mod.
func findGoMod(startPath string) (string, error) {
	dir := startPath
	if !filepath.IsAbs(dir) {
		var err error
		dir, err = filepath.Abs(dir)
		if err != nil {
			return "", err
		}
	}

	// If startPath is a file, start from its directory
	info, err := os.Stat(dir)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		dir = filepath.Dir(dir)
	}

	// Walk up the directory tree
	for {
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return goModPath, nil
		}

		// Move up one directory
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root
			return "", fmt.Errorf("go.mod not found in any parent directory of %s", startPath)
		}
		dir = parent
	}
}

// parseGoMod parses a go.mod file.
func parseGoMod(goModPath string) (*modfile.File, error) {
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read go.mod: %w", err)
	}

	modFile, err := modfile.Parse(goModPath, data, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to parse go.mod: %w", err)
	}

	return modFile, nil
}
