//go:build unit

package main

import (
	"testing"
)

func TestParseEngine(t *testing.T) {
	tests := []struct {
		name        string
		engineURI   string
		wantType    string
		wantCommand string
		wantArgs    []string
		wantErr     bool
	}{
		{
			name:        "simple build-go",
			engineURI:   "go://build-go",
			wantType:    "mcp",
			wantCommand: "go",
			wantArgs:    []string{"run", "github.com/alexandremahdhaoui/forge/cmd/build-go"},
			wantErr:     false,
		},
		{
			name:        "simple build-container",
			engineURI:   "go://build-container",
			wantType:    "mcp",
			wantCommand: "go",
			wantArgs:    []string{"run", "github.com/alexandremahdhaoui/forge/cmd/build-container"},
			wantErr:     false,
		},
		{
			name:        "full path",
			engineURI:   "go://github.com/alexandremahdhaoui/forge/cmd/build-go",
			wantType:    "mcp",
			wantCommand: "go",
			wantArgs:    []string{"run", "github.com/alexandremahdhaoui/forge/cmd/build-go"},
			wantErr:     false,
		},
		{
			name:        "invalid protocol",
			engineURI:   "http://build-go",
			wantType:    "",
			wantCommand: "",
			wantArgs:    nil,
			wantErr:     true,
		},
		{
			name:        "empty after protocol",
			engineURI:   "go://",
			wantType:    "",
			wantCommand: "",
			wantArgs:    nil,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotCommand, gotArgs, err := parseEngine(tt.engineURI)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseEngine() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotType != tt.wantType {
				t.Errorf("parseEngine() gotType = %v, want %v", gotType, tt.wantType)
			}
			if gotCommand != tt.wantCommand {
				t.Errorf("parseEngine() gotCommand = %v, want %v", gotCommand, tt.wantCommand)
			}
			if len(gotArgs) != len(tt.wantArgs) {
				t.Errorf("parseEngine() gotArgs length = %v, want %v", len(gotArgs), len(tt.wantArgs))
			} else {
				for i, arg := range gotArgs {
					if arg != tt.wantArgs[i] {
						t.Errorf("parseEngine() gotArgs[%d] = %v, want %v", i, arg, tt.wantArgs[i])
					}
				}
			}
		})
	}
}
