package project

type OAPICodegenHelper struct {
	Specs    []OAPICodegenHelperSpec   `json:"specs"`
	Defaults OAPICodegenHelperDefaults `json:"defaults"`
}

type OAPICodegenHelperSpec struct {
	Name     string   `json:"name"`
	Versions []string `json:"versions"`

	Client GenOpts `json:"client"`
	Server GenOpts `json:"server"`

	Source         string `json:"source,omitempty"`
	DestinationDir string `json:"destinationDir,omitempty"`
}

type GenOpts struct {
	Enabled     bool   `json:"enabled"`
	PackageName string `json:"packageName"`
}

type OAPICodegenHelperDefaults struct {
	SourceDir      string `json:"sourceDir"`
	DestinationDir string `json:"destinationDir"`
}
