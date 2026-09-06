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

package engineframework

import (
	"fmt"
	"time"

	"github.com/alexandremahdhaoui/forge/internal/gitutil"
	"github.com/alexandremahdhaoui/forge/pkg/forge"
)

// GetGitVersion returns the current git commit hash.
// Returns the commit SHA and nil error on success.
// Returns "unknown" and an error if git operations fail.
//
// Example:
//
//	version, err := GetGitVersion()
//	if err != nil {
//	    log.Printf("Warning: could not get git version: %v", err)
//	    version = "unknown"
//	}
//	fmt.Printf("Building version: %s\n", version)
func GetGitVersion() (string, error) {
	commitSHA, err := gitutil.GetCurrentCommitSHA()
	if err != nil {
		return "unknown", fmt.Errorf("failed to get git commit SHA: %w", err)
	}
	return commitSHA, nil
}

// CreateVersionedArtifact creates an artifact with git version and current timestamp.
// The version field is populated with the current git commit hash.
// The timestamp field is set to the current time in RFC3339 format.
//
// Parameters:
//   - name: Artifact name (from BuildInput.Name)
//   - artifactType: Type of artifact (e.g., "binary", "container", "generated")
//   - location: Location of the artifact (path or registry URL)
//
// Returns:
//   - *forge.Artifact with Name, Type, Location, Version (git SHA), and Timestamp set
//   - error if git version cannot be determined
//
// Example:
//
//	artifact, err := CreateVersionedArtifact("my-app", "binary", "./build/bin/my-app")
//	if err != nil {
//	    return nil, fmt.Errorf("failed to create artifact: %w", err)
//	}
//	// artifact.Version = "a1b2c3d4..." (git commit SHA)
//	// artifact.Timestamp = "2025-01-15T10:30:00Z" (current time)
func CreateVersionedArtifact(name string, artifactType forge.ArtifactType, location string, platform ...string) (*forge.Artifact, error) {
	version, err := GetGitVersion()
	if err != nil {
		return nil, err
	}

	artifact := &forge.Artifact{
		Name:      name,
		Type:      artifactType,
		Location:  location,
		Version:   version,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	if err := setPlatform(artifact, platform); err != nil {
		return nil, err
	}

	return artifact, nil
}

// setPlatform records the one os/arch pair an artifact was built for, when
// the engine names one. A generator names none and the artifact carries no
// platform.
func setPlatform(artifact *forge.Artifact, platform []string) error {
	if len(platform) == 0 {
		return nil
	}

	if len(platform) > 1 {
		return fmt.Errorf("artifact %s: one platform per artifact, got %d", artifact.Name, len(platform))
	}

	os, arch, err := forge.SplitPlatform(platform[0])
	if err != nil {
		return fmt.Errorf("artifact %s: %w", artifact.Name, err)
	}

	artifact.OS, artifact.Arch = os, arch

	return nil
}

// CreateArtifact creates an artifact with current timestamp but NO version field.
// Use this for artifacts that don't have git versioning (e.g., generated code, test reports).
//
// Parameters:
//   - name: Artifact name (from BuildInput.Name)
//   - artifactType: Type of artifact (e.g., "generated", "test-report")
//   - location: Location of the artifact (path or directory)
//
// Returns:
//   - *forge.Artifact with Name, Type, Location, and Timestamp set
//   - Version field is empty (generated artifacts don't have versions)
//
// Example:
//
//	artifact := CreateArtifact("openapi-client", "generated", "./pkg/generated")
//	// artifact.Version = "" (empty - generated code has no version)
//	// artifact.Timestamp = "2025-01-15T10:30:00Z" (current time)
func CreateArtifact(name string, artifactType forge.ArtifactType, location string) *forge.Artifact {
	return &forge.Artifact{
		Name:      name,
		Type:      artifactType,
		Location:  location,
		Version:   "", // Empty for non-versioned artifacts
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}

// CreateCustomArtifact creates an artifact with a custom version string and current timestamp.
// Use this when you need a specific version that's not from git (e.g., semantic version, build number).
//
// Parameters:
//   - name: Artifact name (from BuildInput.Name)
//   - artifactType: Type of artifact (e.g., "binary", "container")
//   - location: Location of the artifact (path or registry URL)
//   - version: Custom version string (e.g., "v1.2.3", "build-123")
//
// Returns:
//   - *forge.Artifact with Name, Type, Location, Version (custom), and Timestamp set
//
// Example:
//
//	artifact := CreateCustomArtifact("my-app", "container", "localhost:5000/my-app:v1.2.3", "v1.2.3")
//	// artifact.Version = "v1.2.3" (custom version)
//	// artifact.Timestamp = "2025-01-15T10:30:00Z" (current time)
func CreateCustomArtifact(name string, artifactType forge.ArtifactType, location, version string) *forge.Artifact {
	return &forge.Artifact{
		Name:      name,
		Type:      artifactType,
		Location:  location,
		Version:   version,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}

// One wraps the single artifact a host-only engine builds into the list a
// build answers.
func One(artifact *forge.Artifact) []forge.Artifact {
	return []forge.Artifact{*artifact}
}
