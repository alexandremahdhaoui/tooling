package main

import (
	"testing"
)

func TestParseEngine(t *testing.T) {
	tests := []struct {
		name           string
		engineURI      string
		wantType       string
		wantBinaryPath string
		wantErr        bool
	}{
		{
			name:           "simple build-go",
			engineURI:      "go://build-go",
			wantType:       "mcp",
			wantBinaryPath: "build-go",
			wantErr:        false,
		},
		{
			name:           "simple build-container",
			engineURI:      "go://build-container",
			wantType:       "mcp",
			wantBinaryPath: "build-container",
			wantErr:        false,
		},
		{
			name:           "full path",
			engineURI:      "go://github.com/alexandremahdhaoui/forge/cmd/build-go",
			wantType:       "mcp",
			wantBinaryPath: "build-go",
			wantErr:        false,
		},
		{
			name:           "invalid protocol",
			engineURI:      "http://build-go",
			wantType:       "",
			wantBinaryPath: "",
			wantErr:        true,
		},
		{
			name:           "empty after protocol",
			engineURI:      "go://",
			wantType:       "",
			wantBinaryPath: "",
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotPath, err := parseEngine(tt.engineURI)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseEngine() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotType != tt.wantType {
				t.Errorf("parseEngine() gotType = %v, want %v", gotType, tt.wantType)
			}
			if gotPath != tt.wantBinaryPath {
				t.Errorf("parseEngine() gotPath = %v, want %v", gotPath, tt.wantBinaryPath)
			}
		})
	}
}
