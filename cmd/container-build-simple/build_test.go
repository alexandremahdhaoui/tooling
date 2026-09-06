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

package main

import (
	"archive/tar"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/alexandremahdhaoui/forge/pkg/forge"
	"github.com/alexandremahdhaoui/forge/pkg/mcptypes"
)

// repo is a member checkout the way a build leaves it: a forge.yaml, and an
// artifact store recording one binary per name per platform. The files on
// disk carry the go-build convention, and nothing in the engine reads it -
// the record is what says which platform a file is for.
func repo(t *testing.T, root, name string, names []string, platforms ...string) string {
	t.Helper()

	dir := filepath.Join(root, name)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "build", "bin"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "forge.yaml"),
		[]byte("artifactStorePath: .forge/artifact-store.yaml\n"), 0o600))

	store := forge.ArtifactStore{Version: "1.0"}

	for _, n := range names {
		for _, p := range platforms {
			goos, goarch, err := forge.SplitPlatform(p)
			require.NoError(t, err)

			file := filepath.Join("build", "bin", n+"_"+goos+"_"+goarch)
			require.NoError(t, os.WriteFile(filepath.Join(dir, file),
				[]byte("#!/bin/sh\necho "+n+" "+p+"\n"), 0o600))

			store.Artifacts = append(store.Artifacts, forge.Artifact{
				Name: n, Type: forge.TypeBinary, OS: goos, Arch: goarch,
				Location:  file,
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				Version:   "abc",
			})
		}
	}

	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".forge"), 0o750))
	require.NoError(t, forge.WriteArtifactStore(filepath.Join(dir, ".forge", "artifact-store.yaml"), store))

	return dir
}

// filesIn reads the image filesystem the way a runtime would.
func filesIn(t *testing.T, img v1.Image) []string {
	t.Helper()

	rc := mutate.Extract(img)
	defer rc.Close()

	out := []string{}
	tr := tar.NewReader(rc)

	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}

		require.NoError(t, err)

		if h.Typeflag == tar.TypeReg {
			out = append(out, h.Name)
		}
	}

	return out
}

func build(t *testing.T, root string, platforms []string, spec *Spec) (forge.Artifact, error) {
	t.Helper()

	artifacts, err := Build(context.Background(), mcptypes.BuildInput{
		Name: "toolchain", Context: root, Dest: "./build/images", Platforms: platforms,
	}, spec)
	if err != nil {
		return forge.Artifact{}, err
	}

	require.Len(t, artifacts, 1, "one layout, whatever the platform count")

	return artifacts[0], nil
}

// The whole engine, on two member repos: assemble both architectures from
// their records, write the layout, and read back what each one actually
// holds. scratch keeps it off the network, which is the only thing a base
// adds here.
func TestBuildWritesALayoutHoldingEachPlatformsOwnBinaries(t *testing.T) {
	root := t.TempDir()
	repo(t, root, "forge", []string{"forge"}, "linux/amd64", "linux/arm64")
	repo(t, root, "forge-ci", []string{"forge-ci"}, "linux/amd64", "linux/arm64")

	artifact, err := build(t, root, []string{"linux/amd64", "linux/arm64"}, &Spec{
		Base:   "scratch",
		BinDir: "/usr/local/bin",
		From:   []string{"forge", "forge-ci"},
		Labels: map[string]string{"org.opencontainers.image.source": "https://example.com/forge"},
	})
	require.NoError(t, err)

	require.Equal(t, forge.TypeContainer, artifact.Type,
		"the release side filters on the type, so a container never travels the binary path")
	require.Equal(t, "toolchain", artifact.Name)
	require.Empty(t, artifact.Platform(), "a multi-architecture layout is tied to no one platform")

	path := strings.TrimPrefix(artifact.Location, "file://")
	require.DirExists(t, path)
	require.Contains(t, artifact.Version, "sha256:", "the version is the digest nobody has to trust")

	idx, err := layout.ImageIndexFromPath(path)
	require.NoError(t, err)

	manifest, err := idx.IndexManifest()
	require.NoError(t, err)
	require.Len(t, manifest.Manifests, 2, "both architectures ride one index")

	for _, d := range manifest.Manifests {
		img, err := idx.Image(d.Digest)
		require.NoError(t, err)

		cf, err := img.ConfigFile()
		require.NoError(t, err)

		files := filesIn(t, img)
		assert.ElementsMatch(t,
			[]string{"usr/local/bin/forge", "usr/local/bin/forge-ci"}, files,
			"%s carries every tool, under its artifact name", cf.Architecture)

		assert.Equal(t, "https://example.com/forge",
			cf.Config.Labels["org.opencontainers.image.source"])
	}
}

// A repo's store holds every platform the repo cross-built. An image asked
// for one of them carries that one, and the rest are not an error.
func TestAPlatformNobodyDeclaredIsSkippedRatherThanRefused(t *testing.T) {
	root := t.TempDir()
	repo(t, root, "forge", []string{"forge"}, "linux/amd64", "linux/arm64", "darwin/arm64")

	artifact, err := build(t, root, []string{"linux/amd64"}, &Spec{
		Base: "scratch", BinDir: "/usr/local/bin", From: []string{"forge"},
	})
	require.NoError(t, err)

	idx, err := layout.ImageIndexFromPath(strings.TrimPrefix(artifact.Location, "file://"))
	require.NoError(t, err)

	manifest, err := idx.IndexManifest()
	require.NoError(t, err)
	require.Len(t, manifest.Manifests, 1)
}

// A glob's file is the same on every architecture, so it goes into all of
// them, under its own name.
func TestAGlobbedFileGoesIntoEveryPlatform(t *testing.T) {
	root := t.TempDir()
	repo(t, root, "forge", []string{"forge"}, "linux/amd64", "linux/arm64")
	require.NoError(t, os.MkdirAll(filepath.Join(root, "extras"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(root, "extras", "entrypoint.sh"), []byte("#!/bin/sh\n"), 0o600))

	artifact, err := build(t, root, []string{"linux/amd64", "linux/arm64"}, &Spec{
		Base: "scratch", BinDir: "/usr/local/bin", From: []string{"forge", "extras/*"},
	})
	require.NoError(t, err)

	idx, err := layout.ImageIndexFromPath(strings.TrimPrefix(artifact.Location, "file://"))
	require.NoError(t, err)

	manifest, err := idx.IndexManifest()
	require.NoError(t, err)

	for _, d := range manifest.Manifests {
		img, err := idx.Image(d.Digest)
		require.NoError(t, err)
		assert.Contains(t, filesIn(t, img), "usr/local/bin/entrypoint.sh")
	}
}

// One glob quietly matching nothing while the others matched is exactly how
// an image ships missing a tool, so each glob is checked on its own.
func TestAGlobThatMatchesNothingFailsEvenWhenTheOthersMatched(t *testing.T) {
	root := t.TempDir()
	repo(t, root, "forge", []string{"forge"}, "linux/amd64")

	_, err := build(t, root, []string{"linux/amd64"}, &Spec{
		Base: "scratch", BinDir: "/usr/local/bin", From: []string{"forge", "build/nothing/*"},
	})
	require.ErrorIs(t, err, ErrNoMatch)
	require.Contains(t, err.Error(), "build/nothing/*", "the glob that failed is named")
}

// An architecture in the index with no tools in it is an image that fails on
// whichever runner happens to be on that architecture. A repo that recorded
// nothing for a declared platform is refused by name.
func TestADeclaredPlatformNoRecordCoversIsRefused(t *testing.T) {
	root := t.TempDir()
	repo(t, root, "forge", []string{"forge"}, "linux/amd64")

	_, err := build(t, root, []string{"linux/amd64", "linux/arm64"}, &Spec{
		Base: "scratch", BinDir: "/usr/local/bin", From: []string{"forge"},
	})
	require.ErrorIs(t, err, ErrNoRecord)
	require.Contains(t, err.Error(), "linux/arm64")
}

// A record that names a file no longer on disk is a stale store, and an
// image assembled from it would silently miss the tool.
func TestARecordWhoseFileIsGoneIsRefused(t *testing.T) {
	root := t.TempDir()
	dir := repo(t, root, "forge", []string{"forge"}, "linux/amd64")
	require.NoError(t, os.Remove(filepath.Join(dir, "build", "bin", "forge_linux_amd64")))

	_, err := build(t, root, []string{"linux/amd64"}, &Spec{
		Base: "scratch", BinDir: "/usr/local/bin", From: []string{"forge"},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "forge_linux_amd64")
}

// A stale layout merged into rather than replaced would carry manifests from
// a build nobody asked to keep, and the published image would hold an
// architecture that no longer exists in the source.
func TestARebuildReplacesTheLayoutRatherThanMergingIntoIt(t *testing.T) {
	root := t.TempDir()
	repo(t, root, "forge", []string{"forge"}, "linux/amd64", "linux/arm64")

	first, err := build(t, root, []string{"linux/amd64", "linux/arm64"}, &Spec{
		Base: "scratch", BinDir: "/usr/local/bin", From: []string{"forge"},
	})
	require.NoError(t, err)

	second, err := build(t, root, []string{"linux/amd64"}, &Spec{
		Base: "scratch", BinDir: "/usr/local/bin", From: []string{"forge"},
	})
	require.NoError(t, err)
	require.Equal(t, first.Location, second.Location)

	idx, err := layout.ImageIndexFromPath(strings.TrimPrefix(second.Location, "file://"))
	require.NoError(t, err)

	manifest, err := idx.IndexManifest()
	require.NoError(t, err)
	require.Len(t, manifest.Manifests, 1, "the arm64 manifest from the first build must be gone")
}

// The same inputs must produce the same digest, or every run republishes an
// image nothing changed in.
func TestTheSameTreeBuildsTheSameDigest(t *testing.T) {
	root := t.TempDir()
	repo(t, root, "forge", []string{"forge", "forge-ci"}, "linux/amd64")

	spec := func() *Spec {
		return &Spec{Base: "scratch", BinDir: "/usr/local/bin", From: []string{"forge"}}
	}

	first, err := Build(context.Background(), mcptypes.BuildInput{Name: "a", Context: root, Platforms: []string{"linux/amd64"}}, spec())
	require.NoError(t, err)

	second, err := Build(context.Background(), mcptypes.BuildInput{Name: "b", Context: root, Platforms: []string{"linux/amd64"}}, spec())
	require.NoError(t, err)

	assert.Equal(t, first[0].Version, second[0].Version)
}

func TestAPlatformItCannotReadIsRefused(t *testing.T) {
	root := t.TempDir()
	repo(t, root, "forge", []string{"forge"}, "linux/amd64")

	_, err := build(t, root, []string{"linux"}, &Spec{Base: "scratch", From: []string{"forge"}})
	require.Error(t, err)
	require.Contains(t, err.Error(), "os/arch")
}
