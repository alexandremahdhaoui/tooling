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

package forge

import (
	"errors"
	"fmt"
	"strings"
)

// ArtifactType is what an artifact is, as a closed vocabulary. Every
// consumer that once compared a free string - the release side selecting
// what uploads, the compute side deciding what travels between jobs, the
// store keying what it prunes - reads one of these instead, so a typo in
// an engine is a build failure here and not a silent no-op three tools
// downstream.
type ArtifactType string

const (
	// TypeBinary is an executable file built for one platform.
	TypeBinary ArtifactType = "binary"
	// TypeContainer is an OCI image layout on disk, one manifest per
	// platform, that a release pushes to a registry.
	TypeContainer ArtifactType = "container"
	// TypeGenerated is source code a generator wrote into the repository.
	TypeGenerated ArtifactType = "generated"
	// TypeCommandOutput is the record of a command that ran and left no
	// file a release could publish.
	TypeCommandOutput ArtifactType = "command-output"
	// TypeBPF is a compiled BPF object.
	TypeBPF ArtifactType = "bpf"
	// TypeProtobuf is code generated from protobuf definitions.
	TypeProtobuf ArtifactType = "protobuf"
)

// ArtifactTypes is every type an artifact may carry.
var ArtifactTypes = []ArtifactType{
	TypeBinary, TypeContainer, TypeGenerated, TypeCommandOutput, TypeBPF, TypeProtobuf,
}

var (
	// ErrArtifactType means an artifact carries a type outside the vocabulary.
	ErrArtifactType = errors.New("artifact type is not one of the known types")
	// ErrArtifactPlatform means an artifact names an os without an arch or
	// the other way round; a platform is both or neither.
	ErrArtifactPlatform = errors.New("artifact platform must name both os and arch, or neither")
	// ErrPlatform means a string is not an os/arch pair.
	ErrPlatform = errors.New("platform must be <os>/<arch>")
)

// Validate refuses a type outside the vocabulary.
func (t ArtifactType) Validate() error {
	for _, known := range ArtifactTypes {
		if t == known {
			return nil
		}
	}

	return fmt.Errorf("%w: %q", ErrArtifactType, string(t))
}

// Platform is the os/arch pair an artifact was built for, or empty when the
// artifact is not tied to one - generated code, a command's output, a
// multi-architecture image layout.
func (a Artifact) Platform() string {
	if a.OS == "" && a.Arch == "" {
		return ""
	}

	return a.OS + "/" + a.Arch
}

// SplitPlatform reads an <os>/<arch> pair. It is the one place the shape is
// parsed; nothing parses a platform out of a file name.
func SplitPlatform(platform string) (os, arch string, err error) {
	parts := strings.Split(platform, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("%w: %q", ErrPlatform, platform)
	}

	return parts[0], parts[1], nil
}
