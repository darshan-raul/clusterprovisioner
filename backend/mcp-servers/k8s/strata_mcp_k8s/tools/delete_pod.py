"""``delete_pod`` MCP tool (MUTATION).

Deletes a pod in a Kubernetes cluster.
"""

from __future__ import annotations

import logging
from typing import Any

from fastmcp import FastMCP
from kubernetes import client
from kubernetes.client.rest import ApiException

from strata_mcp_k8s.kube import load_kubeconfig

logger = logging.getLogger(__name__)

DELETE_POD_DESCRIPTION = """\
[MUTATION] Delete a pod in a Kubernetes cluster.

Use this when the user asks:
- "Delete pod nginx-123 in namespace prod."
- "Kill the crashing pod in default namespace."

This is a MUTATING operation and requires explicit confirmation.
Returns the deletion status and resource identifier.
"""


def register_delete_pod(mcp: FastMCP) -> None:
    """Register the ``delete_pod`` tool on the given FastMCP server."""

    @mcp.tool(
        name="delete_pod",
        description=DELETE_POD_DESCRIPTION,
        tags={"mutation", "k8s", "delete"},
    )
    def delete_pod(
        cluster_id: str,
        name: str,
        namespace: str = "default",
        grace_period_seconds: int | None = None,
        kubeconfig_encrypted: str | None = None,
        kubeconfig_path: str | None = None,
    ) -> dict[str, Any]:
        """Delete a pod in a Kubernetes cluster.

        See ``DELETE_POD_DESCRIPTION`` for full details.
        """
        try:
            api_client = load_kubeconfig(
                cluster_id,
                kubeconfig_encrypted=kubeconfig_encrypted,
                kubeconfig_path=kubeconfig_path,
            )
            api = client.CoreV1Api(api_client)
            delete_opts = (
                client.V1DeleteOptions(grace_period_seconds=grace_period_seconds)
                if grace_period_seconds is not None
                else None
            )
            api.delete_namespaced_pod(
                name=name,
                namespace=namespace,
                body=delete_opts,
            )
        except ApiException as exc:
            logger.warning("delete_pod: k8s API error: %s", exc.reason)
            raise RuntimeError(f"kubernetes API error: {exc.reason}") from exc
        except Exception as exc:  # noqa: BLE001
            logger.exception("delete_pod: unexpected error")
            raise RuntimeError(f"delete_pod failed: {exc}") from exc

        return {
            "status": "deleted",
            "cluster_id": cluster_id,
            "namespace": namespace,
            "name": name,
        }
