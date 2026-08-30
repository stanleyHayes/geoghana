"use client";

import { resolveApiBase, ThemeMenu } from "@ghanageo/ui";
import { AlertCircle, CheckCircle2, LoaderCircle, MapPin } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { webOrigin } from "@/lib/public-origins";

const API = resolveApiBase(process.env.NEXT_PUBLIC_GHANAGEO_API_URL);

type VerificationState =
  | { status: "working" }
  | { status: "success"; message: string }
  | { status: "error"; message: string };

export default function VerifyEmailPage() {
  const started = useRef(false);
  const [state, setState] = useState<VerificationState>({ status: "working" });

  useEffect(() => {
    if (started.current) return;
    started.current = true;

    const url = new URL(window.location.href);
    const token = url.searchParams.get("token")?.trim();
    url.searchParams.delete("token");
    window.history.replaceState(null, "", `${url.pathname}${url.search}${url.hash}`);

    if (!token) {
      setState({ status: "error", message: "This verification link is incomplete. Request a new email from the developer portal." });
      return;
    }

    const verify = async () => {
      try {
        const response = await fetch(`${API}/auth/verify`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ token }),
        });
        const payload = await response.json().catch(() => ({})) as {
          message?: string;
          error?: { message?: string };
        };
        if (!response.ok) {
          throw new Error(payload.error?.message ?? "This link is invalid or has expired.");
        }
        setState({ status: "success", message: payload.message ?? "Your email address is verified." });
      } catch (cause) {
        setState({
          status: "error",
          message: cause instanceof Error ? cause.message : "Email verification could not be completed.",
        });
      }
    };

    void verify();
  }, []);

  return (
    <main className="portal-auth-action">
      <header>
        <a className="portal-brand" href={webOrigin} aria-label="GhanaGeo home">
          <span className="portal-brand__mark"><MapPin size={17} aria-hidden /></span>
          <span><strong>GhanaGeo</strong><small>Developer portal</small></span>
        </a>
        <ThemeMenu />
      </header>

      <section aria-live="polite" aria-busy={state.status === "working"}>
        <span className={`portal-auth-action__icon is-${state.status}`} aria-hidden>
          {state.status === "working" ? <LoaderCircle className="spin" /> : state.status === "success" ? <CheckCircle2 /> : <AlertCircle />}
        </span>
        <p className="portal-eyebrow">Account security</p>
        <h1>{state.status === "working" ? "Verifying your email…" : state.status === "success" ? "Email verified." : "We could not verify that link."}</h1>
        <p>{state.status === "working" ? "This should only take a moment." : state.message}</p>
        {state.status !== "working" ? <a className="portal-primary" href="/#account">Continue to sign in</a> : null}
      </section>
    </main>
  );
}
