"use client";

import { useCallback, useState } from "react";
import { Card, Badge, SkipLink } from "@ghanageo/ui";
import { Play, Terminal, Globe } from "lucide-react";

const API = process.env.NEXT_PUBLIC_GHANAGEO_API_URL ?? "http://localhost:8180/v1";

/** Sample queries from Spec §15, chosen because each demonstrates something
 *  specific rather than just returning a result. */
const SAMPLES = [
  { label: "Search with a typo", path: "/search?q=kumsai", why: "Typo tolerance: finds Kumasi" },
  { label: "Abbreviation", path: "/search?q=tema%20comm", why: "“comm” expands to “community”" },
  { label: "Ghanaian orthography", path: "/search?q=kwabenya", why: "Matches Kwabɛnya" },
  { label: "Reverse geocode", path: "/reverse?lat=6.688&lng=-1.624", why: "Kumasi → region + district" },
  { label: "Nearby", path: "/nearby?lat=5.556&lng=-0.182&radius=2000", why: "Places around central Accra" },
  { label: "Boundary", path: "/boundaries/gh-region-greater-accra", why: "GeoJSON polygon" },
  { label: "List regions", path: "/regions?limit=5", why: "Cursor pagination" },
];

export default function Sandbox() {
  const [path, setPath] = useState(SAMPLES[0]!.path);
  const [body, setBody] = useState("");
  const [status, setStatus] = useState<number | null>(null);
  const [ms, setMs] = useState<number | null>(null);
  const [cost, setCost] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const send = useCallback(async (p: string) => {
    setBusy(true);
    const t0 = performance.now();
    try {
      const res = await fetch(API + p);
      setStatus(res.status);
      setCost(res.headers.get("X-RateLimit-Cost"));
      const json = await res.json();
      setBody(JSON.stringify(json, null, 2));
    } catch (err) {
      setStatus(null);
      setBody(`Could not reach ${API}\n\n${String(err)}`);
    } finally {
      setMs(Math.round(performance.now() - t0));
      setBusy(false);
    }
  }, []);

  return (
    <div style={{ minHeight: "100dvh", background: "var(--bg)", color: "var(--fg)" }}>
      <SkipLink />
      <header className="gg-navbar" data-intensity="balanced">
        <div className="gg-navbar__left">
          <Globe size={20} style={{ color: "var(--brand)" }} aria-hidden />
          <strong style={{ fontFamily: "var(--font-display)" }}>GhanaGeo</strong>
          <span style={{ fontSize: "var(--text-2xs)", textTransform: "uppercase", letterSpacing: ".14em",
                         color: "var(--fg-subtle)", fontWeight: 700 }}>Sandbox</span>
        </div>
        <div className="gg-navbar__right">
          <span className="gg-env gg-env--sandbox">Sandbox</span>
          <a className="gg-button gg-button--ghost gg-button--sm" href="http://localhost:3100">Home</a>
        </div>
      </header>

      <main id="main" style={{ maxWidth: 1200, margin: "0 auto", padding: "var(--space-8) var(--space-6)" }}>
        <h1 style={{ fontSize: "var(--text-2xl)", margin: "0 0 var(--space-2)" }}>Try it, no account needed</h1>
        <p style={{ color: "var(--fg-muted)", margin: "0 0 var(--space-6)", maxWidth: "62ch" }}>
          These requests run against the live API with no credential at all. GhanaGeo is
          free — anonymous access is a supported path, not a trial.
        </p>

        <div style={{ display: "grid", gridTemplateColumns: "minmax(260px,340px) 1fr", gap: "var(--space-5)",
                      alignItems: "start" }}>
          <div style={{ display: "grid", gap: "var(--space-2)" }}>
            {SAMPLES.map((s) => (
              <button key={s.path}
                onClick={() => { setPath(s.path); void send(s.path); }}
                className="gg-card"
                style={{ textAlign: "start", cursor: "pointer", padding: "var(--space-3) var(--space-4)",
                         border: path === s.path ? "1px solid var(--brand)" : undefined }}>
                <span style={{ fontWeight: 650, fontSize: "var(--text-sm)" }}>{s.label}</span>
                <span style={{ display: "block", fontSize: "var(--text-xs)", color: "var(--fg-muted)",
                               marginTop: 2 }}>{s.why}</span>
              </button>
            ))}
          </div>

          <div style={{ display: "grid", gap: "var(--space-3)" }}>
            <div style={{ display: "flex", gap: "var(--space-2)" }}>
              <span className="gg-input" style={{ display: "flex", alignItems: "center", gap: "var(--space-2)",
                                                   fontFamily: "var(--font-mono)", fontSize: "var(--text-xs)" }}>
                <span style={{ color: "var(--fg-subtle)" }}>GET {API}</span>
                <input value={path} onChange={(e) => setPath(e.target.value)}
                  aria-label="Request path"
                  style={{ flex: 1, border: 0, background: "transparent", outline: "none",
                           color: "var(--fg)", fontFamily: "var(--font-mono)", fontSize: "var(--text-xs)" }} />
              </span>
              <button className="gg-button gg-button--primary gg-button--md" disabled={busy}
                onClick={() => void send(path)}>
                <Play size={15} aria-hidden /> Send
              </button>
            </div>

            {status !== null ? (
              <div style={{ display: "flex", gap: "var(--space-3)", alignItems: "center",
                            fontSize: "var(--text-xs)", color: "var(--fg-muted)" }}>
                <Badge tone={status < 400 ? "canonical" : "danger"}>HTTP {status}</Badge>
                <span>{ms} ms</span>
                {cost ? <span>cost {cost} unit{cost === "1" ? "" : "s"}</span> : null}
              </div>
            ) : null}

            <pre data-intensity="restrained" style={{
              margin: 0, padding: "var(--space-4)", background: "var(--bg-subtle)",
              border: "1px solid var(--border)", borderRadius: "var(--mat-radius-sm)",
              fontFamily: "var(--font-mono)", fontSize: "var(--text-xs)", lineHeight: 1.55,
              maxHeight: "60vh", overflow: "auto",
            }}>{body || "Pick a sample on the left, or edit the path and press Send."}</pre>

            <Card data-intensity="restrained">
              <p style={{ fontWeight: 650, margin: "0 0 var(--space-2)", display: "flex",
                          alignItems: "center", gap: "var(--space-2)" }}>
                <Terminal size={15} aria-hidden /> The same request, elsewhere
              </p>
              <pre style={{ margin: 0, fontFamily: "var(--font-mono)", fontSize: "var(--text-xs)",
                            color: "var(--fg-muted)", lineHeight: 1.7 }}>
{`curl "${API}${path}"

ghanageo ${path.startsWith("/search") ? 'search "kumsai"' : path.startsWith("/reverse") ? "reverse 6.688 -1.624" : "regions"}`}
              </pre>
            </Card>
          </div>
        </div>
      </main>
    </div>
  );
}
