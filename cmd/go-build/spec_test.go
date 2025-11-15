//go:build unit

package main

import (
	"testing"

	"github.com/alexandremahdhaoui/forge/pkg/forge"
	"github.com/alexandremahdhaoui/forge/pkg/mcptypes"
)

func TestExtractBuildOptions(t *testing.T) {
	tests := []struct {
		name     string
		spec     forge.BuildSpec
		wantArgs []string
		wantEnv  map[string]string
		wantNil  bool
	}{
		{
			name: "empty spec",
			spec: forge.BuildSpec{
				Name:   "test",
				Src:    "./cmd/test",
				Engine: "go://go-build",
				Spec:   nil,
			},
			wantNil: true,
		},
		{
			name: "spec with args only",
			spec: forge.BuildSpec{
				Name:   "test",
				Src:    "./cmd/test",
				Engine: "go://go-build",
				Spec: map[string]interface{}{
					"args": []interface{}{"-tags=netgo", "-ldflags=-w -s"},
				},
			},
			wantArgs: []string{"-tags=netgo", "-ldflags=-w -s"},
			wantEnv:  nil,
			wantNil:  false,
		},
		{
			name: "spec with env only",
			spec: forge.BuildSpec{
				Name:   "test",
				Src:    "./cmd/test",
				Engine: "go://go-build",
				Spec: map[string]interface{}{
					"env": map[string]interface{}{
						"GOOS":        "linux",
						"GOARCH":      "amd64",
						"CGO_ENABLED": "0",
					},
				},
			},
			wantArgs: nil,
			wantEnv: map[string]string{
				"GOOS":        "linux",
				"GOARCH":      "amd64",
				"CGO_ENABLED": "0",
			},
			wantNil: false,
		},
		{
			name: "spec with both args and env",
			spec: forge.BuildSpec{
				Name:   "test",
				Src:    "./cmd/test",
				Engine: "go://go-build",
				Spec: map[string]interface{}{
					"args": []interface{}{"-tags=netgo"},
					"env": map[string]interface{}{
						"CGO_ENABLED": "0",
					},
				},
			},
			wantArgs: []string{"-tags=netgo"},
			wantEnv: map[string]string{
				"CGO_ENABLED": "0",
			},
			wantNil: false,
		},
		{
			name: "spec with invalid args type",
			spec: forge.BuildSpec{
				Name:   "test",
				Src:    "./cmd/test",
				Engine: "go://go-build",
				Spec: map[string]interface{}{
					"args": "invalid",
				},
			},
			wantNil: true,
		},
		{
			name: "spec with invalid env type",
			spec: forge.BuildSpec{
				Name:   "test",
				Src:    "./cmd/test",
				Engine: "go://go-build",
				Spec: map[string]interface{}{
					"env": "invalid",
				},
			},
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractBuildOptions(tt.spec)

			if tt.wantNil {
				if got != nil {
					t.Errorf("extractBuildOptions() = %v, want nil", got)
				}
				return
			}

			if got == nil {
				t.Fatal("extractBuildOptions() = nil, want non-nil")
			}

			// Check args
			if len(tt.wantArgs) != len(got.CustomArgs) {
				t.Errorf("CustomArgs length = %d, want %d", len(got.CustomArgs), len(tt.wantArgs))
			} else {
				for i, arg := range tt.wantArgs {
					if got.CustomArgs[i] != arg {
						t.Errorf("CustomArgs[%d] = %q, want %q", i, got.CustomArgs[i], arg)
					}
				}
			}

			// Check env
			if len(tt.wantEnv) != len(got.CustomEnv) {
				t.Errorf("CustomEnv length = %d, want %d", len(got.CustomEnv), len(tt.wantEnv))
			} else {
				for key, want := range tt.wantEnv {
					if got.CustomEnv[key] != want {
						t.Errorf("CustomEnv[%q] = %q, want %q", key, got.CustomEnv[key], want)
					}
				}
			}
		})
	}
}

func TestExtractBuildOptionsFromInput(t *testing.T) {
	tests := []struct {
		name     string
		input    mcptypes.BuildInput
		wantArgs []string
		wantEnv  map[string]string
		wantNil  bool
	}{
		{
			name: "empty input",
			input: mcptypes.BuildInput{
				Name:   "test",
				Src:    "./cmd/test",
				Engine: "go://go-build",
			},
			wantNil: true,
		},
		{
			name: "direct args field",
			input: mcptypes.BuildInput{
				Name:   "test",
				Src:    "./cmd/test",
				Engine: "go://go-build",
				Args:   []string{"-tags=netgo"},
			},
			wantArgs: []string{"-tags=netgo"},
			wantNil:  false,
		},
		{
			name: "direct env field",
			input: mcptypes.BuildInput{
				Name:   "test",
				Src:    "./cmd/test",
				Engine: "go://go-build",
				Env: map[string]string{
					"GOOS": "linux",
				},
			},
			wantEnv: map[string]string{
				"GOOS": "linux",
			},
			wantNil: false,
		},
		{
			name: "spec field with args",
			input: mcptypes.BuildInput{
				Name:   "test",
				Src:    "./cmd/test",
				Engine: "go://go-build",
				Spec: map[string]interface{}{
					"args": []interface{}{"-tags=integration"},
				},
			},
			wantArgs: []string{"-tags=integration"},
			wantNil:  false,
		},
		{
			name: "spec field with env",
			input: mcptypes.BuildInput{
				Name:   "test",
				Src:    "./cmd/test",
				Engine: "go://go-build",
				Spec: map[string]interface{}{
					"env": map[string]interface{}{
						"GOARCH": "arm64",
					},
				},
			},
			wantEnv: map[string]string{
				"GOARCH": "arm64",
			},
			wantNil: false,
		},
		{
			name: "direct fields take precedence over spec",
			input: mcptypes.BuildInput{
				Name:   "test",
				Src:    "./cmd/test",
				Engine: "go://go-build",
				Args:   []string{"-tags=direct"},
				Env: map[string]string{
					"GOOS": "direct",
				},
				Spec: map[string]interface{}{
					"args": []interface{}{"-tags=spec"},
					"env": map[string]interface{}{
						"GOOS": "spec",
					},
				},
			},
			wantArgs: []string{"-tags=direct"},
			wantEnv: map[string]string{
				"GOOS": "direct",
			},
			wantNil: false,
		},
		{
			name: "spec and direct fields combined",
			input: mcptypes.BuildInput{
				Name:   "test",
				Src:    "./cmd/test",
				Engine: "go://go-build",
				Args:   []string{"-tags=direct"},
				Spec: map[string]interface{}{
					"env": map[string]interface{}{
						"GOOS": "linux",
					},
				},
			},
			wantArgs: []string{"-tags=direct"},
			wantEnv: map[string]string{
				"GOOS": "linux",
			},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractBuildOptionsFromInput(tt.input)

			if tt.wantNil {
				if got != nil {
					t.Errorf("extractBuildOptionsFromInput() = %v, want nil", got)
				}
				return
			}

			if got == nil {
				t.Fatal("extractBuildOptionsFromInput() = nil, want non-nil")
			}

			// Check args
			if len(tt.wantArgs) != len(got.CustomArgs) {
				t.Errorf("CustomArgs length = %d, want %d", len(got.CustomArgs), len(tt.wantArgs))
			} else {
				for i, arg := range tt.wantArgs {
					if got.CustomArgs[i] != arg {
						t.Errorf("CustomArgs[%d] = %q, want %q", i, got.CustomArgs[i], arg)
					}
				}
			}

			// Check env
			if len(tt.wantEnv) != len(got.CustomEnv) {
				t.Errorf("CustomEnv length = %d, want %d", len(got.CustomEnv), len(tt.wantEnv))
			} else {
				for key, want := range tt.wantEnv {
					if got.CustomEnv[key] != want {
						t.Errorf("CustomEnv[%q] = %q, want %q", key, got.CustomEnv[key], want)
					}
				}
			}
		})
	}
}
