"""Strata TUI command palette — kubectl-style commands.

Commands:
- ``:get pods [-n ns] [--label key=value]``
- ``:delete pod <name> [-n ns] [--grace-period N]`` (MUTATION — confirmation modal)
- ``:apply -f <manifest.yaml> [-n ns]`` (MUTATION — confirmation modal)
- ``:exec <pod> [-c container] [-n ns] -- <cmd>`` (MUTATION — confirmation modal)
- ``:ctx list`` / ``:ctx use <name>`` — manage active cluster context
- ``:login`` / ``:logout``
"""

from strata_tui.commands.apply import ApplyCommand
from strata_tui.commands.context import ContextCommand
from strata_tui.commands.delete import DeleteCommand
from strata_tui.commands.exec import ExecCommand
from strata_tui.commands.get import GetCommand

__all__ = [
    "ApplyCommand",
    "ContextCommand",
    "DeleteCommand",
    "ExecCommand",
    "GetCommand",
    "parse_command_line",
]


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