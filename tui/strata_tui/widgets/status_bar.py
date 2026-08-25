"""Status bar — shows model, cluster, login state."""

from __future__ import annotations

from rich.text import Text
from textual.widget import Widget


class StatusBar(Widget):
    """A single-line status bar at the bottom of the TUI."""

    def __init__(self, model_name: str = "strata", **kwargs) -> None:
        super().__init__(**kwargs)
        self._model_name = model_name
        self._cluster = "-"
        self._user = "-"
        self._busy = False

    def set_model(self, model_name: str) -> None:
        self._model_name = model_name
        self.refresh()

    def set_cluster(self, cluster: str | None) -> None:
        self._cluster = cluster or "-"
        self.refresh()

    def set_user(self, user: str | None) -> None:
        self._user = user or "-"
        self.refresh()

    def set_busy(self, busy: bool) -> None:
        self._busy = busy
        self.refresh()

    def render(self) -> Text:
        state = "thinking…" if self._busy else "ready"
        text = (
            f"user: {self._user} | cluster: {self._cluster} | {state} "
            "| Ctrl+C quit | Ctrl+L clear | :login | :get pods | :ctx list"
        )
        return Text(text)