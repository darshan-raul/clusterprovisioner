export interface Cluster {
  id: string;
  user_id: string;
  name: string;
  context: string;
  created_at: string;
  kubeconfig_path?: string;
}

export interface ClustersResponse {
  clusters: Cluster[] | null;
}

export interface UserMeResponse {
  sub: string;
  email: string;
  name: string;
  preferred_username: string;
  aud: string[] | null;
}

function getOrchestratorUrl(): string {
  return (
    process.env.ORCHESTRATOR_URL ||
    process.env.NEXT_PUBLIC_ORCHESTRATOR_URL ||
    "http://localhost:8080"
  ).replace(/\/$/, "");
}

export async function fetchUserMe(accessToken: string): Promise<UserMeResponse | null> {
  try {
    const res = await fetch(`${getOrchestratorUrl()}/api/v1/me`, {
      headers: {
        Authorization: `Bearer ${accessToken}`,
      },
      cache: "no-store",
    });
    if (!res.ok) return null;
    return (await res.json()) as UserMeResponse;
  } catch {
    return null;
  }
}

export async function fetchClusters(accessToken: string): Promise<Cluster[]> {
  try {
    const res = await fetch(`${getOrchestratorUrl()}/api/v1/clusters/`, {
      headers: {
        Authorization: `Bearer ${accessToken}`,
      },
      cache: "no-store",
    });
    if (!res.ok) return [];
    const data = (await res.json()) as ClustersResponse;
    return data.clusters || [];
  } catch {
    return [];
  }
}
