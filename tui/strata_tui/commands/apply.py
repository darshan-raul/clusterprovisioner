"""``:apply`` command — apply a Kubernetes manifest with confirmation."""

from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path
from typing import TYPE_CHECKING

from strata_tui.screens.confirm import ConfirmScreen

if TYPE_CHECKING:
    from strata_tui.app import StrataTUIApp


@dataclass
class ParsedApply:
    filepath: str
    namespace: str


def parse(args: list[str]) -> ParsedApply | None:
    """Parse ``:apply -f <file> [-n ns]`` or ``:apply <file> [-n ns]``."""
    if not args:
        return None

    filepath: str | None = None
    namespace = "default"

    i = 0
    while i < len(args):
        a = args[i]
        if a in ("-f", "--filename") and i + 1 < len(args):
            filepath = args[i + 1]
            i += 2
            continue
        if a in ("-n", "--namespace") and i + 1 < len(args):
            namespace = args[i + 1]
            i += 2
            continue
        if filepath is None and not a.startswith("-"):
            filepath = a
            i += 1
            continue
        i += 1

    if not filepath:
        return None

    return ParsedApply(filepath=filepath, namespace=namespace)


class ApplyCommand:
    """Executes ``:apply`` against the active cluster context after confirmation."""

    def __init__(self, app: StrataTUIApp) -> None:
        self.app = app

    async def execute(self, args: list[str]) -> None:
        history = self.app.history
        parsed = parse(args)
        if not parsed:
            history.append_error("usage: :apply -f <path/to/manifest.yaml> [-n <namespace>]")
            return

        file_path = Path(parsed.filepath).expanduser().resolve()
        if not file_path.is_file():
            history.append_error(f"file not found: {parsed.filepath}")
            return

        try:
            content = file_path.read_text(encoding="utf-8")
        except Exception as exc:  # noqa: BLE001
            history.append_error(f"failed to read file: {exc}")
            return

        if not self.app.client:
            history.append_error("not logged in — run :login first")
            return

        cluster = self.app.active_cluster
        if cluster is None:
            history.append_error("no active cluster — run :ctx list, then :ctx use <id>")
            return

        screen = ConfirmScreen(
            action="apply manifest",
            target=str(file_path.name),
            cluster=cluster.name,
            details=f"File: {file_path} ({len(content)} bytes)",
            warning="Applying manifests creates or mutates resources in the target cluster.",
        )
        confirmed = await self.app.push_screen_wait(screen)
        if not confirmed:
            history.append_info("apply cancelled")
            return

        history.append_info(f"Applying manifest from {file_path.name!r}…")
        try:
            result = await self.app.client.apply_manifest(
                cluster.id,
                content,
                namespace=parsed.namespace,
            )
        except Exception as exc:  # noqa: BLE001
            history.append_error(f"apply failed: {exc}")
            return

        count = result.get("count", len(result.get("applied", [])))
        history.append_ai(f"Successfully applied {count} resource(s) from {file_path.name!r}")
