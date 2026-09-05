"""``list_pods`` MCP tool.

Lists pods in a Kubernetes cluster. Phase 1 supports an optional
``namespace`` and ``label_selector`` (Kubernetes standard).

The tool reads the user's kubeconfig from an environment variable
populated by the orchestrator (or, in dev, set directly). The
``kubernetes`` Python client constructs an API client from the
kubeconfig on demand.
"""

from __future__ import annotations

import logging
from typing import Any

from fastmcp import FastMCP
from kubernetes import client
from kubernetes.client.rest import ApiException
from pydantic import BaseModel, Field

from strata_mcp_k8s.kube import load_kubeconfig

logger = logging.getLogger(__name__)


class ListPodsInput(BaseModel):
    """Inputs for the ``list_pods`` tool."""

    cluster_id: str = Field(
        description=(
            "The Strata cluster ID this request targets. The "
            "orchestrator resolves the cluster to a kubeconfig "
            "and forwards the request to the right MCP server."
        )
    )
    namespace: str = Field(
        default="default",
        description=(
            "Kubernetes namespace. Empty string lists all namespaces "
            "(same as ``kubectl get pods -A``)."
        ),
    )
    label_selector: str | None = Field(
        default=None,
        description=(
            "Kubernetes label selector, e.g. ``app=nginx``. Same "
            "syntax as ``kubectl --selector``."
        ),
    )


class PodSummary(BaseModel):
    """One row of the ``list_pods`` response."""

    name: str
    namespace: str
    node: str | None
    phase: str
    ready: str  # e.g. "1/1"
    restarts: int
    age: str  # human-readable, "5d", "2h", "30s"


def register_list_pods(mcp: FastMCP) -> None:
    """Register the ``list_pods`` tool on the given FastMCP server."""

    @mcp.tool(name="list_pods", description=LIST_PODS_DESCRIPTION)
    def list_pods(
        cluster_id: str,
        namespace: str = "default",
        label_selector: str | None = None,
        kubeconfig_encrypted: str | None = None,
        kubeconfig_path: str | None = None,
    ) -> list[dict[str, Any]]:
        """List pods in a Kubernetes cluster.

        See ``LIST_PODS_DESCRIPTION`` for the full description that
        the LLM sees.
        """
        try:
            api_client = load_kubeconfig(
                cluster_id,
                kubeconfig_encrypted=kubeconfig_encrypted,
                kubeconfig_path=kubeconfig_path,
            )
            api = client.CoreV1Api(api_client)
            kwargs: dict[str, Any] = {}
            if namespace:
                kwargs["namespace"] = namespace
            else:
                kwargs["namespace"] = ""
            if label_selector:
                kwargs["label_selector"] = label_selector
            resp = api.list_pod_for_all_namespaces if not namespace else api.list_namespaced_pod
            pods = resp(**kwargs)
        except ApiException as exc:
            logger.warning("list_pods: k8s API error: %s", exc.reason)
            raise RuntimeError(f"kubernetes API error: {exc.reason}") from exc
        except Exception as exc:  # noqa: BLE001
            logger.exception("list_pods: unexpected error")
            raise RuntimeError(f"list_pods failed: {exc}") from exc

        return [_format_pod(p) for p in pods.items]


LIST_PODS_DESCRIPTION = """\
List pods in a Kubernetes cluster registered with Strata.

Use this when the user asks:
- "Show me the pods in cluster X."
- "Are there any failing pods in namespace Y?"
- "What pods are running with label app=nginx?"

Returns a list of pod summaries with name, namespace, node, phase,
ready container count (e.g. "1/1"), restart count, and age. The
shape matches the columns ``kubectl get pods`` shows.

If the cluster ID is unknown or the cluster's API server is
unreachable, the tool raises an error that the agent should
report plainly.

This is a read-only tool. Phase 1 ships only this tool; mutations
(delete, apply, exec) land in Phase 3.
"""


def _format_pod(pod) -> dict[str, Any]:
    """Format one ``V1Pod`` into the ``PodSummary`` shape."""
    status = pod.status
    phase = status.phase or "Unknown"
    container_statuses = status.container_statuses or []
    ready_count = sum(1 for c in container_statuses if c.ready)
    total = len(container_statuses)
    restarts = sum(c.restart_count or 0 for c in container_statuses)
    node = getattr(status, "node_name", None) or getattr(status, "host_ip", None)
    return {
        "name": pod.metadata.name,
        "namespace": pod.metadata.namespace,
        "node": node,
        "phase": phase,
        "ready": f"{ready_count}/{total}",
        "restarts": restarts,
        "age": _humanize_age(pod.metadata.creation_timestamp),
    }


def _humanize_age(ts) -> str:
    """Format a Kubernetes timestamp as a kubectl-style age string."""
    from datetime import UTC, datetime

    if ts is None:
        return "unknown"
    if isinstance(ts, str):
        # kubernetes client returns datetime, but defensively handle str.
        ts = datetime.fromisoformat(ts.replace("Z", "+00:00"))
    if ts.tzinfo is None:
        ts = ts.replace(tzinfo=UTC)
    delta = datetime.now(UTC) - ts
    seconds = int(delta.total_seconds())
    if seconds < 60:
        return f"{seconds}s"
    if seconds < 3600:
        return f"{seconds // 60}m"
    if seconds < 86400:
        return f"{seconds // 3600}h"
    return f"{seconds // 86400}d"