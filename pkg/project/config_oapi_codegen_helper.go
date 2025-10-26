package project

// OAPICodegenHelper holds the configuration for the oapi-codegen-helper tool.
type OAPICodegenHelper struct {
	// Specs is a list of OpenAPI specifications to generate code from.
	Specs []OAPICodegenHelperSpec `json:"specs"`
	// Defaults holds the default values for the OpenAPI specifications.
	Defaults OAPICodegenHelperDefaults `json:"defaults"`
}

// OAPICodegenHelperSpec holds the configuration for a single OpenAPI specification.
type OAPICodegenHelperSpec struct {
	// Name is the name of the OpenAPI specification.
	Name string `json:"name"`
	// Versions is a list of versions for the OpenAPI specification.
	Versions []string `json:"versions"`

	// Client holds the configuration for generating the client code.
	Client GenOpts `json:"client"`
	// Server holds the configuration for generating the server code.
	Server GenOpts `json:"server"`

	// Source is the path to the OpenAPI specification file.
	Source string `json:"source,omitempty"`
	// DestinationDir is the directory where the generated code will be placed.
	DestinationDir string `json:"destinationDir,omitempty"`
}

// GenOpts holds the configuration for generating either the client or server code.
type GenOpts struct {
	// Enabled indicates whether to generate the code.
	Enabled bool `json:"enabled"`
	// PackageName is the name of the package for the generated code.
	PackageName string `json:"packageName"`
}

// OAPICodegenHelperDefaults holds the default values for the OpenAPI specifications.
type OAPICodegenHelperDefaults struct {
	// SourceDir is the default directory where the OpenAPI specification files are located.
	SourceDir string `json:"sourceDir"`
	// DestinationDir is the default directory where the generated code will be placed.
	DestinationDir string `json:"destinationDir"`
}
