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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/alexandremahdhaoui/forge/internal/engineresolver"
	"github.com/alexandremahdhaoui/forge/internal/forgepath"
	"github.com/alexandremahdhaoui/forge/internal/runnerexec"
	"github.com/alexandremahdhaoui/forge/pkg/forge"
	"github.com/alexandremahdhaoui/forge/pkg/toolresolver"
)

const materializedEnvVar = "FORGE_RUN_MATERIALIZED"

func runRun(args []string) error {
	target, extra := splitRunArgs(args)
	if target == "" {
		return fmt.Errorf("run needs a target: a runnable name, a ./path, or a module path")
	}

	if os.Getenv(materializedEnvVar) == "" {
		if forgepath.IsExternalModule(target) || targetNeedsContext(target) {
			return delegateToForgeFactory(append([]string{"run"}, args...))
		}
	}

	return runLocalTarget(target, extra)
}

func runClone(args []string) error {
	return delegateToForgeFactory(append([]string{"bootstrap"}, args...))
}

func splitRunArgs(args []string) (string, []string) {
	for i, a := range args {
		if a == "--" {
			if i == 0 {
				return "", args[1:]
			}

			return args[0], args[i+1:]
		}
	}

	if len(args) == 0 {
		return "", nil
	}

	return args[0], args[1:]
}

// targetNeedsContext decides whether forge-factory must materialise a
// dependency context first. A declared run target always does: the context
// rules fire and one stderr line says which one. A bare build artifact is
// the zero-config case and runs in place; so does an unknown target, whose
// local error names what exists.
func targetNeedsContext(target string) bool {
	spec, err := forge.ReadSpec()
	if err != nil {
		return true
	}

	runSpec, _, err := resolveTarget(spec, target)
	if err != nil {
		return !strings.HasPrefix(target, "./") && !strings.HasPrefix(target, "../")
	}

	return runSpec != nil
}

func runLocalTarget(target string, extra []string) error {
	spec, err := forge.ReadSpec()
	if err != nil {
		return err
	}

	runSpec, buildSpec, err := resolveTarget(spec, target)
	if err != nil {
		return err
	}

	if err := runBuild([]string{buildSpec.Name}, false, false); err != nil {
		return fmt.Errorf("building %s: %w", buildSpec.Name, err)
	}

	location, err := artifactLocation(spec, buildSpec.Name)
	if err != nil {
		if runSpec == nil || runSpec.Engine == "" {
			return err
		}

		location = buildSpec.Src
	}

	if runSpec == nil || runSpec.Engine == "" {
		return execAttached(location, extra, nil)
	}

	return execThroughRunner(*runSpec, location, extra)
}

func resolveTarget(spec forge.Spec, target string) (*forge.RunSpec, *forge.BuildSpec, error) {
	isPath := strings.HasPrefix(target, "./") || strings.HasPrefix(target, "../")

	for i, r := range spec.Run {
		if (!isPath && r.Name == target) || (isPath && path.Clean(r.Src) == path.Clean(target)) {
			b, err := buildSpecFor(spec, r)
			if err != nil {
				return nil, nil, err
			}

			return &spec.Run[i], b, nil
		}
	}

	for i, b := range spec.Build {
		if (isPath && path.Clean(b.Src) == path.Clean(target)) || (!isPath && b.Name == target) {
			if b.Dest == "" {
				return nil, nil, fmt.Errorf("artifact %s has no dest, so nothing is runnable; declare a run target", b.Name)
			}

			return nil, &spec.Build[i], nil
		}
	}

	return nil, nil, fmt.Errorf("no runnable or artifact matches %q; declared run targets: %s; artifacts: %s",
		target, names(runNames(spec)), names(buildNames(spec)))
}

// buildSpecFor picks the artifact a run target builds: an exact name wins,
// then the src match that produces a binary, then any src match. Several
// artifacts can share a src when one generates and another compiles.
func buildSpecFor(spec forge.Spec, r forge.RunSpec) (*forge.BuildSpec, error) {
	for i, b := range spec.Build {
		if b.Name == r.Name {
			return &spec.Build[i], nil
		}
	}

	for i, b := range spec.Build {
		if path.Clean(b.Src) == path.Clean(r.Src) && b.Dest != "" {
			return &spec.Build[i], nil
		}
	}

	for i, b := range spec.Build {
		if path.Clean(b.Src) == path.Clean(r.Src) {
			return &spec.Build[i], nil
		}
	}

	return nil, fmt.Errorf("run target %s names src %s but no build artifact builds it", r.Name, r.Src)
}

func runNames(spec forge.Spec) []string {
	out := make([]string, 0, len(spec.Run))
	for _, r := range spec.Run {
		out = append(out, r.Name)
	}

	return out
}

func buildNames(spec forge.Spec) []string {
	out := make([]string, 0, len(spec.Build))
	for _, b := range spec.Build {
		out = append(out, b.Name)
	}

	return out
}

func names(list []string) string {
	if len(list) == 0 {
		return "none"
	}

	return strings.Join(list, ", ")
}

func artifactLocation(spec forge.Spec, name string) (string, error) {
	store, err := forge.ReadArtifactStore(spec.ArtifactStorePath)
	if err != nil {
		return "", fmt.Errorf("reading the artifact store: %w", err)
	}

	// A run executes what was built for this machine.
	artifact, err := forge.GetLatestArtifact(store, name, hostPlatform())
	if err != nil {
		return "", fmt.Errorf("locating artifact %s: %w", name, err)
	}

	return strings.TrimPrefix(artifact.Location, "file://"), nil
}

// execThroughRunner runs the target through its run engine. The built-in
// generic-runner runs in process: go run swallows exit codes and prints its
// own noise, and a runner's whole contract is passing both through untouched.
func execThroughRunner(runSpec forge.RunSpec, location string, extra []string) error {
	execSpec := runnerexec.Spec{Artifact: location, Extra: extra, Spec: runSpec.Spec}

	if runSpec.Engine == "forge://generic-runner" {
		code, err := runnerexec.Run(execSpec)
		if err != nil {
			return err
		}

		os.Exit(code)
	}

	engineType, inv, err := engineresolver.ParseEngineURI(runSpec.Engine, getVersion())
	if err != nil {
		return err
	}
	command, engineArgs := inv.Command, inv.Args

	if engineType != engineresolver.EngineTypeMCP {
		return fmt.Errorf("run target %s: engine %s must be a forge:// URI", runSpec.Name, runSpec.Engine)
	}

	payload, err := json.Marshal(execSpec)
	if err != nil {
		return fmt.Errorf("encoding the runner spec: %w", err)
	}

	args := append(append([]string{}, engineArgs...), "exec", string(payload))

	return execAttached(command, args, nil)
}

func execAttached(command string, args []string, env []string) error {
	cmd := exec.Command(command, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), env...)

	err := cmd.Run()
	if err == nil {
		return nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		os.Exit(exitErr.ExitCode())
	}

	return fmt.Errorf("running %s: %w", command, err)
}

// delegateToForgeFactory hands the verb to the companion forge-factory
// through the shared resolution rule: the workspace checkout in local mode,
// then PATH, then go run at the pinned companion - never latest, so a cold
// machine gets the forge-factory this forge was proved with.
func delegateToForgeFactory(args []string) error {
	inv, err := toolresolver.ForgeFactory()
	if err != nil {
		return fmt.Errorf("delegating to forge-factory: %w", err)
	}

	return execAttached(inv.Path, append(append([]string{}, inv.Args...), args...), nil)
}

// delegate answers whether verb names a companion binary and, when it
// does, hands it the rest of the argv untouched.
func delegate(verb string, rest []string) (error, bool) {
	tool, args, ok := delegation(verb, rest)
	if !ok {
		return nil, false
	}

	if tool.name == "forge-factory" {
		return delegateToForgeFactory(args), true
	}

	return delegateToTool(tool.name, tool.module, args), true
}

// companion names the binary a verb belongs to.
type companion struct{ name, module string }

// delegation answers which companion owns a verb and the argv it gets. It is
// separate from the exec so the argv rule can be stated in a test: "cache"
// is forge-factory's own verb under forge's name, so it travels with the
// rest - dropping it turns "forge cache clean" into a bare
// "forge-factory clean", which is not a command.
func delegation(verb string, rest []string) (companion, []string, bool) {
	factory := companion{name: "forge-factory", module: toolresolver.ForgeFactoryModule}

	switch verb {
	case "factory":
		return factory, rest, true
	case "cache":
		return factory, append([]string{"cache"}, rest...), true
	case "ci":
		return companion{
			name:   "forge-ci",
			module: "github.com/alexandremahdhaoui/forge-ci/cmd/forge-ci",
		}, rest, true
	case "register":
		return companion{
			name:   "forge-register",
			module: "github.com/alexandremahdhaoui/forge-register/cmd/forge-register",
		}, rest, true
	}

	return companion{}, nil, false
}

// delegateToTool hands the invocation to a companion binary through the
// shared resolution rule: workspace checkout, then the pinned store, then
// PATH (including the enclosing workspace's .forge/bin). These tools carry
// no baked version pin on purpose - the register is the pin - so a machine
// with nothing provisioned gets told what provisions it, not a go run at
// some guessed version.
func delegateToTool(name, module string, args []string) error {
	r := toolresolver.Resolver{StoreLookup: toolresolver.DefaultStoreLookup}

	inv, err := r.Resolve(toolresolver.Ref{Name: name, Module: module})
	if err != nil {
		return fmt.Errorf(
			"%s is not provisioned: run `forge factory sync` (or `forge-factory sync`) in your workspace, or install it",
			name)
	}

	return execAttached(inv.Path, append(append([]string{}, inv.Args...), args...), nil)
}
