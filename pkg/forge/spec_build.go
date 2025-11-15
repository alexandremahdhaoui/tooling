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
	// - go://container-build (go://github.com/alexandremahdhaoui/forge/cmd/container-build)
	// - go://go-build        (go://github.com/alexandremahdhaoui/forge/cmd/go-build)
	Engine string `json:"engine"`
	// Spec contains engine-specific configuration (free-form)
	// Supports fields like: command, args, env, envFile, workDir
	// The exact fields supported depend on the engine being used
	Spec map[string]interface{} `json:"spec,omitempty"`
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
