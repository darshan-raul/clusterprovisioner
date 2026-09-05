"""Unit tests for ``delete_pod`` tool."""

from __future__ import annotations

import asyncio
from unittest.mock import MagicMock, patch

import pytest
from fastmcp import FastMCP
from kubernetes.client.rest import ApiException

from strata_mcp_k8s.tools.delete_pod import register_delete_pod


@pytest.fixture
def mcp() -> FastMCP:
    server = FastMCP(name="test")
    register_delete_pod(server)
    return server


async def _get_tool(mcp: FastMCP, name: str):
    tools = await mcp._local_provider.list_tools()
    for tool in tools:
        if tool.name == name:
            return tool
    raise KeyError(name)


@pytest.mark.asyncio
async def test_delete_pod_success(mcp: FastMCP) -> None:
    api = MagicMock()

    with patch("strata_mcp_k8s.tools.delete_pod.load_kubeconfig") as mock_load, \
         patch("strata_mcp_k8s.tools.delete_pod.client") as mock_client:
        mock_load.return_value = MagicMock()
        mock_client.CoreV1Api.return_value = api
        mock_client.V1DeleteOptions = MagicMock()

        tool = await _get_tool(mcp, "delete_pod")
        res = tool.fn(
            cluster_id="cl-01",
            name="test-pod",
            namespace="production",
            grace_period_seconds=10,
        )

    assert res["status"] == "deleted"
    assert res["name"] == "test-pod"
    assert res["namespace"] == "production"
    assert res["cluster_id"] == "cl-01"
    api.delete_namespaced_pod.assert_called_once()
    call_kwargs = api.delete_namespaced_pod.call_args.kwargs
    assert call_kwargs["name"] == "test-pod"
    assert call_kwargs["namespace"] == "production"


@pytest.mark.asyncio
async def test_delete_pod_api_error_propagates(mcp: FastMCP) -> None:
    api = MagicMock()
    api.delete_namespaced_pod.side_effect = ApiException(status=404, reason="NotFound")

    with patch("strata_mcp_k8s.tools.delete_pod.load_kubeconfig"), \
         patch("strata_mcp_k8s.tools.delete_pod.client") as mock_client:
        mock_client.CoreV1Api.return_value = api
        tool = await _get_tool(mcp, "delete_pod")
        with pytest.raises(RuntimeError, match="NotFound"):
            tool.fn(cluster_id="cl-01", name="missing-pod", namespace="default")


def test_delete_pod_is_tagged_mutation(mcp: FastMCP) -> None:
    tools = asyncio.run(mcp._local_provider.list_tools())
    tool = next(t for t in tools if t.name == "delete_pod")
    assert "mutation" in tool.tags
    assert "[MUTATION]" in tool.description
