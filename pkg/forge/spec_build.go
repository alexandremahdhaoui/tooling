package forge

// Build holds the build configuration
type Build struct {
	// Specs holds the list of artifacts to build
	Specs []BuildSpec `json:"specs"`
}

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
	Engine string `json:"builder"`
}
