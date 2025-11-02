package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/alexandremahdhaoui/tooling/internal/util"
	"github.com/alexandremahdhaoui/tooling/pkg/project"
)

const (
	OAPICodegenEnvKey = "OAPI_CODEGEN"

	errEnv = "OAPI_CODEGEN env var must be set"

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

// main is the entrypoint for the oapi-codegen-helper tool.
// It reads the project configuration and generates code from OpenAPI specifications.
func main() {
	executable := os.Getenv(OAPICodegenEnvKey)
	if executable == "" {
		_, _ = fmt.Fprintln(os.Stderr, errEnv)
		os.Exit(1)
	}

	config, err := project.ReadConfig()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	if err := do(executable, config.OAPICodegenHelper); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	_, _ = fmt.Fprintln(os.Stdout, "successfully generated code")
	os.Exit(0)
}

func do(executable string, config project.OAPICodegenHelper) error {
	cmdName, args := parseExecutable(executable)
	errChan := make(chan error)
	wg := &sync.WaitGroup{}

	for i := range config.Specs { // for each spec
		i := i
		for _, version := range config.Specs[i].Versions { // for each version
			version := version

			// for each spec and each version in that spec:

			sourcePath := templateSourcePath(config, i, version)

			for _, pkg := range []struct { // for each client OR server pkg
				opts     project.GenOpts
				template string
			}{
				{ // Client
					opts:     config.Specs[i].Client,
					template: clientTemplate,
				},
				{ // Server
					opts:     config.Specs[i].Server,
					template: serverTemplate,
				},
			} {
    wg.Add(1)
				go func() {
					defer wg.Done()
					if !pkg.opts.Enabled {
						return
					}

					outputPath := templateOutputPath(config, i, pkg.opts.PackageName)
					templatedConfig := fmt.Sprintf(pkg.template, pkg.opts.PackageName, outputPath)

					path, cleanup, err := writeTempCodegenConfig(templatedConfig)
					if err != nil {
						errChan <- err // TODO: wrap err
					}

					defer cleanup()

					if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
						errChan <- err // TODO: wrap err
					}

					args := append(args, "--config", path, sourcePath)
					if err := util.RunCmdWithStdPipes(exec.Command(cmdName, args...)); err != nil {
						errChan <- err // TODO: wrap err
					}
				}()
			}

		}
	}

	go func() {
		wg.Wait()
		close(errChan)
	}()

	// if any error occur or the channel is closed, we return the first error early
	if err := <-errChan; err != nil {
		return err // TODO: wrap error
	}

	return nil
}

func parseExecutable(executable string) (string, []string) {
	split := strings.Split(executable, " ")

	return split[0], split[1:]
}

// ---

// writeTempCodegenConfig return the path to the generated config file, a cleanup function and an error.
func writeTempCodegenConfig(templatedConfig string) (string, func(), error) {
	// 1. create tempfile
	tempFile, err := os.CreateTemp("", "oapi-codegen-*.yaml")
	if err != nil {
		return "", nil, err // TODO: wrap err
	}

	// 2. create a cleanup func
	cleanup := func() {
		os.RemoveAll(tempFile.Name())
	}

	// 3. write to file.
	if _, err := tempFile.WriteString(templatedConfig); err != nil {
		cleanup()

		return "", nil, err // TODO: wrap err
	}

	// 4. close file
	if err := tempFile.Close(); err != nil {
		cleanup()

		return "", nil, err // TODO: wrap err
	}

	return tempFile.Name(), cleanup, nil
}

func templateOutputPath(config project.OAPICodegenHelper, index int, packageName string) string {
	destDir := config.Defaults.DestinationDir
	if config.Specs[index].DestinationDir != "" { // it takes precedence over defaults.
		destDir = config.Specs[index].DestinationDir
	}

	return filepath.Join(destDir, packageName, zzGeneratedFilename)
}

func templateSourcePath(config project.OAPICodegenHelper, index int, version string) string {
	if source := config.Specs[index].Source; source != "" {
		return source
	}

	sourceFile := fmt.Sprintf(sourceFileTemplate, config.Specs[index].Name, version)

	return filepath.Join(config.Defaults.SourceDir, sourceFile)
}
