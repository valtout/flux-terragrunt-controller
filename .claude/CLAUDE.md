# Claude Code Instructions

This is a Kubernetes controller for Flux CD that enables utilization of Terragrunt for GitOps.

## Project Overview

- **Type**: Kubernetes controller (operator pattern) built with controller-runtime
- **Language**: Go 1.26
- **Purpose**: Monitors Flux GitRepository resources and spawns Terragrunt jobs when changes are detected in filtered paths
- **Module**: `flux-terragrunt-controller`

## Project Structure

```
├── cmd/flux-terragrunt-controller/main.go    # Controller entry point
├── pkg/
│   ├── apis/terragrunt/v1alpha1/             # CRD definitions (Units CR)
│   ├── apis/flux/v1/                         # Flux GitRepository types
│   ├── controller/                           # Main reconciliation logic
│   └── internal/
│       ├── git/                             # Git operations (clone, diff)
│       └── runner/                          # Job spawning for Terragrunt
├── charts/flux-terragrunt-controller/       # Helm chart for deployment
├── images/
│   ├── Dockerfile.controller                 # Controller image (distroless)
│   └── Dockerfile.runner                     # Runner image (Terragrunt)
├── config/crd/bases/                        # CRD manifests
└── Makefile                                 # test target only
```

## Architecture

### Core Components

1. **UnitsReconciler** (`pkg/controller/controller.go`)
   - watches the `terragrunt.run_units` CRD
   - monitors Flux `GitRepository` resources for artifact revisions
   - compares commit SHAs across configured filter paths
   - spawns Kubernetes Jobs to run Terragrunt commands
   - cleans up old runner jobs (keeps last 10)

2. **Git Client** (`pkg/internal/git/client.go`)
   - clones repositories at specific commits
   - detects changed files between commits
   - supports filter-based path matching

3. **Runner** (`pkg/internal/runner/runner.go`)
   - spawns Kubernetes Jobs for Terragrunt execution
   - uses container image from `RUNNER_IMAGE` env var (defaults to `ghcr.io/${GITHUB_REPOSITORY}/runner:latest`)

### RBAC Requirements

```go
//+kubebuilder:rbac:groups=terragrunt.run,resources=units,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=terragrunt.run,resources=units/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=source.toolkit.fluxcd.io,resources=gitrepositories,verbs=get;list;watch
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;delete
//+kubebuilder:rbac:groups="",resources=pods;services,verbs=get;list;watch
```

## Development Guidelines

### Code Style

- Follow existing Go conventions in the codebase
- Pre-commit hooks enforce:
  - `go-fmt` (golang formatting)
  - `go-mod-tidy` (dependency management)
  - `tofu-fmt`, `tofu-validate`, `tflint` (Terraform/OpenTofu linting)
  - `terragrunt-hcl-fmt` (HCL formatting)
  - `yamllint` (YAML validation)

### Testing

```bash
go test ./...
```

### Dependencies

- `sigs.k8s.io/controller-runtime v0.24.0`
- `k8s.io/client-go v0.36.0`
- `github.com/fluxcd/pkg/apis/meta v1.25.1`

### Building

The controller builds as a static binary with CGO disabled:

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" ./cmd/flux-terragrunt-controller
```

### Running the Controller

Required environment variables:
- `RUNNER_IMAGE` - Docker image for Terragrunt runner jobs (optional, defaults to `ghcr.io/${GITHUB_REPOSITORY}/runner:latest`)
- `RUNNER_SERVICE_ACCOUNT` - ServiceAccount for runner pods (optional, defaults to `flux-terragrunt-controller`)

The controller listens on port 8080 for metrics by default.

## Helm Chart

Located at `charts/flux-terragrunt-controller/`:
- Deploys the controller with RBAC
- Configurable replica count, image, resources, node selectors, tolerations, and affinity
- Metrics port configurable (disabled by default)

## Docker Images

1. **Controller** (`images/Dockerfile.controller`): distroless/static-debian12 base
2. **Runner** (`images/Dockerfile.runner`): Should contain Terragrunt for executing Terraform/Terragrunt commands

## CRD Reference

The `Units` CRD (`terragrunt.run_units`) spec includes:
- `branch` - Git branch to monitor
- `filters` - Paths to filter for changes

Status fields:
- `lastCommitSHA` - Last processed commit
- `lastRunnerJob` - Name of the most recent runner job
- `lastHandledReconcileAt` - Timestamp of last reconciliation

## Key Files for Reference

- CRD types: `pkg/apis/terragrunt/v1alpha1/units_types.go`
- Controller logic: `pkg/controller/controller.go`
- Git operations: `pkg/internal/git/client.go`
- Job runner: `pkg/internal/runner/runner.go`
- Helm deployment: `charts/flux-terragrunt-controller/templates/deployment.yaml`
