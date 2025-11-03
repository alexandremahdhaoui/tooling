package forge

import "time"

// TestSpec defines a test stage configuration
type TestSpec struct {
	// Name is the test stage name (e.g., "unit", "integration", "e2e")
	Name string `json:"name"`

	// Engine implements create/get/delete/list methods for test environments
	// Can be "noop" or "" to indicate no environment management needed
	// Examples: "go://test-integration", "noop"
	Engine string `json:"engine"`

	// Runner implements the run method to execute tests
	// Examples: "go://test-runner-go", "shell://bash ./scripts/run-test.sh"
	Runner string `json:"runner"`
}

// TestEnvironment represents a test environment instance
type TestEnvironment struct {
	// ID is the unique identifier for this test environment
	ID string `json:"id"`

	// Name is the test stage name (e.g., "integration", "e2e")
	Name string `json:"name"`

	// Status tracks the current state of the environment
	// Values: "created", "running", "passed", "failed", "partially_deleted"
	Status string `json:"status"`

	// CreatedAt is when the environment was created
	CreatedAt time.Time `json:"createdAt"`

	// UpdatedAt is when the environment was last updated
	UpdatedAt time.Time `json:"updatedAt"`

	// ArtifactPath is the root directory for test artifacts
	ArtifactPath string `json:"artifactPath,omitempty"`

	// KubeconfigPath is the path to the kubeconfig file (for kindenv-based tests)
	KubeconfigPath string `json:"kubeconfigPath,omitempty"`

	// RegistryConfig holds local container registry configuration
	RegistryConfig map[string]string `json:"registryConfig,omitempty"`

	// ManagedResources lists all files/directories created for this environment
	// Used for cleanup on delete
	ManagedResources []string `json:"managedResources"`

	// Metadata holds engine-specific data
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Status constants for test environments
const (
	TestStatusCreated          = "created"
	TestStatusRunning          = "running"
	TestStatusPassed           = "passed"
	TestStatusFailed           = "failed"
	TestStatusPartiallyDeleted = "partially_deleted"
)
