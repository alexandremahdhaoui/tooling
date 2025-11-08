package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/alexandremahdhaoui/forge/internal/mcpserver"
	"github.com/alexandremahdhaoui/forge/internal/util"
	"github.com/alexandremahdhaoui/forge/internal/version"
	"github.com/alexandremahdhaoui/forge/pkg/forge"
	"github.com/alexandremahdhaoui/forge/pkg/mcptypes"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Version information (set via ldflags during build)
var (
	Version        = "dev"
	CommitSHA      = "unknown"
	BuildTimestamp = "unknown"
)

var versionInfo *version.Info

func init() {
	versionInfo = version.New("generate-openapi-go")
	versionInfo.Version = Version
	versionInfo.CommitSHA = CommitSHA
	versionInfo.BuildTimestamp = BuildTimestamp
}

const (
	sourceFileTemplate  = "%s.%s.yaml"
	zzGeneratedFilename = "zz_generated.oapi-codegen.go"

	clientTemplate = `---
package: %[1]s
output: %[2]s
generate:
  client: true
  models: true
  embedded-spec: true
output-options:
  # to make sure that all types are generated
  skip-prune: true
`

	serverTemplate = `---
package: %[1]s
output: %[2]s
generate:
  embedded-spec: true
  models: true
  std-http-server: true
  strict-server: true
output-options:
  skip-prune: true
`
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--mcp":
			if err := runMCPServer(); err != nil {
				log.Printf("MCP server error: %v", err)
				os.Exit(1)
			}
			return
		case "version", "--version", "-v":
			versionInfo.Print()
			return
		case "help", "--help", "-h":
			printUsage()
			return
		}
	}

	// Direct invocation
	if err := generateCode("./forge.yaml"); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`generate-openapi-go - Generate OpenAPI client and server code

Usage:
  generate-openapi-go              Generate code from forge.yaml
  generate-openapi-go --mcp        Run as MCP server
  generate-openapi-go version      Show version information

Environment Variables:
  OAPI_CODEGEN_VERSION            Version of oapi-codegen to use (default: v2.3.0)

Configuration:
  Reads from forge.yaml's generateOpenAPI section`)
}

func runMCPServer() error {
	v, _, _ := versionInfo.Get()
	server := mcpserver.New("generate-openapi-go", v)

	mcpserver.RegisterTool(server, &mcp.Tool{
		Name:        "build",
		Description: "Generate OpenAPI client and server code",
	}, handleBuild)

	return server.RunDefault()
}

func handleBuild(
	ctx context.Context,
	req *mcp.CallToolRequest,
	input mcptypes.BuildInput,
) (*mcp.CallToolResult, any, error) {
	// Get configPath from environment variable or use default
	configPath := os.Getenv("OPENAPI_CONFIG_PATH")
	if configPath == "" {
		configPath = "./forge.yaml"
	}

	log.Printf("Generating OpenAPI code from config: %s", configPath)

	if err := generateCode(configPath); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("OpenAPI code generation failed: %v", err)},
			},
			IsError: true,
		}, nil, nil
	}

	// Return artifact information
	artifact := forge.Artifact{
		Name:      "openapi-generated-code",
		Type:      "generated",
		Location:  "pkg/generated",
		Timestamp: time.Now().Format(time.RFC3339),
	}

	artifactJSON, _ := json.Marshal(artifact)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(artifactJSON)},
		},
	}, artifact, nil
}

func generateCode(configPath string) error {
	// Read forge.yaml
	config, err := forge.ReadSpecFromPath(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	if config.GenerateOpenAPI == nil {
		return fmt.Errorf("no generateOpenAPI configuration found in %s", configPath)
	}

	// Get oapi-codegen version and build executable command
	oapiCodegenVersion := os.Getenv("OAPI_CODEGEN_VERSION")
	if oapiCodegenVersion == "" {
		oapiCodegenVersion = "v2.3.0"
	}

	executable := fmt.Sprintf("go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@%s", oapiCodegenVersion)

	return doGenerate(executable, *config.GenerateOpenAPI)
}

func doGenerate(executable string, config forge.GenerateOpenAPIConfig) error {
	cmdName, args := parseExecutable(executable)
	errChan := make(chan error, 100) // Buffered to avoid goroutine leaks
	wg := &sync.WaitGroup{}

	for i := range config.Specs {
		i := i
		for _, version := range config.Specs[i].Versions {
			version := version

			sourcePath := templateSourcePath(config, i, version)

			// Generate client if enabled
			if config.Specs[i].Client.Enabled {
				wg.Add(1)
				go func() {
					defer wg.Done()
					if err := generatePackage(cmdName, args, config, i, version, config.Specs[i].Client, clientTemplate, sourcePath); err != nil {
						errChan <- err
					}
				}()
			}

			// Generate server if enabled
			if config.Specs[i].Server.Enabled {
				wg.Add(1)
				go func() {
					defer wg.Done()
					if err := generatePackage(cmdName, args, config, i, version, config.Specs[i].Server, serverTemplate, sourcePath); err != nil {
						errChan <- err
					}
				}()
			}
		}
	}

	go func() {
		wg.Wait()
		close(errChan)
	}()

	// Collect all errors
	var errors []string
	for err := range errChan {
		if err != nil {
			errors = append(errors, err.Error())
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("generation failed: %s", strings.Join(errors, "; "))
	}

	fmt.Println("✅ Successfully generated OpenAPI code")
	return nil
}

func generatePackage(cmdName string, baseArgs []string, config forge.GenerateOpenAPIConfig, specIndex int, version string, opts forge.GenOpts, template string, sourcePath string) error {
	outputPath := templateOutputPath(config, specIndex, opts.PackageName)
	templatedConfig := fmt.Sprintf(template, opts.PackageName, outputPath)

	path, cleanup, err := writeTempCodegenConfig(templatedConfig)
	if err != nil {
		return fmt.Errorf("failed to write temp config: %w", err)
	}
	defer cleanup()

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	args := append(baseArgs, "--config", path, sourcePath)
	cmd := exec.Command(cmdName, args...)

	if err := util.RunCmdWithStdPipes(cmd); err != nil {
		return fmt.Errorf("oapi-codegen failed for %s: %w", opts.PackageName, err)
	}

	return nil
}

func parseExecutable(executable string) (string, []string) {
	split := strings.Split(executable, " ")
	return split[0], split[1:]
}

func writeTempCodegenConfig(templatedConfig string) (string, func(), error) {
	tempFile, err := os.CreateTemp("", "oapi-codegen-*.yaml")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temp file: %w", err)
	}

	cleanup := func() {
		_ = os.RemoveAll(tempFile.Name())
	}

	if _, err := tempFile.WriteString(templatedConfig); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := tempFile.Close(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("failed to close temp file: %w", err)
	}

	return tempFile.Name(), cleanup, nil
}

func templateOutputPath(config forge.GenerateOpenAPIConfig, index int, packageName string) string {
	destDir := config.Defaults.DestinationDir
	if config.Specs[index].DestinationDir != "" {
		destDir = config.Specs[index].DestinationDir
	}

	return filepath.Join(destDir, packageName, zzGeneratedFilename)
}

func templateSourcePath(config forge.GenerateOpenAPIConfig, index int, version string) string {
	if source := config.Specs[index].Source; source != "" {
		return source
	}

	sourceFile := fmt.Sprintf(sourceFileTemplate, config.Specs[index].Name, version)

	sourceDir := config.Defaults.SourceDir
	if config.Specs[index].SourceDir != "" {
		sourceDir = config.Specs[index].SourceDir
	}

	return filepath.Join(sourceDir, sourceFile)
}
