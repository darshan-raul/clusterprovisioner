#!/usr/bin/env bash
# Phase 1 end-to-end smoke test.
#
# Brings up a kind cluster + the Strata chart, then exercises the
# orchestrator's REST API end-to-end. Designed to run in CI as well
# as locally.
#
# Requires: kind, kubectl, helm, docker, uv, go, python 3.12+,
# Keycloak + Envoy Gateway images reachable.
#
# Phase 1 success = every step exits 0.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
KIND_CLUSTER="${KIND_CLUSTER:-strata-dev}"
NAMESPACE="${STRATA_NAMESPACE:-strata}"
KUBECONFIG="${KUBECONFIG:-$REPO_ROOT/.kind-kubeconfig}"

export KUBECONFIG

step() { echo; echo "==> $*"; }
fail() { echo "FAIL: $*" >&2; exit 1; }

cleanup() {
    step "Tearing down kind cluster"
    kind delete cluster --name "$KIND_CLUSTER" 2>/dev/null || true
}
trap cleanup EXIT

step "Creating kind cluster"
kind create cluster --config "$REPO_ROOT/infra/kind/strata-dev.yaml"
kind get kubeconfig --name "$KIND_CLUSTER" > "$KUBECONFIG"

step "Installing platform deps (Envoy Gateway)"
"$REPO_ROOT/scripts/install-platform-deps.sh"

step "Building orchestrator + MCP k8s + Web images"
docker build -t strata-orchestrator:dev -f "$REPO_ROOT/backend/services/orchestrator/Dockerfile" "$REPO_ROOT"
docker build -t strata-mcp-k8s:dev -f "$REPO_ROOT/backend/mcp-servers/k8s/Dockerfile" "$REPO_ROOT/backend/mcp-servers/k8s"
docker build -t strata-web:dev -f "$REPO_ROOT/web/Dockerfile" "$REPO_ROOT/web"

step "Loading images into kind"
kind load docker-image strata-orchestrator:dev --name "$KIND_CLUSTER"
kind load docker-image strata-mcp-k8s:dev --name "$KIND_CLUSTER"
kind load docker-image strata-web:dev --name "$KIND_CLUSTER"

step "Installing Strata chart"
helm upgrade --install strata "$REPO_ROOT/backend/helm/strata" \
    --namespace "$NAMESPACE" --create-namespace \
    -f "$REPO_ROOT/backend/helm/strata/values-kind.yaml"

step "Waiting for pods to become ready"
kubectl wait --for=condition=ready pod -l app.kubernetes.io/component=keycloak -n "$NAMESPACE" --timeout=300s
kubectl wait --for=condition=ready pod -l app.kubernetes.io/component=orchestrator -n "$NAMESPACE" --timeout=120s
kubectl wait --for=condition=ready pod -l app.kubernetes.io/component=mcp-k8s -n "$NAMESPACE" --timeout=120s
kubectl wait --for=condition=ready pod -l app.kubernetes.io/component=web -n "$NAMESPACE" --timeout=120s
kubectl wait --for=condition=ready pod -l app.kubernetes.io/component=postgres -n "$NAMESPACE" --timeout=120s

step "Port-forwarding orchestrator + Keycloak + Web"
kubectl port-forward svc/orchestrator 8080:8080 -n "$NAMESPACE" >/tmp/orch-portforward.log 2>&1 &
ORCH_PF=$!
kubectl port-forward svc/keycloak 8081:8080 -n "$NAMESPACE" >/tmp/kc-portforward.log 2>&1 &
KC_PF=$!
kubectl port-forward svc/web 3000:3000 -n "$NAMESPACE" >/tmp/web-portforward.log 2>&1 &
WEB_PF=$!
cleanup_pf() {
    kill "$ORCH_PF" "$KC_PF" "$WEB_PF" 2>/dev/null || true
}
trap "cleanup_pf; cleanup" EXIT
# Give port-forward listeners a moment to bind.
sleep 5

step "Testing orchestrator /healthz"
HEALTH=$(curl -sf http://localhost:8080/healthz)
echo "$HEALTH"
echo "$HEALTH" | grep -q '"status":"ok"' || fail "orchestrator /healthz did not return ok"

step "Testing Web landing page /"
WEB_HTML=$(curl -sf http://localhost:3000/)
echo "$WEB_HTML" | head -n 10
echo "$WEB_HTML" | grep -q "STRATA" || fail "web landing page did not contain STRATA"

step "Testing Web /api/auth/login PKCE redirect to Keycloak"
LOGIN_REDIRECT=$(curl -sI http://localhost:3000/api/auth/login)
echo "$LOGIN_REDIRECT" | head -n 5
echo "$LOGIN_REDIRECT" | grep -q "location: http://localhost:8081/realms/strata-dev/protocol/openid-connect/auth" || fail "web login did not redirect to Keycloak auth"

step "Testing Web /dashboard unauthenticated redirect to /login"
DASH_REDIRECT=$(curl -sI http://localhost:3000/dashboard)
echo "$DASH_REDIRECT" | head -n 5
echo "$DASH_REDIRECT" | grep -q "location: /login" || fail "web dashboard did not redirect unauthenticated user to /login"

step "Testing Keycloak /realms/strata-dev is up"
KC_REALM=$(curl -sf http://localhost:8081/realms/strata-dev)
echo "$KC_REALM" | head -c 200
echo
echo "$KC_REALM" | grep -q '"realm":"strata-dev"' || fail "keycloak realm strata-dev not reachable"

# ── Real OIDC flow ──────────────────────────────────────────────────────
# Mint a real Keycloak access token via the Resource Owner Password
# Credentials grant (directAccessGrantsEnabled=true on the strata-tui
# client in the dev realm). The dev/dev user is seeded by realm.json.
# Headless e2e can't open a browser for the device-code grant, so we
# use the password grant to exercise the *same* token-issuance path
# the TUI's device-code flow ends on, then prove the orchestrator's
# JWKS validator accepts the resulting JWT.
step "Exchanging dev/dev credentials for a Keycloak access token"
TOKEN_RESP=$(curl -sf -X POST \
    -H "Content-Type: application/x-www-form-urlencoded" \
    -d "grant_type=password" \
    -d "client_id=strata-tui" \
    -d "username=dev" \
    -d "password=dev" \
    -d "scope=openid" \
    http://localhost:8081/realms/strata-dev/protocol/openid-connect/token)
echo "$TOKEN_RESP" | head -c 300
echo
ACCESS_TOKEN=$(echo "$TOKEN_RESP" | python3 -c 'import json,sys; print(json.load(sys.stdin)["access_token"])')
[ -n "$ACCESS_TOKEN" ] || fail "no access_token in Keycloak response"

step "Decoding JWT to confirm claims (aud=account, azp=strata-tui, preferred_username=dev)"
JWT_PAYLOAD=$(echo "$ACCESS_TOKEN" | cut -d. -f2)
# Add padding so python's base64 decoder is happy.
JWT_PAYLOAD_PAD="$JWT_PAYLOAD$(python3 -c 'import sys; print("=" * (-len(sys.argv[1]) % 4))' "$JWT_PAYLOAD")"
echo "$JWT_PAYLOAD_PAD" | tr '_-' '/+' | base64 -d 2>/dev/null | python3 -m json.tool || true
echo "$JWT_PAYLOAD_PAD" | tr '_-' '/+' | base64 -d 2>/dev/null | python3 -c '
import json, sys
claims = json.load(sys.stdin)
assert claims["preferred_username"] == "dev", claims
assert claims["email"] == "dev@strata.local", claims
assert claims["azp"] == "strata-tui", claims
print("  -> claims ok")
' || fail "JWT claims did not match expected dev user"

step "Testing orchestrator /api/v1/me with real Keycloak JWT"
ME=$(curl -sf -H "Authorization: Bearer $ACCESS_TOKEN" http://localhost:8080/api/v1/me)
echo "$ME"
echo "$ME" | grep -q '"preferred_username":"dev"' || fail "orchestrator /api/v1/me did not return dev user"
echo "$ME" | grep -q '"email":"dev@strata.local"' || fail "orchestrator /api/v1/me did not return dev email"

step "Testing orchestrator /api/v1/clusters/ with real Keycloak JWT"
CLUSTERS=$(curl -sf -H "Authorization: Bearer $ACCESS_TOKEN" http://localhost:8080/api/v1/clusters/)
echo "$CLUSTERS"
echo "$CLUSTERS" | grep -q '"id":"cl-mock-01"' || fail "seeded cluster cl-mock-01 not found for dev user"

step "Testing orchestrator /api/v1/clusters/cl-mock-01/pods with real Keycloak JWT"
PODS=$(curl -sf -H "Authorization: Bearer $ACCESS_TOKEN" http://localhost:8080/api/v1/clusters/cl-mock-01/pods)
echo "$PODS"
# The seeded kubeconfig points to mock-cluster.invalid, so we expect
# a real Kubernetes connectivity error in the MCP response, not a 5xx
# from the orchestrator. The response will be the MCP error envelope.
echo "$PODS" | grep -q '"content"' || fail "MCP pods call did not return an MCP envelope"

# ── Bootstrap admin flow (fast local-iteration shortcut) ──────────────
# The bootstrap admin token is a Phase 1 dev-only shortcut that lets
# developers curl the orchestrator without a running Keycloak. It's
# independent of the OIDC path above; either one alone is sufficient
# to prove the API is wired.
step "Setting bootstrap admin token (dev shortcut)"
helm upgrade strata "$REPO_ROOT/backend/helm/strata" \
    --namespace "$NAMESPACE" \
    --set orchestrator.bootstrapAdminToken=dev-bootstrap-token

kubectl rollout restart deploy/orchestrator -n "$NAMESPACE" >/dev/null
kubectl rollout status deploy/orchestrator -n "$NAMESPACE" --timeout=60s

# Re-bind the port-forward to the new pod.
kill "$ORCH_PF" 2>/dev/null || true
kubectl port-forward svc/orchestrator 8080:8080 -n "$NAMESPACE" >/tmp/orch-portforward.log 2>&1 &
ORCH_PF=$!

# Wait for port-forward and service to respond
for i in $(seq 1 10); do
    ME=$(curl -sf -H "Authorization: Bearer dev-bootstrap-token" http://localhost:8080/api/v1/me 2>/dev/null || true)
    if [ -n "$ME" ]; then
        break
    fi
    sleep 2
done

step "Testing orchestrator /api/v1/me with bootstrap token"
echo "$ME"
echo "$ME" | grep -q '"sub":"bootstrap-admin"' || fail "orchestrator /api/v1/me did not return bootstrap-admin"

step "Tearing down"
echo "All checks passed."
trap - EXIT
cleanup_pf
kind delete cluster --name "$KIND_CLUSTER"