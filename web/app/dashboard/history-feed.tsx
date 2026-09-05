"use client";

import { Activity, CheckCircle2, XCircle, Terminal, Bot, Globe } from "lucide-react";
import { ActionHistoryItem } from "@/lib/orchestrator";

interface HistoryFeedProps {
  history: ActionHistoryItem[];
  title?: string;
  subtitle?: string;
}

export function HistoryFeed({
  history,
  title = "Audit Trail & Recent Activity",
  subtitle = "Real-time log of actions executed via Strata TUI, LangGraph agent, and Web dashboard",
}: HistoryFeedProps) {
  function getActionBadge(action: string) {
    switch (action) {
      case "delete_pod":
        return "bg-rose-950/80 text-rose-400 border-rose-800/80";
      case "apply_manifest":
        return "bg-emerald-950/80 text-emerald-400 border-emerald-800/80";
      case "exec_command":
        return "bg-purple-950/80 text-purple-400 border-purple-800/80";
      case "create_cluster":
        return "bg-sky-950/80 text-sky-400 border-sky-800/80";
      case "delete_cluster":
        return "bg-orange-950/80 text-orange-400 border-orange-800/80";
      default:
        return "bg-slate-800 text-slate-300 border-slate-700";
    }
  }

  function getClientBadge(client: string) {
    switch (client) {
      case "tui_agent":
        return (
          <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full bg-indigo-950/90 border border-indigo-700/80 text-indigo-300 text-[10px] font-mono">
            <Bot className="w-3 h-3" />
            <span>AGENT</span>
          </span>
        );
      case "tui":
        return (
          <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full bg-sky-950/90 border border-sky-700/80 text-sky-300 text-[10px] font-mono">
            <Terminal className="w-3 h-3" />
            <span>TUI</span>
          </span>
        );
      case "web":
      default:
        return (
          <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full bg-slate-800 border border-slate-700 text-slate-300 text-[10px] font-mono">
            <Globe className="w-3 h-3" />
            <span>WEB</span>
          </span>
        );
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Activity className="w-5 h-5 text-sky-400" />
          <h2 className="text-lg font-bold text-white">{title}</h2>
          <span className="px-2 py-0.5 rounded-full bg-slate-800 text-slate-300 text-xs font-mono">
            {history.length}
          </span>
        </div>
      </div>
      {subtitle && <p className="text-xs text-slate-400 -mt-2">{subtitle}</p>}

      {history.length === 0 ? (
        <div className="p-8 rounded-2xl bg-slate-900/40 border border-slate-800 border-dashed text-center flex flex-col items-center gap-2">
          <Activity className="w-8 h-8 text-slate-600" />
          <p className="text-xs text-slate-400">No actions recorded in the audit trail yet.</p>
          <p className="text-[11px] text-slate-500 font-mono">
            Mutations performed via the Strata TUI or Web will automatically be audited here.
          </p>
        </div>
      ) : (
        <div className="overflow-x-auto rounded-xl border border-slate-800 bg-slate-900/60 shadow-lg">
          <table className="w-full text-left text-sm">
            <thead className="bg-slate-950/80 border-b border-slate-800 text-xs font-mono text-slate-400">
              <tr>
                <th className="py-3 px-4">STATUS</th>
                <th className="py-3 px-4">ACTION</th>
                <th className="py-3 px-4">TARGET</th>
                <th className="py-3 px-4">ORIGIN</th>
                <th className="py-3 px-4">DETAILS</th>
                <th className="py-3 px-4 text-right">TIMESTAMP</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800/60 font-mono text-xs">
              {history.map((item) => (
                <tr key={item.id} className="hover:bg-slate-800/30 transition-colors">
                  <td className="py-3 px-4">
                    {item.status === "success" ? (
                      <span className="inline-flex items-center gap-1 text-emerald-400 font-semibold">
                        <CheckCircle2 className="w-3.5 h-3.5" />
                        <span>OK</span>
                      </span>
                    ) : (
                      <span className="inline-flex items-center gap-1 text-rose-400 font-semibold">
                        <XCircle className="w-3.5 h-3.5" />
                        <span>FAIL</span>
                      </span>
                    )}
                  </td>
                  <td className="py-3 px-4">
                    <span
                      className={`inline-block px-2 py-0.5 rounded border text-[11px] font-semibold ${getActionBadge(
                        item.action_type
                      )}`}
                    >
                      {item.action_type}
                    </span>
                  </td>
                  <td className="py-3 px-4 text-slate-200 font-mono">{item.target}</td>
                  <td className="py-3 px-4">{getClientBadge(item.client_type)}</td>
                  <td className="py-3 px-4 text-slate-400 max-w-xs truncate font-mono text-[11px]">
                    {item.details || "—"}
                  </td>
                  <td className="py-3 px-4 text-right text-slate-500 text-[11px]">
                    {new Date(item.created_at).toLocaleString()}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
