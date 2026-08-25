#!/usr/bin/env bash
# Install the platform-level dependencies that the Strata chart
# requires: Envoy Gateway.
#
# Postgres runs in-cluster as a plain StatefulSet (templates/15-postgres.yaml)
# — no external operator needed for Phase 1 dev.
#
# These install the operators and CRDs only — the actual Strata
# deployments (orchestrator, MCP k8s, etc.) come from
# `helm install strata backend/helm/strata`.
#
# Idempotent. Safe to re-run.

set -euo pipefail

ENVOY_GW_VERSION="${ENVOY_GW_VERSION:-v1.0.0}"

echo "==> Installing Envoy Gateway ${ENVOY_GW_VERSION}"
kubectl apply -f "https://github.com/envoyproxy/gateway/releases/download/${ENVOY_GW_VERSION}/install.yaml" 2>&1 | tail -3 || true

echo "==> Waiting for envoy-gateway pod"
for i in {1..30}; do
    if kubectl get pods -n envoy-gateway-system -l app.kubernetes.io/name=gateway-helm 2>/dev/null | grep -q Running; then
        echo "    envoy-gateway running"
        break
    fi
    sleep 2
done

echo "==> Creating GatewayClass 'eg' if missing"
# v1.0.0 of the install.yaml doesn't ship the GatewayClass instance
# (it was added in a later version whose envoyproxies CRD exceeded
# Kubernetes' 256KB annotation limit). We create it explicitly.
if ! kubectl get gatewayclass eg >/dev/null 2>&1; then
    cat <<'EOF' | kubectl apply -f -
apiVersion: gateway.networking.k8s.io/v1
kind: GatewayClass
metadata:
  name: eg
spec:
  controllerName: gateway.envoyproxy.io/gatewayclass-controller
EOF
fi

echo "==> Waiting for GatewayClass 'eg' to be Accepted"
for i in {1..30}; do
    if kubectl get gatewayclass eg -o jsonpath='{.status.conditions[?(@.type=="Accepted")].status}' 2>/dev/null | grep -q True; then
        echo "    GatewayClass accepted"
        break
    fi
    sleep 2
done

echo "==> Done."