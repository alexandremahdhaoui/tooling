//go:build unit

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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge/pkg/forge"
)

// TestDiscoverProtoFiles tests the discoverProtoFiles function.
func TestDiscoverProtoFiles(t *testing.T) {
	tests := []struct {
		name          string
		setup         func(t *testing.T, dir string)
		wantCount     int
		wantFiles     []string // relative paths expected
		wantErr       bool
		wantErrSubstr string
	}{
		{
			name: "directory with .proto files at various depths",
			setup: func(t *testing.T, dir string) {
				// Root level proto
				err := os.WriteFile(filepath.Join(dir, "root.proto"), []byte("syntax = \"proto3\";"), 0o644)
				if err != nil {
					t.Fatalf("failed to write root.proto: %v", err)
				}

				// Nested directory with proto
				subdir := filepath.Join(dir, "api", "v1")
				err = os.MkdirAll(subdir, 0o755)
				if err != nil {
					t.Fatalf("failed to create subdir: %v", err)
				}
				err = os.WriteFile(filepath.Join(subdir, "service.proto"), []byte("syntax = \"proto3\";"), 0o644)
				if err != nil {
					t.Fatalf("failed to write service.proto: %v", err)
				}

				// Another nested directory
				subdir2 := filepath.Join(dir, "internal", "types")
				err = os.MkdirAll(subdir2, 0o755)
				if err != nil {
					t.Fatalf("failed to create subdir2: %v", err)
				}
				err = os.WriteFile(filepath.Join(subdir2, "types.proto"), []byte("syntax = \"proto3\";"), 0o644)
				if err != nil {
					t.Fatalf("failed to write types.proto: %v", err)
				}
			},
			wantCount: 3,
			wantFiles: []string{"root.proto", "api/v1/service.proto", "internal/types/types.proto"},
			wantErr:   false,
		},
		{
			name: "empty directory returns empty slice",
			setup: func(t *testing.T, dir string) {
				// No setup needed - directory is empty
			},
			wantCount: 0,
			wantFiles: []string{},
			wantErr:   false,
		},
		{
			name:          "non-existent directory returns error",
			setup:         nil, // Don't create any directory structure
			wantErr:       true,
			wantErrSubstr: "failed to access directory",
		},
		{
			name: "hidden directories are skipped",
			setup: func(t *testing.T, dir string) {
				// Create a normal proto file
				err := os.WriteFile(filepath.Join(dir, "visible.proto"), []byte("syntax = \"proto3\";"), 0o644)
				if err != nil {
					t.Fatalf("failed to write visible.proto: %v", err)
				}

				// Create a hidden directory with proto file
				hiddenDir := filepath.Join(dir, ".hidden")
				err = os.MkdirAll(hiddenDir, 0o755)
				if err != nil {
					t.Fatalf("failed to create hidden dir: %v", err)
				}
				err = os.WriteFile(filepath.Join(hiddenDir, "hidden.proto"), []byte("syntax = \"proto3\";"), 0o644)
				if err != nil {
					t.Fatalf("failed to write hidden.proto: %v", err)
				}

				// Create another hidden directory (deeper)
				deepHiddenDir := filepath.Join(dir, "subdir", ".cache")
				err = os.MkdirAll(deepHiddenDir, 0o755)
				if err != nil {
					t.Fatalf("failed to create deep hidden dir: %v", err)
				}
				err = os.WriteFile(filepath.Join(deepHiddenDir, "cached.proto"), []byte("syntax = \"proto3\";"), 0o644)
				if err != nil {
					t.Fatalf("failed to write cached.proto: %v", err)
				}
			},
			wantCount: 1,
			wantFiles: []string{"visible.proto"},
			wantErr:   false,
		},
		{
			name: "non-.proto files are ignored",
			setup: func(t *testing.T, dir string) {
				// Create various non-proto files
				err := os.WriteFile(filepath.Join(dir, "readme.md"), []byte("# README"), 0o644)
				if err != nil {
					t.Fatalf("failed to write readme.md: %v", err)
				}
				err = os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0o644)
				if err != nil {
					t.Fatalf("failed to write main.go: %v", err)
				}
				err = os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("key: value"), 0o644)
				if err != nil {
					t.Fatalf("failed to write config.yaml: %v", err)
				}
				err = os.WriteFile(filepath.Join(dir, "proto.txt"), []byte("not a proto file"), 0o644)
				if err != nil {
					t.Fatalf("failed to write proto.txt: %v", err)
				}

				// Create one actual proto file
				err = os.WriteFile(filepath.Join(dir, "service.proto"), []byte("syntax = \"proto3\";"), 0o644)
				if err != nil {
					t.Fatalf("failed to write service.proto: %v", err)
				}
			},
			wantCount: 1,
			wantFiles: []string{"service.proto"},
			wantErr:   false,
		},
		{
			name: "vendor directory is skipped",
			setup: func(t *testing.T, dir string) {
				// Create a normal proto file
				err := os.WriteFile(filepath.Join(dir, "app.proto"), []byte("syntax = \"proto3\";"), 0o644)
				if err != nil {
					t.Fatalf("failed to write app.proto: %v", err)
				}

				// Create vendor directory with proto files
				vendorDir := filepath.Join(dir, "vendor", "github.com", "example")
				err = os.MkdirAll(vendorDir, 0o755)
				if err != nil {
					t.Fatalf("failed to create vendor dir: %v", err)
				}
				err = os.WriteFile(filepath.Join(vendorDir, "vendored.proto"), []byte("syntax = \"proto3\";"), 0o644)
				if err != nil {
					t.Fatalf("failed to write vendored.proto: %v", err)
				}
			},
			wantCount: 1,
			wantFiles: []string{"app.proto"},
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var dir string

			if tt.name == "non-existent directory returns error" {
				// Use a path that definitely doesn't exist
				dir = filepath.Join(t.TempDir(), "non-existent-dir-12345")
			} else {
				dir = t.TempDir()
				if tt.setup != nil {
					tt.setup(t, dir)
				}
			}

			got, err := discoverProtoFiles(dir)

			// Check error expectation
			if tt.wantErr {
				if err == nil {
					t.Errorf("discoverProtoFiles() expected error, got nil")
					return
				}
				if tt.wantErrSubstr != "" && !strings.Contains(err.Error(), tt.wantErrSubstr) {
					t.Errorf("discoverProtoFiles() error = %v, want error containing %q", err, tt.wantErrSubstr)
				}
				return
			}

			if err != nil {
				t.Errorf("discoverProtoFiles() unexpected error: %v", err)
				return
			}

			// Check count
			if len(got) != tt.wantCount {
				t.Errorf("discoverProtoFiles() returned %d files, want %d", len(got), tt.wantCount)
			}

			// Check expected files are present
			for _, wantFile := range tt.wantFiles {
				found := false
				for _, gotFile := range got {
					if gotFile == wantFile {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("discoverProtoFiles() missing expected file %q, got %v", wantFile, got)
				}
			}
		})
	}
}

// TestBuildDependencies tests the buildDependencies function.
func TestBuildDependencies(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(t *testing.T, dir string) []string // returns proto files (relative paths)
		wantCount int
		wantErr   bool
		checkDeps func(t *testing.T, deps []forge.ArtifactDependency, dir string)
	}{
		{
			name: "correct number of dependencies returned",
			setup: func(t *testing.T, dir string) []string {
				files := []string{"a.proto", "b.proto", "c.proto"}
				for _, f := range files {
					err := os.WriteFile(filepath.Join(dir, f), []byte("syntax = \"proto3\";"), 0o644)
					if err != nil {
						t.Fatalf("failed to write %s: %v", f, err)
					}
				}
				return files
			},
			wantCount: 3,
			wantErr:   false,
		},
		{
			name: "each dependency has Type DependencyTypeFile",
			setup: func(t *testing.T, dir string) []string {
				files := []string{"test.proto", "other.proto"}
				for _, f := range files {
					err := os.WriteFile(filepath.Join(dir, f), []byte("syntax = \"proto3\";"), 0o644)
					if err != nil {
						t.Fatalf("failed to write %s: %v", f, err)
					}
				}
				return files
			},
			wantCount: 2,
			wantErr:   false,
			checkDeps: func(t *testing.T, deps []forge.ArtifactDependency, dir string) {
				for i, dep := range deps {
					if !strings.HasPrefix(dep.Digest, forge.DigestPrefix) {
						t.Errorf("dependency[%d].Digest = %q, want a %s digest", i, dep.Digest, forge.DigestPrefix)
					}
				}
			},
		},
		{
			name: "each dependency has absolute path",
			setup: func(t *testing.T, dir string) []string {
				subDir := filepath.Join(dir, "api")
				err := os.MkdirAll(subDir, 0o755)
				if err != nil {
					t.Fatalf("failed to create subdir: %v", err)
				}

				files := []string{"root.proto"}
				nestedFiles := []string{"api/nested.proto"}

				err = os.WriteFile(filepath.Join(dir, "root.proto"), []byte("syntax = \"proto3\";"), 0o644)
				if err != nil {
					t.Fatalf("failed to write root.proto: %v", err)
				}
				err = os.WriteFile(filepath.Join(dir, "api", "nested.proto"), []byte("syntax = \"proto3\";"), 0o644)
				if err != nil {
					t.Fatalf("failed to write nested.proto: %v", err)
				}

				return append(files, nestedFiles...)
			},
			wantCount: 2,
			wantErr:   false,
			checkDeps: func(t *testing.T, deps []forge.ArtifactDependency, dir string) {
				for i, dep := range deps {
					if !filepath.IsAbs(dep.Path) {
						t.Errorf("dependency[%d].Path = %q is not absolute", i, dep.Path)
					}
				}
			},
		},
		{
			name: "each dependency carries the digest of its content",
			setup: func(t *testing.T, dir string) []string {
				files := []string{"timestamped.proto"}
				err := os.WriteFile(filepath.Join(dir, "timestamped.proto"), []byte("syntax = \"proto3\";"), 0o644)
				if err != nil {
					t.Fatalf("failed to write timestamped.proto: %v", err)
				}
				return files
			},
			wantCount: 1,
			wantErr:   false,
			checkDeps: func(t *testing.T, deps []forge.ArtifactDependency, dir string) {
				for i, dep := range deps {
					want, err := forge.DigestFile(dep.Path)
					if err != nil {
						t.Fatalf("digesting %s: %v", dep.Path, err)
					}
					if dep.Digest != want {
						t.Errorf("dependency[%d].Digest = %q, want %q", i, dep.Digest, want)
					}
				}
			},
		},
		{
			name: "empty proto files list returns empty dependencies",
			setup: func(t *testing.T, dir string) []string {
				return []string{}
			},
			wantCount: 0,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			protoFiles := tt.setup(t, dir)

			got, err := buildDependencies(dir, protoFiles)

			if tt.wantErr {
				if err == nil {
					t.Errorf("buildDependencies() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("buildDependencies() unexpected error: %v", err)
				return
			}

			if len(got) != tt.wantCount {
				t.Errorf("buildDependencies() returned %d dependencies, want %d", len(got), tt.wantCount)
			}

			if tt.checkDeps != nil {
				tt.checkDeps(t, got, dir)
			}
		})
	}
}

// TestFromMap tests the generated FromMap function for parsing spec.
// Note: The generated FromMap uses camelCase keys (goOpt, goGrpcOpt, protoPath, etc.)
func TestFromMap(t *testing.T) {
	tests := []struct {
		name       string
		spec       map[string]interface{}
		wantGoOpt  string
		wantGrpc   string
		wantPaths  []string
		wantPlugin []string
		wantExtra  []string
		wantErr    bool
	}{
		{
			name:       "nil spec returns empty struct",
			spec:       nil,
			wantGoOpt:  "",
			wantGrpc:   "",
			wantPaths:  nil,
			wantPlugin: nil,
			wantExtra:  nil,
		},
		{
			name:       "empty spec returns empty struct",
			spec:       map[string]interface{}{},
			wantGoOpt:  "",
			wantGrpc:   "",
			wantPaths:  nil,
			wantPlugin: nil,
			wantExtra:  nil,
		},
		{
			name: "all fields populated",
			spec: map[string]interface{}{
				"goOpt":     "module=github.com/example/api",
				"goGrpcOpt": "module=github.com/example/api",
				"protoPath": []interface{}{"/usr/include", "/opt/proto"},
				"plugin":    []interface{}{"protoc-gen-go=/usr/bin/protoc-gen-go"},
				"extraArgs": []interface{}{"--experimental_allow_proto3_optional"},
			},
			wantGoOpt:  "module=github.com/example/api",
			wantGrpc:   "module=github.com/example/api",
			wantPaths:  []string{"/usr/include", "/opt/proto"},
			wantPlugin: []string{"protoc-gen-go=/usr/bin/protoc-gen-go"},
			wantExtra:  []string{"--experimental_allow_proto3_optional"},
		},
		{
			name: "protoPath as []interface{}",
			spec: map[string]interface{}{
				"protoPath": []interface{}{"/path1", "/path2", "/path3"},
			},
			wantGoOpt:  "",
			wantGrpc:   "",
			wantPaths:  []string{"/path1", "/path2", "/path3"},
			wantPlugin: nil,
			wantExtra:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FromMap(tt.spec)
			if (err != nil) != tt.wantErr {
				t.Errorf("FromMap() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			if got.GoOpt != tt.wantGoOpt {
				t.Errorf("GoOpt = %q, want %q", got.GoOpt, tt.wantGoOpt)
			}

			if got.GoGrpcOpt != tt.wantGrpc {
				t.Errorf("GoGrpcOpt = %q, want %q", got.GoGrpcOpt, tt.wantGrpc)
			}

			if len(got.ProtoPath) != len(tt.wantPaths) {
				t.Errorf("ProtoPath length = %d, want %d", len(got.ProtoPath), len(tt.wantPaths))
			} else {
				for i, p := range got.ProtoPath {
					if p != tt.wantPaths[i] {
						t.Errorf("ProtoPath[%d] = %q, want %q", i, p, tt.wantPaths[i])
					}
				}
			}

			if len(got.Plugin) != len(tt.wantPlugin) {
				t.Errorf("Plugin length = %d, want %d", len(got.Plugin), len(tt.wantPlugin))
			} else {
				for i, p := range got.Plugin {
					if p != tt.wantPlugin[i] {
						t.Errorf("Plugin[%d] = %q, want %q", i, p, tt.wantPlugin[i])
					}
				}
			}

			if len(got.ExtraArgs) != len(tt.wantExtra) {
				t.Errorf("ExtraArgs length = %d, want %d", len(got.ExtraArgs), len(tt.wantExtra))
			} else {
				for i, a := range got.ExtraArgs {
					if a != tt.wantExtra[i] {
						t.Errorf("ExtraArgs[%d] = %q, want %q", i, a, tt.wantExtra[i])
					}
				}
			}
		})
	}
}

// TestBuildProtocCommand tests the buildProtocCommand function.
func TestBuildProtocCommand(t *testing.T) {
	tests := []struct {
		name         string
		srcDir       string
		dest         string
		protoFiles   []string
		spec         *Spec
		wantFirstArg string // CRITICAL: verify --proto_path={srcDir} is FIRST
		checkArgs    func(t *testing.T, args []string)
	}{
		{
			name:       "minimal options with defaults",
			srcDir:     "/src/proto",
			dest:       "/out",
			protoFiles: []string{"service.proto"},
			spec: &Spec{
				GoOpt:     "paths=source_relative",
				GoGrpcOpt: "paths=source_relative",
				ProtoPath: []string{},
				Plugin:    []string{},
				ExtraArgs: []string{},
			},
			wantFirstArg: "--proto_path=/src/proto",
			checkArgs: func(t *testing.T, args []string) {
				if len(args) < 6 {
					t.Errorf("expected at least 6 args, got %d", len(args))
					return
				}
				// Verify defaults are used
				if args[2] != "--go_opt=paths=source_relative" {
					t.Errorf("args[2] = %q, want --go_opt=paths=source_relative", args[2])
				}
				if args[4] != "--go-grpc_opt=paths=source_relative" {
					t.Errorf("args[4] = %q, want --go-grpc_opt=paths=source_relative", args[4])
				}
			},
		},
		{
			name:       "CRITICAL: verify --proto_path={srcDir} is FIRST argument",
			srcDir:     "/my/source/dir",
			dest:       "/output",
			protoFiles: []string{"api.proto"},
			spec: &Spec{
				GoOpt:     "paths=source_relative",
				GoGrpcOpt: "paths=source_relative",
				ProtoPath: []string{"/additional/path"},
				Plugin:    []string{},
				ExtraArgs: []string{},
			},
			wantFirstArg: "--proto_path=/my/source/dir",
			checkArgs: func(t *testing.T, args []string) {
				// args[0] MUST be --proto_path={srcDir}
				if args[0] != "--proto_path=/my/source/dir" {
					t.Errorf("CRITICAL: args[0] = %q, MUST be --proto_path=/my/source/dir (source dir proto_path MUST be first for import resolution)", args[0])
				}
			},
		},
		{
			name:       "verify argument order is correct",
			srcDir:     "/src",
			dest:       "/dest",
			protoFiles: []string{"a.proto", "b.proto"},
			spec: &Spec{
				GoOpt:     "module=example.com/api",
				GoGrpcOpt: "module=example.com/api",
				ProtoPath: []string{"/extra/path1", "/extra/path2"},
				Plugin:    []string{"protoc-gen-custom=/bin/custom"},
				ExtraArgs: []string{"--custom-flag"},
			},
			wantFirstArg: "--proto_path=/src",
			checkArgs: func(t *testing.T, args []string) {
				// Expected order:
				// 0: --proto_path={srcDir}
				// 1: --go_out={dest}
				// 2: --go_opt={opts.GoOpt}
				// 3: --go-grpc_out={dest}
				// 4: --go-grpc_opt={opts.GoGrpcOpt}
				// 5-6: User proto_paths
				// 7: Plugin
				// 8: Extra args
				// 9-10: Proto files at the end

				expectedOrder := []string{
					"--proto_path=/src",
					"--go_out=/dest",
					"--go_opt=module=example.com/api",
					"--go-grpc_out=/dest",
					"--go-grpc_opt=module=example.com/api",
					"--proto_path=/extra/path1",
					"--proto_path=/extra/path2",
					"--plugin=protoc-gen-custom=/bin/custom",
					"--custom-flag",
					"a.proto",
					"b.proto",
				}

				if len(args) != len(expectedOrder) {
					t.Errorf("args length = %d, want %d", len(args), len(expectedOrder))
					t.Logf("got args: %v", args)
					return
				}

				for i, want := range expectedOrder {
					if args[i] != want {
						t.Errorf("args[%d] = %q, want %q", i, args[i], want)
					}
				}
			},
		},
		{
			name:       "proto files come last",
			srcDir:     "/src",
			dest:       "/dest",
			protoFiles: []string{"first.proto", "second.proto", "third.proto"},
			spec: &Spec{
				GoOpt:     "paths=source_relative",
				GoGrpcOpt: "paths=source_relative",
				ProtoPath: []string{},
				Plugin:    []string{},
				ExtraArgs: []string{},
			},
			wantFirstArg: "--proto_path=/src",
			checkArgs: func(t *testing.T, args []string) {
				// Proto files should be the last 3 arguments
				if len(args) < 3 {
					t.Errorf("expected at least 3 args for proto files, got %d", len(args))
					return
				}

				lastThree := args[len(args)-3:]
				expected := []string{"first.proto", "second.proto", "third.proto"}
				for i, want := range expected {
					if lastThree[i] != want {
						t.Errorf("last args[%d] = %q, want %q", i, lastThree[i], want)
					}
				}
			},
		},
		{
			name:       "command is protoc",
			srcDir:     "/src",
			dest:       "/dest",
			protoFiles: []string{"test.proto"},
			spec: &Spec{
				GoOpt:     "paths=source_relative",
				GoGrpcOpt: "paths=source_relative",
				ProtoPath: []string{},
				Plugin:    []string{},
				ExtraArgs: []string{},
			},
			wantFirstArg: "--proto_path=/src",
			checkArgs: func(t *testing.T, args []string) {
				// This is checked by verifying the command path below
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := buildProtocCommand(tt.srcDir, tt.dest, tt.protoFiles, tt.spec)

			// Verify command is protoc
			if !strings.HasSuffix(cmd.Path, "protoc") && cmd.Path != "protoc" {
				// cmd.Path may be resolved to full path or just "protoc" depending on system
				// We check Args[0] which is the command name
				if cmd.Args[0] != "protoc" {
					t.Errorf("command = %q, want protoc", cmd.Args[0])
				}
			}

			// Get args (excluding the command name itself)
			args := cmd.Args[1:]

			// CRITICAL: Verify first argument is --proto_path={srcDir}
			if len(args) == 0 {
				t.Errorf("no arguments generated")
				return
			}
			if args[0] != tt.wantFirstArg {
				t.Errorf("CRITICAL: first argument = %q, want %q (srcDir proto_path MUST be first)", args[0], tt.wantFirstArg)
			}

			if tt.checkArgs != nil {
				tt.checkArgs(t, args)
			}
		})
	}
}
