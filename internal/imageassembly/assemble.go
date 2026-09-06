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

// Package imageassembly builds a container image by appending one layer of
// prebuilt files onto a base, with no daemon, no Containerfile and no
// shellout. It is layer assembly rather than a build: there are no RUN steps,
// so everything a Containerfile builder exists to support is absent, and
// multi-architecture costs one layer set per architecture plus an index.
package imageassembly

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

// fixedTime is the modification time every file in the layer carries, so the
// same inputs assemble to the same digest. A layer whose digest moves on
// every build defeats the whole point of content addressing.
var fixedTime = time.Unix(0, 0).UTC()

var (
	// ErrNoFiles means the globs matched nothing. An image that silently
	// ships empty is worse than one that fails, because the failure surfaces
	// on the runner that tries to use it, days later.
	ErrNoFiles = errors.New("no files matched")
	// ErrCollision means two files would land under one name in the image.
	ErrCollision = errors.New("two files claim the same name in the image")
	// ErrPlatform means a platform string is not os/arch.
	ErrPlatform = errors.New("a platform must be os/arch")
)

// Platform is one os/arch an image is assembled for.
type Platform struct {
	OS   string
	Arch string
}

func (p Platform) String() string { return p.OS + "/" + p.Arch }

// ParsePlatform reads "linux/amd64". Nothing else is accepted, because a
// platform this cannot read would be assembled under the host's own and the
// image would run on the wrong machine.
func ParsePlatform(s string) (Platform, error) {
	// Exactly two parts. "linux/amd64/v2" names a variant, which this does
	// not carry, and reading it as an architecture called "amd64/v2" would
	// assemble under something no registry serves.
	os, arch, ok := strings.Cut(strings.TrimSpace(s), "/")
	if !ok || os == "" || arch == "" || strings.Contains(arch, "/") {
		return Platform{}, fmt.Errorf("%w: %q", ErrPlatform, s)
	}

	return Platform{OS: os, Arch: arch}, nil
}

// Request is one image to assemble.
type Request struct {
	// Base is a reference like "debian:stable-slim", or "scratch" for none.
	// A static binary runs on scratch; the base exists for whatever else
	// runs alongside it, which for a CI job container is a shell.
	Base string

	// BinDir is where the files land inside the image.
	BinDir string

	// Files maps a platform to the files that go into its layer, each under
	// the name the caller gives it. Every platform named here becomes one
	// manifest in the index.
	Files map[Platform][]File

	// Env is added to the image config. PATH is handled separately, so that
	// BinDir is always reachable.
	Env map[string]string

	// Labels is added to the image config.
	Labels map[string]string
}

// Puller fetches a base image for one platform. It is an interface so the
// assembly can be tested with no registry: pulling is the only step here that
// reaches the network.
type Puller interface {
	Pull(ref string, platform Platform) (v1.Image, error)
}

// Assembler turns a Request into images, one per platform.
type Assembler struct {
	puller Puller
}

func New(puller Puller) *Assembler {
	return &Assembler{puller: puller}
}

// File is one file of a layer: where it is on disk, and what it is called
// inside the image. The caller names it - from the artifact record for a
// built binary, from the basename for a glob - so nothing here reads a
// name out of a path.
type File struct {
	Path string
	Name string
}

// Named is the files of these paths, each under its basename.
func Named(paths ...string) []File {
	out := make([]File, 0, len(paths))
	for _, p := range paths {
		out = append(out, File{Path: p, Name: filepath.Base(p)})
	}

	return out
}

// Layer packs every path into one tar layer under binDir. One layer rather
// than one per file: a layer costs a manifest entry and a round trip, and
// nothing here is cached separately from the rest.
func Layer(binDir string, files []File) (v1.Layer, error) {
	if len(files) == 0 {
		return nil, ErrNoFiles
	}

	sorted := append([]File{}, files...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })

	var buf bytes.Buffer

	tw := tar.NewWriter(&buf)
	seen := map[string]string{}
	prefix := strings.Trim(binDir, "/")

	for _, file := range sorted {
		path := file.Path

		data, err := os.ReadFile(path) //nolint:gosec // the caller's own build output
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", path, err)
		}

		named := file.Name
		if first, clash := seen[named]; clash {
			return nil, fmt.Errorf("%w: %q comes from both %s and %s", ErrCollision, named, first, path)
		}

		seen[named] = path

		hdr := &tar.Header{
			Name: prefix + "/" + named,
			Mode: 0o755,
			Size: int64(len(data)),
			// A fixed time, so the same inputs assemble to the same digest.
			// A layer whose digest moves on every build defeats the whole
			// point of content addressing.
			ModTime: fixedTime,
		}

		if err := tw.WriteHeader(hdr); err != nil {
			return nil, fmt.Errorf("writing the header for %s: %w", named, err)
		}

		if _, err := tw.Write(data); err != nil {
			return nil, fmt.Errorf("writing %s: %w", named, err)
		}
	}

	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("closing the layer: %w", err)
	}

	body := buf.Bytes()

	return tarball.LayerFromOpener(func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(body)), nil
	})
}

// Assemble builds one image per platform in the request.
func (a *Assembler) Assemble(req Request) (map[Platform]v1.Image, error) {
	if len(req.Files) == 0 {
		return nil, ErrNoFiles
	}

	out := map[Platform]v1.Image{}

	for platform, files := range req.Files {
		img, err := a.one(req, platform, files)
		if err != nil {
			return nil, fmt.Errorf("assembling %s: %w", platform, err)
		}

		out[platform] = img
	}

	return out, nil
}

func (a *Assembler) one(req Request, platform Platform, files []File) (v1.Image, error) {
	base := v1.Image(empty.Image)

	if req.Base != "" && req.Base != "scratch" {
		pulled, err := a.puller.Pull(req.Base, platform)
		if err != nil {
			return nil, fmt.Errorf("pulling the base %q: %w", req.Base, err)
		}

		base = pulled
	}

	layer, err := Layer(req.BinDir, files)
	if err != nil {
		return nil, err
	}

	img, err := mutate.AppendLayers(base, layer)
	if err != nil {
		return nil, fmt.Errorf("appending the layer: %w", err)
	}

	cf, err := img.ConfigFile()
	if err != nil {
		return nil, fmt.Errorf("reading the base config: %w", err)
	}

	cf = cf.DeepCopy()
	cf.OS, cf.Architecture = platform.OS, platform.Arch
	cf.Config.Env = withPath(cf.Config.Env, req.BinDir, req.Env)
	cf.Config.Labels = merge(cf.Config.Labels, req.Labels)

	// No entrypoint, on purpose. This is a toolbox rather than an
	// application, and a job container that declares an entrypoint fights
	// the runner for it.
	cf.Config.Entrypoint = nil
	cf.Config.Cmd = nil

	return mutate.ConfigFile(img, cf)
}

// withPath puts binDir at the front of PATH, exactly once, keeping whatever
// the base set. A duplicated entry works and still reads like a bug to
// whoever inspects the image.
func withPath(env []string, binDir string, extra map[string]string) []string {
	rest := "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
	kept := []string{}

	for _, e := range env {
		if v, found := strings.CutPrefix(e, "PATH="); found {
			rest = v
			continue
		}

		kept = append(kept, e)
	}

	parts := []string{binDir}

	for _, dir := range strings.Split(rest, ":") {
		if dir != "" && dir != binDir {
			parts = append(parts, dir)
		}
	}

	kept = append(kept, "PATH="+strings.Join(parts, ":"))

	names := make([]string, 0, len(extra))
	for k := range extra {
		names = append(names, k)
	}

	sort.Strings(names)

	for _, k := range names {
		kept = append(kept, k+"="+extra[k])
	}

	return kept
}

func merge(base, extra map[string]string) map[string]string {
	if len(base) == 0 && len(extra) == 0 {
		return nil
	}

	out := map[string]string{}

	for k, v := range base {
		out[k] = v
	}

	for k, v := range extra {
		out[k] = v
	}

	return out
}

// Index combines per-platform images into one multi-architecture manifest, so
// a runner pulling the tag gets the architecture it is on. One image is still
// an index, because a consumer that has to know whether a tag is an index or
// an image is a consumer that will get it wrong.
func Index(images map[Platform]v1.Image) (v1.ImageIndex, error) {
	if len(images) == 0 {
		return nil, ErrNoFiles
	}

	platforms := make([]Platform, 0, len(images))
	for p := range images {
		platforms = append(platforms, p)
	}

	sort.Slice(platforms, func(i, j int) bool {
		return platforms[i].String() < platforms[j].String()
	})

	idx := v1.ImageIndex(empty.Index)
	idx = mutate.IndexMediaType(idx, types.OCIImageIndex)

	for _, p := range platforms {
		idx = mutate.AppendManifests(idx, mutate.IndexAddendum{
			Add: images[p],
			Descriptor: v1.Descriptor{
				Platform: &v1.Platform{OS: p.OS, Architecture: p.Arch},
			},
		})
	}

	return idx, nil
}

// Registry is the only thing here that reaches the outside world, and it
// holds no decision about what to publish.
type Registry interface {
	PushIndex(ref string, idx v1.ImageIndex) error
}

// Remote talks the registry HTTP API directly through go-containerregistry.
// There is no daemon, no CLI and no login step: the auth handshake is part of
// the push.
type Remote struct {
	// Token is the registry credential. GitHub Actions injects
	// secrets.GITHUB_TOKEN, which is why there is no secret to create, seal
	// or rotate.
	Token string
}

var (
	_ Registry = (*Remote)(nil)
	_ Puller   = (*Remote)(nil)
)

// options carries the credential. With no token the requests go out
// anonymous, which is enough to pull a public base and not enough to push:
// the docker config file is deliberately not consulted, because an engine
// that silently picks up whatever credential the machine happens to hold
// publishes to somewhere nobody named.
func (r *Remote) options() []remote.Option {
	if r.Token == "" {
		return []remote.Option{remote.WithAuth(authn.Anonymous)}
	}

	return []remote.Option{remote.WithAuth(authn.FromConfig(authn.AuthConfig{
		Username: "token",
		Password: r.Token,
	}))}
}

func (r *Remote) Pull(ref string, platform Platform) (v1.Image, error) {
	parsed, err := name.ParseReference(ref)
	if err != nil {
		return nil, fmt.Errorf("reading the reference %q: %w", ref, err)
	}

	opts := append(r.options(),
		remote.WithPlatform(v1.Platform{OS: platform.OS, Architecture: platform.Arch}))

	return remote.Image(parsed, opts...)
}

func (r *Remote) PushIndex(ref string, idx v1.ImageIndex) error {
	parsed, err := name.ParseReference(ref)
	if err != nil {
		return fmt.Errorf("reading the reference %q: %w", ref, err)
	}

	if err := remote.WriteIndex(parsed, idx, r.options()...); err != nil {
		return fmt.Errorf("pushing %s: %w", ref, err)
	}

	return nil
}
