# Next.js Web Dashboard in Strata

Strata's web tier is built with [Next.js 15](https://nextjs.org/) using the **App Router**, **TypeScript**, and **Tailwind CSS**. It serves as the account system of record and read-only cluster viewer.

---

## 1. Responsibilities & Design Principles

| Concern | Web Dashboard Scope |
|---|---|
| **Account System** | User signup and login via Keycloak OIDC (`strata-web` client with PKCE). |
| **Terminal Integration** | Device verification landing page (`/device`) for TUI logins. |
| **Cluster Browser** | Read-only view of registered Kubernetes clusters (`/dashboard`), resource viewers. |
| **Safety Invariant** | **Zero Mutations & No Web Chat.** The TUI is the sole mutating surface and AI agent rail. |

---

## 2. Directory Structure

```
web/
├── app/
│   ├── api/auth/
│   │   ├── login/route.ts       # PKCE challenge + Keycloak redirect
│   │   ├── callback/route.ts    # Code exchange + session cookie issue
│   │   └── logout/route.ts      # Session wipe + Keycloak logout redirect
│   ├── dashboard/page.tsx       # Protected cluster viewer & profile
│   ├── device/page.tsx          # TUI device verification helper
│   ├── login/page.tsx           # Sign-in UI
│   ├── signup/page.tsx          # Keycloak user registration trigger
│   ├── layout.tsx               # Root dark-mode terminal layout
│   ├── page.tsx                 # Landing page
│   └── globals.css              # Tailwind CSS styles
├── lib/
│   ├── auth.ts                  # PKCE generator & Keycloak endpoints
│   ├── session.ts               # Encrypted cookie handling via `jose`
│   └── orchestrator.ts          # Server-side REST client for orchestrator
├── tests/
│   └── auth.test.ts             # Vitest test suite
├── Dockerfile                   # Standalone multi-stage build (<150MB)
└── package.json                 # Next 15 + React 19 + Jose + Tailwind
```

---

## 3. Authentication & Session Architecture

1. **Authorization Code + PKCE**:
   - `/api/auth/login` creates high-entropy `code_verifier` and `code_challenge` (S256).
   - Stores `{ verifier, state }` in a short-lived HTTP-only cookie.
   - Redirects to Keycloak `/protocol/openid-connect/auth`.
2. **Token Exchange**:
   - Keycloak redirects back to `/api/auth/callback`.
   - The route handler validates `state`, extracts `code_verifier`, and requests tokens from Keycloak's token endpoint using the confidential client secret.
3. **Sealed Session Cookie (`strata_session`)**:
   - Uses `jose` with `dir` algorithm and `A256GCM` encryption.
   - Contains `{ accessToken, refreshToken, idToken, user, expiresAt }`.
   - Never exposes raw tokens to client JavaScript (`httpOnly: true`, `sameSite: "lax"`, `secure: true` in prod).

---

## 4. Talking to the Orchestrator

Server components and route handlers communicate directly with the Go Orchestrator REST API:

```typescript
// web/lib/orchestrator.ts
export async function fetchClusters(accessToken: string): Promise<Cluster[]> {
  const res = await fetch(`${getOrchestratorUrl()}/api/v1/clusters/`, {
    headers: {
      Authorization: `Bearer ${accessToken}`,
    },
    cache: "no-store",
  });
  if (!res.ok) return [];
  const data = await res.json();
  return data.clusters || [];
}
```

---

## 5. Development & Testing Workflow

```bash
make web-dev     # start local Next.js dev server on http://localhost:3000
make web-test    # run vitest unit tests
make web-lint    # eslint + typescript typecheck
make web-build   # build standalone production bundle
make web-image   # build Docker container strata-web:dev
```