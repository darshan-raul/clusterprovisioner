"""Kubeconfig loading and Kubernetes client utilities."""

from __future__ import annotations

import logging
from typing import Any

import yaml
from kubernetes import client, config

from strata_mcp_k8s.crypto import decrypt_kubeconfig

logger = logging.getLogger(__name__)


def load_kubeconfig(
    cluster_id: str,
    *,
    kubeconfig_encrypted: str | None = None,
    kubeconfig_path: str | None = None,
) -> client.ApiClient:
    """Load a kubeconfig for the given cluster ID.

    If ``kubeconfig_encrypted`` is provided, it is decrypted in-memory
    via AES-256-GCM and loaded directly into a ``client.Configuration``
    without ever writing cleartext credentials to disk.

    Otherwise, it loads from ``kubeconfig_path`` if specified, or
    falls back to ``/etc/strata/kubeconfigs/{cluster_id}``.

    Returns an ``ApiClient`` configured for the cluster.
    """
    cfg = client.Configuration()

    if kubeconfig_encrypted:
        raw_yaml = decrypt_kubeconfig(kubeconfig_encrypted)
        config_dict: dict[str, Any] = yaml.safe_load(raw_yaml)
        config.load_kube_config_from_dict(config_dict, client_configuration=cfg)
        return client.ApiClient(configuration=cfg)

    path = kubeconfig_path or f"/etc/strata/kubeconfigs/{cluster_id}"
    config.load_kube_config(config_file=path, client_configuration=cfg)
    return client.ApiClient(configuration=cfg)
