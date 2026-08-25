# handoff

> Live cross-session state for Strata v2. The last agent to leave
> the repo updates this file before signing off. New agents read
> AGENTS.md first, then this file.

---

## Current phase

**Phase 2 — OIDC + signup/login.** (queued)

Phase 1 is complete. The backend skeleton, Go orchestrator, Keycloak OIDC, FastMCP k8s server, TUI client with `:get pods`, umbrella Helm chart, and automated kind e2e validation are fully built and tested.

---

## What's done in Phase 1

- [x] **Go Orchestrator & Shared Modules:**
  - `backend/services/shared/pkg/jwks`: thread-safe JWKS token validation with issuer/audience/azp verification, lazy retrieval, and JWKS caching.
  - `backend/services/shared/pkg/mcp`: streamable-HTTP FastMCP JSON-RPC client with SSE and NDJSON stream decoding, session lifecycle management, and header forwarding (`X-Strata-Kubeconfig`, `X-Strata-User`).
  - `backend/services/orchestrator`: REST API (`/healthz`, `/api/v1/me`, `/api/v1/clusters/`, `/api/v1/clusters/{id}/pods`), Postgres persistence via `sqlx` + `pgx`/`lib/pq` with auto-migration (`0001_init.sql`), and dev-mode `BOOTSTRAP_ADMIN_TOKEN` support.
- [x] **FastMCP Kubernetes Server:**
  - `backend/mcp-servers/k8s`: FastMCP 3.x server running over streamable-HTTP transport (`/mcp`).
  - `list_pods` tool supporting multi-cluster kubeconfig loading from headers, namespace filtering, label selectors, and ISO8601 formatting.
- [x] **TUI Client & Commands:**
  - `tui/strata_tui/api/client.py`: Async `StrataClient` with endpoints for `/healthz`, `/me`, `list_clusters`, `list_pods`.
  - `tui/strata_tui/commands/get.py`: `:get pods` command with namespace support, rendering tabular output into `ResourceTable`.
  - `tui/strata_tui/screens/login.py`: OIDC device-code flow modal.
- [x] **Helm Umbrella Chart & Infra:**
  - `backend/helm/strata`: Deployments, Services, ConfigMaps, and StatefulSets for Keycloak (26.0), Postgres 16, Orchestrator, FastMCP k8s server, Envoy Gateway HTTPRoutes.
  - `22-seed-clusters.yaml`: Self-contained post-install hook that resolves Keycloak user ID dynamically and seeds initial cluster and credentials.
  - `infra/kind/strata-dev.yaml`: Multi-port kind cluster configuration.
- [x] **Automated E2E & Test Suite:**
  - `scripts/e2e.sh` (`make e2e`): End-to-end kind validation testing Keycloak OIDC authentication, `/api/v1/me`, cluster listing, MCP `list_pods` proxying, and rolling updates.
  - 100% test pass rate across TUI pytest (21 tests), backend Go tests, and FastMCP pytest (4 tests).
  - Clean linters: `ruff` and `gofmt` / `go vet`.

---

## What's next (Phase 2)

See [AGENTS.md §5](AGENTS.md#5-build-phases).

- **Next.js Web Application:** Setup `web/` with Next.js 15 App Router, TypeScript, Tailwind CSS.
- **Keycloak Auth Integration:** Standard Auth Code flow with PKCE for web dashboard login and user registration.
- **Device-Code Flow Surface:** Web landing page for TUI device-code confirmation (`/device`).
- **TUI Login Polish:** Verify `strata login` against live Keycloak web device flow.

---

## Open questions / decisions

1. **Envoy Gateway + Auth:** Envoy Gateway ext-authz vs Envoy OIDC filter for routing. Decision to be finalized in Phase 2 web setup.
2. **KMS-wrapped DEK:** Single-key AES-GCM for kind dev; AWS KMS envelope encryption for Phase 8.

---

## Session log

### Session 2 — Phase 1 implementation & Kind E2E validation

- Implemented Go orchestrator, Keycloak JWKS validator, and FastMCP client in `backend/services/`.
- Implemented FastMCP k8s server in `backend/mcp-servers/k8s/`.
- Updated TUI with `StrataClient` and `:get pods` command in `tui/strata_tui/`.
- Created Helm umbrella chart in `backend/helm/strata/`.
- Validated complete flow end-to-end with `make e2e` against local `kind`.
- All linters and tests passing. Phase 1 closed.
