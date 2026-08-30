"use client";

import { resolveApiBase, ThemeMenu } from "@ghanageo/ui";
import { AlertCircle, CheckCircle2, Eye, EyeOff, KeyRound, LoaderCircle, LockKeyhole, MapPin } from "lucide-react";
import { useEffect, useRef, useState, type FormEvent } from "react";
import { passwordPolicyError } from "@/lib/password-policy";
import { webOrigin } from "@/lib/public-origins";

const API = resolveApiBase(process.env.NEXT_PUBLIC_GHANAGEO_API_URL);
type ResetState = { status: "initializing" | "ready" | "working" } | { status: "success" | "error"; message: string };

export default function ResetPasswordPage() {
  const token = useRef("");
  const initialized = useRef(false);
  const [state, setState] = useState<ResetState>({ status: "initializing" });
  const [visible, setVisible] = useState({ password: false, confirmation: false });

  useEffect(() => {
    if (initialized.current) return;
    initialized.current = true;
    const url = new URL(window.location.href);
    token.current = url.searchParams.get("token")?.trim() ?? "";
    url.searchParams.delete("token");
    window.history.replaceState(null, "", `${url.pathname}${url.search}${url.hash}`);
    setState(token.current ? { status: "ready" } : { status: "error", message: "This reset link is incomplete. Request a new one to continue." });
  }, []);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    const password = String(form.get("password") ?? "");
    const confirmation = String(form.get("confirmation") ?? "");
    const policyError = passwordPolicyError(password);
    if (policyError) return setState({ status: "error", message: policyError });
    if (password !== confirmation) return setState({ status: "error", message: "The passwords do not match." });
    if (!token.current) return setState({ status: "error", message: "This reset link is incomplete. Request a new one to continue." });

    setState({ status: "working" });
    try {
      const response = await fetch(`${API}/auth/password-reset/confirm`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ token: token.current, password }),
      });
      const payload = await response.json().catch(() => ({})) as { message?: string; error?: { message?: string } };
      if (!response.ok) {
        if (response.status === 400) token.current = "";
        setState({ status: "error", message: payload.error?.message ?? "That password reset link has expired or been used." });
        return;
      }
      token.current = "";
      event.currentTarget.reset();
      setState({ status: "success", message: payload.message ?? "Password changed. Sign in again on every device." });
    } catch (cause) {
      setState({ status: "error", message: cause instanceof Error ? cause.message : "The password could not be reset. Check your connection and try again." });
    }
  }

  const unavailable = state.status !== "initializing" && !token.current && state.status !== "success";
  return (
    <main className="portal-auth-action">
      <header>
        <a className="portal-brand" href={webOrigin} aria-label="GhanaGeo home"><span className="portal-brand__mark"><MapPin size={17} aria-hidden /></span><span><strong>GhanaGeo</strong><small>Developer portal</small></span></a>
        <ThemeMenu />
      </header>
      <section aria-live="polite" aria-busy={state.status === "working"}>
        <span className={`portal-auth-action__icon is-${state.status}`} aria-hidden>{state.status === "working" ? <LoaderCircle className="spin" /> : state.status === "success" ? <CheckCircle2 /> : state.status === "error" ? <AlertCircle /> : <KeyRound />}</span>
        <p className="portal-eyebrow">Account recovery</p>
        <h1>{state.status === "initializing" ? "Checking your reset link…" : state.status === "success" ? "Password changed." : unavailable ? "Request a new link." : "Choose a new password."}</h1>
        {state.status === "initializing" ? <p>This should only take a moment.</p> : state.status === "success" ? <><p>{state.message}</p><a className="portal-primary" href="/#account">Continue to sign in</a></> : unavailable ? <><p>{state.status === "error" ? state.message : "This reset link is incomplete."}</p><a className="portal-primary" href="/forgot-password">Request a reset link</a></> : (
          <form className="portal-auth-action__form" onSubmit={submit}>
            <p>Use 12–1,024 characters. Long, memorable passphrases work well; special characters are optional.</p>
            <label>New password<span className="portal-input-wrap"><LockKeyhole aria-hidden /><input name="password" type={visible.password ? "text" : "password"} autoComplete="new-password" placeholder="Create a secure password" minLength={12} maxLength={1024} required /><button type="button" className="portal-password-toggle" onClick={() => setVisible((value) => ({ ...value, password: !value.password }))} aria-label={visible.password ? "Hide new password" : "Show new password"} aria-pressed={visible.password}>{visible.password ? <EyeOff aria-hidden /> : <Eye aria-hidden />}</button></span></label>
            <label>Confirm new password<span className="portal-input-wrap"><LockKeyhole aria-hidden /><input name="confirmation" type={visible.confirmation ? "text" : "password"} autoComplete="new-password" placeholder="Repeat your new password" minLength={12} maxLength={1024} required /><button type="button" className="portal-password-toggle" onClick={() => setVisible((value) => ({ ...value, confirmation: !value.confirmation }))} aria-label={visible.confirmation ? "Hide confirmation password" : "Show confirmation password"} aria-pressed={visible.confirmation}>{visible.confirmation ? <EyeOff aria-hidden /> : <Eye aria-hidden />}</button></span></label>
            <button className="portal-primary" disabled={state.status === "working"}>{state.status === "working" ? <LoaderCircle className="spin" size={16} /> : <KeyRound size={16} />} Change password</button>
            {state.status === "error" ? <p className="account-message"><AlertCircle size={15} />{state.message}</p> : null}
          </form>
        )}
      </section>
    </main>
  );
}
