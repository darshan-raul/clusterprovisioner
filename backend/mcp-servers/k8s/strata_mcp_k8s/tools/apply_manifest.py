"""``apply_manifest`` MCP tool (MUTATION).

Applies a YAML or JSON Kubernetes manifest to a cluster.
Creates new resources or patches existing ones.
"""

from __future__ import annotations

import logging
import re
from typing import Any

import yaml
from fastmcp import FastMCP
from kubernetes import client, utils
from kubernetes.client.rest import ApiException

from strata_mcp_k8s.kube import load_kubeconfig

logger = logging.getLogger(__name__)

APPLY_MANIFEST_DESCRIPTION = """\
[MUTATION] Apply a Kubernetes YAML/JSON manifest to the cluster.

Creates or updates resources (Deployments, Services, ConfigMaps, etc.)
defined in the given manifest string.

Use this when the user asks:
- "Apply this deployment manifest."
- "Create a service from this YAML."
- "Update the configmap."

This is a MUTATING operation and requires explicit confirmation.
Returns the list of applied resources with their status.
"""

UPPER_FOLLOWED_BY_LOWER_RE = re.compile(r"(.)([A-Z][a-z]+)")
LOWER_OR_NUM_FOLLOWED_BY_UPPER_RE = re.compile(r"([a-z0-9])([A-Z])")


def _get_api_and_kind(api_client: client.ApiClient, obj: dict[str, Any]):
    group, _, version = obj["apiVersion"].partition("/")
    if version == "":
        version = group
        group = "core"
    group = "".join(group.rsplit(".k8s.io", 1))
    group = "".join(word.capitalize() for word in group.split("."))
    fcn_to_call = f"{group}{version.capitalize()}Api"
    api_class = getattr(client, fcn_to_call, None)
    if api_class is None:
        return None, None
    k8s_api = api_class(api_client)
    kind = obj["kind"]
    kind = UPPER_FOLLOWED_BY_LOWER_RE.sub(r"\1_\2", kind)
    kind = LOWER_OR_NUM_FOLLOWED_BY_UPPER_RE.sub(r"\1_\2", kind).lower()
    return k8s_api, kind


def register_apply_manifest(mcp: FastMCP) -> None:
    """Register the ``apply_manifest`` tool on the given FastMCP server."""

    @mcp.tool(
        name="apply_manifest",
        description=APPLY_MANIFEST_DESCRIPTION,
        tags={"mutation", "k8s", "apply"},
    )
    def apply_manifest(
        cluster_id: str,
        manifest_yaml: str,
        namespace: str = "default",
        kubeconfig_encrypted: str | None = None,
        kubeconfig_path: str | None = None,
    ) -> dict[str, Any]:
        """Apply a Kubernetes YAML/JSON manifest.

        See ``APPLY_MANIFEST_DESCRIPTION`` for full details.
        """
        try:
            documents = list(yaml.safe_load_all(manifest_yaml))
        except Exception as exc:
            logger.warning("apply_manifest: failed to parse YAML: %s", exc)
            raise RuntimeError(f"invalid manifest YAML/JSON: {exc}") from exc

        valid_docs = [
            d for d in documents if isinstance(d, dict) and "kind" in d and "apiVersion" in d
        ]
        if not valid_docs:
            raise RuntimeError("manifest contains no valid Kubernetes resources")

        api_client = load_kubeconfig(
            cluster_id,
            kubeconfig_encrypted=kubeconfig_encrypted,
            kubeconfig_path=kubeconfig_path,
        )
        applied: list[dict[str, Any]] = []

        for doc in valid_docs:
            kind = doc.get("kind", "Unknown")
            metadata = doc.get("metadata", {})
            name = metadata.get("name", "unnamed")
            target_ns = metadata.get("namespace") or namespace

            try:
                # First attempt: create_from_dict
                utils.create_from_dict(api_client, doc, namespace=target_ns)
                applied.append({
                    "kind": kind,
                    "name": name,
                    "namespace": target_ns,
                    "action": "created",
                })
            except utils.FailToCreateError as exc:
                # Check if this is a 409 Conflict (already exists)
                is_conflict = any(
                    getattr(e, "status", None) == 409
                    for e in exc.api_exceptions
                )
                if not is_conflict:
                    err_msg = str(exc)
                    logger.warning("apply_manifest create failed: %s", err_msg)
                    raise RuntimeError(f"failed to create {kind}/{name}: {err_msg}") from exc

                # Attempt patch / update
                k8s_api, snake_kind = _get_api_and_kind(api_client, doc)
                patched = False
                if k8s_api and snake_kind:
                    namespaced_patch = getattr(k8s_api, f"patch_namespaced_{snake_kind}", None)
                    cluster_patch = getattr(k8s_api, f"patch_{snake_kind}", None)
                    try:
                        if namespaced_patch:
                            namespaced_patch(name=name, namespace=target_ns, body=doc)
                            patched = True
                        elif cluster_patch:
                            cluster_patch(name=name, body=doc)
                            patched = True
                    except ApiException as patch_exc:
                        logger.warning("apply_manifest patch failed: %s", patch_exc.reason)
                        raise RuntimeError(
                            f"failed to update {kind}/{name}: {patch_exc.reason}"
                        ) from patch_exc

                if patched:
                    applied.append({
                        "kind": kind,
                        "name": name,
                        "namespace": target_ns,
                        "action": "configured",
                    })
                else:
                    raise RuntimeError(
                        f"failed to apply {kind}/{name}: resource exists and patch is not supported"
                    ) from exc
            except ApiException as exc:
                logger.warning("apply_manifest API error: %s", exc.reason)
                raise RuntimeError(f"kubernetes API error: {exc.reason}") from exc
            except Exception as exc:  # noqa: BLE001
                logger.exception("apply_manifest unexpected error")
                raise RuntimeError(f"apply_manifest failed on {kind}/{name}: {exc}") from exc

        return {
            "status": "applied",
            "cluster_id": cluster_id,
            "count": len(applied),
            "applied": applied,
        }
