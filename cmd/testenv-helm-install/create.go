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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/alexandremahdhaoui/forge/pkg/engineframework"
	"gopkg.in/yaml.v3"
)

func resolveChartPath(chart ChartSpec, rootDir string) (string, error) {
	if chart.SourceType != "local" || chart.Path == "" {
		return chart.Path, nil
	}

	resolvedPath := chart.Path

	if rootDir != "" && !filepath.IsAbs(chart.Path) {
		resolvedPath = filepath.Join(rootDir, chart.Path)
		log.Printf("Resolved local chart path: %s", resolvedPath)
	}

	if _, err := os.Stat(resolvedPath); os.IsNotExist(err) {
		return "", fmt.Errorf("local chart not found: %s", resolvedPath)
	}

	return resolvedPath, nil
}

func Create(
	_ context.Context,
	input engineframework.CreateInput,
	spec *Spec,
) (*engineframework.TestEnvArtifact, error) {
	log.Printf("Installing Helm charts: testID=%s, stage=%s", input.TestID, input.Stage)

	var charts []ChartSpec
	if spec != nil {
		charts = spec.Charts
	}

	if len(charts) == 0 {
		log.Printf("No charts declared, skipping helm installation")

		return &engineframework.TestEnvArtifact{
			TestID:           input.TestID,
			Files:            map[string]string{},
			Metadata:         map[string]string{"testenv-helm-install.chartCount": "0"},
			ManagedResources: []string{},
		}, nil
	}

	for i := range charts {
		if charts[i].SourceType == "local" && charts[i].Path != "" {
			resolvedPath, err := resolveChartPath(charts[i], input.RootDir)
			if err != nil {
				return nil, err
			}
			charts[i].Path = resolvedPath
		}
	}

	kubeconfigPath := ""
	if envKubeconfig, ok := input.Env["KUBECONFIG"]; ok && envKubeconfig != "" {
		kubeconfigPath = envKubeconfig
		log.Printf("Using KUBECONFIG from environment: %s", kubeconfigPath)
	} else {
		var err error
		kubeconfigPath, err = findKubeconfig(input.TmpDir, input.Metadata)
		if err != nil {
			return nil, fmt.Errorf("failed to find kubeconfig: %w", err)
		}
		log.Printf("Using kubeconfig from legacy sources (tmpDir/metadata): %s", kubeconfigPath)
	}

	installedCharts := []string{}
	metadata := map[string]string{}

	for i, chart := range charts {
		releaseName := chart.ReleaseName
		if releaseName == "" {
			releaseName = chart.Name
		}

		log.Printf("Installing chart %d/%d: %s (release: %s)", i+1, len(charts), chart.Name, releaseName)

		if chart.SourceType == "helm-repo" && chart.Url != "" {
			repoName := extractRepoNameFromURL(chart.Url)
			if err := addHelmRepo(repoName, chart.Url); err != nil {
				return nil, fmt.Errorf("failed to add helm repo %s: %w", chart.Url, err)
			}
		}

		if err := installChart(chart, kubeconfigPath); err != nil {
			return nil, fmt.Errorf("failed to install chart %s: %w", chart.Name, err)
		}

		installedCharts = append(installedCharts, releaseName)

		prefix := fmt.Sprintf("testenv-helm-install.chart.%d", i)
		metadata[prefix+".name"] = chart.Name
		metadata[prefix+".releaseName"] = releaseName
		if chart.Namespace != "" {
			metadata[prefix+".namespace"] = chart.Namespace
		}
	}

	metadata["testenv-helm-install.chartCount"] = fmt.Sprintf("%d", len(installedCharts))

	return &engineframework.TestEnvArtifact{
		TestID:           input.TestID,
		Files:            map[string]string{},
		Metadata:         metadata,
		ManagedResources: []string{},
	}, nil
}

func Delete(_ context.Context, input engineframework.DeleteInput, _ *Spec) error {
	log.Printf("Uninstalling Helm charts: testID=%s", input.TestID)

	chartCountStr, ok := input.Metadata["testenv-helm-install.chartCount"]
	if !ok {
		log.Printf("No charts found in metadata, skipping uninstall")
		return nil
	}

	var chartCount int
	if _, err := fmt.Sscanf(chartCountStr, "%d", &chartCount); err != nil {
		log.Printf("Warning: invalid chartCount in metadata: %v", err)
		return nil
	}

	kubeconfigPath, ok := input.Metadata["testenv-kind.kubeconfigPath"]
	if !ok {
		log.Printf("Warning: kubeconfig not found in metadata, skipping helm uninstall")
		return nil
	}

	if _, err := os.Stat(kubeconfigPath); os.IsNotExist(err) {
		return fmt.Errorf(
			"kubeconfig file does not exist at %s - cluster was deleted before helm uninstall (cleanup order bug)",
			kubeconfigPath,
		)
	}

	for i := chartCount - 1; i >= 0; i-- {
		prefix := fmt.Sprintf("testenv-helm-install.chart.%d", i)
		releaseName := input.Metadata[prefix+".releaseName"]
		namespace := input.Metadata[prefix+".namespace"]

		if releaseName == "" {
			log.Printf("Warning: chart %d missing release name, skipping", i)
			continue
		}

		log.Printf("Uninstalling chart %d/%d: %s", chartCount-i, chartCount, releaseName)

		if err := uninstallChart(releaseName, namespace, kubeconfigPath); err != nil {
			log.Printf("Warning: failed to uninstall chart %s: %v", releaseName, err)
		}
	}

	return nil
}

func findKubeconfig(tmpDir string, metadata map[string]string) (string, error) {
	if path, ok := metadata["testenv-kind.kubeconfigPath"]; ok && path != "" {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	commonNames := []string{"kubeconfig", "kubeconfig.yaml", ".kube/config"}
	for _, name := range commonNames {
		path := filepath.Join(tmpDir, name)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("kubeconfig not found in tmpDir or metadata")
}

func extractRepoNameFromURL(url string) string {
	url = strings.TrimSuffix(url, "/")

	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return "repo"
}

func resolveGitRef(chart ChartSpec) (ref string, refType string, err error) {
	if chart.GitCommit != "" {
		if len(chart.GitCommit) < 7 {
			return "", "", fmt.Errorf("invalid git commit: too short (minimum 7 characters)")
		}
		for _, c := range chart.GitCommit {
			isDigit := c >= '0' && c <= '9'
			isLowerHex := c >= 'a' && c <= 'f'
			isUpperHex := c >= 'A' && c <= 'F'
			if !isDigit && !isLowerHex && !isUpperHex {
				return "", "", fmt.Errorf("invalid git commit: contains non-hexadecimal character: %c", c)
			}
		}
		return chart.GitCommit, "commit", nil
	}

	if chart.GitTag != "" {
		return chart.GitTag, "tag", nil
	}

	if chart.GitSemVer != "" {
		return chart.GitSemVer, "semver", nil
	}

	if chart.GitBranch != "" {
		return chart.GitBranch, "branch", nil
	}

	return "", "", fmt.Errorf(
		"no git reference specified: one of GitCommit, GitTag, GitSemVer, or GitBranch is required",
	)
}

func resolveSemVerTag(repoPath string, semverConstraint string) (string, error) {
	constraint, err := semver.NewConstraint(semverConstraint)
	if err != nil {
		return "", fmt.Errorf("invalid semver constraint %q: %w", semverConstraint, err)
	}

	cmd := exec.Command("git", "tag", "-l")
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to list git tags: %w, output: %s", err, string(output))
	}

	tagLines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var matchingVersions []*semver.Version
	var matchingTags []string

	for _, tagLine := range tagLines {
		tag := strings.TrimSpace(tagLine)
		if tag == "" {
			continue
		}

		versionStr := strings.TrimPrefix(tag, "v")
		version, err := semver.NewVersion(versionStr)
		if err != nil {
			continue
		}

		if constraint.Check(version) {
			matchingVersions = append(matchingVersions, version)
			matchingTags = append(matchingTags, tag)
		}
	}

	if len(matchingVersions) == 0 {
		return "", fmt.Errorf("no git tags match semver constraint %q", semverConstraint)
	}

	latestIndex := 0
	for i := 1; i < len(matchingVersions); i++ {
		if matchingVersions[i].GreaterThan(matchingVersions[latestIndex]) {
			latestIndex = i
		}
	}

	return matchingTags[latestIndex], nil
}

func validateGitSource(chart ChartSpec) error {
	if chart.Url == "" {
		return fmt.Errorf("url is required for git source type")
	}

	if !strings.HasPrefix(chart.Url, "http://") &&
		!strings.HasPrefix(chart.Url, "https://") &&
		!strings.HasPrefix(chart.Url, "git@") {
		return fmt.Errorf("invalid url format: must start with http://, https://, or git@")
	}

	if chart.ChartPath == "" {
		return fmt.Errorf("chartPath is required for git source type")
	}

	if strings.HasPrefix(chart.ChartPath, "/") {
		return fmt.Errorf("chartPath must be relative (no leading /)")
	}

	if chart.GitCommit == "" && chart.GitTag == "" && chart.GitSemVer == "" && chart.GitBranch == "" {
		return fmt.Errorf(
			"at least one git reference (GitCommit, GitTag, GitSemVer, or GitBranch) is required",
		)
	}

	return nil
}

func buildGitCloneCommand(url, destDir, ref, refType string) []string {
	args := []string{"clone"}

	if refType == "branch" || refType == "tag" {
		args = append(args, "--branch", ref, "--depth", "1")
	}

	args = append(args, url, destDir)
	return args
}

func cloneGitRepository(chart ChartSpec, destDir string) (chartPath string, cleanup func(), err error) {
	if chart.Url == "" {
		return "", nil, fmt.Errorf("URL is required for git source type")
	}
	if chart.ChartPath == "" {
		return "", nil, fmt.Errorf("ChartPath is required for git source type")
	}

	ref, refType, err := resolveGitRef(chart)
	if err != nil {
		return "", nil, err
	}

	cloneDir := filepath.Join(destDir, "git-clone")
	if err := os.MkdirAll(cloneDir, 0o755); err != nil {
		return "", nil, fmt.Errorf("failed to create clone directory: %w", err)
	}

	cleanup = func() {
		if err := os.RemoveAll(cloneDir); err != nil {
			log.Printf("Warning: failed to remove git clone directory %s: %v", cloneDir, err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	args := buildGitCloneCommand(chart.Url, cloneDir, ref, refType)
	cmd := exec.CommandContext(ctx, "git", args...)

	log.Printf("Cloning git repository: %s (ref: %s, type: %s)", chart.Url, ref, refType)
	startTime := time.Now()

	output, err := cmd.CombinedOutput()
	if err != nil {
		cleanup()
		if ctx.Err() == context.DeadlineExceeded {
			return "", nil, fmt.Errorf("git clone timed out after 5 minutes")
		}
		return "", nil, fmt.Errorf(
			"failed to clone git repository %s: %w, output: %s", chart.Url, err, string(output),
		)
	}

	cloneDuration := time.Since(startTime)
	if cloneDuration > 30*time.Second {
		log.Printf("Warning: git clone took %v (>30s)", cloneDuration)
	}

	switch refType {
	case "commit":
		checkoutCmd := exec.CommandContext(ctx, "git", "checkout", ref)
		checkoutCmd.Dir = cloneDir
		output, err := checkoutCmd.CombinedOutput()
		if err != nil {
			cleanup()
			return "", nil, fmt.Errorf(
				"failed to checkout commit %s: %w, output: %s", ref, err, string(output),
			)
		}
		log.Printf("Checked out commit: %s", ref)
	case "semver":
		tag, err := resolveSemVerTag(cloneDir, ref)
		if err != nil {
			cleanup()
			return "", nil, fmt.Errorf("failed to resolve semver %s: %w", ref, err)
		}
		checkoutCmd := exec.CommandContext(ctx, "git", "checkout", tag)
		checkoutCmd.Dir = cloneDir
		output, err := checkoutCmd.CombinedOutput()
		if err != nil {
			cleanup()
			return "", nil, fmt.Errorf(
				"failed to checkout tag %s: %w, output: %s", tag, err, string(output),
			)
		}
		log.Printf("Resolved semver %s to tag %s and checked out", ref, tag)
	}

	chartPath = filepath.Join(cloneDir, chart.ChartPath)

	chartYamlPath := filepath.Join(chartPath, "Chart.yaml")
	if _, err := os.Stat(chartYamlPath); os.IsNotExist(err) {
		cleanup()
		return "", nil, fmt.Errorf("chart.yaml not found at %s", chartPath)
	}

	log.Printf("Successfully cloned and validated chart at: %s", chartPath)
	return chartPath, cleanup, nil
}

func addHelmRepo(name, repoURL string) error {
	log.Printf("Adding helm repo: %s -> %s", name, repoURL)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "helm", "repo", "add", name, repoURL)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("helm repo add timed out after 2 minutes")
		}
		return fmt.Errorf("helm repo add failed: %w, output: %s", err, string(output))
	}

	cmd = exec.CommandContext(ctx, "helm", "repo", "update")
	output, err = cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("helm repo update timed out after 2 minutes")
		}
		return fmt.Errorf("helm repo update failed: %w, output: %s", err, string(output))
	}

	return nil
}

func parseYAMLValue(yamlStr string) (interface{}, error) {
	var result interface{}
	err := yaml.Unmarshal([]byte(yamlStr), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse YAML value: %w", err)
	}

	return result, nil
}

func extractValueByKey(data map[string]string, valuesKey string) (interface{}, error) {
	if valuesKey == "" {
		result := make(map[string]interface{})
		for k, v := range data {
			result[k] = v
		}
		return result, nil
	}

	value, ok := data[valuesKey]
	if !ok {
		return nil, fmt.Errorf("key %q not found in data", valuesKey)
	}

	parsed, err := parseYAMLValue(value)
	if err != nil {
		return value, nil
	}

	return parsed, nil
}

func resolveValueReference(kubeconfigPath, namespace string, ref ValueReference) (interface{}, error) {
	var data map[string]string
	var err error

	switch ref.Kind {
	case "ConfigMap":
		data, err = fetchConfigMap(kubeconfigPath, namespace, ref.Name)
	case "Secret":
		data, err = fetchSecret(kubeconfigPath, namespace, ref.Name)
	default:
		return nil, fmt.Errorf("unsupported Kind %q: must be ConfigMap or Secret", ref.Kind)
	}

	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "NotFound") {
			if ref.Optional {
				log.Printf("Info: optional %s %s/%s not found, skipping", ref.Kind, namespace, ref.Name)
				return nil, nil
			}
		}
		return nil, err
	}

	return extractValueByKey(data, ref.ValuesKey)
}

func mergeValuesAtPath(baseValues map[string]interface{}, newValues interface{}, targetPath string) error {
	if targetPath == "" {
		newMap, ok := newValues.(map[string]interface{})
		if !ok {
			return fmt.Errorf("cannot merge non-map value at root level (got type %T)", newValues)
		}
		mergeMap(baseValues, newMap)
		return nil
	}

	pathParts := strings.Split(targetPath, ".")

	current := baseValues
	for i := 0; i < len(pathParts)-1; i++ {
		key := pathParts[i]

		if existing, ok := current[key]; ok {
			existingMap, ok := existing.(map[string]interface{})
			if !ok {
				return fmt.Errorf("type conflict at path %q: existing value is %T, cannot navigate deeper",
					strings.Join(pathParts[:i+1], "."), existing)
			}
			current = existingMap
		} else {
			newMap := make(map[string]interface{})
			current[key] = newMap
			current = newMap
		}
	}

	finalKey := pathParts[len(pathParts)-1]

	if existing, ok := current[finalKey]; ok {
		existingMap, existingIsMap := existing.(map[string]interface{})
		newMap, newIsMap := newValues.(map[string]interface{})

		if existingIsMap && newIsMap {
			mergeMap(existingMap, newMap)
			return nil
		}
	}

	current[finalKey] = newValues
	return nil
}

func mergeMap(dst, src map[string]interface{}) {
	for key, srcValue := range src {
		if dstValue, ok := dst[key]; ok {
			dstMap, dstIsMap := dstValue.(map[string]interface{})
			srcMap, srcIsMap := srcValue.(map[string]interface{})

			if dstIsMap && srcIsMap {
				mergeMap(dstMap, srcMap)
			} else {
				dst[key] = srcValue
			}
		} else {
			dst[key] = srcValue
		}
	}
}

func installChart(chart ChartSpec, kubeconfigPath string) error {
	releaseName := chart.ReleaseName
	if releaseName == "" {
		releaseName = chart.Name
	}

	var chartRef string
	var cleanup func()

	switch chart.SourceType {
	case "helm-repo":
		if chart.Url == "" {
			return fmt.Errorf("url is required when sourceType is helm-repo")
		}
		if chart.ChartName == "" {
			return fmt.Errorf("chartName is required when sourceType is helm-repo")
		}
		repoName := extractRepoNameFromURL(chart.Url)
		chartRef = fmt.Sprintf("%s/%s", repoName, chart.ChartName)

	case "local":
		if chart.Path == "" {
			return fmt.Errorf("path is required when sourceType is local")
		}
		chartRef = chart.Path

	case "git":
		if err := validateGitSource(chart); err != nil {
			return fmt.Errorf("invalid git source: %w", err)
		}

		tmpDir, err := os.MkdirTemp("", "helm-git-*")
		if err != nil {
			return fmt.Errorf("failed to create temp dir: %w", err)
		}

		chartPath, cleanupFunc, err := cloneGitRepository(chart, tmpDir)
		if err != nil {
			_ = os.RemoveAll(tmpDir)
			return fmt.Errorf("failed to clone git repository: %w", err)
		}
		cleanup = func() {
			cleanupFunc()
			_ = os.RemoveAll(tmpDir)
		}
		defer cleanup()

		chartRef = chartPath
		log.Printf("Using git chart at: %s", chartRef)

	case "oci":
		if err := validateOCISource(chart); err != nil {
			return fmt.Errorf("invalid oci source: %w", err)
		}

		authCleanup, err := setupOCIAuth(kubeconfigPath, chart)
		if err != nil {
			return fmt.Errorf("failed to setup OCI auth: %w", err)
		}
		defer authCleanup()

		if err := verifyOCISignature(chart); err != nil {
			return fmt.Errorf("failed to verify OCI signature: %w", err)
		}

		chartRef = chart.Url
		log.Printf("Using OCI chart: %s", chartRef)

	case "s3":
		if err := validateS3Source(chart); err != nil {
			return fmt.Errorf("invalid s3 source: %w", err)
		}

		s3Client, err := setupS3Auth(kubeconfigPath, chart)
		if err != nil {
			return fmt.Errorf("failed to setup S3 auth: %w", err)
		}

		tmpDir, err := os.MkdirTemp("", "helm-s3-*")
		if err != nil {
			return fmt.Errorf("failed to create temp dir for S3 download: %w", err)
		}
		cleanup = func() {
			_ = os.RemoveAll(tmpDir)
		}
		defer cleanup()

		chartPath, err := downloadFromS3(s3Client, chart, tmpDir)
		if err != nil {
			return fmt.Errorf("failed to download chart from S3: %w", err)
		}

		chartRef = chartPath
		log.Printf("Using S3 chart at: %s", chartRef)

	default:
		return fmt.Errorf("sourceType %s is not yet implemented", chart.SourceType)
	}

	args := []string{
		"install",
		releaseName,
		chartRef,
		"--kubeconfig", kubeconfigPath,
	}

	if chart.Version != "" {
		args = append(args, "--version", chart.Version)
	}

	if chart.Namespace != "" {
		args = append(args, "--namespace", chart.Namespace)
		if chart.CreateNamespace {
			args = append(args, "--create-namespace")
		}
	}

	timeout := chart.Timeout
	if timeout == "" {
		timeout = "5m"
	}
	args = append(args, "--timeout", timeout)

	if !chart.DisableWait {
		args = append(args, "--wait")
	}

	if chart.ForceUpgrade {
		args = append(args, "--force")
	}

	if chart.DisableHooks {
		args = append(args, "--no-hooks")
	}

	composedValues := make(map[string]interface{})

	for _, ref := range chart.ValueReferences {
		namespace := chart.Namespace
		if namespace == "" {
			namespace = "default"
		}
		refValues, err := resolveValueReference(kubeconfigPath, namespace, ref)
		if err != nil {
			return fmt.Errorf("failed to resolve ValueReference %s/%s: %w", ref.Kind, ref.Name, err)
		}

		if refValues != nil {
			if ref.TargetPath != "" {
				err = mergeValuesAtPath(composedValues, refValues, ref.TargetPath)
			} else {
				refMap, ok := refValues.(map[string]interface{})
				if !ok {
					return fmt.Errorf(
						"ValueReference %s/%s returned non-map value at root level (type %T)",
						ref.Kind, ref.Name, refValues,
					)
				}
				mergeMap(composedValues, refMap)
			}
			if err != nil {
				return fmt.Errorf("failed to merge values from %s/%s: %w", ref.Kind, ref.Name, err)
			}
		}
	}

	for key, value := range chart.Values {
		composedValues[key] = value
	}

	var valuesTempFile string
	if len(composedValues) > 0 {
		tmpFile, err := os.CreateTemp("", "helm-values-*.yaml")
		if err != nil {
			return fmt.Errorf("failed to create temp values file: %w", err)
		}
		valuesTempFile = tmpFile.Name()
		defer func() {
			if err := os.Remove(valuesTempFile); err != nil {
				log.Printf("Warning: failed to remove temp values file %s: %v", valuesTempFile, err)
			}
		}()

		valuesYAML, err := yaml.Marshal(composedValues)
		if err != nil {
			return fmt.Errorf("failed to marshal values to YAML: %w", err)
		}

		if _, err := tmpFile.Write(valuesYAML); err != nil {
			if closeErr := tmpFile.Close(); closeErr != nil {
				log.Printf("Warning: failed to close temp file: %v", closeErr)
			}
			return fmt.Errorf("failed to write values to temp file: %w", err)
		}
		if err := tmpFile.Close(); err != nil {
			log.Printf("Warning: failed to close temp file: %v", err)
		}

		log.Printf(
			"Composed values from %d ValueReferences and inline values, wrote to: %s",
			len(chart.ValueReferences), valuesTempFile,
		)
	}

	for _, valuesFile := range chart.ValuesFiles {
		args = append(args, "--values", valuesFile)
	}

	if valuesTempFile != "" {
		args = append(args, "--values", valuesTempFile)
	}

	log.Printf("Running: helm %v", args)

	helmTimeout, err := time.ParseDuration(timeout)
	if err != nil {
		log.Printf("Warning: invalid timeout %s, defaulting to 5m", timeout)
		helmTimeout = 5 * time.Minute
	}
	contextTimeout := helmTimeout + 1*time.Minute

	ctx, cancel := context.WithTimeout(context.Background(), contextTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "helm", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("helm install timed out after %v", contextTimeout)
		}
		return fmt.Errorf("helm install failed: %w, output: %s", err, string(output))
	}

	log.Printf("Chart installed successfully: %s", releaseName)

	if chart.TestEnable {
		log.Printf("Running helm tests for release: %s", releaseName)
		if err := runHelmTest(releaseName, chart.Namespace, kubeconfigPath, timeout); err != nil {
			log.Printf("Warning: helm test failed for %s: %v", releaseName, err)
		}
	}

	return nil
}

func runHelmTest(releaseName, namespace, kubeconfigPath, timeout string) error {
	args := []string{
		"test",
		releaseName,
		"--kubeconfig", kubeconfigPath,
		"--timeout", timeout,
	}

	if namespace != "" {
		args = append(args, "--namespace", namespace)
	}

	log.Printf("Running: helm %v", args)

	helmTimeout, err := time.ParseDuration(timeout)
	if err != nil {
		helmTimeout = 5 * time.Minute
	}
	contextTimeout := helmTimeout + 1*time.Minute

	ctx, cancel := context.WithTimeout(context.Background(), contextTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "helm", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("helm test timed out after %v", contextTimeout)
		}
		return fmt.Errorf("helm test failed: %w, output: %s", err, string(output))
	}

	log.Printf("Helm tests passed for: %s", releaseName)
	return nil
}

func uninstallChart(releaseName, namespace, kubeconfigPath string) error {
	args := []string{
		"uninstall",
		releaseName,
		"--kubeconfig", kubeconfigPath,
		"--timeout", "2m",
	}

	if namespace != "" {
		args = append(args, "--namespace", namespace)
	}

	log.Printf("Running: helm %v", args)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "helm", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("helm uninstall timed out after 3 minutes")
		}
		return fmt.Errorf("helm uninstall failed: %w, output: %s", err, string(output))
	}

	log.Printf("Chart uninstalled successfully: %s", releaseName)
	return nil
}

func parseOCIReference(ociURL string) (registry, repository, chart, tag, digest string, err error) {
	if ociURL == "" {
		return "", "", "", "", "", fmt.Errorf("empty OCI URL")
	}
	if !strings.HasPrefix(ociURL, "oci://") {
		return "", "", "", "", "", fmt.Errorf("OCI URL must start with oci://")
	}

	path := strings.TrimPrefix(ociURL, "oci://")
	if path == "" {
		return "", "", "", "", "", fmt.Errorf("invalid OCI URL: no path after oci://")
	}

	var digestPart string
	if strings.Contains(path, "@") {
		parts := strings.SplitN(path, "@", 2)
		path = parts[0]
		digestPart = parts[1]
	}

	pathComponents := strings.Split(path, "/")
	if len(pathComponents) < 2 {
		return "", "", "", "", "", fmt.Errorf("invalid OCI URL: must have at least registry and chart")
	}

	var tagPart string
	lastComponent := pathComponents[len(pathComponents)-1]
	if strings.Contains(lastComponent, ":") {
		parts := strings.SplitN(lastComponent, ":", 2)
		pathComponents[len(pathComponents)-1] = parts[0]
		tagPart = parts[1]
	}

	registry = pathComponents[0]
	if registry == "" {
		return "", "", "", "", "", fmt.Errorf("invalid OCI URL: empty registry")
	}

	chart = pathComponents[len(pathComponents)-1]
	if chart == "" {
		return "", "", "", "", "", fmt.Errorf("invalid OCI URL: empty chart name")
	}

	if len(pathComponents) > 2 {
		repository = strings.Join(pathComponents[1:len(pathComponents)-1], "/")
	}

	if digestPart != "" {
		digest = digestPart
		tag = ""
	} else if tagPart != "" {
		tag = tagPart
	} else {
		tag = "latest"
	}

	return registry, repository, chart, tag, digest, nil
}

func extractRegistryFromOCI(ociURL string) (string, error) {
	registry, _, _, _, _, err := parseOCIReference(ociURL)
	if err != nil {
		return "", err
	}
	return registry, nil
}

type DockerConfig struct {
	Auths map[string]DockerAuth `json:"auths"`
}

type DockerAuth struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Auth     string `json:"auth,omitempty"`
}

func parseDockerConfigJSON(configJSON string, registry string) (username, password string, err error) {
	var config DockerConfig
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		return "", "", fmt.Errorf("failed to parse docker config JSON: %w", err)
	}

	if config.Auths == nil {
		return "", "", fmt.Errorf("no auths found in docker config")
	}

	auth, ok := config.Auths[registry]
	if !ok {
		return "", "", fmt.Errorf("no auth found for registry %s", registry)
	}

	if auth.Username != "" && auth.Password != "" {
		return auth.Username, auth.Password, nil
	}

	if auth.Auth != "" {
		decoded, err := base64.StdEncoding.DecodeString(auth.Auth)
		if err != nil {
			return "", "", fmt.Errorf("failed to decode auth field: %w", err)
		}

		parts := strings.SplitN(string(decoded), ":", 2)
		if len(parts) != 2 {
			return "", "", fmt.Errorf("invalid auth format: expected 'username:password'")
		}

		return parts[0], parts[1], nil
	}

	return "", "", fmt.Errorf("no credentials found in auth for registry %s", registry)
}

func setupOCIAuth(kubeconfigPath string, chart ChartSpec) (cleanup func(), err error) {
	if chart.AuthSecretName == "" {
		return func() {}, nil
	}

	registry, err := extractRegistryFromOCI(chart.Url)
	if err != nil {
		return nil, fmt.Errorf("failed to extract registry from OCI URL: %w", err)
	}

	namespace := chart.Namespace
	if namespace == "" {
		namespace = "default"
	}

	log.Printf("Fetching auth secret %s from namespace %s", chart.AuthSecretName, namespace)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeconfigPath,
		"get", "secret", chart.AuthSecretName,
		"-n", namespace,
		"-o", "json")

	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("kubectl get secret timed out after 30 seconds")
		}
		return nil, fmt.Errorf(
			"failed to fetch secret %s: %w, output: %s", chart.AuthSecretName, err, string(output),
		)
	}

	var secret struct {
		Data map[string]string `json:"data"`
	}
	if err := json.Unmarshal(output, &secret); err != nil {
		return nil, fmt.Errorf("failed to parse secret JSON: %w", err)
	}

	dockerConfigJSON, ok := secret.Data[".dockerconfigjson"]
	if !ok {
		return nil, fmt.Errorf(
			"secret %s does not contain .dockerconfigjson field", chart.AuthSecretName,
		)
	}

	decoded, err := base64.StdEncoding.DecodeString(dockerConfigJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to decode .dockerconfigjson: %w", err)
	}

	username, password, err := parseDockerConfigJSON(string(decoded), registry)
	if err != nil {
		return nil, fmt.Errorf("failed to parse docker config: %w", err)
	}

	tempDockerConfig, err := os.MkdirTemp("", "docker-config-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp docker config directory: %w", err)
	}

	originalDockerConfig := os.Getenv("DOCKER_CONFIG")
	if err := os.Setenv("DOCKER_CONFIG", tempDockerConfig); err != nil {
		_ = os.RemoveAll(tempDockerConfig)
		return nil, fmt.Errorf("failed to set DOCKER_CONFIG: %w", err)
	}

	log.Printf("Using temporary Docker config: %s", tempDockerConfig)

	ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd = exec.CommandContext(ctx, "helm", "registry", "login", registry,
		"--username", username,
		"--password", password)

	output, err = cmd.CombinedOutput()
	if err != nil {
		_ = os.RemoveAll(tempDockerConfig)
		if originalDockerConfig != "" {
			_ = os.Setenv("DOCKER_CONFIG", originalDockerConfig)
		} else {
			_ = os.Unsetenv("DOCKER_CONFIG")
		}

		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("helm registry login timed out after 30 seconds")
		}
		return nil, fmt.Errorf("helm registry login failed: %w, output: %s", err, string(output))
	}

	log.Printf("Successfully logged in to registry: %s", registry)

	cleanup = func() {
		logoutCtx, logoutCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer logoutCancel()

		logoutCmd := exec.CommandContext(logoutCtx, "helm", "registry", "logout", registry)
		if logoutOutput, logoutErr := logoutCmd.CombinedOutput(); logoutErr != nil {
			log.Printf(
				"Warning: helm registry logout failed: %v, output: %s", logoutErr, string(logoutOutput),
			)
		}

		if originalDockerConfig != "" {
			if err := os.Setenv("DOCKER_CONFIG", originalDockerConfig); err != nil {
				log.Printf("Warning: failed to restore DOCKER_CONFIG: %v", err)
			}
		} else {
			if err := os.Unsetenv("DOCKER_CONFIG"); err != nil {
				log.Printf("Warning: failed to unset DOCKER_CONFIG: %v", err)
			}
		}

		if err := os.RemoveAll(tempDockerConfig); err != nil {
			log.Printf("Warning: failed to remove temp docker config %s: %v", tempDockerConfig, err)
		}

		log.Printf("Cleaned up OCI auth for registry: %s", registry)
	}

	return cleanup, nil
}

func validateOCISource(chart ChartSpec) error {
	if chart.Url == "" {
		return fmt.Errorf("url is required for oci source type")
	}

	if !strings.HasPrefix(chart.Url, "oci://") {
		return fmt.Errorf("url must start with oci:// for oci source type")
	}

	_, _, _, _, _, err := parseOCIReference(chart.Url)
	if err != nil {
		return fmt.Errorf("invalid oci url: %w", err)
	}

	if chart.GitBranch != "" || chart.GitTag != "" || chart.GitCommit != "" || chart.GitSemVer != "" {
		return fmt.Errorf(
			"git reference fields (GitBranch, GitTag, GitCommit, GitSemVer) should not be set for oci source type",
		)
	}

	if chart.ChartPath != "" {
		return fmt.Errorf("chartPath should not be set for oci source type")
	}

	if chart.ChartName != "" {
		return fmt.Errorf(
			"chartName should not be set for oci source type (chart name is part of the oci URL)",
		)
	}

	return nil
}

func verifyOCISignature(chart ChartSpec) error {
	if chart.OciProvider == "" {
		return nil
	}

	log.Printf(
		"Warning: OciProvider verification (%s) is not yet fully implemented. Skipping signature verification for chart %s",
		chart.OciProvider, chart.Name,
	)

	return nil
}

func extractS3CredentialsFromSecret(
	secretData map[string]string,
) (accessKeyID, secretAccessKey, sessionToken string, err error) {
	accessKeyID, ok := secretData["accessKeyID"]
	if !ok || accessKeyID == "" {
		return "", "", "", fmt.Errorf("accessKeyID is required in Secret")
	}

	secretAccessKey, ok = secretData["secretAccessKey"]
	if !ok || secretAccessKey == "" {
		return "", "", "", fmt.Errorf("secretAccessKey is required in Secret")
	}

	sessionToken = secretData["sessionToken"]

	return accessKeyID, secretAccessKey, sessionToken, nil
}

func validateS3Credentials(accessKeyID, secretAccessKey string) error {
	if accessKeyID == "" {
		return fmt.Errorf("accessKeyID is required")
	}
	if secretAccessKey == "" {
		return fmt.Errorf("secretAccessKey is required")
	}
	return nil
}

func setupS3Auth(kubeconfigPath string, chart ChartSpec) (*S3Client, error) {
	region := chart.S3BucketRegion
	if region == "" {
		region = "us-east-1"
	}

	if chart.AuthSecretName == "" {
		log.Printf("No AuthSecretName specified, using default AWS credentials (IAM role)")
		return NewS3Client(chart.Url, region)
	}

	namespace := chart.Namespace
	if namespace == "" {
		namespace = "default"
	}

	log.Printf("Fetching S3 auth secret %s from namespace %s", chart.AuthSecretName, namespace)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeconfigPath,
		"get", "secret", chart.AuthSecretName,
		"-n", namespace,
		"-o", "jsonpath={.data}")

	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("kubectl get secret timed out after 30 seconds")
		}
		return nil, fmt.Errorf(
			"failed to fetch secret %s: %w, output: %s", chart.AuthSecretName, err, string(output),
		)
	}

	var secretData map[string]string
	if err := json.Unmarshal(output, &secretData); err != nil {
		return nil, fmt.Errorf("failed to parse secret data: %w", err)
	}

	decodedData := make(map[string]string)
	for key, value := range secretData {
		decoded, err := base64.StdEncoding.DecodeString(value)
		if err != nil {
			return nil, fmt.Errorf("failed to decode secret field %s: %w", key, err)
		}
		decodedData[key] = string(decoded)
	}

	accessKeyID, secretAccessKey, sessionToken, err := extractS3CredentialsFromSecret(decodedData)
	if err != nil {
		return nil, fmt.Errorf("failed to extract S3 credentials: %w", err)
	}

	if err := validateS3Credentials(accessKeyID, secretAccessKey); err != nil {
		return nil, err
	}

	log.Printf("Creating S3 client with credentials from Secret %s", chart.AuthSecretName)
	return NewS3ClientWithCredentials(chart.Url, region, accessKeyID, secretAccessKey, sessionToken)
}

func parseS3Path(chartPath string) string {
	return chartPath
}

func buildS3DownloadParams(chart ChartSpec) (bucket, key string, err error) {
	if chart.S3BucketName == "" {
		return "", "", fmt.Errorf("s3BucketName is required for S3 source type")
	}

	if chart.ChartPath == "" {
		return "", "", fmt.Errorf("chartPath is required for S3 source type")
	}

	bucket = chart.S3BucketName
	key = parseS3Path(chart.ChartPath)

	return bucket, key, nil
}

func downloadFromS3(client *S3Client, chart ChartSpec, destDir string) (string, error) {
	bucket, key, err := buildS3DownloadParams(chart)
	if err != nil {
		return "", err
	}

	log.Printf("Downloading chart from S3: bucket=%s, key=%s", bucket, key)

	filename := filepath.Base(key)
	if filename == "" || filename == "." || filename == "/" {
		return "", fmt.Errorf("invalid chart path: cannot determine filename from %s", key)
	}

	destPath := filepath.Join(destDir, filename)

	if err := client.DownloadFile(bucket, key, destPath); err != nil {
		return "", fmt.Errorf("failed to download from S3: %w", err)
	}

	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		return "", fmt.Errorf("downloaded file not found at %s", destPath)
	}

	log.Printf("Successfully downloaded chart to: %s", destPath)
	return destPath, nil
}

func validateS3Source(chart ChartSpec) error {
	if chart.Url == "" {
		return fmt.Errorf("url is required for s3 source type")
	}

	if !strings.HasPrefix(chart.Url, "http://") && !strings.HasPrefix(chart.Url, "https://") {
		return fmt.Errorf("invalid url format: must start with http:// or https://")
	}

	if chart.S3BucketName == "" {
		return fmt.Errorf("s3BucketName is required for s3 source type")
	}

	if chart.ChartPath == "" {
		return fmt.Errorf("chartPath is required for s3 source type")
	}

	if !strings.HasSuffix(chart.ChartPath, ".tgz") && !strings.HasSuffix(chart.ChartPath, ".tar.gz") {
		return fmt.Errorf("chartPath must end with .tgz or .tar.gz for s3 source type")
	}

	if chart.GitBranch != "" || chart.GitTag != "" || chart.GitCommit != "" || chart.GitSemVer != "" {
		return fmt.Errorf(
			"git reference fields (GitBranch, GitTag, GitCommit, GitSemVer) should not be set for s3 source type",
		)
	}

	if chart.OciProvider != "" || chart.OciLayerMediaType != "" {
		return fmt.Errorf(
			"oci fields (OCIProvider, OCILayerMediaType) should not be set for s3 source type",
		)
	}

	if chart.ChartName != "" {
		return fmt.Errorf(
			"chartName should not be set for s3 source type (chart name is in the tarball)",
		)
	}

	return nil
}
