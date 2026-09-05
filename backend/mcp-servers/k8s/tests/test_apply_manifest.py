"""Unit tests for ``apply_manifest`` tool."""

from __future__ import annotations

import asyncio
from unittest.mock import MagicMock, patch

import pytest
from fastmcp import FastMCP
from kubernetes.client.rest import ApiException
from kubernetes.utils import FailToCreateError

from strata_mcp_k8s.tools.apply_manifest import register_apply_manifest


@pytest.fixture
def mcp() -> FastMCP:
    server = FastMCP(name="test")
    register_apply_manifest(server)
    return server


async def _get_tool(mcp: FastMCP, name: str):
    tools = await mcp._local_provider.list_tools()
    for tool in tools:
        if tool.name == name:
            return tool
    raise KeyError(name)


SAMPLE_YAML = """
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
  namespace: default
data:
  key: value
"""


@pytest.mark.asyncio
async def test_apply_manifest_create_success(mcp: FastMCP) -> None:
    with patch("strata_mcp_k8s.tools.apply_manifest.load_kubeconfig") as mock_load, \
         patch("strata_mcp_k8s.tools.apply_manifest.utils.create_from_dict") as mock_create:
        mock_load.return_value = MagicMock()
        mock_create.return_value = MagicMock()

        tool = await _get_tool(mcp, "apply_manifest")
        res = tool.fn(cluster_id="cl-01", manifest_yaml=SAMPLE_YAML, namespace="default")

    assert res["status"] == "applied"
    assert res["count"] == 1
    assert res["applied"][0]["name"] == "test-config"
    assert res["applied"][0]["action"] == "created"
    mock_create.assert_called_once()


@pytest.mark.asyncio
async def test_apply_manifest_conflict_patches(mcp: FastMCP) -> None:
    mock_api = MagicMock()

    with patch("strata_mcp_k8s.tools.apply_manifest.load_kubeconfig") as mock_load, \
         patch("strata_mcp_k8s.tools.apply_manifest.utils.create_from_dict") as mock_create, \
         patch("strata_mcp_k8s.tools.apply_manifest._get_api_and_kind") as mock_get_api:
        mock_load.return_value = MagicMock()
        conflict_exc = ApiException(status=409, reason="Conflict")
        mock_create.side_effect = FailToCreateError([conflict_exc])
        mock_get_api.return_value = (mock_api, "config_map")

        tool = await _get_tool(mcp, "apply_manifest")
        res = tool.fn(cluster_id="cl-01", manifest_yaml=SAMPLE_YAML, namespace="default")

    assert res["status"] == "applied"
    assert res["count"] == 1
    assert res["applied"][0]["action"] == "configured"
    mock_api.patch_namespaced_config_map.assert_called_once()


@pytest.mark.asyncio
async def test_apply_manifest_invalid_yaml(mcp: FastMCP) -> None:
    tool = await _get_tool(mcp, "apply_manifest")
    with pytest.raises(RuntimeError, match="manifest contains no valid Kubernetes resources"):
        tool.fn(cluster_id="cl-01", manifest_yaml="foo: bar", namespace="default")
    with pytest.raises(RuntimeError, match="invalid manifest YAML/JSON"):
        tool.fn(cluster_id="cl-01", manifest_yaml="foo: [unclosed list", namespace="default")


def test_apply_manifest_is_tagged_mutation(mcp: FastMCP) -> None:
    tools = asyncio.run(mcp._local_provider.list_tools())
    tool = next(t for t in tools if t.name == "apply_manifest")
    assert "mutation" in tool.tags
    assert "[MUTATION]" in tool.description
