"""Unit tests for AES-256-GCM decryption and encrypted kubeconfig loading."""

import base64
import os
from unittest.mock import patch

import pytest
from cryptography.exceptions import InvalidTag
from cryptography.hazmat.primitives.ciphers.aead import AESGCM

from strata_mcp_k8s.crypto import decrypt_kubeconfig, derive_key
from strata_mcp_k8s.kube import load_kubeconfig


def test_derive_key_length():
    k = derive_key("any-secret")
    assert len(k) == 32
    assert isinstance(k, bytes)


def test_decrypt_roundtrip():
    secret = "my-secret-key-phrase"
    key = derive_key(secret)
    aes = AESGCM(key)
    nonce = os.urandom(12)
    plaintext = b"apiVersion: v1\nkind: Config\nclusters: []"
    ct = aes.encrypt(nonce, plaintext, None)
    encoded = base64.b64encode(nonce + ct).decode("ascii")

    decrypted = decrypt_kubeconfig(encoded, secret=secret)
    assert decrypted == plaintext.decode("utf-8")


def test_decrypt_with_default_env_secret(monkeypatch):
    monkeypatch.setenv("ENCRYPTION_SECRET", "custom-env-secret")
    key = derive_key("custom-env-secret")
    aes = AESGCM(key)
    nonce = os.urandom(12)
    plaintext = b"test-cluster-kubeconfig"
    ct = aes.encrypt(nonce, plaintext, None)
    encoded = base64.b64encode(nonce + ct).decode("ascii")

    decrypted = decrypt_kubeconfig(encoded)
    assert decrypted == "test-cluster-kubeconfig"


def test_decrypt_wrong_key_fails():
    key1 = derive_key("key-one")
    aes = AESGCM(key1)
    nonce = os.urandom(12)
    ct = aes.encrypt(nonce, b"secret-data", None)
    encoded = base64.b64encode(nonce + ct).decode("ascii")

    with pytest.raises(InvalidTag):
        decrypt_kubeconfig(encoded, secret="key-two")


def test_decrypt_tampered_fails():
    secret = "secret-phrase"
    key = derive_key(secret)
    aes = AESGCM(key)
    nonce = os.urandom(12)
    ct = aes.encrypt(nonce, b"secret-data", None)
    encoded = base64.b64encode(nonce + ct).decode("ascii")
    tampered = "AAAA" + encoded[4:]

    with pytest.raises(InvalidTag):
        decrypt_kubeconfig(tampered, secret=secret)


def test_load_kubeconfig_encrypted():
    secret = "test-secret"
    key = derive_key(secret)
    aes = AESGCM(key)
    nonce = os.urandom(12)
    sample_kubeconfig = """
apiVersion: v1
kind: Config
current-context: memory-ctx
clusters:
- name: memory-cluster
  cluster:
    server: https://10.96.0.1:443
contexts:
- name: memory-ctx
  context:
    cluster: memory-cluster
    user: test-user
users:
- name: test-user
  user:
    token: memory-token
"""
    ct = aes.encrypt(nonce, sample_kubeconfig.encode("utf-8"), None)
    enc_payload = base64.b64encode(nonce + ct).decode("ascii")

    with patch.dict(os.environ, {"ENCRYPTION_SECRET": secret}):
        client_api = load_kubeconfig("cl-memory", kubeconfig_encrypted=enc_payload)
        assert client_api is not None
        assert client_api.configuration.host == "https://10.96.0.1:443"
