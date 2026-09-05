import { NextRequest, NextResponse } from "next/server";
import { getSession } from "@/lib/session";
import { fetchHistory } from "@/lib/orchestrator";

export async function GET(request: NextRequest) {
  const session = await getSession();
  if (!session) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  const { searchParams } = new URL(request.url);
  const clusterId = searchParams.get("cluster_id") || undefined;
  const limitParam = searchParams.get("limit");
  const limit = limitParam ? parseInt(limitParam, 10) : undefined;

  const history = await fetchHistory(session.accessToken, clusterId, limit);
  return NextResponse.json({ history });
}
