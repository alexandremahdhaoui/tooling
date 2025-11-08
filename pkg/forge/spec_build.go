package forge

// Build holds the list of artifacts to build
type Build []BuildSpec

// BuildSpec represents a single artifact to build
type BuildSpec struct {
	// Name of the artifact to build
	Name string `json:"name"`
	// Path to the source, e.g.:
	// - ./cmd/<NAME>
	// - ./containers/<NAME>/Containerfile
	Src string `json:"src"`
	// The destination of the artifact, e.g.:
	// - "./build/bin/<NAME>"
	// - can be left empty for container images
	Dest string `json:"dest,omitempty"`
	// Engine that will build this artifact, e.g.:
	// - go://build-container (go://github.com/alexandremahdhaoui/forge/cmd/build-container)
	// - go://build-go        (go://github.com/alexandremahdhaoui/forge/cmd/build-go)
	Engine string `json:"engine"`
}

// Validate validates the BuildSpec
func (bs *BuildSpec) Validate() error {
	errs := NewValidationErrors()

	// Validate required fields
	if err := ValidateRequired(bs.Name, "name", "BuildSpec"); err != nil {
		errs.Add(err)
	}
	if err := ValidateRequired(bs.Src, "src", "BuildSpec"); err != nil {
		errs.Add(err)
	}

	// Validate engine URI
	if err := ValidateURI(bs.Engine, "BuildSpec.engine"); err != nil {
		errs.Add(err)
	}

	return errs.ErrorOrNil()
}
