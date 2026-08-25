# handoff

> Live cross-session state for Strata v2. The last agent to leave
> the repo updates this file before signing off. New agents read
> AGENTS.md first, then this file.

---

## Current phase

**Phase 3 — Mutation tools + confirmation.** (queued)

Phase 2 (OIDC + Next.js Web Signup/Login, Device Flow, and Gateway Integration) is complete. The Next.js 15 App Router web application is deployed in Helm, integrated with Keycloak OIDC via PKCE, protected with AES-GCM sealed session cookies, and tested end-to-end in kind.

---

## What's done in Phase 2

- [x] **Keycloak OIDC & Realm Setup:**
  - `backend/helm/strata/keycloak/realm.json`: Enabled self-service user registration (`registrationAllowed: true`), configured `strata-web` client (confidential, PKCE S256, redirect URIs, claim mappers for `sub` and `preferred_username`).
  - Added `strata-profile` scope and mapped user claims for both TUI and web clients.
- [x] **Next.js 15 App Router Web Application (`web/`):**
  - Next.js 15.2, React 19, TypeScript, Tailwind CSS v4, Vitest, Jose (JWT/JWE).
  - Multi-stage standalone `Dockerfile` producing minimal production images (<150MB).
  - `web/lib/auth.ts`: High-entropy PKCE S256 verifier/challenge generation, Keycloak authorization URL builder with `prompt=create` support for signup, token exchange client.
  - `web/lib/session.ts`: Encrypted session cookie management (`strata_session`) using `jose` AES-256-GCM sealed payloads.
  - `web/lib/orchestrator.ts`: Server-side REST client communicating with orchestrator using the user's bearer access token.
  - Route handlers: `/api/auth/login`, `/api/auth/callback`, `/api/auth/logout`.
  - Pages: `/` (Landing & Architecture), `/login` & `/signup` (Keycloak auth initiation), `/device` (TUI device approval helper), `/dashboard` (Authenticated user profile & cluster viewer).
- [x] **Helm & Gateway Integration:**
  - `backend/helm/strata/templates/50-web.yaml`: Deployment and Service (`svc/web:3000`).
  - `backend/helm/strata/templates/99-gateway.yaml`: HTTPRoute routing `web.strata.local` to `svc/web:3000`.
  - `backend/helm/strata/values.yaml`: Configured web environment variables for in-cluster and public Keycloak & Orchestrator URLs.
- [x] **TUI Device-Code Scope Alignment:**
  - `tui/strata_tui/api/auth.py`: Updated device code request scope to `openid strata-profile email`.
- [x] **Documentation & CI:**
  - `docs/keycloak.md`: Full reference architecture for OIDC flows (Device Flow RFC 8628, PKCE RFC 7636, JWKS validation).
  - `docs/nextjs.md`: Full reference guide for web tier architecture, auth cookies, server components, and testing.
  - `.github/workflows/web.yml`: Added full CI workflow (pnpm setup, lint, typecheck, unit tests, standalone build).
  - `Makefile`: Added `web-dev`, `web-test`, `web-lint`, `web-build`, `web-image` targets.
- [x] **Automated E2E Validation (`make e2e`):**
  - Builds and loads `strata-web:dev`, `strata-orchestrator:dev`, `strata-mcp-k8s:dev` into `kind`.
  - Verifies web landing page, `/api/auth/login` PKCE redirect, unauthenticated `/dashboard` redirect to `/login`, Keycloak realm, OIDC dev user token minting, orchestrator `/api/v1/me` & `/api/v1/clusters/` validation, FastMCP pod listing, and rolling updates.
  - 100% test pass rate across all tiers: Web Vitest (5 tests), TUI Pytest (21 tests), Backend Go tests, FastMCP Pytest (4 tests).

---

## What's next (Phase 3)

See [AGENTS.md §5](AGENTS.md#5-build-phases).

- **MCP Kubernetes Mutation Tools:** Add `delete_pod`, `apply_manifest`, `exec_command` to `backend/mcp-servers/k8s`.
- **TUI Mutation Confirmation Modal:** Implement confirmation dialog in TUI before dispatching mutating commands (`:delete`, `:apply`).
- **LangGraph Confirmation Interrupts:** Add LangGraph human-in-the-loop `interrupt()` for backend agent mutations.
- **E2E Validation:** Verify safe mutation workflows with confirmation.

---

## Open questions / decisions

1. **Envoy Gateway + Auth:** Envoy Gateway ext-authz vs Envoy OIDC filter for routing. (Using Envoy Gateway HTTPRoutes with Keycloak OIDC PKCE at web layer + JWKS validation at orchestrator).
2. **KMS-wrapped DEK:** Single-key AES-GCM for kind dev; AWS KMS envelope encryption for Phase 8.

---

## Session log

### Session 3 — Phase 2 OIDC, Next.js Web Signup/Login, Device Flow, and Gateway Integration

- Scaffolded Next.js 15 App Router web application in `web/` with TypeScript, Tailwind v4, Jose, and Vitest.
- Implemented OIDC PKCE authentication, encrypted session cookies, and route handlers.
- Created responsive dark-mode UI for landing, login, signup, device verification helper, and cluster dashboard.
- Configured Keycloak `realm.json` with self-service registration and `strata-web` client with claim mappers.
- Created Kubernetes Deployment, Service, and Gateway HTTPRoute for `strata-web`.
- Expanded `docs/keycloak.md` and `docs/nextjs.md` with full reference documentation.
- Updated `scripts/e2e.sh` and verified entire stack in `kind` with `make e2e`.
- All linters, unit tests, and e2e integration checks passing with 100% success. Phase 2 closed.
