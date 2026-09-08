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
	"strings"
	"testing"
)

func TestValidateValueFrom(t *testing.T) {
	tests := []struct {
		name      string
		vf        ValueFrom
		fieldName string
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "valid envName",
			vf:        ValueFrom{EnvName: "MY_VAR"},
			fieldName: "username",
			wantErr:   false,
		},
		{
			name:      "valid literal",
			vf:        ValueFrom{Literal: "myvalue"},
			fieldName: "password",
			wantErr:   false,
		},
		{
			name:      "both envName and literal set",
			vf:        ValueFrom{EnvName: "MY_VAR", Literal: "myvalue"},
			fieldName: "username",
			wantErr:   true,
			errMsg:    "cannot specify both",
		},
		{
			name:      "neither envName nor literal set",
			vf:        ValueFrom{},
			fieldName: "password",
			wantErr:   true,
			errMsg:    "must specify either",
		},
		{
			name:      "empty literal treated as not set",
			vf:        ValueFrom{Literal: ""},
			fieldName: "username",
			wantErr:   true,
			errMsg:    "must specify either",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateValueFrom(tt.vf, tt.fieldName)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errMsg)
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestValidateBasicAuth(t *testing.T) {
	tests := []struct {
		name    string
		auth    BasicAuth
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid auth with env vars",
			auth: BasicAuth{
				Username: ValueFrom{EnvName: "USER_VAR"},
				Password: ValueFrom{EnvName: "PASS_VAR"},
			},
			wantErr: false,
		},
		{
			name: "valid auth with literals",
			auth: BasicAuth{
				Username: ValueFrom{Literal: "myuser"},
				Password: ValueFrom{Literal: "mypass"},
			},
			wantErr: false,
		},
		{
			name: "valid auth with mixed",
			auth: BasicAuth{
				Username: ValueFrom{EnvName: "USER_VAR"},
				Password: ValueFrom{Literal: "mypass"},
			},
			wantErr: false,
		},
		{
			name: "invalid username - neither set",
			auth: BasicAuth{
				Username: ValueFrom{},
				Password: ValueFrom{Literal: "mypass"},
			},
			wantErr: true,
			errMsg:  "username",
		},
		{
			name: "invalid password - both set",
			auth: BasicAuth{
				Username: ValueFrom{Literal: "myuser"},
				Password: ValueFrom{EnvName: "PASS", Literal: "mypass"},
			},
			wantErr: true,
			errMsg:  "password",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBasicAuth(tt.auth)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errMsg)
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestValidateImageSource(t *testing.T) {
	tests := []struct {
		name    string
		img     ImageSource
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid local image",
			img:     ImageSource{Name: "local://myapp:v1"},
			wantErr: false,
		},
		{
			name:    "valid remote image with registry",
			img:     ImageSource{Name: "quay.io/example/img:v1"},
			wantErr: false,
		},
		{
			name:    "valid remote image docker hub org",
			img:     ImageSource{Name: "myorg/myapp:v1"},
			wantErr: false,
		},
		{
			name:    "valid remote image docker hub library",
			img:     ImageSource{Name: "alpine:latest"},
			wantErr: false,
		},
		{
			name: "valid local with auth",
			img: ImageSource{
				Name: "local://myapp:v1",
				BasicAuth: BasicAuth{
					Username: ValueFrom{Literal: "user"},
					Password: ValueFrom{Literal: "pass"},
				},
			},
			wantErr: false,
		},
		{
			name: "valid remote with auth",
			img: ImageSource{
				Name: "quay.io/example:v1",
				BasicAuth: BasicAuth{
					Username: ValueFrom{EnvName: "USER"},
					Password: ValueFrom{EnvName: "PASS"},
				},
			},
			wantErr: false,
		},
		{
			name:    "empty name",
			img:     ImageSource{Name: ""},
			wantErr: true,
			errMsg:  "name must not be empty",
		},
		{
			name:    "local without tag",
			img:     ImageSource{Name: "local://myapp"},
			wantErr: true,
			errMsg:  "must include tag",
		},
		{
			name:    "local with registry domain",
			img:     ImageSource{Name: "local://registry.io/myapp:v1"},
			wantErr: true,
			errMsg:  "must not contain registry domain or slashes",
		},
		{
			name:    "local with slash in name",
			img:     ImageSource{Name: "local://org/myapp:v1"},
			wantErr: true,
			errMsg:  "must not contain registry domain or slashes",
		},
		{
			name:    "remote without tag",
			img:     ImageSource{Name: "quay.io/example/img"},
			wantErr: true,
			errMsg:  "must include tag",
		},
		{
			name:    "remote docker hub without tag",
			img:     ImageSource{Name: "myorg/myapp"},
			wantErr: true,
			errMsg:  "must include tag",
		},
		{
			name: "invalid basicAuth - username both set",
			img: ImageSource{
				Name: "local://myapp:v1",
				BasicAuth: BasicAuth{
					Username: ValueFrom{EnvName: "USER", Literal: "user"},
					Password: ValueFrom{Literal: "pass"},
				},
			},
			wantErr: true,
			errMsg:  "basicAuth",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateImageSource(tt.img)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errMsg)
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestValidateImages(t *testing.T) {
	tests := []struct {
		name    string
		images  []ImageSource
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid list with multiple images",
			images: []ImageSource{
				{Name: "local://app1:v1"},
				{Name: "quay.io/example/app2:v1"},
				{Name: "alpine:latest"},
			},
			wantErr: false,
		},
		{
			name:    "empty list is valid",
			images:  []ImageSource{},
			wantErr: false,
		},
		{
			name:    "nil list is valid",
			images:  nil,
			wantErr: false,
		},
		{
			name: "duplicate images",
			images: []ImageSource{
				{Name: "local://myapp:v1"},
				{Name: "quay.io/example:v1"},
				{Name: "local://myapp:v1"},
			},
			wantErr: true,
			errMsg:  "duplicate",
		},
		{
			name: "invalid image in list",
			images: []ImageSource{
				{Name: "local://myapp:v1"},
				{Name: ""},
			},
			wantErr: true,
			errMsg:  "images[1]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateImages(tt.images)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errMsg)
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestResolveValueFrom(t *testing.T) {
	tests := []struct {
		name      string
		vf        ValueFrom
		fieldName string
		envSet    map[string]string
		wantValue string
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "env var exists and non-empty",
			vf:        ValueFrom{EnvName: "TEST_VAR"},
			fieldName: "username",
			envSet:    map[string]string{"TEST_VAR": "testvalue"},
			wantValue: "testvalue",
			wantErr:   false,
		},
		{
			name:      "env var not set",
			vf:        ValueFrom{EnvName: "MISSING_VAR"},
			fieldName: "username",
			envSet:    map[string]string{},
			wantErr:   true,
			errMsg:    "not set",
		},
		{
			name:      "env var set to empty",
			vf:        ValueFrom{EnvName: "EMPTY_VAR"},
			fieldName: "password",
			envSet:    map[string]string{"EMPTY_VAR": ""},
			wantErr:   true,
			errMsg:    "is empty",
		},
		{
			name:      "literal value",
			vf:        ValueFrom{Literal: "myliteral"},
			fieldName: "username",
			envSet:    map[string]string{},
			wantValue: "myliteral",
			wantErr:   false,
		},
		{
			name:      "neither set",
			vf:        ValueFrom{},
			fieldName: "username",
			envSet:    map[string]string{},
			wantErr:   true,
			errMsg:    "neither",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up environment before test
			if tt.vf.EnvName != "" {
				_ = os.Unsetenv(tt.vf.EnvName)
			}

			// Set up environment for test
			for k, v := range tt.envSet {
				_ = os.Setenv(k, v)
			}

			// Clean up after test
			defer func() {
				for k := range tt.envSet {
					_ = os.Unsetenv(k)
				}
			}()

			value, err := ResolveValueFrom(tt.vf, tt.fieldName)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errMsg)
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
				if value != tt.wantValue {
					t.Errorf("expected value %q, got %q", tt.wantValue, value)
				}
			}
		})
	}
}

func TestParseImageName(t *testing.T) {
	tests := []struct {
		name         string
		imageName    string
		wantType     ImageType
		wantImage    string
		wantRegistry string
	}{
		{
			name:         "local image",
			imageName:    "local://myapp:latest",
			wantType:     ImageTypeLocal,
			wantImage:    "myapp:latest",
			wantRegistry: "",
		},
		{
			name:         "remote with full registry",
			imageName:    "quay.io/example/img:v1",
			wantType:     ImageTypeRemote,
			wantImage:    "quay.io/example/img:v1",
			wantRegistry: "quay.io",
		},
		{
			name:         "remote docker hub org/image",
			imageName:    "myorg/myapp:v1",
			wantType:     ImageTypeRemote,
			wantImage:    "myorg/myapp:v1",
			wantRegistry: "docker.io",
		},
		{
			name:         "remote docker hub library image",
			imageName:    "alpine:latest",
			wantType:     ImageTypeRemote,
			wantImage:    "alpine:latest",
			wantRegistry: "docker.io",
		},
		{
			name:         "registry with port",
			imageName:    "localhost:5000/myapp:v1",
			wantType:     ImageTypeRemote,
			wantImage:    "localhost:5000/myapp:v1",
			wantRegistry: "localhost:5000",
		},
		{
			name:         "gcr.io registry",
			imageName:    "gcr.io/project/image:tag",
			wantType:     ImageTypeRemote,
			wantImage:    "gcr.io/project/image:tag",
			wantRegistry: "gcr.io",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed := ParseImageName(tt.imageName)

			if parsed.Type != tt.wantType {
				t.Errorf("expected type %v, got %v", tt.wantType, parsed.Type)
			}
			if parsed.ImageName != tt.wantImage {
				t.Errorf("expected ImageName %q, got %q", tt.wantImage, parsed.ImageName)
			}
			if parsed.Registry != tt.wantRegistry {
				t.Errorf("expected Registry %q, got %q", tt.wantRegistry, parsed.Registry)
			}
			if parsed.OriginalName != tt.imageName {
				t.Errorf("expected OriginalName %q, got %q", tt.imageName, parsed.OriginalName)
			}
		})
	}
}
