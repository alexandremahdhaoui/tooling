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
	"errors"
	"fmt"
	"runtime"
	"strings"

	"github.com/alexandremahdhaoui/forge/pkg/forge"
)

// Capabilities is what an engine declares it can do, in forge-dev.yaml,
// and what the framework holds it to before its build function runs. An
// engine that declares nothing builds for the host and refuses everything
// else, loudly, instead of ignoring a platform it was handed and answering
// a host binary under a foreign name.
type Capabilities struct {
	// Platforms is the os/arch pairs this engine can build: PlatformsAny,
	// PlatformsHost (the default), or an explicit list.
	Platforms []string

	// Frozen says the engine reads a `frozen` input: it proves the recorded
	// dependency lock before building and never repairs it. forge sends the
	// repo's `frozen:` setting only to an engine that declares this; an
	// engine that does not is refused the input rather than ignoring it.
	// Every build engine holds the invariant itself - a build never writes
	// a lockfile - whether or not it declares the capability.
	Frozen bool

	// Incremental says the output of this engine may be reused when its
	// recorded inputs are unchanged. It is what turns forge's freshness
	// rule on for an entry: an engine that does not declare it runs every
	// time, which is what a generator wants and what forge does by
	// default. A compiler with a real dependency detector declares it; a
	// generator must not, because its output depends on the generator
	// itself and no record holds that.
	Incremental bool
}

const (
	// PlatformsAny says the engine builds for any platform it is handed. A
	// compiler with a target flag declares this.
	PlatformsAny = "any"
	// PlatformsHost says the engine builds only for the machine it runs on.
	// A generator or a formatter declares this, or declares nothing.
	PlatformsHost = "host"
)

// ErrPlatformRefused means an engine was handed a platform it declared it
// cannot build.
var ErrPlatformRefused = errors.New("platform refused")

// ErrCapabilityRefused means an engine was handed an input for a capability
// it does not declare.
var ErrCapabilityRefused = errors.New("capability refused")

// Declaration is the capabilities as an engine answers them over
// config-validate, so a caller learns what to send without guessing:
// platforms as declared (host when nothing is), frozen and incremental as
// bools.
func (c Capabilities) Declaration() map[string]any {
	platforms := c.Platforms
	if len(platforms) == 0 {
		platforms = []string{PlatformsHost}
	}

	return map[string]any{
		"platforms":   platforms,
		"frozen":      c.Frozen,
		"incremental": c.Incremental,
	}
}

// RefuseFrozen refuses a frozen input handed to an engine that never
// declared it reads one. A caller that sends frozen to every engine would
// have it silently ignored by most of them, which is the class of defect a
// declaration exists to remove.
func RefuseFrozen(engine string, caps Capabilities, frozen bool) error {
	if frozen && !caps.Frozen {
		return fmt.Errorf("%w: %s was handed frozen and declares no frozen capability; declare capabilities.frozen: true in its forge-dev.yaml or stop sending it",
			ErrCapabilityRefused, engine)
	}

	return nil
}

// HostPlatform is the os/arch pair of the machine this process runs on.
func HostPlatform() string {
	return runtime.GOOS + "/" + runtime.GOARCH
}

// Builds answers whether these capabilities admit one platform.
func (c Capabilities) Builds(platform string) bool {
	if len(c.Platforms) == 0 {
		return platform == HostPlatform()
	}

	for _, p := range c.Platforms {
		switch p {
		case PlatformsAny:
			return true
		case PlatformsHost:
			if platform == HostPlatform() {
				return true
			}
		default:
			if p == platform {
				return true
			}
		}
	}

	return false
}

// Declared is the declaration as a person reads it in a refusal.
func (c Capabilities) Declared() string {
	if len(c.Platforms) == 0 {
		return PlatformsHost
	}

	return strings.Join(c.Platforms, ", ")
}

// RefusePlatforms is the check every build runs before an engine's own
// function: each requested platform is well-formed and declared. The
// message names the engine, the platform and the declaration, so the fix is
// readable from the error alone.
func RefusePlatforms(engine string, caps Capabilities, platforms []string) error {
	if len(platforms) == 0 {
		return fmt.Errorf("%w: %s was handed no platform; the core always names at least one",
			ErrPlatformRefused, engine)
	}

	for _, platform := range platforms {
		if _, _, err := forge.SplitPlatform(platform); err != nil {
			return fmt.Errorf("%w: %s: %w", ErrPlatformRefused, engine, err)
		}

		if !caps.Builds(platform) {
			return fmt.Errorf("%w: %s cannot build %s; it declares platforms: %s",
				ErrPlatformRefused, engine, platform, caps.Declared())
		}
	}

	return nil
}
