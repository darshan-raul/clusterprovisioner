"""Strata TUI configuration.

Reads settings from environment variables and ``.env``. The TUI
uses MiniMax M3 (OpenAI-compatible) directly via
``langchain-openai`` for the BYOK LLM path. Cluster data goes
through the Strata backend over HTTPS with a per-user JWT.

Environment variables:

- ``MINIMAX_BASE_URL``           — OpenAI-compatible base URL.
- ``MINIMAX_API_KEY``            — required. The user's LLM provider key.
- ``MINIMAX_MODEL``              — model name. Default: ``MiniMax-M3``.
- ``STRATA_BACKEND_URL``         — base URL of the Strata backend
                                   orchestrator. Default:
                                   ``http://localhost:8080`` for dev.
- ``STRATA_KEYCLOAK_URL``        — base URL of the Keycloak server.
                                   Default: ``http://localhost:8081``.
- ``STRATA_KEYCLOAK_REALM``      — OIDC realm name. Default:
                                   ``strata-dev``.
- ``STRATA_KEYCLOAK_CLIENT_ID``  — TUI's registered client_id.
                                   Default: ``strata-tui``.
- ``TEMPERATURE``                — sampling temperature. Default:
                                   ``0.2``.
"""

from __future__ import annotations

import os
from dataclasses import dataclass
from pathlib import Path

from dotenv import load_dotenv


@dataclass(frozen=True)
class Settings:
    """Resolved TUI settings."""

    base_url: str
    api_key: str
    model: str
    backend_url: str
    keycloak_url: str
    keycloak_realm: str
    keycloak_client_id: str
    temperature: float


def load_settings(env_path: Path | None = None) -> Settings:
    """Load settings from ``.env`` (if present) + environment."""
    if env_path is None:
        env_path = Path(__file__).resolve().parent.parent / ".env"
    load_dotenv(env_path, override=False)

    return Settings(
        base_url=os.getenv("MINIMAX_BASE_URL", "https://api.minimax.chat/v1"),
        api_key=os.getenv("MINIMAX_API_KEY", ""),
        model=os.getenv("MINIMAX_MODEL", "MiniMax-M3"),
        backend_url=os.getenv("STRATA_BACKEND_URL", "http://localhost:8080"),
        keycloak_url=os.getenv("STRATA_KEYCLOAK_URL", "http://localhost:8081"),
        keycloak_realm=os.getenv("STRATA_KEYCLOAK_REALM", "strata-dev"),
        keycloak_client_id=os.getenv("STRATA_KEYCLOAK_CLIENT_ID", "strata-tui"),
        temperature=float(os.getenv("TEMPERATURE", "0.2")),
    )