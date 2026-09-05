"""Phase 1 MCP k8s server tests.

Tests ``list_pods`` with a mocked Kubernetes API client. We avoid
spinning up a real cluster — the FastMCP tool is a thin wrapper.
"""

from __future__ import annotations

import asyncio
from datetime import UTC, datetime, timedelta
from unittest.mock import MagicMock, patch

import pytest
from fastmcp import FastMCP

from strata_mcp_k8s.tools.list_pods import register_list_pods


@pytest.fixture
def mcp() -> FastMCP:
    server = FastMCP(name="test")
    register_list_pods(server)
    return server


async def _get_tool(mcp: FastMCP, name: str):
    tools = await mcp._local_provider.list_tools()
    for tool in tools:
        if tool.name == name:
            return tool
    raise KeyError(name)


def _make_pod(
    name: str,
    namespace: str = "default",
    phase: str = "Running",
    node: str = "node-1",
    ready: tuple[int, int] = (1, 1),
    restarts: int = 0,
    age_seconds: int = 60,
):
    pod = MagicMock()
    pod.metadata.name = name
    pod.metadata.namespace = namespace
    pod.metadata.creation_timestamp = datetime.now(UTC) - timedelta(seconds=age_seconds)
    pod.status.phase = phase
    pod.status.node_name = node
    statuses = []
    ready_count, total = ready
    for i in range(total):
        cs = MagicMock()
        cs.ready = i < ready_count
        cs.restart_count = restarts if i == 0 else 0
        statuses.append(cs)
    pod.status.container_statuses = statuses
    return pod


@pytest.mark.asyncio
async def test_list_pods_returns_formatted_summaries(mcp) -> None:
    """Happy path: three pods come back as formatted JSON."""
    pods = [
        _make_pod("a", "default", ready=(1, 1), restarts=0, age_seconds=30),
        _make_pod(
            "b", "default", phase="Pending", node=None, ready=(0, 1), restarts=2, age_seconds=120
        ),
        _make_pod("c", "kube-system", ready=(2, 2), restarts=5, age_seconds=86400),
    ]
    api = MagicMock()
    api.list_namespaced_pod.return_value.items = pods

    with patch("strata_mcp_k8s.tools.list_pods.client") as kc, \
         patch("strata_mcp_k8s.tools.list_pods.load_kubeconfig") as mock_load:
        mock_load.return_value = MagicMock()
        kc.CoreV1Api.return_value = api
        tool = await _get_tool(mcp, "list_pods")
        result = tool.fn(cluster_id="cl-mock-01", namespace="default")

    assert len(result) == 3
    assert result[0]["name"] == "a"
    assert result[0]["ready"] == "1/1"
    assert result[1]["phase"] == "Pending"
    assert result[1]["ready"] == "0/1"
    assert result[2]["age"].endswith("d")  # 86400s -> "1d"
    mock_load.assert_called_once()


@pytest.mark.asyncio
async def test_list_pods_propagates_k8s_api_error(mcp) -> None:
    """A 503 from the k8s API should surface as a clean error."""
    from kubernetes.client.rest import ApiException

    api = MagicMock()
    api.list_namespaced_pod.side_effect = ApiException(status=503, reason="unavailable")

    with patch("strata_mcp_k8s.tools.list_pods.client") as kc, \
         patch("strata_mcp_k8s.tools.list_pods.load_kubeconfig") as mock_load:
        mock_load.return_value = MagicMock()
        kc.CoreV1Api.return_value = api
        tool = await _get_tool(mcp, "list_pods")
        with pytest.raises(RuntimeError, match="unavailable"):
            tool.fn(cluster_id="cl-mock-01")


def test_list_pods_tool_is_registered(mcp) -> None:
    """The tool should be discoverable on the FastMCP instance."""
    tools = asyncio.run(mcp._local_provider.list_tools())
    names = [t.name for t in tools]
    assert "list_pods" in names


def test_list_pods_description_mentions_use_cases() -> None:
    """The docstring should describe when to use the tool."""
    from strata_mcp_k8s.tools.list_pods import LIST_PODS_DESCRIPTION

    text = LIST_PODS_DESCRIPTION.lower()
    assert "list" in text
    assert "pods" in text