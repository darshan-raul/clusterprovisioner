"""Strata MCP k8s server — tool registry."""

from strata_mcp_k8s.tools.list_pods import register_list_pods

__all__ = ["register_list_pods"]