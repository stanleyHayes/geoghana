"use client";

import { useEffect, useRef, useState, type ReactNode } from "react";
import { Badge, Card, Skeleton, verificationTone } from "@ghanageo/ui";
import { AlertTriangle, SearchX } from "lucide-react";
import { ApiError } from "@/lib/api";

export interface Async<T> {
  data: T | null;
  loading: boolean;
  error: ApiError | Error | null;
}

/**
 * Load once per key change, cancelling the in-flight request.
 *
 * The AbortSignal is not decoration: a steward typing in a filter fires a
 * request per keystroke, and without cancellation a slow early response can
 * land after a fast later one and overwrite it with stale rows.
 */
export function useApi<T>(
  fn: (signal: AbortSignal) => Promise<T>,
  deps: unknown[],
): Async<T> {
  const [state, setState] = useState<Async<T>>({
    data: null,
    loading: true,
    error: null,
  });
  const fnRef = useRef(fn);
  // The callback is intentionally not an effect dependency: callers often
  // create it inline, while `deps` is the explicit request identity.
  // eslint-disable-next-line react-hooks/refs
  fnRef.current = fn;

  useEffect(() => {
    const ac = new AbortController();
    // A key change is a new remote state; showing stale rows as current would
    // be more misleading than this deliberate loading transition.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setState((s) => ({ ...s, loading: true, error: null }));
    fnRef.current(ac.signal).then(
      (data) => {
        if (!ac.signal.aborted) setState({ data, loading: false, error: null });
      },
      (error: Error) => {
        if (ac.signal.aborted || error.name === "AbortError") return;
        setState({ data: null, loading: false, error });
      },
    );
    return () => ac.abort();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps);

  return state;
}

/** Renders the three states a remote list can be in, so no screen invents its own. */
export function AsyncState<T>({
  state,
  empty,
  children,
}: {
  state: Async<T>;
  empty?: string;
  children: (data: T) => ReactNode;
}) {
  if (state.loading) {
    return (
      <Card
        className="admin-data-skeleton"
        data-intensity="restrained"
        role="status"
        aria-label="Loading data"
      >
        <div className="admin-data-skeleton__head">
          <Skeleton />
          <Skeleton />
        </div>
        {[0, 1, 2, 3, 4, 5].map((row) => (
          <div className="admin-data-skeleton__row" key={row}>
            <Skeleton />
            <Skeleton />
            <Skeleton />
            <Skeleton />
          </div>
        ))}
      </Card>
    );
  }
  if (state.error) {
    const code =
      state.error instanceof ApiError ? state.error.code : "UNREACHABLE";
    const reqId =
      state.error instanceof ApiError ? state.error.requestId : undefined;
    return (
      <Card>
        <div className="gg-stack-row" style={{ alignItems: "flex-start" }}>
          <AlertTriangle
            size={18}
            style={{ color: "var(--danger)", flexShrink: 0 }}
            aria-hidden
          />
          <div style={{ flex: 1, minWidth: 220 }}>
            <p style={{ margin: 0, fontWeight: 650 }}>{code}</p>
            <p
              style={{
                margin: "var(--space-1) 0 0",
                color: "var(--fg-muted)",
                fontSize: "var(--text-sm)",
              }}
            >
              {state.error.message}
            </p>
            {/* Quotable in a bug report — the whole point of returning it. */}
            {reqId ? (
              <p
                style={{
                  margin: "var(--space-2) 0 0",
                  fontFamily: "var(--font-mono)",
                  fontSize: "var(--text-xs)",
                  color: "var(--fg-subtle)",
                }}
              >
                requestId {reqId}
              </p>
            ) : (
              <p
                style={{
                  margin: "var(--space-2) 0 0",
                  color: "var(--fg-subtle)",
                  fontSize: "var(--text-xs)",
                }}
              >
                Is the API running?{" "}
                <code style={{ fontFamily: "var(--font-mono)" }}>make api</code>{" "}
                on :8180.
              </p>
            )}
          </div>
        </div>
      </Card>
    );
  }
  const d = state.data;
  if (d == null || (Array.isArray(d) && d.length === 0)) {
    return (
      <Card data-intensity="restrained">
        <div className="gg-stack-row" style={{ color: "var(--fg-muted)" }}>
          <SearchX size={16} aria-hidden />
          <span>{empty ?? "Nothing to show."}</span>
        </div>
      </Card>
    );
  }
  return <>{children(d)}</>;
}

/** The verification chip, identical on every screen that shows a record. */
export function VerificationBadge({ status }: { status: string }) {
  const v = verificationTone(status);
  return (
    <Badge tone={v.tone}>
      <span aria-hidden>{v.glyph}</span> {v.label}
    </Badge>
  );
}

/** Provenance is shown wherever a record is — never buried in a detail drawer. */
export function Provenance({
  source,
  url,
}: {
  source?: string | undefined;
  url?: string | undefined;
}) {
  if (!source) return <span style={{ color: "var(--fg-subtle)" }}>—</span>;
  return url ? (
    <a
      href={url}
      target="_blank"
      rel="noopener noreferrer"
      style={{ fontSize: "var(--text-xs)", color: "var(--fg-muted)" }}
    >
      {source}
    </a>
  ) : (
    <span style={{ fontSize: "var(--text-xs)", color: "var(--fg-muted)" }}>
      {source}
    </span>
  );
}
