#!/usr/bin/env bash
# deployments/k8s/mud/scripts/cluster-down.sh
# Destroys the Kind cluster and all its data.
set -euo pipefail

echo "Deleting Kind cluster 'mud'..."
kind delete cluster --name mud
echo "Cluster deleted."
