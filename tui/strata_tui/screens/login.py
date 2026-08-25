"""``:login`` screen — OIDC device-code flow modal.

Walks the user through the device-code grant:

1. POST to Keycloak's device endpoint → get device_code, user_code,
   verification_uri.
2. Show the URL + code in the modal.
3. Poll the token endpoint until the user approves.
4. Save the token to disk and dismiss.
"""

from __future__ import annotations

import asyncio
from dataclasses import dataclass

from textual.app import ComposeResult
from textual.containers import Vertical
from textual.screen import ModalScreen
from textual.widgets import Button, Label, Static

from strata_tui.api.auth import DeviceCodeFlow, TokenResponse
from strata_tui.api.tokens import from_token_response, save_token


@dataclass
class LoginResult:
    """Returned via ``dismiss`` when the user successfully authenticates."""

    token: TokenResponse
    issuer: str


class LoginScreen(ModalScreen[LoginResult | None]):
    """Modal that walks the user through OIDC device-code authentication."""

    DEFAULT_CSS = """
    LoginScreen {
        align: center middle;
    }
    #login-frame {
        width: 70;
        height: auto;
        border: round $primary;
        padding: 1 2;
    }
    #login-frame Label {
        width: 100%;
        content-align: center middle;
    }
    #login-actions {
        height: auto;
        align-horizontal: center;
        margin-top: 1;
    }
    """

    def __init__(
        self,
        *,
        flow: DeviceCodeFlow,
        keycloak_url: str,
        realm: str,
    ) -> None:
        super().__init__()
        self._flow = flow
        self._keycloak_url = keycloak_url
        self._realm = realm
        self._code = None  # type: ignore[var-annotated]
        self._status_label: Label | None = None

    def compose(self) -> ComposeResult:
        with Vertical(id="login-frame"):
            yield Label("[b]Sign in to Strata[/b]", id="login-title")
            yield Static(
                "Starting device-code flow…",
                id="login-detail",
            )
            yield Label("", id="login-status")
            with Vertical(id="login-actions"):
                yield Button("Cancel", id="login-cancel")

    async def on_mount(self) -> None:
        self._status_label = self.query_one("#login-status", Label)
        asyncio.create_task(self._run_flow())

    async def _run_flow(self) -> None:
        detail = self.query_one("#login-detail", Static)
        status = self.query_one("#login-status", Label)
        try:
            code = await self._flow.request_device_code()
        except Exception as exc:  # noqa: BLE001
            detail.update(f"[red]Failed to start device flow: {exc}[/red]")
            return
        self._code = code
        detail.update(
            f"[b]Open this URL in a browser:[/b]\n\n"
            f"  {code.verification_uri}\n\n"
            f"[b]Then enter this code:[/b]\n\n"
            f"  [reverse]{code.user_code}[/reverse]\n\n"
            f"Waiting for you to approve…"
        )
        status.update(f"Code expires in {code.expires_in // 60} minutes")
        try:
            token = await self._flow.poll_for_token(code)
        except Exception as exc:  # noqa: BLE001
            status.update(f"[red]{exc}[/red]")
            return
        issuer = f"{self._keycloak_url}/realms/{self._realm}"
        save_token(from_token_response(token, issuer=issuer))
        status.update("[green]Signed in.[/green]")
        self.dismiss(LoginResult(token=token, issuer=issuer))

    def on_button_pressed(self, event: Button.Pressed) -> None:
        if event.button.id == "login-cancel":
            self.dismiss(None)