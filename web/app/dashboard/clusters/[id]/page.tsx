import { redirect } from "next/navigation";
import { getSession } from "@/lib/session";
import { fetchCluster, fetchPods, fetchHistory } from "@/lib/orchestrator";
import { ClusterDetailClient } from "./cluster-detail-client";

export default async function ClusterDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const session = await getSession();
  if (!session) {
    redirect("/login");
  }

  const { id } = await params;
  if (!id) {
    redirect("/dashboard");
  }

  const cluster = await fetchCluster(session.accessToken, id);
  if (!cluster) {
    redirect("/dashboard");
  }

  const [pods, history] = await Promise.all([
    fetchPods(session.accessToken, id),
    fetchHistory(session.accessToken, id, 50),
  ]);

  return (
    <ClusterDetailClient
      cluster={cluster}
      initialPods={pods}
      history={history}
    />
  );
}
