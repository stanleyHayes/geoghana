"use client";

import { useEffect, type ReactNode } from "react";
import { usePathname, useRouter } from "next/navigation";
import { Logo } from "@ghanageo/ui";
import { AdminShell } from "@/components/admin-shell";
import { useSession } from "@/components/session";
import { safeReturnTo } from "@/lib/safe-return-to";

export function AuthenticatedAdmin({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const { session, loading } = useSession();
  const isAuthScreen = pathname === "/login" || pathname === "/access-denied";

  useEffect(() => {
    if (loading) return;
    if (!session && !isAuthScreen) {
      const returnTo = safeReturnTo(pathname, window.location.origin);
      router.replace(`/login?returnTo=${encodeURIComponent(returnTo)}`);
    }
  }, [isAuthScreen, loading, pathname, router, session]);

  if (isAuthScreen) return <>{children}</>;
  if (loading || !session) {
    return (
      <main className="admin-auth-loading" aria-busy="true" aria-live="polite">
        <Logo size={34} suffix="Admin" />
        <span className="admin-auth-loading__pulse" aria-hidden />
        <span className="admin-auth-loading__label">Securing your workspace…</span>
      </main>
    );
  }
  return <AdminShell>{children}</AdminShell>;
}
