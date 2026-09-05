"""Unit tests for ``exec_command`` tool."""

from __future__ import annotations

import asyncio
from unittest.mock import MagicMock, patch

import pytest
from fastmcp import FastMCP
from kubernetes.client.rest import ApiException

from strata_mcp_k8s.tools.exec_command import register_exec_command


@pytest.fixture
def mcp() -> FastMCP:
    server = FastMCP(name="test")
    register_exec_command(server)
    return server


async def _get_tool(mcp: FastMCP, name: str):
    tools = await mcp._local_provider.list_tools()
    for tool in tools:
        if tool.name == name:
            return tool
    raise KeyError(name)


@pytest.mark.asyncio
async def test_exec_command_string_converted_to_sh(mcp: FastMCP) -> None:
    api = MagicMock()

    with patch("strata_mcp_k8s.tools.exec_command.load_kubeconfig") as mock_load, \
         patch("strata_mcp_k8s.tools.exec_command.client") as mock_client, \
         patch("strata_mcp_k8s.tools.exec_command.stream") as mock_stream:
        mock_load.return_value = MagicMock()
        mock_client.CoreV1Api.return_value = api
        mock_stream.return_value = "hello world\n"

        tool = await _get_tool(mcp, "exec_command")
        res = tool.fn(
            cluster_id="cl-01",
            pod="my-pod",
            command="echo 'hello world'",
            namespace="default",
            container="app",
        )

    assert res["status"] == "completed"
    assert res["pod"] == "my-pod"
    assert res["container"] == "app"
    assert res["output"] == "hello world\n"
    mock_stream.assert_called_once()
    _, kwargs = mock_stream.call_args
    assert kwargs["command"] == ["sh", "-c", "echo 'hello world'"]
    assert kwargs["container"] == "app"


@pytest.mark.asyncio
async def test_exec_command_list(mcp: FastMCP) -> None:
    api = MagicMock()

    with patch("strata_mcp_k8s.tools.exec_command.load_kubeconfig") as mock_load, \
         patch("strata_mcp_k8s.tools.exec_command.client") as mock_client, \
         patch("strata_mcp_k8s.tools.exec_command.stream") as mock_stream:
        mock_load.return_value = MagicMock()
        mock_client.CoreV1Api.return_value = api
        mock_stream.return_value = "root\n"

        tool = await _get_tool(mcp, "exec_command")
        res = tool.fn(
            cluster_id="cl-01",
            pod="my-pod",
            command=["whoami"],
            namespace="kube-system",
        )

    assert res["status"] == "completed"
    assert res["output"] == "root\n"
    _, kwargs = mock_stream.call_args
    assert kwargs["command"] == ["whoami"]
    assert "container" not in kwargs


@pytest.mark.asyncio
async def test_exec_command_empty_fails(mcp: FastMCP) -> None:
    tool = await _get_tool(mcp, "exec_command")
    with pytest.raises(RuntimeError, match="command cannot be empty"):
        tool.fn(cluster_id="cl-01", pod="my-pod", command=[])


@pytest.mark.asyncio
async def test_exec_command_error_propagates(mcp: FastMCP) -> None:
    with patch("strata_mcp_k8s.tools.exec_command.load_kubeconfig"), \
         patch("strata_mcp_k8s.tools.exec_command.client"), \
         patch("strata_mcp_k8s.tools.exec_command.stream") as mock_stream:
        mock_stream.side_effect = ApiException(status=500, reason="Server error")

        tool = await _get_tool(mcp, "exec_command")
        with pytest.raises(RuntimeError, match="Server error"):
            tool.fn(cluster_id="cl-01", pod="my-pod", command="exit 1")


def test_exec_command_is_tagged_mutation(mcp: FastMCP) -> None:
    tools = asyncio.run(mcp._local_provider.list_tools())
    tool = next(t for t in tools if t.name == "exec_command")
    assert "mutation" in tool.tags
    assert "[MUTATION]" in tool.description
