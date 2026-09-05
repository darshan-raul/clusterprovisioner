"use client";

import { useState, useMemo } from "react";
import {
  ArrowLeft,
  Server,
  Layers,
  Activity,
  RefreshCw,
  Search,
  CheckCircle2,
  AlertTriangle,
  XCircle,
  Clock,
  Terminal,
  X,
  Info,
  ShieldCheck,
} from "lucide-react";
import { Cluster, Pod, ActionHistoryItem } from "@/lib/orchestrator";
import { HistoryFeed } from "@/app/dashboard/history-feed";

interface ClusterDetailClientProps {
  cluster: Cluster;
  initialPods: Pod[];
  history: ActionHistoryItem[];
}

export function ClusterDetailClient({
  cluster,
  initialPods,
  history,
}: ClusterDetailClientProps) {
  const [activeTab, setActiveTab] = useState<"pods" | "history">("pods");
  const [pods, setPods] = useState<Pod[]>(initialPods);
  const [selectedPod, setSelectedPod] = useState<Pod | null>(null);
  const [searchQuery, setSearchQuery] = useState("");
  const [namespaceFilter, setNamespaceFilter] = useState("");
  const [statusFilter, setStatusFilter] = useState<string>("ALL");
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [refreshError, setRefreshError] = useState<string | null>(null);

  async function handleRefresh() {
    setIsRefreshing(true);
    setRefreshError(null);
    try {
      const params = new URLSearchParams();
      if (namespaceFilter.trim()) {
        params.set("namespace", namespaceFilter.trim());
      }
      const qs = params.toString() ? `?${params.toString()}` : "";
      const res = await fetch(`/api/clusters/${cluster.id}/pods${qs}`);
      if (!res.ok) {
        throw new Error(`Failed to refresh pods (HTTP ${res.status})`);
      }
      const data = await res.json();
      setPods(data.pods || []);
    } catch (err: unknown) {
      setRefreshError(err instanceof Error ? err.message : "Refresh failed");
    } finally {
      setIsRefreshing(false);
    }
  }

  const filteredPods = useMemo(() => {
    return pods.filter((pod) => {
      // Search query filter
      if (searchQuery.trim()) {
        const q = searchQuery.toLowerCase();
        const matchName = pod.name.toLowerCase().includes(q);
        const matchNs = pod.namespace.toLowerCase().includes(q);
        const matchNode = pod.node ? pod.node.toLowerCase().includes(q) : false;
        if (!matchName && !matchNs && !matchNode) return false;
      }
      // Status filter
      if (statusFilter !== "ALL") {
        const phase = pod.phase.toUpperCase();
        if (statusFilter === "RUNNING" && phase !== "RUNNING") return false;
        if (statusFilter === "PENDING" && phase !== "PENDING") return false;
        if (
          statusFilter === "FAILED" &&
          phase !== "FAILED" &&
          phase !== "CRASHLOOPBACKOFF" &&
          phase !== "ERROR"
        )
          return false;
      }
      return true;
    });
  }, [pods, searchQuery, statusFilter]);

  const stats = useMemo(() => {
    const total = pods.length;
    let running = 0;
    let pending = 0;
    let failed = 0;
    for (const p of pods) {
      const phase = p.phase.toLowerCase();
      if (phase === "running") running++;
      else if (phase === "pending") pending++;
      else if (phase === "failed" || phase.includes("crash") || phase === "error") failed++;
    }
    return { total, running, pending, failed };
  }, [pods]);

  function getStatusBadge(phase: string) {
    const p = phase.toLowerCase();
    if (p === "running") {
      return (
        <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full bg-emerald-950/80 border border-emerald-800/60 text-emerald-400 text-[11px]">
          <CheckCircle2 className="w-3 h-3" />
          <span>Running</span>
        </span>
      );
    }
    if (p === "pending") {
      return (
        <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full bg-amber-950/80 border border-amber-800/60 text-amber-400 text-[11px]">
          <Clock className="w-3 h-3" />
          <span>Pending</span>
        </span>
      );
    }
    if (p === "succeeded" || p === "completed") {
      return (
        <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full bg-sky-950/80 border border-sky-800/60 text-sky-400 text-[11px]">
          <CheckCircle2 className="w-3 h-3" />
          <span>Completed</span>
        </span>
      );
    }
    return (
      <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full bg-rose-950/80 border border-rose-800/60 text-rose-400 text-[11px]">
        <AlertTriangle className="w-3 h-3" />
        <span>{phase}</span>
      </span>
    );
  }

  return (
    <div className="flex flex-col gap-6">
      {/* Back Link & Header */}
      <div className="flex flex-col gap-4">
        <a
          href="/dashboard"
          className="inline-flex items-center gap-1.5 text-xs text-slate-400 hover:text-white transition-colors w-fit font-mono"
        >
          <ArrowLeft className="w-4 h-4" />
          <span>Back to Clusters</span>
        </a>

        <div className="p-6 rounded-2xl bg-slate-900/80 border border-slate-800 flex flex-col md:flex-row md:items-center justify-between gap-4 shadow-xl">
          <div className="flex items-center gap-4">
            <div className="w-12 h-12 rounded-xl bg-sky-950 border border-sky-700/60 flex items-center justify-center text-sky-400">
              <Server className="w-6 h-6" />
            </div>
            <div>
              <div className="flex items-center gap-3">
                <h1 className="text-xl font-bold text-white">{cluster.name}</h1>
                <span className="px-2 py-0.5 rounded-full bg-emerald-950 border border-emerald-800/80 text-emerald-400 text-[10px] font-mono">
                  READY
                </span>
              </div>
              <p className="text-xs text-slate-400 font-mono mt-0.5">
                ID: <span className="text-sky-400">{cluster.id}</span> • Context:{" "}
                <span className="text-slate-300">{cluster.context}</span> • Registered:{" "}
                {new Date(cluster.created_at).toLocaleDateString()}
              </p>
            </div>
          </div>

          <div className="flex items-center gap-2">
            <button
              onClick={handleRefresh}
              disabled={isRefreshing}
              className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold border border-slate-700 transition-colors disabled:opacity-50"
            >
              <RefreshCw className={`w-3.5 h-3.5 ${isRefreshing ? "animate-spin" : ""}`} />
              <span>{isRefreshing ? "Refreshing..." : "Refresh"}</span>
            </button>
          </div>
        </div>
      </div>

      {/* Read-Only Safety Banner */}
      <div className="p-3.5 rounded-xl bg-slate-950/70 border border-slate-800 flex items-center gap-3 text-xs text-slate-400">
        <ShieldCheck className="w-4 h-4 text-sky-400 shrink-0" />
        <div>
          <span className="font-semibold text-slate-200">Read-Only Resource Browser: </span>
          Cluster workloads are inspected safely without mutation privileges. All mutations (apply,
          delete, exec) are gated by confirmation modals in the Strata TUI.
        </div>
      </div>

      {/* Navigation Tabs */}
      <div className="flex items-center gap-2 border-b border-slate-800 pb-2">
        <button
          onClick={() => setActiveTab("pods")}
          className={`flex items-center gap-2 px-4 py-2 rounded-lg text-xs font-semibold transition-colors ${
            activeTab === "pods"
              ? "bg-sky-950 border border-sky-800 text-sky-300"
              : "text-slate-400 hover:text-white hover:bg-slate-800/50"
          }`}
        >
          <Layers className="w-4 h-4" />
          <span>Pods & Workloads</span>
          <span className="px-1.5 py-0.2 rounded-full bg-slate-800 text-[10px] font-mono">
            {pods.length}
          </span>
        </button>
        <button
          onClick={() => setActiveTab("history")}
          className={`flex items-center gap-2 px-4 py-2 rounded-lg text-xs font-semibold transition-colors ${
            activeTab === "history"
              ? "bg-sky-950 border border-sky-800 text-sky-300"
              : "text-slate-400 hover:text-white hover:bg-slate-800/50"
          }`}
        >
          <Activity className="w-4 h-4" />
          <span>Cluster Audit History</span>
          <span className="px-1.5 py-0.2 rounded-full bg-slate-800 text-[10px] font-mono">
            {history.length}
          </span>
        </button>
      </div>

      {/* TAB CONTENT: PODS */}
      {activeTab === "pods" && (
        <div className="flex flex-col gap-4">
          {/* Quick Metrics Bar */}
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
            <div className="p-3 rounded-xl bg-slate-900/60 border border-slate-800 flex flex-col gap-1">
              <span className="text-[11px] font-mono text-slate-400 uppercase">Total Pods</span>
              <span className="text-lg font-bold text-white font-mono">{stats.total}</span>
            </div>
            <div className="p-3 rounded-xl bg-slate-900/60 border border-slate-800 flex flex-col gap-1">
              <span className="text-[11px] font-mono text-emerald-400 uppercase">Running</span>
              <span className="text-lg font-bold text-emerald-300 font-mono">{stats.running}</span>
            </div>
            <div className="p-3 rounded-xl bg-slate-900/60 border border-slate-800 flex flex-col gap-1">
              <span className="text-[11px] font-mono text-amber-400 uppercase">Pending</span>
              <span className="text-lg font-bold text-amber-300 font-mono">{stats.pending}</span>
            </div>
            <div className="p-3 rounded-xl bg-slate-900/60 border border-slate-800 flex flex-col gap-1">
              <span className="text-[11px] font-mono text-rose-400 uppercase">Failed / Issues</span>
              <span className="text-lg font-bold text-rose-300 font-mono">{stats.failed}</span>
            </div>
          </div>

          {/* Search & Filter Controls */}
          <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3 p-3 rounded-xl bg-slate-900/60 border border-slate-800">
            <div className="flex items-center gap-2 flex-1 max-w-md">
              <div className="relative w-full">
                <Search className="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-slate-500" />
                <input
                  type="text"
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  placeholder="Filter by pod, namespace, or node..."
                  className="w-full pl-9 pr-3 py-1.5 rounded-lg bg-slate-950 border border-slate-800 text-white text-xs placeholder-slate-600 focus:outline-none focus:border-sky-500"
                />
              </div>
            </div>

            <div className="flex items-center gap-2">
              <input
                type="text"
                value={namespaceFilter}
                onChange={(e) => setNamespaceFilter(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") handleRefresh();
                }}
                placeholder="Namespace (e.g. default)"
                className="px-3 py-1.5 rounded-lg bg-slate-950 border border-slate-800 text-white text-xs placeholder-slate-600 focus:outline-none focus:border-sky-500 w-36"
              />

              <select
                value={statusFilter}
                onChange={(e) => setStatusFilter(e.target.value)}
                className="px-3 py-1.5 rounded-lg bg-slate-950 border border-slate-800 text-white text-xs focus:outline-none focus:border-sky-500"
              >
                <option value="ALL">All Statuses</option>
                <option value="RUNNING">Running</option>
                <option value="PENDING">Pending</option>
                <option value="FAILED">Failed</option>
              </select>
            </div>
          </div>

          {refreshError && (
            <div className="p-3 rounded-xl bg-rose-950/80 border border-rose-800/80 text-rose-300 text-xs flex items-center gap-2">
              <AlertTriangle className="w-4 h-4 shrink-0" />
              <span>{refreshError}</span>
            </div>
          )}

          {/* Pods Table */}
          {filteredPods.length === 0 ? (
            <div className="p-12 rounded-2xl bg-slate-900/40 border border-slate-800 border-dashed text-center flex flex-col items-center gap-3">
              <Layers className="w-10 h-10 text-slate-600" />
              <h3 className="text-sm font-semibold text-slate-300">No pods found</h3>
              <p className="text-xs text-slate-500 max-w-sm">
                No pods matched your filter or the namespace has no active workloads.
              </p>
            </div>
          ) : (
            <div className="overflow-x-auto rounded-xl border border-slate-800 bg-slate-900/60 shadow-lg">
              <table className="w-full text-left text-sm">
                <thead className="bg-slate-950/80 border-b border-slate-800 text-xs font-mono text-slate-400">
                  <tr>
                    <th className="py-3 px-4">POD NAME</th>
                    <th className="py-3 px-4">NAMESPACE</th>
                    <th className="py-3 px-4">STATUS</th>
                    <th className="py-3 px-4">READY</th>
                    <th className="py-3 px-4">RESTARTS</th>
                    <th className="py-3 px-4">NODE</th>
                    <th className="py-3 px-4">AGE</th>
                    <th className="py-3 px-4 text-right">ACTION</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-800/60 font-mono text-xs">
                  {filteredPods.map((pod) => (
                    <tr
                      key={`${pod.namespace}-${pod.name}`}
                      onClick={() => setSelectedPod(pod)}
                      className="hover:bg-slate-800/40 cursor-pointer transition-colors"
                    >
                      <td className="py-3 px-4 text-sky-400 font-semibold">{pod.name}</td>
                      <td className="py-3 px-4 text-slate-300">{pod.namespace}</td>
                      <td className="py-3 px-4">{getStatusBadge(pod.phase)}</td>
                      <td className="py-3 px-4 text-slate-300">{pod.ready}</td>
                      <td className="py-3 px-4">
                        <span
                          className={pod.restarts > 0 ? "text-amber-400 font-bold" : "text-slate-400"}
                        >
                          {pod.restarts}
                        </span>
                      </td>
                      <td className="py-3 px-4 text-slate-400">{pod.node || "—"}</td>
                      <td className="py-3 px-4 text-slate-400">{pod.age}</td>
                      <td className="py-3 px-4 text-right">
                        <button
                          onClick={(e) => {
                            e.stopPropagation();
                            setSelectedPod(pod);
                          }}
                          className="px-2 py-1 rounded bg-slate-800 hover:bg-slate-700 text-slate-300 hover:text-white text-[11px] font-sans transition-colors"
                        >
                          Inspect
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      )}

      {/* TAB CONTENT: HISTORY */}
      {activeTab === "history" && (
        <HistoryFeed
          history={history}
          title={`Cluster Actions (${cluster.name})`}
          subtitle={`Audit log of all mutation and execution events recorded for ${cluster.id}`}
        />
      )}

      {/* POD INSPECT MODAL */}
      {selectedPod && (
        <div className="fixed inset-0 z-50 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4">
          <div className="w-full max-w-lg bg-slate-900 border border-slate-800 rounded-2xl p-6 shadow-2xl flex flex-col gap-5">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2.5">
                <div className="w-8 h-8 rounded-lg bg-sky-950 border border-sky-800 flex items-center justify-center text-sky-400">
                  <Layers className="w-4 h-4" />
                </div>
                <div>
                  <h3 className="text-base font-bold text-white">{selectedPod.name}</h3>
                  <p className="text-xs text-slate-400 font-mono">
                    Namespace: {selectedPod.namespace}
                  </p>
                </div>
              </div>
              <button
                onClick={() => setSelectedPod(null)}
                className="p-1.5 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800 transition-colors"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            <div className="grid grid-cols-2 gap-3 p-4 rounded-xl bg-slate-950/80 border border-slate-800 font-mono text-xs">
              <div>
                <span className="text-slate-500 block text-[10px] uppercase">Phase</span>
                <span className="text-white font-semibold">{selectedPod.phase}</span>
              </div>
              <div>
                <span className="text-slate-500 block text-[10px] uppercase">Ready Containers</span>
                <span className="text-white font-semibold">{selectedPod.ready}</span>
              </div>
              <div>
                <span className="text-slate-500 block text-[10px] uppercase">Restarts</span>
                <span
                  className={`font-semibold ${
                    selectedPod.restarts > 0 ? "text-amber-400" : "text-white"
                  }`}
                >
                  {selectedPod.restarts}
                </span>
              </div>
              <div>
                <span className="text-slate-500 block text-[10px] uppercase">Age</span>
                <span className="text-white font-semibold">{selectedPod.age}</span>
              </div>
              <div className="col-span-2">
                <span className="text-slate-500 block text-[10px] uppercase">Node Placement</span>
                <span className="text-slate-300 font-semibold">{selectedPod.node || "Unassigned"}</span>
              </div>
            </div>

            {/* Quick TUI commands for this pod */}
            <div className="p-4 rounded-xl bg-slate-950 border border-slate-800 flex flex-col gap-2">
              <div className="flex items-center gap-1.5 text-xs text-sky-400 font-mono font-semibold">
                <Terminal className="w-3.5 h-3.5" />
                <span>Strata TUI Commands for this Pod</span>
              </div>
              <p className="text-[11px] text-slate-400">
                To interact, tail logs, or mutate this pod, run in your local terminal:
              </p>
              <div className="font-mono text-xs text-slate-300 space-y-1 bg-slate-900/90 p-2.5 rounded-lg border border-slate-800">
                <p>
                  <span className="text-slate-500">:logs </span>
                  <span className="text-sky-300">{selectedPod.name}</span>
                  <span className="text-slate-400"> -n {selectedPod.namespace}</span>
                </p>
                <p>
                  <span className="text-slate-500">:delete pod </span>
                  <span className="text-rose-400">{selectedPod.name}</span>
                  <span className="text-slate-400"> -n {selectedPod.namespace}</span>
                </p>
                <p>
                  <span className="text-slate-500">:exec </span>
                  <span className="text-sky-300">{selectedPod.name}</span>
                  <span className="text-slate-400"> -n {selectedPod.namespace} -- /bin/sh</span>
                </p>
              </div>
            </div>

            <div className="flex justify-end">
              <button
                onClick={() => setSelectedPod(null)}
                className="px-4 py-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-white text-xs font-semibold transition-colors"
              >
                Close
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
