# flux-terragrunt-controller

A controller for Flux CD, which enables utilization of Terragrunt for GitOps.

## Testing

### Unit Tests
Short unit tests using `testing` package.
```bash
make unit-tests
```


### Lint
Static analysis with `go vet` and `golangci-lint`.
```bash
make lint
```

### Integration Tests
Tests requiring Kubernetes environment via `envtest`.
```bash
make integration-tests
```


### Helm Chart Tests
Helm unittest plugin and chart-testing linting.
```bash
make helm-chart-tests
```


### All Tests
Run unit, lint, integration, and helm tests.
```bash
make test-all
```

### E2E Tests (Kind)
Full end-to-end tests with a local Kind cluster. Creates a cluster, deploys the controller, runs tests, and tears down.

```bash
make e2e-full           # Full cycle with teardown
make e2e                # Full cycle without teardown
make e2e-setup          # Create Kind cluster
make e2e-deploy         # Deploy controller
make e2e-test           # Run E2E tests
make e2e-teardown       # Delete cluster
```

## Build

```bash
make build              # Build controller binary
```
