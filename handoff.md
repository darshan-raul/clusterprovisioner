# handoff

> Live cross-session state for Strata v2. The last agent to leave
> the repo updates this file before signing off. New agents read
> AGENTS.md first, then this file.

---

## Current phase

**Phase 4 — Encrypted cluster registry.** (queued)

Phase 3 (Mutation tools + confirmation modal + LangGraph human-in-the-loop interrupts) is complete. Safe mutation workflows are enforced across the FastMCP server, Go orchestrator, Textual TUI command palette, and LangGraph agent chat rail.

---

## What's done in Phase 3

- [x] **FastMCP Kubernetes Mutation Tools (`backend/mcp-servers/k8s`):**
  - Added `delete_pod`: Deletes namespaced pods with optional grace period.
  - Added `apply_manifest`: Multi-document YAML/JSON parser with create and server-side patch fallback on 409 conflict.
  - Added `exec_command`: Executes commands in container via `kubernetes.stream`.
  - Annotated all mutating tools with `tags={"mutation", "k8s", ...}` and `[MUTATION]` descriptions.
  - Added `kube.py` shared kubeconfig loading utility.
  - Comprehensive unit test suite in `tests/test_delete_pod.py`, `tests/test_apply_manifest.py`, and `tests/test_exec_command.py` (16 tests, 100% pass).
- [x] **Orchestrator Mutation Endpoints (`backend/services/orchestrator`):**
  - Added `DELETE /api/v1/clusters/{id}/pods/{name}` (proxies to FastMCP `delete_pod`).
  - Added `POST /api/v1/clusters/{id}/apply` (proxies to FastMCP `apply_manifest`).
  - Added `POST /api/v1/clusters/{id}/pods/{name}/exec` (proxies to FastMCP `exec_command`).
  - Verified JWT validation, tenant isolation, and parameter passing.
  - Added unit test cases for all mutation endpoints in `internal/api/clusters_test.go`.
- [x] **Textual TUI Confirmation Modal & Commands (`tui/strata_tui`):**
  - Created `ConfirmScreen`: Modal screen prompting user to allow (`y`) or deny (`n`/`Esc`) mutating operations with target, cluster, and warning details.
  - Added `:delete pod <name> [-n ns] [--grace-period N]` command gated by confirmation modal.
  - Added `:apply -f <file.yaml> [-n ns]` command gated by confirmation modal.
  - Added `:exec <pod> [-c c] [-n ns] -- <cmd>` command gated by confirmation modal.
  - Added `delete_pod`, `apply_manifest`, and `exec_command` to `StrataClient` with error envelope unwrapping.
- [x] **LangGraph Human-in-the-Loop Interrupts (`tui/strata_tui/agent.py`):**
  - Built LangGraph `StateGraph` co-pilot with tools bound to active cluster client.
  - Injected `interrupt()` before executing mutating tools (`delete_pod`, `apply_manifest`, `exec_command`).
  - TUI catches interrupt, displays `ConfirmScreen`, and resumes execution via `Command(resume=True)` or cancels with `Command(resume=False)`.
  - Added comprehensive agent tests in `tui/tests/test_agent.py` and command tests in `tui/tests/test_commands.py` (37 tests, 100% pass).
- [x] **E2E & Platform Validation:**
  - Updated `scripts/e2e.sh` to test `DELETE`, `apply`, and `exec` endpoints against the cluster.
  - All test suites passing: TUI (37 tests), FastMCP (16 tests), Orchestrator & Shared Go tests, Web Vitest (5 tests).

---

## What's next (Phase 4)

See [AGENTS.md §5](AGENTS.md#5-build-phases).

- **Web Dashboard "Add Cluster" Flow:** UI form for uploading/registering new Kubernetes cluster credentials.
- **Encrypted Storage (AES-GCM):** Backend stores kubeconfigs encrypted at rest with KMS-wrapped DEK (single-key AES-GCM for kind dev; AWS KMS envelope encryption for Phase 8).
- **Decryption in MCP Server:** FastMCP server decrypts credentials per authenticated request.
- **TUI Context Switching:** Enhanced `:ctx list` and `:ctx use <id-or-name>` multi-cluster management.

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
