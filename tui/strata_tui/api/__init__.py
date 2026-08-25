"""Strata TUI — backend HTTP client and auth.

This package contains:

- :class:`StrataClient` — httpx-based client for the orchestrator's
  REST API.
- :class:`DeviceCodeFlow` — OIDC device-code flow against Keycloak.
- :func:`load_token` / :func:`save_token` — read/write the
  JWT at ``~/.config/strata/token.json``.

Phase 1 only needs the minimal surface: list clusters, list pods,
identify the current user.
"""

from strata_tui.api.auth import DeviceCodeFlow
from strata_tui.api.client import StrataClient, StrataClientError
from strata_tui.api.tokens import TokenError, load_token, save_token

__all__ = [
    "DeviceCodeFlow",
    "StrataClient",
    "StrataClientError",
    "TokenError",
    "load_token",
    "save_token",
]