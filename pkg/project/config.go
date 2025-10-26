package project

import (
	"errors"
	"os"

	"github.com/alexandremahdhaoui/tooling/pkg/flaterrors"

	"sigs.k8s.io/yaml"
)

const (
	// ConfigPath is the default path to the project configuration file.
	ConfigPath = ".project.yaml"
)

// ----------------------------------------------------- PROJECT CONFIG --------------------------------------------- //

// Config represents the project configuration.
// It is read from the .project.yaml file.
type Config struct {
	// Name is the name of the project.
	Name string `json:"name"`

	// Kindenv holds the configuration for the kindenv tool.
	Kindenv Kindenv `json:"kindenv"`
	// LocalContainerRegistry holds the configuration for the local-container-registry tool.
	LocalContainerRegistry LocalContainerRegistry `json:"localContainerRegistry"`
	// OAPICodegenHelper holds the configuration for the oapi-codegen-helper tool.
	OAPICodegenHelper OAPICodegenHelper `json:"oapiCodegenHelper"`
}

var errReadingProjectConfig = errors.New("error reading project config")

// ReadConfig reads the project configuration from the .project.yaml file.
// It returns a Config struct and an error if the file cannot be read or parsed.
func ReadConfig() (Config, error) {
	b, err := os.ReadFile(ConfigPath) //nolint:varnamelen
	if err != nil {
		return Config{}, flaterrors.Join(err, errReadingProjectConfig)
	}

	out := Config{} //nolint:exhaustruct // unmarshal

	if err := yaml.Unmarshal(b, &out); err != nil {
		return Config{}, flaterrors.Join(err, errReadingProjectConfig)
	}

	return out, nil
}
