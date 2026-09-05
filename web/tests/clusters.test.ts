import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import {
  createCluster,
  deleteCluster,
  fetchClusters,
  fetchCluster,
  fetchPods,
  fetchHistory,
} from "../lib/orchestrator";

describe("Orchestrator Cluster Client", () => {
  const originalFetch = global.fetch;

  beforeEach(() => {
    vi.stubGlobal("fetch", vi.fn());
  });

  afterEach(() => {
    vi.restoreAllMocks();
    global.fetch = originalFetch;
  });

  it("fetchClusters returns list of clusters", async () => {
    const mockClusters = [
      { id: "cl-1", user_id: "u-1", name: "dev", context: "ctx", created_at: "2026-01-01" },
    ];
    vi.mocked(global.fetch).mockResolvedValueOnce({
      ok: true,
      json: async () => ({ clusters: mockClusters }),
    } as Response);

    const res = await fetchClusters("test-token");
    expect(res).toEqual(mockClusters);
    expect(global.fetch).toHaveBeenCalledWith(
      expect.stringContaining("/api/v1/clusters/"),
      expect.objectContaining({
        headers: { Authorization: "Bearer test-token" },
      })
    );
  });

  it("fetchCluster finds cluster by ID", async () => {
    const mockClusters = [
      { id: "cl-1", user_id: "u-1", name: "dev", context: "ctx", created_at: "2026-01-01" },
      { id: "cl-2", user_id: "u-1", name: "prod", context: "ctx2", created_at: "2026-01-02" },
    ];
    vi.mocked(global.fetch).mockResolvedValueOnce({
      ok: true,
      json: async () => ({ clusters: mockClusters }),
    } as Response);

    const res = await fetchCluster("test-token", "cl-2");
    expect(res).toEqual(mockClusters[1]);

    const notFound = await fetchCluster("test-token", "cl-999");
    expect(notFound).toBeNull();
  });

  it("createCluster sends POST and returns created cluster", async () => {
    const created = {
      id: "cl-new",
      user_id: "u-1",
      name: "staging",
      context: "stage-ctx",
      created_at: "2026-01-01",
    };
    vi.mocked(global.fetch).mockResolvedValueOnce({
      ok: true,
      json: async () => ({ cluster: created }),
    } as Response);

    const res = await createCluster("test-token", {
      name: "staging",
      kubeconfig: "apiVersion: v1",
    });
    expect(res).toEqual(created);
    expect(global.fetch).toHaveBeenCalledWith(
      expect.stringContaining("/api/v1/clusters"),
      expect.objectContaining({
        method: "POST",
        headers: {
          Authorization: "Bearer test-token",
          "Content-Type": "application/json",
        },
        body: JSON.stringify({ name: "staging", kubeconfig: "apiVersion: v1" }),
      })
    );
  });

  it("deleteCluster sends DELETE and returns true on success", async () => {
    vi.mocked(global.fetch).mockResolvedValueOnce({
      ok: true,
    } as Response);

    const ok = await deleteCluster("test-token", "cl-1");
    expect(ok).toBe(true);
    expect(global.fetch).toHaveBeenCalledWith(
      expect.stringContaining("/api/v1/clusters/cl-1"),
      expect.objectContaining({
        method: "DELETE",
        headers: {
          Authorization: "Bearer test-token",
          "X-Strata-Client": "web",
        },
      })
    );
  });

  it("fetchPods returns pods from orchestrator", async () => {
    const mockPods = [
      {
        name: "nginx-123",
        namespace: "default",
        node: "node-1",
        phase: "Running",
        ready: "1/1",
        restarts: 0,
        age: "5m",
      },
    ];
    vi.mocked(global.fetch).mockResolvedValueOnce({
      ok: true,
      json: async () => mockPods,
    } as Response);

    const res = await fetchPods("test-token", "cl-1", "default", "app=nginx");
    expect(res).toEqual(mockPods);
    expect(global.fetch).toHaveBeenCalledWith(
      expect.stringContaining("/api/v1/clusters/cl-1/pods?namespace=default&label-selector=app%3Dnginx"),
      expect.objectContaining({
        headers: {
          Authorization: "Bearer test-token",
          "X-Strata-Client": "web",
        },
      })
    );
  });

  it("fetchHistory returns audit history items", async () => {
    const mockHistory = [
      {
        id: "act-1",
        user_id: "u-1",
        cluster_id: "cl-1",
        action_type: "delete_pod",
        target: "default/nginx",
        status: "success",
        details: "",
        client_type: "tui",
        created_at: "2026-01-01T00:00:00Z",
      },
    ];
    vi.mocked(global.fetch).mockResolvedValueOnce({
      ok: true,
      json: async () => ({ history: mockHistory }),
    } as Response);

    const res = await fetchHistory("test-token", "cl-1", 10);
    expect(res).toEqual(mockHistory);
    expect(global.fetch).toHaveBeenCalledWith(
      expect.stringContaining("/api/v1/clusters/cl-1/history?limit=10"),
      expect.objectContaining({
        headers: {
          Authorization: "Bearer test-token",
          "X-Strata-Client": "web",
        },
      })
    );
  });
});
