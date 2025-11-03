package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "build":
		if err := runBuild(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "integration":
		if err := runIntegration(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`forge - A build orchestration tool

Usage:
  forge build [artifact-name]    Build artifacts from forge.yaml
  forge integration <command>    Manage integration environments

Commands:
  build                         Build all artifacts
  integration create [name]     Create integration environment
  integration list              List integration environments
  integration get <id>          Get environment details
  integration delete <id>       Delete integration environment
  help                          Show this help message`)
}
