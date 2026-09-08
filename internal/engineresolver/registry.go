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

package engineresolver

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/alexandremahdhaoui/forge/internal/forgepath"
	"github.com/alexandremahdhaoui/forge/pkg/forge"

	"sigs.k8s.io/yaml"
)

// FactoryEnginesPath is the file forge-factory sync writes at the factory
// root: the factory's engines by alias, the outer ring of the registry.
const FactoryEnginesPath = ".forge/engines.yaml"

// Registry is what forge://<short-name> consults before any fallback. The
// inner ring is the repo's own forge.yaml engines: entries that carry an
// engine; the outer ring is the enclosing factory's .forge/engines.yaml;
// and a name in neither resolves to forge's own module, as it always did.
// A customer with their own engines changes one entry, in one file.
type Registry struct {
	inner    map[string]string
	innerDir string
	outer    map[string]string
	outerDir string
}

// Lookup answers the target an alias names and the directory a relative
// path target resolves from.
func (r Registry) Lookup(alias string) (target, dir string, ok bool) {
	if t, ok := r.inner[alias]; ok {
		return t, r.innerDir, true
	}

	if t, ok := r.outer[alias]; ok {
		return t, r.outerDir, true
	}

	return "", "", false
}

// factoryEngines is the shape of .forge/engines.yaml.
type factoryEngines struct {
	Engines []struct {
		Alias  string `json:"alias"`
		Engine string `json:"engine"`
	} `json:"engines"`
}

// Load builds the registry for a repo whose forge.yaml lives in specDir.
// The factory root is found by walking up to the first forge-factory.yaml;
// none is a lone checkout with no outer ring.
func Load(spec *forge.Spec, specDir string) (Registry, error) {
	r := Registry{innerDir: specDir}

	if spec != nil {
		r.inner = spec.Registry()
	}

	root, ok := factoryRoot(specDir)
	if !ok {
		return r, nil
	}

	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(FactoryEnginesPath)))
	if err != nil {
		if os.IsNotExist(err) {
			return r, nil
		}

		return Registry{}, fmt.Errorf("reading %s: %w", FactoryEnginesPath, err)
	}

	var doc factoryEngines
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return Registry{}, fmt.Errorf("reading %s: %w", FactoryEnginesPath, err)
	}

	r.outer = map[string]string{}
	r.outerDir = root

	for _, e := range doc.Engines {
		r.outer[e.Alias] = e.Engine
	}

	return r, nil
}

func factoryRoot(from string) (string, bool) {
	dir := from

	for {
		if info, err := os.Stat(filepath.Join(dir, "forge-factory.yaml")); err == nil && !info.IsDir() {
			return dir, true
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}

		dir = parent
	}
}

var (
	currentMu sync.RWMutex
	current   Registry
)

// Use installs the registry every resolution consults. A binary loads it
// once at startup from the repo it runs in; tests install their own.
func Use(r Registry) {
	currentMu.Lock()
	defer currentMu.Unlock()

	current = r
}

func registry() Registry {
	currentMu.RLock()
	defer currentMu.RUnlock()

	return current
}

// resolveRegistered answers the invocation for a registry target. A path
// is built from source in place and run as a binary, so the engine keeps
// the caller's working directory; a forge:// target resolves like any
// other URI, except that it may not itself be a short name, which would
// be a registry naming the registry.
func resolveRegistered(alias, target, dir, forgeVersion string) (Invocation, error) {
	if forge.IsEnginePath(target) {
		pkgDir := target
		if !filepath.IsAbs(pkgDir) {
			pkgDir = filepath.Join(dir, pkgDir)
		}

		bin, err := forgepath.BuildEngineFromSource(pkgDir, alias)
		if err != nil {
			return Invocation{}, fmt.Errorf("engine %s registered at %s: %w", alias, target, err)
		}

		return Invocation{Command: bin}, nil
	}

	bare, _, _ := cutVersion(target)
	if !forgepath.IsExternalModule(bare) {
		return Invocation{}, fmt.Errorf("engine %s is registered as %s, which is another short name; a registry entry names a module path or a source directory", alias, target)
	}

	return ResolveForgeURI(target, forgeVersion)
}

func cutVersion(uri string) (bare, version string, found bool) {
	path := uri
	if len(path) > len("forge://") && path[:len("forge://")] == "forge://" {
		path = path[len("forge://"):]
	}

	for i := 0; i < len(path); i++ {
		if path[i] == '@' {
			return path[:i], path[i+1:], true
		}
	}

	return path, "", false
}
