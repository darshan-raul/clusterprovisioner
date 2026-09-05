"""AES-256-GCM decryption for Strata cluster credentials."""

from __future__ import annotations

import base64
import hashlib
import os

from cryptography.hazmat.primitives.ciphers.aead import AESGCM

DEFAULT_ENCRYPTION_SECRET = "strata-dev-insecure-master-key-change-me"


def derive_key(secret: str) -> bytes:
    """Derive a 32-byte AES-256 key from a secret using SHA-256."""
    return hashlib.sha256(secret.encode("utf-8")).digest()


def decrypt_kubeconfig(encrypted_b64: str, secret: str | None = None) -> str:
    """Decrypt a base64-encoded AES-256-GCM ciphertext containing a kubeconfig.

    Expects the first 12 bytes to be the nonce, followed by the ciphertext
    and 16-byte authentication tag (matching Go's crypto.Encrypt format).
    """
    if not secret:
        secret = os.environ.get("ENCRYPTION_SECRET", DEFAULT_ENCRYPTION_SECRET)
    key = derive_key(secret)
    raw = base64.b64decode(encrypted_b64)
    if len(raw) < 12:
        raise ValueError("encrypted payload too short to contain nonce")
    nonce = raw[:12]
    ciphertext = raw[12:]
    aesgcm = AESGCM(key)
    plaintext = aesgcm.decrypt(nonce, ciphertext, None)
    return plaintext.decode("utf-8")
