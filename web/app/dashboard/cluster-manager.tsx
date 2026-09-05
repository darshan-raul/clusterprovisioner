"use client";

import { useState } from "react";
import { Server, Plus, Trash2, CheckCircle2, Lock, Upload, X, AlertCircle, Layers, ExternalLink } from "lucide-react";
import { Cluster } from "@/lib/orchestrator";

export function ClusterManager({ initialClusters }: { initialClusters: Cluster[] }) {
  const [clusters, setClusters] = useState<Cluster[]>(initialClusters);
  const [isOpen, setIsOpen] = useState(false);
  const [name, setName] = useState("");
  const [context, setContext] = useState("");
  const [kubeconfig, setKubeconfig] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [deletingId, setDeletingId] = useState<string | null>(null);

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    if (!name.trim()) {
      setError("Cluster name is required");
      return;
    }
    if (!kubeconfig.trim()) {
      setError("Kubeconfig is required");
      return;
    }

    setLoading(true);
    try {
      const res = await fetch("/api/clusters", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          name: name.trim(),
          context: context.trim() || undefined,
          kubeconfig: kubeconfig.trim(),
        }),
      });

      const data = await res.json();
      if (!res.ok) {
        throw new Error(data.error || "Failed to register cluster");
      }

      setClusters((prev) => [data.cluster, ...prev]);
      setName("");
      setContext("");
      setKubeconfig("");
      setIsOpen(false);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Registration failed");
    } finally {
      setLoading(false);
    }
  }

  async function handleDelete(clusterId: string, clusterName: string) {
    if (!confirm(`Are you sure you want to remove cluster "${clusterName}" (${clusterId})?`)) {
      return;
    }

    setDeletingId(clusterId);
    try {
      const res = await fetch(`/api/clusters/${clusterId}`, {
        method: "DELETE",
      });
      if (res.ok) {
        setClusters((prev) => prev.filter((c) => c.id !== clusterId));
      } else {
        const data = await res.json();
        alert(data.error || "Failed to delete cluster");
      }
    } catch {
      alert("Network error deleting cluster");
    } finally {
      setDeletingId(null);
    }
  }

  function handleFileUpload(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    const reader = new FileReader();
    reader.onload = (event) => {
      const content = event.target?.result as string;
      if (content) {
        setKubeconfig(content);
        if (!name) {
          const base = file.name.replace(/\.(ya?ml|conf|kubeconfig)$/i, "");
          setName(base);
        }
      }
    };
    reader.readAsText(file);
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Server className="w-5 h-5 text-sky-400" />
          <h2 className="text-lg font-bold text-white">Registered Clusters</h2>
          <span className="px-2 py-0.5 rounded-full bg-slate-800 text-slate-300 text-xs font-mono">
            {clusters.length}
          </span>
        </div>
        <button
          onClick={() => {
            setError(null);
            setIsOpen(true);
          }}
          className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-sky-600 hover:bg-sky-500 text-white text-xs font-semibold shadow-md transition-colors"
        >
          <Plus className="w-4 h-4" />
          <span>Add Cluster</span>
        </button>
      </div>

      {/* Clusters Table */}
      {clusters.length === 0 ? (
        <div className="p-12 rounded-2xl bg-slate-900/40 border border-slate-800 border-dashed text-center flex flex-col items-center gap-3">
          <Server className="w-10 h-10 text-slate-600" />
          <h3 className="text-sm font-semibold text-slate-300">No clusters registered yet</h3>
          <p className="text-xs text-slate-500 max-w-sm">
            Add an existing cluster with its kubeconfig to manage it via the Strata TUI.
          </p>
          <button
            onClick={() => setIsOpen(true)}
            className="mt-2 px-4 py-2 rounded-lg bg-sky-600 hover:bg-sky-500 text-white text-xs font-semibold transition-colors"
          >
            Add Your First Cluster
          </button>
        </div>
      ) : (
        <div className="overflow-x-auto rounded-xl border border-slate-800 bg-slate-900/60">
          <table className="w-full text-left text-sm">
            <thead className="bg-slate-950/80 border-b border-slate-800 text-xs font-mono text-slate-400">
              <tr>
                <th className="py-3.5 px-4">CLUSTER ID</th>
                <th className="py-3.5 px-4">NAME</th>
                <th className="py-3.5 px-4">CONTEXT</th>
                <th className="py-3.5 px-4">STATUS</th>
                <th className="py-3.5 px-4">REGISTERED</th>
                <th className="py-3.5 px-4 text-right">ACTIONS</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800/60 font-mono text-xs">
              {clusters.map((cluster) => (
                <tr key={cluster.id} className="hover:bg-slate-800/30 transition-colors">
                  <td className="py-3.5 px-4 text-sky-400 font-semibold">{cluster.id}</td>
                  <td className="py-3.5 px-4">
                    <a
                      href={`/dashboard/clusters/${cluster.id}`}
                      className="text-white hover:text-sky-400 font-sans font-medium inline-flex items-center gap-1.5 transition-colors group"
                      title="Inspect cluster resources"
                    >
                      <span>{cluster.name}</span>
                      <ExternalLink className="w-3 h-3 opacity-0 group-hover:opacity-100 transition-opacity text-sky-400" />
                    </a>
                  </td>
                  <td className="py-3.5 px-4 text-slate-300">{cluster.context}</td>
                  <td className="py-3.5 px-4">
                    <span className="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full bg-emerald-950/80 border border-emerald-800/60 text-emerald-400 text-[11px]">
                      <CheckCircle2 className="w-3 h-3" />
                      <span>Ready</span>
                    </span>
                  </td>
                  <td className="py-3.5 px-4 text-slate-400">
                    {new Date(cluster.created_at).toLocaleDateString()}
                  </td>
                  <td className="py-3.5 px-4 text-right">
                    <div className="flex items-center justify-end gap-2">
                      <a
                        href={`/dashboard/clusters/${cluster.id}`}
                        className="inline-flex items-center gap-1 px-2.5 py-1 rounded bg-slate-800 hover:bg-slate-700 text-sky-400 hover:text-sky-300 text-[11px] font-sans transition-colors"
                        title="Browse Kubernetes resources"
                      >
                        <Layers className="w-3.5 h-3.5" />
                        <span>Browse</span>
                      </a>
                      <button
                        onClick={() => handleDelete(cluster.id, cluster.name)}
                        disabled={deletingId === cluster.id}
                        className="p-1 rounded text-slate-400 hover:text-rose-400 transition-colors"
                        title="Remove cluster"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Add Cluster Modal */}
      {isOpen && (
        <div className="fixed inset-0 z-50 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4">
          <div className="w-full max-w-xl bg-slate-900 border border-slate-800 rounded-2xl p-6 shadow-2xl flex flex-col gap-5">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2.5">
                <div className="w-8 h-8 rounded-lg bg-sky-950 border border-sky-800 flex items-center justify-center text-sky-400">
                  <Server className="w-4 h-4" />
                </div>
                <div>
                  <h3 className="text-base font-bold text-white">Add Kubernetes Cluster</h3>
                  <p className="text-xs text-slate-400">
                    Credentials are encrypted with AES-256-GCM at rest
                  </p>
                </div>
              </div>
              <button
                onClick={() => setIsOpen(false)}
                className="p-1.5 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800 transition-colors"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            {error && (
              <div className="p-3 rounded-xl bg-rose-950/80 border border-rose-800/80 text-rose-300 text-xs flex items-center gap-2">
                <AlertCircle className="w-4 h-4 shrink-0" />
                <span>{error}</span>
              </div>
            )}

            <form onSubmit={handleCreate} className="flex flex-col gap-4">
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <div>
                  <label className="block text-xs font-semibold text-slate-300 mb-1">
                    Cluster Name <span className="text-rose-400">*</span>
                  </label>
                  <input
                    type="text"
                    required
                    value={name}
                    onChange={(e) => setName(e.target.value)}
                    placeholder="e.g. dev-cluster"
                    className="w-full px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-white text-xs placeholder-slate-600 focus:outline-none focus:border-sky-500"
                  />
                </div>
                <div>
                  <label className="block text-xs font-semibold text-slate-300 mb-1">
                    Context Name (optional)
                  </label>
                  <input
                    type="text"
                    value={context}
                    onChange={(e) => setContext(e.target.value)}
                    placeholder="Defaults from kubeconfig"
                    className="w-full px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-white text-xs placeholder-slate-600 focus:outline-none focus:border-sky-500"
                  />
                </div>
              </div>

              <div>
                <div className="flex items-center justify-between mb-1">
                  <label className="text-xs font-semibold text-slate-300">
                    Kubeconfig YAML <span className="text-rose-400">*</span>
                  </label>
                  <label className="flex items-center gap-1 text-[11px] text-sky-400 hover:text-sky-300 cursor-pointer">
                    <Upload className="w-3.5 h-3.5" />
                    <span>Upload file</span>
                    <input
                      type="file"
                      accept=".yaml,.yml,.conf,kubeconfig"
                      className="hidden"
                      onChange={handleFileUpload}
                    />
                  </label>
                </div>
                <textarea
                  required
                  rows={8}
                  value={kubeconfig}
                  onChange={(e) => setKubeconfig(e.target.value)}
                  placeholder={`apiVersion: v1\nclusters:\n- cluster:\n    server: https://...\n...`}
                  className="w-full p-3 rounded-lg bg-slate-950 border border-slate-800 text-slate-200 font-mono text-xs placeholder-slate-700 focus:outline-none focus:border-sky-500 resize-none"
                />
              </div>

              <div className="flex items-center gap-2 p-3 rounded-xl bg-slate-950/60 border border-slate-800 text-[11px] text-slate-400">
                <Lock className="w-4 h-4 text-emerald-400 shrink-0" />
                <span>
                  Stored with AES-256-GCM. FastMCP decrypts credentials in-memory per request and never writes raw tokens to disk.
                </span>
              </div>

              <div className="flex items-center justify-end gap-3 pt-2">
                <button
                  type="button"
                  onClick={() => setIsOpen(false)}
                  className="px-4 py-2 rounded-lg text-xs font-semibold text-slate-400 hover:text-white hover:bg-slate-800 transition-colors"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={loading}
                  className="px-5 py-2 rounded-lg bg-sky-600 hover:bg-sky-500 text-white text-xs font-semibold shadow-md transition-colors disabled:opacity-50"
                >
                  {loading ? "Registering..." : "Save & Encrypt"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
