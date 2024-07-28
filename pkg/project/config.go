package project

import (
	"errors"
	"github.com/alexandremahdhaoui/tooling/pkg/flaterrors"
	"os"
	"sigs.k8s.io/yaml"
)

const (
	ConfigPath = ".project.yaml"
)

// ----------------------------------------------------- PROJECT CONFIG --------------------------------------------- //

type Config struct {
	Name string `json:"name"`

	Kindenv                Kindenv                `json:"kindenv"`
	LocalContainerRegistry LocalContainerRegistry `json:"localContainerRegistry"`
	OAPICodegenHelper      OAPICodegenHelper      `json:"oapiCodegenHelper"`
}

var errReadingProjectConfig = errors.New("error reading project config")

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
