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
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/v1/layout"
	"sigs.k8s.io/yaml"

	"github.com/alexandremahdhaoui/forge/internal/imageassembly"
	"github.com/alexandremahdhaoui/forge/pkg/forge"
	"github.com/alexandremahdhaoui/forge/pkg/mcptypes"
)

var (
	// ErrNoMatch means a glob matched nothing. An image that silently ships
	// empty fails on the runner that tries to use it, days later and far
	// from the cause, so the build fails here instead.
	ErrNoMatch = errors.New("a from glob matched nothing")
	// ErrEmptyPlatform means a declared platform got no files.
	ErrEmptyPlatform = errors.New("a declared platform got no files")
	// ErrNoRecord means a repository named in from: carries no binary record
	// for a declared platform.
	ErrNoRecord = errors.New("a from repository carries no binary for a platform")
)

// entry is one file the layer carries, and the platform it belongs to: one
// platform for a binary read from a record, every platform for a glob.
type entry struct {
	path string
	// name is what the file is called inside the image: the artifact's name
	// for a record, the basename for a glob.
	name     string
	platform *imageassembly.Platform
}

// expand resolves the from: list against root. A directory holding a
// forge.yaml is a repository whose artifact store is read; the binaries it
// records for a declared platform land on that platform, by record and by
// nothing else. Anything else is a glob whose files land everywhere.
func expand(root string, from []string, platforms []imageassembly.Platform) ([]entry, error) {
	out := []entry{}
	seen := map[string]bool{}

	add := func(e entry) {
		key := e.path
		if e.platform != nil {
			key += "@" + e.platform.String()
		}

		if !seen[key] {
			seen[key] = true

			out = append(out, e)
		}
	}

	for _, item := range from {
		path := item
		if !filepath.IsAbs(path) {
			path = filepath.Join(root, item)
		}

		if info, err := os.Stat(filepath.Join(path, "forge.yaml")); err == nil && !info.IsDir() {
			records, err := recorded(path, platforms)
			if err != nil {
				return nil, fmt.Errorf("reading the records of %q: %w", item, err)
			}

			for _, r := range records {
				add(r)
			}

			continue
		}

		matches, err := filepath.Glob(path)
		if err != nil {
			return nil, fmt.Errorf("reading the glob %q: %w", item, err)
		}

		found := 0

		for _, m := range matches {
			info, err := os.Stat(m)
			if err != nil || info.IsDir() {
				continue
			}

			found++

			add(entry{path: m, name: filepath.Base(m)})
		}

		// Per glob, not overall: one glob quietly matching nothing while the
		// others matched is exactly how an image ships missing a tool.
		if found == 0 {
			return nil, fmt.Errorf("%w: %q", ErrNoMatch, item)
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].path != out[j].path {
			return out[i].path < out[j].path
		}

		return platformKey(out[i].platform) < platformKey(out[j].platform)
	})

	return out, nil
}

func platformKey(p *imageassembly.Platform) string {
	if p == nil {
		return ""
	}

	return p.String()
}

// recorded reads a repository's artifact store and answers, for each
// declared platform, the latest binary of every name it carries. A platform
// no record covers is an error: the image was declared for it and nothing
// was built for it.
func recorded(repo string, platforms []imageassembly.Platform) ([]entry, error) {
	// Only the store's path is read out of the repository's forge.yaml: this
	// engine has no business validating another repository's build.
	raw, err := os.ReadFile(filepath.Join(repo, "forge.yaml"))
	if err != nil {
		return nil, err
	}

	var head struct {
		ArtifactStorePath string `json:"artifactStorePath"`
	}

	if err := yaml.Unmarshal(raw, &head); err != nil {
		return nil, fmt.Errorf("reading forge.yaml: %w", err)
	}

	if head.ArtifactStorePath == "" {
		return nil, fmt.Errorf("forge.yaml in %s names no artifactStorePath", repo)
	}

	storePath := head.ArtifactStorePath
	if !filepath.IsAbs(storePath) {
		storePath = filepath.Join(repo, storePath)
	}

	store, err := forge.ReadArtifactStore(storePath)
	if err != nil {
		return nil, err
	}

	out := []entry{}

	for _, p := range platforms {
		platform := p
		names := map[string]bool{}

		for _, a := range store.Artifacts {
			if a.Type != forge.TypeBinary || a.Platform() != platform.String() || names[a.Name] {
				continue
			}

			latest, err := forge.GetLatestArtifact(store, a.Name, platform.String())
			if err != nil {
				return nil, err
			}

			names[a.Name] = true

			location := strings.TrimPrefix(latest.Location, "file://")
			if !filepath.IsAbs(location) {
				location = filepath.Join(repo, location)
			}

			if _, err := os.Stat(location); err != nil {
				return nil, fmt.Errorf("the record of %s for %s names %s: %w", a.Name, platform, location, err)
			}

			out = append(out, entry{path: location, name: a.Name, platform: &platform})
		}

		if len(names) == 0 {
			return nil, fmt.Errorf("%w: %s for %s", ErrNoRecord, repo, platform)
		}
	}

	return out, nil
}

// group sorts the entries onto the platforms they belong to. An entry read
// from a record goes to its platform alone; a glob's file goes to every
// platform, because a script or a certificate is the same on all of them.
// In the image a binary lands under its artifact name, whatever the file
// on disk was called.
func group(entries []entry, platforms []imageassembly.Platform) (map[imageassembly.Platform][]imageassembly.File, error) {
	out := map[imageassembly.Platform][]imageassembly.File{}
	for _, p := range platforms {
		out[p] = []imageassembly.File{}
	}

	for _, e := range entries {
		file := imageassembly.File{Path: e.path, Name: e.name}

		if e.platform != nil {
			out[*e.platform] = append(out[*e.platform], file)

			continue
		}

		for _, p := range platforms {
			out[p] = append(out[p], file)
		}
	}

	for _, p := range platforms {
		if len(out[p]) == 0 {
			return nil, fmt.Errorf("%w: %s", ErrEmptyPlatform, p)
		}
	}

	return out, nil
}

// Build assembles the image and writes it to disk as an OCI image layout. It
// names no registry and pushes nothing: a build writes a file and a release
// publishes it, exactly as a binary does, so this engine holds no credential.
//
// One call assembles every platform it was handed into one index, and
// answers one artifact for the layout, tied to no single platform.
func Build(_ context.Context, input mcptypes.BuildInput, spec *Spec) ([]forge.Artifact, error) {
	root := input.Context
	if root == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("reading the working directory: %w", err)
		}

		root = cwd
	}

	platforms, err := platformsOf(input.Platforms)
	if err != nil {
		return nil, err
	}

	entries, err := expand(root, spec.From, platforms)
	if err != nil {
		return nil, err
	}

	byPlatform, err := group(entries, platforms)
	if err != nil {
		return nil, err
	}

	binDir := spec.BinDir
	if binDir == "" {
		binDir = "/usr/local/bin"
	}

	base := spec.Base
	if base == "" {
		base = "debian:stable-slim"
	}

	log.Printf("assembling %s: base %s, %d files, %d platform(s)",
		input.Name, base, len(entries), len(platforms))

	images, err := imageassembly.New(&imageassembly.Remote{Token: os.Getenv("REGISTRY_TOKEN")}).
		Assemble(imageassembly.Request{
			Base:   base,
			BinDir: binDir,
			Files:  byPlatform,
			Env:    spec.Env,
			Labels: spec.Labels,
		})
	if err != nil {
		return nil, err
	}

	index, err := imageassembly.Index(images)
	if err != nil {
		return nil, err
	}

	dest := input.Dest
	if dest == "" {
		dest = filepath.Join(root, "build", "images")
	} else if !filepath.IsAbs(dest) {
		dest = filepath.Join(root, dest)
	}

	out := filepath.Join(dest, input.Name+".oci")

	// A stale layout would be merged into rather than replaced, so the image
	// would carry manifests from a build nobody asked to keep.
	if err := os.RemoveAll(out); err != nil {
		return nil, fmt.Errorf("clearing %s: %w", out, err)
	}

	if err := os.MkdirAll(out, 0o750); err != nil {
		return nil, fmt.Errorf("creating %s: %w", out, err)
	}

	if _, err := layout.Write(out, index); err != nil {
		return nil, fmt.Errorf("writing the image layout to %s: %w", out, err)
	}

	digest, err := index.Digest()
	if err != nil {
		return nil, fmt.Errorf("reading the image digest: %w", err)
	}

	log.Printf("wrote %s (%s)", out, digest)

	return []forge.Artifact{{
		Name:      input.Name,
		Type:      forge.TypeContainer,
		Location:  "file://" + out,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Version:   digest.String(),
	}}, nil
}

func platformsOf(raw []string) ([]imageassembly.Platform, error) {
	out := make([]imageassembly.Platform, 0, len(raw))

	for _, s := range raw {
		p, err := imageassembly.ParsePlatform(s)
		if err != nil {
			return nil, err
		}

		out = append(out, p)
	}

	return out, nil
}
