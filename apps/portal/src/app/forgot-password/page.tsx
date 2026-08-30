"use client";

import { resolveApiBase, ThemeMenu } from "@ghanageo/ui";
import { CheckCircle2, KeyRound, LoaderCircle, Mail, MapPin } from "lucide-react";
import { useState, type FormEvent } from "react";
import { webOrigin } from "@/lib/public-origins";

const API = resolveApiBase(process.env.NEXT_PUBLIC_GHANAGEO_API_URL);
const SAFE_MESSAGE = "If that address belongs to an account, a password reset link is on its way.";

export default function ForgotPasswordPage() {
  const [status, setStatus] = useState<"idle" | "working" | "sent" | "network-error">("idle");

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setStatus("working");
    const form = new FormData(event.currentTarget);
    try {
      await fetch(`${API}/auth/password-reset/request`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email: form.get("email") }),
      });
      // Keep every HTTP response identical so existence and throttling cannot leak.
      setStatus("sent");
    } catch {
      setStatus("network-error");
    }
  }

  return (
    <main className="portal-auth-action">
      <header>
        <a className="portal-brand" href={webOrigin} aria-label="GhanaGeo home"><span className="portal-brand__mark"><MapPin size={17} aria-hidden /></span><span><strong>GhanaGeo</strong><small>Developer portal</small></span></a>
        <ThemeMenu />
      </header>
      <section aria-live="polite" aria-busy={status === "working"}>
        <span className={`portal-auth-action__icon ${status === "sent" ? "is-success" : ""}`} aria-hidden>{status === "working" ? <LoaderCircle className="spin" /> : status === "sent" ? <CheckCircle2 /> : <KeyRound />}</span>
        <p className="portal-eyebrow">Account recovery</p>
        <h1>{status === "sent" ? "Check your inbox." : "Reset your password."}</h1>
        {status === "sent" ? <><p>{SAFE_MESSAGE}</p><a className="portal-secondary" href="/#account">Return to sign in</a></> : (
          <form className="portal-auth-action__form" onSubmit={submit}>
            <p>Enter the email address used for your developer account.</p>
            <label>Email address<span className="portal-input-wrap"><Mail aria-hidden /><input name="email" type="email" autoComplete="email" placeholder="name@example.com" maxLength={320} required /></span></label>
            <button className="portal-primary" disabled={status === "working"}>{status === "working" ? <LoaderCircle className="spin" size={16} /> : <KeyRound size={16} />} Send reset link</button>
            {status === "network-error" ? <p className="account-message">The recovery service could not be reached. Check your connection and try again.</p> : null}
          </form>
        )}
      </section>
    </main>
  );
}
