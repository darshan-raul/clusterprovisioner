"""``exec_command`` MCP tool (MUTATION).

Executes a command inside a Kubernetes pod's container.
"""

from __future__ import annotations

import logging
from typing import Any

from fastmcp import FastMCP
from kubernetes import client
from kubernetes.client.rest import ApiException
from kubernetes.stream import stream

from strata_mcp_k8s.kube import load_kubeconfig

logger = logging.getLogger(__name__)

EXEC_COMMAND_DESCRIPTION = """\
[MUTATION] Execute a command inside a container within a Kubernetes pod.

Use this when the user asks:
- "Exec into pod nginx-123 and run 'cat /etc/nginx/nginx.conf'."
- "Run 'env' inside the web container in pod api-789."

This is a MUTATING/INTERACTIVE operation and requires explicit confirmation.
Returns the command execution output.
"""


def register_exec_command(mcp: FastMCP) -> None:
    """Register the ``exec_command`` tool on the given FastMCP server."""

    @mcp.tool(
        name="exec_command",
        description=EXEC_COMMAND_DESCRIPTION,
        tags={"mutation", "k8s", "exec"},
    )
    def exec_command(
        cluster_id: str,
        pod: str,
        command: list[str] | str,
        namespace: str = "default",
        container: str | None = None,
    ) -> dict[str, Any]:
        """Execute a command inside a Kubernetes pod.

        See ``EXEC_COMMAND_DESCRIPTION`` for full details.
        """
        if isinstance(command, str):
            cmd_list = ["sh", "-c", command]
        else:
            cmd_list = list(command)

        if not cmd_list:
            raise RuntimeError("command cannot be empty")

        try:
            api_client = load_kubeconfig(cluster_id)
            api = client.CoreV1Api(api_client)

            kwargs: dict[str, Any] = {
                "name": pod,
                "namespace": namespace,
                "command": cmd_list,
                "stderr": True,
                "stdin": False,
                "stdout": True,
                "tty": False,
            }
            if container:
                kwargs["container"] = container

            resp = stream(api.connect_get_namespaced_pod_exec, **kwargs)
        except ApiException as exc:
            logger.warning("exec_command: k8s API error: %s", exc.reason)
            raise RuntimeError(f"kubernetes API error: {exc.reason}") from exc
        except Exception as exc:  # noqa: BLE001
            logger.exception("exec_command: unexpected error")
            raise RuntimeError(f"exec_command failed: {exc}") from exc

        return {
            "status": "completed",
            "cluster_id": cluster_id,
            "namespace": namespace,
            "pod": pod,
            "container": container,
            "output": str(resp) if resp is not None else "",
        }
