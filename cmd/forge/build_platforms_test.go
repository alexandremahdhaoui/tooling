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

//go:build unit

package main

import (
	"testing"

	"github.com/alexandremahdhaoui/forge/pkg/forge"
)

// An entry builds what it declares. Declaring platforms is the whole of
// what makes an artifact public: a repo's own tool that declares none builds
// for this machine and never travels by accident. Live case: two repos each
// shipped their own cmd/docgen into one release because the dist step
// globbed cmd/* instead of reading a declaration.
func TestAnEntryBuildsThePlatformsItDeclares(t *testing.T) {
	public := forge.BuildSpec{Name: "forge", Platforms: []string{"linux/amd64", "linux/arm64"}}
	own := forge.BuildSpec{Name: "docgen"}

	got := platformsFor(public, nil)
	if len(got) != 2 || got[0] != "linux/amd64" || got[1] != "linux/arm64" {
		t.Fatalf("a public entry builds every platform it declares, got %v", got)
	}

	got = platformsFor(own, nil)
	if len(got) != 1 || got[0] != hostPlatform() {
		t.Fatalf("an entry that declares nothing builds for the host, got %v", got)
	}
}

// The flag narrows; it never widens. Asking for a platform an entry never
// declared builds nothing for that entry rather than guessing.
func TestTheFlagIsASubsetOfTheDeclaration(t *testing.T) {
	spec := forge.BuildSpec{Name: "forge", Platforms: []string{"linux/amd64", "linux/arm64"}}

	got := platformsFor(spec, []string{"linux/arm64"})
	if len(got) != 1 || got[0] != "linux/arm64" {
		t.Fatalf("got %v, want the one declared platform asked for", got)
	}

	if got := platformsFor(spec, []string{"darwin/arm64"}); len(got) != 0 {
		t.Fatalf("an undeclared platform must build nothing, got %v", got)
	}

	own := forge.BuildSpec{Name: "docgen"}
	if got := platformsFor(own, []string{"linux/arm64"}); len(got) != 0 {
		t.Fatalf("a host-only entry is left home by a filter naming another platform, got %v", got)
	}

	// And by a filter naming the host itself: the selection is over what an
	// entry declared, and this one declared nothing. A distribution build on
	// a linux/amd64 runner must not sweep every host-only tool - or the
	// fixture image that wants a daemon - into the release.
	if got := platformsFor(own, []string{hostPlatform()}); len(got) != 0 {
		t.Fatalf("a host-only entry is outside any platform selection, got %v", got)
	}
}

func TestThePlatformsFlagIsParsedEitherWay(t *testing.T) {
	for _, args := range [][]string{
		{"--platforms", "linux/amd64,linux/arm64"},
		{"--platforms=linux/amd64,linux/arm64"},
	} {
		rest, platforms, err := parsePlatformsFlag(args)
		if err != nil {
			t.Fatalf("%v: %v", args, err)
		}

		if len(rest) != 0 {
			t.Fatalf("%v: the flag must not survive as an artifact name: %v", args, rest)
		}

		if len(platforms) != 2 || platforms[0] != "linux/amd64" || platforms[1] != "linux/arm64" {
			t.Fatalf("%v: got %v", args, platforms)
		}
	}

	if _, _, err := parsePlatformsFlag([]string{"--platforms"}); err == nil {
		t.Fatal("a flag with no list must fail loudly")
	}
}

// Freshness is per platform: a fresh host binary says nothing about the
// arm64 one, so an entry whose arm64 record is missing rebuilds.
func TestFreshnessIsPerPlatform(t *testing.T) {
	store := forge.ArtifactStore{Artifacts: []forge.Artifact{{
		Name: "forge", Type: forge.TypeBinary, OS: "linux", Arch: "amd64",
		Location: "/nowhere", Timestamp: "2026-01-01T00:00:00Z",
	}}}

	rebuild, reason, err := shouldRebuild("forge", []string{"linux/amd64", "linux/arm64"}, store, false)
	if err != nil {
		t.Fatal(err)
	}

	if !rebuild {
		t.Fatal("a platform with no record must rebuild")
	}

	if reason == "" {
		t.Fatal("the reason names what was missing")
	}
}
