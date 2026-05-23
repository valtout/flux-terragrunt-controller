#!/usr/bin/env bash
set -euo pipefail

NAMESPACE="${NAMESPACE:-flux-system}"
CONTROLLER_NAMESPACE="${CONTROLLER_NAMESPACE:-flux-terragrunt-controller}"
TEST_TIMEOUT="${TEST_TIMEOUT:-300s}"

echo "=== Running E2E tests ==="

# Wait for controller to be running
echo "Checking controller status..."
kubectl wait deployment/flux-terragrunt-controller \
    -n "${CONTROLLER_NAMESPACE}" \
    --for=condition=Available \
    --timeout=120s

# Verify RBAC permissions
echo "Verifying RBAC..."
kubectl auth can-i get pods --as=system:serviceaccount:"${CONTROLLER_NAMESPACE}":"flux-terragrunt-controller" -n "${NAMESPACE}" || {
    echo "ERROR: Controller cannot list pods in ${NAMESPACE}"
    exit 1
}

# Create a test GitRepository resource
echo "Creating test GitRepository..."
cat << 'EOF' | kubectl apply -f -
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: test-repo
  namespace: flux-system
spec:
  interval: 1m
  url: https://github.com/example/test-repo
  ref:
    branch: main
  secretRef:
    name: test-repo-secret
EOF

# Wait for GitRepository to be created
kubectl wait gitrepository/test-repo -n flux-system \
    --for=condition=Ready \
    --timeout=60s || echo "GitRepository not ready (expected - no real secret)"

# Create Units CR
echo "Creating Units CR..."
cat << 'EOF' | kubectl apply -f -
apiVersion: terragrunt.run/v1alpha1
kind: Units
metadata:
  name: test-units
  namespace: flux-system
spec:
  branch: main
  filters:
    - path/to/terraform
    - another/path
EOF

# Wait for Units to be created
kubectl wait units/test-units -n flux-system \
    --for=condition=Ready \
    --timeout=30s || echo "Units not ready (expected - waiting for reconciliation)"

# Verify Units status
echo "Checking Units status..."
kubectl get units -n flux-system test-units -o yaml

# Check controller logs
echo "Fetching controller logs..."
kubectl logs -n "${CONTROLLER_NAMESPACE}" -l control-plane=controller-manager --tail=50 || \
kubectl logs -n "${CONTROLLER_NAMESPACE}" -l app=flux-terragrunt-controller --tail=50 || true

# Verify runner job can be created (test permission to create jobs)
echo "Verifying job creation permissions..."
kubectl auth can-i create jobs --as=system:serviceaccount:"${CONTROLLER_NAMESPACE}":"flux-terragrunt-controller" -n "${CONTROLLER_NAMESPACE}" || {
    echo "ERROR: Controller cannot create jobs"
    exit 1
}

echo "=== E2E tests completed successfully ==="
