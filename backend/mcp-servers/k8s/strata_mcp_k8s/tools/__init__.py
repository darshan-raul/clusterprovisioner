"""Strata MCP k8s server — tool registry."""

from strata_mcp_k8s.tools.apply_manifest import register_apply_manifest
from strata_mcp_k8s.tools.delete_pod import register_delete_pod
from strata_mcp_k8s.tools.exec_command import register_exec_command
from strata_mcp_k8s.tools.list_pods import register_list_pods

__all__ = [
    "register_apply_manifest",
    "register_delete_pod",
    "register_exec_command",
    "register_list_pods",
]