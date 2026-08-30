"use client";

import { useRouter } from "next/navigation";
import { Logo } from "@ghanageo/ui";
import { useSession } from "@/components/session";

export default function AccessDeniedPage() {
  const router = useRouter();
  const { session, signOut } = useSession();

  return (
    <main className="admin-login">
      <section className="admin-login__intro">
        <Logo size={30} suffix="Admin" />
        <p className="admin-login__eyebrow">Administrative access required</p>
        <h1>This account is signed in, but it is not an operator.</h1>
        <p>The GhanaGeo admin is limited to approved data, support, operations and security roles.</p>
      </section>
      <section className="admin-login__form admin-access-card">
        <p>{session?.email ? <>Signed in as <strong>{session.email}</strong>.</> : "This account has no admin role."}</p>
        <button
          className="gg-button gg-button--primary gg-button--md"
          type="button"
          onClick={async () => { await signOut(); router.replace("/login"); }}
        >
          Sign out and use another account
        </button>
      </section>
    </main>
  );
}
