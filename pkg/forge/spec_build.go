package forge

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

type Build struct {
	// Path to the artifact store. The artifact store is a yaml data structures that
	// tracks the name, timestamp etc of all built artifacts
	ArtifactStorePath string      `json:"artifactStorePath"`
	Specs             []BuildSpec `json:"specs"`
}

type Artifact struct {
	// The name of the artifact
	Name string `json:"name"`
	// Type of artifact
	Type string `json:"type"` // e.g.: "container" or "binary"
	// Location of the artifact (can be a url or the path to a file, which must start as a url like file://)
	Location string `json:"location"`
	// Timestamp when the artifact was built
	Timestamp string `json:"timestamp"`
	// Version is the hash/commit
	Version string `json:"version"`
}

type ArtifactStore struct {
	Artifacts []Artifact `json:"artifacts"`
}
