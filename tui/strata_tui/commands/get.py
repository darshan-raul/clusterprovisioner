"""``:get`` command — fetch pods from the active cluster."""

from __future__ import annotations

from dataclasses import dataclass
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from strata_tui.app import StrataTUIApp


@dataclass
class ParsedGet:
    kind: str
    namespace: str | None
    label_selector: str | None


def parse(args: list[str]) -> ParsedGet:
    """Parse ``:get pods [-n ns] [--label k=v]``."""
    kind = args[0] if args else "pods"
    namespace: str | None = None
    label_selector: str | None = None
    i = 1
    while i < len(args):
        a = args[i]
        if a in ("-n", "--namespace") and i + 1 < len(args):
            namespace = args[i + 1]
            i += 2
            continue
        if a in ("-l", "--label") and i + 1 < len(args):
            label_selector = args[i + 1]
            i += 2
            continue
        i += 1
    return ParsedGet(kind=kind, namespace=namespace, label_selector=label_selector)


SUPPORTED_KINDS = {"pods"}


class GetCommand:
    """Executes ``:get`` against the active cluster context."""

    def __init__(self, app: StrataTUIApp) -> None:
        self.app = app

    async def execute(self, args: list[str]) -> None:
        history = self.app.history
        parsed = parse(args)
        if parsed.kind not in SUPPORTED_KINDS:
            history.append_error(
                f"unsupported resource kind {parsed.kind!r}; "
                f"Phase 1 supports: {', '.join(sorted(SUPPORTED_KINDS))}"
            )
            return
        if not self.app.client:
            history.append_error("not logged in — run :login first")
            return
        cluster = self.app.active_cluster
        if cluster is None:
            history.append_error(
                "no active cluster — run :ctx list, then :ctx use <id>"
            )
            return
        history.append_info(
            f"Fetching {parsed.kind} from cluster {cluster.name!r}…"
        )
        try:
            pods = await self.app.client.list_pods(
                cluster.id,
                namespace=parsed.namespace,
                label_selector=parsed.label_selector,
            )
        except Exception as exc:  # noqa: BLE001
            history.append_error(f"list_pods failed: {exc}")
            return
        self.app.resource_table.replace(pods)
        history.append_ai(
            f"{len(pods)} pod{'s' if len(pods) != 1 else ''} in {cluster.name!r}"
        )