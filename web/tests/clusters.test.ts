import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { createCluster, deleteCluster, fetchClusters } from "../lib/orchestrator";

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
        headers: { Authorization: "Bearer test-token" },
      })
    );
  });
});
