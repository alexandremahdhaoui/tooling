# ci-orchestrator

The `ci-orchestrator` is a tool responsible for scheduling CI jobs. While jobs targetting the DEFAULT
branch usually would be triggered by ArgoCD, PR/MR jobs or other sorts of jobs needs to be triggered 
per demand.

The kubernetes project's CI is interesting because it make use of PR commands and controllers to
schedule jobs, tests, approvals, etc...

The CI scheduler toolchain will aim at providing a framework to design CI in a similar approach.

Below will be listed advantages of this approach:
- It's open source.
- Control and security: end-to-end control over the CI.
- It's vendor agnostic: this solution is independent of Github workflows or Gitlab CI, migrating from
  one to another does not require any refactoring.
- It's declarative.

## Brainstorm

### Why do we want a new solution?

- ACCESSABILITY
  - Allow anyone to run their CI anywhere. 
  - No need for a subscription or anything. 
  - If you own one computer, you can already start running your CI jobs.
  - It must be simple: what matters is to address complex problems with simple solutions.
  - It must be reproducible. What runs in the CI (except special end-to-end tests)
    should be reproducible in your local environment.

- SECURITY
  - You control everything end-to-end.
  - Best practices built-in.

## How do we want to achieve these goals ?

- OPINIONATED
  - One paradigm to rule them all.
  - No need to solve problems in thousands of different ways.
  - Get started easily by following common recipes
  - Mono-repo enabled.
  - No defaults, no side-effects.

- OPEN SOURCE
  - Most of the problems you want to solve, might already have been solved by someone else. Instead
    of reinventing the wheel checkout others contributions. And if you find a new way to solve the
    problem, feel free to share it with others.

### What are the goals we want to achieve?

- ARTIFACTS
  - Build.
  - Making them available where you need them. (push)
  - Ensure all security aspects of these artifacts. (sign, SBOM...)
  - Test them.

- CODE QUALITY
  - All tiers of tests (unit, integration, end-to-end...).
  - Static analysis, linting...
  - Vulnerability scanning.

- OBSERVABILITY
  - Metrics.
  - Dashboards.
  - Reports

- PERFORMANCES & SUSTAINABILITY
  - Caching.
  - Speed: parallelism, etc...
  - Resource efficiency.

- REPRODUCIBILITY
  - What runs in the CI must run locally.

## Exhaustive list of features per category

### Quality

| Feature | Description |
|---------|-------------|
| Static analysis | |
| Vulnerability scanning | |
| Linting | |
| Unit tests | |
| Integration tests | |
| Functional tests | |
| End-to-end tests | |

Approval? -> Once quality gate has been satisfied, the system should report back that the changes pass the tests.

### Artifacts

| Feature | Description |
|---------|-------------|
| Build | |
| Push | |
| Sign | |
| SBOM | |

### Observability

| Feature | Description |
|---------|-------------|
| Metrics | Provide metrics: success/failure of jobs, etc... Must be identified: job id, commit sha, who triggered the job/how the job was triggered, etc... |
| Dashboards | Grafana dashboards for each feature, for each project, etc... |
| Reports | When a job/pipeline was triggered, always report back the results. Endpoint to return the result? Specify a callback or something? |

### Performance & sustainability

| Feature | Description |
|---------|-------------|
|         |             |

### Reproducibility

| Feature | Description |
|---------|-------------|
|         |             |

## Architecture

### Input, output and side-effects

The goal of this section is to identify inputs, outputs and side-effects. When these have
been identified, we can proceed.

### Adapters

We want to adapt to different concrete implementations. Let's identify the interfaces
that will need adapters.

- Review process. (PR, MR, local API call to trigger tests or artifact build, etc...)
- Language (let's start only with Go and C).
- Container engine (Docker, Podman, etc...).
- Git servers?
- Observability stack.
- SBOM system.

### Core services

