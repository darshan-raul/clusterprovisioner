import crypto from "crypto";

export interface PKCEPair {
  verifier: string;
  challenge: string;
  state: string;
}

export interface TokenResponse {
  access_token: string;
  token_type: string;
  expires_in: number;
  refresh_token?: string;
  id_token?: string;
  scope?: string;
}

export interface UserClaims {
  sub: string;
  email?: string;
  preferred_username: string;
  name?: string;
}

export function getAuthConfig() {
  const publicUrl = (
    process.env.KEYCLOAK_PUBLIC_URL ||
    process.env.NEXT_PUBLIC_KEYCLOAK_URL ||
    "http://localhost:8081"
  ).replace(/\/$/, "");

  const internalUrl = (
    process.env.KEYCLOAK_INTERNAL_URL ||
    process.env.KEYCLOAK_URL ||
    publicUrl
  ).replace(/\/$/, "");

  const realm = process.env.KEYCLOAK_REALM || "strata-dev";
  const clientId = process.env.KEYCLOAK_CLIENT_ID || "strata-web";
  const clientSecret =
    process.env.KEYCLOAK_CLIENT_SECRET ||
    "strata-web-dev-secret-do-not-use-in-prod";

  return {
    publicUrl,
    internalUrl,
    realm,
    clientId,
    clientSecret,
    publicAuthUrl: `${publicUrl}/realms/${realm}/protocol/openid-connect/auth`,
    publicRegistrationUrl: `${publicUrl}/realms/${realm}/protocol/openid-connect/registrations`,
    publicLogoutUrl: `${publicUrl}/realms/${realm}/protocol/openid-connect/logout`,
    internalTokenUrl: `${internalUrl}/realms/${realm}/protocol/openid-connect/token`,
    publicDeviceUrl: `${publicUrl}/realms/${realm}/protocol/openid-connect/auth/device`,
  };
}

export function generatePKCE(): PKCEPair {
  const verifier = crypto.randomBytes(32).toString("base64url");
  const challenge = crypto
    .createHash("sha256")
    .update(verifier)
    .digest("base64url");
  const state = crypto.randomBytes(16).toString("hex");

  return { verifier, challenge, state };
}

export function getAuthorizationUrl(params: {
  redirectUri: string;
  state: string;
  challenge: string;
  prompt?: "login" | "create";
}): string {
  const cfg = getAuthConfig();
  const base =
    params.prompt === "create"
      ? cfg.publicRegistrationUrl
      : cfg.publicAuthUrl;

  const url = new URL(base);
  url.searchParams.set("client_id", cfg.clientId);
  url.searchParams.set("response_type", "code");
  url.searchParams.set("scope", "openid strata-profile email");
  url.searchParams.set("redirect_uri", params.redirectUri);
  url.searchParams.set("state", params.state);
  url.searchParams.set("code_challenge", params.challenge);
  url.searchParams.set("code_challenge_method", "S256");

  return url.toString();
}

export async function exchangeCodeForTokens(params: {
  code: string;
  verifier: string;
  redirectUri: string;
}): Promise<TokenResponse> {
  const cfg = getAuthConfig();
  const body = new URLSearchParams({
    grant_type: "authorization_code",
    client_id: cfg.clientId,
    client_secret: cfg.clientSecret,
    code: params.code,
    code_verifier: params.verifier,
    redirect_uri: params.redirectUri,
  });

  const res = await fetch(cfg.internalTokenUrl, {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body: body.toString(),
  });

  if (!res.ok) {
    const errorText = await res.text();
    throw new Error(`Token exchange failed (${res.status}): ${errorText}`);
  }

  return (await res.json()) as TokenResponse;
}

export function decodeJwtClaims(token: string): UserClaims | null {
  try {
    const parts = token.split(".");
    if (parts.length !== 3) return null;
    const payload = Buffer.from(parts[1], "base64url").toString("utf-8");
    const json = JSON.parse(payload);
    return {
      sub: json.sub || "",
      email: json.email,
      preferred_username: json.preferred_username || json.sub || "user",
      name: json.name,
    };
  } catch {
    return null;
  }
}
