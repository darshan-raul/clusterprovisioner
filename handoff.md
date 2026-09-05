# handoff

> Live cross-session state for Strata v2. The last agent to leave
> the repo updates this file before signing off. New agents read
> AGENTS.md first, then this file.

---

## Current phase

## Current phase

**Phase 6 — RAG (per-user).** (queued)

Phase 5 (Web dashboard) is complete. The read-only web dashboard features a cluster manager, per-cluster Kubernetes resource browser (`/dashboard/clusters/[id]`), pod inspection drawer with TUI command snippets, live workload metrics, namespace & status filters, and an end-to-end action audit trail (`/api/v1/history`) capturing mutations across TUI, agent, and web surfaces.

---

## What's done in Phase 5

- [x] **Audit Trail Schema & Store Layer (`backend/services/orchestrator/internal/store`):**
  - Added `action_history` table in `0001_init.sql` with foreign keys referencing `users(id)` and `clusters(id)` with `ON DELETE CASCADE`, indexed on `(user_id, created_at DESC)` and `(cluster_id, created_at DESC)`.
  - Implemented `store.ActionHistory` model and `RecordAction` / `ListHistory` store methods with limit clamping.
  - Comprehensive unit tests in `store_test.go` verifying action insertion, filtering by user/cluster, and cascade deletion.
- [x] **Orchestrator Audit Logging & History Endpoints (`backend/services/orchestrator/internal/api`):**
  - Added `GET /api/v1/history` for user audit trail and `GET /api/v1/clusters/{id}/history` for cluster-specific activity.
  - Wired `recordAction` helper into all mutation and lifecycle handlers (`handleCreateCluster`, `handleDeleteCluster`, `handleDeletePod`, `handleApplyManifest`, `handleExecCommand`) tracking `action_type`, `target`, `status` (`success`/`failed`), `details`, and `client_type` (`tui`, `tui_agent`, `web`).
  - Added fake store history methods and unit tests in `fakes_test.go` and `clusters_test.go`.
- [x] **Next.js API Routes & Data Layer (`web/lib/orchestrator.ts`, `web/app/api`):**
  - Added `Pod` and `ActionHistoryItem` TypeScript interfaces, `fetchCluster`, `fetchPods`, and `fetchHistory`.
  - Created Next.js Route Handlers:
    - `web/app/api/clusters/[id]/pods/route.ts` (query parameters for `namespace`, `label_selector`).
    - `web/app/api/clusters/[id]/history/route.ts` (query parameter for `limit`).
    - `web/app/api/history/route.ts` (query parameters for `cluster_id`, `limit`).
  - Vitest test coverage in `web/tests/clusters.test.ts` (11 tests in suite, 100% passing).
- [x] **Next.js Resource Browser & Audit UI (`web/app/dashboard`):**
  - Built `ClusterDetailClient` and server component at `/dashboard/clusters/[id]`:
    - Summary banner with cluster context, ready badge, registration date, and live refresh button.
    - Read-only safety invariant notice emphasizing TUI as sole mutating surface.
    - Quick metrics bar (Total, Running, Pending, Failed/Issues).
    - Instant search filter, namespace filter, and status filter dropdown.
    - Pods table with color-coded status badges and container readiness.
    - Pod inspection drawer modal with metadata and copyable TUI command examples.
    - Tabbed view switching between Pods and Cluster Audit History.
  - Built `HistoryFeed` component in `web/app/dashboard/history-feed.tsx` showing status, action pills, target, client badges (`[TUI]`, `[AGENT]`, `[WEB]`), and timestamps.
  - Enhanced `ClusterManager` with "Browse" links and integrated `HistoryFeed` on the main dashboard (`/dashboard`).
  - Verified production bundle (`next build`) and linting (`next lint`).

---

## What's next (Phase 6)

See [AGENTS.md §5](AGENTS.md#5-build-phases).

- **Qdrant Vector Database:** Qdrant in backend cluster with per-user collections (`user_{user_id}`).
- **RAG Indexer (`backend/services/rag-indexer/`):** Go service ingesting cluster state (pods, events, nodes) periodically into vector collections.
- **Retriever Service (`backend/services/retriever/`):** Go service exposing `/retrieve` endpoint queried by the agent.
- **LangGraph Retrieval Node:** Agent service incorporates `retrieve` node with conditional routing.

---

## Open questions / decisions

1. **Envoy Gateway + Auth:** Envoy Gateway ext-authz vs Envoy OIDC filter for routing. (Using Envoy Gateway HTTPRoutes with Keycloak OIDC PKCE at web layer + JWKS validation at orchestrator).
2. **KMS-wrapped DEK:** Single-key AES-GCM for kind dev; AWS KMS envelope encryption for Phase 8.

---

## Session log

### Session 4 — Phase 3 Mutation Tools, Confirmation Modal, and LangGraph Interrupts

- Implemented FastMCP mutation tools (`delete_pod`, `apply_manifest`, `exec_command`) in `backend/mcp-servers/k8s/strata_mcp_k8s/tools/`.
- Exposed orchestrator mutation endpoints (`DELETE /pods/{name}`, `POST /apply`, `POST /pods/{name}/exec`) with tenant isolation in Go orchestrator.
- Built Textual `ConfirmScreen` modal and added `:delete`, `:apply`, `:exec` commands to TUI command palette.
- Integrated LangGraph Human-in-the-Loop `interrupt()` breakpoint in `strata_tui/agent.py` with `Command(resume=...)` support.
- Updated `scripts/e2e.sh` and verified all tiers with 100% test coverage. Phase 3 closed.

### Session 5 — Phase 4 Encrypted Cluster Registry

- Implemented AES-256-GCM encryption/decryption in shared Go package (`backend/services/shared/pkg/crypto`).
- Added encrypted kubeconfig columns to CloudNativePG PostgreSQL schema and store layer.
- Added orchestrator `POST /api/v1/clusters` and `DELETE /api/v1/clusters/{id}` endpoints with credential encryption and MCP argument propagation.
- Implemented in-memory AES-256-GCM decryption in FastMCP k8s server using `cryptography` and `load_kube_config_from_dict`.
- Built Next.js cluster manager modal with kubeconfig upload/paste, route handlers, and Vitest test coverage.
- Enhanced TUI `:ctx` with active cluster indicator (`*`), `:ctx add`, and `:ctx delete`.
- Verified all tiers (Backend, MCP, Web, TUI) with 100% pass rate. Phase 4 closed.

### Session 6 — Phase 5 Web Dashboard (Resource Browser & Audit Trail)

- Created PostgreSQL `action_history` audit table and store layer methods (`RecordAction`, `ListHistory`) with user & cluster cascading foreign keys and indexes.
- Added orchestrator `GET /api/v1/history` and `GET /api/v1/clusters/{id}/history` REST endpoints with limit validation and cross-tenant filtering.
- Instrumented all cluster lifecycle and mutation endpoints (`create_cluster`, `delete_cluster`, `delete_pod`, `apply_manifest`, `exec_command`) to log audit actions with status and client type (`tui`, `tui_agent`, `web`).
- Added Next.js client methods and route handlers for pods and history (`/api/clusters/[id]/pods`, `/api/clusters/[id]/history`, `/api/history`).
- Built read-only Kubernetes resource browser (`/dashboard/clusters/[id]`) with live pod metrics, search & namespace filters, status badges, and pod inspection drawer modal with copyable TUI commands.
- Built `HistoryFeed` component and integrated into the main dashboard and cluster details view.
- Verified all tiers: Go backend tests + lint, FastMCP k8s tests + lint, Next.js build + vitest + eslint, and Textual TUI tests + lint. Phase 5 closed.
