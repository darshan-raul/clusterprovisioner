import { describe, it, expect } from "vitest";
import { generatePKCE, decodeJwtClaims } from "../lib/auth";
import { sealSession, unsealSession, SessionData } from "../lib/session";

describe("PKCE Generation", () => {
  it("generates non-empty verifier, challenge, and state", () => {
    const pkce = generatePKCE();
    expect(pkce.verifier.length).toBeGreaterThan(20);
    expect(pkce.challenge.length).toBeGreaterThan(20);
    expect(pkce.state.length).toBeGreaterThan(10);
    expect(pkce.verifier).not.toEqual(pkce.challenge);
  });
});

describe("JWT Claims Decoding", () => {
  it("extracts sub, email, and preferred_username from payload", () => {
    const payload = Buffer.from(
      JSON.stringify({
        sub: "user-123",
        email: "test@example.com",
        preferred_username: "testuser",
        name: "Test User",
      })
    ).toString("base64url");

    const token = `header.${payload}.signature`;
    const claims = decodeJwtClaims(token);

    expect(claims).toEqual({
      sub: "user-123",
      email: "test@example.com",
      preferred_username: "testuser",
      name: "Test User",
    });
  });

  it("returns null for malformed tokens", () => {
    expect(decodeJwtClaims("invalid")).toBeNull();
    expect(decodeJwtClaims("a.b")).toBeNull();
  });
});

describe("Session Sealing & Unsealing", () => {
  it("seals and unseals session data correctly", async () => {
    const session: SessionData = {
      accessToken: "mock-access-token",
      refreshToken: "mock-refresh-token",
      user: {
        id: "usr-456",
        username: "dev",
        email: "dev@strata.local",
      },
      expiresAt: Math.floor(Date.now() / 1000) + 3600,
    };

    const sealed = await sealSession(session);
    expect(typeof sealed).toBe("string");

    const unsealed = await unsealSession(sealed);
    expect(unsealed).not.toBeNull();
    expect(unsealed?.user.username).toBe("dev");
    expect(unsealed?.user.id).toBe("usr-456");
    expect(unsealed?.accessToken).toBe("mock-access-token");
  });

  it("returns null on tampered token", async () => {
    const unsealed = await unsealSession("tampered.token.value");
    expect(unsealed).toBeNull();
  });
});
