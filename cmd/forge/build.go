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
func runBuild(args []string, forceRebuild, frozenBuild bool) error {
	var artifactName string

	rest, platforms, err := parsePlatformsFlag(args)
	if err != nil {
		return err
	}

	buildPlatforms = platforms

	if len(rest) > 0 {
		artifactName = rest[0]
	}

	result, err := buildAll(artifactName, forceRebuild, frozenBuild)
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

// parsePlatformsFlag pulls --platforms off the argv. It narrows a build to
// these os/arch pairs, comma separated:
//
//	forge build --platforms linux/amd64,linux/arm64
//
// An entry builds every platform its build entry declares, the host when it
// declares none; the flag selects a subset of the DECLARED platforms and
// never widens it, so an entry that declares none of the named platforms is
// skipped - and an entry that declares no platform at all is skipped by any
// selection, host or not, so a repo's own tools never travel by accident.
func parsePlatformsFlag(args []string) ([]string, []string, error) {
	rest := make([]string, 0, len(args))

	var platforms []string

	for i := 0; i < len(args); i++ {
		arg := args[i]

		value := ""

		switch {
		case arg == "--platforms":
			if i+1 >= len(args) {
				return nil, nil, fmt.Errorf("--platforms needs a list, e.g. --platforms linux/amd64,linux/arm64")
			}

			i++
			value = args[i]
		case strings.HasPrefix(arg, "--platforms="):
			value = strings.TrimPrefix(arg, "--platforms=")
		default:
			rest = append(rest, arg)

			continue
		}

		for _, platform := range strings.Split(value, ",") {
			if trimmed := strings.TrimSpace(platform); trimmed != "" {
				platforms = append(platforms, trimmed)
			}
		}

		if len(platforms) == 0 {
			return nil, nil, fmt.Errorf("--platforms needs a list, e.g. --platforms linux/amd64,linux/arm64")
		}
	}

	return rest, platforms, nil
}
