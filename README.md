# tooling

This repository contains tooling and utilities to simplify development.

## Available tools

| Name                       | Description                                                                                                                                                                                                     |
|----------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `kindenv`                  | It wraps `kind` to create a k8s cluster and output the kubeconfig to a local path specified by the `.project.yaml` file.                                                                                        |
| `local-container-registry` | It creates a container registry in the kind cluster created by `kindenv`. It reads it's configuration from `.project.yaml`.                                                                                     | 
| `oapi-codegen-helper`      | It wraps `oapi-codegen` to conveniently generate server and/or client code from a local or remote OpenAPI Specification. It reads its configuration from `.oapi-codegen.yaml`. Code generation is parallelized. | 

## Project Config

The project config or `.project.yaml` file is a single configuration file that declares intent about the project and is
used by the tools and utilities defined in this project.
