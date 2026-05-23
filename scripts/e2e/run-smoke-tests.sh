#!/usr/bin/env bash
# Smoke tests to verify the cluster is healthy before running full E2E
set -euo pipefail

echo "=== Running smoke tests ==="

# Test kubectl connectivity
echo "Testing kubectl connectivity..."
kubectl cluster-info
kubectl get nodes

# Test core CRDs
echo "Testing CRD availability..."
kubectl get crd/gitrepositories.source.toolkit.fluxcd.io
kubectl get crd/terragrunt.run_units

# Test namespace creation
echo "Testing namespace operations..."
kubectl create namespace e2e-test-ns --dry-run=client -o yaml | kubectl apply -f -
kubectl get namespace e2e-test-ns
kubectl delete namespace e2e-test-ns

# Test pod creation
echo "Testing pod creation in default namespace..."
kubectl run test-pod --image=busybox --restart=Never --rm -it -- echo "Smoke test passed"
kubectl get pods

# Test service account creation
echo "Testing service account creation..."
kubectl create sa test-sa --dry-run=client -o yaml | kubectl apply -f -
kubectl get sa test-sa

# Test RBAC
echo "Testing RBAC setup..."
kubectl auth can-i get pods --as=system:serviceaccount:default:test-sa 2>/dev/null && echo "RBAC working" || echo "No RBAC configured (OK)"

echo "=== Smoke tests passed ==="
