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
	"strings"
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

	got := platformsFor(public)
	if len(got) != 2 || got[0] != "linux/amd64" || got[1] != "linux/arm64" {
		t.Fatalf("a public entry builds every platform it declares, got %v", got)
	}

	got = platformsFor(own)
	if len(got) != 1 || got[0] != hostPlatform() {
		t.Fatalf("an entry that declares nothing builds for the host, got %v", got)
	}
}

// There is no flag. A build takes no policy on its command line: what an
// entry builds is declared in forge.yaml, and a flag that once narrowed the
// declaration put the policy back into argv of every pipeline that typed
// it. An unknown flag is refused by name rather than read as an artifact.
func TestABuildTakesNoFlagThatCarriesPolicy(t *testing.T) {
	for _, args := range [][]string{
		{"--platforms", "linux/amd64,linux/arm64"},
		{"--platforms=linux/amd64"},
		{"--frozen"},
	} {
		err := runBuild(args, false)
		if err == nil || !strings.Contains(err.Error(), args[0]) {
			t.Fatalf("%v: must be refused naming the flag, got %v", args, err)
		}
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
