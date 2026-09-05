import { NextRequest, NextResponse } from "next/server";
import { getSession } from "@/lib/session";
import { deleteCluster } from "@/lib/orchestrator";

export async function DELETE(
  _request: NextRequest,
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

  const ok = await deleteCluster(session.accessToken, id);
  if (!ok) {
    return NextResponse.json({ error: "Failed to delete cluster" }, { status: 500 });
  }

  return NextResponse.json({ status: "deleted", cluster_id: id });
}
