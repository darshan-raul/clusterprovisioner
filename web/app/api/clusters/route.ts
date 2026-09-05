import { NextRequest, NextResponse } from "next/server";
import { getSession } from "@/lib/session";
import { createCluster } from "@/lib/orchestrator";

export async function POST(request: NextRequest) {
  const session = await getSession();
  if (!session) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  try {
    const body = await request.json();
    if (!body.name || !body.kubeconfig) {
      return NextResponse.json(
        { error: "Name and kubeconfig are required" },
        { status: 400 }
      );
    }
    const cluster = await createCluster(session.accessToken, {
      name: body.name,
      context: body.context,
      kubeconfig: body.kubeconfig,
    });
    return NextResponse.json({ cluster }, { status: 201 });
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : "Failed to create cluster";
    return NextResponse.json({ error: msg }, { status: 500 });
  }
}
