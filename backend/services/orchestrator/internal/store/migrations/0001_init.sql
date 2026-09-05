-- Strata Phase 1 schema
--
-- Tables:
--   users          — one row per Keycloak user (we cache the basics for joins)
--   clusters       — one row per k8s cluster a user has registered
--   cluster_creds  — encrypted kubeconfig (Phase 1: cleartext path; Phase 4 encrypts)
--
-- Note: we use TIMESTAMP and CURRENT_TIMESTAMP (rather than TIMESTAMPTZ
-- and now()) so the schema works on both PostgreSQL (the production
-- target via CloudNativePG) and SQLite (used for tests in dev shells
-- without a running Postgres).

CREATE TABLE IF NOT EXISTS users (
    id           TEXT PRIMARY KEY,        -- Keycloak subject (sub claim)
    username     TEXT NOT NULL UNIQUE,
    email        TEXT,
    created_at   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS clusters (
    id          TEXT PRIMARY KEY,         -- cluster ID surfaced to the TUI
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    context     TEXT NOT NULL,            -- kubeconfig context name (or "default")
    created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (user_id, name)
);

CREATE INDEX IF NOT EXISTS idx_clusters_user ON clusters (user_id);

-- Phase 1 stored the kubeconfig path the MCP server should use.
-- Phase 4 adds encrypted ciphertext + a KMS-wrapped DEK.
CREATE TABLE IF NOT EXISTS cluster_creds (
    cluster_id           TEXT PRIMARY KEY REFERENCES clusters(id) ON DELETE CASCADE,
    kubeconfig_path      TEXT NOT NULL DEFAULT '',
    encrypted_kubeconfig TEXT NOT NULL DEFAULT '',
    dek_ciphertext       TEXT NOT NULL DEFAULT ''
);