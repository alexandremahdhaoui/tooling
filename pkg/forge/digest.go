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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// DigestPrefix names the algorithm every digest in the artifact store
// carries, so a reader never guesses.
const DigestPrefix = "sha256:"

// DigestFile answers the content digest of one file, as the store records
// it: "sha256:" and the hex of the file's bytes. Freshness is decided by
// comparing this, never by a modification time - a touch changes nothing,
// a one-byte edit changes everything, and a file that came back from a
// clone with today's date is exactly as fresh as its content says.
func DigestFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("digesting %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("digesting %s: %w", path, err)
	}

	return DigestPrefix + hex.EncodeToString(h.Sum(nil)), nil
}

// DependencyOf records one path as a dependency with its current digest.
func DependencyOf(path string) (ArtifactDependency, error) {
	digest, err := DigestFile(path)
	if err != nil {
		return ArtifactDependency{}, err
	}

	return ArtifactDependency{Path: path, Digest: digest}, nil
}
