# tooling

This repository contains tooling and utilities to simplify development.

## Available tools

| Name                       | Description                                                                                                                                                                                                     |
|----------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `build-binary` | Wrapper script around `go build` to build go binaries. |
| `build-container` | Wrapper script arounce `kaniko` to build container images. |
| `chart-prereq` | Helper to install necessary helm charts in k8s cluster dedicated for tests. |
| `ci-orchestrator` | The `ci-orchestrator` is a tool responsible for orchestrating CI jobs. |
| `e2e` | Script to execute e2e tests. |
| `kindenv`                  | It wraps `kind` to create a k8s cluster and output the kubeconfig to a local path specified by the `.project.yaml` file.                                                                                        |
| `local-container-registry` | It creates a container registry in the kind cluster created by `kindenv`. It reads it's configuration from `.project.yaml`.                                                                                     | 
| `oapi-codegen-helper`      | It wraps `oapi-codegen` to conveniently generate server and/or client code from a local or remote OpenAPI Specification. It reads its configuration from `.oapi-codegen.yaml`. Code generation is parallelized. | 
| `test-go` | Wrapper script around `gotestsum` to execute scoped tests. |

## Project Config

The project config or `.project.yaml` file is a single configuration file that declares intent about the project and is
used by the tools and utilities defined in this project.

## Templates

### Containerfile template

#### Go

```Dockerfile
FROM docker.io/golang:1.23 as downloader

WORKDIR /workdir
COPY ./go.* ./
RUN go mod download

FROM downloader as builder

ARG GO_BUILD_LDFLAGS
ARG NAME=your-cmd
ARG INPUT_CMD="./cmd/${NAME}"
ARG OUTPUT_BIN="/bin/${NAME}"
WORKDIR /workdir
COPY . ./
RUN CG0_ENABLED=0 \
    GOOS=linux \
    go build \
      -ldflags "${GO_BUILD_LDFLAGS}" \
      -o "${OUTPUT_BIN}" \
      "${INPUT_CMD}"

FROM docker.io/alpine:3.20.1

ARG NAME=your-cmd
ARG OUTPUT_BIN="/bin/${NAME}"
COPY --from=builder ${OUTPUT_BIN} ${OUTPUT_BIN}
CMD [ "your-cmd" ]
```

### Makefile template

```Makefile
TODO
```

