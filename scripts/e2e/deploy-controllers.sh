#!/usr/bin/env bash
set -euo pipefail

NAMESPACE="${NAMESPACE:-flux-system}"
CONTROLLER_NAMESPACE="${CONTROLLER_NAMESPACE:-flux-terragrunt-controller}"
RELEASE_NAME="${RELEASE_NAME:-flux-terragrunt}"
CLUSTER_NAME="${CLUSTER_NAME:-flux-terragrunt-e2e}"

# Runner image configuration
RUNNER_IMAGE="${RUNNER_IMAGE:-flux-terragrunt-runner:latest}"
RUNNER_SERVICE_ACCOUNT="${RUNNER_SERVICE_ACCOUNT:-flux-terragrunt-controller}"

echo "=== Deploying controllers ==="
echo "Cluster: ${CLUSTER_NAME}"
echo "Controller namespace: ${CONTROLLER_NAMESPACE}"
echo "Runner image: ${RUNNER_IMAGE}"

# Load the controller image into kind if available
if docker image inspect flux-terragrunt-controller:latest &>/dev/null; then
    echo "Loading controller image into kind..."
    kind load docker-image flux-terragrunt-controller:latest --name "${CLUSTER_NAME}"
fi

# Load the runner image into kind if available
if docker image inspect "${RUNNER_IMAGE}" &>/dev/null; then
    echo "Loading runner image into kind..."
    kind load docker-image "${RUNNER_IMAGE}" --name "${CLUSTER_NAME}"
fi

# Create namespaces
echo "Creating namespaces..."
kubectl create namespace "${CONTROLLER_NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -
kubectl create namespace "${NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -

# Install Flux CRDs (GitRepository and core types)
echo "Installing Flux CRDs..."
kubectl apply -f https://raw.githubusercontent.com/fluxcd/flux2/main/crds/gitrepositories.source.toolkit.fluxcd.io.yaml || true
kubectl apply -f https://raw.githubusercontent.com/fluxcd/flux2/main/crds/kustomizations.source.toolkit.fluxcd.io.yaml || true

# Wait for CRDs to be established
echo "Waiting for CRDs to be established..."
kubectl wait --for=condition=Established crd/gitrepositories.source.toolkit.fluxcd.io --timeout=60s || true
kubectl wait --for=condition=Established crd/kustomizations.source.toolkit.fluxcd.io --timeout=60s || true

# Apply the Units CRD
echo "Applying Units CRD..."
kubectl apply -f config/crd/bases/terragrunt.run_units.yaml

# Create service account for runner pods
echo "Creating runner service account..."
kubectl create sa "${RUNNER_SERVICE_ACCOUNT}" -n "${CONTROLLER_NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -

# Bind the service account to the proper RBAC roles for job creation
echo "Setting up RBAC for runner service account..."
cat <<EOF | kubectl apply -f -
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: ${RUNNER_SERVICE_ACCOUNT}-job-creator
  namespace: ${CONTROLLER_NAMESPACE}
subjects:
- kind: ServiceAccount
  name: ${RUNNER_SERVICE_ACCOUNT}
  namespace: ${CONTROLLER_NAMESPACE}
roleRef:
  kind: Role
  name: job-creator
  apiGroup: rbac.authorization.k8s.io
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: job-creator
  namespace: ${CONTROLLER_NAMESPACE}
rules:
- apiGroups: ["batch"]
  resources: ["jobs"]
  verbs: ["create", "delete", "get", "list", "watch"]
EOF

# Build and deploy the controller via helm
echo "Deploying controller via Helm..."
helm upgrade --install "${RELEASE_NAME}" charts/flux-terragrunt-controller \
    --namespace "${CONTROLLER_NAMESPACE}" \
    --create-namespace \
    --set controller.image.repository="flux-terragrunt-controller" \
    --set controller.image.tag="latest" \
    --set controller.image.pullPolicy="Never" \
    --set runner.serviceAccount.name="${RUNNER_SERVICE_ACCOUNT}" \
    --set runner.image.repository="$(echo "${RUNNER_IMAGE}" | cut -d':' -f1)" \
    --set runner.image.tag="$(echo "${RUNNER_IMAGE}" | cut -d':' -f2)" \
    --set runner.image.pullPolicy="Never" \
    --wait --debug

# Wait for controller deployment
echo "Waiting for controller deployment..."
kubectl wait --for=condition=available deployment/"${RELEASE_NAME}-controller" \
    --namespace "${CONTROLLER_NAMESPACE}" \
    --timeout=120s

# Show status
echo "=== Deployment complete ==="
kubectl get all -n "${CONTROLLER_NAMESPACE}"
kubectl get crd | grep -E "(gitrepositories|terragrunt)"

echo ""
echo "=== Controller deployed successfully ==="
echo "To run E2E tests with a real git repository, set these environment variables:"
echo "  TEST_GIT_REPO_URL=https://your-git-server/your-org/your-repo"
echo "  TEST_GIT_BRANCH=main"
echo "  TEST_GIT_SECRET_REF=git-auth-secret  # optional, for private repos"
