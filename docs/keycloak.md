# Keycloak & OIDC in Strata

Keycloak is the central OpenID Connect (OIDC) Identity Provider (IdP) for Strata v2. It acts as the single source of truth for user authentication across both the local Textual TUI and the Next.js web dashboard.

---

## 1. Architecture & Security Boundaries

```
                 ┌─────────────────────────────┐
                 │        Keycloak IdP         │
                 │    (realm: strata-dev)      │
                 └──────────────┬──────────────┘
                                │
        ┌───────────────────────┴───────────────────────┐
        │ OIDC Device Code Grant (RFC 8628)             │ OIDC Auth Code + PKCE (RFC 7636)
        ▼                                               ▼
┌───────────────┐                               ┌───────────────┐
│  Strata TUI   │                               │  Strata Web   │
│ (strata-tui)  │                               │ (strata-web)  │
└───────┬───────┘                               └───────┬───────┘
        │ Bearer JWT (access_token)                     │ Bearer JWT (access_token)
        ▼                                               ▼
┌───────────────────────────────────────────────────────────────┐
│                    Go Orchestrator API                        │
│             (validates JWT via in-memory JWKS)                │
└───────────────────────────────────────────────────────────────┘
```

---

## 2. Authentication Flows

### A. TUI: Device Authorization Flow (`strata-tui`)

Designed for CLI/TUI surfaces where opening a direct browser callback listener on localhost is unergonomic or blocked by remote SSH environments.

1. **Device Code Request**:
   TUI sends `POST /protocol/openid-connect/auth/device` with `client_id: strata-tui` and `scope: openid strata-profile email`.
2. **User Prompt**:
   Keycloak returns `{ device_code, user_code: "ABCD-1234", verification_uri: "http://localhost:8081/realms/strata-dev/device" }`.
3. **User Approval**:
   The user opens the verification URL in any browser, enters the 8-character code, logs in, and approves the terminal device.
4. **Token Polling**:
   The TUI periodically polls `POST /protocol/openid-connect/token` with `grant_type: urn:ietf:params:oauth:grant-type:device_code` until Keycloak issues the tokens.
5. **Token Storage**:
   The resulting `access_token` and `refresh_token` are cached in `~/.config/strata/tokens.json` (or OS keyring).

### B. Web: Authorization Code Flow with PKCE (`strata-web`)

Designed for the Next.js 15 App Router web dashboard.

1. **Authorization Initiation**:
   The web server generates a cryptographically random PKCE pair (`code_verifier` and S256 `code_challenge`), stores them in a short-lived HTTP-only cookie, and redirects the browser to Keycloak `/auth` with `client_id: strata-web`, `response_type: code`, `code_challenge`, and `redirect_uri: /api/auth/callback`.
2. **Login / Signup**:
   The user enters credentials on Keycloak (or registers via the Keycloak registration page).
3. **Token Exchange**:
   Keycloak redirects back to `/api/auth/callback?code=...&state=...`. The route handler verifies the state, retrieves `code_verifier`, and makes a direct server-to-server POST to Keycloak's `/token` endpoint with client secret authentication.
4. **Encrypted Session Cookie**:
   The web application seals the access token and user identity claims into an encrypted, HTTP-only `strata_session` cookie using `jose` (AES-256-GCM).

---

## 3. Realm Configuration (`realm.json`)

The declarative realm definition lives in `backend/helm/strata/keycloak/realm.json` and is imported automatically on startup:

- **Realm Name**: `strata-dev`
- **Registration**: `registrationAllowed: true` enables self-service signup.
- **Clients**:
  - `strata-tui`: Public client, `standardFlowEnabled: false`, `directAccessGrantsEnabled: true`, `oauth2.device.authorization.grant.enabled: true`.
  - `strata-web`: Confidential client, `standardFlowEnabled: true`, `directAccessGrantsEnabled: false`, client secret authentication.
- **Protocol Mappers**:
  - `sub-mapper`: Maps Keycloak user ID property (`id`) to the `sub` claim in both `access_token` and `id_token`.
  - `username-mapper`: Maps Keycloak username property (`username`) to `preferred_username`.

---

## 4. Token Validation in Backend Services

All backend services (Go Orchestrator, FastMCP servers) validate tokens statelessly against Keycloak's JSON Web Key Set (JWKS) endpoint:

- **JWKS Endpoint**: `http://keycloak:8080/realms/strata-dev/protocol/openid-connect/certs`
- **Validator**: Implemented in Go package `backend/services/shared/pkg/jwks`.
  - Caches public keys with thread-safe refresh.
  - Verifies RS256 signature against Keycloak RSA public key.
  - Enforces `iss` (Issuer URL matches Keycloak realm).
  - Enforces `aud` / `azp` (Authorized Party is `strata-tui` or `strata-web`).
  - Enforces token expiration (`exp`) and clock skew bounds.

---

## 5. Local Dev & Testing

- **Keycloak Admin UI**: Accessible at `http://localhost:8081` (credentials: `admin` / `admin`).
- **Default Dev User**: `dev` / `dev` (email: `dev@strata.local`).
- **Device Verification Endpoint**: `http://localhost:8081/realms/strata-dev/device`.