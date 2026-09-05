# Strata

> A two-tier system for managing and troubleshooting existing Kubernetes clusters conversationally and via `k9s`-style commands.

Strata combines a hands-on local terminal interface (Python + Textual + LangGraph) with a remote, multi-tenant Kubernetes backend operating over the Model Context Protocol (MCP).

---

## 1. Architecture Overview

Strata separates user interaction from cluster operations through a secure two-tier model:

```
        ┌────────────────────────────────────────────────────────┐
        │                      Local Laptop                      │
        │  ┌──────────────────────────────────────────────────┐  │
        │  │                   Strata TUI                     │  │
        │  │  • Textual command palette (:get, :delete, etc.) │  │
        │  │  • LangGraph AI co-pilot (BYOK: MiniMax / OpenAI)│  │
        │  │  • Human-in-the-loop mutation confirmation       │  │
        │  └─────────────────────────┬────────────────────────┘  │
        └────────────────────────────┼───────────────────────────┘
                                     │ HTTPS / JWT
                                     ▼
┌────────────────────────────────────────────────────────────────────────┐
│                   Backend Remote Cluster (EKS / Kind)                  │
│                                                                        │
│  AWS NLB / Ingress ──▶ Envoy Gateway (Gateway API)                     │
│    │                                                                   │
│    ├─▶ Web Dashboard (Next.js 15)      Signup, login, cluster registry,│
│    │                                   read-only resource browser      │
│    │                                                                   │
│    ├─▶ Keycloak (OIDC Provider)        Device code (TUI) & PKCE (Web)  │
│    │                                                                   │
│    ├─▶ Orchestrator (Go / Chi)         REST gateway, JWT validation,   │
│    │    │                              AES-256-GCM credential vault,   │
│    │    │                              central audit logging           │
│    │    │                                                              │
│    │    ├─▶ PostgreSQL (CloudNativePG) Users, clusters, encrypted      │
│    │    │                              credentials, action history     │
│    │    │                                                              │
│    │    ├─▶ Retriever Service (Go)     Tenant-isolated vector retrieval│
│    │    │    └─▶ Qdrant Vector Store   Collections: user_{id}_{col}    │
│    │    │                                                              │
│    │    ├─▶ RAG Indexer (Go)           Workloads, history & runbooks   │
│    │    │                                                              │
│    │    └─▶ FastMCP Servers (Python / streamable-HTTP)                 │
│    │         ├─ mcp-k8s                Pods, manifests, exec           │
│    │         ├─ mcp-argocd             Apps, sync, status              │
│    │         ├─ mcp-aws                EKS clusters, nodegroups        │
│    │         └─ mcp-helm               Releases, values, revisions     │
│    │                                                                   │
│    └─▶ LiteLLM Proxy                   Backend LLM & embedding routing │
└────────────────────────────────────────────────────────────────────────┘
```

### Core Tenets

1. **Client Never Holds Raw Kubeconfigs:** The TUI and web client authenticate via Keycloak JWTs. Kubeconfigs are stored encrypted at rest (AES-256-GCM) in the backend and passed in-memory to MCP servers on demand.
2. **TUI is the Primary Mutating Surface:** The Web Dashboard is strictly read-only for inspection and onboarding. The local TUI is the only mutating interface.
3. **Safety by Default:** All mutating operations (`delete_pod`, `apply_manifest`, `exec_command`, `sync_app`) require explicit human confirmation—both in the TUI command palette and via LangGraph `interrupt()` breakpoints in the conversational agent.
4. **Bring Your Own Key (BYOK) for TUI:** The TUI connects directly to your preferred OpenAI-compatible provider (default: MiniMax M3) for low-latency streaming without backend roundtrips for LLM tokens.

---

## 2. Key Features

### 💻 Textual TUI (`tui/`)
- **Dual Interface:** Full `k9s`-style keyboard-driven command palette (`:get pods`, `:ctx`, `:history`, `:login`, `:apply`, `:delete`, `:exec`) alongside an AI co-pilot chat rail.
- **LangGraph Human-in-the-Loop:** Mutating tool calls pause execution with `interrupt()`, rendering an interactive confirmation modal before performing real cluster modifications.
- **Context-Aware RAG:** Diagnostic queries trigger the `retrieve_docs` tool to pull relevant cluster metrics, pod statuses, and troubleshooting runbooks into the context window.

### 🛡️ Multi-Tenant Backend (`backend/`)
- **Go Orchestrator:** High-throughput Chi router with JWKS token validation, tenant-isolated routing, and encrypted credential storage.
- **FastMCP Microservices:** Modular Python FastMCP 3.x servers running over streamable-HTTP, providing fine-grained tools with standard docstring schemas.
- **Multi-Tenant RAG Engine:** Strict per-user collection isolation (`user_{user_id}_{collection}`) in Qdrant (with in-memory cosine similarity fallback), paired with an automated Go `rag-indexer` synchronizing cluster state and markdown runbooks.
- **Immutable Audit Trail:** Comprehensive `action_history` table recording every mutation, author, target cluster, and originating client type (`tui`, `tui_agent`, `web`).

### 🌐 Web Dashboard (`web/`)
- **Cluster Management:** Secure onboarding modal to paste or upload kubeconfigs with client-side context parsing and backend encryption.
- **Resource Browser:** Real-time pod monitoring with search, namespace filters, container status badges, and inspection drawer with copyable TUI commands.
- **Audit Feed:** Live activity feed tracking operational history across registered clusters.
- **OIDC Authentication:** Next.js 15 App Router with Keycloak PKCE authorization code flow.

---

## 3. Repository Structure

```
strata/
├── AGENTS.md                 # Source of truth for architecture & phase plan
├── handoff.md                # Live cross-session state & engineering log
├── README.md                 # This overview
├── Makefile                  # Unified developer targets across all tiers
├── tui/                      # Textual TUI & local LangGraph agent (Python, uv)
│   ├── strata_tui/           # TUI application, command palette, agent graph, API client
│   └── tests/                # Async TUI and client test suite
├── backend/                  # Remote Kubernetes multi-tenant backend
│   ├── services/
│   │   ├── shared/           # Shared Go packages: crypto (AES-GCM), JWKS, MCP client
│   │   ├── orchestrator/     # Chi REST API, cluster registry, audit store, proxy routes
│   │   ├── retriever/        # Vector retrieval microservice (Qdrant & in-memory)
│   │   ├── rag-indexer/      # Cluster state & runbook ingestion daemon
│   │   └── agent-service/    # Backend agent service (FastAPI + LangGraph)
│   ├── mcp-servers/          # FastMCP servers (Python, streamable-HTTP)
│   │   ├── k8s/              # Pods, manifests, exec tools
│   │   ├── argocd/           # Applications, sync, status tools
│   │   ├── aws/              # EKS clusters, nodegroups, CloudWatch tools
│   │   ├── helm/             # Releases, values, revisions tools
│   │   └── shared/           # Shared MCP utilities (strata_mcp_common)
│   └── helm/strata/          # Umbrella Helm chart for backend deployment
├── web/                      # Next.js 15 Web Dashboard (TypeScript, Tailwind, pnpm)
├── infra/                    # Infrastructure as Code
│   ├── kind/                 # Local kind cluster definitions (strata-dev.yaml)
│   └── bootstrap/            # AWS Terraform configs (EKS, KMS, IRSA, ACM)
├── docs/                     # Comprehensive architecture and technical deep-dives
│   ├── langchain/            # 8 LangChain reference guides
│   ├── langgraph/            # 12 LangGraph reference guides
│   ├── rag.md                # Multi-tenant RAG architecture & reference
│   ├── mcp.md                # Model Context Protocol specifications
│   ├── strata/               # System architecture, security model, and data flows
│   └── ...                   # Keycloak, Envoy Gateway, CNPG, Textual docs
└── scripts/                  # Automated verification & e2e smoke test scripts
```

---

## 4. Phase Roadmap

| Phase | Description | Status |
|---|---|---|
| **Phase 0** | Repo Reset & TUI Graduation from sandbox | ✅ Complete |
| **Phase 1** | Local kind Backend Skeleton + FastMCP k8s Server | ✅ Complete |
| **Phase 2** | Keycloak OIDC, Next.js Web Auth, and TUI Device Flow | ✅ Complete |
| **Phase 3** | Mutation Tools, Confirmation Modal, and LangGraph Interrupts | ✅ Complete |
| **Phase 4** | Encrypted Cluster Registry (AES-256-GCM) & Web Onboarding | ✅ Complete |
| **Phase 5** | Web Dashboard (Read-Only Resource Browser & Audit Feed) | ✅ Complete |
| **Phase 6** | Multi-Tenant RAG (Per-User Collections, Retriever, RAG Indexer) | ✅ Complete |
| **Phase 7** | **More MCP Servers (ArgoCD, AWS, Helm) & Agent Tool Chaining** | 🚀 **In Progress** |
| **Phase 8** | Real EKS Bootstrap (Terraform VPC/EKS/KMS/IRSA & Production Helm) | ⏳ Queued |
| **Phase 9** | Polish, Observability (Prometheus/Grafana/Tempo), and CI Hardening | ⏳ Queued |

---

## 5. Quickstart

### Prerequisites

- **Docker** & **Kind** (for local backend)
- **Go 1.22+**
- **Python 3.12+** and [`uv`](https://docs.astral.sh/uv/)
- **Node.js 20+** and [`pnpm`](https://pnpm.io/)
- **Helm 3.x**

### 1. Environment Setup

Configure your LLM credentials for the TUI (default MiniMax M3, or any OpenAI-compatible API):

```bash
# In your shell profile or local environment:
export MINIMAX_API_KEY="your-api-key"
export MINIMAX_GROUP_ID="your-group-id"   # if required by your account
# Alternatively, configure OpenAI:
# export OPENAI_API_KEY="your-openai-key"
```

### 2. Local Kind Backend

Spin up the local development environment:

```bash
# 1. Create the kind cluster
make kind-up

# 2. Deploy platform dependencies (Envoy Gateway, CloudNativePG)
make platform-deps

# 3. Build container images and deploy via Helm
make backend-up
```

### 3. Web Dashboard

Run the Next.js web application for cluster onboarding and inspection:

```bash
make web-dev
# Open http://localhost:3000
```

### 4. Textual TUI

Launch the TUI on your laptop:

```bash
make tui-dev
```

Inside the TUI:
- Type `:login` to trigger the OIDC device authorization flow.
- Type `:ctx` to list or switch registered clusters.
- Type `:get pods` to inspect running workloads.
- Type `:delete <pod>` or `:apply` to perform mutations with safety confirmation.
- Use the chat rail to ask the AI agent questions:  
  *“Why is the checkout pod crashing?”*  
  *“Retrieve the operational runbook for memory leaks.”*

---

## 6. Developer Commands

The root [`Makefile`](Makefile) provides unified commands across all tiers:

```bash
# TUI
make tui-dev          # Run TUI with uv
make tui-test         # Run TUI pytest suite (45 tests)
make tui-lint         # Lint TUI with ruff

# Backend Go Services (shared, orchestrator, retriever, rag-indexer)
make backend-test     # Run unit tests across all 4 Go services
make backend-lint     # Run go vet and gofmt verification

# FastMCP Python Servers
make mcp-k8s-test     # Run pytest suite for FastMCP k8s server (22 tests)
make mcp-k8s-lint     # Lint FastMCP k8s server with ruff

# Web Dashboard (Next.js)
make web-dev          # Start Next.js dev server
make web-test         # Run Vitest test suite (11 tests)
make web-lint         # Run ESLint and TypeScript checks
make web-build        # Build Next.js production bundle

# Local Kind & End-to-End
make kind-up          # Create strata-dev kind cluster
make kind-down        # Tear down kind cluster
make backend-up       # Build images, load to kind, helm upgrade --install
make backend-logs     # Follow pod logs across the strata namespace
make e2e              # Automated end-to-end smoke test
make reset            # Clean caches, virtual environments, and node_modules
```

---

## 7. Security Model

- **Zero Client-Side Credentials:** The TUI and browser never store raw kubeconfig files. Credentials are encrypted upon upload with AES-256-GCM using authenticated envelope encryption.
- **Per-Tenant Vector Isolation:** Vector search queries are restricted to collections prefixed with `user_{user_id}_`, preventing cross-tenant leakage in RAG retrieval.
- **Role-Based Token Validation:** All backend services validate Keycloak JWTs via public key sets (JWKS). Tenant identity is extracted directly from the verified `sub` claim.
- **Strict Human Confirmation:** Destructive actions cannot be executed autonomously by LLM tool calls. The LangGraph agent relies on explicit user approval before mutations execute.

---

## 8. Documentation

Comprehensive reference documentation is available in the [`docs/`](docs/) directory:

- [System Architecture](docs/strata/backend-architecture.md) & [Data Flow](docs/strata/data-flow.md)
- [Security Model](docs/strata/security-model.md)
- [Multi-Tenant RAG Guide](docs/rag.md)
- [Model Context Protocol (MCP)](docs/mcp.md) & [MCP Architecture](docs/strata/mcp-architecture.md)
- [LangGraph Deep-Dive (12 chapters)](docs/langgraph/)
- [LangChain Deep-Dive (8 chapters)](docs/langchain/)
- [Keycloak OIDC Integration](docs/keycloak.md)
- [Envoy Gateway Routing](docs/envoy-gateway.md)

---

## 9. License

Apache 2.0 (see [LICENSE](LICENSE) when published).
