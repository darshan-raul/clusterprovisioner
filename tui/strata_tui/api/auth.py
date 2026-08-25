"""OIDC device-code flow against Keycloak.

The TUI initiates a device-code grant against the user's Keycloak
realm. The user opens the ``verification_uri`` in a browser,
enters the ``user_code``, and approves the device. The TUI polls
the token endpoint until the user approves (or the device code
expires).

Reference:
    https://datatracker.ietf.org/doc/html/rfc8628
"""

from __future__ import annotations

import asyncio
import json
from dataclasses import dataclass
from typing import Any
from urllib.parse import urlencode

import httpx


class DeviceFlowError(RuntimeError):
    """Raised when the device-code flow cannot complete."""


@dataclass
class DeviceCodeResponse:
    """The initial response from Keycloak's device endpoint."""

    device_code: str
    user_code: str
    verification_uri: str
    expires_in: int
    interval: int

    @classmethod
    def from_payload(cls, payload: dict[str, Any]) -> DeviceCodeResponse:
        try:
            return cls(
                device_code=payload["device_code"],
                user_code=payload["user_code"],
                verification_uri=payload["verification_uri"],
                expires_in=int(payload["expires_in"]),
                interval=int(payload["interval"]),
            )
        except KeyError as exc:
            raise DeviceFlowError(f"missing field in device-code response: {exc}") from exc


class TokenResponse:
    """The successful token response."""

    def __init__(self, payload: dict[str, Any]) -> None:
        self.access_token: str = payload["access_token"]
        self.token_type: str = payload.get("token_type", "Bearer")
        self.expires_in: int = int(payload.get("expires_in", 3600))
        self.refresh_token: str | None = payload.get("refresh_token")
        # The OIDC id_token isn't required by the orchestrator
        # (it uses the access_token), but we capture it for future use.
        self.id_token: str | None = payload.get("id_token")
        self.scope: str = payload.get("scope", "")


class DeviceCodeFlow:
    """Run the OIDC device-code flow against a Keycloak realm.

    Args:
        keycloak_url: Base URL of the Keycloak server, e.g.
            ``http://localhost:8081`` for a port-forwarded dev cluster.
        realm: The OIDC realm name, e.g. ``strata-dev``.
        client_id: The client_id registered for the TUI in the realm,
            e.g. ``strata-tui``.
    """

    def __init__(
        self,
        *,
        keycloak_url: str,
        realm: str,
        client_id: str,
        http_client: httpx.AsyncClient | None = None,
    ) -> None:
        self._base = keycloak_url.rstrip("/")
        self._realm = realm
        self._client_id = client_id
        self._http = http_client or httpx.AsyncClient(timeout=10.0)

    def _device_url(self) -> str:
        return f"{self._base}/realms/{self._realm}/protocol/openid-connect/auth/device"

    def _token_url(self) -> str:
        return f"{self._base}/realms/{self._realm}/protocol/openid-connect/token"

    async def request_device_code(self) -> DeviceCodeResponse:
        """POST to Keycloak's device endpoint. Returns the codes
        the user needs to enter on the verification page.
        """
        body = {"client_id": self._client_id, "scope": "openid"}
        resp = await self._http.post(
            self._device_url(),
            data=body,
            headers={"Content-Type": "application/x-www-form-urlencoded"},
        )
        if resp.status_code != 200:
            raise DeviceFlowError(
                f"device-code endpoint returned {resp.status_code}: {resp.text}"
            )
        return DeviceCodeResponse.from_payload(resp.json())

    async def poll_for_token(self, code: DeviceCodeResponse) -> TokenResponse:
        """Poll the token endpoint until the user approves.

        Implements the standard back-off: respect ``code.interval``
        and slow down on ``slow_down`` errors.
        """
        interval = max(code.interval, 1)
        deadline = asyncio.get_event_loop().time() + code.expires_in
        body = {
            "grant_type": "urn:ietf:params:oauth:grant-type:device_code",
            "device_code": code.device_code,
            "client_id": self._client_id,
        }
        while True:
            if asyncio.get_event_loop().time() > deadline:
                raise DeviceFlowError("device-code grant expired")
            await asyncio.sleep(interval)
            resp = await self._http.post(
                self._token_url(),
                data=body,
                headers={"Content-Type": "application/x-www-form-urlencoded"},
            )
            payload = resp.json()
            if "access_token" in payload:
                return TokenResponse(payload)
            err = payload.get("error")
            if err == "authorization_pending":
                continue
            if err == "slow_down":
                interval += 5
                continue
            if err == "expired_token":
                raise DeviceFlowError("device-code grant expired")
            raise DeviceFlowError(
                f"token endpoint error: {err}: {payload.get('error_description', '')}"
            )

    async def login(self) -> TokenResponse:
        """Convenience: request a code, then poll for a token."""
        code = await self.request_device_code()
        return await self.poll_for_token(code)

    async def aclose(self) -> None:
        if self._http is not None:
            await self._http.aclose()

    # Convenience for tests: encode the form body without using httpx.
    @staticmethod
    def _encode_form(data: dict[str, str]) -> str:
        return urlencode(data)

    @staticmethod
    def _dump_for_log(payload: dict[str, Any]) -> str:
        return json.dumps({k: v for k, v in payload.items() if k != "device_code"})