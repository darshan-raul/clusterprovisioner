"""Tests for the Strata API client + auth + tokens.

Uses respx to mock httpx; no network calls.
"""

from __future__ import annotations

from pathlib import Path

import httpx
import pytest
import respx

from strata_tui.api.auth import DeviceCodeFlow, DeviceFlowError
from strata_tui.api.client import StrataClient, StrataClientError
from strata_tui.api.tokens import (
    StoredToken,
    clear_token,
    from_token_response,
    load_token,
    save_token,
)


@pytest.fixture
def mock_keycloak() -> respx.MockRouter:
    with respx.mock(assert_all_called=False) as router:
        yield router


# ── tokens.py ──────────────────────────────────────────────────────


def test_save_and_load_token_roundtrip(tmp_path: Path) -> None:
    path = tmp_path / "token.json"
    token = StoredToken(access_token="abc", expires_at=1234567890.0, issuer="http://x")
    save_token(token, path)
    loaded = load_token(path)
    assert loaded is not None
    assert loaded.access_token == "abc"
    assert loaded.expires_at == 1234567890.0
    assert loaded.issuer == "http://x"


def test_load_token_missing_returns_none(tmp_path: Path) -> None:
    assert load_token(tmp_path / "nope.json") is None


def test_token_file_is_owner_only(tmp_path: Path) -> None:
    path = tmp_path / "token.json"
    save_token(StoredToken(access_token="x", expires_at=1.0), path)
    mode = path.stat().st_mode & 0o777
    assert mode == 0o600, f"expected 0600, got {oct(mode)}"


def test_token_expiry_check() -> None:
    now = 1000.0
    import time
    time.time = lambda: now  # type: ignore[assignment]
    try:
        expired = StoredToken(access_token="x", expires_at=900.0)
        assert expired.is_expired() is True
        assert expired.is_expired(leeway=0) is True
        fresh = StoredToken(access_token="y", expires_at=1100.0)
        assert fresh.is_expired() is False
        assert fresh.is_expired(leeway=200) is True
    finally:
        pass  # no need to restore


def test_clear_token_is_idempotent(tmp_path: Path) -> None:
    path = tmp_path / "token.json"
    clear_token(path)  # missing
    save_token(StoredToken(access_token="x", expires_at=1.0), path)
    clear_token(path)
    assert load_token(path) is None


def test_from_token_response() -> None:
    class Fake:
        access_token = "abc"
        expires_in = 60
        refresh_token = "rt"

    stored = from_token_response(Fake(), issuer="http://x")
    assert stored.access_token == "abc"
    assert stored.refresh_token == "rt"
    assert stored.issuer == "http://x"


# ── auth.py (device-code flow) ─────────────────────────────────────


def test_device_flow_login_success(mock_keycloak: respx.MockRouter) -> None:
    mock_keycloak.post(
        "http://kc/realms/strata-dev/protocol/openid-connect/auth/device"
    ).respond(200, json={
        "device_code": "dev-code",
        "user_code": "USER-CODE",
        "verification_uri": "http://kc/device",
        "expires_in": 600,
        "interval": 1,
    })
    mock_keycloak.post(
        "http://kc/realms/strata-dev/protocol/openid-connect/token"
    ).mock(side_effect=[
        httpx.Response(400, json={"error": "authorization_pending"}),
        httpx.Response(200, json={
            "access_token": "AT",
            "refresh_token": "RT",
            "expires_in": 3600,
            "token_type": "Bearer",
        }),
    ])

    flow = DeviceCodeFlow(
        keycloak_url="http://kc", realm="strata-dev", client_id="strata-tui"
    )
    import asyncio
    code, token = asyncio.run(_do_login(flow))
    assert code.user_code == "USER-CODE"
    assert token.access_token == "AT"


def test_device_flow_expired_token(mock_keycloak: respx.MockRouter) -> None:
    mock_keycloak.post(
        "http://kc/realms/strata-dev/protocol/openid-connect/auth/device"
    ).respond(200, json={
        "device_code": "x", "user_code": "y",
        "verification_uri": "http://kc/device",
        "expires_in": 600, "interval": 1,
    })
    mock_keycloak.post(
        "http://kc/realms/strata-dev/protocol/openid-connect/token"
    ).respond(400, json={"error": "expired_token"})

    flow = DeviceCodeFlow(
        keycloak_url="http://kc", realm="strata-dev", client_id="strata-tui"
    )
    with pytest.raises(DeviceFlowError, match="expired"):
        import asyncio
        asyncio.run(flow.login())


async def _do_login(flow):
    code = await flow.request_device_code()
    token = await flow.poll_for_token(code)
    return code, token


# ── client.py ──────────────────────────────────────────────────────


@pytest.fixture
def mock_orchestrator() -> respx.MockRouter:
    with respx.mock(base_url="http://orch", assert_all_called=False) as router:
        yield router


async def test_me_returns_claims(mock_orchestrator: respx.MockRouter) -> None:
    mock_orchestrator.get("/api/v1/me").respond(
        200, json={
            "sub": "alice",
            "email": "alice@example.com",
            "name": "Alice",
            "preferred_username": "alice",
            "aud": ["strata-tui"],
        }
    )
    client = StrataClient(base_url="http://orch", token="AT")
    me = await client.me()
    assert me.sub == "alice"
    assert me.preferred_username == "alice"


async def test_me_401_raises(mock_orchestrator: respx.MockRouter) -> None:
    mock_orchestrator.get("/api/v1/me").respond(401, json={"error": "invalid token"})
    client = StrataClient(base_url="http://orch", token="BAD")
    with pytest.raises(StrataClientError, match="unauthorized"):
        await client.me()


async def test_list_clusters(mock_orchestrator: respx.MockRouter) -> None:
    mock_orchestrator.get("/api/v1/clusters/").respond(
        200,
        json={
            "clusters": [
                {"id": "cl-001", "user_id": "alice", "name": "demo", "context": "demo"},
                {"id": "cl-002", "user_id": "alice", "name": "prod", "context": "prod"},
            ]
        },
    )
    client = StrataClient(base_url="http://orch", token="AT")
    clusters = await client.list_clusters()
    assert [c.id for c in clusters] == ["cl-001", "cl-002"]


async def test_list_pods_success(mock_orchestrator: respx.MockRouter) -> None:
    mock_orchestrator.get("/api/v1/clusters/cl-001/pods").respond(
        200,
        json=[
            {"name": "p1", "namespace": "default", "phase": "Running"},
            {"name": "p2", "namespace": "default", "phase": "Pending"},
        ],
    )
    client = StrataClient(base_url="http://orch", token="AT")
    pods = await client.list_pods("cl-001")
    assert pods[0]["name"] == "p1"


async def test_list_pods_mcp_error_propagates(mock_orchestrator: respx.MockRouter) -> None:
    mock_orchestrator.get("/api/v1/clusters/cl-001/pods").respond(
        200,
        json={
            "content": [
                {"type": "text", "text": "kubernetes API error: forbidden"}
            ],
            "isError": True,
        },
    )
    client = StrataClient(base_url="http://orch", token="AT")
    with pytest.raises(StrataClientError, match="forbidden"):
        await client.list_pods("cl-001")


async def test_list_pods_404(mock_orchestrator: respx.MockRouter) -> None:
    mock_orchestrator.get("/api/v1/clusters/missing/pods").respond(
        404, json={"error": "cluster not found"}
    )
    client = StrataClient(base_url="http://orch", token="AT")
    with pytest.raises(StrataClientError, match="not found"):
        await client.list_pods("missing")


async def test_healthz(mock_orchestrator: respx.MockRouter) -> None:
    mock_orchestrator.get("/healthz").respond(
        200, json={"status": "ok", "started_at": "2026-01-01T00:00:00Z"}
    )
    client = StrataClient(base_url="http://orch")
    body = await client.healthz()
    assert body["status"] == "ok"


async def test_delete_pod_success(mock_orchestrator: respx.MockRouter) -> None:
    mock_orchestrator.delete("/api/v1/clusters/cl-001/pods/test-pod").respond(
        200,
        json={"status": "deleted", "name": "test-pod", "namespace": "default"},
    )
    client = StrataClient(base_url="http://orch", token="AT")
    res = await client.delete_pod("cl-001", "test-pod")
    assert res["status"] == "deleted"
    assert res["name"] == "test-pod"


async def test_delete_pod_mcp_error(mock_orchestrator: respx.MockRouter) -> None:
    mock_orchestrator.delete("/api/v1/clusters/cl-001/pods/test-pod").respond(
        200,
        json={
            "content": [{"type": "text", "text": "kubernetes API error: NotFound"}],
            "isError": True,
        },
    )
    client = StrataClient(base_url="http://orch", token="AT")
    with pytest.raises(StrataClientError, match="NotFound"):
        await client.delete_pod("cl-001", "test-pod")


async def test_apply_manifest_success(mock_orchestrator: respx.MockRouter) -> None:
    mock_orchestrator.post("/api/v1/clusters/cl-001/apply").respond(
        200,
        json={"status": "applied", "count": 1, "applied": [{"name": "cm", "action": "created"}]},
    )
    client = StrataClient(base_url="http://orch", token="AT")
    res = await client.apply_manifest("cl-001", "apiVersion: v1\nkind: ConfigMap")
    assert res["status"] == "applied"
    assert res["count"] == 1


async def test_exec_command_success(mock_orchestrator: respx.MockRouter) -> None:
    mock_orchestrator.post("/api/v1/clusters/cl-001/pods/test-pod/exec").respond(
        200,
        json={"status": "completed", "output": "root\n"},
    )
    client = StrataClient(base_url="http://orch", token="AT")
    res = await client.exec_command("cl-001", "test-pod", "whoami")
    assert res["status"] == "completed"
    assert res["output"] == "root\n"


async def test_create_cluster_success(mock_orchestrator: respx.MockRouter) -> None:
    mock_orchestrator.post("/api/v1/clusters").respond(
        201,
        json={
            "cluster": {
                "id": "cl-new",
                "name": "staging",
                "context": "staging-ctx",
            }
        },
    )
    client = StrataClient(base_url="http://orch", token="AT")
    cluster = await client.create_cluster("staging", "apiVersion: v1", context="staging-ctx")
    assert cluster.id == "cl-new"
    assert cluster.name == "staging"
    assert cluster.context == "staging-ctx"


async def test_delete_cluster_success(mock_orchestrator: respx.MockRouter) -> None:
    mock_orchestrator.delete("/api/v1/clusters/cl-new").respond(
        200,
        json={"status": "deleted", "cluster_id": "cl-new"},
    )
    client = StrataClient(base_url="http://orch", token="AT")
    res = await client.delete_cluster("cl-new")
    assert res["status"] == "deleted"