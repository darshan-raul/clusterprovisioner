import { EncryptJWT, jwtDecrypt } from "jose";
import { cookies } from "next/headers";
import crypto from "crypto";

export interface SessionData {
  accessToken: string;
  refreshToken?: string;
  idToken?: string;
  user: {
    id: string;
    username: string;
    email?: string;
    name?: string;
  };
  expiresAt: number;
}

const SESSION_COOKIE_NAME = "strata_session";
const PKCE_COOKIE_NAME = "strata_pkce";

function getEncryptionKey(): Uint8Array {
  const secret =
    process.env.SESSION_SECRET ||
    "strata-dev-secret-key-must-be-at-least-32-chars-long";
  return crypto.createHash("sha256").update(secret).digest();
}

export async function sealSession(data: SessionData): Promise<string> {
  const key = getEncryptionKey();
  return new EncryptJWT({ ...data })
    .setProtectedHeader({ alg: "dir", enc: "A256GCM" })
    .setIssuedAt()
    .setExpirationTime(data.expiresAt)
    .encrypt(key);
}

export async function unsealSession(token: string): Promise<SessionData | null> {
  try {
    const key = getEncryptionKey();
    const { payload } = await jwtDecrypt(token, key);
    return payload as unknown as SessionData;
  } catch {
    return null;
  }
}

export async function getSession(): Promise<SessionData | null> {
  const cookieStore = await cookies();
  const token = cookieStore.get(SESSION_COOKIE_NAME)?.value;
  if (!token) return null;
  return unsealSession(token);
}

export async function setSessionCookie(data: SessionData): Promise<void> {
  const cookieStore = await cookies();
  const sealed = await sealSession(data);
  cookieStore.set(SESSION_COOKIE_NAME, sealed, {
    httpOnly: true,
    secure: process.env.NODE_ENV === "production",
    sameSite: "lax",
    path: "/",
    maxAge: 60 * 60 * 24 * 7, // 7 days
  });
}

export async function clearSessionCookie(): Promise<void> {
  const cookieStore = await cookies();
  cookieStore.delete(SESSION_COOKIE_NAME);
}

export async function setPKCECookie(data: { verifier: string; state: string }): Promise<void> {
  const cookieStore = await cookies();
  const serialized = JSON.stringify(data);
  cookieStore.set(PKCE_COOKIE_NAME, serialized, {
    httpOnly: true,
    secure: process.env.NODE_ENV === "production",
    sameSite: "lax",
    path: "/",
    maxAge: 600, // 10 minutes
  });
}

export async function getAndClearPKCECookie(): Promise<{ verifier: string; state: string } | null> {
  const cookieStore = await cookies();
  const raw = cookieStore.get(PKCE_COOKIE_NAME)?.value;
  if (!raw) return null;
  cookieStore.delete(PKCE_COOKIE_NAME);
  try {
    return JSON.parse(raw);
  } catch {
    return null;
  }
}
