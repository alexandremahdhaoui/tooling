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
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/alexandremahdhaoui/forge/internal/cmdutil"
	"github.com/alexandremahdhaoui/forge/pkg/engineframework"
	"github.com/alexandremahdhaoui/forge/pkg/forge"
	"github.com/alexandremahdhaoui/forge/pkg/mcptypes"
)

// Build is the core business logic for executing a shell command as a build step.
func Build(ctx context.Context, input mcptypes.BuildInput, spec *Spec) ([]forge.Artifact, error) {
	command := spec.Command
	if command == "" {
		command = input.Command
	}

	args := spec.Args
	if len(args) == 0 {
		args = input.Args
	}

	env := spec.Env
	if len(env) == 0 {
		env = input.Env
	}

	envFile := spec.EnvFile
	if envFile == "" {
		envFile = input.EnvFile
	}

	ctxDir := spec.Context
	if ctxDir == "" {
		ctxDir = input.Context
	}

	log.Printf("Executing command: %s %v (context: %s) for %s", command, args, ctxDir, strings.Join(input.Platforms, ", "))

	if command == "" {
		return nil, fmt.Errorf("command is required")
	}

	processedArgs, err := processTemplatedArgs(args, input)
	if err != nil {
		return nil, fmt.Errorf("template processing failed: %w", err)
	}

	host := engineframework.HostPlatform()
	artifacts := make([]forge.Artifact, 0, len(input.Platforms))

	// The command runs once per platform. This engine names no language, so
	// the platform reaches the command as three neutral variables and the
	// command decides what a target means: a compiler reads them, a script
	// that builds nothing platform-specific ignores them.
	for _, platform := range input.Platforms {
		goos, goarch, err := forge.SplitPlatform(platform)
		if err != nil {
			return nil, err
		}

		platformEnv := map[string]string{}
		for k, v := range env {
			platformEnv[k] = v
		}

		platformEnv["FORGE_PLATFORM"] = platform
		platformEnv["FORGE_OS"] = goos
		platformEnv["FORGE_ARCH"] = goarch

		// The repo's frozen setting, for a command that reads locks: the
		// engine declares the capability and the command decides what a
		// frozen build means in its ecosystem.
		platformEnv["FORGE_FROZEN"] = strconv.FormatBool(input.Frozen)

		// Where the command writes what it builds, so the convention that
		// the record below reads is told to the command rather than
		// re-expressed by every forge.yaml in shell: dest/<name> for the
		// host, dest/<name>_<os>_<arch> for a cross build. Absent when the
		// entry declares no dest.
		if input.Dest != "" {
			platformEnv["FORGE_OUT"] = builtPath(input.Dest, input.Name, platform, host, goos, goarch)
		}

		execInput := cmdutil.ExecuteInput{
			Command: command,
			Args:    processedArgs,
			Env:     platformEnv,
			EnvFile: envFile,
			Context: ctxDir,
		}

		output := cmdutil.ExecuteCommand(execInput)

		if output.ExitCode != 0 {
			errorMsg := fmt.Sprintf("command failed for %s with exit code %d", platform, output.ExitCode)
			if output.Error != "" {
				errorMsg += fmt.Sprintf(": %s", output.Error)
			}
			if output.Stderr != "" {
				errorMsg += fmt.Sprintf(" (stderr: %s)", output.Stderr)
			}
			return nil, fmt.Errorf("%s", errorMsg)
		}

		if output.Stdout != "" {
			log.Printf("Stdout: %s", output.Stdout)
		}
		if output.Stderr != "" {
			log.Printf("Stderr: %s", output.Stderr)
		}

		location := ctxDir
		if location == "" {
			location = input.Src
		}
		if location == "" {
			location = "."
		}

		artifact := forge.Artifact{
			Name:      input.Name,
			Type:      forge.TypeCommandOutput,
			Location:  location,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Version:   fmt.Sprintf("%s-exit%d", command, output.ExitCode),
		}

		// A generic build that declares a dest and leaves the named file
		// there produced a real artifact: record the file itself, as a
		// binary for that platform, so the release side can publish it. The
		// host build is dest/<name>; a cross build is dest/<name>_<os>_<arch>,
		// the same convention go-build writes, so one release reads both. A
		// command that wrote nothing keeps the command-output record it
		// always had.
		if input.Dest != "" {
			built := builtPath(input.Dest, input.Name, platform, host, goos, goarch)

			if info, err := os.Stat(built); err == nil && !info.IsDir() {
				artifact.Location = built
				artifact.Type = forge.TypeBinary
				artifact.OS = goos
				artifact.Arch = goarch
			}
		}

		artifacts = append(artifacts, artifact)
	}

	return artifacts, nil
}

func processTemplatedArgs(args []string, data mcptypes.BuildInput) ([]string, error) {
	if len(args) == 0 {
		return args, nil
	}

	result := make([]string, len(args))
	for i, arg := range args {
		tmpl, err := template.New(fmt.Sprintf("arg%d", i)).Parse(arg)
		if err != nil {
			return nil, fmt.Errorf("failed to parse template in arg[%d]: %w", i, err)
		}

		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			return nil, fmt.Errorf("failed to execute template in arg[%d]: %w", i, err)
		}

		result[i] = buf.String()
	}

	return result, nil
}

// builtPath is where a command is told to write, and where the record is
// read from: one function, so the two cannot disagree.
func builtPath(dest, name, platform, host, goos, goarch string) string {
	if platform == host {
		return filepath.Join(dest, name)
	}

	return filepath.Join(dest, fmt.Sprintf("%s_%s_%s", name, goos, goarch))
}
