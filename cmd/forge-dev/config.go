// Copyright 2024 Alexandre Mahdhaoui
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// ConfigFileName is the expected name of the forge-dev configuration file.
const ConfigFileName = "forge-dev.yaml"

// EngineType is the contract a generated MCP engine implements: the tool
// list and framework wiring that contract fixes. It is derived from kind,
// and it survives as the templates' own vocabulary.
type EngineType string

const (
	// EngineTypeBuilder is for build engines that produce artifacts.
	EngineTypeBuilder EngineType = "builder"
	// EngineTypeTestRunner is for test runner engines.
	EngineTypeTestRunner EngineType = "test-runner"
	// EngineTypeTestEnvSubengine is for test environment subengines.
	EngineTypeTestEnvSubengine EngineType = "testenv-subengine"
	// EngineTypeDependencyDetector is for dependency detector engines.
	EngineTypeDependencyDetector EngineType = "dependency-detector"
	// EngineTypeGeneric is for engines whose tools are declared in
	// forge-dev.yaml rather than fixed by the engine family. Their inputs and
	// outputs are schemas from components.schemas, so a sibling repository can
	// generate an engine forge has never heard of. It is the default profile:
	// an mcp-server with no profile key and a layout.tools list is generic.
	EngineTypeGeneric EngineType = "generic"
)

// ValidProfiles are the contract kinds under their old spelling. profile:
// was folded into kind: on 2026-09-08 and is accepted only so a checkout
// that predates the fold still generates; it is refused after the fleet has
// moved. Generic is not among them: it is kind: mcp-server plus a
// layout.tools list.
var ValidProfiles = []EngineType{
	EngineTypeBuilder,
	EngineTypeTestRunner,
	EngineTypeTestEnvSubengine,
	EngineTypeDependencyDetector,
}

// ContractKinds are the kinds whose tools, inputs and outputs come from a
// contract rather than from this file. An engine of one of these kinds
// implements the contract's tools and forge-dev generates the wiring; what
// the contract says is not this file's to restate.
var ContractKinds = []string{
	string(EngineTypeBuilder),
	string(EngineTypeTestRunner),
	string(EngineTypeTestEnvSubengine),
	string(EngineTypeDependencyDetector),
}

// isContractKind answers whether the kind names a contract.
func isContractKind(kind string) bool {
	for _, k := range ContractKinds {
		if kind == k {
			return true
		}
	}

	return false
}

// isEngineKind answers whether the kind generates an MCP engine: a contract
// kind, or mcp-server, whose tools come from layout.tools instead.
func isEngineKind(kind string) bool {
	return kind == KindMCPServer || isContractKind(kind)
}

// The kinds forge-dev knows. A kind says what the generated program is; a
// generator fills one kind and language cell. An unknown kind is allowed
// exactly when a generator: URI owns it.
const (
	// KindMCPServer answers MCP over stdio. Layout: tools.
	KindMCPServer = "mcp-server"
	// KindRestAPI serves the operations of the OpenAPI paths over HTTP.
	// Defined; no builtin cell yet, so it needs a generator: URI.
	KindRestAPI = "rest-api"
	// KindCLI parses argv into commands. Layout: commands.
	KindCLI = "cli"
	// KindBinary is a plain entrypoint. No layout, no generated code; the
	// runnable manifest and the docs are the whole output.
	KindBinary = "binary"
)

// BuiltinKinds are the kinds with builtin behavior: the four programs
// forge-dev knows how to write, plus one per contract it can wire.
var BuiltinKinds = append(
	[]string{KindMCPServer, KindRestAPI, KindCLI, KindBinary},
	ContractKinds...,
)

// Config represents the forge-dev.yaml configuration file.
type Config struct {
	// Name is the engine name (required).
	// Must be lowercase alphanumeric with hyphens, starting with a letter.
	Name string `yaml:"name"`

	// Kind is what the generated program is (required): a contract kind
	// (builder, test-runner, testenv-subengine, dependency-detector) whose
	// tools that contract fixes, mcp-server whose tools are layout.tools,
	// rest-api, cli, binary, or a custom name owned by a generator.
	Kind string `yaml:"kind"`

	// Profile is the contract kind's old spelling, folded into kind on
	// 2026-09-08. It is read only so a checkout written before the fold
	// still generates, and it is refused once the fleet has moved.
	Profile string `yaml:"profile,omitempty"`

	// Generator names a forge:// engine that emits this kind and language
	// cell instead of a builtin. Required for a custom kind.
	Generator string `yaml:"generator,omitempty"`

	// ConfigGenerator names a forge:// engine that emits the config keys
	// of a cell from the Spec schema: typed loading, flag beats env beats
	// default, unknown flag fails loud. The rest of the cell keeps
	// everything else, so the spec stays the one source of the keys.
	ConfigGenerator ConfigGeneratorConfig `yaml:"configGenerator,omitempty"`

	// Layout is the kind's vocabulary: tools for mcp-server, commands for
	// cli, anything the owning generator defines for a custom kind.
	Layout *LayoutConfig `yaml:"layout,omitempty"`

	// Type is removed. It fails validation with one line naming kind and
	// profile, so a stale file never generates silently.
	Type string `yaml:"type,omitempty"`

	// Version is the engine version in semver format (required).
	Version string `yaml:"version"`

	// Description is a human-readable description of the engine (optional).
	Description string `yaml:"description,omitempty"`

	// Language selects the template set for generated code. Absent means go.
	// Only the generic engine type generates in another language.
	Language string `yaml:"language,omitempty"`

	// Runtime declares the inputs a runnable built from this engine needs:
	// environment variables and files that must exist before it runs. They
	// land in zz_generated.runnable.yaml, never in hand-written yaml.
	Runtime *RuntimeConfig `yaml:"runtime,omitempty"`

	// Capabilities is what a builder declares it can do. It is generated
	// into the engine's build and config-validate tools, so a platform the
	// engine did not declare is refused before its own code runs. Absent
	// means the host platform only.
	Capabilities *CapabilitiesConfig `yaml:"capabilities,omitempty"`

	// OpenAPI contains OpenAPI spec configuration.
	OpenAPI OpenAPIConfig `yaml:"openapi"`

	Proto ProtoConfig `yaml:"proto,omitempty"`

	Wiring WiringConfig `yaml:"wiring,omitempty"`

	// Generate contains code generation settings.
	Generate GenerateConfig `yaml:"generate"`
}

// CapabilitiesConfig is the capabilities block of a builder.
type CapabilitiesConfig struct {
	// Platforms is `any`, `host`, or a list of os/arch pairs. A scalar and
	// a list both parse, so `platforms: any` reads as the word it is.
	Platforms PlatformsConfig `yaml:"platforms,omitempty"`

	// Frozen says the engine reads the `frozen` build input: it proves the
	// recorded dependency lock before building and never repairs it. forge
	// sends a repo's `frozen:` setting only to engines that declare this.
	Frozen bool `yaml:"frozen,omitempty"`
}

// PlatformsConfig is a platform declaration that parses from a scalar or a
// list.
type PlatformsConfig []string

func (p *PlatformsConfig) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		var one string
		if err := node.Decode(&one); err != nil {
			return err
		}

		*p = PlatformsConfig{one}

		return nil
	}

	var many []string
	if err := node.Decode(&many); err != nil {
		return err
	}

	*p = PlatformsConfig(many)

	return nil
}

// platforms is the declaration as the generated code carries it: nil for
// an engine that declares nothing, which the framework reads as host only.
func (c *Config) platforms() []string {
	if c.Capabilities == nil {
		return nil
	}

	return []string(c.Capabilities.Platforms)
}

// frozen is whether the engine declares it reads a frozen input.
func (c *Config) frozen() bool {
	return c.Capabilities != nil && c.Capabilities.Frozen
}

// RuntimeConfig declares run-time inputs of the engine's runnable.
type RuntimeConfig struct {
	// Env names environment variables that must be set to run.
	Env []string `yaml:"env,omitempty"`
	// Files names paths, relative to the repo root, that must exist to run.
	Files []string `yaml:"files,omitempty"`
}

// ValidLanguages are the template sets forge-dev can generate.
var ValidLanguages = []string{"go", "rust", "python", "typescript"}

// ConfigGeneratorConfig names the engine that emits the config loader and
// the directory its files land in. A plain string is the engine with no
// directory of its own.
type ConfigGeneratorConfig struct {
	Engine    string `yaml:"engine,omitempty"`
	OutputDir string `yaml:"outputDir,omitempty"`
}

func (c *ConfigGeneratorConfig) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		return node.Decode(&c.Engine)
	}

	type block ConfigGeneratorConfig

	var decoded block
	if err := node.Decode(&decoded); err != nil {
		return err
	}

	*c = ConfigGeneratorConfig(decoded)

	return nil
}

func (c *Config) declaresConfigGenerator() bool {
	return c.ConfigGenerator.Engine != ""
}

// LayoutConfig is the kind vocabulary. Exactly one block fits a builtin
// kind; a custom kind carries whatever its generator defines in Extra.
type LayoutConfig struct {
	// Tools declares the MCP tools of an mcp-server without a profile.
	Tools []ToolConfig `yaml:"tools,omitempty"`
	// Commands declares the subcommands of a cli.
	Commands []CommandConfig `yaml:"commands,omitempty"`
	// Extra carries a custom kind's layout, validated by its generator.
	Extra map[string]interface{} `yaml:",inline"`
}

// CommandConfig declares one cli subcommand. The generated dispatcher routes
// it to a handler the author writes.
type CommandConfig struct {
	// Name is the subcommand as typed on the command line.
	Name string `yaml:"name"`
	// Description is what the subcommand does. Shown in help.
	Description string `yaml:"description"`
}

// engineType derives the contract the templates key on. A contract kind
// names it; mcp-server declares its own tools and is generic. profile: is
// the old spelling and still answers, until it is refused.
func (c *Config) engineType() EngineType {
	switch {
	case c.Profile != "":
		return EngineType(c.Profile)
	case isContractKind(c.Kind):
		return EngineType(c.Kind)
	default:
		return EngineTypeGeneric
	}
}

// tools answers the declared MCP tools of a generic mcp-server.
func (c *Config) tools() []ToolConfig {
	if c.Layout == nil {
		return nil
	}

	return c.Layout.Tools
}

// commands answers the declared subcommands of a cli.
func (c *Config) commands() []CommandConfig {
	if c.Layout == nil {
		return nil
	}

	return c.Layout.Commands
}

func isBuiltinKind(kind string) bool {
	for _, k := range BuiltinKinds {
		if kind == k {
			return true
		}
	}

	return false
}

// OpenAPIConfig contains OpenAPI specification configuration.
type OpenAPIConfig struct {
	// SpecPath is the path to the OpenAPI spec file, relative to forge-dev.yaml.
	SpecPath string `yaml:"specPath"`
}

// GenerateConfig contains code generation settings.
type GenerateConfig struct {
	// PackageName is the Go package name for generated code.
	PackageName string `yaml:"packageName"`
	// BuildFunc is the function name for builder engines (default: "Build").
	BuildFunc string `yaml:"buildFunc,omitempty"`
	// RunFunc is the function name for test-runner engines (default: "Run").
	RunFunc string `yaml:"runFunc,omitempty"`
	// CreateFunc is the function name for testenv-subengine create operation (default: "Create").
	CreateFunc string `yaml:"createFunc,omitempty"`
	// DeleteFunc is the function name for testenv-subengine delete operation (default: "Delete").
	DeleteFunc string `yaml:"deleteFunc,omitempty"`
	// SpecTypes configures external spec types generation (optional).
	SpecTypes *SpecTypesConfig `yaml:"specTypes,omitempty"`
	// Tools is removed. It fails validation naming layout.tools.
	Tools []ToolConfig `yaml:"tools,omitempty"`
	// HandlersFunc names the constructor the engine author writes to return
	// Handlers. Defaults to NewHandlers.
	HandlersFunc string `yaml:"handlersFunc,omitempty"`
	// DocsBaseURL is the raw content base URL used for remote documentation
	// fetching. Defaults to DefaultDocsBaseURL, which points at the forge
	// repository. A sibling repository must set its own.
	DocsBaseURL string `yaml:"docsBaseURL,omitempty"`
}

// ToolConfig declares one MCP tool of a generic engine. Input and Output name
// schemas that must exist in components.schemas of the OpenAPI spec.
type ToolConfig struct {
	// Name is the MCP tool name as callers see it.
	Name string `yaml:"name"`
	// Description is what the tool does. Shown to callers.
	Description string `yaml:"description"`
	// Input names a schema in components.schemas.
	Input string `yaml:"input"`
	// Output names a schema in components.schemas. Empty means the handler
	// returns only an error.
	Output string `yaml:"output,omitempty"`
	// UseSpec parses and validates Spec from the input's spec property and
	// hands the handler a typed value.
	UseSpec bool `yaml:"useSpec,omitempty"`
}

// SpecTypesConfig contains configuration for external spec types generation.
type SpecTypesConfig struct {
	// Enabled enables generating spec types to a separate package.
	// When false (default), spec types are generated in the same package as MCP code.
	Enabled bool `yaml:"enabled"`
	// OutputPath is the path relative to project root (go.mod location) where spec types will be generated.
	// Required when Enabled is true. Example: "pkg/api/v1"
	OutputPath string `yaml:"outputPath,omitempty"`
	// PackageName is the Go package name for the spec types.
	// Required when Enabled is true. Example: "v1"
	PackageName string `yaml:"packageName,omitempty"`
}

// GetBuildFunc returns the BuildFunc from config, or "Build" if not set.
// GetHandlersFunc returns the configured Handlers constructor name, or the
// default.
func (c *Config) GetHandlersFunc() string {
	if c.Generate.HandlersFunc == "" {
		return "NewHandlers"
	}

	return c.Generate.HandlersFunc
}

// GetDocsBaseURL returns the configured documentation base URL, or the forge
// repository default when unset.
func (c *Config) GetDocsBaseURL() string {
	if c.Generate.DocsBaseURL == "" {
		return DefaultDocsBaseURL
	}

	return c.Generate.DocsBaseURL
}

func (c *Config) GetBuildFunc() string {
	if c.Generate.BuildFunc == "" {
		return "Build"
	}
	return c.Generate.BuildFunc
}

// GetRunFunc returns the RunFunc from config, or "Run" if not set.
func (c *Config) GetRunFunc() string {
	if c.Generate.RunFunc == "" {
		return "Run"
	}
	return c.Generate.RunFunc
}

// GetCreateFunc returns the CreateFunc from config, or "Create" if not set.
func (c *Config) GetCreateFunc() string {
	if c.Generate.CreateFunc == "" {
		return "Create"
	}
	return c.Generate.CreateFunc
}

// GetDeleteFunc returns the DeleteFunc from config, or "Delete" if not set.
func (c *Config) GetDeleteFunc() string {
	if c.Generate.DeleteFunc == "" {
		return "Delete"
	}
	return c.Generate.DeleteFunc
}

// ValidationError represents a single validation error.
type ValidationError struct {
	// Field is the path to the field that failed validation.
	Field string
	// Message describes the validation failure.
	Message string
}

// Error implements the error interface.
func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ReadConfig reads and parses the forge-dev.yaml configuration file from the specified directory.
func ReadConfig(dir string) (*Config, error) {
	configPath := filepath.Join(dir, ConfigFileName)

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("reading config file %s: %w", configPath, err)
	}

	var renamed renamedBlocks
	if err := yaml.Unmarshal(data, &renamed); err != nil {
		return nil, fmt.Errorf("parsing config file %s: %w", configPath, err)
	}

	if renamed.Surface != nil {
		return nil, fmt.Errorf("reading config file %s: %s", configPath, SurfaceRenamedMessage)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parsing config file %s: %w", configPath, err)
	}

	return &config, nil
}

// SurfaceRenamedMessage names the rename a stale forge-dev.yaml must follow.
const SurfaceRenamedMessage = "surface was renamed to layout on 2026-09-04, rename the block"

type renamedBlocks struct {
	Surface *yaml.Node `yaml:"surface"`
}

// nameRegexp validates that name is lowercase alphanumeric with hyphens, starting with a letter.
var nameRegexp = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

var (
	toolNameRegexp      = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)
	schemaNameRegexp    = regexp.MustCompile(`^[A-Z][A-Za-z0-9]*$`)
	exportedIdentRegexp = regexp.MustCompile(`^[A-Z][A-Za-z0-9_]*$`)
)

var reservedToolNames = map[string]bool{
	"config-validate": true,
	"docs-list":       true,
	"docs-get":        true,
}

// semverRegexp validates semantic versioning format (x.y.z).
var semverRegexp = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)

// packageNameRegexp validates Go package names.
var packageNameRegexp = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// validateCapabilities holds the capabilities block to what the framework
// can act on: only a builder has one, and platforms is the word any, the
// word host, or os/arch pairs - never a word beside a pair.
func validateCapabilities(c *Config) []ValidationError {
	if c.Capabilities == nil {
		return nil
	}

	if c.engineType() != EngineTypeBuilder {
		return []ValidationError{{
			Field:   "capabilities",
			Message: "only a builder declares capabilities; this engine's kind is " + c.Kind,
		}}
	}

	platforms := c.Capabilities.Platforms
	if len(platforms) == 0 {
		if c.Capabilities.Frozen {
			// frozen alone is a complete declaration: host only, frozen read.
			return nil
		}

		return []ValidationError{{
			Field:   "capabilities.platforms",
			Message: "declare any, host, or a list of os/arch pairs, frozen: true, or drop the block for host only",
		}}
	}

	var errs []ValidationError

	for _, p := range platforms {
		switch p {
		case "any", "host":
			if len(platforms) > 1 {
				errs = append(errs, ValidationError{
					Field:   "capabilities.platforms",
					Message: p + " stands alone; it cannot be listed beside a platform",
				})
			}
		default:
			if parts := strings.Split(p, "/"); len(parts) != 2 || parts[0] == "" || parts[1] == "" {
				errs = append(errs, ValidationError{
					Field:   "capabilities.platforms",
					Message: fmt.Sprintf("%q is not any, host, or <os>/<arch>", p),
				})
			}
		}
	}

	return errs
}

// ValidateConfig validates the configuration and returns any validation errors.
// validateDocsBaseURL checks generate.docsBaseURL when it is set. A trailing
// slash is rejected because every consumer joins it with a slash already.
func validateLanguage(c *Config) []ValidationError {
	if c.Language == "" || c.Language == "go" || c.Generator != "" {
		return nil
	}

	valid := false

	for _, l := range ValidLanguages {
		if c.Language == l {
			valid = true
		}
	}

	if !valid {
		return []ValidationError{{
			Field:   "language",
			Message: fmt.Sprintf("must be one of: %s", strings.Join(ValidLanguages, ", ")),
		}}
	}

	if isEngineKind(c.Kind) && c.engineType() != EngineTypeGeneric {
		return []ValidationError{{
			Field:   "language",
			Message: "a contract kind generates go only; declare kind: mcp-server with layout.tools for another language",
		}}
	}

	if c.Kind == KindCLI {
		return []ValidationError{{
			Field:   "language",
			Message: "the builtin cli cell generates go only; name a generator: for another language",
		}}
	}

	if c.Kind == KindRestAPI {
		return []ValidationError{{
			Field:   "language",
			Message: "the builtin rest-api cell generates go only; name a generator: for another language",
		}}
	}

	return nil
}

// validateKind enforces the two axis rules: what kinds exist, which need a
// generator, and what each kind's layout may carry.
func validateKind(c *Config) []ValidationError {
	var errors []ValidationError

	if c.Type != "" {
		errors = append(errors, ValidationError{
			Field:   "type",
			Message: "removed; write kind: " + c.Type + ", or kind: mcp-server with layout.tools",
		})
	}

	if len(c.Generate.Tools) > 0 {
		errors = append(errors, ValidationError{
			Field:   "generate.tools",
			Message: "moved; declare the tools under layout.tools",
		})
	}

	if c.Generator != "" && !strings.HasPrefix(c.Generator, "forge://") {
		errors = append(errors, ValidationError{
			Field:   "generator",
			Message: "must be a forge:// engine URI",
		})
	}

	if c.declaresConfigGenerator() && !strings.HasPrefix(c.ConfigGenerator.Engine, "forge://") {
		errors = append(errors, ValidationError{
			Field:   "configGenerator",
			Message: "must be a forge:// engine URI",
		})
	}

	if c.ConfigGenerator.OutputDir != "" && !c.declaresConfigGenerator() {
		errors = append(errors, ValidationError{
			Field:   "configGenerator.outputDir",
			Message: "an output directory needs an engine: to answer files for it",
		})
	}

	if c.ConfigGenerator.OutputDir != "" && escapesEngineDir(filepath.Clean(c.ConfigGenerator.OutputDir)) {
		errors = append(errors, ValidationError{
			Field:   "configGenerator.outputDir",
			Message: "must stay inside the engine directory, and " + c.ConfigGenerator.OutputDir + " is absolute or escapes it",
		})
	}

	switch {
	case c.Kind == "":
		errors = append(errors, ValidationError{
			Field:   "kind",
			Message: fmt.Sprintf("required field is missing; one of %s, or a custom kind with a generator:", strings.Join(BuiltinKinds, ", ")),
		})

		return errors
	case !isBuiltinKind(c.Kind) && c.Generator == "":
		errors = append(errors, ValidationError{
			Field:   "kind",
			Message: fmt.Sprintf("%q is not a builtin kind, so a generator: URI must own it", c.Kind),
		})
	}

	if c.Profile != "" {
		if c.Kind != KindMCPServer {
			errors = append(errors, ValidationError{
				Field:   "profile",
				Message: "profile was folded into kind; write kind: " + c.Profile + " and drop this key",
			})
		} else if !isValidProfile(c.Profile) {
			errors = append(errors, ValidationError{
				Field:   "profile",
				Message: fmt.Sprintf("must be one of: %s; generic is the default, drop the profile and declare layout.tools", profileStrings()),
			})
		}
	}

	if c.Kind == KindBinary && c.Layout != nil {
		errors = append(errors, ValidationError{
			Field:   "layout",
			Message: "the binary kind has no layout",
		})
	}

	if c.Kind == KindRestAPI && c.Generator == "" && c.Layout != nil {
		errors = append(errors, ValidationError{
			Field:   "layout",
			Message: "the rest-api kind's layout is the OpenAPI paths; declare operations in the spec",
		})
	}

	if c.Kind == KindCLI && c.Generator == "" && len(c.commands()) == 0 {
		errors = append(errors, ValidationError{
			Field:   "layout.commands",
			Message: "at least one command is required for the cli kind",
		})
	}

	if c.Kind != KindCLI && len(c.commands()) > 0 {
		errors = append(errors, ValidationError{
			Field:   "layout.commands",
			Message: "only the cli kind declares commands",
		})
	}

	seen := map[string]bool{}

	for i, cmd := range c.commands() {
		field := fmt.Sprintf("layout.commands[%d]", i)

		switch {
		case cmd.Name == "":
			errors = append(errors, ValidationError{Field: field + ".name", Message: "required field is missing"})
		case !toolNameRegexp.MatchString(cmd.Name):
			errors = append(errors, ValidationError{
				Field:   field + ".name",
				Message: "must be alphanumeric with hyphens or underscores, starting with a letter",
			})
		case seen[cmd.Name]:
			errors = append(errors, ValidationError{Field: field + ".name", Message: fmt.Sprintf("duplicate command name %q", cmd.Name)})
		}

		seen[cmd.Name] = true

		if cmd.Description == "" {
			errors = append(errors, ValidationError{Field: field + ".description", Message: "required field is missing"})
		}
	}

	return errors
}

func isValidProfile(p string) bool {
	for _, v := range ValidProfiles {
		if EngineType(p) == v {
			return true
		}
	}

	return false
}

func profileStrings() string {
	strs := make([]string, len(ValidProfiles))
	for i, t := range ValidProfiles {
		strs[i] = string(t)
	}

	return strings.Join(strs, ", ")
}

func validateDocsBaseURL(raw string) []ValidationError {
	if raw == "" {
		return nil
	}

	var errors []ValidationError

	parsed, err := url.Parse(raw)
	if err != nil || !parsed.IsAbs() || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		errors = append(errors, ValidationError{
			Field:   "generate.docsBaseURL",
			Message: "must be an absolute http or https URL",
		})

		return errors
	}

	if strings.HasSuffix(raw, "/") {
		errors = append(errors, ValidationError{
			Field:   "generate.docsBaseURL",
			Message: "must not end with a trailing slash",
		})
	}

	return errors
}

// validateTools checks the tools block. Cross referencing an input or output
// against components.schemas needs the parsed spec and happens in
// ValidateGenericTools instead.
func validateTools(c *Config) []ValidationError {
	var errors []ValidationError

	generic := c.Kind == KindMCPServer && c.engineType() == EngineTypeGeneric && c.Generator == ""

	if !generic {
		if len(c.tools()) > 0 && c.Generator == "" {
			errors = append(errors, ValidationError{
				Field:   "layout.tools",
				Message: "only kind: mcp-server declares tools; a contract kind's tools are the contract's",
			})
		}

		return errors
	}

	if len(c.tools()) == 0 {
		errors = append(errors, ValidationError{
			Field:   "layout.tools",
			Message: "at least one tool is required for kind: mcp-server",
		})

		return errors
	}

	if c.Generate.HandlersFunc != "" && !exportedIdentRegexp.MatchString(c.Generate.HandlersFunc) {
		errors = append(errors, ValidationError{
			Field:   "generate.handlersFunc",
			Message: "must be an exported Go identifier",
		})
	}

	seen := map[string]bool{}

	for i, t := range c.tools() {
		field := fmt.Sprintf("layout.tools[%d]", i)
		errors = append(errors, validateTool(field, t, seen)...)
	}

	return errors
}

func validateTool(field string, t ToolConfig, seen map[string]bool) []ValidationError {
	var errors []ValidationError

	switch {
	case t.Name == "":
		errors = append(errors, ValidationError{
			Field: field + ".name", Message: "required field is missing",
		})
	case !toolNameRegexp.MatchString(t.Name):
		errors = append(errors, ValidationError{
			Field:   field + ".name",
			Message: "must be alphanumeric with hyphens or underscores, starting with a letter",
		})
	case reservedToolNames[t.Name]:
		errors = append(errors, ValidationError{
			Field:   field + ".name",
			Message: fmt.Sprintf("%q is reserved and registered automatically", t.Name),
		})
	case seen[t.Name]:
		errors = append(errors, ValidationError{
			Field: field + ".name", Message: fmt.Sprintf("duplicate tool name %q", t.Name),
		})
	}

	seen[t.Name] = true

	if t.Description == "" {
		errors = append(errors, ValidationError{
			Field: field + ".description", Message: "required field is missing",
		})
	}

	if t.Input == "" {
		errors = append(errors, ValidationError{
			Field: field + ".input", Message: "required field is missing",
		})
	} else if !schemaNameRegexp.MatchString(t.Input) {
		errors = append(errors, ValidationError{
			Field:   field + ".input",
			Message: "must be a schema name from components.schemas, CamelCase starting with an uppercase letter",
		})
	}

	if t.Output != "" && !schemaNameRegexp.MatchString(t.Output) {
		errors = append(errors, ValidationError{
			Field:   field + ".output",
			Message: "must be a schema name from components.schemas, CamelCase starting with an uppercase letter",
		})
	}

	return errors
}

func ValidateConfig(c *Config) []ValidationError {
	var errors []ValidationError

	errors = append(errors, validateDocsBaseURL(c.Generate.DocsBaseURL)...)
	errors = append(errors, validateKind(c)...)
	errors = append(errors, validateTools(c)...)
	errors = append(errors, validateLanguage(c)...)
	errors = append(errors, validateCapabilities(c)...)

	// Validate name (required)
	if c.Name == "" {
		errors = append(errors, ValidationError{
			Field:   "name",
			Message: "required field is missing",
		})
	} else if !nameRegexp.MatchString(c.Name) {
		errors = append(errors, ValidationError{
			Field:   "name",
			Message: "must be lowercase alphanumeric with hyphens, starting with a letter",
		})
	} else if len(c.Name) > 64 {
		errors = append(errors, ValidationError{
			Field:   "name",
			Message: "must be 64 characters or less",
		})
	}

	// Validate version (required)
	if c.Version == "" {
		errors = append(errors, ValidationError{
			Field:   "version",
			Message: "required field is missing",
		})
	} else if !semverRegexp.MatchString(c.Version) {
		errors = append(errors, ValidationError{
			Field:   "version",
			Message: "must be in semver format (x.y.z)",
		})
	}

	errors = append(errors, c.validateSpecSources()...)

	// Validate generate.packageName (required)
	if c.Generate.PackageName == "" {
		errors = append(errors, ValidationError{
			Field:   "generate.packageName",
			Message: "required field is missing",
		})
	} else if !packageNameRegexp.MatchString(c.Generate.PackageName) {
		errors = append(errors, ValidationError{
			Field:   "generate.packageName",
			Message: "must be a valid Go package name (lowercase alphanumeric with underscores, starting with a letter)",
		})
	}

	// Validate generate.specTypes when enabled
	if c.Generate.SpecTypes != nil && c.Generate.SpecTypes.Enabled {
		// OutputPath is required when enabled
		if c.Generate.SpecTypes.OutputPath == "" {
			errors = append(errors, ValidationError{
				Field:   "generate.specTypes.outputPath",
				Message: "required when specTypes.enabled is true",
			})
		} else if err := validateOutputPath(c.Generate.SpecTypes.OutputPath); err != nil {
			errors = append(errors, ValidationError{
				Field:   "generate.specTypes.outputPath",
				Message: err.Error(),
			})
		}

		// PackageName is required when enabled
		if c.Generate.SpecTypes.PackageName == "" {
			errors = append(errors, ValidationError{
				Field:   "generate.specTypes.packageName",
				Message: "required when specTypes.enabled is true",
			})
		} else if !packageNameRegexp.MatchString(c.Generate.SpecTypes.PackageName) {
			errors = append(errors, ValidationError{
				Field:   "generate.specTypes.packageName",
				Message: "must be a valid Go package name (lowercase alphanumeric with underscores, starting with a letter)",
			})
		}
	}

	return errors
}

// validateOutputPath validates the output path for specTypes.
func validateOutputPath(path string) error {
	if path == "" {
		return fmt.Errorf("output path cannot be empty")
	}

	// Normalize the path
	cleanPath := filepath.Clean(path)

	// Check if path is absolute
	if filepath.IsAbs(cleanPath) {
		return fmt.Errorf("output path must be relative, not absolute")
	}

	// Check if path escapes the project root
	if cleanPath == ".." || strings.HasPrefix(cleanPath, ".."+string(filepath.Separator)) {
		return fmt.Errorf("output path must not escape the project root")
	}

	// Check if path is current directory
	if cleanPath == "." {
		return fmt.Errorf("output path must not be the current directory")
	}

	return nil
}
