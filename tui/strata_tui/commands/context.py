"""``:ctx`` command — manage the active cluster context."""

from __future__ import annotations

from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from strata_tui.app import StrataTUIApp


class ContextCommand:
    def __init__(self, app: StrataTUIApp) -> None:
        self.app = app

    async def execute(self, args: list[str]) -> None:
        history = self.app.history
        if not args:
            await self._show_current(history)
            return
        sub = args[0].lower()
        if sub in ("list", "ls"):
            await self._list(history)
            return
        if sub == "use":
            if len(args) < 2:
                history.append_error("usage: :ctx use <cluster-id-or-name>")
                return
            await self._use(args[1], history)
            return
        history.append_error(f"unknown :ctx subcommand {sub!r}")

    async def _show_current(self, history) -> None:
        c = self.app.active_cluster
        if c is None:
            history.append_info("no active cluster; run :ctx list")
            return
        history.append_ai(f"active cluster: {c.id} ({c.name!r})")

    async def _list(self, history) -> None:
        if not self.app.client:
            history.append_error("not logged in")
            return
        try:
            clusters = await self.app.client.list_clusters()
        except Exception as exc:  # noqa: BLE001
            history.append_error(f"list_clusters failed: {exc}")
            return
        if not clusters:
            history.append_info("no clusters registered for this user")
            return
        rows = [f"  {c.id:<20}  {c.name:<30}  ({c.context})" for c in clusters]
        header = f"  {'ID':<20}  {'NAME':<30}  (CONTEXT)"
        history.append_ai(f"{len(clusters)} cluster(s):\n{header}\n" + "\n".join(rows))

    async def _use(self, target: str, history) -> None:
        if not self.app.client:
            history.append_error("not logged in")
            return
        try:
            clusters = await self.app.client.list_clusters()
        except Exception as exc:  # noqa: BLE001
            history.append_error(f"list_clusters failed: {exc}")
            return
        # Match by id, then by name.
        match = next(
            (c for c in clusters if c.id == target or c.name == target),
            None,
        )
        if match is None:
            history.append_error(f"no cluster with id or name {target!r}")
            return
        self.app.set_active_cluster(match)
        history.append_ai(f"switched to cluster {match.id} ({match.name!r})")