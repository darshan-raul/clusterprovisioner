"""LangGraph agent for Strata TUI.

Provides an agent graph with human-in-the-loop ``interrupt()`` for
mutating tools (delete_pod, apply_manifest, exec_command).
"""

from __future__ import annotations

import json
from collections.abc import Callable
from typing import Annotated, Any

from langchain_core.messages import AIMessage, BaseMessage, ToolMessage
from langchain_core.tools import tool
from langgraph.checkpoint.memory import MemorySaver
from langgraph.graph import END, START, StateGraph
from langgraph.graph.message import add_messages
from langgraph.types import interrupt
from typing_extensions import TypedDict

from strata_tui.api.client import StrataClient

MUTATING_TOOLS = {"delete_pod", "apply_manifest", "exec_command"}

SYSTEM_PROMPT = """\
You are Strata, an AI co-pilot for managing Kubernetes clusters.
You can query and mutate Kubernetes resources using your tools:
- retrieve_docs (read-only): retrieve relevant cluster state, troubleshooting info, or operational runbooks
- list_pods (read-only): inspect running pods
- delete_pod (MUTATION): delete a pod
- apply_manifest (MUTATION): apply YAML/JSON manifest
- exec_command (MUTATION): run a command inside a pod container

When performing mutating actions, be precise. The system will prompt the user
for explicit confirmation before executing any mutating tool.
"""


class AgentState(TypedDict):
    messages: Annotated[list[BaseMessage], add_messages]


def build_strata_tools(
    client_provider: Callable[[], StrataClient | None],
    cluster_id_provider: Callable[[], str | None],
) -> list[Any]:
    """Create tool bindings that delegate to the active StrataClient."""

    @tool
    async def list_pods(
        namespace: str = "default",
        label_selector: str | None = None,
    ) -> str:
        """List pods in the active Kubernetes cluster."""
        client = client_provider()
        cluster_id = cluster_id_provider()
        if not client or not cluster_id:
            return "Error: not logged in or no active cluster selected"
        try:
            pods = await client.list_pods(
                cluster_id, namespace=namespace, label_selector=label_selector
            )
            return json.dumps(pods)
        except Exception as exc:  # noqa: BLE001
            return f"Error listing pods: {exc}"

    @tool
    async def delete_pod(
        name: str,
        namespace: str = "default",
        grace_period_seconds: int | None = None,
    ) -> str:
        """Delete a pod in the active Kubernetes cluster [MUTATION]."""
        client = client_provider()
        cluster_id = cluster_id_provider()
        if not client or not cluster_id:
            return "Error: not logged in or no active cluster selected"
        try:
            res = await client.delete_pod(
                cluster_id,
                name,
                namespace=namespace,
                grace_period_seconds=grace_period_seconds,
            )
            return json.dumps(res)
        except Exception as exc:  # noqa: BLE001
            return f"Error deleting pod: {exc}"

    @tool
    async def apply_manifest(manifest_yaml: str, namespace: str = "default") -> str:
        """Apply a YAML or JSON manifest to the active Kubernetes cluster [MUTATION]."""
        client = client_provider()
        cluster_id = cluster_id_provider()
        if not client or not cluster_id:
            return "Error: not logged in or no active cluster selected"
        try:
            res = await client.apply_manifest(
                cluster_id, manifest_yaml, namespace=namespace
            )
            return json.dumps(res)
        except Exception as exc:  # noqa: BLE001
            return f"Error applying manifest: {exc}"

    @tool
    async def exec_command(
        pod: str,
        command: str,
        namespace: str = "default",
        container: str | None = None,
    ) -> str:
        """Execute a command inside a pod container [MUTATION]."""
        client = client_provider()
        cluster_id = cluster_id_provider()
        if not client or not cluster_id:
            return "Error: not logged in or no active cluster selected"
        try:
            res = await client.exec_command(
                cluster_id, pod, command, namespace=namespace, container=container
            )
            return json.dumps(res)
        except Exception as exc:  # noqa: BLE001
            return f"Error executing command: {exc}"

    @tool
    async def retrieve_docs(
        query: str,
        collection: str = "clusters",
        top_k: int = 5,
    ) -> str:
        """Retrieve relevant background knowledge and context chunks from the Strata RAG engine.

        Use this tool when you need:
        - Diagnostic information or knowledge about clusters, pods, or recent audit events (collection='clusters')
        - Operational guides, architecture explanations, or runbooks (collection='docs')

        Args:
            query: Specific question or keywords to search for.
            collection: 'clusters' for live cluster/pod state, or 'docs' for platform runbooks and guides.
            top_k: Number of relevant chunks to retrieve (default 5).
        """
        client = client_provider()
        if not client:
            return "Error: not logged in"
        try:
            chunks = await client.retrieve(query=query, collection=collection, top_k=top_k)
            return json.dumps(chunks)
        except Exception as exc:  # noqa: BLE001
            return f"Error retrieving docs: {exc}"

    return [retrieve_docs, list_pods, delete_pod, apply_manifest, exec_command]


def should_retrieve(text: str) -> bool:
    """Return True if the text suggests a retrieval or knowledge lookup."""
    lower = text.lower()
    question_words = {"how", "what", "why", "when", "where", "explain", "show", "describe", "troubleshoot"}
    tokens = set(lower.split())
    if tokens & question_words:
        return True
    keywords = ["docs", "runbook", "guide", "status", "upgrade", "architecture", "failing", "error", "crash"]
    return any(k in lower for k in keywords)


def create_agent_graph(llm: Any, tools: list[Any], checkpointer: Any | None = None):
    """Compile the LangGraph agent StateGraph with human-in-the-loop interrupts."""
    tools_by_name = {t.name: t for t in tools}
    try:
        llm_with_tools = llm.bind_tools(tools)
    except (NotImplementedError, AttributeError):
        llm_with_tools = llm

    async def model_node(state: AgentState) -> dict[str, Any]:
        response = await llm_with_tools.ainvoke(state["messages"])
        return {"messages": [response]}

    async def tool_node(state: AgentState) -> dict[str, Any]:
        last = state["messages"][-1]
        tool_messages: list[ToolMessage] = []
        if not isinstance(last, AIMessage) or not last.tool_calls:
            return {"messages": []}

        for call in last.tool_calls:
            name = call["name"]
            args = call["args"]
            tool_id = call["id"]

            if name in MUTATING_TOOLS:
                # Human-in-the-loop interrupt
                confirmation = interrupt({
                    "type": "mutation_confirmation",
                    "tool": name,
                    "args": args,
                })
                if not confirmation:
                    tool_messages.append(
                        ToolMessage(
                            content="User cancelled or denied this mutating action.",
                            tool_call_id=tool_id,
                        )
                    )
                    continue

            fn = tools_by_name.get(name)
            if not fn:
                tool_messages.append(
                    ToolMessage(
                        content=f"Error: unknown tool {name!r}",
                        tool_call_id=tool_id,
                    )
                )
                continue

            try:
                result = await fn.ainvoke(args)
                tool_messages.append(
                    ToolMessage(
                        content=str(result),
                        tool_call_id=tool_id,
                    )
                )
            except Exception as exc:  # noqa: BLE001
                tool_messages.append(
                    ToolMessage(
                        content=f"Tool error: {exc}",
                        tool_call_id=tool_id,
                    )
                )

        return {"messages": tool_messages}

    def should_continue(state: AgentState) -> str:
        last = state["messages"][-1]
        if isinstance(last, AIMessage) and last.tool_calls:
            return "tools"
        return END

    workflow = StateGraph(AgentState)
    workflow.add_node("agent", model_node)
    workflow.add_node("tools", tool_node)

    workflow.add_edge(START, "agent")
    workflow.add_conditional_edges(
        "agent", should_continue, {"tools": "tools", END: END}
    )
    workflow.add_edge("tools", "agent")

    cp = checkpointer if checkpointer is not None else MemorySaver()
    return workflow.compile(checkpointer=cp)
