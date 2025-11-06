//go:build integration

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestForgePassesTestenvInfoToTests verifies that forge passes testenv information
// (tmpDir, artifact files, metadata) to integration tests via environment variables.
// This test expects to be run in an environment created by testenv-kind with testenv-helm-install.
func TestForgePassesTestenvInfoToTests(t *testing.T) {
	// Verify FORGE_TESTENV_TMPDIR is set
	testenvTmpDir := os.Getenv("FORGE_TESTENV_TMPDIR")
	if testenvTmpDir == "" {
		t.Fatal("FORGE_TESTENV_TMPDIR not set - forge must pass testenv tmpDir to tests")
	}

	t.Logf("Testenv tmpDir: %s", testenvTmpDir)

	// Verify artifact files environment variables are set
	kubeconfigEnv := os.Getenv("FORGE_ARTIFACT_TESTENV_KIND_KUBECONFIG")
	if kubeconfigEnv == "" {
		t.Fatal("FORGE_ARTIFACT_TESTENV_KIND_KUBECONFIG not set - forge must pass artifact files to tests")
	}
	t.Logf("Kubeconfig path from env: %s", kubeconfigEnv)

	// Verify kubeconfig file exists
	if _, err := os.Stat(kubeconfigEnv); os.IsNotExist(err) {
		t.Fatalf("Kubeconfig file does not exist: %s", kubeconfigEnv)
	}

	// Verify we can read the kubeconfig
	kubeconfigContent, err := os.ReadFile(kubeconfigEnv)
	if err != nil {
		t.Fatalf("Failed to read kubeconfig: %v", err)
	}
	if len(kubeconfigContent) == 0 {
		t.Fatal("Kubeconfig is empty")
	}
	t.Logf("Kubeconfig size: %d bytes", len(kubeconfigContent))

	// Verify metadata is passed
	clusterName := os.Getenv("FORGE_METADATA_TESTENV_KIND_CLUSTERNAME")
	if clusterName == "" {
		t.Error("FORGE_METADATA_TESTENV_KIND_CLUSTERNAME not set")
	} else {
		t.Logf("Cluster name: %s", clusterName)
	}

	// Test that we can access the cluster using kubectl
	testClusterAccess(t, kubeconfigEnv, clusterName)

	// Test that we can verify helm chart deployment
	testHelmChartDeployment(t, kubeconfigEnv)
}

// testClusterAccess verifies we can access the Kubernetes cluster using the kubeconfig.
func testClusterAccess(t *testing.T, kubeconfigPath, expectedClusterName string) {
	t.Run("ClusterAccess", func(t *testing.T) {
		// Run kubectl cluster-info
		cmd := exec.Command("kubectl", "cluster-info", "--kubeconfig", kubeconfigPath)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("kubectl cluster-info failed: %v\nOutput: %s", err, string(output))
		}
		t.Logf("Cluster info:\n%s", string(output))

		// Run kubectl get nodes
		cmd = exec.Command("kubectl", "get", "nodes", "--kubeconfig", kubeconfigPath)
		output, err = cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("kubectl get nodes failed: %v\nOutput: %s", err, string(output))
		}
		t.Logf("Nodes:\n%s", string(output))

		// Verify current context matches expected cluster name
		if expectedClusterName != "" {
			cmd = exec.Command("kubectl", "config", "current-context", "--kubeconfig", kubeconfigPath)
			output, err = cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("kubectl config current-context failed: %v\nOutput: %s", err, string(output))
			}
			context := string(output)
			t.Logf("Current context: %s", context)

			// Context should contain the cluster name (format: kind-{clusterName})
			expectedContext := "kind-" + expectedClusterName
			if context[:len(context)-1] != expectedContext {
				t.Errorf("Expected context to be %s, got %s", expectedContext, context)
			}
		}
	})
}

// testHelmChartDeployment verifies that the helm chart was deployed successfully.
func testHelmChartDeployment(t *testing.T, kubeconfigPath string) {
	t.Run("HelmChartDeployment", func(t *testing.T) {
		// Check if testenv-helm-install was configured
		chartName := os.Getenv("FORGE_METADATA_TESTENV_HELM_INSTALL_CHART_0_NAME")
		if chartName == "" {
			t.Fatal("FORGE_METADATA_TESTENV_HELM_INSTALL_CHART_0_NAME not set - integration testenv should have helm chart configured")
		}

		chartNamespace := os.Getenv("FORGE_METADATA_TESTENV_HELM_INSTALL_CHART_0_NAMESPACE")
		if chartNamespace == "" {
			t.Fatal("FORGE_METADATA_TESTENV_HELM_INSTALL_CHART_0_NAMESPACE not set - chart namespace should be passed via metadata")
		}

		t.Logf("Verifying chart: %s in namespace: %s", chartName, chartNamespace)

		// Wait for namespace to exist
		cmd := exec.Command("kubectl", "get", "namespace", chartNamespace, "--kubeconfig", kubeconfigPath)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Namespace %s does not exist: %v\nOutput: %s", chartNamespace, err, string(output))
		}

		// Get pods in the namespace
		cmd = exec.Command("kubectl", "get", "pods",
			"-n", chartNamespace,
			"--kubeconfig", kubeconfigPath,
			"-o", "wide")
		output, err = cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to get pods in namespace %s: %v\nOutput: %s", chartNamespace, err, string(output))
		}
		t.Logf("Pods in namespace %s:\n%s", chartNamespace, string(output))

		// Verify at least one pod exists
		cmd = exec.Command("kubectl", "get", "pods",
			"-n", chartNamespace,
			"--kubeconfig", kubeconfigPath,
			"--no-headers")
		output, err = cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to get pod count: %v\nOutput: %s", err, string(output))
		}
		if len(output) == 0 {
			t.Errorf("No pods found in namespace %s - helm chart may not be deployed", chartNamespace)
		}
	})
}

// TestConstructFullPathFromRelative verifies that we can construct full paths
// from the testenv tmpDir and relative artifact file paths.
func TestConstructFullPathFromRelative(t *testing.T) {
	testenvTmpDir := os.Getenv("FORGE_TESTENV_TMPDIR")
	if testenvTmpDir == "" {
		t.Fatal("FORGE_TESTENV_TMPDIR not set - forge must pass testenv tmpDir to tests")
	}

	// Test with kubeconfig
	kubeconfigRel := "kubeconfig" // This is what's stored in env.Files
	kubeconfigAbs := filepath.Join(testenvTmpDir, kubeconfigRel)

	// Verify the constructed path matches what forge passed
	kubeconfigEnv := os.Getenv("FORGE_ARTIFACT_TESTENV_KIND_KUBECONFIG")
	if kubeconfigEnv == "" {
		t.Fatal("FORGE_ARTIFACT_TESTENV_KIND_KUBECONFIG not set")
	}

	if kubeconfigAbs != kubeconfigEnv {
		t.Errorf("Constructed path %s doesn't match env path %s", kubeconfigAbs, kubeconfigEnv)
	}

	t.Logf("Successfully verified path construction: %s", kubeconfigAbs)
}
