package forge

import (
	"errors"
	"os"

	"github.com/alexandremahdhaoui/forge/pkg/flaterrors"

	"sigs.k8s.io/yaml"
)

const (
	// ConfigPath is the default path to the forge configuration file.
	ConfigPath = "forge.yaml"
)

// Spec represents the forge configuration.
// It is read from the forge.yaml file.
type Spec struct {
	// Name is the name of the project.
	Name string `json:"name"`

	// Path to the artifact store. The artifact store is a yaml data structures that
	// tracks the name, timestamp etc of all built artifacts
	ArtifactStorePath string `json:"artifactStorePath"`

	// Kindenv holds the configuration for the kindenv tool.
	Kindenv Kindenv `json:"kindenv"`
	// LocalContainerRegistry holds the configuration for the local-container-registry tool.
	LocalContainerRegistry LocalContainerRegistry `json:"localContainerRegistry"`
	// GenerateOpenAPI holds the configuration for generating OpenAPI client/server code.
	GenerateOpenAPI *GenerateOpenAPIConfig `json:"generateOpenAPI,omitempty"`

	// Build holds the build configuration
	Build Build `json:"build"`

	// Test holds the test stage configurations
	Test []TestSpec `json:"test"`
}

var errReadingProjectConfig = errors.New("error reading project config")

// ReadSpec reads the forge configuration from the forge.yaml file.
// It returns a Spec struct and an error if the file cannot be read or parsed.
func ReadSpec() (Spec, error) {
	return ReadSpecFromPath(ConfigPath)
}

// ReadSpecFromPath reads the forge configuration from the specified file path.
// It returns a Spec struct and an error if the file cannot be read or parsed.
func ReadSpecFromPath(path string) (Spec, error) {
	b, err := os.ReadFile(path) //nolint:varnamelen
	if err != nil {
		return Spec{}, flaterrors.Join(err, errReadingProjectConfig)
	}

	out := Spec{} //nolint:exhaustruct // unmarshal

	if err := yaml.Unmarshal(b, &out); err != nil {
		return Spec{}, flaterrors.Join(err, errReadingProjectConfig)
	}

	return out, nil
}
