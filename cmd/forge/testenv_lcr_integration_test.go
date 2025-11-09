//go:build integration

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestLocalContainerRegistryPushPull verifies that we can push and pull images
// from the local container registry created by testenv-lcr.
func TestLocalContainerRegistryPushPull(t *testing.T) {
	// Verify testenv-lcr metadata is set
	registryFQDN := os.Getenv("FORGE_METADATA_TESTENV_LCR_REGISTRYFQDN")
	if registryFQDN == "" {
		t.Skip("FORGE_METADATA_TESTENV_LCR_REGISTRYFQDN not set - testenv-lcr may not be fully set up (possibly due to permissions)")
	}

	t.Logf("Registry FQDN: %s", registryFQDN)

	// Verify credentials file exists
	credentialPath := os.Getenv("FORGE_ARTIFACT_TESTENV_LCR_CREDENTIALS_YAML")
	if credentialPath == "" {
		t.Skip("FORGE_ARTIFACT_TESTENV_LCR_CREDENTIALS_YAML not set - testenv-lcr may not be fully set up")
	}

	if _, err := os.Stat(credentialPath); os.IsNotExist(err) {
		t.Skipf("Credentials file does not exist: %s - testenv-lcr may not be fully set up (possibly due to permissions)", credentialPath)
	}
	t.Logf("Credentials file: %s", credentialPath)

	// Verify CA certificate file exists
	caCrtPath := os.Getenv("FORGE_ARTIFACT_TESTENV_LCR_CA_CRT")
	if caCrtPath == "" {
		t.Fatal("FORGE_ARTIFACT_TESTENV_LCR_CA_CRT not set")
	}

	if _, err := os.Stat(caCrtPath); os.IsNotExist(err) {
		t.Fatalf("CA certificate file does not exist: %s", caCrtPath)
	}
	t.Logf("CA certificate: %s", caCrtPath)

	// Test push and pull with a minimal alpine image
	testPushPullAlpineImage(t, registryFQDN)
}

// testPushPullAlpineImage tests pushing and pulling a minimal alpine image.
func testPushPullAlpineImage(t *testing.T, registryFQDN string) {
	t.Run("PushPullAlpineImage", func(t *testing.T) {
		// Get kubeconfig for port-forward
		kubeconfigPath := os.Getenv("FORGE_ARTIFACT_TESTENV_KIND_KUBECONFIG")
		if kubeconfigPath == "" {
			t.Fatal("FORGE_ARTIFACT_TESTENV_KIND_KUBECONFIG not set")
		}

		// Get registry namespace
		namespace := os.Getenv("FORGE_METADATA_TESTENV_LCR_NAMESPACE")
		if namespace == "" {
			namespace = "testenv-lcr" // default
		}

		// Get credentials path
		credentialPath := os.Getenv("FORGE_ARTIFACT_TESTENV_LCR_CREDENTIALS_YAML")
		if credentialPath == "" {
			t.Fatal("FORGE_ARTIFACT_TESTENV_LCR_CREDENTIALS_YAML not set")
		}

		// Establish port-forward to the registry
		t.Log("Establishing port-forward to registry...")
		portForwardCmd := exec.Command("kubectl", "port-forward",
			"-n", namespace,
			"svc/testenv-lcr", "5000:5000",
			"--kubeconfig", kubeconfigPath)

		// Start port-forward in background
		if err := portForwardCmd.Start(); err != nil {
			t.Fatalf("Failed to start port-forward: %v", err)
		}
		defer func() {
			if portForwardCmd.Process != nil {
				portForwardCmd.Process.Kill()
			}
		}()

		// Wait a bit for port-forward to establish
		t.Log("Waiting for port-forward to be ready...")
		exec.Command("sleep", "2").Run()

		// Log in to the registry
		t.Log("Logging in to registry...")
		// Read credentials
		credBytes, err := os.ReadFile(credentialPath)
		if err != nil {
			t.Fatalf("Failed to read credentials: %v", err)
		}

		// Parse credentials (simple YAML parsing - look for username: and password: lines)
		var creds struct {
			Username string
			Password string
		}
		lines := strings.Split(string(credBytes), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "username:") {
				creds.Username = strings.TrimSpace(strings.TrimPrefix(line, "username:"))
			} else if strings.HasPrefix(line, "password:") {
				creds.Password = strings.TrimSpace(strings.TrimPrefix(line, "password:"))
			}
		}

		if creds.Username == "" || creds.Password == "" {
			t.Fatalf("Failed to parse credentials from file")
		}

		// Login using stdin for password
		loginCmd := exec.Command("docker", "login", registryFQDN, "--username", creds.Username, "--password-stdin")
		loginCmd.Stdin = strings.NewReader(creds.Password)
		output, err := loginCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to login to registry: %v\nOutput: %s", err, string(output))
		}
		t.Logf("Successfully logged in to registry")

		// Pull alpine image from public registry
		t.Log("Pulling alpine:latest from public registry...")
		cmd := exec.Command("docker", "pull", "alpine:latest")
		output, err = cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to pull alpine:latest: %v\nOutput: %s", err, string(output))
		}
		t.Logf("Successfully pulled alpine:latest")

		// Tag image for local registry
		localImage := fmt.Sprintf("%s/alpine:test", registryFQDN)
		t.Logf("Tagging image as %s...", localImage)
		cmd = exec.Command("docker", "tag", "alpine:latest", localImage)
		output, err = cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to tag image: %v\nOutput: %s", err, string(output))
		}

		// Push image to local registry
		t.Logf("Pushing image to local registry...")
		cmd = exec.Command("docker", "push", localImage)
		output, err = cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to push image: %v\nOutput: %s", err, string(output))
		}
		t.Logf("Successfully pushed image to local registry")

		// Remove local image to test pull
		t.Log("Removing local image to test pull...")
		cmd = exec.Command("docker", "rmi", localImage)
		output, err = cmd.CombinedOutput()
		if err != nil {
			t.Logf("Warning: Failed to remove local image (non-fatal): %v", err)
		}

		// Pull image from local registry
		t.Logf("Pulling image from local registry...")
		cmd = exec.Command("docker", "pull", localImage)
		output, err = cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to pull image from local registry: %v\nOutput: %s", err, string(output))
		}
		t.Logf("Successfully pulled image from local registry")

		// Cleanup
		t.Log("Cleaning up test images...")
		exec.Command("docker", "rmi", localImage).Run()
		exec.Command("docker", "rmi", "alpine:latest").Run()
		exec.Command("docker", "logout", registryFQDN).Run()
	})
}

// TestLocalContainerRegistryImagePullSecrets verifies that image pull secrets
// are automatically created in the configured namespaces.
func TestLocalContainerRegistryImagePullSecrets(t *testing.T) {
	// Verify testenv-lcr metadata is set
	registryFQDN := os.Getenv("FORGE_METADATA_TESTENV_LCR_REGISTRYFQDN")
	if registryFQDN == "" {
		t.Skip("FORGE_METADATA_TESTENV_LCR_REGISTRYFQDN not set - testenv-lcr may be disabled")
	}

	// Get kubeconfig
	kubeconfigPath := os.Getenv("FORGE_ARTIFACT_TESTENV_KIND_KUBECONFIG")
	if kubeconfigPath == "" {
		t.Fatal("FORGE_ARTIFACT_TESTENV_KIND_KUBECONFIG not set")
	}

	// Check if image pull secrets were created
	imagePullSecretCount := os.Getenv("FORGE_METADATA_TESTENV_LCR_IMAGEPULLSECRETCOUNT")
	if imagePullSecretCount == "" {
		t.Skip("No image pull secrets configured - skipping test")
	}

	t.Logf("Image pull secret count: %s", imagePullSecretCount)

	// Test each image pull secret
	testImagePullSecretInNamespace(t, kubeconfigPath, 0)
	testImagePullSecretInNamespace(t, kubeconfigPath, 1)
}

// testImagePullSecretInNamespace verifies an image pull secret exists in a namespace.
func testImagePullSecretInNamespace(t *testing.T, kubeconfigPath string, index int) {
	t.Run(fmt.Sprintf("ImagePullSecret_%d", index), func(t *testing.T) {
		// Get namespace and secret name from metadata
		namespaceKey := fmt.Sprintf("FORGE_METADATA_TESTENV_LCR_IMAGEPULLSECRET_%d_NAMESPACE", index)
		secretNameKey := fmt.Sprintf("FORGE_METADATA_TESTENV_LCR_IMAGEPULLSECRET_%d_SECRETNAME", index)

		namespace := os.Getenv(namespaceKey)
		if namespace == "" {
			t.Skipf("%s not set", namespaceKey)
		}

		secretName := os.Getenv(secretNameKey)
		if secretName == "" {
			t.Fatalf("%s not set", secretNameKey)
		}

		t.Logf("Checking image pull secret: %s/%s", namespace, secretName)

		// Verify secret exists
		cmd := exec.Command("kubectl", "get", "secret", secretName,
			"-n", namespace,
			"--kubeconfig", kubeconfigPath,
			"-o", "jsonpath={.type}")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to get secret: %v\nOutput: %s", err, string(output))
		}

		secretType := strings.TrimSpace(string(output))
		expectedType := "kubernetes.io/dockerconfigjson"
		if secretType != expectedType {
			t.Errorf("Expected secret type %s, got %s", expectedType, secretType)
		}

		t.Logf("✅ Image pull secret verified: %s/%s (type: %s)", namespace, secretName, secretType)

		// Verify secret has the correct data keys
		cmd = exec.Command("kubectl", "get", "secret", secretName,
			"-n", namespace,
			"--kubeconfig", kubeconfigPath,
			"-o", "jsonpath={.data}")
		output, err = cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to get secret data: %v\nOutput: %s", err, string(output))
		}

		if !strings.Contains(string(output), ".dockerconfigjson") {
			t.Errorf("Secret does not contain .dockerconfigjson key")
		}

		// Verify secret has the correct label
		cmd = exec.Command("kubectl", "get", "secret", secretName,
			"-n", namespace,
			"--kubeconfig", kubeconfigPath,
			"-o", "jsonpath={.metadata.labels.app\\.kubernetes\\.io/managed-by}")
		output, err = cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to get secret label: %v\nOutput: %s", err, string(output))
		}

		managedBy := strings.TrimSpace(string(output))
		if managedBy != "testenv-lcr" {
			t.Errorf("Expected managed-by label to be 'testenv-lcr', got '%s'", managedBy)
		}
	})
}

// TestLocalContainerRegistryDeployment verifies that the local container registry
// deployment is running in the cluster.
func TestLocalContainerRegistryDeployment(t *testing.T) {
	// Check if testenv-lcr was set up
	registryFQDN := os.Getenv("FORGE_METADATA_TESTENV_LCR_REGISTRYFQDN")
	if registryFQDN == "" {
		t.Skip("FORGE_METADATA_TESTENV_LCR_REGISTRYFQDN not set - testenv-lcr may not be fully set up (possibly due to permissions)")
	}

	// Get kubeconfig
	kubeconfigPath := os.Getenv("FORGE_ARTIFACT_TESTENV_KIND_KUBECONFIG")
	if kubeconfigPath == "" {
		t.Fatal("FORGE_ARTIFACT_TESTENV_KIND_KUBECONFIG not set")
	}

	// Get registry namespace
	namespace := os.Getenv("FORGE_METADATA_TESTENV_LCR_NAMESPACE")
	if namespace == "" {
		namespace = "testenv-lcr" // default
	}

	t.Run("RegistryDeployment", func(t *testing.T) {
		// Check deployment exists and is ready
		cmd := exec.Command("kubectl", "get", "deployment", "testenv-lcr",
			"-n", namespace,
			"--kubeconfig", kubeconfigPath,
			"-o", "jsonpath={.status.availableReplicas}")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to get deployment: %v\nOutput: %s", err, string(output))
		}

		availableReplicas := strings.TrimSpace(string(output))
		if availableReplicas != "1" {
			t.Errorf("Expected 1 available replica, got %s", availableReplicas)
		}

		t.Logf("✅ Registry deployment is running with %s replica(s)", availableReplicas)
	})

	t.Run("RegistryService", func(t *testing.T) {
		// Check service exists
		cmd := exec.Command("kubectl", "get", "service", "testenv-lcr",
			"-n", namespace,
			"--kubeconfig", kubeconfigPath,
			"-o", "jsonpath={.spec.ports[0].port}")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to get service: %v\nOutput: %s", err, string(output))
		}

		port := strings.TrimSpace(string(output))
		if port != "5000" {
			t.Errorf("Expected service port 5000, got %s", port)
		}

		t.Logf("✅ Registry service is running on port %s", port)
	})
}
