import { NextRequest, NextResponse } from "next/server";
import { getSession } from "@/lib/session";
import { fetchHistory } from "@/lib/orchestrator";

export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ id: string }> }
) {
  const session = await getSession();
  if (!session) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  const { id } = await params;
  if (!id) {
    return NextResponse.json({ error: "Cluster ID is required" }, { status: 400 });
  }

  const { searchParams } = new URL(request.url);
  const limitParam = searchParams.get("limit");
  const limit = limitParam ? parseInt(limitParam, 10) : undefined;

  const history = await fetchHistory(session.accessToken, id, limit);
  return NextResponse.json({ history });
}
