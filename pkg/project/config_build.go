package project

// BinarySpec defines the configuration for building a binary.
type BinarySpec struct {
	// Name of the binary
	Name string `json:"name"`
	// The destination directory of the binary, default is ./build/bin
	Destination string `json:"destination"`
	// Path to the source code that must be built (e.g. ./cmd/<NAME>)
	Source string `json:"source"`
	// The url to an executable builder (reserved for future use).
	// e.g. "go://github.com/alexandremahdhaoui/tooling/cmd/build-go"
	// or just "go://build-go" if it's in github.com/alexandremahdhaoui/tooling
	Builder string `json:"builder"`
}

type ContainerSpec struct {
	// Name of the container image
	Name string `json:"name"`
	// Path to the Containerfile
	File string `json:"file"` // e.g. ./containers/<NAME>/Containerfile
}

type BuildSpec struct {
	Binary    BinarySpec    `json:"binary"`
	Container ContainerSpec `json:"container"`
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
