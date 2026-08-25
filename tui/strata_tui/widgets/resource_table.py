"""Resource table widget — renders ``:get`` results."""

from __future__ import annotations

from typing import Any

from textual.widgets import DataTable


class ResourceTable(DataTable):
    """A DataTable preconfigured for cluster-resource rows."""

    DEFAULT_CSS = """
    ResourceTable {
        height: 1fr;
    }
    """

    def __init__(self) -> None:
        super().__init__(zebra_stripes=True, header_height=1, id="resource-table")
        self._columns: list[str] = []
        self._rows: list[dict[str, Any]] = []
        self._show()

    def _show(self) -> None:
        """(Re)render the current rows into the table."""
        self.clear(columns=True)
        if not self._rows:
            self.add_column("(empty)")
            return
        for col in self._columns:
            self.add_column(col.upper(), key=col)
        for row in self._rows:
            self.add_row(*[str(row.get(c, "")) for c in self._columns], key=row.get("name"))

    def replace(self, rows: list[dict[str, Any]]) -> None:
        """Replace all rows. Infers columns from the keys of the first row."""
        self._rows = rows
        if rows:
            keys = list(rows[0].keys())
            order = ["name", "namespace", "ready", "phase", "restarts", "node", "age"]
            self._columns = [k for k in order if k in keys] + [
                k for k in keys if k not in order
            ]
        else:
            self._columns = []
        self._show()

    def clear_rows(self) -> None:
        self._rows = []
        self._columns = []
        self._show()