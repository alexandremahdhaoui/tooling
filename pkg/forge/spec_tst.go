package forge

import "time"

// TestSpec defines a test stage configuration
type TestSpec struct {
	// Name is the test stage name (e.g., "unit", "integration", "e2e")
	Name string `json:"name"`

	// Testenv orchestrates test environment setup (create/delete)
	// Defaults to "go://test-report" if not specified
	// Can be "noop" or "" to use default
	// Examples: "alias://k8senv", "go://testenv"
	Testenv string `json:"testenv,omitempty"`

	// Runner implements the run method to execute tests
	// Examples: "go://test-runner-go", "shell://bash ./scripts/run-test.sh"
	Runner string `json:"runner"`
}

// Validate validates the TestSpec
func (ts *TestSpec) Validate() error {
	errs := NewValidationErrors()

	// Validate required fields
	if err := ValidateRequired(ts.Name, "name", "TestSpec"); err != nil {
		errs.Add(err)
	}

	// Validate runner URI
	if err := ValidateURI(ts.Runner, "TestSpec.runner"); err != nil {
		errs.Add(err)
	}

	// Validate testenv URI if specified and not empty/noop
	if ts.Testenv != "" && ts.Testenv != "noop" {
		if err := ValidateURI(ts.Testenv, "TestSpec.testenv"); err != nil {
			errs.Add(err)
		}
	}

	return errs.ErrorOrNil()
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

	// TmpDir is the temporary directory for this test environment
	// All testenv-subengines write their files here
	// Format: /tmp/forge-test-{stage}-{testID}/
	TmpDir string `json:"tmpDir,omitempty"`

	// Files maps file keys to relative paths (relative to TmpDir)
	// Keys are namespaced by engine name (e.g., "testenv-kind.kubeconfig")
	Files map[string]string `json:"files,omitempty"`

	// ManagedResources lists all files/directories created for this environment
	// Used for cleanup on delete
	ManagedResources []string `json:"managedResources"`

	// Metadata holds engine-specific data, namespaced by engine name
	// Keys are in format "engineName.key" (e.g., "testenv-kind.clusterName")
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
