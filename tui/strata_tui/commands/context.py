"""``:ctx`` command — manage the active cluster context."""

from __future__ import annotations

import os
from pathlib import Path
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
        if sub == "add":
            if len(args) < 3:
                history.append_error("usage: :ctx add <name> <kubeconfig-file-path> [context]")
                return
            ctx_override = args[3] if len(args) > 3 else None
            await self._add(args[1], args[2], ctx_override, history)
            return
        if sub in ("delete", "rm"):
            if len(args) < 2:
                history.append_error("usage: :ctx delete <cluster-id-or-name>")
                return
            await self._delete(args[1], history)
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
        active_id = self.app.active_cluster.id if self.app.active_cluster else None
        rows = [
            f" {'*' if c.id == active_id else ' '} {c.id:<20}  {c.name:<30}  ({c.context})"
            for c in clusters
        ]
        header = f"   {'ID':<20}  {'NAME':<30}  (CONTEXT)"
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

    async def _add(self, name: str, kubeconfig_path: str, context: str | None, history) -> None:
        if not self.app.client:
            history.append_error("not logged in")
            return
        p = Path(os.path.expanduser(kubeconfig_path))
        if not p.is_file():
            history.append_error(f"kubeconfig file not found: {kubeconfig_path}")
            return
        try:
            content = p.read_text(encoding="utf-8")
        except Exception as exc:  # noqa: BLE001
            history.append_error(f"failed to read kubeconfig: {exc}")
            return

        try:
            cluster = await self.app.client.create_cluster(name, content, context=context)
            self.app.set_active_cluster(cluster)
            history.append_ai(
                f"registered cluster {cluster.id} ({cluster.name!r}) with context {cluster.context!r}\n"
                f"credentials encrypted at rest via AES-256-GCM. Set as active cluster."
            )
        except Exception as exc:  # noqa: BLE001
            history.append_error(f"create_cluster failed: {exc}")

    async def _delete(self, target: str, history) -> None:
        if not self.app.client:
            history.append_error("not logged in")
            return
        try:
            clusters = await self.app.client.list_clusters()
        except Exception as exc:  # noqa: BLE001
            history.append_error(f"list_clusters failed: {exc}")
            return
        match = next(
            (c for c in clusters if c.id == target or c.name == target),
            None,
        )
        if match is None:
            history.append_error(f"no cluster with id or name {target!r}")
            return
        try:
            await self.app.client.delete_cluster(match.id)
            if self.app.active_cluster and self.app.active_cluster.id == match.id:
                self.app.set_active_cluster(None)
            history.append_ai(f"deleted cluster {match.id} ({match.name!r})")
        except Exception as exc:  # noqa: BLE001
            history.append_error(f"delete_cluster failed: {exc}")