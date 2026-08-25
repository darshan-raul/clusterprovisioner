import { redirect } from "next/navigation";
import { Layers, Server, Terminal, Shield, CheckCircle2, RefreshCw } from "lucide-react";
import { getSession } from "@/lib/session";
import { fetchClusters, fetchUserMe } from "@/lib/orchestrator";

export default async function DashboardPage() {
  const session = await getSession();
  if (!session) {
    redirect("/login");
  }

  const [clusters, me] = await Promise.all([
    fetchClusters(session.accessToken),
    fetchUserMe(session.accessToken),
  ]);

  return (
    <div className="flex flex-col gap-8">
      {/* Header Profile Card */}
      <div className="p-6 rounded-2xl bg-slate-900/80 border border-slate-800 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4 shadow-xl">
        <div className="flex items-center gap-4">
          <div className="w-12 h-12 rounded-xl bg-sky-950 border border-sky-700/60 flex items-center justify-center text-sky-400 font-mono font-bold text-lg">
            {session.user.username.slice(0, 2).toUpperCase()}
          </div>
          <div>
            <div className="flex items-center gap-2">
              <h1 className="text-xl font-bold text-white">{session.user.username}</h1>
              <span className="px-2 py-0.5 rounded-full bg-emerald-950 border border-emerald-800/80 text-emerald-400 text-[10px] font-mono">
                AUTHENTICATED
              </span>
            </div>
            <p className="text-xs text-slate-400 font-mono mt-0.5">
              {session.user.email || "no-email"} • ID: {session.user.id}
            </p>
          </div>
        </div>

        <div className="flex items-center gap-3">
          <a
            href="/api/auth/logout"
            className="px-4 py-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 hover:text-white text-xs font-semibold border border-slate-700 transition-colors"
          >
            Sign Out
          </a>
        </div>
      </div>

      {/* Clusters Section */}
      <div className="flex flex-col gap-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Server className="w-5 h-5 text-sky-400" />
            <h2 className="text-lg font-bold text-white">Registered Clusters</h2>
            <span className="px-2 py-0.5 rounded-full bg-slate-800 text-slate-300 text-xs font-mono">
              {clusters.length}
            </span>
          </div>
          <span className="text-xs text-slate-500 font-mono">Read-Only Viewer</span>
        </div>

        {clusters.length === 0 ? (
          <div className="p-12 rounded-2xl bg-slate-900/40 border border-slate-800 border-dashed text-center flex flex-col items-center gap-3">
            <Server className="w-10 h-10 text-slate-600" />
            <h3 className="text-sm font-semibold text-slate-300">No clusters registered yet</h3>
            <p className="text-xs text-slate-500 max-w-sm">
              Use the Strata TUI or register existing clusters via the backend registry.
            </p>
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
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800/60 font-mono text-xs">
                {clusters.map((cluster) => (
                  <tr key={cluster.id} className="hover:bg-slate-800/30 transition-colors">
                    <td className="py-3.5 px-4 text-sky-400 font-semibold">{cluster.id}</td>
                    <td className="py-3.5 px-4 text-white">{cluster.name}</td>
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
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* TUI Quickstart card */}
      <div className="p-6 rounded-2xl bg-gradient-to-br from-slate-900/90 to-sky-950/30 border border-sky-900/40 flex flex-col gap-4">
        <div className="flex items-center gap-2 text-sky-400 font-mono text-sm font-semibold">
          <Terminal className="w-4 h-4" />
          <span>Connect via Strata TUI</span>
        </div>
        <p className="text-xs text-slate-400 leading-relaxed">
          The TUI is the primary mutating interface. Run commands directly against your clusters with confirmation safety:
        </p>
        <div className="p-3.5 rounded-xl bg-slate-950 border border-slate-800 font-mono text-xs text-slate-300 space-y-1.5">
          <p><span className="text-slate-500"># Authenticate terminal session</span></p>
          <p className="text-sky-300">:login</p>
          <p><span className="text-slate-500"># Switch active cluster context</span></p>
          <p className="text-sky-300">:ctx use mock-cluster</p>
          <p><span className="text-slate-500"># Query Kubernetes resources</span></p>
          <p className="text-sky-300">:get pods -n default</p>
        </div>
      </div>
    </div>
  );
}
