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

// Package toolresolver answers how to execute a companion tool with a
// deterministic version. It is the one precedence rule for every exec the
// toolchain performs:
//
//  1. an explicit override from the caller (user config)
//  2. the enclosing workspace's checkout - dev always wins
//  3. the pinned store view (populated by forge-factory sync; absent until
//     a store exists)
//  4. PATH
//  5. `go run module@version` - and the version is always a pin: the ref's
//     own, the running binary's recorded dependency on the module, or a
//     companion pin stamped at release time. Never latest.
package toolresolver

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/alexandremahdhaoui/forge/internal/forgepath"
)

// Sources name where an invocation came from, for logs and doctor output.
const (
	SourceOverride  = "override"
	SourceWorkspace = "workspace"
	SourceStore     = "store"
	SourcePath      = "path"
	SourceGoRun     = "go-run"
	// SourceWorkspaceBin is a PATH hit served by an enclosing workspace's
	// .forge/bin - the pinned view sync provisioned, not a user install.
	SourceWorkspaceBin = "workspace-bin"
)

// workspaceBinDir is the per-workspace directory sync fills with the
// pinned tooling. EnsureWorkspaceBin exposes it; Resolve labels it.
const workspaceBinDir = ".forge/bin"

// EnsureWorkspaceBin puts the enclosing workspace's pinned tooling on this
// process's PATH, so every child - engines, delegated tools, run targets -
// resolves it with no .envrc sourced. It walks up from wd to the first
// directory carrying forge-factory.yaml alongside a .forge/bin directory,
// prepends that bin dir to PATH once, and answers it; outside a workspace,
// or when the dir is already on PATH, it changes nothing and answers "".
func EnsureWorkspaceBin(wd string) string {
	dir := wd

	for {
		factory := filepath.Join(dir, "forge-factory.yaml")
		bin := filepath.Join(dir, workspaceBinDir)

		if info, err := os.Stat(factory); err == nil && !info.IsDir() {
			if binInfo, err := os.Stat(bin); err == nil && binInfo.IsDir() {
				current := os.Getenv("PATH")

				for _, entry := range filepath.SplitList(current) {
					if entry == bin {
						return ""
					}
				}

				_ = os.Setenv("PATH", bin+string(os.PathListSeparator)+current)

				return bin
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}

		dir = parent
	}
}

// ForgeFactoryModule is the main package forge delegates workspace verbs to.
const ForgeFactoryModule = "github.com/alexandremahdhaoui/forge-factory/cmd/forge-factory"

// companionForgeFactory pins the forge-factory this forge build ships with -
// a commit sha, because forge-factory publishes no tags. The release
// pipeline overrides it with
//
//	-ldflags "-X github.com/alexandremahdhaoui/forge/pkg/toolresolver.companionForgeFactory=<sha>"
//
// and until then the source carries the last sha the workspace pipeline
// proved against this forge.
var companionForgeFactory = "d37002e4281fa6b29203c830da9d52dd5cd14edf"

// CompanionRevision is the distribution revision id a released binary
// shipped with. Empty on dev builds; release builds stamp it with -ldflags.
// The store resolves it to a provisioned view of the whole toolchain.
var CompanionRevision = ""

// Ref names one tool to resolve.
type Ref struct {
	// Name is the binary's base name, e.g. "forge-factory".
	Name string
	// Module is the full main-package module path; empty for a PATH-only
	// tool, which then has no go-run fallback.
	Module string
	// Version pins the go-run fallback. Empty consults the pin sources; it
	// never defaults to latest.
	Version string
}

// Invocation is how to exec the resolved tool: the ref's args go after Args.
type Invocation struct {
	Path   string
	Args   []string
	Source string
}

// Resolver resolves refs through the shared precedence. The zero value uses
// the process environment and PATH.
type Resolver struct {
	// Override is an explicit binary path that wins over everything.
	Override string
	// SourceDir is a checkout carrying cmd/<name>; when present the
	// workspace wins over PATH. The go run form is relative because go run
	// cannot cross module boundaries on an absolute path, so it works when
	// the process runs inside SourceDir - the callers that set it do.
	SourceDir string
	// LookPath defaults to exec.LookPath.
	LookPath func(string) (string, error)
	// StoreLookup answers the pinned store's absolute path for a name. Nil
	// until a store exists.
	StoreLookup func(name string) (string, bool)
	// PinVersion answers the version pinning a module's go-run fallback
	// when the ref carries none. Defaults to DepVersion.
	PinVersion func(module string) string
}

// Resolve answers how to exec the ref, or fails naming what would fix it.
func (r Resolver) Resolve(ref Ref) (Invocation, error) {
	if ref.Name == "" {
		return Invocation{}, fmt.Errorf("resolving a tool: the ref names no binary")
	}

	if r.Override != "" {
		return Invocation{Path: r.Override, Source: SourceOverride}, nil
	}

	// The workspace wins: in local mode a go.work that carries the module
	// runs the checkout, and a source dir with the command runs it in place.
	if ref.Module != "" && forgepath.RunLocal() && forgepath.IsWorkspaceModule(ref.Module) {
		return Invocation{Path: "go", Args: []string{"run", ref.Module}, Source: SourceWorkspace}, nil
	}

	if r.SourceDir != "" {
		if info, err := os.Stat(filepath.Join(r.SourceDir, "cmd", ref.Name)); err == nil && info.IsDir() {
			return Invocation{Path: "go", Args: []string{"run", "./cmd/" + ref.Name}, Source: SourceWorkspace}, nil
		}
	}

	if r.StoreLookup != nil {
		if path, ok := r.StoreLookup(ref.Name); ok {
			return Invocation{Path: path, Source: SourceStore}, nil
		}
	}

	lookPath := r.LookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}

	if path, err := lookPath(ref.Name); err == nil {
		// A hit under a workspace's .forge/bin is the pinned view sync
		// provisioned, not a user install: say so, honestly.
		source := SourcePath
		if strings.Contains(filepath.ToSlash(path), "/"+workspaceBinDir+"/") {
			source = SourceWorkspaceBin
		}

		return Invocation{Path: path, Source: source}, nil
	}

	version := ref.Version
	if version == "" && ref.Module != "" {
		pin := r.PinVersion
		if pin == nil {
			pin = DepVersion
		}

		version = pin(ref.Module)
	}

	if ref.Module == "" || version == "" || version == "dev" {
		return Invocation{}, fmt.Errorf(
			"nothing resolves %s: not on PATH and no version pins a go run fallback; install it or name a version",
			ref.Name)
	}

	version = strings.TrimSuffix(strings.TrimSuffix(version, "-dirty"), "+dirty")

	return Invocation{Path: "go", Args: []string{"run", ref.Module + "@" + version}, Source: SourceGoRun}, nil
}

// ForgeFactory resolves the forge-factory companion for delegation: the
// workspace copy in local mode, then the pinned store view a released
// binary ships with, then PATH, then go run at the pinned companion sha -
// never latest.
func ForgeFactory() (Invocation, error) {
	r := Resolver{
		StoreLookup: DefaultStoreLookup,
		PinVersion: func(module string) string {
			if v := DepVersion(module); v != "" {
				return v
			}

			return companionForgeFactory
		},
	}

	return r.Resolve(Ref{Name: "forge-factory", Module: ForgeFactoryModule})
}

// StoreDir answers the store root: FORGE_STORE_DIR, else the user cache's
// forge/store - the layout forge-factory sync populates.
func StoreDir() string {
	if env := os.Getenv("FORGE_STORE_DIR"); env != "" {
		return env
	}

	cache, err := os.UserCacheDir()
	if err != nil {
		return ""
	}

	return filepath.Join(cache, "forge", "store")
}

// DefaultStoreLookup consults the companion revision's provisioned view: a
// released binary stamped with CompanionRevision finds its whole toolchain
// in the store without PATH or network. Dev builds carry no revision and
// answer nothing.
func DefaultStoreLookup(name string) (string, bool) {
	if CompanionRevision == "" {
		return "", false
	}

	store := StoreDir()
	if store == "" {
		return "", false
	}

	path := filepath.Join(store, "rev", CompanionRevision,
		runtime.GOOS+"-"+runtime.GOARCH, "bin", name)

	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		return path, true
	}

	return "", false
}

// DepVersion answers the version the running binary was built against for
// the module owning modulePath, from build info. It answers "" when the
// binary records no dependency on it - a workspace build, or a module
// outside the dependency graph - so a caller can fall through to its own
// pin.
func DepVersion(modulePath string) string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}

	for _, dep := range info.Deps {
		if dep == nil {
			continue
		}

		if modulePath != dep.Path && !strings.HasPrefix(modulePath, dep.Path+"/") {
			continue
		}

		m := dep
		if m.Replace != nil {
			m = m.Replace
		}

		if m.Version == "" || m.Version == "(devel)" {
			continue
		}

		return m.Version
	}

	return ""
}
