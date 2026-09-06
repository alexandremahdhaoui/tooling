//go:build unit

/*
Copyright 2024 Alexandre Mahdhaoui

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package imageassembly_test

import (
	"archive/tar"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/alexandremahdhaoui/forge/internal/imageassembly"
)

var (
	amd64 = imageassembly.Platform{OS: "linux", Arch: "amd64"}
	arm64 = imageassembly.Platform{OS: "linux", Arch: "arm64"}
)

// fakeBase is a base that never reaches the network. It carries a PATH and a
// label so the tests can prove the base's own config survives.
type fakeBase struct {
	pulled []string
}

func (f *fakeBase) Pull(ref string, platform imageassembly.Platform) (v1.Image, error) {
	f.pulled = append(f.pulled, ref+" "+platform.String())

	cf, err := empty.Image.ConfigFile()
	if err != nil {
		return nil, err
	}

	cf = cf.DeepCopy()
	cf.Config.Env = []string{"PATH=/usr/bin:/bin", "LANG=C.UTF-8"}
	cf.Config.Labels = map[string]string{"org.opencontainers.image.base.name": ref}

	return mutate.ConfigFile(empty.Image, cf)
}

func binaries(t *testing.T, names ...string) []string {
	t.Helper()

	dir := t.TempDir()
	out := make([]string, 0, len(names))

	for _, n := range names {
		p := filepath.Join(dir, n)
		require.NoError(t, os.WriteFile(p, []byte("#!/bin/sh\necho "+n+"\n"), 0o600))
		out = append(out, p)
	}

	return out
}

// entries reads what actually landed in the image filesystem, which is the
// only question that matters: a layer that assembled cleanly and holds
// nothing is an image that fails on the runner days later.
func entries(t *testing.T, img v1.Image) map[string]int64 {
	t.Helper()

	rc := mutate.Extract(img)
	defer rc.Close()

	out := map[string]int64{}
	tr := tar.NewReader(rc)

	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}

		require.NoError(t, err)

		if h.Typeflag == tar.TypeReg {
			out[h.Name] = h.Mode
		}
	}

	return out
}

func TestTheBinariesLandUnderBinDirAndAreExecutable(t *testing.T) {
	t.Parallel()

	paths := binaries(t, "forge", "forge-ci")

	images, err := imageassembly.New(&fakeBase{}).Assemble(imageassembly.Request{
		Base:   "debian:stable-slim",
		BinDir: "/usr/local/bin",
		Files:  map[imageassembly.Platform][]imageassembly.File{amd64: imageassembly.Named(paths...)},
	})
	require.NoError(t, err)
	require.Len(t, images, 1)

	got := entries(t, images[amd64])
	require.Equal(t, int64(0o755), got["usr/local/bin/forge"], "a file nobody can execute is not a tool")
	require.Equal(t, int64(0o755), got["usr/local/bin/forge-ci"])
}

// A file lands under the name it is given, whatever it was called on disk:
// a cross-built binary is recorded as forge-ci and stored as
// forge-ci_linux_arm64, and the record's name is the one a script inside the
// image calls.
func TestAFileLandsUnderTheNameItIsGiven(t *testing.T) {
	t.Parallel()

	paths := binaries(t, "forge-ci_linux_arm64")

	images, err := imageassembly.New(&fakeBase{}).Assemble(imageassembly.Request{
		BinDir: "/usr/local/bin",
		Base:   "scratch",
		Files:  map[imageassembly.Platform][]imageassembly.File{arm64: {{Path: paths[0], Name: "forge-ci"}}},
	})
	require.NoError(t, err)

	got := entries(t, images[arm64])
	require.Contains(t, got, "usr/local/bin/forge-ci")
	require.NotContains(t, got, "usr/local/bin/forge-ci_linux_arm64")
}

// Two files landing under one name would overwrite each other silently, and
// the image would ship whichever the tar writer saw last.
func TestTwoFilesUnderOneNameIsRefused(t *testing.T) {
	t.Parallel()

	dirA, dirB := t.TempDir(), t.TempDir()
	a := filepath.Join(dirA, "forge-ci_linux_amd64")
	b := filepath.Join(dirB, "forge-ci")

	require.NoError(t, os.WriteFile(a, []byte("a"), 0o600))
	require.NoError(t, os.WriteFile(b, []byte("b"), 0o600))

	_, err := imageassembly.Layer("/usr/local/bin", []imageassembly.File{{Path: a, Name: "forge-ci"}, {Path: b, Name: "forge-ci"}})
	require.ErrorIs(t, err, imageassembly.ErrCollision)
	require.Contains(t, err.Error(), a)
	require.Contains(t, err.Error(), b, "both sources are named, because this is a declaration mistake")
}

// An image that silently ships empty fails on the runner that tries to use
// it, days later and far from the cause.
func TestAnEmptyLayerIsRefused(t *testing.T) {
	t.Parallel()

	_, err := imageassembly.Layer("/usr/local/bin", nil)
	require.ErrorIs(t, err, imageassembly.ErrNoFiles)

	_, err = imageassembly.New(&fakeBase{}).Assemble(imageassembly.Request{BinDir: "/usr/local/bin"})
	require.ErrorIs(t, err, imageassembly.ErrNoFiles)
}

// The same inputs must assemble to the same digest. A layer whose digest
// moves on every build defeats content addressing, and every run would
// republish an image nothing changed in.
func TestTheSameInputsAssembleToTheSameDigest(t *testing.T) {
	t.Parallel()

	paths := binaries(t, "forge", "forge-ci")

	first, err := imageassembly.Layer("/usr/local/bin", imageassembly.Named(paths...))
	require.NoError(t, err)

	// Reversed, because what is in the layer is a set and not an order.
	second, err := imageassembly.Layer("/usr/local/bin", imageassembly.Named(paths[1], paths[0]))
	require.NoError(t, err)

	fd, err := first.Digest()
	require.NoError(t, err)

	sd, err := second.Digest()
	require.NoError(t, err)

	assert.Equal(t, fd, sd)
}

func TestBinDirGoesToTheFrontOfPathExactlyOnce(t *testing.T) {
	t.Parallel()

	images, err := imageassembly.New(&fakeBase{}).Assemble(imageassembly.Request{
		Base:   "debian:stable-slim",
		BinDir: "/usr/bin",
		Files:  map[imageassembly.Platform][]imageassembly.File{amd64: imageassembly.Named(binaries(t, "forge")...)},
		Env:    map[string]string{"FORGE_HOME": "/opt/forge"},
	})
	require.NoError(t, err)

	cf, err := images[amd64].ConfigFile()
	require.NoError(t, err)

	var path string

	for _, e := range cf.Config.Env {
		if v, ok := strings.CutPrefix(e, "PATH="); ok {
			path = v
		}
	}

	require.Equal(t, "/usr/bin:/bin", path,
		"the base's PATH is kept, binDir goes first, and it appears once")
	require.Contains(t, cf.Config.Env, "FORGE_HOME=/opt/forge")
	require.Contains(t, cf.Config.Env, "LANG=C.UTF-8", "the base's own environment survives")
}

// A job container that declares an entrypoint fights the runner for it, and
// the runner wins by overriding it in a way that surprises everybody.
func TestNoEntrypointIsDeclared(t *testing.T) {
	t.Parallel()

	images, err := imageassembly.New(&fakeBase{}).Assemble(imageassembly.Request{
		Base:   "scratch",
		BinDir: "/usr/local/bin",
		Files:  map[imageassembly.Platform][]imageassembly.File{amd64: imageassembly.Named(binaries(t, "forge")...)},
	})
	require.NoError(t, err)

	cf, err := images[amd64].ConfigFile()
	require.NoError(t, err)
	assert.Empty(t, cf.Config.Entrypoint)
	assert.Empty(t, cf.Config.Cmd)
}

// The base is pulled once per architecture. Pulling it once and reusing it
// would ship amd64 binaries on top of an arm64 base, or the reverse.
func TestTheBaseIsPulledForEachArchitecture(t *testing.T) {
	t.Parallel()

	fake := &fakeBase{}

	_, err := imageassembly.New(fake).Assemble(imageassembly.Request{
		Base:   "debian:stable-slim",
		BinDir: "/usr/local/bin",
		Files: map[imageassembly.Platform][]imageassembly.File{
			amd64: imageassembly.Named(binaries(t, "forge")...),
			arm64: imageassembly.Named(binaries(t, "forge")...),
		},
	})
	require.NoError(t, err)

	assert.ElementsMatch(t,
		[]string{"debian:stable-slim linux/amd64", "debian:stable-slim linux/arm64"},
		fake.pulled)
}

func TestScratchPullsNothing(t *testing.T) {
	t.Parallel()

	fake := &fakeBase{}

	_, err := imageassembly.New(fake).Assemble(imageassembly.Request{
		Base:   "scratch",
		BinDir: "/usr/local/bin",
		Files:  map[imageassembly.Platform][]imageassembly.File{amd64: imageassembly.Named(binaries(t, "forge")...)},
	})
	require.NoError(t, err)
	assert.Empty(t, fake.pulled, "a static binary needs no base, and an empty base is not a pull")
}

func TestParsePlatformRefusesWhatItCannotRead(t *testing.T) {
	t.Parallel()

	for _, bad := range []string{"", "linux", "/amd64", "linux/", "linux/amd64/v2"} {
		_, err := imageassembly.ParsePlatform(bad)
		require.ErrorIs(t, err, imageassembly.ErrPlatform, "input %q", bad)
	}

	got, err := imageassembly.ParsePlatform(" linux/arm64 ")
	require.NoError(t, err)
	assert.Equal(t, arm64, got)
}

// The whole round trip against a real registry, in process: assemble, push an
// index, pull it back, and get the architecture that was asked for. This is
// what a runner does with the published tag.
func TestTheIndexRoundTripsThroughARealRegistry(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(registry.New())
	t.Cleanup(srv.Close)

	host := strings.TrimPrefix(srv.URL, "http://")

	images, err := imageassembly.New(&fakeBase{}).Assemble(imageassembly.Request{
		Base:   "scratch",
		BinDir: "/usr/local/bin",
		Files: map[imageassembly.Platform][]imageassembly.File{
			amd64: imageassembly.Named(binaries(t, "forge", "forge-ci")...),
			arm64: imageassembly.Named(binaries(t, "forge", "forge-ci")...),
		},
		Labels: map[string]string{"org.opencontainers.image.source": "https://example.com/forge"},
	})
	require.NoError(t, err)

	idx, err := imageassembly.Index(images)
	require.NoError(t, err)

	ref := host + "/forge:v0.50.0"
	require.NoError(t, (&imageassembly.Remote{}).PushIndex(ref, idx))

	parsed, err := name.ParseReference(ref)
	require.NoError(t, err)

	back, err := remote.Index(parsed)
	require.NoError(t, err)

	manifest, err := back.IndexManifest()
	require.NoError(t, err)
	require.Len(t, manifest.Manifests, 2, "both architectures ride one tag")

	// The runner picks by platform. Asking for arm64 must not answer amd64.
	got, err := remote.Image(parsed, remote.WithPlatform(v1.Platform{OS: "linux", Architecture: "arm64"}))
	require.NoError(t, err)

	cf, err := got.ConfigFile()
	require.NoError(t, err)
	assert.Equal(t, "arm64", cf.Architecture)
	assert.Equal(t, "https://example.com/forge", cf.Config.Labels["org.opencontainers.image.source"])

	assert.Contains(t, entries(t, got), "usr/local/bin/forge-ci")
}

func TestAnEmptyIndexIsRefused(t *testing.T) {
	t.Parallel()

	_, err := imageassembly.Index(nil)
	require.ErrorIs(t, err, imageassembly.ErrNoFiles)
}
