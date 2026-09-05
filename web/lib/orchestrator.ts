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

export interface CreateClusterInput {
  name: string;
  context?: string;
  kubeconfig: string;
}

export async function createCluster(
  accessToken: string,
  input: CreateClusterInput
): Promise<Cluster> {
  const res = await fetch(`${getOrchestratorUrl()}/api/v1/clusters`, {
    method: "POST",
    headers: {
      Authorization: `Bearer ${accessToken}`,
      "Content-Type": "application/json",
    },
    body: JSON.stringify(input),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.error || `HTTP ${res.status}`);
  }
  const data = await res.json();
  return data.cluster;
}

export async function deleteCluster(
  accessToken: string,
  clusterId: string
): Promise<boolean> {
  const res = await fetch(`${getOrchestratorUrl()}/api/v1/clusters/${clusterId}`, {
    method: "DELETE",
    headers: {
      Authorization: `Bearer ${accessToken}`,
      "X-Strata-Client": "web",
    },
  });
  return res.ok;
}

export interface Pod {
  name: string;
  namespace: string;
  node: string | null;
  phase: string;
  ready: string;
  restarts: number;
  age: string;
}

export interface ActionHistoryItem {
  id: string;
  user_id: string;
  cluster_id: string;
  action_type: string;
  target: string;
  status: string;
  details: string;
  client_type: string;
  created_at: string;
}

export async function fetchCluster(
  accessToken: string,
  clusterId: string
): Promise<Cluster | null> {
  const clusters = await fetchClusters(accessToken);
  return clusters.find((c) => c.id === clusterId) || null;
}

export async function fetchPods(
  accessToken: string,
  clusterId: string,
  namespace?: string,
  labelSelector?: string
): Promise<Pod[]> {
  try {
    const params = new URLSearchParams();
    if (namespace) params.set("namespace", namespace);
    if (labelSelector) params.set("label-selector", labelSelector);
    const qs = params.toString() ? `?${params.toString()}` : "";
    const res = await fetch(
      `${getOrchestratorUrl()}/api/v1/clusters/${clusterId}/pods${qs}`,
      {
        headers: {
          Authorization: `Bearer ${accessToken}`,
          "X-Strata-Client": "web",
        },
        cache: "no-store",
      }
    );
    if (!res.ok) return [];
    return (await res.json()) as Pod[];
  } catch {
    return [];
  }
}

export async function fetchHistory(
  accessToken: string,
  clusterId?: string,
  limit?: number
): Promise<ActionHistoryItem[]> {
  try {
    let url: string;
    const params = new URLSearchParams();
    if (limit) params.set("limit", String(limit));
    const qs = params.toString() ? `?${params.toString()}` : "";

    if (clusterId) {
      url = `${getOrchestratorUrl()}/api/v1/clusters/${clusterId}/history${qs}`;
    } else {
      url = `${getOrchestratorUrl()}/api/v1/history${qs}`;
    }

    const res = await fetch(url, {
      headers: {
        Authorization: `Bearer ${accessToken}`,
        "X-Strata-Client": "web",
      },
      cache: "no-store",
    });
    if (!res.ok) return [];
    const data = await res.json();
    return (data.history || []) as ActionHistoryItem[];
  } catch {
    return [];
  }
}
