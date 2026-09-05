"""Tests for Strata TUI mutation commands and ConfirmScreen."""

from __future__ import annotations

from unittest.mock import AsyncMock

import pytest

from strata_tui.app import StrataTUIApp
from strata_tui.commands.apply import ApplyCommand
from strata_tui.commands.apply import parse as parse_apply
from strata_tui.commands.delete import DeleteCommand
from strata_tui.commands.delete import parse as parse_delete
from strata_tui.commands.exec import ExecCommand
from strata_tui.commands.exec import parse as parse_exec
from strata_tui.screens.confirm import ConfirmScreen
from strata_tui.widgets.history import MessageHistory


class _FakeCluster:
    id = "cl-test"
    name = "test-cluster"
    context = "test-context"


@pytest.fixture
def fake_app() -> StrataTUIApp:
    app = StrataTUIApp()
    app._active_cluster = _FakeCluster()
    app._client = AsyncMock()
    return app


# ── Parsing tests ──────────────────────────────────────────────────


def test_parse_delete() -> None:
    res = parse_delete(["pod", "nginx-123", "-n", "prod", "--grace-period", "10"])
    assert res is not None
    assert res.kind == "pod"
    assert res.name == "nginx-123"
    assert res.namespace == "prod"
    assert res.grace_period_seconds == 10

    # Incomplete arguments
    assert parse_delete(["pod"]) is None


def test_parse_apply() -> None:
    res = parse_apply(["-f", "deploy.yaml", "-n", "staging"])
    assert res is not None
    assert res.filepath == "deploy.yaml"
    assert res.namespace == "staging"

    assert parse_apply([]) is None


def test_parse_exec() -> None:
    res = parse_exec(["my-pod", "-c", "app", "-n", "kube-system", "--", "ls", "-la"])
    assert res is not None
    assert res.pod == "my-pod"
    assert res.container == "app"
    assert res.namespace == "kube-system"
    assert res.command == "ls -la"

    assert parse_exec([]) is None


# ── ConfirmScreen UI tests ─────────────────────────────────────────


@pytest.mark.asyncio
async def test_confirm_screen_allow() -> None:
    from textual import work

    screen = ConfirmScreen(
        action="delete pod",
        target="nginx",
        cluster="prod",
        warning="Caution!",
    )
    result = None

    class ScreenApp(StrataTUIApp):
        @work
        async def ask(self):
            nonlocal result
            result = await self.push_screen_wait(screen)

    app = ScreenApp()
    async with app.run_test() as pilot:
        app.ask()
        await pilot.pause()
        await pilot.press("y")
        await pilot.pause()
        assert result is True


@pytest.mark.asyncio
async def test_confirm_screen_deny() -> None:
    from textual import work

    screen = ConfirmScreen(
        action="delete pod",
        target="nginx",
        cluster="prod",
    )
    result = None

    class ScreenApp(StrataTUIApp):
        @work
        async def ask(self):
            nonlocal result
            result = await self.push_screen_wait(screen)

    app = ScreenApp()
    async with app.run_test() as pilot:
        app.ask()
        await pilot.pause()
        await pilot.press("n")
        await pilot.pause()
        assert result is False


# ── Command execution tests with mocked confirmation ──────────────


@pytest.mark.asyncio
async def test_delete_command_allowed(fake_app: StrataTUIApp, monkeypatch) -> None:
    async def fake_push_wait(self, screen):
        return True

    monkeypatch.setattr(fake_app, "push_screen_wait", fake_push_wait.__get__(fake_app))
    fake_app._client.delete_pod.return_value = {"status": "deleted"}

    async with fake_app.run_test() as pilot:
        await pilot.pause()
        cmd = DeleteCommand(fake_app)
        await cmd.execute(["pod", "nginx", "-n", "default"])
        fake_app._client.delete_pod.assert_called_once_with(
            "cl-test", "nginx", namespace="default", grace_period_seconds=None
        )
        history = fake_app.query_one("#history", MessageHistory)
        assert any("deleted successfully" in str(line) for line in history.lines)


@pytest.mark.asyncio
async def test_delete_command_denied(fake_app: StrataTUIApp, monkeypatch) -> None:
    async def fake_push_wait(self, screen):
        return False

    monkeypatch.setattr(fake_app, "push_screen_wait", fake_push_wait.__get__(fake_app))

    async with fake_app.run_test() as pilot:
        await pilot.pause()
        cmd = DeleteCommand(fake_app)
        await cmd.execute(["pod", "nginx", "-n", "default"])
        fake_app._client.delete_pod.assert_not_called()
        history = fake_app.query_one("#history", MessageHistory)
        assert any("cancelled" in str(line) for line in history.lines)


@pytest.mark.asyncio
async def test_apply_command_allowed(fake_app: StrataTUIApp, tmp_path, monkeypatch) -> None:
    manifest_file = tmp_path / "cm.yaml"
    manifest_file.write_text("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: my-cm")

    async def fake_push_wait(self, screen):
        return True

    monkeypatch.setattr(fake_app, "push_screen_wait", fake_push_wait.__get__(fake_app))
    fake_app._client.apply_manifest.return_value = {"status": "applied", "count": 1}

    async with fake_app.run_test() as pilot:
        await pilot.pause()
        cmd = ApplyCommand(fake_app)
        await cmd.execute(["-f", str(manifest_file), "-n", "default"])
        fake_app._client.apply_manifest.assert_called_once()
        history = fake_app.query_one("#history", MessageHistory)
        assert any("Successfully applied 1 resource(s)" in str(line) for line in history.lines)


@pytest.mark.asyncio
async def test_exec_command_allowed(fake_app: StrataTUIApp, monkeypatch) -> None:
    async def fake_push_wait(self, screen):
        return True

    monkeypatch.setattr(fake_app, "push_screen_wait", fake_push_wait.__get__(fake_app))
    fake_app._client.exec_command.return_value = {"status": "completed", "output": "test output\n"}

    async with fake_app.run_test() as pilot:
        await pilot.pause()
        cmd = ExecCommand(fake_app)
        await cmd.execute(["web-pod", "-c", "main", "--", "date"])
        fake_app._client.exec_command.assert_called_once_with(
            "cl-test", "web-pod", "date", namespace="default", container="main"
        )
        history = fake_app.query_one("#history", MessageHistory)
        assert any("test output" in str(line) for line in history.lines)
