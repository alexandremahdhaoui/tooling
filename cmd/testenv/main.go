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

	"github.com/alexandremahdhaoui/forge/internal/engineresolver"
	"github.com/alexandremahdhaoui/forge/pkg/enginecli"
	"github.com/alexandremahdhaoui/forge/pkg/enginedocs"
	"github.com/alexandremahdhaoui/forge/pkg/engineversion"
	"github.com/alexandremahdhaoui/forge/pkg/forge"
)

// Version information (set via ldflags during build)
var (
	Version        = "dev"
	CommitSHA      = "unknown"
	BuildTimestamp = "unknown"
)

// versionInfo holds testenv's version information
var versionInfo *engineversion.Info

// docsConfig is the configuration for the docs subcommand.
var docsConfig = &enginedocs.Config{
	EngineName:   "testenv",
	LocalDir:     "cmd/testenv/docs",
	BaseURL:      "https://raw.githubusercontent.com/alexandremahdhaoui/forge/refs/heads/main",
	RequiredDocs: []string{"usage", "schema"},
}

func init() {
	versionInfo = engineversion.New("testenv")
	versionInfo.Version = Version
	versionInfo.CommitSHA = CommitSHA
	versionInfo.BuildTimestamp = BuildTimestamp
}

// getVersion returns the actual testenv version, using build info if available
func getVersion() string {
	v, _, _ := versionInfo.Get()
	return v
}

func main() {
	// The orchestrator resolves its members' engines the way forge does:
	// through the registry of the repo it runs in and the factory above.
	installEngineRegistry()

	// Check if running in direct CLI mode (testenv <command>)
	if len(os.Args) >= 2 && os.Args[1] != "--mcp" && os.Args[1] != "version" && os.Args[1] != "--version" && os.Args[1] != "-v" && os.Args[1] != "help" && os.Args[1] != "--help" && os.Args[1] != "-h" {
		command := os.Args[1]

		switch command {
		case "create":
			stageName := ""
			if len(os.Args) >= 3 {
				stageName = os.Args[2]
			}
			if _, err := cmdCreate(stageName); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		case "delete":
			if len(os.Args) < 3 {
				fmt.Fprintf(os.Stderr, "Error: test ID required\n\n")
				printUsage()
				os.Exit(1)
			}
			testID := os.Args[2]
			if err := cmdDelete(testID); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		default:
			fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
			printUsage()
			os.Exit(1)
		}
		return
	}

	// Otherwise, use standard cli.Bootstrap for MCP mode and version handling
	enginecli.Bootstrap(enginecli.Config{
		Name:           "testenv",
		Version:        Version,
		CommitSHA:      CommitSHA,
		BuildTimestamp: BuildTimestamp,
		RunMCP:         runMCPServer,
		DocsConfig:     docsConfig,
	})
}

func printUsage() {
	fmt.Println(`testenv - Orchestrate test environments

Usage:
  testenv create <STAGE>        Create a test environment
  testenv delete <TEST-ID>      Delete a test environment
  testenv --mcp                 Run as MCP server
  testenv version               Show version information

Arguments:
  STAGE     Test stage name (e.g., "integration", "e2e")
  TEST-ID   Test environment ID

Examples:
  testenv create integration
  testenv delete test-integration-20241103-abc123
  testenv --mcp

Note:
  Use 'forge test <stage> get/list' to view test environments.
  testenv only handles create/delete operations.`)
}

// installEngineRegistry loads the registry from the working directory's
// forge.yaml, when there is one, and the factory above it. Failures are
// not fatal here: a name then falls through to forge's own module, and
// the resolution that needed the entry reports it.
func installEngineRegistry() {
	wd, err := os.Getwd()
	if err != nil {
		return
	}

	var spec *forge.Spec

	if loaded, err := forge.ReadSpec(); err == nil {
		spec = &loaded
	}

	registry, err := engineresolver.Load(spec, wd)
	if err != nil {
		log.Printf("testenv: engine registry: %v", err)

		return
	}

	engineresolver.Use(registry)
}
