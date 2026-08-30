"use client";

import { Suspense, useEffect } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { Logo } from "@ghanageo/ui";
import { SignInPanel, useSession } from "@/components/session";
import { safeReturnTo } from "@/lib/safe-return-to";

function LoginContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { session } = useSession();
  // Only a relative result is returned, so the stable synthetic origin also
  // keeps server and client rendering identical.
  const requested = safeReturnTo(searchParams.get("returnTo"), "https://admin.invalid");
  const returnTo = requested.startsWith("/login") || requested.startsWith("/access-denied") ? "/" : requested;

  useEffect(() => {
    if (session) router.replace(returnTo);
  }, [returnTo, router, session]);

  return (
    <main className="admin-login">
      <section className="admin-login__intro" aria-labelledby="admin-login-title">
        <Logo size={30} suffix="Admin" />
        <p className="admin-login__eyebrow">Protected operations workspace</p>
        <h1 id="admin-login-title">Sign in to steward Ghana’s location infrastructure.</h1>
        <p>Administrative records, release controls and audit data are available only to authorised operators.</p>
      </section>
      <div className="admin-login__form"><SignInPanel /></div>
    </main>
  );
}

export default function LoginPage() {
  return <Suspense><LoginContent /></Suspense>;
}
