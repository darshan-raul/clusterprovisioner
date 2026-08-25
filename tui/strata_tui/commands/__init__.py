"""Strata TUI command palette — kubectl-style commands.

Phase 1 commands:

- ``:get pods [namespace] [--label key=value]``
- ``:describe <kind> <name>`` (stub — returns a placeholder; full
  implementation lands when the MCP k8s server adds
  ``describe_*`` tools in Phase 3.)
- ``:ctx list`` / ``:ctx use <name>`` — manage the active cluster
  context.

The :get command renders results into a ``ResourceTable``
widget. The :ctx command updates the App's active context and
re-renders.
"""

from strata_tui.commands.context import ContextCommand
from strata_tui.commands.get import GetCommand

__all__ = ["ContextCommand", "GetCommand"]


def parse_command_line(line: str) -> tuple[str, list[str]]:
    """Strip the leading ``:`` and split into ``(name, args)``.

    Returns ``("", [])`` for non-commands so the caller can
    decide what to do (chat, help, etc.).
    """
    stripped = line.strip()
    if not stripped.startswith(":"):
        return "", []
    parts = stripped[1:].split()
    if not parts:
        return "", []
    return parts[0].lower(), parts[1:]