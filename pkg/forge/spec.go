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

	// Kindenv holds the configuration for the kindenv tool.
	Kindenv Kindenv `json:"kindenv"`
	// LocalContainerRegistry holds the configuration for the local-container-registry tool.
	LocalContainerRegistry LocalContainerRegistry `json:"localContainerRegistry"`
	// OAPICodegenHelper holds the configuration for the oapi-codegen-helper tool.
	OAPICodegenHelper OAPICodegenHelper `json:"oapiCodegenHelper"`

	// Build holds the list of artifacts to build
	Build Build `json:"build"`
}

var errReadingProjectConfig = errors.New("error reading project config")

// ReadSpec reads the forge configuration from the forge.yaml file.
// It returns a Spec struct and an error if the file cannot be read or parsed.
func ReadSpec() (Spec, error) {
	b, err := os.ReadFile(ConfigPath) //nolint:varnamelen
	if err != nil {
		return Spec{}, flaterrors.Join(err, errReadingProjectConfig)
	}

	out := Spec{} //nolint:exhaustruct // unmarshal

	if err := yaml.Unmarshal(b, &out); err != nil {
		return Spec{}, flaterrors.Join(err, errReadingProjectConfig)
	}

	return out, nil
}
