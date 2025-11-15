package forge

// GenerateOpenAPIConfig holds the configuration for generating OpenAPI client and server code.
type GenerateOpenAPIConfig struct {
	// Specs is a list of OpenAPI specifications to generate code from.
	Specs []GenerateOpenAPISpec `json:"specs"`
	// Defaults holds the default values for the OpenAPI specifications.
	Defaults GenerateOpenAPIDefaults `json:"defaults"`
}

// GenOpts holds the options for generating code.
type GenOpts struct {
	// Enabled indicates whether to generate code for this package.
	Enabled bool `json:"enabled"`
	// PackageName is the name of the package for the generated code.
	PackageName string `json:"packageName"`
}

// GenerateOpenAPISpec holds the configuration for a single OpenAPI specification.
type GenerateOpenAPISpec struct {
	// Name is the name of the OpenAPI specification.
	Name string `json:"name"`
	// Versions is a list of versions for the OpenAPI specification.
	Versions []string `json:"versions"`

	// Source is the path to the OpenAPI specification file.
	// If not set, the source file will be templated as: {SourceDir}/{Name}.{Version}.yaml
	Source string `json:"source,omitempty"`

	// SourceDir overrides the default source directory for this spec.
	SourceDir string `json:"sourceDir,omitempty"`

	// DestinationDir overrides the default destination directory for this spec.
	DestinationDir string `json:"destinationDir,omitempty"`

	// Client holds the configuration for generating the client code.
	Client GenOpts `json:"client"`
	// Server holds the configuration for generating the server code.
	Server GenOpts `json:"server"`
}

// GenerateOpenAPIDefaults holds the default values for the OpenAPI specifications.
type GenerateOpenAPIDefaults struct {
	// SourceDir is the default directory where the OpenAPI specification files are located.
	SourceDir string `json:"sourceDir"`
	// DestinationDir is the default directory where the generated code will be placed.
	DestinationDir string `json:"destinationDir"`
	// Engine is the code generation engine to use (e.g., "go://go-gen-openapi")
	Engine string `json:"engine"`
}
