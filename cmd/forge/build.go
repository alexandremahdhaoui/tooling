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
	"os"
	"strings"
)

// runBuild is the CLI entry point for the build command.
// It calls the shared buildAll function and prints human-readable output.
func runBuild(args []string) error {
	var artifactName string

	for _, arg := range args {
		// A build takes no policy on its command line: what an entry builds
		// is declared in forge.yaml, and how strictly is declared there too.
		if strings.HasPrefix(arg, "-") {
			return fmt.Errorf("unknown flag %q: a build builds what forge.yaml declares; there is no flag to narrow or widen it", arg)
		}
	}

	if len(args) > 0 {
		artifactName = args[0]
	}

	result, err := buildAll(artifactName)
	if err != nil {
		return err
	}

	printBuildResult(result, artifactName)
	return nil
}

// printBuildResult prints human-readable build results to stderr.
// Uses stderr because this function is called from runTestAll which is shared
// between CLI and MCP. Stdout is the JSON-RPC transport in MCP mode.
func printBuildResult(result *BuildAllResult, artifactName string) {
	if result.TotalBuilt > 0 && result.Skipped > 0 {
		fmt.Fprintf(os.Stderr, "✅ Successfully built %d artifact(s), skipped %d unchanged\n", result.TotalBuilt, result.Skipped)
	} else if result.TotalBuilt > 0 {
		fmt.Fprintf(os.Stderr, "✅ Successfully built %d artifact(s)\n", result.TotalBuilt)
	} else if result.Skipped > 0 {
		if artifactName != "" {
			fmt.Fprintf(os.Stderr, "✅ Artifact %s is up to date\n", artifactName)
		} else {
			fmt.Fprintf(os.Stderr, "✅ All %d artifact(s) up to date\n", result.Skipped)
		}
	} else {
		fmt.Fprintln(os.Stderr, "No artifacts to build")
	}
}
