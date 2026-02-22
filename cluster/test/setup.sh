#!/usr/bin/env bash
set -aeuo pipefail

echo "Running setup.sh"

# The DeploymentRuntimeConfig (runtimeconfig-provider-kind) was already patched
# by the local-deploy Makefile target to mount the host Docker socket and run as
# root. We just need to wait for everything to be ready before running tests.

echo "Waiting until provider is healthy..."
${KUBECTL} wait provider.pkg --all --for condition=Healthy --timeout 5m

echo "Waiting for all pods to come online..."
${KUBECTL} -n crossplane-system wait --for=condition=Available deployment --all --timeout=5m

echo "Creating a default provider config..."
cat <<EOF | ${KUBECTL} apply -f -
apiVersion: kind.crossplane.io/v1beta1
kind: ProviderConfig
metadata:
  name: default
spec: {}
EOF
