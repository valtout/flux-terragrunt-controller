#!/usr/bin/env bash
set -euo pipefail

NAMESPACE="${NAMESPACE:-flux-system}"
CONTROLLER_NAMESPACE="${CONTROLLER_NAMESPACE:-flux-terragrunt-controller}"
RELEASE_NAME="${RELEASE_NAME:-flux-terragrunt}"

echo "=== Deploying controllers ==="

# Load the controller image into kind if available
if docker image inspect flux-terragrunt-controller:latest &>/dev/null; then
    echo "Loading controller image into kind..."
    kind load docker-image flux-terragrunt-controller:latest --name "${CLUSTER_NAME:-flux-terragrunt-e2e}"
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
kubectl create namespace "${CONTROLLER_NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -
kubectl create sa flux-terragrunt-controller -n "${CONTROLLER_NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -

# Build and deploy the controller via helm
echo "Deploying controller via Helm..."
helm upgrade --install "${RELEASE_NAME}" charts/flux-terragrunt-controller \
    --namespace "${CONTROLLER_NAMESPACE}" \
    --create-namespace \
    --set controller.image.repository="flux-terragrunt-controller" \
    --set controller.image.tag="latest" \
    --set controller.image.pullPolicy="Never" \
    --set runner.serviceAccount.name="flux-terragrunt-controller" \
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
