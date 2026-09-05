"""``:exec`` command — execute a command in a pod with confirmation."""

from __future__ import annotations

from dataclasses import dataclass
from typing import TYPE_CHECKING

from strata_tui.screens.confirm import ConfirmScreen

if TYPE_CHECKING:
    from strata_tui.app import StrataTUIApp


@dataclass
class ParsedExec:
    pod: str
    command: str
    namespace: str = "default"
    container: str | None = None


def parse(args: list[str]) -> ParsedExec | None:
    """Parse ``:exec <pod> [-c container] [-n ns] -- <command>``."""
    if not args:
        return None

    pod: str | None = None
    namespace = "default"
    container: str | None = None
    command_parts: list[str] = []

    if "--" in args:
        dash_idx = args.index("--")
        pre_dash = args[:dash_idx]
        command_parts = args[dash_idx + 1 :]
    else:
        pre_dash = args
        command_parts = []

    i = 0
    while i < len(pre_dash):
        a = pre_dash[i]
        if a in ("-n", "--namespace") and i + 1 < len(pre_dash):
            namespace = pre_dash[i + 1]
            i += 2
            continue
        if a in ("-c", "--container") and i + 1 < len(pre_dash):
            container = pre_dash[i + 1]
            i += 2
            continue
        if pod is None and not a.startswith("-"):
            pod = a
            i += 1
            continue
        # If no '--' was provided, trailing tokens are treated as the command.
        command_parts.append(a)
        i += 1

    if not pod or not command_parts:
        return None

    return ParsedExec(
        pod=pod,
        command=" ".join(command_parts),
        namespace=namespace,
        container=container,
    )


class ExecCommand:
    """Executes ``:exec`` against the active cluster context after confirmation."""

    def __init__(self, app: StrataTUIApp) -> None:
        self.app = app

    async def execute(self, args: list[str]) -> None:
        history = self.app.history
        parsed = parse(args)
        if not parsed:
            history.append_error("usage: :exec <pod> [-c <container>] [-n <namespace>] -- <command>")
            return

        if not self.app.client:
            history.append_error("not logged in — run :login first")
            return

        cluster = self.app.active_cluster
        if cluster is None:
            history.append_error("no active cluster — run :ctx list, then :ctx use <id>")
            return

        target = f"{parsed.pod}" + (f":{parsed.container}" if parsed.container else "")
        screen = ConfirmScreen(
            action="exec command",
            target=f"{target} (ns: {parsed.namespace})",
            cluster=cluster.name,
            details=f"Command: {parsed.command}",
            warning="Executing commands directly in a container may alter runtime state.",
        )
        confirmed = await self.app.push_screen_wait(screen)
        if not confirmed:
            history.append_info("exec cancelled")
            return

        history.append_info(f"Executing in {target!r}…")
        try:
            result = await self.app.client.exec_command(
                cluster.id,
                parsed.pod,
                parsed.command,
                namespace=parsed.namespace,
                container=parsed.container,
            )
        except Exception as exc:  # noqa: BLE001
            history.append_error(f"exec failed: {exc}")
            return

        output = result.get("output", "")
        history.append_ai(f"Output from {target!r}:\n{output}" if output else f"Command completed in {target!r} (no output).")
