import Link from "next/link";
import { redirect } from "next/navigation";
import { UserPlus, ArrowRight } from "lucide-react";
import { getSession } from "@/lib/session";

export default async function SignupPage() {
  const session = await getSession();
  if (session) {
    redirect("/dashboard");
  }

  return (
    <div className="flex flex-col items-center justify-center py-12">
      <div className="w-full max-w-md p-8 rounded-2xl bg-slate-900/80 border border-slate-800 shadow-2xl flex flex-col gap-6">
        <div className="flex flex-col items-center text-center gap-2">
          <div className="p-3 rounded-xl bg-teal-950 border border-teal-800/60 text-teal-400">
            <UserPlus className="w-6 h-6" />
          </div>
          <h1 className="text-2xl font-bold text-white tracking-tight">Create an Account</h1>
          <p className="text-sm text-slate-400">
            Register your Strata account in the Keycloak authentication realm
          </p>
        </div>

        <div className="flex flex-col gap-4">
          <a
            href="/api/auth/login?prompt=create"
            className="w-full py-3 px-4 rounded-xl bg-teal-600 hover:bg-teal-500 text-white font-semibold text-sm flex items-center justify-center gap-2 transition-all shadow-md shadow-teal-950"
          >
            <span>Register on Keycloak</span>
            <ArrowRight className="w-4 h-4" />
          </a>
        </div>

        <div className="text-center text-xs text-slate-400">
          Already have an account?{" "}
          <Link href="/login" className="text-sky-400 hover:underline font-semibold">
            Sign in
          </Link>
        </div>
      </div>
    </div>
  );
}
