import { NextRequest, NextResponse } from "next/server";
import { exchangeCodeForTokens, decodeJwtClaims } from "@/lib/auth";
import { getAndClearPKCECookie, setSessionCookie } from "@/lib/session";

export async function GET(request: NextRequest) {
  const url = new URL(request.url);
  const code = url.searchParams.get("code");
  const state = url.searchParams.get("state");
  const error = url.searchParams.get("error");
  const errorDescription = url.searchParams.get("error_description");

  const origin =
    process.env.APP_URL ||
    request.headers.get("x-forwarded-host")
      ? `${request.headers.get("x-forwarded-proto") || "http"}://${request.headers.get("x-forwarded-host")}`
      : url.origin;

  if (error) {
    return NextResponse.redirect(
      `${origin}/login?error=${encodeURIComponent(errorDescription || error)}`
    );
  }

  if (!code || !state) {
    return NextResponse.redirect(`${origin}/login?error=missing_code_or_state`);
  }

  const pkceData = await getAndClearPKCECookie();
  if (!pkceData || pkceData.state !== state) {
    return NextResponse.redirect(`${origin}/login?error=invalid_state`);
  }

  const redirectUri = `${origin}/api/auth/callback`;

  try {
    const tokens = await exchangeCodeForTokens({
      code,
      verifier: pkceData.verifier,
      redirectUri,
    });

    const claims = decodeJwtClaims(tokens.access_token);
    if (!claims || !claims.sub) {
      return NextResponse.redirect(`${origin}/login?error=invalid_token_claims`);
    }

    const expiresAt = Math.floor(Date.now() / 1000) + tokens.expires_in;

    await setSessionCookie({
      accessToken: tokens.access_token,
      refreshToken: tokens.refresh_token,
      idToken: tokens.id_token,
      user: {
        id: claims.sub,
        username: claims.preferred_username,
        email: claims.email,
        name: claims.name,
      },
      expiresAt,
    });

    return NextResponse.redirect(`${origin}/dashboard`);
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : "Token exchange error";
    return NextResponse.redirect(
      `${origin}/login?error=${encodeURIComponent(msg)}`
    );
  }
}
