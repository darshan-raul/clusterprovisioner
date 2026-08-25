"""Persist the JWT to disk between TUI invocations.

Tokens live at ``~/.config/strata/token.json``. The file is
created with ``0600`` permissions so other users on the same host
can't read the token.

Phase 1 stores only the access token. Phase 2 stores the refresh
token as well.
"""

from __future__ import annotations

import json
import os
import time
from dataclasses import asdict, dataclass
from pathlib import Path

CONFIG_DIR = Path(os.environ.get("XDG_CONFIG_HOME", str(Path.home() / ".config")))
STRATA_DIR = CONFIG_DIR / "strata"
TOKEN_PATH = STRATA_DIR / "token.json"


class TokenError(RuntimeError):
    """Raised when the token file can't be read or written."""


@dataclass
class StoredToken:
    access_token: str
    expires_at: float  # unix epoch seconds
    refresh_token: str | None = None
    issuer: str = ""

    def is_expired(self, *, leeway: int = 30) -> bool:
        """Return True if the token is past ``expires_at - leeway``."""
        return time.time() >= self.expires_at - leeway


def load_token(path: Path = TOKEN_PATH) -> StoredToken | None:
    """Read the token from disk. Returns None if the file doesn't exist
    or can't be parsed.
    """
    try:
        raw = path.read_text()
    except FileNotFoundError:
        return None
    except OSError as exc:
        raise TokenError(f"failed to read {path}: {exc}") from exc
    try:
        data = json.loads(raw)
    except json.JSONDecodeError as exc:
        raise TokenError(f"corrupt token file {path}: {exc}") from exc
    try:
        return StoredToken(**data)
    except TypeError as exc:
        raise TokenError(f"unexpected token shape in {path}: {exc}") from exc


def save_token(token: StoredToken, path: Path = TOKEN_PATH) -> None:
    """Atomically write the token to disk with 0600 permissions."""
    try:
        path.parent.mkdir(parents=True, exist_ok=True)
    except OSError as exc:
        raise TokenError(f"failed to create {path.parent}: {exc}") from exc
    tmp = path.with_suffix(".json.tmp")
    try:
        tmp.write_text(json.dumps(asdict(token)))
        os.chmod(tmp, 0o600)
        os.replace(tmp, path)
    except OSError as exc:
        raise TokenError(f"failed to write {path}: {exc}") from exc


def clear_token(path: Path = TOKEN_PATH) -> None:
    """Remove the token file if it exists."""
    try:
        path.unlink()
    except FileNotFoundError:
        pass


def from_token_response(payload, *, issuer: str = "") -> StoredToken:
    """Build a StoredToken from a TokenResponse-like object."""
    expires_at = time.time() + payload.expires_in
    return StoredToken(
        access_token=payload.access_token,
        expires_at=expires_at,
        refresh_token=payload.refresh_token,
        issuer=issuer,
    )