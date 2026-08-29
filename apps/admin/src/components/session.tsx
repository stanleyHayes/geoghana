"use client";

import {
  createContext, useCallback, useContext, useEffect, useState, type ReactNode,
} from "react";
import { Badge, Card, Field, Input } from "@ghanageo/ui";
import { LogIn, ShieldCheck } from "lucide-react";
import {
  AdminApiError, completeTotp, getPermissions, getSession, login,
  type PermissionInfo, type SessionInfo,
} from "@/lib/admin-api";

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
  can: (permission: string) => boolean;
  refresh: () => Promise<void>;
  signIn: (email: string, password: string) => Promise<void>;
  submitTotp: (code: string) => Promise<void>;
}

const Ctx = createContext<SessionState | null>(null);

export function SessionProvider({ children }: { children: ReactNode }) {
  const [session, setSession] = useState<SessionInfo | null>(null);
  const [permissions, setPermissions] = useState<string[]>([]);
  const [loading, setLoading] = useState(true);
  const [mfaPending, setMfaPending] = useState(false);

  const refresh = useCallback(async () => {
    try {
      const s = await getSession();
      setSession(s);
      setMfaPending(false);
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

  const signIn = useCallback(async (email: string, password: string) => {
    const res = await login(email, password);
    if (res.mfaRequired) {
      setMfaPending(true);
      setLoading(false);
      return;
    }
    await refresh();
  }, [refresh]);

  const submitTotp = useCallback(async (code: string) => {
    await completeTotp(code);
    await refresh();
  }, [refresh]);

  const can = useCallback((p: string) => permissions.includes(p), [permissions]);

  return (
    <Ctx.Provider value={{ session, permissions, loading, mfaPending, can, refresh, signIn, submitTotp }}>
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

/** The sign-in panel, including the MFA step. */
export function SignInPanel() {
  const { signIn, submitTotp, mfaPending, loading, session } = useSession();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [code, setCode] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  if (loading || session) return null;

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      if (mfaPending) await submitTotp(code);
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
          <Field label="Authenticator code" htmlFor="totp">
            <Input
              id="totp" value={code} onChange={(e) => setCode(e.target.value)}
              inputMode="numeric" autoComplete="one-time-code" placeholder="123456" required
            />
          </Field>
        ) : (
          <>
            <Field label="Email" htmlFor="email">
              <Input
                id="email" type="email" value={email} autoComplete="username"
                onChange={(e) => setEmail(e.target.value)} required
              />
            </Field>
            <Field label="Password" htmlFor="password">
              <Input
                id="password" type="password" value={password} autoComplete="current-password"
                onChange={(e) => setPassword(e.target.value)} required
              />
            </Field>
          </>
        )}

        {error ? (
          <p role="alert" style={{ margin: 0, color: "var(--danger)", fontSize: "var(--text-sm)" }}>
            {error}
          </p>
        ) : null}

        <button className="gg-button gg-button--primary gg-button--md" type="submit" disabled={busy}>
          {busy ? "Working…" : mfaPending ? "Verify" : "Sign in"}
        </button>
      </form>
    </Card>
  );
}
