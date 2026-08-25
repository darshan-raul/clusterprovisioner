import Link from "next/link";
import { Terminal, Shield, Cpu, Key, ArrowRight, CheckCircle2 } from "lucide-react";
import { getSession } from "@/lib/session";

export default async function HomePage() {
  const session = await getSession();

  return (
    <div className="flex flex-col items-center justify-center py-12 gap-16">
      {/* Hero Section */}
      <div className="text-center max-w-3xl flex flex-col items-center gap-6">
        <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-sky-950/80 border border-sky-800/60 text-sky-400 text-xs font-mono">
          <Terminal className="w-3.5 h-3.5" />
          <span>Strata v2 Architecture</span>
        </div>

        <h1 className="text-4xl sm:text-5xl font-extrabold tracking-tight text-white">
          Manage Kubernetes Clusters <br />
          <span className="text-transparent bg-clip-text bg-gradient-to-r from-sky-400 to-teal-300">
            Conversationally & from your Terminal
          </span>
        </h1>

        <p className="text-base sm:text-lg text-slate-400 max-w-2xl">
          A high-performance two-tier system: hands-on Textual TUI on your laptop with BYOK LLM, paired with a remote multi-tenant backend running FastMCP servers and RAG over your clusters.
        </p>

        <div className="flex items-center gap-4 pt-2">
          {session ? (
            <Link
              href="/dashboard"
              className="inline-flex items-center gap-2 px-5 py-2.5 rounded-lg bg-sky-600 hover:bg-sky-500 text-white font-semibold text-sm transition-all shadow-lg shadow-sky-950/50"
            >
              <span>Go to Dashboard</span>
              <ArrowRight className="w-4 h-4" />
            </Link>
          ) : (
            <>
              <Link
                href="/login"
                className="inline-flex items-center gap-2 px-5 py-2.5 rounded-lg bg-sky-600 hover:bg-sky-500 text-white font-semibold text-sm transition-all shadow-lg shadow-sky-950/50"
              >
                <span>Sign In</span>
                <ArrowRight className="w-4 h-4" />
              </Link>
              <Link
                href="/signup"
                className="inline-flex items-center gap-2 px-5 py-2.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-200 font-semibold text-sm border border-slate-700 transition-all"
              >
                <span>Create Account</span>
              </Link>
            </>
          )}
        </div>
      </div>

      {/* Feature Grid */}
      <div className="grid sm:grid-cols-3 gap-6 w-full">
        <div className="p-6 rounded-xl bg-slate-900/60 border border-slate-800 flex flex-col gap-3">
          <div className="w-10 h-10 rounded-lg bg-sky-950 border border-sky-800/60 flex items-center justify-center text-sky-400">
            <Terminal className="w-5 h-5" />
          </div>
          <h3 className="text-lg font-semibold text-white">Interactive TUI</h3>
          <p className="text-sm text-slate-400">
            Full-keyboard k9s-style command palette (<code className="text-xs text-sky-300">:get</code>, <code className="text-xs text-sky-300">:ctx</code>, <code className="text-xs text-sky-300">:logs</code>) with an agent chat rail.
          </p>
        </div>

        <div className="p-6 rounded-xl bg-slate-900/60 border border-slate-800 flex flex-col gap-3">
          <div className="w-10 h-10 rounded-lg bg-teal-950 border border-teal-800/60 flex items-center justify-center text-teal-400">
            <Cpu className="w-5 h-5" />
          </div>
          <h3 className="text-lg font-semibold text-white">FastMCP & RAG</h3>
          <p className="text-sm text-slate-400">
            Remote FastMCP servers interface directly with user clusters using secure encrypted credentials.
          </p>
        </div>

        <div className="p-6 rounded-xl bg-slate-900/60 border border-slate-800 flex flex-col gap-3">
          <div className="w-10 h-10 rounded-lg bg-purple-950 border border-purple-800/60 flex items-center justify-center text-purple-400">
            <Shield className="w-5 h-5" />
          </div>
          <h3 className="text-lg font-semibold text-white">OIDC & Keycloak</h3>
          <p className="text-sm text-slate-400">
            Device-code flow for terminal login plus Auth Code with PKCE on the web dashboard.
          </p>
        </div>
      </div>
    </div>
  );
}
