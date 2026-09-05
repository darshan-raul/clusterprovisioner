import { NextRequest, NextResponse } from "next/server";
import { getSession } from "@/lib/session";
import { fetchPods } from "@/lib/orchestrator";

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
  const namespace = searchParams.get("namespace") || undefined;
  const labelSelector = searchParams.get("label_selector") || undefined;

  const pods = await fetchPods(session.accessToken, id, namespace, labelSelector);
  return NextResponse.json({ pods });
}
