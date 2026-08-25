import { Shield, Terminal, ArrowRight, ExternalLink } from "lucide-react";
import { getAuthConfig } from "@/lib/auth";

export default function DeviceAuthPage() {
  const cfg = getAuthConfig();

  return (
    <div className="flex flex-col items-center justify-center py-12">
      <div className="w-full max-w-lg p-8 rounded-2xl bg-slate-900/80 border border-slate-800 shadow-2xl flex flex-col gap-6">
        <div className="flex items-center gap-3">
          <div className="p-3 rounded-xl bg-sky-950 border border-sky-800/60 text-sky-400">
            <Shield className="w-6 h-6" />
          </div>
          <div>
            <h1 className="text-xl font-bold text-white">TUI Device Verification</h1>
            <p className="text-xs text-slate-400">Authorize your terminal session via OIDC Device Flow</p>
          </div>
        </div>

        <div className="p-4 rounded-xl bg-slate-950 border border-slate-800 flex flex-col gap-3">
          <h2 className="text-sm font-semibold text-slate-200">How to authenticate the TUI:</h2>
          <ol className="list-decimal list-inside text-xs text-slate-400 space-y-2 leading-relaxed">
            <li>Open your terminal and launch Strata TUI (<code className="text-sky-300">strata</code>).</li>
            <li>Type <code className="text-sky-300 font-mono">:login</code> in the command palette.</li>
            <li>A unique 8-character user code will appear (e.g. <code className="text-amber-300 font-mono">ABCD-1234</code>).</li>
            <li>Click the button below to enter your code on Keycloak and grant terminal access.</li>
          </ol>
        </div>

        <a
          href={cfg.publicDeviceUrl}
          target="_blank"
          rel="noopener noreferrer"
          className="w-full py-3 px-4 rounded-xl bg-sky-600 hover:bg-sky-500 text-white font-semibold text-sm flex items-center justify-center gap-2 transition-all shadow-md shadow-sky-950"
        >
          <span>Open Keycloak Device Approval</span>
          <ExternalLink className="w-4 h-4" />
        </a>
      </div>
    </div>
  );
}
