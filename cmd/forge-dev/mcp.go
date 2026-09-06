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
	"context"

	"github.com/alexandremahdhaoui/forge/pkg/enginedocs"
	"github.com/alexandremahdhaoui/forge/pkg/engineframework"
	"github.com/alexandremahdhaoui/forge/pkg/forge"
	"github.com/alexandremahdhaoui/forge/pkg/mcpserver"
	"github.com/alexandremahdhaoui/forge/pkg/mcptypes"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// runMCPServer creates and runs the MCP server for forge-dev.
// It registers the build tool using engineframework.RegisterBuilderTools
// and the config-validate tool for validating forge-dev.yaml and spec.openapi.yaml.
func runMCPServer() error {
	server := mcpserver.New(Name, Version)

	// Register builder tools (build, buildBatch). forge-dev generates code
	// on the machine it runs on and declares no platform: the framework
	// refuses anything but the host before generate runs.
	config := engineframework.BuilderConfig{
		Name:    Name,
		Version: Version,
		BuildFunc: func(ctx context.Context, input mcptypes.BuildInput) ([]forge.Artifact, error) {
			artifact, err := generate(ctx, input)
			if err != nil {
				return nil, err
			}

			return []forge.Artifact{*artifact}, nil
		},
	}

	if err := engineframework.RegisterBuilderTools(server, config); err != nil {
		return err
	}

	// Register docs tools
	if err := enginedocs.RegisterDocsTools(server, *docsConfig); err != nil {
		return err
	}

	// Register config-validate tool
	mcpserver.RegisterTool(server, &mcp.Tool{
		Name:        "config-validate",
		Description: "Validate forge-dev engine scaffolding configuration. Checks forge-dev.yaml structure and spec.openapi.yaml schema definitions for code generation correctness.",
	}, handleConfigValidate)

	return server.RunDefault()
}
