"""The Textual App — top-level layout, bindings, event wiring.

This module is the **top of the runtime stack**. It ties together:

- The LangChain chat surface (BYOK MiniMax M3).
- The Strata backend client (httpx + JWT).
- The :command palette (:login, :get, :ctx, etc.).
- Modal screens (LoginScreen for OIDC device-code auth).
- Widgets (MessageHistory, ResourceTable, StatusBar).
"""

from __future__ import annotations

import asyncio
from typing import TYPE_CHECKING

from langchain_core.messages import HumanMessage, SystemMessage
from langchain_openai import ChatOpenAI
from textual import work
from textual.app import App, ComposeResult
from textual.containers import Vertical
from textual.widgets import Header, Input

from strata_tui.api.auth import DeviceCodeFlow
from strata_tui.api.client import Cluster, StrataClient
from strata_tui.api.tokens import clear_token, load_token
from strata_tui.commands import ContextCommand, GetCommand, parse_command_line
from strata_tui.config import load_settings
from strata_tui.screens import LoginScreen
from strata_tui.widgets import MessageHistory, ResourceTable, StatusBar

if TYPE_CHECKING:
    from strata_tui.api.tokens import StoredToken


SYSTEM_PROMPT = (
    "You are Strata, an AI co-pilot for managing Kubernetes clusters. "
    "Phase 1 placeholder: the agent-driven tool calls land in Phase 2. "
    "For now, use the :command palette for cluster operations "
    "(:get pods, :ctx list, :ctx use <name>)."
)


class StrataTUIApp(App):
    """The Strata terminal interface."""

    CSS_PATH = None
    TITLE = "Strata"

    BINDINGS = [
        ("ctrl+c", "quit", "Quit"),
        ("ctrl+l", "clear", "Clear history"),
    ]

    def __init__(self) -> None:
        super().__init__()
        self._settings = load_settings()
        self._llm: ChatOpenAI | None = None
        self._stored_token: StoredToken | None = None
        self._client: StrataClient | None = None
        self._active_cluster: Cluster | None = None

    # ── layout ────────────────────────────────────────────────────

    def compose(self) -> ComposeResult:
        yield Header()
        with Vertical():
            yield MessageHistory(id="history")
            yield ResourceTable()
            yield Input(placeholder="Ask Strata, or type :command", id="input")
            yield StatusBar(self._settings.model, id="status")

    def on_mount(self) -> None:
        self.query_one("#status", StatusBar).set_model(self._settings.model)
        self._init_llm()
        asyncio.create_task(self._bootstrap_session())

    # ── session bootstrap ──────────────────────────────────────────

    def _init_llm(self) -> None:
        try:
            self._llm = ChatOpenAI(
                model=self._settings.model,
                base_url=self._settings.base_url,
                api_key=self._settings.api_key or "missing",
                temperature=self._settings.temperature,
            )
        except Exception as exc:  # noqa: BLE001
            self.history.append_error(f"LLM init failed: {exc}")

    async def _bootstrap_session(self) -> None:
        history = self.history
        # Try a cached token first.
        try:
            stored = load_token()
        except Exception as exc:  # noqa: BLE001
            history.append_error(f"failed to read token: {exc}")
            stored = None

        if stored and not stored.is_expired():
            self._apply_token(stored)
            await self._refresh_identity()
            return
        if stored:
            history.append_info("cached token expired; please :login again")
            clear_token()
        history.append_info("not logged in — type :login to authenticate")

    def _apply_token(self, stored: StoredToken) -> None:
        self._stored_token = stored
        self._client = StrataClient(
            base_url=self._settings.backend_url,
            token=stored.access_token,
        )

    async def _refresh_identity(self) -> None:
        history = self.history
        if not self._client:
            return
        try:
            me = await self._client.me()
            self.status.set_user(me.preferred_username or me.sub)
            history.append_info(f"signed in as {me.preferred_username or me.sub}")
            clusters = await self._client.list_clusters()
            if clusters:
                self.set_active_cluster(clusters[0])
        except Exception as exc:  # noqa: BLE001
            history.append_error(f"identity refresh failed: {exc}")

    # ── properties ─────────────────────────────────────────────────

    @property
    def history(self) -> MessageHistory:
        return self.query_one("#history", MessageHistory)

    @property
    def resource_table(self) -> ResourceTable:
        return self.query_one("#resource-table", ResourceTable)

    @property
    def status(self) -> StatusBar:
        return self.query_one("#status", StatusBar)

    @property
    def client(self) -> StrataClient | None:
        return self._client

    @property
    def active_cluster(self) -> Cluster | None:
        return self._active_cluster

    def set_active_cluster(self, cluster: Cluster) -> None:
        self._active_cluster = cluster
        self.status.set_cluster(f"{cluster.id} ({cluster.name})")
        self.resource_table.clear_rows()

    # ── input handling ────────────────────────────────────────────

    def on_input_submitted(self, event: Input.Submitted) -> None:
        text = event.value.strip()
        if not text:
            return
        self.query_one("#input", Input).value = ""
        cmd, args = parse_command_line(text)
        if cmd:
            asyncio.create_task(self._dispatch_command(cmd, args))
            return
        self._chat(text)

    async def _dispatch_command(self, cmd: str, args: list[str]) -> None:
        if cmd == "login":
            await self._run_login()
            return
        if cmd == "logout":
            clear_token()
            self._stored_token = None
            self._client = None
            self._active_cluster = None
            self.status.set_user("-")
            self.status.set_cluster("-")
            self.history.append_info("signed out")
            return
        if cmd == "get":
            await GetCommand(self).execute(args)
            return
        if cmd == "ctx":
            await ContextCommand(self).execute(args)
            return
        if cmd == "help":
            self._show_help()
            return
        self.history.append_error(f"unknown command :{cmd}; try :help")

    def _show_help(self) -> None:
        self.history.append_ai(
            "Available commands:\n"
            "  :login                            Sign in via OIDC device-code\n"
            "  :logout                           Clear the cached token\n"
            "  :ctx list                         List your registered clusters\n"
            "  :ctx use <id-or-name>             Switch the active cluster\n"
            "  :get pods [-n ns] [-l k=v]         List pods in the active cluster\n"
            "  :help                             Show this help\n"
            "\nAnything else goes to the chat rail."
        )

    async def _run_login(self) -> None:
        flow = DeviceCodeFlow(
            keycloak_url=self._settings.keycloak_url,
            realm=self._settings.keycloak_realm,
            client_id=self._settings.keycloak_client_id,
        )
        screen = LoginScreen(
            flow=flow,
            keycloak_url=self._settings.keycloak_url,
            realm=self._settings.keycloak_realm,
        )
        result = await self.push_screen_wait(screen)
        if result is None:
            self.history.append_info("login cancelled")
            return
        # Re-read the saved token so the file mtime is honored.
        stored = load_token()
        if stored is None:
            self.history.append_error("login succeeded but token could not be read")
            return
        self._apply_token(stored)
        await self._refresh_identity()

    # ── chat rail (BYOK, no agent in Phase 1) ─────────────────────

    def _chat(self, text: str) -> None:
        self.history.append_user(text)
        if self._llm is None:
            self.history.append_error("LLM not initialised — check MINIMAX_API_KEY in tui/.env")
            return
        self._run_turn(text)

    @work(thread=True, exclusive=True)
    def _run_turn(self, text: str) -> None:
        assert self._llm is not None
        status = self.status
        self.call_from_thread(status.set_busy, True)
        try:
            messages = [SystemMessage(content=SYSTEM_PROMPT), HumanMessage(content=text)]
            response = self._llm.invoke(messages)
            self.call_from_thread(self.history.append_ai, response.content)
        except Exception as exc:  # noqa: BLE001
            self.call_from_thread(self.history.append_error, f"LLM call failed: {exc}")
        finally:
            self.call_from_thread(status.set_busy, False)

    # ── action handlers ────────────────────────────────────────────

    def action_clear(self) -> None:
        self.history.clear()