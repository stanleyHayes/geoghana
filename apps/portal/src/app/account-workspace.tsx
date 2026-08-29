"use client";

import {
  AlertCircle,
  Check,
  Copy,
  KeyRound,
  LoaderCircle,
  LogIn,
  LogOut,
  Plus,
  Play,
  RefreshCw,
  ShieldCheck,
  Trash2,
  UserPlus,
} from "lucide-react";
import { resolveApiBase } from "@ghanageo/ui";
import { useCallback, useEffect, useState } from "react";
import type { FormEvent } from "react";
import { OrganizationAccess } from "./organization-access";
import { UsageAnalytics } from "./usage-analytics";

const API =
  // resolveApiBase, not the raw env var: NEXT_PUBLIC_* is inlined at BUILD
  // time and `next build` runs in production mode, so this baked in
  // api.geo.digitalghana.dev — a domain that does not exist yet — and every
  // request from a locally served build failed with ERR_NAME_NOT_RESOLVED.
  resolveApiBase(process.env.NEXT_PUBLIC_GHANAGEO_API_URL);
type Session = {
  accountId: string;
  email: string;
  role: string;
  mfaEnrolled: boolean;
};
export type OrganizationMember = {
  accountId: string;
  email: string;
  role: "OWNER" | "ADMIN" | "MEMBER" | "VIEWER";
  joinedAt: string;
};
export type Organization = {
  id: string;
  name: string;
  ownerId: string;
  members: OrganizationMember[];
  createdAt: string;
};
type Application = {
  id: string;
  organizationId: string;
  name: string;
  description: string;
  environments: ("test" | "live")[];
  domains: string[];
  callbackUrl: string;
  plan: "free";
  createdAt: string;
};
type APIKey = {
  id: string;
  name: string;
  prefix: string;
  class: "BROWSER" | "SERVER" | "TEST";
  environment: "live" | "test";
  scopes: string[];
  allowedOrigins: string[] | null;
  allowedIps: string[] | null;
  createdAt: string;
  lastUsedAt: string | null;
  revokedAt: string | null;
  expiresAt: string | null;
};
type PreservedRequest = {
  protocol: "rest" | "graphql" | "grpc";
  request: string;
  payload: string;
};

let portalRequestQueue: Promise<void> = Promise.resolve();

export function portalRequest<T>(path: string, init?: RequestInit): Promise<T> {
  const run = async () => {
    const response = await fetch(`${API}${path}`, {
      ...init,
      credentials: "include",
      headers: { "Content-Type": "application/json", ...init?.headers },
    });
    const payload = await response.json().catch(() => ({}));
    if (!response.ok)
      throw new Error(
        payload?.error?.message ?? "That request could not be completed.",
      );
    return payload as T;
  };
  const result = portalRequestQueue.then(run, run);
  portalRequestQueue = result.then(
    () => undefined,
    () => undefined,
  );
  return result;
}

export function AccountWorkspace() {
  const [session, setSession] = useState<Session | null>(null);
  const [checking, setChecking] = useState(true);
  const [mode, setMode] = useState<"login" | "register">("login");
  const [notice, setNotice] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [orgs, setOrgs] = useState<Organization[]>([]);
  const [orgId, setOrgId] = useState("");
  const [apps, setApps] = useState<Application[]>([]);
  const [appId, setAppId] = useState("");
  const [keys, setKeys] = useState<APIKey[]>([]);
  const [keyClass, setKeyClass] = useState<"TEST" | "BROWSER" | "SERVER">(
    "TEST",
  );
  const [secret, setSecret] = useState("");
  const [secretCopied, setSecretCopied] = useState(false);
  const [preserved, setPreserved] = useState<PreservedRequest | null>(null);
  const [replayResult, setReplayResult] = useState("");
  const [replaying, setReplaying] = useState(false);

  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    if (params.get("from") !== "sandbox") return;
    const protocol = params.get("protocol");
    const request = params.get("request");
    if (!request || !["rest", "graphql", "grpc"].includes(protocol ?? "")) return;
    setPreserved({
      protocol: protocol as PreservedRequest["protocol"],
      request,
      payload: params.get("payload") ?? "{}",
    });
    setMode("register");
  }, []);

  const loadSession = useCallback(async () => {
    try {
      const x = await portalRequest<{ data: Session }>("/auth/session");
      setSession(x.data);
    } catch {
      setSession(null);
    } finally {
      setChecking(false);
    }
  }, []);
  useEffect(() => {
    void loadSession();
  }, [loadSession]);
  useEffect(() => {
    if (!session) return;
    portalRequest<{ data: Organization[] }>("/developer/organizations")
      .then((x) => {
        setOrgs(x.data);
        setOrgId((v) => v || x.data[0]?.id || "");
      })
      .catch((e) => setError(e.message));
  }, [session]);
  useEffect(() => {
    if (!orgId) {
      setApps([]);
      setAppId("");
      return;
    }
    portalRequest<{ data: Application[] }>(
      `/developer/organizations/${orgId}/applications`,
    )
      .then((x) => {
        setApps(x.data);
        setAppId(x.data[0]?.id || "");
      })
      .catch((e) => setError(e.message));
  }, [orgId]);
  useEffect(() => {
    if (!orgId || !appId) {
      setKeys([]);
      return;
    }
    portalRequest<{ data: APIKey[] }>(
      `/developer/organizations/${orgId}/applications/${appId}/keys`,
    )
      .then((x) => setKeys(x.data))
      .catch((e) => setError(e.message));
  }, [orgId, appId]);

  async function authSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setBusy(true);
    setError("");
    setNotice("");
    const form = new FormData(event.currentTarget);
    try {
      if (mode === "register") {
        const x = await portalRequest<{ message: string }>("/auth/register", {
          method: "POST",
          body: JSON.stringify({
            email: form.get("email"),
            password: form.get("password"),
          }),
        });
        setNotice(x.message);
        setMode("login");
      } else {
        await portalRequest("/auth/login", {
          method: "POST",
          body: JSON.stringify({
            email: form.get("email"),
            password: form.get("password"),
          }),
        });
        await loadSession();
      }
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  }
  async function createOrg(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    const form = event.currentTarget;
    const name = new FormData(form).get("name");
    try {
      const x = await portalRequest<{ data: Organization }>(
        "/developer/organizations",
        { method: "POST", body: JSON.stringify({ name }) },
      );
      setOrgs((v) => [...v, x.data]);
      setOrgId(x.data.id);
      form.reset();
    } catch (e) {
      setError((e as Error).message);
    }
  }
  async function createApp(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    const form = event.currentTarget;
    const data = new FormData(form);
    try {
      const x = await portalRequest<{ data: Application }>(
        `/developer/organizations/${orgId}/applications`,
        {
          method: "POST",
          body: JSON.stringify({
            name: data.get("name"),
            description: data.get("description"),
            environments: ["test", "live"],
            domains: String(data.get("domains") || "")
              .split(",")
              .filter(Boolean)
              .map((value) => value.trim()),
            callbackUrl: data.get("callbackUrl"),
          }),
        },
      );
      setApps((v) => [...v, x.data]);
      setAppId(x.data.id);
      form.reset();
    } catch (e) {
      setError((e as Error).message);
    }
  }
  async function createKey(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    setSecret("");
    const form = event.currentTarget;
    const data = new FormData(form);
    const klass = String(data.get("class"));
    const environment =
      klass === "TEST" ? "test" : String(data.get("environment"));
    try {
      const x = await portalRequest<{ data: { key: APIKey; secret: string } }>(
        `/developer/organizations/${orgId}/applications/${appId}/keys`,
        {
          method: "POST",
          body: JSON.stringify({
            name: data.get("name"),
            class: klass,
            environment,
            scopes: data.getAll("scopes"),
            allowedOrigins: String(data.get("origins") || "")
              .split(",")
              .filter(Boolean)
              .map((v) => v.trim()),
            allowedIps: String(data.get("ips") || "")
              .split(",")
              .filter(Boolean)
              .map((v) => v.trim()),
            expiresAt: data.get("expiresAt")
              ? new Date(String(data.get("expiresAt"))).toISOString()
              : "",
          }),
        },
      );
      setKeys((v) => [x.data.key, ...v]);
      setSecret(x.data.secret);
      setSecretCopied(false);
      form.reset();
    } catch (e) {
      setError((e as Error).message);
    }
  }
  async function keyAction(key: APIKey, action: "rotate" | "revoke") {
    setError("");
    try {
      const x = await portalRequest<{ data?: { key: APIKey; secret: string } }>(
        `/developer/organizations/${orgId}/applications/${appId}/keys/${key.id}/${action}`,
        { method: "POST" },
      );
      if (action === "rotate" && x.data) {
        setSecret(x.data.secret);
        setSecretCopied(false);
      }
      const refreshed = await portalRequest<{ data: APIKey[] }>(
        `/developer/organizations/${orgId}/applications/${appId}/keys`,
      );
      setKeys(refreshed.data);
    } catch (e) {
      setError((e as Error).message);
    }
  }
  async function logout() {
    await portalRequest("/auth/logout", { method: "POST" });
    setSession(null);
    setOrgs([]);
    setApps([]);
    setKeys([]);
  }
  async function replayPreservedRequest() {
    if (!preserved) return;
    setReplaying(true);
    setReplayResult("");
    try {
      let response: Response;
      if (preserved.protocol === "rest") {
        response = await fetch(`${API}${preserved.request}`);
      } else if (preserved.protocol === "graphql") {
        response = await fetch(API.replace(/\/v1\/?$/, "/graphql"), {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ query: preserved.request, variables: JSON.parse(preserved.payload || "{}") }),
        });
      } else {
        const sandbox = process.env.NEXT_PUBLIC_GHANAGEO_SANDBOX_URL ?? "http://localhost:3101";
        response = await fetch(`${sandbox}/api/grpc`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ method: preserved.request, payload: JSON.parse(preserved.payload || "{}") }),
        });
      }
      const text = await response.text();
      setReplayResult(JSON.stringify(JSON.parse(text), null, 2));
    } catch (cause) {
      setReplayResult(JSON.stringify({ error: cause instanceof Error ? cause.message : "Replay failed" }, null, 2));
    } finally {
      setReplaying(false);
    }
  }
  async function reloadOrganizations() {
    const x = await portalRequest<{ data: Organization[] }>(
      "/developer/organizations",
    );
    setOrgs(x.data);
  }

  if (checking)
    return (
      <section
        className="account-workspace account-workspace--loading"
        aria-label="Loading account"
      >
        <LoaderCircle size={22} />
        <span>Checking your workspace…</span>
      </section>
    );
  if (!session)
    return (
      <section className="account-workspace" id="account">
        <div className="account-intro">
          <p className="portal-eyebrow">
            <ShieldCheck size={14} /> Optional account
          </p>
          <h2>Own your integration.</h2>
          <p>
            Create an account when you need attributed usage, restricted
            credentials and an audit trail. Public reads remain anonymous.
          </p>
          {preserved ? <p className="preserved-request__notice"><Check size={15} /> Your {preserved.protocol.toUpperCase()} sandbox request is saved and will be ready after sign-in.</p> : null}
        </div>
        <form className="account-auth" onSubmit={authSubmit}>
          <div className="account-tabs">
            <button
              type="button"
              aria-pressed={mode === "login"}
              onClick={() => setMode("login")}
            >
              <LogIn size={15} /> Sign in
            </button>
            <button
              type="button"
              aria-pressed={mode === "register"}
              onClick={() => setMode("register")}
            >
              <UserPlus size={15} /> Register
            </button>
          </div>
          <label>
            Email
            <input name="email" type="email" autoComplete="email" required />
          </label>
          <label>
            Password
            <input
              name="password"
              type="password"
              autoComplete={
                mode === "login" ? "current-password" : "new-password"
              }
              minLength={12}
              required
            />
          </label>
          <button className="portal-primary" disabled={busy}>
            {busy ? (
              <LoaderCircle className="spin" size={16} />
            ) : mode === "login" ? (
              <LogIn size={16} />
            ) : (
              <UserPlus size={16} />
            )}{" "}
            {mode === "login" ? "Open workspace" : "Create account"}
          </button>
          {notice ? (
            <p className="account-message is-ok">
              <Check size={15} />
              {notice}
            </p>
          ) : null}
          {error ? (
            <p className="account-message">
              <AlertCircle size={15} />
              {error}
            </p>
          ) : null}
        </form>
      </section>
    );

  return (
    <section
      className="account-workspace account-workspace--signed"
      id="account"
    >
      <header className="account-heading">
        <div>
          <p className="portal-eyebrow">
            <ShieldCheck size={14} /> Authenticated workspace
          </p>
          <h2>{session.email}</h2>
        </div>
        <button className="portal-secondary" onClick={() => void logout()}>
          <LogOut size={15} /> Sign out
        </button>
      </header>
      {error ? (
        <p className="account-message">
          <AlertCircle size={15} />
          {error}
        </p>
      ) : null}
      {preserved ? (
        <section className="preserved-request" aria-labelledby="preserved-request-title">
          <div>
            <span>{preserved.protocol.toUpperCase()} · Preserved from sandbox</span>
            <h3 id="preserved-request-title">Continue where you left off.</h3>
            <code>{preserved.request}</code>
          </div>
          <button className="portal-primary" onClick={() => void replayPreservedRequest()} disabled={replaying}>
            {replaying ? <LoaderCircle className="spin" size={15} /> : <Play size={15} />} {replaying ? "Replaying…" : "Replay request"}
          </button>
          {replayResult ? <pre tabIndex={0}>{replayResult}</pre> : null}
        </section>
      ) : null}
      <div className="account-columns">
        <section>
          <span className="account-step">01 · Organization</span>
          <select
            value={orgId}
            onChange={(e) => setOrgId(e.target.value)}
            aria-label="Organization"
          >
            <option value="">Choose an organization</option>
            {orgs.map((o) => (
              <option key={o.id} value={o.id}>
                {o.name}
              </option>
            ))}
          </select>
          <form onSubmit={createOrg}>
            <input
              name="name"
              placeholder="New organization name"
              aria-label="New organization name"
              required
            />
            <button aria-label="Create organization">
              <Plus size={16} />
            </button>
          </form>
        </section>
        <section>
          <span className="account-step">02 · Application</span>
          <select
            value={appId}
            onChange={(e) => setAppId(e.target.value)}
            disabled={!orgId}
            aria-label="Application"
          >
            <option value="">Choose an application</option>
            {apps.map((a) => (
              <option key={a.id} value={a.id}>
                {a.name}
              </option>
            ))}
          </select>
          <form onSubmit={createApp}>
            <input
              name="name"
              placeholder="New application name"
              aria-label="New application name"
              required
              disabled={!orgId}
            />
            <input
              name="description"
              placeholder="What are you building?"
              aria-label="Application description"
              disabled={!orgId}
            />
            <input
              name="domains"
              placeholder="Allowed domains, comma separated"
              aria-label="Application domains"
              disabled={!orgId}
            />
            <input
              name="callbackUrl"
              type="url"
              placeholder="Callback URL (optional)"
              aria-label="Application callback URL"
              disabled={!orgId}
            />
            <button aria-label="Create application" disabled={!orgId}>
              <Plus size={16} />
            </button>
          </form>
        </section>
      </div>
      {orgId ? (
        <OrganizationAccess
          organization={orgs.find((organization) => organization.id === orgId)!}
          currentAccountId={session.accountId}
          onChanged={reloadOrganizations}
        />
      ) : null}
      {appId ? (
        <>
          <UsageAnalytics organizationId={orgId} applicationId={appId} />
          <div className="key-workspace">
            <div className="portal-section-head">
              <div>
                <p>03 · Credentials</p>
                <h3>API keys</h3>
              </div>
              <span>
                <KeyRound size={15} /> Secrets appear once
              </span>
            </div>
            {secret ? (
              <div className="secret-reveal" role="alert">
                <div>
                  <strong>Copy this key now</strong>
                  <code tabIndex={0}>{secret}</code>
                </div>
                <button
                  onClick={async () => {
                    await navigator.clipboard.writeText(secret);
                    setSecretCopied(true);
                  }}
                >
                  {secretCopied ? <Check size={16} /> : <Copy size={16} />}{" "}
                  {secretCopied ? "Copied" : "Copy"}
                </button>
                <button onClick={() => setSecret("")} disabled={!secretCopied}>
                  I have stored it
                </button>
              </div>
            ) : null}
            <form className="key-form" onSubmit={createKey}>
              <input
                name="name"
                placeholder="Key label"
                aria-label="Key label"
                required
              />
              <select
                name="class"
                value={keyClass}
                onChange={(e) => setKeyClass(e.target.value as typeof keyClass)}
                aria-label="Key class"
              >
                <option>TEST</option>
                <option>BROWSER</option>
                <option>SERVER</option>
              </select>
              <select
                name="environment"
                defaultValue="test"
                disabled={keyClass === "TEST"}
                aria-label="Key environment"
              >
                <option value="test">Test</option>
                <option value="live">Live</option>
              </select>
              <fieldset>
                <legend>Scopes</legend>
                {[
                  "locations:read",
                  "search:read",
                  "geocode:read",
                  "datasets:read",
                  "boundaries:read",
                  "graphql:access",
                  "grpc:access",
                ].map((scope) => (
                  <label key={scope}>
                    <input
                      type="checkbox"
                      name="scopes"
                      value={scope}
                      defaultChecked={[
                        "locations:read",
                        "search:read",
                      ].includes(scope)}
                      disabled={
                        keyClass === "BROWSER" && scope === "grpc:access"
                      }
                    />
                    {scope}
                  </label>
                ))}
              </fieldset>
              {keyClass === "BROWSER" ? (
                <p className="key-safety">
                  <ShieldCheck size={14} /> Browser keys require an allowed
                  origin and cannot receive server-only gRPC access.
                </p>
              ) : null}
              <input
                name="origins"
                placeholder="Allowed origins, comma separated"
                aria-label="Allowed origins"
                required={keyClass === "BROWSER"}
              />
              <input
                name="ips"
                placeholder="Allowed IPs or CIDRs, comma separated"
                aria-label="Allowed IP addresses or CIDRs"
              />
              <input
                name="expiresAt"
                type="datetime-local"
                aria-label="Key expiry date and time"
              />
              <button className="portal-primary">
                <Plus size={15} /> Create key
              </button>
            </form>
            <div className="key-list">
              {keys.length ? (
                keys.map((key) => (
                  <article
                    key={key.id}
                    className={key.revokedAt ? "is-revoked" : ""}
                  >
                    <div>
                      <span>
                        {key.class} · {key.environment}
                      </span>
                      <strong>{key.name}</strong>
                      <code>{key.prefix}</code>
                      <small>
                        {key.lastUsedAt
                          ? `Last used ${new Date(key.lastUsedAt).toLocaleString()}`
                          : "Never used"}
                        {key.expiresAt
                          ? ` · Expires ${new Date(key.expiresAt).toLocaleString()}`
                          : " · No expiry"}
                      </small>
                    </div>
                    <div>
                      {key.revokedAt ? (
                        <span>Revoked</span>
                      ) : (
                        <>
                          <button onClick={() => void keyAction(key, "rotate")}>
                            <RefreshCw size={14} /> Rotate
                          </button>
                          <button onClick={() => void keyAction(key, "revoke")}>
                            <Trash2 size={14} /> Revoke
                          </button>
                        </>
                      )}
                    </div>
                  </article>
                ))
              ) : (
                <p>
                  No keys yet. Create a test key for your first attributed
                  request.
                </p>
              )}
            </div>
          </div>
        </>
      ) : null}
    </section>
  );
}
