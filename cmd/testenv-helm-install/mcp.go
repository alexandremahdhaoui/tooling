package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/alexandremahdhaoui/forge/internal/mcpserver"
	"github.com/alexandremahdhaoui/forge/pkg/mcputil"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// CreateInput represents the input for the create tool.
type CreateInput struct {
	TestID   string            `json:"testID"`         // Test environment ID (required)
	Stage    string            `json:"stage"`          // Test stage name (required)
	TmpDir   string            `json:"tmpDir"`         // Temporary directory for this test environment
	Metadata map[string]string `json:"metadata"`       // Metadata from previous testenv-subengines (optional)
	Spec     map[string]any    `json:"spec,omitempty"` // Spec containing charts configuration
}

// DeleteInput represents the input for the delete tool.
type DeleteInput struct {
	TestID   string            `json:"testID"`   // Test environment ID (required)
	Metadata map[string]string `json:"metadata"` // Metadata from test environment (required for chart names)
}

// ChartSpec defines a Helm chart to install
type ChartSpec struct {
	Name        string            `json:"name"`                  // Chart name (required)
	Repo        string            `json:"repo,omitempty"`        // Helm repository URL (optional)
	Version     string            `json:"version,omitempty"`     // Chart version (optional)
	Namespace   string            `json:"namespace,omitempty"`   // Kubernetes namespace (optional, defaults to default)
	Values      map[string]string `json:"values,omitempty"`      // Helm values to override (optional)
	ReleaseName string            `json:"releaseName,omitempty"` // Custom release name (optional, defaults to chart name)
}

// runMCPServer starts the MCP server.
func runMCPServer() error {
	v, _, _ := versionInfo.Get()
	server := mcpserver.New("testenv-helm-install", v)

	// Register create tool
	mcpserver.RegisterTool(server, &mcp.Tool{
		Name:        "create",
		Description: "Install Helm charts into a Kubernetes cluster",
	}, handleCreateTool)

	// Register delete tool
	mcpserver.RegisterTool(server, &mcp.Tool{
		Name:        "delete",
		Description: "Uninstall Helm charts from a Kubernetes cluster",
	}, handleDeleteTool)

	// Run the MCP server
	return server.RunDefault()
}

// handleCreateTool handles the "create" tool call from MCP clients.
func handleCreateTool(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input CreateInput,
) (*mcp.CallToolResult, any, error) {
	log.Printf("Installing Helm charts: testID=%s, stage=%s", input.TestID, input.Stage)

	// Validate inputs
	if result := mcputil.ValidateRequiredWithPrefix("Create failed", map[string]string{
		"testID": input.TestID,
		"stage":  input.Stage,
		"tmpDir": input.TmpDir,
	}); result != nil {
		return result, nil, nil
	}

	// Parse charts from spec
	charts, err := parseChartsFromSpec(input.Spec)
	if err != nil {
		// If spec.charts is not found or empty, skip gracefully
		log.Printf("No charts specified, skipping helm installation")

		// Return success with empty metadata
		artifact := map[string]interface{}{
			"testID":           input.TestID,
			"files":            map[string]string{},
			"metadata":         map[string]string{"testenv-helm-install.chartCount": "0"},
			"managedResources": []string{},
		}
		result, returnedArtifact := mcputil.SuccessResultWithArtifact(
			"Skipped Helm chart installation (no charts specified)",
			artifact,
		)
		return result, returnedArtifact, nil
	}

	if len(charts) == 0 {
		log.Printf("Empty charts list, skipping helm installation")

		// Return success with empty metadata
		artifact := map[string]interface{}{
			"testID":           input.TestID,
			"files":            map[string]string{},
			"metadata":         map[string]string{"testenv-helm-install.chartCount": "0"},
			"managedResources": []string{},
		}
		result, returnedArtifact := mcputil.SuccessResultWithArtifact(
			"Skipped Helm chart installation (empty charts list)",
			artifact,
		)
		return result, returnedArtifact, nil
	}

	// Find kubeconfig from tmpDir or metadata
	kubeconfigPath, err := findKubeconfig(input.TmpDir, input.Metadata)
	if err != nil {
		return mcputil.ErrorResult(fmt.Sprintf("Create failed: %v", err)), nil, nil
	}

	log.Printf("Using kubeconfig: %s", kubeconfigPath)

	// Install each chart
	installedCharts := []string{}
	metadata := map[string]string{}

	for i, chart := range charts {
		releaseName := chart.ReleaseName
		if releaseName == "" {
			releaseName = chart.Name
		}

		log.Printf("Installing chart %d/%d: %s (release: %s)", i+1, len(charts), chart.Name, releaseName)

		// Add helm repo if specified
		if chart.Repo != "" {
			// Extract repo name from chart name (e.g., "bitnami/nginx" -> "bitnami")
			repoName := extractRepoName(chart.Name, chart.Repo)
			if err := addHelmRepo(repoName, chart.Repo); err != nil {
				return mcputil.ErrorResult(fmt.Sprintf("Create failed: failed to add helm repo %s: %v", chart.Repo, err)), nil, nil
			}
		}

		// Install the chart
		if err := installChart(chart, kubeconfigPath); err != nil {
			return mcputil.ErrorResult(fmt.Sprintf("Create failed: failed to install chart %s: %v", chart.Name, err)), nil, nil
		}

		installedCharts = append(installedCharts, releaseName)

		// Store chart info in metadata
		prefix := fmt.Sprintf("testenv-helm-install.chart.%d", i)
		metadata[prefix+".name"] = chart.Name
		metadata[prefix+".releaseName"] = releaseName
		if chart.Namespace != "" {
			metadata[prefix+".namespace"] = chart.Namespace
		}
	}

	// Store count of installed charts
	metadata["testenv-helm-install.chartCount"] = fmt.Sprintf("%d", len(installedCharts))

	// Prepare files map (no files produced by helm install)
	files := map[string]string{}

	// Prepare managed resources (for cleanup)
	managedResources := []string{}

	// Return success with artifact
	artifact := map[string]interface{}{
		"testID":           input.TestID,
		"files":            files,
		"metadata":         metadata,
		"managedResources": managedResources,
	}

	result, returnedArtifact := mcputil.SuccessResultWithArtifact(
		fmt.Sprintf("Installed %d Helm chart(s): %v", len(installedCharts), installedCharts),
		artifact,
	)
	return result, returnedArtifact, nil
}

// handleDeleteTool handles the "delete" tool call from MCP clients.
func handleDeleteTool(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input DeleteInput,
) (*mcp.CallToolResult, any, error) {
	log.Printf("Uninstalling Helm charts: testID=%s", input.TestID)

	// Validate inputs
	if result := mcputil.ValidateRequiredWithPrefix("Delete failed", map[string]string{
		"testID": input.TestID,
	}); result != nil {
		return result, nil, nil
	}

	// Extract chart count from metadata
	chartCountStr, ok := input.Metadata["testenv-helm-install.chartCount"]
	if !ok {
		// No charts to uninstall
		log.Printf("No charts found in metadata, skipping uninstall")
		return mcputil.SuccessResult("No Helm charts to uninstall"), nil, nil
	}

	var chartCount int
	if _, err := fmt.Sscanf(chartCountStr, "%d", &chartCount); err != nil {
		return mcputil.ErrorResult(fmt.Sprintf("Delete failed: invalid chartCount: %v", err)), nil, nil
	}

	// Find kubeconfig from metadata (use testenv-kind's kubeconfig)
	kubeconfigPath, ok := input.Metadata["testenv-kind.kubeconfigPath"]
	if !ok {
		log.Printf("Warning: kubeconfig not found in metadata, skipping helm uninstall")
		return mcputil.SuccessResult("Skipped Helm chart uninstall (no kubeconfig)"), nil, nil
	}

	// Uninstall each chart in reverse order
	uninstalledCharts := []string{}
	for i := chartCount - 1; i >= 0; i-- {
		prefix := fmt.Sprintf("testenv-helm-install.chart.%d", i)
		releaseName := input.Metadata[prefix+".releaseName"]
		namespace := input.Metadata[prefix+".namespace"]

		if releaseName == "" {
			log.Printf("Warning: chart %d missing release name, skipping", i)
			continue
		}

		log.Printf("Uninstalling chart %d/%d: %s", chartCount-i, chartCount, releaseName)

		// Uninstall the chart (best effort)
		if err := uninstallChart(releaseName, namespace, kubeconfigPath); err != nil {
			log.Printf("Warning: failed to uninstall chart %s: %v", releaseName, err)
			// Continue with other charts
		} else {
			uninstalledCharts = append(uninstalledCharts, releaseName)
		}
	}

	return mcputil.SuccessResult(fmt.Sprintf("Uninstalled %d Helm chart(s)", len(uninstalledCharts))), nil, nil
}

// parseChartsFromSpec extracts chart specifications from the spec map
func parseChartsFromSpec(spec map[string]any) ([]ChartSpec, error) {
	if spec == nil {
		return nil, fmt.Errorf("spec is nil")
	}

	chartsRaw, ok := spec["charts"]
	if !ok {
		return nil, fmt.Errorf("spec.charts not found")
	}

	// Marshal and unmarshal to convert to ChartSpec slice
	chartsJSON, err := json.Marshal(chartsRaw)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal charts: %w", err)
	}

	var charts []ChartSpec
	if err := json.Unmarshal(chartsJSON, &charts); err != nil {
		return nil, fmt.Errorf("failed to unmarshal charts: %w", err)
	}

	return charts, nil
}

// findKubeconfig locates the kubeconfig file from tmpDir or metadata
func findKubeconfig(tmpDir string, metadata map[string]string) (string, error) {
	// First try to get from metadata (testenv-kind provides this)
	if path, ok := metadata["testenv-kind.kubeconfigPath"]; ok && path != "" {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	// Try common locations in tmpDir
	commonNames := []string{"kubeconfig", "kubeconfig.yaml", ".kube/config"}
	for _, name := range commonNames {
		path := filepath.Join(tmpDir, name)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("kubeconfig not found in tmpDir or metadata")
}

// extractRepoName extracts the repository name from a chart name or repo URL
// For chart names like "bitnami/nginx", returns "bitnami"
// For simple names like "nginx", derives from repo URL (e.g., "https://charts.bitnami.com/bitnami" -> "bitnami")
func extractRepoName(chartName, repoURL string) string {
	// If chart name contains "/", use the part before it
	if idx := len(chartName); idx > 0 {
		for i := 0; i < len(chartName); i++ {
			if chartName[i] == '/' {
				return chartName[:i]
			}
		}
	}

	// Otherwise, try to derive from repo URL
	// Extract last path segment before trailing slash
	// e.g., "https://charts.bitnami.com/bitnami" -> "bitnami"
	url := repoURL
	if len(url) > 0 && url[len(url)-1] == '/' {
		url = url[:len(url)-1]
	}

	for i := len(url) - 1; i >= 0; i-- {
		if url[i] == '/' {
			return url[i+1:]
		}
	}

	// Fallback: use the chart name itself
	return chartName
}

// addHelmRepo adds a helm repository
func addHelmRepo(name, repoURL string) error {
	log.Printf("Adding helm repo: %s -> %s", name, repoURL)

	// Add timeout for repo operations (2 minutes should be plenty)
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

	// Update repo with same context
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

// installChart installs a helm chart
func installChart(chart ChartSpec, kubeconfigPath string) error {
	releaseName := chart.ReleaseName
	if releaseName == "" {
		releaseName = chart.Name
	}

	args := []string{
		"install",
		releaseName,
		chart.Name,
		"--kubeconfig", kubeconfigPath,
		"--wait",
		"--timeout", "3m", // Helm-level timeout for pod readiness
	}

	if chart.Version != "" {
		args = append(args, "--version", chart.Version)
	}

	if chart.Namespace != "" {
		args = append(args, "--namespace", chart.Namespace, "--create-namespace")
	}

	// Add values
	for key, value := range chart.Values {
		args = append(args, "--set", fmt.Sprintf("%s=%s", key, value))
	}

	log.Printf("Running: helm %v", args)

	// Add context timeout (4 minutes to allow helm's internal 3m timeout plus buffer)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "helm", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("helm install timed out after 4 minutes")
		}
		return fmt.Errorf("helm install failed: %w, output: %s", err, string(output))
	}

	log.Printf("Chart installed successfully: %s", releaseName)
	return nil
}

// uninstallChart uninstalls a helm chart
func uninstallChart(releaseName, namespace, kubeconfigPath string) error {
	args := []string{
		"uninstall",
		releaseName,
		"--kubeconfig", kubeconfigPath,
		"--timeout", "2m", // Helm-level timeout
	}

	if namespace != "" {
		args = append(args, "--namespace", namespace)
	}

	log.Printf("Running: helm %v", args)

	// Add context timeout (3 minutes to allow helm's internal 2m timeout plus buffer)
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
