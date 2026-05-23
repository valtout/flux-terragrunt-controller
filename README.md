# flux-terragrunt-controller

A controller for Flux CD, which enables utilization of Terragrunt for GitOps.

## Testing

```bash
# Prerequisites
brew install kind helm kubectl docker

# Run unit tests
go test ./...

# Run full E2E test cycle (creates cluster, deploys, tests, tears down)
make e2e-full

# Or run steps individually
make e2e-setup    # Create Kind cluster
make e2e-deploy   # Deploy controller
make e2e-test      # Run E2E tests
make e2e-teardown  # Delete cluster
```
