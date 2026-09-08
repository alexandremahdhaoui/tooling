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

// ValidateValueFrom validates a ValueFrom struct.
// It checks that exactly one of EnvName or Literal is set.
// IMPORTANT: Empty string in Literal field is treated as NOT SET (validation-rules.txt requires non-empty).
// fieldName is used for error context (e.g., "username", "password").
func validateValueFrom(vf ValueFrom, fieldName string) error {
	hasEnv := vf.EnvName != ""
	hasLit := vf.Literal != "" // Empty string means NOT set (per validation-rules.txt line 50)

	if hasEnv && hasLit {
		return fmt.Errorf("%s: cannot specify both envName and literal", fieldName)
	}
	if !hasEnv && !hasLit {
		return fmt.Errorf("%s: must specify either envName or literal", fieldName)
	}

	return nil
}

// ValidateBasicAuth validates a BasicAuth struct.
// It validates both Username and Password ValueFrom fields.
func validateBasicAuth(auth BasicAuth) error {
	if err := validateValueFrom(auth.Username, "username"); err != nil {
		return err
	}
	if err := validateValueFrom(auth.Password, "password"); err != nil {
		return err
	}
	return nil
}

// ValidateImageSource validates an ImageSource struct.
func validateImageSource(img ImageSource) error {
	// 1. Name must not be empty
	if img.Name == "" {
		return fmt.Errorf("name must not be empty")
	}

	// 2. Local image validation
	if strings.HasPrefix(img.Name, "local://") {
		after := strings.TrimPrefix(img.Name, "local://")
		if after == "" {
			return fmt.Errorf("local:// prefix requires image name")
		}
		if !strings.Contains(after, ":") {
			return fmt.Errorf("local:// image must include tag (e.g., local://myapp:v1)")
		}
		namePart := strings.Split(after, ":")[0]
		if strings.Contains(namePart, "/") || strings.Contains(namePart, ".") {
			return fmt.Errorf("local:// image name must not contain registry domain or slashes")
		}
	} else {
		// 3. Remote image validation - must include tag
		if !strings.Contains(img.Name, ":") {
			return fmt.Errorf("remote image must include tag (no :latest inference)")
		}
	}

	// 4. Validate BasicAuth if present
	if hasBasicAuth(img) {
		if err := validateBasicAuth(img.BasicAuth); err != nil {
			return fmt.Errorf("basicAuth: %w", err)
		}
	}

	return nil
}

// ValidateImages validates a slice of ImageSource structs.
// It validates each ImageSource and checks for duplicates.
func ValidateImages(images []ImageSource) error {
	seen := make(map[string]bool)

	for i, img := range images {
		// Check for duplicates
		if seen[img.Name] {
			return fmt.Errorf("duplicate image in images: %q", img.Name)
		}
		seen[img.Name] = true

		// Validate each ImageSource
		if err := validateImageSource(img); err != nil {
			return fmt.Errorf("images[%d]: %w", i, err)
		}
	}

	return nil
}

// ResolveValueFrom resolves a ValueFrom to its actual string value.
// For EnvName, it reads from the environment.
// For Literal, it returns the value directly.
func ResolveValueFrom(vf ValueFrom, fieldName string) (string, error) {
	if vf.EnvName != "" {
		value, exists := os.LookupEnv(vf.EnvName)
		if !exists {
			return "", fmt.Errorf("%s: environment variable %q not set", fieldName, vf.EnvName)
		}
		if value == "" {
			return "", fmt.Errorf("%s: environment variable %q is empty", fieldName, vf.EnvName)
		}
		return value, nil
	}
	if vf.Literal != "" {
		return vf.Literal, nil
	}
	return "", fmt.Errorf("%s: neither envName nor literal set", fieldName)
}

// ImageType represents the type of image source.
type ImageType int

const (
	ImageTypeLocal  ImageType = iota // local:// prefix, already in daemon
	ImageTypeRemote                  // remote registry, needs pull
)

// ParsedImage contains parsed image information.
type ParsedImage struct {
	Type         ImageType // LOCAL or REMOTE
	OriginalName string    // Original name as specified
	ImageName    string    // Name without local:// prefix (for local) or as-is (for remote)
	Registry     string    // Registry domain (for remote images, empty for local)
}

// ParseImageName parses an image name and determines its type.
// Local images: local://name:tag -> strips prefix
// Remote images: registry/path:tag or name:tag -> used as-is
func ParseImageName(name string) ParsedImage {
	if strings.HasPrefix(name, "local://") {
		imageName := strings.TrimPrefix(name, "local://")
		return ParsedImage{
			Type:         ImageTypeLocal,
			OriginalName: name,
			ImageName:    imageName,
			Registry:     "",
		}
	}

	// Remote image - extract registry
	registry := ""
	if strings.Contains(name, "/") {
		parts := strings.SplitN(name, "/", 2)
		// Check if first part looks like a registry (contains "." or ":")
		if strings.Contains(parts[0], ".") || strings.Contains(parts[0], ":") {
			registry = parts[0]
		} else {
			// Docker Hub org/image format
			registry = "docker.io"
		}
	} else {
		// Simple name:tag format (Docker Hub library image)
		registry = "docker.io"
	}

	return ParsedImage{
		Type:         ImageTypeRemote,
		OriginalName: name,
		ImageName:    name,
		Registry:     registry,
	}
}
