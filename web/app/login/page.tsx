import Link from "next/link";
import { redirect } from "next/navigation";
import { Terminal, Shield, ArrowRight, AlertCircle, Key } from "lucide-react";
import { getSession } from "@/lib/session";

export default async function LoginPage({
  searchParams,
}: {
  searchParams: Promise<{ error?: string }>;
}) {
  const session = await getSession();
  if (session) {
    redirect("/dashboard");
  }

  const { error } = await searchParams;

  return (
    <div className="flex flex-col items-center justify-center py-12">
      <div className="w-full max-w-md p-8 rounded-2xl bg-slate-900/80 border border-slate-800 shadow-2xl flex flex-col gap-6">
        <div className="flex flex-col items-center text-center gap-2">
          <div className="p-3 rounded-xl bg-sky-950 border border-sky-800/60 text-sky-400">
            <Terminal className="w-6 h-6" />
          </div>
          <h1 className="text-2xl font-bold text-white tracking-tight">Sign in to Strata</h1>
          <p className="text-sm text-slate-400">
            Authenticate with Keycloak to access your cluster dashboard
          </p>
        </div>

        {error && (
          <div className="p-3 rounded-lg bg-red-950/60 border border-red-800/60 text-red-300 text-xs flex items-center gap-2">
            <AlertCircle className="w-4 h-4 text-red-400 shrink-0" />
            <span>{error}</span>
          </div>
        )}

        <div className="flex flex-col gap-4">
          <a
            href="/api/auth/login"
            className="w-full py-3 px-4 rounded-xl bg-sky-600 hover:bg-sky-500 text-white font-semibold text-sm flex items-center justify-center gap-2 transition-all shadow-md shadow-sky-950"
          >
            <Shield className="w-4 h-4" />
            <span>Continue with Keycloak</span>
            <ArrowRight className="w-4 h-4" />
          </a>

          <div className="relative my-2">
            <div className="absolute inset-0 flex items-center">
              <div className="w-full border-t border-slate-800" />
            </div>
            <div className="relative flex justify-center text-xs">
              <span className="bg-slate-900 px-2 text-slate-500 font-mono">DEV CREDENTIALS</span>
            </div>
          </div>

          <div className="p-3.5 rounded-xl bg-slate-950 border border-slate-800/80 text-xs font-mono text-slate-400 flex flex-col gap-1.5">
            <div className="flex justify-between items-center">
              <span className="text-slate-500">Username:</span>
              <span className="text-sky-300 font-semibold">dev</span>
            </div>
            <div className="flex justify-between items-center">
              <span className="text-slate-500">Password:</span>
              <span className="text-sky-300 font-semibold">dev</span>
            </div>
          </div>
        </div>

        <div className="text-center text-xs text-slate-400">
          Don&apos;t have an account?{" "}
          <Link href="/signup" className="text-sky-400 hover:underline font-semibold">
            Create account
          </Link>
        </div>
      </div>
    </div>
  );
}
