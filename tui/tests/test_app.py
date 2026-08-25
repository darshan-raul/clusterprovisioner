"""Tests for the Strata TUI App.

The chat surface uses a FakeListChatModel (no network). The
backend client is mocked via respx.
"""

from __future__ import annotations

import pytest
from langchain_core.language_models.fake_chat_models import FakeListChatModel

from strata_tui.app import StrataTUIApp
from strata_tui.widgets.history import MessageHistory
from strata_tui.widgets.resource_table import ResourceTable
from strata_tui.widgets.status_bar import StatusBar


@pytest.fixture
def fake_llm(monkeypatch):
    """Patch ChatOpenAI to return canned responses."""

    def _factory(**_kwargs):
        return FakeListChatModel(responses=["hello from strata"])

    monkeypatch.setattr("strata_tui.app.ChatOpenAI", _factory)


@pytest.mark.asyncio
async def test_app_starts_and_sends_chat(fake_llm) -> None:
    """End-to-end smoke: type, send, see the AI reply."""
    app = StrataTUIApp()
    async with app.run_test() as pilot:
        await pilot.pause()
        await pilot.click("#input")
        await pilot.press(*"hi there")
        await pilot.press("enter")
        await pilot.pause()
        await pilot.pause()
        history = app.query_one("#history", MessageHistory)
        text = history.lines[-1] if history.lines else ""
        assert "hello from strata" in str(text)


@pytest.mark.asyncio
async def test_help_command_shows_help(fake_llm) -> None:
    """``:help`` prints the command list into the history."""
    app = StrataTUIApp()
    async with app.run_test() as pilot:
        await pilot.pause()
        await pilot.click("#input")
        await pilot.press(*":help")
        await pilot.press("enter")
        await pilot.pause()
        history = app.query_one("#history", MessageHistory)
        all_text = "\n".join(str(line) for line in history.lines)
        assert ":login" in all_text
        assert ":get pods" in all_text


@pytest.mark.asyncio
async def test_status_bar_mentions_user(fake_llm) -> None:
    """The status bar starts with the placeholder user before login."""
    app = StrataTUIApp()
    async with app.run_test() as pilot:
        await pilot.pause()
        status = app.query_one("#status", StatusBar)
        text = str(status.render())
        assert "user:" in text
        assert "cluster:" in text


@pytest.mark.asyncio
async def test_get_command_routes_to_handler(fake_llm, monkeypatch) -> None:
    """``:get pods`` should call the GetCommand (which calls the client)."""
    seen: list[tuple[str, list[str]]] = []

    async def fake_execute(self, args):
        seen.append(("get", args))
        # Also exercise the resource table code path
        rt = self.app.resource_table
        rt.replace([{"name": "p1", "namespace": "default", "phase": "Running"}])

    monkeypatch.setattr("strata_tui.commands.get.GetCommand.execute", fake_execute)

    app = StrataTUIApp()
    async with app.run_test() as pilot:
        await pilot.pause()
        # Bypass the login requirement by injecting a client directly.
        app._client = _FakeClient()
        app._active_cluster = _FakeCluster()
        await pilot.click("#input")
        await pilot.press(*":get pods")
        await pilot.press("enter")
        await pilot.pause()
        assert ("get", ["pods"]) in seen
        rt = app.query_one("#resource-table", ResourceTable)
        assert len(rt._rows) == 1


@pytest.mark.asyncio
async def test_ctx_list_routes_to_handler(fake_llm, monkeypatch) -> None:
    """``:ctx list`` should call the ContextCommand."""

    seen: list[tuple[str, list[str]]] = []

    async def fake_execute(self, args):
        seen.append(("ctx", args))

    monkeypatch.setattr("strata_tui.commands.context.ContextCommand.execute", fake_execute)

    app = StrataTUIApp()
    async with app.run_test() as pilot:
        await pilot.pause()
        await pilot.click("#input")
        await pilot.press(*":ctx list")
        await pilot.press("enter")
        await pilot.pause()
        assert ("ctx", ["list"]) in seen


@pytest.mark.asyncio
async def test_unknown_command_writes_error(fake_llm) -> None:
    """``:foobar`` writes a 'unknown command' error to history."""
    app = StrataTUIApp()
    async with app.run_test() as pilot:
        await pilot.pause()
        await pilot.click("#input")
        await pilot.press(*":foobar")
        await pilot.press("enter")
        await pilot.pause()
        history = app.query_one("#history", MessageHistory)
        all_text = "\n".join(str(line) for line in history.lines)
        assert "unknown command" in all_text


class _FakeCluster:
    id = "cl-test"
    name = "demo"
    context = "demo"


class _FakeClient:
    pass