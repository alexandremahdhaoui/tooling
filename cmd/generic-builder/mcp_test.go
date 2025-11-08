//go:build unit

package main

import (
	"testing"

	"github.com/alexandremahdhaoui/forge/pkg/mcptypes"
)

func TestProcessTemplatedArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		input    mcptypes.BuildInput
		expected []string
		wantErr  bool
	}{
		{
			name: "simple template substitution",
			args: []string{"build", "-o", "{{ .Dest }}/{{ .Name }}", "{{ .Src }}"},
			input: mcptypes.BuildInput{
				Name: "myapp",
				Src:  "./cmd/myapp",
				Dest: "./build/bin",
			},
			expected: []string{"build", "-o", "./build/bin/myapp", "./cmd/myapp"},
			wantErr:  false,
		},
		{
			name: "no templates",
			args: []string{"echo", "hello", "world"},
			input: mcptypes.BuildInput{
				Name: "test",
			},
			expected: []string{"echo", "hello", "world"},
			wantErr:  false,
		},
		{
			name: "empty args",
			args: []string{},
			input: mcptypes.BuildInput{
				Name: "test",
			},
			expected: []string{},
			wantErr:  false,
		},
		{
			name: "all template variables",
			args: []string{"Name={{ .Name }}", "Src={{ .Src }}", "Dest={{ .Dest }}", "Engine={{ .Engine }}"},
			input: mcptypes.BuildInput{
				Name:   "myapp",
				Src:    "./src",
				Dest:   "./dest",
				Engine: "go://build-go",
			},
			expected: []string{"Name=myapp", "Src=./src", "Dest=./dest", "Engine=go://build-go"},
			wantErr:  false,
		},
		{
			name: "invalid template",
			args: []string{"{{ .Invalid }"},
			input: mcptypes.BuildInput{
				Name: "test",
			},
			expected: nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := processTemplatedArgs(tt.args, tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("processTemplatedArgs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if len(got) != len(tt.expected) {
				t.Errorf("processTemplatedArgs() length = %v, want %v", len(got), len(tt.expected))
				return
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("processTemplatedArgs()[%d] = %v, want %v", i, got[i], tt.expected[i])
				}
			}
		})
	}
}
