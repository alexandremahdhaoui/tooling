package main

import (
	"fmt"
	"log"
	"os"

	"github.com/alexandremahdhaoui/forge/internal/version"
)

// Version information (set via ldflags during build)
var (
	Version        = "dev"
	CommitSHA      = "unknown"
	BuildTimestamp = "unknown"
)

// versionInfo holds forge's version information
var versionInfo *version.Info

// configPath is the path to the forge.yaml file
var configPath string

func init() {
	versionInfo = version.New("forge")
	versionInfo.Version = Version
	versionInfo.CommitSHA = CommitSHA
	versionInfo.BuildTimestamp = BuildTimestamp
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// Parse global flags
	args := os.Args[1:]
	args = parseGlobalFlags(args)

	if len(args) < 1 {
		printUsage()
		os.Exit(1)
	}

	command := args[0]
	cmdArgs := args[1:]

	switch command {
	case "--mcp":
		// Run in MCP server mode
		if err := runMCPServer(); err != nil {
			log.Printf("MCP server error: %v", err)
			os.Exit(1)
		}
	case "build":
		if err := runBuild(cmdArgs); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "test":
		if err := runTest(cmdArgs); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "test-all":
		if err := runTestAll(cmdArgs); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "prompt":
		if err := runPrompt(cmdArgs); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "docs":
		if err := runDocs(cmdArgs); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "config":
		if err := runConfig(cmdArgs); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "version", "--version", "-v":
		versionInfo.Print()
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

// parseGlobalFlags parses global flags like --config and returns remaining args
func parseGlobalFlags(args []string) []string {
	result := make([]string, 0, len(args))

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if arg == "--config" {
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "Error: --config requires a path argument\n")
				os.Exit(1)
			}
			configPath = args[i+1]
			i++ // Skip the next argument (the path)
		} else {
			result = append(result, arg)
		}
	}

	return result
}

func printUsage() {
	fmt.Println(`forge - A build orchestration tool

Usage:
  forge [--config <path>] <command> [args...]

Global Flags:
  --config <path>                    Use custom forge.yaml path (default: forge.yaml)

Commands:
  build [artifact-name]              Build all artifacts
  test <stage> <operation>           Manage test environments
  test-all                           Build all artifacts and run all test stages
  prompt <list|get> [name]           Fetch documentation prompts
  docs <list|get> [name]             Fetch project documentation
  config <subcommand>                Configuration management
  version                            Show version information

Build:
  build                              Build all artifacts from forge.yaml
  build <artifact-name>              Build specific artifact

Test:
  test <stage> create                Create test environment for stage
  test <stage> get <id>              Get test environment details
  test <stage> delete <id>           Delete test environment
  test <stage> list                  List test environments for stage
  test <stage> run [test-id]         Run tests for stage

Test All:
  test-all                           Build all artifacts and run all test stages sequentially

Prompts:
  prompt list                        List all available prompts
  prompt get <name>                  Fetch a specific prompt

Docs:
  docs list                          List all available documentation
  docs get <name>                    Fetch a specific document

Config:
  config validate [path]             Validate forge.yaml configuration

Other:
  version                            Show version information
  help                               Show this help message`)
}
