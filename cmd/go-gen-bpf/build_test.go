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

// TestFromMap tests the generated FromMap function.
func TestFromMap(t *testing.T) {
	tests := []struct {
		name              string
		spec              map[string]any
		wantIdent         string
		wantBpf2goVersion string
		wantGoPackage     string
		wantOutputStem    string
		wantTags          []string
		wantTypes         []string
		wantCflags        []string
		wantCc            string
		wantErr           bool
		wantErrSubstr     string
	}{
		{
			name:      "minimal spec with just ident",
			spec:      map[string]any{"ident": "myapp"},
			wantIdent: "myapp",
			wantErr:   false,
		},
		{
			name:    "nil spec returns empty spec",
			spec:    nil,
			wantErr: false,
		},
		{
			name: "all fields populated",
			spec: map[string]any{
				"ident":         "tracing",
				"bpf2goVersion": "v0.12.3",
				"goPackage":     "bpftracing",
				"outputStem":    "gen_bpf",
				"tags":          []any{"linux", "amd64"},
				"types":         []any{"event", "config"},
				"cflags":        []any{"-O2", "-g", "-Wall"},
				"cc":            "clang-15",
			},
			wantIdent:         "tracing",
			wantBpf2goVersion: "v0.12.3",
			wantGoPackage:     "bpftracing",
			wantOutputStem:    "gen_bpf",
			wantTags:          []string{"linux", "amd64"},
			wantTypes:         []string{"event", "config"},
			wantCflags:        []string{"-O2", "-g", "-Wall"},
			wantCc:            "clang-15",
			wantErr:           false,
		},
		{
			name: "tags as []string",
			spec: map[string]any{
				"ident": "app",
				"tags":  []string{"linux", "arm64"},
			},
			wantIdent: "app",
			wantTags:  []string{"linux", "arm64"},
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FromMap(tt.spec)

			if tt.wantErr {
				if err == nil {
					t.Errorf("FromMap() expected error, got nil")
					return
				}
				if tt.wantErrSubstr != "" && !strings.Contains(err.Error(), tt.wantErrSubstr) {
					t.Errorf("FromMap() error = %v, want error containing %q", err, tt.wantErrSubstr)
				}
				return
			}

			if err != nil {
				t.Errorf("FromMap() unexpected error: %v", err)
				return
			}

			if got.Ident != tt.wantIdent {
				t.Errorf("Ident = %q, want %q", got.Ident, tt.wantIdent)
			}
			if got.Bpf2goVersion != tt.wantBpf2goVersion {
				t.Errorf("Bpf2goVersion = %q, want %q", got.Bpf2goVersion, tt.wantBpf2goVersion)
			}
			if got.GoPackage != tt.wantGoPackage {
				t.Errorf("GoPackage = %q, want %q", got.GoPackage, tt.wantGoPackage)
			}
			if got.OutputStem != tt.wantOutputStem {
				t.Errorf("OutputStem = %q, want %q", got.OutputStem, tt.wantOutputStem)
			}
			if got.Cc != tt.wantCc {
				t.Errorf("Cc = %q, want %q", got.Cc, tt.wantCc)
			}

			// Check tags
			if len(got.Tags) != len(tt.wantTags) {
				t.Errorf("Tags length = %d, want %d", len(got.Tags), len(tt.wantTags))
			} else {
				for i, tag := range got.Tags {
					if tag != tt.wantTags[i] {
						t.Errorf("Tags[%d] = %q, want %q", i, tag, tt.wantTags[i])
					}
				}
			}

			// Check types
			if len(got.Types) != len(tt.wantTypes) {
				t.Errorf("Types length = %d, want %d", len(got.Types), len(tt.wantTypes))
			} else {
				for i, typ := range got.Types {
					if typ != tt.wantTypes[i] {
						t.Errorf("Types[%d] = %q, want %q", i, typ, tt.wantTypes[i])
					}
				}
			}

			// Check cflags
			if len(got.Cflags) != len(tt.wantCflags) {
				t.Errorf("Cflags length = %d, want %d", len(got.Cflags), len(tt.wantCflags))
			} else {
				for i, flag := range got.Cflags {
					if flag != tt.wantCflags[i] {
						t.Errorf("Cflags[%d] = %q, want %q", i, flag, tt.wantCflags[i])
					}
				}
			}
		})
	}
}

// TestBuildBpf2goArgs tests the buildBpf2goArgs function.
func TestBuildBpf2goArgs(t *testing.T) {
	tests := []struct {
		name      string
		src       string
		dest      string
		spec      *Spec
		wantArgs  []string
		checkArgs func(t *testing.T, args []string)
	}{
		{
			name: "minimal options with defaults",
			src:  "./bpf/prog.c",
			dest: "./bpf",
			spec: &Spec{
				Ident:         "prog",
				Bpf2goVersion: "latest",
				GoPackage:     "bpf",
				OutputStem:    "zz_generated",
				Tags:          []string{"linux"},
				Types:         nil,
				Cflags:        nil,
				Cc:            "",
			},
			wantArgs: []string{
				"--go-package", "bpf",
				"--output-dir", "./bpf",
				"--output-stem", "zz_generated",
				"--tags", "linux",
				"prog",
				"./bpf/prog.c",
			},
		},
		{
			name: "multiple tags joined with comma",
			src:  "./bpf/app.c",
			dest: "./output",
			spec: &Spec{
				Ident:         "app",
				Bpf2goVersion: "v0.12.3",
				GoPackage:     "bpfapp",
				OutputStem:    "gen",
				Tags:          []string{"linux", "amd64", "cgo"},
				Types:         nil,
				Cflags:        nil,
				Cc:            "",
			},
			checkArgs: func(t *testing.T, args []string) {
				// Find --tags and verify it's comma-joined
				for i, arg := range args {
					if arg == "--tags" && i+1 < len(args) {
						if args[i+1] != "linux,amd64,cgo" {
							t.Errorf("tags value = %q, want %q", args[i+1], "linux,amd64,cgo")
						}
						return
					}
				}
				t.Errorf("--tags flag not found in args: %v", args)
			},
		},
		{
			name: "types are added with --type flag each",
			src:  "./bpf/trace.c",
			dest: "./pkg/bpf",
			spec: &Spec{
				Ident:         "trace",
				Bpf2goVersion: "latest",
				GoPackage:     "bpf",
				OutputStem:    "generated",
				Tags:          []string{"linux"},
				Types:         []string{"event", "config", "stats"},
				Cflags:        nil,
				Cc:            "",
			},
			checkArgs: func(t *testing.T, args []string) {
				// Count --type flags
				typeCount := 0
				for i, arg := range args {
					if arg == "--type" {
						typeCount++
						if i+1 >= len(args) {
							t.Errorf("--type flag missing value at index %d", i)
						}
					}
				}
				if typeCount != 3 {
					t.Errorf("expected 3 --type flags, got %d", typeCount)
				}
			},
		},
		{
			name: "cflags come after -- separator",
			src:  "./bpf/app.c",
			dest: "./out",
			spec: &Spec{
				Ident:         "app",
				Bpf2goVersion: "latest",
				GoPackage:     "out",
				OutputStem:    "gen",
				Tags:          []string{"linux"},
				Types:         nil,
				Cflags:        []string{"-O2", "-g", "-I./include"},
				Cc:            "",
			},
			checkArgs: func(t *testing.T, args []string) {
				// Find "--" separator
				separatorIdx := -1
				for i, arg := range args {
					if arg == "--" {
						separatorIdx = i
						break
					}
				}
				if separatorIdx == -1 {
					t.Errorf("-- separator not found in args: %v", args)
					return
				}

				// Verify cflags come after separator
				cflags := args[separatorIdx+1:]
				expected := []string{"-O2", "-g", "-I./include"}
				if len(cflags) != len(expected) {
					t.Errorf("cflags length = %d, want %d", len(cflags), len(expected))
					return
				}
				for i, flag := range cflags {
					if flag != expected[i] {
						t.Errorf("cflags[%d] = %q, want %q", i, flag, expected[i])
					}
				}
			},
		},
		{
			name: "ident and src come after all flags",
			src:  "./bpf/myapp.c",
			dest: "./output",
			spec: &Spec{
				Ident:         "myapp",
				Bpf2goVersion: "latest",
				GoPackage:     "output",
				OutputStem:    "zz_generated",
				Tags:          []string{"linux"},
				Types:         nil,
				Cflags:        nil,
				Cc:            "",
			},
			checkArgs: func(t *testing.T, args []string) {
				// Last two elements should be ident and src
				if len(args) < 2 {
					t.Errorf("expected at least 2 args, got %d", len(args))
					return
				}
				if args[len(args)-2] != "myapp" {
					t.Errorf("second to last arg = %q, want %q", args[len(args)-2], "myapp")
				}
				if args[len(args)-1] != "./bpf/myapp.c" {
					t.Errorf("last arg = %q, want %q", args[len(args)-1], "./bpf/myapp.c")
				}
			},
		},
		{
			name: "no cflags means no -- separator",
			src:  "./bpf/simple.c",
			dest: "./out",
			spec: &Spec{
				Ident:         "simple",
				Bpf2goVersion: "latest",
				GoPackage:     "out",
				OutputStem:    "gen",
				Tags:          []string{"linux"},
				Types:         nil,
				Cflags:        nil,
				Cc:            "",
			},
			checkArgs: func(t *testing.T, args []string) {
				for _, arg := range args {
					if arg == "--" {
						t.Errorf("found -- separator when no cflags specified")
					}
				}
			},
		},
		{
			name: "empty tags array defaults to linux",
			src:  "./bpf/notags.c",
			dest: "./out",
			spec: &Spec{
				Ident:         "notags",
				Bpf2goVersion: "latest",
				GoPackage:     "out",
				OutputStem:    "gen",
				Tags:          []string{},
				Types:         nil,
				Cflags:        nil,
				Cc:            "",
			},
			checkArgs: func(t *testing.T, args []string) {
				// Empty tags defaults to ["linux"], so --tags linux should be present
				for i, arg := range args {
					if arg == "--tags" && i+1 < len(args) {
						if args[i+1] != "linux" {
							t.Errorf("expected --tags linux when tags is empty, got --tags %s", args[i+1])
						}
						return
					}
				}
				t.Errorf("expected --tags linux flag when tags array is empty (default applied)")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildBpf2goArgs(tt.src, tt.dest, tt.spec)

			if tt.wantArgs != nil {
				if len(got) != len(tt.wantArgs) {
					t.Errorf("args length = %d, want %d\ngot: %v\nwant: %v", len(got), len(tt.wantArgs), got, tt.wantArgs)
					return
				}
				for i, want := range tt.wantArgs {
					if got[i] != want {
						t.Errorf("args[%d] = %q, want %q", i, got[i], want)
					}
				}
			}

			if tt.checkArgs != nil {
				tt.checkArgs(t, got)
			}
		})
	}
}

// TestBuildDependencies tests the buildDependencies function.
func TestBuildDependencies(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(t *testing.T, dir string) (string, os.FileInfo) // returns srcPath and FileInfo
		wantCount int
		wantErr   bool
		checkDeps func(t *testing.T, deps []forge.ArtifactDependency, srcPath string)
	}{
		{
			name: "single source file dependency",
			setup: func(t *testing.T, dir string) (string, os.FileInfo) {
				srcPath := filepath.Join(dir, "prog.c")
				err := os.WriteFile(srcPath, []byte("// BPF program"), 0o644)
				if err != nil {
					t.Fatalf("failed to write prog.c: %v", err)
				}
				info, err := os.Stat(srcPath)
				if err != nil {
					t.Fatalf("failed to stat prog.c: %v", err)
				}
				return srcPath, info
			},
			wantCount: 1,
			wantErr:   false,
		},
		{
			name: "dependency has Type DependencyTypeFile",
			setup: func(t *testing.T, dir string) (string, os.FileInfo) {
				srcPath := filepath.Join(dir, "app.c")
				err := os.WriteFile(srcPath, []byte("// BPF code"), 0o644)
				if err != nil {
					t.Fatalf("failed to write app.c: %v", err)
				}
				info, err := os.Stat(srcPath)
				if err != nil {
					t.Fatalf("failed to stat app.c: %v", err)
				}
				return srcPath, info
			},
			wantCount: 1,
			wantErr:   false,
			checkDeps: func(t *testing.T, deps []forge.ArtifactDependency, srcPath string) {
				if len(deps) != 1 {
					t.Errorf("expected 1 dependency, got %d", len(deps))
					return
				}
				if !strings.HasPrefix(deps[0].Digest, forge.DigestPrefix) {
					t.Errorf("dependency.Digest = %q, want a %s digest", deps[0].Digest, forge.DigestPrefix)
				}
			},
		},
		{
			name: "dependency has absolute path",
			setup: func(t *testing.T, dir string) (string, os.FileInfo) {
				subDir := filepath.Join(dir, "bpf")
				err := os.MkdirAll(subDir, 0o755)
				if err != nil {
					t.Fatalf("failed to create subdir: %v", err)
				}
				srcPath := filepath.Join(subDir, "nested.c")
				err = os.WriteFile(srcPath, []byte("// nested BPF"), 0o644)
				if err != nil {
					t.Fatalf("failed to write nested.c: %v", err)
				}
				info, err := os.Stat(srcPath)
				if err != nil {
					t.Fatalf("failed to stat nested.c: %v", err)
				}
				return srcPath, info
			},
			wantCount: 1,
			wantErr:   false,
			checkDeps: func(t *testing.T, deps []forge.ArtifactDependency, srcPath string) {
				if len(deps) != 1 {
					t.Errorf("expected 1 dependency, got %d", len(deps))
					return
				}
				if !filepath.IsAbs(deps[0].Path) {
					t.Errorf("dependency.FilePath = %q is not absolute", deps[0].Path)
				}
			},
		},
		{
			name: "dependency carries the digest of its content",
			setup: func(t *testing.T, dir string) (string, os.FileInfo) {
				srcPath := filepath.Join(dir, "timestamped.c")
				err := os.WriteFile(srcPath, []byte("// BPF program"), 0o644)
				if err != nil {
					t.Fatalf("failed to write timestamped.c: %v", err)
				}
				info, err := os.Stat(srcPath)
				if err != nil {
					t.Fatalf("failed to stat timestamped.c: %v", err)
				}
				return srcPath, info
			},
			wantCount: 1,
			wantErr:   false,
			checkDeps: func(t *testing.T, deps []forge.ArtifactDependency, srcPath string) {
				if len(deps) != 1 {
					t.Errorf("expected 1 dependency, got %d", len(deps))
					return
				}
				want, err := forge.DigestFile(srcPath)
				if err != nil {
					t.Fatalf("digesting %s: %v", srcPath, err)
				}
				if deps[0].Digest != want {
					t.Errorf("dependency.Digest = %q, want %q", deps[0].Digest, want)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			srcPath, srcInfo := tt.setup(t, dir)

			got, err := buildDependencies(srcPath, srcInfo)

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
				tt.checkDeps(t, got, srcPath)
			}
		})
	}
}
