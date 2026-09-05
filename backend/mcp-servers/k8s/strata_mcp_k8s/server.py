"""Strata MCP k8s server.

A FastMCP server that exposes Kubernetes operations as MCP tools.
Phase 1 ships one tool: ``list_pods``. Mutations land in Phase 3.

Authentication: the orchestrator forwards the user's Keycloak JWT
in the ``Authorization`` header. We validate the JWT against the
configured JWKS endpoint, then extract the ``sub`` claim as the
``user_id``. The orchestrator also passes the cluster's
kubeconfig in an ``X-Strata-Kubeconfig`` header (the path on
disk to a kubeconfig the orchestrator mounted).

Per the AGENTS.md locked decisions, this server runs in the
backend k8s cluster and is reached over streamable-HTTP via the
Envoy Gateway. The TUI never sees the kubeconfig.
"""

from __future__ import annotations

import logging
import os

from fastmcp import FastMCP

from strata_mcp_k8s.tools import (
    register_apply_manifest,
    register_delete_pod,
    register_exec_command,
    register_list_pods,
)

logging.basicConfig(
    level=os.getenv("LOG_LEVEL", "info").upper(),
    format="%(asctime)s %(levelname)s %(name)s: %(message)s",
)
logger = logging.getLogger(__name__)

mcp = FastMCP(
    name="strata-k8s",
    instructions=(
        "Kubernetes tools for the Strata backend. Tools take a "
        "cluster_id and resource-scoped arguments (namespace, "
        "manifest, command, etc.). Mutating operations require confirmation."
    ),
)

register_list_pods(mcp)
register_delete_pod(mcp)
register_apply_manifest(mcp)
register_exec_command(mcp)


def main() -> None:
    """Run the MCP server over streamable-HTTP."""
    host = os.getenv("MCP_HOST", "0.0.0.0")
    port = int(os.getenv("MCP_PORT", "8000"))
    stateless = os.getenv("MCP_STATELESS", "true").lower() == "true"
    logger.info(
        "starting strata-mcp-k8s on %s:%d (stateless_http=%s)", host, port, stateless
    )
    mcp.run(
        transport="streamable-http",
        host=host,
        port=port,
        show_banner=False,
        stateless_http=stateless,
    )


if __name__ == "__main__":
    main()