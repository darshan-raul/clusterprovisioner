import type { Metadata } from "next";
import Link from "next/link";
import { Terminal, Shield, LogOut, User, Layers } from "lucide-react";
import { getSession } from "@/lib/session";
import "./globals.css";

export const metadata: Metadata = {
  title: "Strata — Kubernetes Copilot & Cluster Registry",
  description: "Two-tier conversational Kubernetes management for existing clusters.",
};

export default async function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const session = await getSession();

  return (
    <html lang="en" className="dark">
      <body className="flex flex-col min-h-screen bg-slate-950 text-slate-100 antialiased">
        <header className="sticky top-0 z-50 border-b border-slate-800/80 bg-slate-950/80 backdrop-blur-md">
          <div className="max-w-6xl mx-auto px-4 h-16 flex items-center justify-between">
            <Link href="/" className="flex items-center gap-2 font-mono font-bold text-lg text-sky-400 hover:text-sky-300 transition-colors">
              <div className="p-1.5 rounded-lg bg-sky-500/10 border border-sky-500/20 text-sky-400">
                <Terminal className="w-5 h-5" />
              </div>
              <span>STRATA</span>
              <span className="text-xs px-2 py-0.5 rounded-full bg-slate-800 text-slate-400 border border-slate-700 font-sans font-normal">v2</span>
            </Link>

            <nav className="flex items-center gap-6 text-sm">
              <Link href="/dashboard" className="text-slate-300 hover:text-white transition-colors flex items-center gap-1.5">
                <Layers className="w-4 h-4 text-slate-400" />
                <span>Dashboard</span>
              </Link>
              <Link href="/device" className="text-slate-300 hover:text-white transition-colors flex items-center gap-1.5">
                <Shield className="w-4 h-4 text-slate-400" />
                <span>Device Auth</span>
              </Link>

              {session ? (
                <div className="flex items-center gap-4 pl-4 border-l border-slate-800">
                  <div className="flex items-center gap-2">
                    <div className="w-7 h-7 rounded-full bg-sky-950 border border-sky-500/40 flex items-center justify-center text-xs font-mono text-sky-300">
                      {session.user.username.slice(0, 2).toUpperCase()}
                    </div>
                    <span className="font-mono text-xs text-slate-300">{session.user.username}</span>
                  </div>
                  <a
                    href="/api/auth/logout"
                    className="p-1.5 rounded-md hover:bg-slate-800 text-slate-400 hover:text-red-400 transition-colors"
                    title="Sign Out"
                  >
                    <LogOut className="w-4 h-4" />
                  </a>
                </div>
              ) : (
                <div className="flex items-center gap-3 pl-4 border-l border-slate-800">
                  <Link
                    href="/login"
                    className="px-3.5 py-1.5 rounded-md bg-sky-600 hover:bg-sky-500 text-white font-medium text-xs transition-colors shadow-sm shadow-sky-900/20"
                  >
                    Sign In
                  </Link>
                </div>
              )}
            </nav>
          </div>
        </header>

        <main className="flex-1 max-w-6xl w-full mx-auto px-4 py-8">
          {children}
        </main>

        <footer className="border-t border-slate-900 bg-slate-950/60 py-6 text-center text-xs text-slate-500 font-mono">
          Strata v2 — Two-Tier Kubernetes Copilot • Local TUI + Multi-Tenant Backend
        </footer>
      </body>
    </html>
  );
}
