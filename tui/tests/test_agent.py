"""Tests for LangGraph agent human-in-the-loop interrupts."""

from __future__ import annotations

from unittest.mock import AsyncMock, MagicMock

import pytest
from langchain_core.messages import AIMessage, HumanMessage
from langgraph.types import Command

from strata_tui.agent import build_strata_tools, create_agent_graph


@pytest.fixture
def mock_client() -> AsyncMock:
    client = AsyncMock()
    client.list_pods.return_value = [{"name": "p1", "namespace": "default"}]
    client.delete_pod.return_value = {"status": "deleted", "name": "p1"}
    return client


@pytest.fixture
def agent_tools(mock_client: AsyncMock):
    return build_strata_tools(lambda: mock_client, lambda: "cl-test")


@pytest.mark.asyncio
async def test_read_only_tool_executes_without_interrupt(
    mock_client: AsyncMock, agent_tools
) -> None:
    mock_llm = MagicMock()
    mock_bound = MagicMock()
    mock_llm.bind_tools.return_value = mock_bound

    msg1 = AIMessage(
        content="",
        tool_calls=[{"name": "list_pods", "args": {"namespace": "default"}, "id": "c1"}],
    )
    msg2 = AIMessage(content="You have 1 pod running.")
    mock_bound.ainvoke = AsyncMock(side_effect=[msg1, msg2])

    graph = create_agent_graph(mock_llm, agent_tools)
    cfg = {"configurable": {"thread_id": "test-ro"}}

    result = await graph.ainvoke({"messages": [HumanMessage(content="list pods")]}, cfg)
    state = await graph.aget_state(cfg)
    assert not state.next  # Reached END without interrupts
    assert result["messages"][-1].content == "You have 1 pod running."
    mock_client.list_pods.assert_called_once_with("cl-test", namespace="default", label_selector=None)


@pytest.mark.asyncio
async def test_mutating_tool_interrupts_and_allows(
    mock_client: AsyncMock, agent_tools
) -> None:
    mock_llm = MagicMock()
    mock_bound = MagicMock()
    mock_llm.bind_tools.return_value = mock_bound

    msg1 = AIMessage(
        content="",
        tool_calls=[{"name": "delete_pod", "args": {"name": "p1"}, "id": "c2"}],
    )
    msg2 = AIMessage(content="Pod p1 deleted.")
    mock_bound.ainvoke = AsyncMock(side_effect=[msg1, msg2])

    graph = create_agent_graph(mock_llm, agent_tools)
    cfg = {"configurable": {"thread_id": "test-mut-allow"}}

    await graph.ainvoke({"messages": [HumanMessage(content="delete p1")]}, cfg)
    state = await graph.aget_state(cfg)
    assert state.next == ("tools",)
    assert len(state.tasks[0].interrupts) == 1
    val = state.tasks[0].interrupts[0].value
    assert val["tool"] == "delete_pod"
    assert val["args"] == {"name": "p1"}

    # Resume with approval
    result = await graph.ainvoke(Command(resume=True), cfg)
    state = await graph.aget_state(cfg)
    assert not state.next
    assert result["messages"][-1].content == "Pod p1 deleted."
    mock_client.delete_pod.assert_called_once()


@pytest.mark.asyncio
async def test_mutating_tool_interrupts_and_denies(
    mock_client: AsyncMock, agent_tools
) -> None:
    mock_llm = MagicMock()
    mock_bound = MagicMock()
    mock_llm.bind_tools.return_value = mock_bound

    msg1 = AIMessage(
        content="",
        tool_calls=[{"name": "delete_pod", "args": {"name": "p1"}, "id": "c3"}],
    )
    msg2 = AIMessage(content="Action was cancelled.")
    mock_bound.ainvoke = AsyncMock(side_effect=[msg1, msg2])

    graph = create_agent_graph(mock_llm, agent_tools)
    cfg = {"configurable": {"thread_id": "test-mut-deny"}}

    await graph.ainvoke({"messages": [HumanMessage(content="delete p1")]}, cfg)
    state = await graph.aget_state(cfg)
    assert state.next == ("tools",)

    # Resume with denial
    result = await graph.ainvoke(Command(resume=False), cfg)
    state = await graph.aget_state(cfg)
    assert not state.next
    assert result["messages"][-1].content == "Action was cancelled."
    mock_client.delete_pod.assert_not_called()
