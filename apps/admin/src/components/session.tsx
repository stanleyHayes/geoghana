"use client";

import {
  createContext, useCallback, useContext, useEffect, useRef, useState, type ClipboardEvent,
  type KeyboardEvent, type ReactNode,
} from "react";
import Image from "next/image";
import { Badge, Card, Field, Input } from "@ghanageo/ui";
import { Download, Eye, EyeOff, KeyRound, LockKeyhole, LogIn, Mail, QrCode, ShieldCheck } from "lucide-react";
import QRCode from "qrcode";
import {
  AdminApiError, completeRecovery, completeTotp, enrolTotp, getPermissions, getSession, login, logout,
  type PermissionInfo, type SessionInfo, type TotpEnrolment,
} from "@/lib/admin-api";
import { publicPortalHref } from "@/lib/runtime-config";

/**
 * Who is signed in, and what they may do.
 *
 * The permission list drives which controls the console RENDERS. It is not a
 * security boundary — every one of these is enforced again at the API, and a
 * console that relies on hiding a button is one crafted request away from an
 * unauthorised write. Hiding is a courtesy to the user, nothing more.
 */
interface SessionState {
  session: SessionInfo | null;
  permissions: string[];
  loading: boolean;
  /** True while a first factor has passed but MFA has not. */
  mfaPending: boolean;
  mfaEnrolment: TotpEnrolment | null;
  can: (permission: string) => boolean;
  refresh: () => Promise<void>;
  signIn: (email: string, password: string) => Promise<void>;
  submitTotp: (code: string) => Promise<void>;
  submitRecovery: (code: string) => Promise<void>;
  signOut: () => Promise<void>;
}

const Ctx = createContext<SessionState | null>(null);

export function SessionProvider({ children }: { children: ReactNode }) {
  const [session, setSession] = useState<SessionInfo | null>(null);
  const [permissions, setPermissions] = useState<string[]>([]);
  const [loading, setLoading] = useState(true);
  const [mfaPending, setMfaPending] = useState(false);
  const [mfaEnrolment, setMfaEnrolment] = useState<TotpEnrolment | null>(null);

  const refresh = useCallback(async () => {
    try {
      const s = await getSession();
      setSession(s);
      setMfaPending(false);
      setMfaEnrolment(null);
      try {
        const p: PermissionInfo = await getPermissions();
        setPermissions(p.permissions);
      } catch {
        // A signed-in developer has no console permissions at all. That is a
        // valid state, not an error — they simply see nothing to act on.
        setPermissions([]);
      }
    } catch (err) {
      setSession(null);
      setPermissions([]);
      // PERMISSION_DENIED here means the session exists but has not finished
      // MFA, which is a different screen from "not signed in".
      setMfaPending(err instanceof AdminApiError && err.code === "PERMISSION_DENIED");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    // Initial session hydration must begin after mount; refresh owns the
    // asynchronous request and the resulting loading/session state.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void refresh();
  }, [refresh]);

  useEffect(() => {
    const expire = () => {
      setSession(null);
      setPermissions([]);
      setMfaPending(false);
      setMfaEnrolment(null);
      setLoading(false);
    };
    window.addEventListener("ghanageo:session-expired", expire);
    return () => window.removeEventListener("ghanageo:session-expired", expire);
  }, []);

  useEffect(() => {
    if (!session) return;
    const remaining = new Date(session.expiresAt).getTime() - Date.now();
    const expire = () => {
      setSession(null);
      setPermissions([]);
    };
    const timer = window.setTimeout(expire, Math.max(0, Math.min(remaining, 2_147_483_647)));
    return () => window.clearTimeout(timer);
  }, [session]);

  const signIn = useCallback(async (email: string, password: string) => {
    const res = await login(email, password);
    if (res.mfaRequired) {
      setMfaPending(true);
      if (res.mfaEnrolmentRequired) setMfaEnrolment(await enrolTotp());
      setLoading(false);
      return;
    }
    await refresh();
  }, [refresh]);

  const submitTotp = useCallback(async (code: string) => {
    await completeTotp(code);
    await refresh();
  }, [refresh]);

  const submitRecovery = useCallback(async (code: string) => {
    await completeRecovery(code);
    await refresh();
  }, [refresh]);

  const can = useCallback((p: string) => permissions.includes(p), [permissions]);

  const signOut = useCallback(async () => {
    try {
      await logout();
    } finally {
      setSession(null);
      setPermissions([]);
      setMfaPending(false);
      setMfaEnrolment(null);
      setLoading(false);
    }
  }, []);

  return (
    <Ctx.Provider value={{ session, permissions, loading, mfaPending, mfaEnrolment, can, refresh, signIn, submitTotp, submitRecovery, signOut }}>
      {children}
    </Ctx.Provider>
  );
}

export function useSession(): SessionState {
  const ctx = useContext(Ctx);
  if (!ctx) throw new Error("useSession must be used inside <SessionProvider>");
  return ctx;
}

/**
 * Renders children only for a signed-in steward holding a permission.
 *
 * `fallback` explains WHY something is unavailable rather than leaving a blank
 * space — an operator who cannot see a control and cannot see a reason files a
 * bug report.
 */
export function RequirePermission({
  permission, children, fallback,
}: {
  permission: string;
  children: ReactNode;
  fallback?: ReactNode;
}) {
  const { can, session, loading } = useSession();
  if (loading) return null;
  if (!session) return <>{fallback ?? null}</>;
  if (!can(permission)) {
    return (
      <>
        {fallback ?? (
          <Badge tone="needsRecon">
            Your role cannot do this — needs {permission}
          </Badge>
        )}
      </>
    );
  }
  return <>{children}</>;
}

function TotpEnrollmentCard({ enrolment }: { enrolment: TotpEnrolment }) {
  const [qrDataUrl, setQrDataUrl] = useState("");

  useEffect(() => {
    let active = true;
    void QRCode.toDataURL(enrolment.uri, {
      width: 220,
      margin: 2,
      errorCorrectionLevel: "M",
      color: { dark: "#082d24", light: "#ffffff" },
    }).then((url) => { if (active) setQrDataUrl(url); });
    return () => { active = false; };
  }, [enrolment.uri]);

  const downloadRecoveryCodes = () => {
    const contents = [
      "GhanaGeo admin recovery codes",
      "Save these somewhere private. Each code can be used once.",
      "",
      ...enrolment.recoveryCodes,
      "",
    ].join("\n");
    const url = URL.createObjectURL(new Blob([contents], { type: "text/plain;charset=utf-8" }));
    const link = document.createElement("a");
    link.href = url;
    link.download = "ghanageo-admin-recovery-codes.txt";
    link.click();
    URL.revokeObjectURL(url);
  };

  return (
    <div className="admin-mfa-enrolment">
      <div className="admin-mfa-enrolment__heading">
        <span><QrCode aria-hidden /></span>
        <div><strong>Scan with your authenticator</strong><p>Use Google Authenticator, 1Password, Authy or another TOTP app.</p></div>
      </div>
      <div className="admin-mfa-enrolment__setup">
        <div className="admin-mfa-enrolment__qr">
          {qrDataUrl ? <Image src={qrDataUrl} width={220} height={220} unoptimized alt="QR code for adding GhanaGeo Admin to an authenticator app" /> : <span aria-label="Generating QR code" />}
        </div>
        <div className="admin-mfa-enrolment__manual">
          <span>Can’t scan it?</span>
          <p>Enter this setup key manually:</p>
          <code>{enrolment.secret}</code>
        </div>
      </div>
      <div className="admin-mfa-enrolment__recovery">
        <div><strong>Keep your recovery codes safe</strong><p>Each code works once if you lose access to your authenticator.</p></div>
        <button className="gg-button gg-button--secondary gg-button--sm" type="button" onClick={downloadRecoveryCodes}><Download size={15} aria-hidden /> Download codes</button>
        <ul>{enrolment.recoveryCodes.map((recoveryCode) => <li key={recoveryCode}><code>{recoveryCode}</code></li>)}</ul>
      </div>
    </div>
  );
}

function OtpInput({ value, onChange }: { value: string; onChange: (value: string) => void }) {
  const inputs = useRef<Array<HTMLInputElement | null>>([]);
  const [digits, setDigits] = useState(() => Array.from({ length: 6 }, (_, index) => value[index] ?? ""));

  const setDigit = (index: number, next: string) => {
    const digit = next.replace(/\D/g, "").slice(-1);
    const updated = [...digits];
    updated[index] = digit;
    setDigits(updated);
    onChange(updated.join(""));
    if (digit && index < 5) inputs.current[index + 1]?.focus();
  };

  const onKeyDown = (index: number, event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === "Backspace" && !digits[index] && index > 0) inputs.current[index - 1]?.focus();
    if (event.key === "ArrowLeft" && index > 0) { event.preventDefault(); inputs.current[index - 1]?.focus(); }
    if (event.key === "ArrowRight" && index < 5) { event.preventDefault(); inputs.current[index + 1]?.focus(); }
  };

  const onPaste = (event: ClipboardEvent<HTMLDivElement>) => {
    const pasted = event.clipboardData.getData("text").replace(/\D/g, "").slice(0, 6);
    if (!pasted) return;
    event.preventDefault();
    const updated = Array.from({ length: 6 }, (_, index) => pasted[index] ?? "");
    setDigits(updated);
    onChange(pasted);
    inputs.current[Math.min(pasted.length, 6) - 1]?.focus();
  };

  return (
    <div className="admin-otp" onPaste={onPaste} role="group" aria-label="Six-digit authenticator code">
      {digits.map((digit, index) => <input
        key={index} id={index === 0 ? "totp" : undefined} ref={(element) => { inputs.current[index] = element; }} value={digit}
        onChange={(event) => setDigit(index, event.target.value)} onKeyDown={(event) => onKeyDown(index, event)}
        onFocus={(event) => event.currentTarget.select()} inputMode="numeric" pattern="[0-9]*"
        autoComplete={index === 0 ? "one-time-code" : "off"} maxLength={1}
        aria-label={`Authenticator digit ${index + 1}`} required
      />)}
    </div>
  );
}

/** The sign-in panel, including the MFA step. */
export function SignInPanel() {
  const { signIn, submitTotp, submitRecovery, mfaPending, mfaEnrolment, loading, session } = useSession();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [code, setCode] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [useRecovery, setUseRecovery] = useState(false);
  const [showPassword, setShowPassword] = useState(false);

  if (loading || session) return null;

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      if (mfaPending) await (useRecovery ? submitRecovery(code) : submitTotp(code));
      else await signIn(email, password);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Sign-in failed.");
    } finally {
      setBusy(false);
    }
  };

  return (
    <Card style={{ maxWidth: 420 }}>
      <form onSubmit={submit} style={{ display: "grid", gap: "var(--space-4)" }}>
        <div className="gg-stack-row">
          {mfaPending ? <ShieldCheck size={18} style={{ color: "var(--brand)" }} aria-hidden />
                      : <LogIn size={18} style={{ color: "var(--brand)" }} aria-hidden />}
          <strong>{mfaPending ? "Two-factor authentication" : "Sign in"}</strong>
        </div>

        {mfaPending ? (
          <>
            {mfaEnrolment ? (
              <TotpEnrollmentCard enrolment={mfaEnrolment} />
            ) : null}
            <Field label={useRecovery ? "Recovery code" : "Authenticator code"} htmlFor="totp">
              {useRecovery ? <div className="admin-input-wrap"><KeyRound aria-hidden /><Input
                id="totp" value={code} onChange={(e) => setCode(e.target.value)} className="admin-input-with-start"
                inputMode="text" autoComplete="one-time-code" placeholder="Enter a recovery code" required
              /></div> : <OtpInput value={code} onChange={setCode} />}
            </Field>
            {!mfaEnrolment ? (
              <button className="gg-button gg-button--ghost gg-button--sm" type="button" onClick={() => { setUseRecovery((value) => !value); setCode(""); }}>
                {useRecovery ? "Use authenticator code" : "Use a recovery code"}
              </button>
            ) : null}
          </>
        ) : (
          <>
            <Field label="Email" htmlFor="email">
              <div className="admin-input-wrap"><Mail aria-hidden /><Input
                id="email" type="email" value={email} autoComplete="username"
                placeholder="name@example.com" onChange={(e) => setEmail(e.target.value)} className="admin-input-with-start" required
              /></div>
            </Field>
            <Field label="Password" htmlFor="password">
              <div className="admin-input-wrap"><LockKeyhole aria-hidden /><Input
                id="password" type={showPassword ? "text" : "password"} value={password} autoComplete="current-password"
                placeholder="Enter your password" onChange={(e) => setPassword(e.target.value)} className="admin-input-with-icons" required
              /><button className="admin-password-toggle" type="button" onClick={() => setShowPassword((value) => !value)} aria-label={showPassword ? "Hide password" : "Show password"} aria-pressed={showPassword}>{showPassword ? <EyeOff aria-hidden /> : <Eye aria-hidden />}</button></div>
            </Field>
            <a className="admin-forgot-link" href={publicPortalHref("/forgot-password")}>Forgot password?</a>
          </>
        )}

        {error ? (
          <p role="alert" style={{ margin: 0, color: "var(--danger)", fontSize: "var(--text-sm)" }}>
            {error}
          </p>
        ) : null}

        <button className="gg-button gg-button--primary gg-button--md" type="submit" disabled={busy}>
          {mfaPending ? <ShieldCheck size={16} aria-hidden /> : <LogIn size={16} aria-hidden />}
          {busy ? "Working…" : mfaPending ? "Verify" : "Sign in"}
        </button>
      </form>
    </Card>
  );
}
