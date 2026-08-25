import { NextRequest, NextResponse } from "next/server";
import { generatePKCE, getAuthorizationUrl } from "@/lib/auth";
import { setPKCECookie } from "@/lib/session";

export async function GET(request: NextRequest) {
  const url = new URL(request.url);
  const prompt = url.searchParams.get("prompt") as "login" | "create" | null;
  const origin =
    process.env.APP_URL ||
    request.headers.get("x-forwarded-host")
      ? `${request.headers.get("x-forwarded-proto") || "http"}://${request.headers.get("x-forwarded-host")}`
      : url.origin;

  const redirectUri = `${origin}/api/auth/callback`;

  const pkce = generatePKCE();
  await setPKCECookie({
    verifier: pkce.verifier,
    state: pkce.state,
  });

  const authUrl = getAuthorizationUrl({
    redirectUri,
    state: pkce.state,
    challenge: pkce.challenge,
    prompt: prompt || undefined,
  });

  return NextResponse.redirect(authUrl);
}
