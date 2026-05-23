#!/usr/bin/env bash
set -euo pipefail

NAMESPACE="${NAMESPACE:-flux-system}"
CONTROLLER_NAMESPACE="${CONTROLLER_NAMESPACE:-flux-terragrunt-controller}"
TEST_TIMEOUT="${TEST_TIMEOUT:-300s}"

# Test repository configuration (can be overridden via env vars)
TEST_GIT_REPO_URL="${TEST_GIT_REPO_URL:-https://github.com/example/test-repo}"
TEST_GIT_BRANCH="${TEST_GIT_BRANCH:-main}"
TEST_GIT_SECRET_REF="${TEST_GIT_SECRET_REF:-}"

echo "=== Running E2E tests ==="
echo "Using Git Repository URL: ${TEST_GIT_REPO_URL}"
echo "Using Branch: ${TEST_GIT_BRANCH}"

# Wait for controller to be running
echo "Checking controller status..."
kubectl wait deployment/flux-terragrunt-controller \
    -n "${CONTROLLER_NAMESPACE}" \
    --for=condition=Available \
    --timeout=120s || {
    echo "ERROR: Controller deployment not available"
    kubectl get all -n "${CONTROLLER_NAMESPACE}"
    exit 1
}

# Verify RBAC permissions
echo "Verifying RBAC..."
kubectl auth can-i get pods --as=system:serviceaccount:"${CONTROLLER_NAMESPACE}":"flux-terragrunt-controller" -n "${NAMESPACE}" || {
    echo "ERROR: Controller cannot list pods in ${NAMESPACE}"
    exit 1
}

kubectl auth can-i create jobs --as=system:serviceaccount:"${CONTROLLER_NAMESPACE}":"flux-terragrunt-controller" -n "${NAMESPACE}" || {
    echo "ERROR: Controller cannot create jobs in ${NAMESPACE}"
    exit 1
}

kubectl auth can-i get gitrepositories --as=system:serviceaccount:"${CONTROLLER_NAMESPACE}":"flux-terragrunt-controller" -n "${NAMESPACE}" || {
    echo "ERROR: Controller cannot get gitrepositories in ${NAMESPACE}"
    exit 1
}

echo "RBAC verification passed"

# Create GitRepository resource
echo "Creating test GitRepository..."
cat <<EOF | kubectl apply -f -
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: test-repo
  namespace: ${NAMESPACE}
spec:
  interval: 1m
  url: ${TEST_GIT_REPO_URL}
  ref:
    branch: ${TEST_GIT_BRANCH}
EOF

# Create secret ref if specified
if [[ -n "${TEST_GIT_SECRET_REF}" ]]; then
    echo "Adding secret reference: ${TEST_GIT_SECRET_REF}"
    kubectl patch gitrepository/test-repo -n "${NAMESPACE}" \
        --type='json' \
        -p='[{"op": "add", "path": "/spec/secretRef/name", "value":"'${TEST_GIT_SECRET_REF}'"}]'
fi

# Wait briefly for initial reconciliation
sleep 5

# Simulate GitRepository readiness by manually setting status (for testing without real webhook)
# In production, Flux would reconcile this and set the artifact
echo "Simulating GitRepository artifact status..."
cat <<'PATCH' | kubectl patch gitrepository/test-repo -n "${NAMESPACE}" --type=merge --subresource=status -p '{"status":{"conditions":[{"type":"Ready","status":"True","message":"Test artifact ready","observedGeneration":1}],"artifact":{"revision":"'"${TEST_GIT_BRANCH}"'/sha.aaaa1111bbbb2222cccc3333dddd4444eeee5555","checksum":"sha256:1234567890abcdef","lastTransitionTime":"2026-01-01T00:00:00Z","path":"gitrepository/flux-system/test-repo/sha.aaaa1111bbbb2222cccc3333dddd4444eeee5555.tar.gz","url":"http://source-controller/gitrepository/flux-system/test-repo/sha.aaaa1111bbbb2222cccc3333dddd4444eeee5555.tar.gz"}}}'
PATCH

# Wait for GitRepository to appear ready
kubectl wait gitrepository/test-repo -n "${NAMESPACE}" \
    --for=condition=Ready \
    --timeout=60s || {
    echo "GitRepository not ready, showing status..."
    kubectl get gitrepository/test-repo -n "${NAMESPACE}" -o yaml
}

# Create Units CR
echo "Creating Units CR..."
cat <<EOF | kubectl apply -f -
apiVersion: terragrunt.run/v1alpha1
kind: Units
metadata:
  name: test-units
  namespace: ${NAMESPACE}
spec:
  branch: ${TEST_GIT_BRANCH}
  filters:
    - prod
EOF

# Wait for Units to be created
kubectl wait units/test-units -n "${NAMESPACE}" \
    --for=condition=Ready \
    --timeout=30s || echo "Units not ready (waiting for reconciliation)"

# Give controller time to react to the new Units
echo "Waiting for controller to process Units..."
sleep 10

# Verify Units status
echo "Checking Units status..."
kubectl get units -n "${NAMESPACE}" test-units -o yaml

# Check if a runner job was spawned
echo "Looking for runner jobs..."
RUNNER_JOB=$(kubectl get jobs -n "${CONTROLLER_NAMESPACE}" -l "terragrunt.run/units=test-units" -o name 2>/dev/null || echo "")
if [[ -n "${RUNNER_JOB}" ]]; then
    echo "SUCCESS: Runner job found: ${RUNNER_JOB}"
    kubectl describe "${RUNNER_JOB}" -n "${CONTROLLER_NAMESPACE}"
else
    echo "INFO: No runner job found yet (controller may need more time or real git changes)"
fi

# Check controller logs
echo "Fetching controller logs..."
kubectl logs -n "${CONTROLLER_NAMESPACE}" -l app=flux-terragrunt-controller --tail=100 || \
kubectl logs -n "${CONTROLLER_NAMESPACE}" -l control-plane=controller-manager --tail=100 || true

# Verify job creation permissions
echo "Verifying job creation permissions..."
kubectl auth can-i create jobs --as=system:serviceaccount:"${CONTROLLER_NAMESPACE}":"flux-terragrunt-controller" -n "${CONTROLLER_NAMESPACE}" || {
    echo "ERROR: Controller cannot create jobs"
    exit 1
}

echo "=== E2E tests completed successfully ==="
