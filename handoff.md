# handoff

> Live cross-session state for Strata v2. The last agent to leave
> the repo updates this file before signing off. New agents read
> AGENTS.md first, then this file.

---

## Current phase

**Phase 5 — Web dashboard.** (queued)

Phase 4 (Encrypted cluster registry) is complete. End-to-end encrypted cluster registration, AES-256-GCM storage in Postgres via CloudNativePG schema, in-memory decryption in the FastMCP Kubernetes server without writing credentials to disk, Next.js dashboard cluster management with modal form & file upload, and TUI `:ctx` active cluster highlighting, registration, and switching are fully implemented and verified.

---

## What's done in Phase 4

- [x] **Shared AES-256-GCM Crypto Module (`backend/services/shared/pkg/crypto`):**
  - SHA-256 key derivation (`DeriveKey`), authenticated encryption with 12-byte random nonce (`Encrypt`), and authenticated decryption with integrity tag check (`Decrypt`).
  - Cross-compatible byte format verified between Go and Python `cryptography.hazmat.primitives.ciphers.aead.AESGCM`.
  - Comprehensive unit tests in `crypto_test.go` including roundtrip, wrong key rejection, tampered ciphertext rejection, and cross-language ciphertext decryption.
- [x] **PostgreSQL Schema & Store Layer (`backend/services/orchestrator/internal/store`):**
  - Updated `0001_init.sql` schema: `cluster_creds` now supports `encrypted_kubeconfig TEXT` and `dek_ciphertext TEXT`.
  - Updated `store.Cluster` struct with `EncryptedKubeconfig` and `DEKCiphertext` marked `json:"-"` so ciphertext is never leaked to API responses.
  - Added `store.ClusterCreds` and updated `CreateCluster` to store encrypted credentials.
  - Added `DeleteCluster` with `ON DELETE CASCADE` ensuring associated credentials are deleted cleanly.
  - Unit tests in `store_test.go` (100% passing).
- [x] **Orchestrator REST Endpoints (`backend/services/orchestrator/internal/api`):**
  - Added `POST /api/v1/clusters` (`handleCreateCluster`): validates kubeconfig YAML, extracts context, encrypts credentials via AES-256-GCM, stores cluster, and returns 201 Created.
  - Added `DELETE /api/v1/clusters/{id}` (`handleDeleteCluster`): deletes user cluster with cross-tenant authorization checks.
  - Added `attachClusterCreds` helper to propagate `args["kubeconfig_encrypted"]` and `X-Strata-Encrypted-Kubeconfig` header to MCP tool calls.
  - Added unit test cases in `clusters_test.go` (100% passing).
- [x] **FastMCP In-Memory Decryption (`backend/mcp-servers/k8s`):**
  - Created `strata_mcp_k8s/crypto.py` with AES-256-GCM decryption using `cryptography` AEAD.
  - Updated `strata_mcp_k8s/kube.py` `load_kubeconfig` to accept `kubeconfig_encrypted` and load directly via `kubernetes.config.load_kube_config_from_dict` without disk writes.
  - Updated all tools (`list_pods`, `delete_pod`, `apply_manifest`, `exec_command`) to pass `kubeconfig_encrypted` through.
  - Added unit tests in `tests/test_crypto.py` (22 tests in suite, 100% passing).
- [x] **Next.js Web Dashboard Cluster Manager (`web/`):**
  - Added `createCluster` and `deleteCluster` to `web/lib/orchestrator.ts`.
  - Created API route handlers `/api/clusters` (POST) and `/api/clusters/[id]` (DELETE) with session authentication.
  - Created `ClusterManager` client component in `web/app/dashboard/cluster-manager.tsx` with modal for uploading/pasting kubeconfigs, validating, and deleting.
  - Integrated into `web/app/dashboard/page.tsx` and validated production build (`next build`) and unit tests in `web/tests/clusters.test.ts` (8 vitest tests, 100% passing).
- [x] **Textual TUI Context Management (`tui/strata_tui`):**
  - Updated `StrataClient` with `create_cluster` and `delete_cluster`.
  - Updated `ContextCommand` (`:ctx`) to highlight the active cluster with `*` in `:ctx list`.
  - Added `:ctx add <name> <file> [context]` to register local kubeconfig files with the backend and set active context.
  - Added `:ctx delete <name-or-id>` to remove registered clusters.
  - Added unit tests in `tui/tests/test_api.py` and `tui/tests/test_commands.py` (42 tests in suite, 100% passing).

---

## What's next (Phase 5)

See [AGENTS.md §5](AGENTS.md#5-build-phases).

- **Web Dashboard Resource Browser:** Read-only viewer for pods, deployments, services across registered clusters.
- **Action History View:** View of recent TUI / agent commands and mutation audit trails.
- **Cluster Status Indicator:** Real-time health check / connectivity indicator for registered clusters in the dashboard.

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
