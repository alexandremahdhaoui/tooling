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

package main

import (
	"fmt"
	"os"

	"github.com/alexandremahdhaoui/forge/internal/engineresolver"
	"github.com/alexandremahdhaoui/forge/pkg/forge"
)

// installEngineRegistry loads the registry from the forge.yaml in the
// working directory, when there is one, and the factory above it. A
// missing forge.yaml is not an error here: verbs that need one say so
// themselves, and the registry is then just the factory's ring.
func installEngineRegistry() error {
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("installing the engine registry: %w", err)
	}

	var spec *forge.Spec

	if loaded, err := loadConfig(); err == nil {
		spec = &loaded
	}

	registry, err := engineresolver.Load(spec, wd)
	if err != nil {
		return fmt.Errorf("installing the engine registry: %w", err)
	}

	engineresolver.Use(registry)

	return nil
}
