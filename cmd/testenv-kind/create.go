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
	"context"
	"fmt"
	"log"
	"path/filepath"

	"github.com/alexandremahdhaoui/forge/pkg/engineframework"
	"github.com/alexandremahdhaoui/forge/pkg/forge"
)

// Create implements the CreateFunc for creating kind clusters.
func Create(ctx context.Context, input engineframework.CreateInput, spec *Spec) (*engineframework.TestEnvArtifact, error) {
	log.Printf("Creating kind cluster: testID=%s, stage=%s", input.TestID, input.Stage)

	// RootDir is available via input.RootDir for resolving relative paths
	// Currently unused, but available for future features that may need path resolution
	// (e.g., custom kind config files with relative paths)
	_ = input.RootDir // Acknowledge availability for consistency

	// Read forge.yaml configuration
	config, err := forge.ReadSpec()
	if err != nil {
		return nil, fmt.Errorf("failed to read forge spec: %w", err)
	}

	// Read environment variables
	envs, err := readEnvs()
	if err != nil {
		return nil, fmt.Errorf("failed to read environment variables: %w", err)
	}

	// Generate cluster name and kubeconfig path
	clusterName := fmt.Sprintf("%s-%s", config.Name, input.TestID)
	kubeconfigPath := filepath.Join(input.TmpDir, "kubeconfig")

	// Create the kind cluster
	if err := doSetup(cluster{Name: clusterName, KubeconfigPath: kubeconfigPath}, envs); err != nil {
		return nil, fmt.Errorf("failed to create kind cluster: %w", err)
	}

	// Prepare files map (relative paths within tmpDir)
	files := map[string]string{
		"testenv-kind.kubeconfig": "kubeconfig",
	}

	// Prepare metadata
	metadata := map[string]string{
		"testenv-kind.clusterName":    clusterName,
		"testenv-kind.kubeconfigPath": kubeconfigPath,
	}

	// Prepare managed resources (for cleanup)
	managedResources := []string{
		kubeconfigPath,
	}

	// Return artifact
	return &engineframework.TestEnvArtifact{
		TestID:           input.TestID,
		Files:            files,
		Metadata:         metadata,
		ManagedResources: managedResources,
		Env: map[string]string{
			"KUBECONFIG": kubeconfigPath,
		},
	}, nil
}

// Delete implements the DeleteFunc for deleting kind clusters.
func Delete(ctx context.Context, input engineframework.DeleteInput, _ *Spec) error {
	log.Printf("Deleting kind cluster: testID=%s", input.TestID)

	// Read forge.yaml configuration
	config, err := forge.ReadSpec()
	if err != nil {
		return fmt.Errorf("failed to read forge spec: %w", err)
	}

	// Read environment variables
	envs, err := readEnvs()
	if err != nil {
		return fmt.Errorf("failed to read environment variables: %w", err)
	}

	// Get cluster name from metadata (preferred) or reconstruct from testID (fallback)
	// Using metadata ensures we delete the exact cluster that was created,
	// preventing accidental deletion of clusters from other forge instances
	var clusterName string
	var kubeconfigPath string
	if input.Metadata != nil {
		if name, ok := input.Metadata["testenv-kind.clusterName"]; ok && name != "" {
			clusterName = name
			log.Printf("Using cluster name from metadata: %s", clusterName)
		}
		if path, ok := input.Metadata["testenv-kind.kubeconfigPath"]; ok && path != "" {
			kubeconfigPath = path
			log.Printf("Using kubeconfig path from metadata: %s", kubeconfigPath)
		}
	}
	if clusterName == "" {
		// Fallback: reconstruct cluster name (for backward compatibility)
		clusterName = fmt.Sprintf("%s-%s", config.Name, input.TestID)
		log.Printf("Reconstructing cluster name from testID: %s", clusterName)
	}
	// Delete the kind cluster - return error on failure to prevent silent leaks
	if err := doTeardown(cluster{Name: clusterName, KubeconfigPath: kubeconfigPath}, envs); err != nil {
		return fmt.Errorf("failed to delete kind cluster %s: %w", clusterName, err)
	}

	return nil
}
