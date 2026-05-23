# E2E Testing Guide

This document describes how to run end-to-end tests for the flux-terragrunt-controller using Kind (Kubernetes in Docker).

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) installed and running
- [Kind](https://kind.sigs.k8s.io/docs/user/quick-start/) installed, or it will be downloaded automatically
- [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/) installed
- [Helm](https://helm.sh/docs/intro/install/) installed
- A Git repository with terragrunt code (for real e2e testing)

## Quick Start

### 1. Build the Docker images

```bash
# Build the controller image
docker build -t flux-terragrunt-controller:latest -f images/Dockerfile.controller .

# Build the runner image
docker build -t flux-terragrunt-runner:latest -f images/Dockerfile.runner ./images
```

### 2. Run the full E2E test suite

```bash
make e2e
```

This will:
1. Create a Kind cluster
2. Run smoke tests
3. Deploy the controller via Helm
4. Run e2e tests
5. Tear down the cluster

### 3. Individual steps

```bash
make e2e-setup    # Create Kind cluster
make e2e-smoke    # Run smoke tests
make e2e-deploy   # Deploy controller
make e2e-test     # Run e2e tests
make e2e-teardown # Delete cluster
```

## Testing with a Real Git Repository

For full e2e testing with real git changes, you need a Git repository containing terragrunt code.

### Option A: Use this repository as-is (basic testing)

The e2e tests simulate GitRepository status, allowing you to test the controller's reaction to new Units resources without pushing git changes. This verifies:
- RBAC permissions
- Controller deployment
- Units CR creation
- Job spawning logic

### Option B: Use a real Git repository (full workflow testing)

1. **Fork or create a Git repository** with terragrunt code structure:

```bash
# Example structure
your-repo/
├── prod/
│   ├── terragrunt.hcl
│   └── main.tf (or module reference)
└── modules/
    └── example/
        └── main.tf
```

2. **Push the test-fixtures to your repository:**

```bash
git clone https://github.com/your-org/your-repo
cp -r flux-terragrunt-controller/test-fixtures/terragrunt/* your-repo/
git add .
git commit -m "Add terragrunt workloads"
git push origin main
```

3. **Configure the e2e test environment:**

```bash
export TEST_GIT_REPO_URL=https://github.com/your-org/your-repo
export TEST_GIT_BRANCH=main

# For private repositories, create a secret first:
kubectl create secret generic git-auth-secret \
  --from-literal=credentials=YOUR_GIT_TOKEN \
  -n flux-system
export TEST_GIT_SECRET_REF=git-auth-secret
```

4. **Run the tests:**

```bash
make e2e
```

5. **Trigger a real change** to see the full workflow:

```bash
# Make a change to your terragrunt code
git commit -m "Update prod workload" --allow-empty
git push origin main

# Wait for Flux to detect the change and update the GitRepository status
# The controller will detect the artifact revision change and spawn a runner job
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `CLUSTER_NAME` | Name of the Kind cluster | `flux-terragrunt-e2e` |
| `K8S_VERSION` | Kubernetes version for Kind | `1.36.0` |
| `NAMESPACE` | Namespace for GitRepository and Units | `flux-system` |
| `CONTROLLER_NAMESPACE` | Namespace for the controller | `flux-terragrunt-controller` |
| `TEST_GIT_REPO_URL` | Git repository URL | `https://github.com/example/test-repo` |
| `TEST_GIT_BRANCH` | Git branch to monitor | `main` |
| `TEST_GIT_SECRET_REF` | Secret name for git credentials | (empty) |
| `RUNNER_IMAGE` | Image for terragrunt runner jobs | `flux-terragrunt-runner:latest` |

## What the E2E Tests Verify

1. **Cluster Setup**
   - Kind cluster creation
   - Kubernetes version compatibility

2. **CRD Installation**
   - Flux GitRepository CRD
   - Terragrunt Units CRD

3. **RBAC Permissions**
   - Controller can list/watch pods
   - Controller can create/delete jobs
   - Controller can get/list/watch gitrepositories

4. **Controller Deployment**
   - Controller pod starts successfully
   - Controller responds to metrics endpoint
   - Controller reconciles Units resources

5. **End-to-End Workflow** (with real git repo)
   - GitRepository status updates when commits are pushed
   - Controller detects changes in filtered paths
   - Controller spawns runner jobs with correct parameters
   - Runner job executes terragrunt commands

## Troubleshooting

### Controller not starting

Check pod logs:
```bash
kubectl logs -n flux-terragrunt-controller -l app=flux-terragrunt-controller
```

### Jobs not spawning

1. Verify the GitRepository has an artifact:
```bash
kubectl get gitrepository/test-repo -n flux-system -o yaml
```

2. Check controller logs for "No ready GitRepository" events:
```bash
kubectl logs -n flux-terragrunt-controller -l app=flux-terragrunt-controller | grep -i gitrepository
```

### Runner job failing

Describe the failed job:
```bash
kubectl describe job/tg-runner-test-units-xxxxxxxx -n flux-terragrunt-controller
```

View job logs:
```bash
kubectl logs job/tg-runner-test-units-xxxxxxxx -n flux-terragrunt-controller
```

Common issues:
- Wrong `RUNNER_IMAGE`: Ensure the image has terragrunt installed
- Network issues: Runner needs to pull from the GitRepository artifact URL
- Authentication: If using a private repo, ensure `TEST_GIT_SECRET_REF` is set

### Image pull issues

Since Kind loads images directly, ensure you're using `pullPolicy: Never`:
```bash
helm upgrade --install flux-terragrunt charts/flux-terragrunt-controller \
  --set controller.image.pullPolicy=Never \
  --set runner.image.pullPolicy=Never
```

## Manual Testing Workflow

For iterative testing without rebuilding images:

```bash
# 1. Setup cluster
make e2e-setup

# 2. Deploy controller
make e2e-deploy

# 3. Run initial e2e tests
make e2e-test

# 4. Make changes to your git repo
# git commit & push

# 5. Watch controller react
kubectl logs -n flux-terragrunt-controller -f -l app=flux-terragrunt-controller

# 6. Check for new runner jobs
kubectl get jobs -n flux-terragrunt-controller -w

# 7. Clean up when done
make e2e-teardown
```

## Developing Test Cases

To add new e2e test scenarios, edit `scripts/e2e/run-e2e-tests.sh`. Common test patterns:

```bash
# Wait for a condition
kubectl wait deployment/flux-terragrunt-controller -n flux-terragrunt-controller \
    --for=condition=Available --timeout=120s

# Verify RBAC
kubectl auth can-i create jobs --as=system:serviceaccount:flux-terragrunt-controller:flux-terragrunt-controller

# Patch resource status for testing
kubectl patch gitrepository/test-repo -n flux-system --type=merge --subresource=status -p '...'
```
