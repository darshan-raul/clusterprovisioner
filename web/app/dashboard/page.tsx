import { redirect } from "next/navigation";
import { Terminal } from "lucide-react";
import { getSession } from "@/lib/session";
import { fetchClusters, fetchUserMe, fetchHistory } from "@/lib/orchestrator";
import { ClusterManager } from "./cluster-manager";
import { HistoryFeed } from "./history-feed";

export default async function DashboardPage() {
  const session = await getSession();
  if (!session) {
    redirect("/login");
  }

  const [clusters, , history] = await Promise.all([
    fetchClusters(session.accessToken),
    fetchUserMe(session.accessToken),
    fetchHistory(session.accessToken, undefined, 25),
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

      {/* Clusters Management Section */}
      <ClusterManager initialClusters={clusters} />

      {/* Audit Trail / History Section */}
      <HistoryFeed history={history} />

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
