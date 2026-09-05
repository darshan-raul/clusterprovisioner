"""``:delete`` command — delete a resource from the active cluster with confirmation."""

from __future__ import annotations

from dataclasses import dataclass
from typing import TYPE_CHECKING

from strata_tui.screens.confirm import ConfirmScreen

if TYPE_CHECKING:
    from strata_tui.app import StrataTUIApp


@dataclass
class ParsedDelete:
    kind: str
    name: str
    namespace: str
    grace_period_seconds: int | None = None


def parse(args: list[str]) -> ParsedDelete | None:
    """Parse ``:delete pod <name> [-n ns] [--grace-period N]``."""
    if len(args) < 2:
        return None
    kind = args[0].rstrip("s")  # "pods" -> "pod"
    name = args[1]
    namespace = "default"
    grace_period = None

    i = 2
    while i < len(args):
        a = args[i]
        if a in ("-n", "--namespace") and i + 1 < len(args):
            namespace = args[i + 1]
            i += 2
            continue
        if a == "--grace-period" and i + 1 < len(args):
            try:
                grace_period = int(args[i + 1])
            except ValueError:
                pass
            i += 2
            continue
        i += 1

    return ParsedDelete(
        kind=kind,
        name=name,
        namespace=namespace,
        grace_period_seconds=grace_period,
    )


SUPPORTED_KINDS = {"pod"}


class DeleteCommand:
    """Executes ``:delete`` against the active cluster context after confirmation."""

    def __init__(self, app: StrataTUIApp) -> None:
        self.app = app

    async def execute(self, args: list[str]) -> None:
        history = self.app.history
        parsed = parse(args)
        if not parsed:
            history.append_error("usage: :delete pod <name> [-n <namespace>] [--grace-period <sec>]")
            return

        if parsed.kind not in SUPPORTED_KINDS:
            history.append_error(
                f"unsupported resource kind {parsed.kind!r}; supported: {', '.join(sorted(SUPPORTED_KINDS))}"
            )
            return

        if not self.app.client:
            history.append_error("not logged in — run :login first")
            return

        cluster = self.app.active_cluster
        if cluster is None:
            history.append_error("no active cluster — run :ctx list, then :ctx use <id>")
            return

        screen = ConfirmScreen(
            action=f"delete {parsed.kind}",
            target=f"{parsed.name} (ns: {parsed.namespace})",
            cluster=cluster.name,
            warning="Pod deletion will immediately terminate running containers.",
        )
        confirmed = await self.app.push_screen_wait(screen)
        if not confirmed:
            history.append_info(f"deletion of pod {parsed.name!r} cancelled")
            return

        history.append_info(f"Deleting {parsed.kind} {parsed.name!r} in cluster {cluster.name!r}…")
        try:
            await self.app.client.delete_pod(
                cluster.id,
                parsed.name,
                namespace=parsed.namespace,
                grace_period_seconds=parsed.grace_period_seconds,
            )
        except Exception as exc:  # noqa: BLE001
            history.append_error(f"delete failed: {exc}")
            return

        history.append_ai(f"Pod {parsed.name!r} deleted successfully in namespace {parsed.namespace!r}")
