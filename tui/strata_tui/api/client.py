"""HTTP client for the Strata orchestrator REST API.

The TUI talks to the orchestrator (which proxies to MCP servers)
for everything cluster-related. The client wraps httpx and adds
the bearer token to every request.
"""

from __future__ import annotations

from dataclasses import dataclass
from typing import Any

import httpx


class StrataClientError(RuntimeError):
    """Raised when the orchestrator returns an error."""


@dataclass
class Cluster:
    id: str
    name: str
    context: str


@dataclass
class Me:
    sub: str
    email: str
    name: str
    preferred_username: str


class StrataClient:
    """Async HTTP client for the Strata backend.

    Args:
        base_url: Where the orchestrator is reachable, e.g.
            ``http://localhost:8080`` (port-forwarded) or
            ``https://strata.example.com`` (production via Envoy).
        token: Optional bearer token. If set, every request carries it.
        http_client: Optional httpx.AsyncClient to reuse. If not set,
            the client creates its own with sensible timeouts.
    """

    def __init__(
        self,
        *,
        base_url: str,
        token: str | None = None,
        http_client: httpx.AsyncClient | None = None,
    ) -> None:
        self._base = base_url.rstrip("/")
        self._token = token
        self._owns_client = http_client is None
        self._http = http_client or httpx.AsyncClient(timeout=15.0)

    def with_token(self, token: str) -> StrataClient:
        """Return a new client that uses the given token."""
        return StrataClient(
            base_url=self._base,
            token=token,
            http_client=self._http,
        )

    def _headers(self) -> dict[str, str]:
        if self._token:
            return {"Authorization": f"Bearer {self._token}"}
        return {}

    async def _get(self, path: str) -> dict[str, Any]:
        resp = await self._http.get(
            f"{self._base}{path}",
            headers=self._headers(),
        )
        return _parse(resp)

    async def me(self) -> Me:
        body = await self._get("/api/v1/me")
        return Me(
            sub=body.get("sub", ""),
            email=body.get("email", ""),
            name=body.get("name", ""),
            preferred_username=body.get("preferred_username", ""),
        )

    async def list_clusters(self) -> list[Cluster]:
        body = await self._get("/api/v1/clusters/")
        clusters = body.get("clusters") or []
        return [
            Cluster(
                id=c["id"],
                name=c["name"],
                context=c.get("context", ""),
            )
            for c in clusters
        ]

    async def create_cluster(
        self,
        name: str,
        kubeconfig: str,
        *,
        context: str | None = None,
    ) -> Cluster:
        payload: dict[str, Any] = {
            "name": name,
            "kubeconfig": kubeconfig,
        }
        if context:
            payload["context"] = context
        resp = await self._http.post(
            f"{self._base}/api/v1/clusters",
            headers=self._headers(),
            json=payload,
        )
        data = _parse(resp)
        c = data.get("cluster") or {}
        return Cluster(
            id=c.get("id", ""),
            name=c.get("name", ""),
            context=c.get("context", ""),
        )

    async def delete_cluster(self, cluster_id: str) -> dict[str, Any]:
        resp = await self._http.delete(
            f"{self._base}/api/v1/clusters/{cluster_id}",
            headers=self._headers(),
        )
        return _parse(resp)

    async def list_pods(
        self,
        cluster_id: str,
        *,
        namespace: str | None = None,
        label_selector: str | None = None,
    ) -> list[dict[str, Any]]:
        params: dict[str, str] = {}
        if namespace:
            params["namespace"] = namespace
        if label_selector:
            params["label-selector"] = label_selector
        # The pods endpoint returns the MCP tool's content envelope
        # directly (no wrapping), so we unwrap it for callers.
        body = await self._http.get(
            f"{self._base}/api/v1/clusters/{cluster_id}/pods",
            headers=self._headers(),
            params=params,
        )
        if body.status_code == 404:
            raise StrataClientError(f"cluster {cluster_id!r} not found")
        data = body.json()
        if not isinstance(data, list):
            # The MCP tool returned an error envelope: {"content": [{"type":"text","text": "..."}]}
            content = data.get("content") if isinstance(data, dict) else None
            if content and isinstance(content, list) and content:
                first = content[0]
                if isinstance(first, dict) and first.get("type") == "text":
                    raise StrataClientError(first.get("text", "unknown error"))
            raise StrataClientError(f"unexpected response shape: {data!r}")
        return data

    async def delete_pod(
        self,
        cluster_id: str,
        name: str,
        *,
        namespace: str | None = None,
        grace_period_seconds: int | None = None,
    ) -> dict[str, Any]:
        params: dict[str, str] = {}
        if namespace:
            params["namespace"] = namespace
        if grace_period_seconds is not None:
            params["grace-period-seconds"] = str(grace_period_seconds)
        resp = await self._http.delete(
            f"{self._base}/api/v1/clusters/{cluster_id}/pods/{name}",
            headers=self._headers(),
            params=params,
        )
        return _parse_mcp(resp)

    async def apply_manifest(
        self,
        cluster_id: str,
        manifest: str,
        *,
        namespace: str | None = None,
    ) -> dict[str, Any]:
        payload: dict[str, Any] = {"manifest": manifest}
        if namespace:
            payload["namespace"] = namespace
        resp = await self._http.post(
            f"{self._base}/api/v1/clusters/{cluster_id}/apply",
            headers=self._headers(),
            json=payload,
        )
        return _parse_mcp(resp)

    async def exec_command(
        self,
        cluster_id: str,
        pod: str,
        command: str | list[str],
        *,
        namespace: str | None = None,
        container: str | None = None,
    ) -> dict[str, Any]:
        payload: dict[str, Any] = {"command": command}
        if namespace:
            payload["namespace"] = namespace
        if container:
            payload["container"] = container
        resp = await self._http.post(
            f"{self._base}/api/v1/clusters/{cluster_id}/pods/{pod}/exec",
            headers=self._headers(),
            json=payload,
        )
        return _parse_mcp(resp)

    async def healthz(self) -> dict[str, Any]:
        return await self._get("/healthz")

    async def retrieve(
        self,
        query: str,
        *,
        collection: str = "clusters",
        top_k: int = 5,
        filter: dict[str, Any] | None = None,
    ) -> list[dict[str, Any]]:
        """Retrieve relevant RAG context chunks from the backend."""
        payload: dict[str, Any] = {
            "query": query,
            "collection": collection,
            "top_k": top_k,
        }
        if filter:
            payload["filter"] = filter
        resp = await self._http.post(
            f"{self._base}/api/v1/retrieve",
            headers=self._headers(),
            json=payload,
        )
        data = _parse(resp)
        return data.get("chunks", [])

    async def aclose(self) -> None:
        if self._owns_client and self._http is not None:
            await self._http.aclose()


def _parse(resp: httpx.Response) -> dict[str, Any]:
    if resp.status_code == 401:
        raise StrataClientError("unauthorized (token may be expired)")
    if resp.status_code == 404:
        raise StrataClientError(f"not found: {resp.url.path}")
    if resp.status_code >= 400:
        raise StrataClientError(
            f"HTTP {resp.status_code} on {resp.url.path}: {resp.text[:200]}"
        )
    try:
        return resp.json()
    except ValueError as exc:
        raise StrataClientError(
            f"non-JSON response from {resp.url.path}: {exc}"
        ) from exc


def _parse_mcp(resp: httpx.Response) -> dict[str, Any]:
    if resp.status_code == 401:
        raise StrataClientError("unauthorized (token may be expired)")
    if resp.status_code == 404:
        raise StrataClientError(f"not found: {resp.url.path}")
    if resp.status_code >= 400:
        raise StrataClientError(
            f"HTTP {resp.status_code} on {resp.url.path}: {resp.text[:200]}"
        )
    try:
        data = resp.json()
    except ValueError as exc:
        raise StrataClientError(f"non-JSON response from {resp.url.path}: {exc}") from exc

    if isinstance(data, dict):
        if data.get("isError") and "content" in data:
            content = data.get("content")
            if isinstance(content, list) and content:
                first = content[0]
                if isinstance(first, dict) and first.get("type") == "text":
                    raise StrataClientError(first.get("text", "unknown error"))
            raise StrataClientError("MCP tool reported error")
        return data
    raise StrataClientError(f"unexpected response shape: {data!r}")