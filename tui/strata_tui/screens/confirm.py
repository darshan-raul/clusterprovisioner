"""Confirmation modal for mutating operations.

Pops when a command or agent wishes to perform a mutation (delete, apply, exec).
The user presses `y` to allow, `n` (or `Esc`) to deny.
"""

from __future__ import annotations

from textual.app import ComposeResult
from textual.binding import Binding
from textual.containers import Vertical
from textual.screen import ModalScreen
from textual.widgets import Label


class ConfirmScreen(ModalScreen[bool]):
    """A modal that asks the user to allow/deny a mutating Kubernetes action."""

    BINDINGS = [
        Binding("y", "allow", "Allow", show=True),
        Binding("n", "deny", "Deny", show=True),
        Binding("escape", "deny", "Deny", show=False),
    ]

    CSS = """
    ConfirmScreen {
        align: center middle;
    }
    #confirm-dialog {
        width: 80%;
        max-width: 90;
        height: auto;
        border: thick $warning;
        background: $surface;
        padding: 1 2;
    }
    #confirm-title {
        color: $warning;
        text-style: bold;
        margin-bottom: 1;
    }
    #confirm-action {
        color: $text;
        text-style: bold;
    }
    #confirm-target {
        color: $accent;
        margin-bottom: 1;
    }
    #confirm-details {
        color: $text-muted;
        background: $boost;
        padding: 0 1;
        margin-bottom: 1;
    }
    #confirm-warning {
        color: $error;
        text-style: bold;
        margin-bottom: 1;
    }
    #confirm-help {
        color: $text-muted;
    }
    """

    def __init__(
        self,
        action: str,
        target: str,
        cluster: str = "",
        details: str | None = None,
        warning: str | None = None,
    ) -> None:
        super().__init__()
        self._action = action
        self._target = target
        self._cluster = cluster
        self._details = details
        self._warning = warning

    def compose(self) -> ComposeResult:
        with Vertical(id="confirm-dialog"):
            yield Label("⚠  Confirmation Required (MUTATION)", id="confirm-title")
            yield Label(f"Action:  [bold]{self._action}[/]", id="confirm-action")
            cluster_info = f"  (Cluster: {self._cluster})" if self._cluster else ""
            yield Label(
                f"Target:  [bold]{self._target}[/]{cluster_info}", id="confirm-target"
            )
            if self._details:
                yield Label(f"Details: {self._details}", id="confirm-details")
            if self._warning:
                yield Label(f"⚠  {self._warning}", id="confirm-warning")
            yield Label(
                "Press [bold green]y[/] to proceed, [bold red]n[/] (or [bold]Esc[/]) to cancel.",
                id="confirm-help",
            )

    def action_allow(self) -> None:
        self.dismiss(True)

    def action_deny(self) -> None:
        self.dismiss(False)
