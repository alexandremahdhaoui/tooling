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

package engineframework

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/alexandremahdhaoui/forge/pkg/forge"
	"github.com/alexandremahdhaoui/forge/pkg/mcptypes"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// An engine that declares nothing builds for the host and nothing else. One
// that declares any builds whatever it is handed. One that declares a list
// builds that list. The refusal names the engine, the platform and the
// declaration, so the fix is readable from the error alone.
func TestCapabilitiesDecideWhatAnEngineBuilds(t *testing.T) {
	host := HostPlatform()

	cases := []struct {
		name     string
		caps     Capabilities
		platform string
		refused  bool
	}{
		{"nothing declared builds the host", Capabilities{}, host, false},
		{"nothing declared refuses another", Capabilities{}, "plan9/mips", true},
		{"host builds the host", Capabilities{Platforms: []string{PlatformsHost}}, host, false},
		{"host refuses another", Capabilities{Platforms: []string{PlatformsHost}}, "plan9/mips", true},
		{"any builds anything", Capabilities{Platforms: []string{PlatformsAny}}, "plan9/mips", false},
		{"a list builds what it names", Capabilities{Platforms: []string{"linux/amd64", "linux/arm64"}}, "linux/arm64", false},
		{"a list refuses what it does not", Capabilities{Platforms: []string{"linux/amd64"}}, "linux/arm64", true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := RefusePlatforms("engine-x", c.caps, []string{c.platform})
			if c.refused != (err != nil) {
				t.Fatalf("refused=%v, err=%v", c.refused, err)
			}

			if err == nil {
				return
			}

			if !errors.Is(err, ErrPlatformRefused) {
				t.Fatalf("not the refusal error: %v", err)
			}

			for _, want := range []string{"engine-x", c.platform, c.caps.Declared()} {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("the refusal must name %q: %v", want, err)
				}
			}
		})
	}
}

func TestAMalformedOrMissingPlatformIsRefused(t *testing.T) {
	if err := RefusePlatforms("engine-x", Capabilities{Platforms: []string{PlatformsAny}}, []string{"linux"}); err == nil {
		t.Fatal("a platform with no arch must be refused even by an engine that builds anything")
	}

	if err := RefusePlatforms("engine-x", Capabilities{Platforms: []string{PlatformsAny}}, nil); err == nil {
		t.Fatal("a build with no platform must be refused: the core always names one")
	}
}

// The framework holds the contract before the engine's own code runs: a
// build handed a platform outside the declaration is refused, and the
// engine's function is never called.
func TestTheBuildHandlerRefusesBeforeTheEngineRuns(t *testing.T) {
	called := false

	handler := makeBuildHandler(BuilderConfig{
		Name: "engine-x",
		BuildFunc: func(context.Context, mcptypes.BuildInput) ([]forge.Artifact, error) {
			called = true

			return nil, nil
		},
	})

	result, _, err := handler(context.Background(), &mcp.CallToolRequest{}, mcptypes.BuildInput{
		Name: "thing", Engine: "forge://engine-x", Platforms: []string{"plan9/mips"},
	})
	if err != nil {
		t.Fatal(err)
	}

	if !result.IsError {
		t.Fatal("the build must be refused")
	}

	if called {
		t.Fatal("the engine's function must not run for a refused platform")
	}

	text := result.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "plan9/mips") || !strings.Contains(text, "engine-x") {
		t.Fatalf("the refusal names the platform and the engine: %s", text)
	}
}

// An artifact the engine answers with a type outside the vocabulary, or a
// platform half named, never reaches the store.
func TestTheBuildHandlerRefusesAnInvalidArtifact(t *testing.T) {
	handler := makeBuildHandler(BuilderConfig{
		Name: "engine-x",
		BuildFunc: func(context.Context, mcptypes.BuildInput) ([]forge.Artifact, error) {
			return []forge.Artifact{{Name: "thing", Type: "mystery", Location: "/x"}}, nil
		},
	})

	result, _, err := handler(context.Background(), &mcp.CallToolRequest{}, mcptypes.BuildInput{
		Name: "thing", Engine: "forge://engine-x", Platforms: []string{HostPlatform()},
	})
	if err != nil {
		t.Fatal(err)
	}

	if !result.IsError {
		t.Fatal("an artifact of an unknown type must be refused")
	}
}

// Frozen reaches only an engine that declared it reads one; handed to any
// other it is refused by name, never ignored.
func TestFrozenIsRefusedUnlessDeclared(t *testing.T) {
	if err := RefuseFrozen("go-format", Capabilities{}, true); err == nil || !strings.Contains(err.Error(), "go-format") {
		t.Fatalf("an undeclared frozen must be refused naming the engine, got %v", err)
	}

	if err := RefuseFrozen("go-format", Capabilities{}, false); err != nil {
		t.Fatalf("frozen false is nothing to refuse, got %v", err)
	}

	if err := RefuseFrozen("go-build", Capabilities{Frozen: true}, true); err != nil {
		t.Fatalf("a declared frozen is admitted, got %v", err)
	}

	decl := Capabilities{Platforms: []string{"any"}, Frozen: true}.Declaration()
	if decl["frozen"] != true || !reflect.DeepEqual(decl["platforms"], []string{"any"}) {
		t.Fatalf("the declaration must carry both, got %v", decl)
	}

	if got := (Capabilities{}).Declaration()["platforms"]; !reflect.DeepEqual(got, []string{PlatformsHost}) {
		t.Fatalf("declaring nothing answers host, got %v", got)
	}
}
