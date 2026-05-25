#!/usr/bin/env bash
set -euo pipefail

CLUSTER_NAME="${CLUSTER_NAME:-flux-terragrunt-e2e}"
KIND_VERSION="${KIND_VERSION:-v0.31.0}"

# Kubernetes version must be passed in as $1 (no default here)
K8S_VERSION="${1:?Kubernetes version argument is required (passed as $1 from Makefile)}"


echo "=== Setting up Kind cluster: ${CLUSTER_NAME} ==="

# Check if kind is installed
if ! command -v kind &> /dev/null; then
    echo "Installing kind..."
    curl -sLo /tmp/kind "https://github.com/kubernetes-sigs/kind/releases/download/${KIND_VERSION}/kind-linux-amd64"
    chmod +x /tmp/kind
    export PATH="/tmp:${PATH}"
fi

# Check if cluster already exists
if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
    echo "Cluster ${CLUSTER_NAME} already exists"
    echo "Deleting existing cluster..."
    kind delete cluster --name "${CLUSTER_NAME}"
fi

# Create kind config with extra ports for webhook testing
cat > /tmp/kind-config.yaml << 'EOF'
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: InitConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-labels: "topology.kubernetes.io/region=us-east1"
containerRuntimeConfig:
  endpointsPos: 2375
name: flux-terragrunt-e2e
EOF

# Create the cluster
echo "Creating Kind cluster with Kubernetes ${K8S_VERSION}..."
kind create cluster \
    --name "${CLUSTER_NAME}" \
    --image "kindest/node:v${K8S_VERSION}" \
    --wait 5m \
    --config /tmp/kind-config.yaml

# Wait for the node to be ready
echo "Waiting for node to be ready..."
kubectl wait --for=condition=Ready nodes/fLux-terragrunt-e2e-control-plane --timeout=5m || \
kubectl wait --for=condition=Ready nodes/"$(kubectl get nodes -o name | cut -d/ -f2)" --timeout=5m

echo "=== Kind cluster setup complete ==="
kubectl get nodes
