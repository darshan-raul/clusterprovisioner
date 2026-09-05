# RAG — Retrieval-Augmented Generation in Strata

Retrieval-Augmented Generation (RAG) is the pattern of *retrieving relevant context from an external store and injecting it into the LLM prompt* before answering. Strata's co-pilot uses RAG so it can answer questions about Strata itself (architecture, runbooks, ops), about specific Kubernetes clusters (status, pods, recent audit activity), and about platform tools (ArgoCD, EKS, Helm) — without stuffing all cluster state into the LLM's system prompt.

---

## 1. Why RAG in Strata

LLMs are trained on public snapshots and have no awareness of:
1. **Live cluster state:** What pods are failing in `default` or `kube-system` right now.
2. **Cluster registration & audit logs:** Recent mutations, pod deletions, or manifest applications.
3. **Internal runbooks & architecture:** Project docs under `docs/` or operational playbooks.

Three approaches to provide this knowledge:
1. **Prompt stuffing:** Infeasible beyond a few dozen lines; context window exhaustion and high latency.
2. **Fine-tuning:** Slow, expensive, static snapshot.
3. **Query-time retrieval (RAG):** Dynamic, multi-tenant, fresh, and provides source citations.

---

## 2. Multi-Tenant Architecture & Data Flow

```
                      ┌───────────────────────┐
                      │      Strata TUI       │
                      │ (LangGraph agent rail)│
                      └──────────┬────────────┘
                                 │
                                 │ POST /api/v1/retrieve
                                 │ Authorization: Bearer <jwt>
                                 ▼
                      ┌───────────────────────┐
                      │      Orchestrator     │
                      │ (JWT auth + user_id)  │
                      └──────────┬────────────┘
                                 │
                                 │ POST /retrieve
                                 │ X-Strata-User: <user_id>
                                 ▼
                      ┌───────────────────────┐
                      │   retriever-service   │
                      │ (Go, user isolation)  │
                      └──────┬──────────┬─────┘
       POST /v1/embeddings   │          │  Vector Search (Cosine)
                             ▼          ▼
                      ┌────────────┐ ┌─────────────┐
                      │  LiteLLM   │ │   Qdrant    │
                      │ (Embedder) │ │ (Vector DB) │
                      └────────────┘ └─────────────┘

  Ingestion loop (every 60s or on-demand):
    Postgres / K8s State ──▶ rag-indexer (Go) ──▶ POST /index ──▶ retriever-service
```

### Multi-Tenant Isolation Invariant
Per-user isolation is enforced at the retriever layer:
- Every collection name is namespaced per user: `user_{user_id}_{collection}` (e.g. `user_alice_clusters`, `user_alice_docs`).
- Chunks and points also contain a payload attribute `user_id = "<user_id>"`.
- The `retriever-service` checks the caller's authenticated user identity from the validated JWT claims or `X-Strata-User` header. Users can never query or cross-retrieve vectors belonging to another tenant.

---

## 3. The `retriever-service` REST API

### `POST /retrieve`
Retrieve relevant chunks from a user-scoped collection.

**Headers:**
`Authorization: Bearer <jwt>` or `X-Strata-User: <user_id>`

**Request:**
```json
{
  "collection": "clusters",
  "query": "which pods have restarts in prod?",
  "top_k": 5,
  "filter": {
    "cluster_id": "cl-001"
  }
}
```

**Response:**
```json
{
  "chunks": [
    {
      "id": "cl-001/prod/api-deployment-789",
      "text": "Cluster cl-001, Namespace prod. Pod api-deployment-789 has 3 restarts, status CrashLoopBackOff.",
      "score": 0.89,
      "metadata": {
        "cluster_id": "cl-001",
        "namespace": "prod",
        "kind": "pod"
      }
    }
  ]
}
```

### `POST /index`
Upsert a chunk and its vector embedding into the user collection.

**Request:**
```json
{
  "collection": "clusters",
  "id": "cl-001/prod/api-deployment-789",
  "text": "Cluster cl-001, Namespace prod. Pod api-deployment-789 has 3 restarts, status CrashLoopBackOff.",
  "metadata": {
    "cluster_id": "cl-001",
    "namespace": "prod",
    "kind": "pod"
  }
}
```

**Response:**
```json
{
  "upserted": true,
  "id": "cl-001/prod/api-deployment-789"
}
```

### `DELETE /index/{collection}/{id}`
Deletes a point from the user-scoped collection.

### `GET /healthz`
Returns `{"status": "ok"}`.

---

## 4. Vector Store & Embedder Abstractions

To ensure fast local development and CI testing without running external dependencies:
1. **Vector Store:**
   - `QdrantStore`: Communicates with Qdrant REST API (`/collections/{name}/points/search` and upsert).
   - `MemoryStore`: Thread-safe in-memory vector storage with exact cosine similarity matching, ideal for local tests and offline dev.
2. **Embedder:**
   - `OpenAIEmbedder`: Embeds text via LiteLLM / OpenAI `/v1/embeddings` endpoint.
   - `DeterministicMockEmbedder`: Generates deterministic unit-normalized pseudo-embeddings based on text hashing and term frequencies, enabling full semantic assertions in unit tests without API keys.

---

## 5. Ingestion Pipeline (`rag-indexer`)

The `rag-indexer` service synchronizes cluster resource states into vector collections:
1. Queries registered clusters from the orchestrator / database.
2. Formats cluster status, workload pods, and recent audit events into structured text chunks.
3. Posts chunks to `retriever-service` `POST /index` with `collection: "clusters"` and tenant metadata.
4. Reads markdown documentation and runbooks from disk, chunks headers and paragraphs, and indexes into `collection: "docs"`.
5. Supports both a daemon mode (periodic ticker) and a single-shot execution mode (`--once`).

---

## 6. LangGraph Agent Integration

The Strata TUI LangGraph agent exposes retrieval via both a tool and a conditional routing edge:
- **`retrieve_docs` tool:** Allows the LLM to dynamically retrieve context when it decides it needs background information.
- **Conditional Routing (`should_retrieve`):** Inspects user queries. When questions contain informational or diagnostic intent ("how do I", "why is", "check cluster status", "what is the architecture of"), the graph routes through the `retrieve` node to prepopulate context before calling the language model.