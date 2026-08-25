import { NextRequest, NextResponse } from "next/server";
import { clearSessionCookie, getSession } from "@/lib/session";
import { getAuthConfig } from "@/lib/auth";

export async function GET(request: NextRequest) {
  const session = await getSession();
  await clearSessionCookie();

  const url = new URL(request.url);
  const origin =
    process.env.APP_URL ||
    request.headers.get("x-forwarded-host")
      ? `${request.headers.get("x-forwarded-proto") || "http"}://${request.headers.get("x-forwarded-host")}`
      : url.origin;

  const cfg = getAuthConfig();
  const logoutUrl = new URL(cfg.publicLogoutUrl);
  logoutUrl.searchParams.set("post_logout_redirect_uri", `${origin}/login`);
  if (session?.idToken) {
    logoutUrl.searchParams.set("id_token_hint", session.idToken);
  }

  return NextResponse.redirect(logoutUrl.toString());
}
