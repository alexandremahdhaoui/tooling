package project

import (
	"os"

	"gopkg.in/yaml.v3"
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

func ReadConfig() (Config, error) {
	b, err := os.ReadFile(ConfigPath) //nolint:varnamelen
	if err != nil {
		return Config{}, err // TODO: wrap err
	}

	out := Config{} //nolint:exhaustruct // unmarshal

	if err := yaml.Unmarshal(b, &out); err != nil {
		return Config{}, err // TODO: wrap err
	}

	err = nil // ensures err is nil
	if err != nil {
		return Config{}, err // TODO: wrap error.
	}

	return out, nil
}
