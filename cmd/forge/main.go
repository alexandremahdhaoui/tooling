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
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/alexandremahdhaoui/forge/internal/cmdutil"
	"github.com/alexandremahdhaoui/forge/pkg/engineversion"
	"github.com/alexandremahdhaoui/forge/pkg/toolresolver"
)

// Version information (set via ldflags during build)
var (
	Version        = "dev"
	CommitSHA      = "unknown"
	BuildTimestamp = "unknown"
)

// versionInfo holds forge's version information
var versionInfo *engineversion.Info

// configPath is the path to the forge.yaml file
var configPath string

// cwdOverride is set by --cwd flag to change working directory before command execution
var cwdOverride string

// skipWorkspaceResolution is set by --skip-workspace-resolution flag to disable
// automatic Go workspace detection
var skipWorkspaceResolution bool

func init() {
	versionInfo = engineversion.New("forge")
	versionInfo.Version = Version
	versionInfo.CommitSHA = CommitSHA
	versionInfo.BuildTimestamp = BuildTimestamp
}

// getVersion returns the actual forge version, using build info if available
func getVersion() string {
	v, _, _ := versionInfo.Get()
	return v
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// Global flags that come BEFORE the verb are forge's, and --cwd among
	// them decides which workspace everything after this reads. Applying it
	// after the workspace-bin lookup picked up the wrong .forge/bin, and
	// running delegation before it made "forge --cwd x ci ..." an unknown
	// command rather than a delegation.
	//
	// Only the leading run is parsed. Everything from the verb on belongs to
	// whoever owns the verb: a --config after "ci" is forge-ci's, not ours.
	verb, rest := splitLeadingFlags(os.Args[1:])

	if verb == "" {
		printUsage()
		os.Exit(1)
	}

	applyCwdOverride()

	// The enclosing workspace's pinned tooling joins PATH before anything
	// else, so delegation and engines resolve provisioned binaries with no
	// .envrc sourced. After --cwd, so it is the named workspace's.
	if wd, err := os.Getwd(); err == nil {
		toolresolver.EnsureWorkspaceBin(wd)
	}

	if err, handled := delegate(verb, rest); handled {
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		return
	}

	// The rest of the argv, with any remaining global flags taken out.
	args := parseGlobalFlags(append([]string{verb}, rest...))

	if len(args) < 1 {
		printUsage()
		os.Exit(1)
	}

	applyCwdOverride()

	// Resolve Go workspace if present
	if err := resolveWorkspace(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: workspace resolution failed: %v\n", err)
		// Non-fatal: continue without workspace resolution
	}

	// Change to the directory containing forge.yaml if --config specifies a path with directory components.
	// This ensures all relative paths in forge.yaml resolve correctly.
	if err := changeToProjectDir(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// The registry every forge://<short-name> consults: this repo's own
	// entries, then the enclosing factory's. Outside a repo there is none,
	// and every name falls through to forge's own module.
	if err := installEngineRegistry(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Source global environment file if configured
	// TODO: This calls loadConfig() which is also called in command handlers.
	// This is intentional to avoid refactoring all handlers, but is technical debt.
	// The double-call is acceptable as YAML parsing is fast (<1ms).
	config, err := loadConfig()
	if err == nil && config.EnvFile != "" {
		if err := cmdutil.SourceEnvFile(config.EnvFile); err != nil {
			fmt.Fprintf(os.Stderr, "Error sourcing envFile: %v\n", err)
			os.Exit(1)
		}
	}
	// Note: We don't fail if loadConfig errors here - let the command handler report it

	command := args[0]
	cmdArgs := args[1:]

	switch command {
	case "--mcp":
		// Run in MCP server mode
		if err := runMCPServer(); err != nil {
			log.Printf("MCP server error: %v", err)
			os.Exit(1)
		}
	case "build":
		// No flag decides anything here: what builds is declared, how
		// strictly is declared, and whether a build is needed is the digest
		// of what it was built from.
		if err := runBuild(cmdArgs); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "test":
		if err := runTest(cmdArgs); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "test-all":
		if err := runTestAll(cmdArgs); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "run":
		if err := runRun(cmdArgs); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "clone":
		if err := runClone(cmdArgs); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "docs":
		if err := runDocs(cmdArgs); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "config":
		if err := runConfig(cmdArgs); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "cu":
		if err := runCU(cmdArgs); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "ws", "workspace":
		if err := runWS(cmdArgs); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "ui":
		if err := runUI(cmdArgs); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "list":
		if err := runList(cmdArgs); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "version", "--version", "-v":
		versionInfo.Print()
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

// changeToProjectDir changes the working directory to the directory containing
// the config file when --config specifies a path with directory components.
// This ensures all relative paths in forge.yaml resolve correctly when forge
// is invoked from a parent directory (e.g. a Go workspace root).
func changeToProjectDir() error {
	if configPath == "" {
		return nil
	}
	dir := filepath.Dir(configPath)
	if dir == "." {
		return nil
	}
	if err := os.Chdir(dir); err != nil {
		return fmt.Errorf("cannot change to project directory %q: %w", dir, err)
	}
	configPath = filepath.Base(configPath)
	fmt.Fprintf(os.Stderr, "forge: changed working directory to %s\n", dir)
	return nil
}

// parseGlobalFlags parses global flags like --config and returns remaining args
// splitLeadingFlags takes forge's own global flags off the front of the argv
// and answers the verb plus everything after it, untouched. A flag after the
// verb is not read here: it belongs to whoever owns the verb.
func splitLeadingFlags(args []string) (string, []string) {
	for i := 0; i < len(args); i++ {
		arg := args[i]

		if !strings.HasPrefix(arg, "-") {
			return arg, args[i+1:]
		}

		switch {
		case arg == "--config" || arg == "--cwd":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "Error: %s requires a path argument\n", arg)
				os.Exit(1)
			}

			set(arg, args[i+1])

			i++
		case strings.HasPrefix(arg, "--config=") || strings.HasPrefix(arg, "--cwd="):
			name, value, _ := strings.Cut(arg, "=")
			if value == "" {
				fmt.Fprintf(os.Stderr, "Error: %s requires a path argument\n", name)
				os.Exit(1)
			}

			set(name, value)
		case arg == "--skip-workspace-resolution":
			skipWorkspaceResolution = true
		default:
			// Not one of ours. It is the verb's problem, and there is no
			// verb yet, so let the usual parser report it.
			return arg, args[i+1:]
		}
	}

	return "", nil
}

func set(name, value string) {
	if name == "--cwd" {
		cwdOverride = value

		return
	}

	configPath = value
}

// applyCwdOverride moves to the named directory before anything reads the
// working directory. Everything downstream - the workspace bin, workspace
// resolution, config discovery - answers about the workspace the user named.
func applyCwdOverride() {
	if cwdOverride == "" {
		return
	}

	absPath, err := filepath.Abs(cwdOverride)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot resolve --cwd path %q: %v\n", cwdOverride, err)
		os.Exit(1)
	}

	if err := os.Chdir(absPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot change to --cwd directory %q: %v\n", absPath, err)
		os.Exit(1)
	}

	// Applied once. parseGlobalFlags runs again over the rest of the argv
	// and must not chdir a second time from a now-relative path.
	cwdOverride = ""
}

func parseGlobalFlags(args []string) []string {
	result := make([]string, 0, len(args))

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if arg == "--config" {
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "Error: --config requires a path argument\n")
				os.Exit(1)
			}
			configPath = args[i+1]
			i++ // Skip the next argument (the path)
		} else if val, ok := strings.CutPrefix(arg, "--config="); ok {
			configPath = val
			if configPath == "" {
				fmt.Fprintf(os.Stderr, "Error: --config requires a path argument\n")
				os.Exit(1)
			}
		} else if arg == "--cwd" {
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "Error: --cwd requires a path argument\n")
				os.Exit(1)
			}
			cwdOverride = args[i+1]
			i++ // Skip the next argument (the path)
		} else if val, ok := strings.CutPrefix(arg, "--cwd="); ok {
			cwdOverride = val
			if cwdOverride == "" {
				fmt.Fprintf(os.Stderr, "Error: --cwd requires a path argument\n")
				os.Exit(1)
			}
		} else if arg == "--skip-workspace-resolution" {
			skipWorkspaceResolution = true
		} else {
			result = append(result, arg)
		}
	}

	return result
}

func printUsage() {
	fmt.Println(`forge - build, test and run through one front door

Usage:
  forge [global flags] <command> [args...]

Build and test (this repo's forge.yaml):
  build [artifact]                Build every artifact, or one; skips what its digests say is fresh
  test run <stage>                Run one test stage
  test-all                        Build everything, then every stage, fail-fast
  list [build|test]               Show what this repo can build and test

Run:
  run <target> [-- args...]       Run a declared target: a name, a ./path, or a module path
  clone <factory-url> [dir]       Stand up a whole workspace from its factory, from nothing

Toolchain (delegated; resolved pinned - checkout, then store, then workspace bin):
  factory <args...>               The workspace tool: sync, status, add, bump ...
  ci <args...>                    The pipeline tool: apply, bootstrap ...
  register <args...>              The package register tool: status, add, apply ...
  cache clean                     Remove the derived run cache; the next run rebuilds it

More:
  test <verb> <stage> [args]      Reports and environments: list, get, delete,
                                  list-env, get-env, create-env, delete-env
  docs <list|get> [name]          Read this project's documentation
  config validate [path]          Check a forge.yaml without running anything
  cu <verb>                       Continuous update: status, commit, checkout,
                                  list-branches, go-get
  ws <verb>                       Remote workspaces: list, create, get, delete,
                                  suspend, resume
  ui                              The terminal dashboard
  version                         What forge this is
  help                            This message

Global flags:
  --config <path>                 Use this forge.yaml instead of ./forge.yaml
  --cwd <path>                    Switch to this directory first
  --skip-workspace-resolution     Ignore any enclosing Go workspace`)
}
