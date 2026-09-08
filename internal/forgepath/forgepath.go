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

// Package forgepath provides utilities for locating the forge source repository
// and constructing commands to execute forge tools via `go run`.
package forgepath

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

const (
	forgeModule = "github.com/alexandremahdhaoui/forge"

	// The two inputs run-local mode reads, and the only two. LocalCheckout
	// is their one reader.
	runLocalEnabledEnv = "FORGE_RUN_LOCAL_ENABLED"
	runLocalBaseDirEnv = "FORGE_RUN_LOCAL_BASEDIR"
)

// RunLocal reports whether engines run from a forge checkout rather than
// from a released module. It is the one place the switch is read.
func RunLocal() bool {
	return os.Getenv(runLocalEnabledEnv) == "true"
}

// LocalCheckout answers the forge checkout engines run from in run-local
// mode: the directory FORGE_RUN_LOCAL_BASEDIR names, or the checkout
// FindForgeRepo finds when it names none. It is an error to ask outside
// run-local mode, so a caller checks RunLocal first. This is the one owner
// of the ladder; nothing else reads the two variables.
func LocalCheckout() (string, error) {
	if !RunLocal() {
		return "", fmt.Errorf("engines run from released modules; set %s=true to run them from a checkout", runLocalEnabledEnv)
	}

	if dir := os.Getenv(runLocalBaseDirEnv); dir != "" {
		return dir, nil
	}

	dir, err := FindForgeRepo()
	if err != nil {
		return "", fmt.Errorf("%s=true but no forge checkout found; set %s=/path/to/forge: %w", runLocalEnabledEnv, runLocalBaseDirEnv, err)
	}

	return dir, nil
}

var (
	// Cache for forge repository path to avoid repeated filesystem/command operations
	cachedForgeRepoPath string
	cachedForgeRepoErr  error
	cacheOnce           sync.Once
)

// FindForgeRepo locates the forge source repository using multiple detection methods.
// It checks in the following order:
// 1. Go module cache using `go list -m -f '{{.Dir}}' github.com/alexandremahdhaoui/forge`
// 2. Walking up from os.Executable() to find forge repository
//
// Returns the absolute path to the forge repository or an error if not found.
func FindForgeRepo() (string, error) {
	cacheOnce.Do(func() {
		cachedForgeRepoPath, cachedForgeRepoErr = findForgeRepoUncached()
	})
	return cachedForgeRepoPath, cachedForgeRepoErr
}

// findForgeRepoUncached performs the actual forge repository detection without caching.
func findForgeRepoUncached() (string, error) {
	// Method 1: Use `go list` to find the module in Go's module cache
	cmd := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", forgeModule)
	output, err := cmd.Output()
	if err == nil {
		modulePath := strings.TrimSpace(string(output))
		if modulePath != "" && IsForgeRepo(modulePath) {
			return modulePath, nil
		}
	}

	// Method 2: Walk up from os.Executable() to find forge repository
	execPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get executable path: %w", err)
	}

	// Resolve symlinks
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve executable symlinks: %w", err)
	}

	// Walk up the directory tree
	dir := filepath.Dir(execPath)
	for {
		if IsForgeRepo(dir) {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("forge repository not found (checked: env var, go list, executable path)")
}

// IsForgeRepo checks if the given directory is the forge repository.
// It verifies by checking:
// 1. go.mod exists and contains the forge module path
// 2. cmd/forge/main.go exists (main forge CLI)
func IsForgeRepo(dir string) bool {
	// Check if go.mod exists and contains forge module
	goModPath := filepath.Join(dir, "go.mod")
	goModContent, err := os.ReadFile(goModPath)
	if err != nil {
		return false
	}

	// Check if go.mod declares the forge module
	if !strings.Contains(string(goModContent), forgeModule) {
		return false
	}

	// Check if cmd/forge/main.go exists
	forgeMainPath := filepath.Join(dir, "cmd", "forge", "main.go")
	if _, err := os.Stat(forgeMainPath); err != nil {
		return false
	}

	return true
}

// BuildGoRunCommand constructs the command arguments for executing a forge MCP server
// via `go run`. The returned slice is suitable for use with exec.Command("go", args...).
//
// Run-local mode (RunLocal, LocalCheckout) decides where the source comes
// from; this function only shapes the argv.
//
// Behavior:
//   - If run-local mode is on:
//     → Use `go run {basedir}/cmd/{packageName}` (absolute path preserves caller's CWD)
//   - Otherwise:
//     → Use `go run github.com/alexandremahdhaoui/forge/cmd/{packageName}@{forgeVersion}`
//
// Using @version syntax ensures go run uses forge's own dependencies from its go.mod/go.sum,
// not the consuming project's dependencies. This prevents dependency conflicts when forge
// is used as a library in other projects.
//
// Example usage:
//
//	args, err := BuildGoRunCommand("testenv-kind", "v0.9.0")
//	// Returns: ["run", "github.com/alexandremahdhaoui/forge/cmd/testenv-kind@v0.9.0"]
//	cmd := exec.Command("go", args...)
func BuildGoRunCommand(packageName, forgeVersion string) ([]string, error) {
	if packageName == "" {
		return nil, fmt.Errorf("package name cannot be empty")
	}
	if forgeVersion == "" {
		return nil, fmt.Errorf("forge version cannot be empty")
	}

	if RunLocal() {
		baseDir, err := LocalCheckout()
		if err != nil {
			return nil, err
		}

		// When the CWD is inside a Go module or workspace, use an absolute
		// filesystem path so the subprocess inherits the caller's working
		// directory. This is critical for workspace development where forge
		// operates on a different project than the forge repo itself.
		//
		// When the CWD has no module context (e.g. temp directories in tests),
		// fall back to -C which provides the forge repo's module context but
		// changes the subprocess CWD to the forge directory.
		pkgPath := filepath.Join(baseDir, "cmd", packageName)
		if cwdHasModuleContext() {
			return []string{"run", pkgPath}, nil
		}
		return []string{"-C", baseDir, "run", fmt.Sprintf("./cmd/%s", packageName)}, nil
	}

	if forgeVersion == "dev" {
		return nil, fmt.Errorf(
			"forge version is dev and no go.work above %s carries %s, so there is no version to run %s at; run from a workspace whose go.work lists forge, install a released forge, or set FORGE_RUN_LOCAL_ENABLED=true with FORGE_RUN_LOCAL_BASEDIR",
			cwdOrDot(), forgeModule, packageName)
	}

	moduleVersion := forgeVersion
	moduleVersion = strings.TrimSuffix(moduleVersion, "-dirty")
	moduleVersion = strings.TrimSuffix(moduleVersion, "+dirty")
	return []string{"run", fmt.Sprintf("%s/cmd/%s@%s", forgeModule, packageName, moduleVersion)}, nil
}

// IsForgeModulePath reports whether a full module path points inside the
// forge repository itself, e.g. github.com/alexandremahdhaoui/forge/cmd/go-build.
func IsForgeModulePath(path string) bool {
	return path == forgeModule || strings.HasPrefix(path, forgeModule+"/")
}

// IsExternalModule determines if a module path refers to an external module
// (i.e., not part of the forge repository).
//
// Returns true for external module paths like:
//   - github.com/user/repo/cmd/tool
//   - gitlab.com/org/project/pkg/util
//
// Returns false for:
//   - Short names (testenv-kind, go-build)
//   - Local paths (./cmd/tool, ../pkg/util) - these should be handled separately
//   - Empty paths
func IsExternalModule(path string) bool {
	if path == "" {
		return false
	}
	// Local paths are not external modules (handled by run-local mode)
	if strings.HasPrefix(path, "./") || strings.HasPrefix(path, "../") {
		return false
	}
	// Short names without "/" are internal forge packages
	if !strings.Contains(path, "/") {
		return false
	}
	// Check if first segment contains "." (e.g., github.com, gitlab.com)
	firstSlash := strings.Index(path, "/")
	firstSegment := path[:firstSlash]
	return strings.Contains(firstSegment, ".")
}

// cwdHasModuleContext checks whether the current working directory is inside
// a Go module or workspace. It walks up from CWD looking for go.mod or go.work.
func cwdHasModuleContext() bool {
	dir, err := os.Getwd()
	if err != nil {
		return false
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return true
		}
		if _, err := os.Stat(filepath.Join(dir, "go.work")); err == nil {
			return true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return false
		}
		dir = parent
	}
}

func EngineCommand(packageName, forgeVersion string) (string, []string, error) {
	if RunLocal() {
		return buildLocalEngine(packageName)
	}

	if packageName == "" {
		return "", nil, fmt.Errorf("package name cannot be empty")
	}

	if IsForgeWorkspaceMember() {
		return "go", []string{"run", forgeModule + "/cmd/" + packageName}, nil
	}

	args, err := BuildGoRunCommand(packageName, forgeVersion)
	if err != nil {
		return "", nil, err
	}

	return "go", args, nil
}

var (
	forgeMemberMu    sync.Mutex
	forgeMemberByCwd = map[string]bool{}
)

func IsForgeWorkspaceMember() bool {
	key := cwdOrDot() + "\x00" + os.Getenv("GOWORK")

	forgeMemberMu.Lock()
	defer forgeMemberMu.Unlock()

	if answer, known := forgeMemberByCwd[key]; known {
		return answer
	}

	answer := isWorkspaceModule(forgeModule)
	forgeMemberByCwd[key] = answer

	return answer
}

func cwdOrDot() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}

	return cwd
}

func buildLocalEngine(packageName string) (string, []string, error) {
	baseDir, err := LocalCheckout()
	if err != nil {
		return "", nil, err
	}

	bin := filepath.Join(baseDir, "build", "local-engines", packageName)

	build := exec.Command("go", "build", "-o", bin, "./cmd/"+packageName)
	build.Dir = baseDir
	build.Env = append(os.Environ(), "GOWORK=off")

	if out, err := build.CombinedOutput(); err != nil {
		return "", nil, fmt.Errorf("building local engine %s: %w: %s", packageName, err, string(out))
	}

	return bin, nil, nil
}

// IsWorkspaceModule reports whether the enclosing go.work carries the module,
// which is the engine-resolution twin of run's rule 2: the workspace wins.
func IsWorkspaceModule(modulePath string) bool {
	return isWorkspaceModule(modulePath)
}

// isWorkspaceModule checks if modulePath belongs to a module listed in the
// nearest go.work file. It parses the go.work use directives and reads each
// member's go.mod to extract module paths.
func isWorkspaceModule(modulePath string) bool {
	// GOWORK=off means the go command this answer becomes an argument to
	// will not read the workspace at all. Answering yes anyway produced
	// "go run <module path>" with no version, in a directory with no
	// go.mod, and the failure named the module rather than the workspace
	// that was switched off: "no required module provides package ...".
	// forge clone into a fresh directory hit it every time.
	if os.Getenv("GOWORK") == "off" {
		return false
	}

	goWorkDir := findGoWork()
	if goWorkDir == "" {
		return false
	}

	goWorkPath := filepath.Join(goWorkDir, "go.work")
	content, err := os.ReadFile(goWorkPath)
	if err != nil {
		return false
	}

	// Parse use directives from go.work
	useDirs := parseGoWorkUseDirs(string(content))

	for _, useDir := range useDirs {
		var absDir string
		if filepath.IsAbs(useDir) {
			absDir = useDir
		} else {
			absDir = filepath.Join(goWorkDir, useDir)
		}

		modPath := readModulePath(filepath.Join(absDir, "go.mod"))
		if modPath != "" && moduleCarries(modPath, modulePath) {
			return true
		}
	}

	return false
}

func moduleCarries(modPath, packagePath string) bool {
	return packagePath == modPath || strings.HasPrefix(packagePath, modPath+"/")
}

// findGoWork walks up from CWD looking for a go.work file and returns the
// directory containing it, or "" if not found.
func findGoWork() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.work")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// parseGoWorkUseDirs extracts directory paths from go.work use directives.
// Handles both single-line `use ./foo` and block `use ( ./foo \n ./bar )` syntax.
func parseGoWorkUseDirs(content string) []string {
	var dirs []string
	lines := strings.Split(content, "\n")
	inUseBlock := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if inUseBlock {
			if line == ")" {
				inUseBlock = false
				continue
			}
			// Strip comments
			if idx := strings.Index(line, "//"); idx >= 0 {
				line = strings.TrimSpace(line[:idx])
			}
			if line != "" {
				dirs = append(dirs, line)
			}
			continue
		}

		if strings.HasPrefix(line, "use (") {
			inUseBlock = true
			continue
		}
		if strings.HasPrefix(line, "use ") {
			dir := strings.TrimSpace(strings.TrimPrefix(line, "use "))
			if dir != "" {
				dirs = append(dirs, dir)
			}
		}
	}

	return dirs
}

// FindGoWork walks up from CWD looking for a go.work file and returns the
// directory containing it, or "" if not found.
func FindGoWork() string {
	return findGoWork()
}

// ParseGoWorkUseDirs extracts directory paths from go.work use directives.
// Handles both single-line `use ./foo` and block `use ( ./foo \n ./bar )` syntax.
func ParseGoWorkUseDirs(content string) []string {
	return parseGoWorkUseDirs(content)
}

// ReadModulePath reads a go.mod file and returns the module path declared in it.
func ReadModulePath(goModPath string) string {
	return readModulePath(goModPath)
}

// readModulePath reads a go.mod file and returns the module path declared in it.
func readModulePath(goModPath string) string {
	content, err := os.ReadFile(goModPath)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return ""
}
