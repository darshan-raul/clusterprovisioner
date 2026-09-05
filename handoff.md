# handoff

> Live cross-session state for Strata v2. The last agent to leave
> the repo updates this file before signing off. New agents read
> AGENTS.md first, then this file.

---

## Current phase

**Phase 7 — More MCP servers.** (queued)

Phase 6 (RAG per-user) is complete. The multi-tenant RAG architecture features:
1. `retriever` Go service with strict per-user collection isolation (`user_{user_id}_{collection}`), Qdrant REST integration, in-memory cosine fallback, and OpenAI/LiteLLM embedding integration.
2. `rag-indexer` Go service with markdown runbook chunking and cluster workload/history state synchronization (daemon and `--once` modes).
3. Orchestrator `POST /api/v1/retrieve` proxy injecting tenant identity.
4. TUI `StrataClient.retrieve`, LangGraph `retrieve_docs` tool, and `should_retrieve` routing logic.

---

## What's done in Phase 6

- [x] **Authoritative RAG Architecture Reference (`docs/rag.md`):**
  - Full design covering multi-tenant per-user collections, request flows, API contracts, Qdrant/Memory store abstractions, ingestion, and LangGraph integration.
- [x] **Retriever Service (`backend/services/retriever/`):**
  - `internal/embedder`: `Embedder` interface with `OpenAIEmbedder` (LiteLLM/OpenAI-compatible) and deterministic, unit-normalized `MockEmbedder`.
  - `internal/vectorstore`: `Store` interface with `QdrantStore` (HTTP REST `/collections`, `/points/search`, `/points`) and thread-safe `MemoryStore` with cosine similarity and metadata filtering.
  - `internal/api`: Chi router with `GET /healthz`, `POST /retrieve`, `POST /index`, and `DELETE /index/{collection}/*`.
  - Enforced multi-tenant isolation via `UserScopedCollection(userID, collection)` (`user_{user_id}_{collection}`) and `user_id` payload attributes.
  - Unit tests in `embedder_test.go`, `vectorstore_test.go`, and `server_test.go` (100% passing).
- [x] **RAG Indexer Service (`backend/services/rag-indexer/`):**
  - `internal/indexer/docs.go`: recursive markdown traversal and header-based chunking (`# `, `## `, `### `) with content hashing.
  - `internal/indexer/indexer.go`: cluster summary, pod workload, and audit history chunking and ingestion into retriever.
  - `cmd/rag-indexer`: supporting both continuous ticker daemon and single-shot execution (`--once`).
  - Unit tests in `indexer_test.go` (100% passing).
- [x] **Orchestrator Retrieval Gateway (`backend/services/orchestrator/`):**
  - Added `RetrieverURL` to configuration with environment fallback.
  - Added `POST /api/v1/retrieve` endpoint with JWT authentication and `X-Strata-User` header injection.
  - Added unit test in `clusters_test.go` verifying proxying to mock retriever service.
- [x] **Textual TUI & LangGraph Agent Integration (`tui/strata_tui/`):**
  - Added `retrieve` method to `StrataClient` in `tui/strata_tui/api/client.py`.
  - Added `retrieve_docs` tool in `tui/strata_tui/agent.py` supporting `clusters` and `docs` collections.
  - Added `should_retrieve` heuristic function detecting diagnostic and informational intent.
  - Updated `SYSTEM_PROMPT` to guide agent on retrieval usage.
  - Unit tests in `test_api.py` and `test_agent.py` (45 tests in suite, 100% passing).
- [x] **Workflow & CI (`Makefile`):**
  - Updated `backend-test` and `backend-lint` to test and vet all 4 Go services (`shared`, `orchestrator`, `retriever`, `rag-indexer`).

---

## What's next (Phase 7)

See [AGENTS.md §5](AGENTS.md#5-build-phases).

- **Shared MCP Library (`backend/mcp-servers/shared/strata_mcp_common`):** Shared cryptography, credential handling, and error envelope formatting.
- **ArgoCD FastMCP Server (`backend/mcp-servers/argocd/`):** Streamable-HTTP MCP server exposing tools: `list_apps`, `get_app`, `sync_app` [MUTATION], `get_app_logs`.
- **AWS FastMCP Server (`backend/mcp-servers/aws/`):** Read-only tools for inspecting EKS clusters, node groups, and CloudWatch alerts.
- **Helm FastMCP Server (`backend/mcp-servers/helm/`):** Tools for listing releases, inspecting values, and validating charts.
- **Orchestrator Routing & Agent Tool Chaining:** Orchestrator proxy routes, `StrataClient` bindings, LangGraph agent tool chaining, and `interrupt()` confirmation for mutations.
- **MCP Documentation:** Comprehensive documentation in `docs/mcp.md` and `docs/strata/mcp-architecture.md`.

---

## Open questions / decisions

1. **Envoy Gateway + Auth:** Envoy Gateway ext-authz vs Envoy OIDC filter for routing. (Using Envoy Gateway HTTPRoutes with Keycloak OIDC PKCE at web layer + JWKS validation at orchestrator).
2. **KMS-wrapped DEK:** Single-key AES-GCM for kind dev; AWS KMS envelope encryption for Phase 8.

---

## Session log

### Session 1 — Phase 0 & Phase 1 Reset, TUI Graduation, and Backend Skeleton

- Nuked legacy v1 artifacts, graduated Textual TUI from sandbox to `tui/strata_tui/`.
- Built Go orchestrator skeleton with Chi router, health checks, and JWKS validator.
- Built FastMCP k8s server (`backend/mcp-servers/k8s/`) over streamable-HTTP with `list_pods` tool.
- Established kind local development cluster, Envoy Gateway routing, and verified TUI `:get pods` flow. Phase 1 closed.

### Session 2 & 3 — Phase 2 OIDC, Next.js Web Dashboard Auth, and TUI Device Flow

- Deployed Keycloak as backend OIDC identity provider with realm, clients, and test users.
- Built Next.js 15 web application with Keycloak OAuth2 Authorization Code + PKCE login/signup flow.
- Added TUI OIDC Device Code flow (`strata login` / `:login`), storing session tokens locally.
- Enforced JWT validation across Go orchestrator endpoints. Phase 2 closed.

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

### Session 7 — Phase 6 Multi-Tenant RAG (Per-User Collections & LangGraph Tool)

- Rewrote `docs/rag.md` as the authoritative multi-tenant RAG architecture and reference guide.
- Built `retriever` Go service (`backend/services/retriever/`) with `OpenAIEmbedder` (LiteLLM/OpenAI-compatible) and deterministic `MockEmbedder`, Qdrant REST integration and in-memory cosine fallback, and Chi API (`/retrieve`, `/index`, `/delete`).
- Enforced strict per-user collection isolation (`user_{user_id}_{collection}`) preventing cross-tenant vector retrieval.
- Built `rag-indexer` Go service (`backend/services/rag-indexer/`) with markdown document chunking by headings and cluster workload/history state synchronization, supporting both ticker daemon and `--once` execution.
- Added `POST /api/v1/retrieve` to Go orchestrator, proxying user requests to `retriever-service` with `X-Strata-User` injection.
- Added `StrataClient.retrieve`, `retrieve_docs` tool, and `should_retrieve` routing helper to Textual TUI LangGraph agent.
- Updated `Makefile` to include all 4 Go services in `backend-test` and `backend-lint`.
- Verified all tiers (Backend, MCP, Web, TUI) with 100% test pass rate and clean linting. Phase 6 closed.
