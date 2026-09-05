"""Kubeconfig loading and Kubernetes client utilities."""

from __future__ import annotations

import logging

from kubernetes import client, config

logger = logging.getLogger(__name__)


def load_kubeconfig(cluster_id: str) -> client.ApiClient:
    """Load a kubeconfig for the given cluster ID.

    In the backend cluster, the orchestrator mounts the cluster's
    kubeconfig at ``/etc/strata/kubeconfigs/{cluster_id}``.

    Returns an ``ApiClient`` configured for the cluster.
    """
    kubeconfig_path = f"/etc/strata/kubeconfigs/{cluster_id}"
    cfg = client.Configuration()
    config.load_kube_config(config_file=kubeconfig_path, client_configuration=cfg)
    return client.ApiClient(configuration=cfg)
