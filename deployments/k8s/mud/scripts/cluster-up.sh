#!/usr/bin/env bash
# deployments/k8s/mud/scripts/cluster-up.sh
# Creates the Kind cluster. Run once before first deploy.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../../../.." && pwd)"

echo "Creating Kind cluster 'mud'..."
kind create cluster --name mud --config "${REPO_ROOT}/kind-config.yaml"
echo "Cluster ready."
kubectl cluster-info --context kind-mud
