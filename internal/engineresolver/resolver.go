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

// Package engineresolver provides shared engine URI parsing functionality.
// This package handles the parsing of engine URIs (forge://, alias://) and
// returns the engine type, command, and args for execution.
//
// IMPORTANT: This package ONLY parses URIs. For alias:// URIs, it returns
// EngineTypeAlias - the caller must handle alias resolution separately.
package engineresolver

import (
	"fmt"
	"strings"

	"github.com/alexandremahdhaoui/forge/internal/forgepath"
	"github.com/alexandremahdhaoui/forge/pkg/toolresolver"
)

const (
	// EngineTypeMCP indicates a forge:// URI that should be executed as an MCP server.
	EngineTypeMCP = "mcp"
	// EngineTypeAlias indicates an alias:// URI that requires resolution by the caller.
	EngineTypeAlias = "alias"
)

// GoSchemeError is the message a go:// URI fails with. The scheme is removed,
// not aliased, so every stale file fails loudly and identically.
const GoSchemeError = "the go:// scheme is removed; use forge:// " +
	"(forge://<name> for forge's own engines, " +
	"forge://<module-path>[@rev] resolves a factory member through its register)"

// ParseEngineURI parses an engine URI and returns the engine type, command, and args.
// Supports forge:// and alias:// protocols:
//   - forge://go-build -> forge's own engine at the running forge version,
//     executed via `go run github.com/alexandremahdhaoui/forge/cmd/go-build@{forgeVersion}`
//   - forge://github.com/x/repo/cmd/tool -> a factory member, materialized and
//     executed via `forge-factory run github.com/x/repo/cmd/tool -- --mcp`;
//     the version comes from the member's register unless an @rev is given
//   - alias://my-engine -> returns EngineTypeAlias with aliasName - caller must resolve
//
// Returns:
//   - engineType: EngineTypeMCP for forge:// URIs, EngineTypeAlias for alias:// URIs
//   - command: "go" or "forge-factory" for forge:// URIs, aliasName for alias:// URIs
//   - args: the command's arguments, nil for alias:// URIs
//   - err: error if parsing fails
func ParseEngineURI(engineURI, forgeVersion string) (engineType string, inv Invocation, err error) {
	if strings.HasPrefix(engineURI, "alias://") {
		aliasName := strings.TrimPrefix(engineURI, "alias://")
		if aliasName == "" {
			return "", Invocation{}, fmt.Errorf("empty alias name after alias://")
		}

		return EngineTypeAlias, Invocation{Command: aliasName}, nil
	}

	if strings.HasPrefix(engineURI, "go://") {
		return "", Invocation{}, fmt.Errorf("%s: %s", engineURI, GoSchemeError)
	}

	inv, err = ResolveForgeURI(engineURI, forgeVersion)
	if err != nil {
		return "", Invocation{}, err
	}

	return EngineTypeMCP, inv, nil
}

type Invocation struct {
	Command string
	Args    []string
	Dir     string
}

func ResolveForgeURI(engineURI, forgeVersion string) (Invocation, error) {
	if !strings.HasPrefix(engineURI, "forge://") {
		return Invocation{}, fmt.Errorf("unsupported engine protocol: %s (must start with forge:// or alias://)", engineURI)
	}

	path := strings.TrimPrefix(engineURI, "forge://")
	if path == "" {
		return Invocation{}, fmt.Errorf("empty engine path after forge://")
	}

	bare, _, _ := strings.Cut(path, "@")
	if forgepath.IsExternalModule(bare) && !forgepath.IsForgeModulePath(bare) {
		return resolveMember(engineURI, path, bare)
	}

	// A short name asks the registry first: the repo's own entries, then
	// the factory's, then forge's own module.
	if !forgepath.IsExternalModule(bare) {
		if target, dir, ok := registry().Lookup(bare); ok {
			return resolveRegistered(bare, target, dir, forgeVersion)
		}
	}

	return resolveBuiltin(engineURI, bare, forgeVersion)
}

func resolveMember(engineURI, path, bare string) (Invocation, error) {
	if forgepath.IsWorkspaceModule(bare) {
		return Invocation{Command: "go", Args: []string{"run", bare}}, nil
	}

	factory, err := toolresolver.ForgeFactory()
	if err != nil {
		return Invocation{}, fmt.Errorf("resolving forge-factory for %s: %w", engineURI, err)
	}

	args := append(append([]string{}, factory.Args...), "run", "--quiet", path, "--", "--mcp")

	return Invocation{Command: factory.Path, Args: args}, nil
}

func resolveBuiltin(engineURI, bare, forgeVersion string) (Invocation, error) {
	packageName := bare
	if idx := strings.LastIndex(bare, "/"); idx != -1 {
		packageName = bare[idx+1:]
	}

	if packageName == "" {
		return Invocation{}, fmt.Errorf("could not extract package name from engine URI: %s", engineURI)
	}

	command, args, err := forgepath.EngineCommand(packageName, forgeVersion)
	if err != nil {
		return Invocation{}, fmt.Errorf("failed to resolve engine command for %s: %w", packageName, err)
	}

	return Invocation{Command: command, Args: args, Dir: forgeModuleDirForGoRun(command, args)}, nil
}

func forgeModuleDirForGoRun(command string, args []string) string {
	if command != "go" || len(args) == 0 || args[0] != "run" {
		return ""
	}

	if forgepath.IsForgeWorkspaceMember() {
		return ""
	}

	forgeRepo, err := forgepath.FindForgeRepo()
	if err != nil {
		return ""
	}

	return forgeRepo
}
