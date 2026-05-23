#!/usr/bin/env bash
set -euo pipefail

CLUSTER_NAME="${CLUSTER_NAME:-flux-terragrunt-e2e}"

echo "=== Tearing down Kind cluster: ${CLUSTER_NAME} ==="

if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
    kind delete cluster --name "${CLUSTER_NAME}"
    echo "Cluster deleted"
else
    echo "Cluster ${CLUSTER_NAME} does not exist"
fi

echo "=== Teardown complete ==="
