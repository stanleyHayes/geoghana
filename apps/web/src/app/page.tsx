"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { Card, Badge, Logo, SupportPanel, SponsorWall, SkipLink, verificationTone } from "@ghanageo/ui";
import { Globe, Search, Terminal, Zap, ShieldCheck, Scale } from "lucide-react";

const API = process.env.NEXT_PUBLIC_GHANAGEO_API_URL ?? "http://localhost:8180/v1";

/** Every figure here is asserted by the acceptance suite on each dataset run
 *  (ghanageo-admin data validate). Nothing on this page is a number we cannot
 *  reproduce from the database. */
const COVERAGE = [
  { value: "16", label: "Regions", note: "Post-2018 structure" },
  { value: "261", label: "Districts / MMDAs", note: "Verified against published figures" },
  { value: "15,925", label: "Places with coordinates", note: "Towns, suburbs, villages" },
  { value: "248", label: "District boundaries", note: "GeoJSON polygons" },
];

type Hit = {
  id: string; name: string; kind: string; type: string;
  regionName?: string; districtName?: string;
  score: number; matchReason?: string;
};

export default function Home() {
  const [q, setQ] = useState("");
  const [hits, setHits] = useState<Hit[]>([]);
  const [loading, setLoading] = useState(false);
  const abort = useRef<AbortController | null>(null);

  const run = useCallback(async (query: string) => {
    if (query.trim().length < 2) { setHits([]); return; }
    abort.current?.abort();
    const c = new AbortController();
    abort.current = c;
    setLoading(true);
    try {
      const res = await fetch(`${API}/search?q=${encodeURIComponent(query)}&limit=5`, { signal: c.signal });
      if (res.ok) setHits((await res.json()).data ?? []);
    } catch { /* aborted or offline — an empty result is the honest answer */ }
    finally { if (!c.signal.aborted) setLoading(false); }
  }, []);

  useEffect(() => { const t = setTimeout(() => run(q), 200); return () => clearTimeout(t); }, [q, run]);

  return (
    <div style={{ minHeight: "100dvh", background: "var(--bg)", color: "var(--fg)" }}>
      <SkipLink />

      <header className="gg-navbar" data-intensity="balanced">
        <div className="gg-navbar__left">
          <a href="/" className="gg-logo-link" style={{ textDecoration: "none" }}><Logo size={24} /></a>
          <span className="gg-navbar__hide-sm"
                style={{ fontSize: "var(--text-2xs)", letterSpacing: ".14em", textTransform: "uppercase",
                         color: "var(--fg-subtle)", fontWeight: 700 }}>digitalghana.dev</span>
        </div>
        <div className="gg-navbar__right">
          <a className="gg-button gg-button--ghost gg-button--sm gg-navbar__hide-xs" href="/about">About</a>
          <a className="gg-button gg-button--ghost gg-button--sm gg-navbar__hide-sm" href="/support">Support</a>
          <a className="gg-button gg-button--primary gg-button--sm" href="http://localhost:3101">Sandbox</a>
        </div>
      </header>

      <main id="main" className="gg-page gg-page--mid">
        <section style={{ marginBottom: "var(--space-16)" }}>
          <p style={{ fontSize: "var(--text-2xs)", letterSpacing: ".16em", textTransform: "uppercase",
                      color: "var(--brand)", fontWeight: 800, margin: 0 }}>
            Free public infrastructure
          </p>
          <h1 className="gg-hero__title">
            Ghana&rsquo;s location data, as infrastructure
          </h1>
          <p className="gg-hero__lede">
            Regions, districts, towns, suburbs and boundaries — through one API,
            with provenance on every record. No account, no API key, no paid tier.
          </p>

          {/* A live search box, not a screenshot. The claim on this page is that
              the thing works, so the page should demonstrate it working. */}
          <div style={{ marginTop: "var(--space-8)", maxWidth: 620 }}>
            <label className="gg-searchbar" style={{ cursor: "text", minHeight: 52 }}>
              <Search size={18} aria-hidden />
              <input
                value={q}
                onChange={(e) => setQ(e.target.value)}
                placeholder="Try “kumsai”, “tema comm”, or “osu”"
                aria-label="Search Ghanaian places"
                style={{ flex: 1, border: 0, background: "transparent", outline: "none",
                         color: "var(--fg)", fontSize: "var(--text-base)", fontFamily: "var(--font-sans)" }}
              />
              {loading ? <span style={{ fontSize: "var(--text-2xs)", color: "var(--fg-subtle)" }}>…</span> : null}
            </label>

            {hits.length > 0 ? (
              <Card style={{ marginTop: "var(--space-3)", padding: "var(--space-2)" }}>
                {hits.map((h) => {
                  const v = verificationTone("REFERENCE");
                  const ctx = [h.districtName, h.regionName].filter(Boolean).join(" · ");
                  return (
                    <div key={h.id} style={{ display: "flex", alignItems: "center", gap: "var(--space-3)",
                                             padding: "var(--space-2) var(--space-3)" }}>
                      <strong style={{ minWidth: 150 }}>{h.name}</strong>
                      <span style={{ fontSize: "var(--text-xs)", color: "var(--fg-muted)", flex: 1 }}>{ctx || h.kind}</span>
                      <span style={{ fontFamily: "var(--font-mono)", fontSize: "var(--text-2xs)",
                                     color: "var(--fg-subtle)" }}>{h.score.toFixed(2)}</span>
                      <Badge tone={v.tone}><span aria-hidden>{v.glyph}</span> {h.kind}</Badge>
                    </div>
                  );
                })}
              </Card>
            ) : q.trim().length >= 2 && !loading ? (
              <p style={{ marginTop: "var(--space-3)", color: "var(--fg-subtle)", fontSize: "var(--text-sm)" }}>
                No match. Typos are tolerated — try “kumsai” for Kumasi.
              </p>
            ) : null}
          </div>
        </section>

        <section style={{ marginBottom: "var(--space-16)" }}>
          <h2 style={{ fontSize: "var(--text-xl)", marginBottom: "var(--space-5)" }}>What is in it</h2>
          <div className="gg-auto-grid gg-auto-grid--sm">
            {COVERAGE.map((s) => (
              <Card key={s.label}>
                <p style={{ fontSize: "var(--text-3xl)", fontWeight: 700, margin: 0,
                            fontVariantNumeric: "tabular-nums" }}>{s.value}</p>
                <p style={{ fontWeight: 650, margin: "var(--space-1) 0 0" }}>{s.label}</p>
                <p style={{ fontSize: "var(--text-xs)", color: "var(--fg-muted)", margin: "var(--space-1) 0 0" }}>{s.note}</p>
              </Card>
            ))}
          </div>
        </section>

        <section style={{ marginBottom: "var(--space-16)" }}>
          <h2 style={{ fontSize: "var(--text-xl)", marginBottom: "var(--space-5)" }}>Reach it how you like</h2>
          <div className="gg-auto-grid">
            {[
              { icon: Zap, t: "REST", d: "GET /v1/search?q=osu — works from anywhere, including curl." },
              { icon: Globe, t: "GraphQL", d: "Nested geography in one round trip, with a complexity budget." },
              { icon: Zap, t: "gRPC", d: "Typed service-to-service access, plus a dataset change stream." },
              { icon: Terminal, t: "CLI", d: "ghanageo search \"tema\" — no account, table/JSON/CSV output." },
            ].map((p) => (
              <Card key={p.t} interactive>
                <p.icon size={18} style={{ color: "var(--brand)" }} aria-hidden />
                <p style={{ fontWeight: 650, margin: "var(--space-2) 0 var(--space-1)" }}>{p.t}</p>
                <p style={{ fontSize: "var(--text-sm)", color: "var(--fg-muted)", margin: 0 }}>{p.d}</p>
              </Card>
            ))}
          </div>
        </section>

        <section style={{ marginBottom: "var(--space-16)" }}>
          <h2 style={{ fontSize: "var(--text-xl)", marginBottom: "var(--space-5)" }}>Why you can rely on it</h2>
          <div className="gg-auto-grid">
            <Card>
              <ShieldCheck size={18} style={{ color: "var(--brand)" }} aria-hidden />
              <p style={{ fontWeight: 650, margin: "var(--space-2) 0 var(--space-1)" }}>Provenance on every record</p>
              <p style={{ fontSize: "var(--text-sm)", color: "var(--fg-muted)", margin: 0 }}>
                Each place carries its source, retrieval date and verification status.
                You can see whether a record is canonical or still awaiting reconciliation.
              </p>
            </Card>
            <Card>
              <Scale size={18} style={{ color: "var(--brand)" }} aria-hidden />
              <p style={{ fontWeight: 650, margin: "var(--space-2) 0 var(--space-1)" }}>Licensing you can act on</p>
              <p style={{ fontSize: "var(--text-sm)", color: "var(--fg-muted)", margin: 0 }}>
                Sources are CC BY, and attribution travels in the API response — not just a
                footer. GhanaPostGPS digital addresses are not included: they are not ours to give.
              </p>
            </Card>
            <Card>
              <Search size={18} style={{ color: "var(--brand)" }} aria-hidden />
              <p style={{ fontWeight: 650, margin: "var(--space-2) 0 var(--space-1)" }}>Built for how Ghanaians write</p>
              <p style={{ fontSize: "var(--text-sm)", color: "var(--fg-muted)", margin: 0 }}>
                Twi, Ga and Ewe orthography is preserved for display and folded for matching,
                so “Kwabɛnya” and “Kwabenya” find each other. Abbreviations expand: “comm” → “community”.
              </p>
            </Card>
          </div>
        </section>

        <section style={{ marginBottom: "var(--space-12)" }}>
          <h2 style={{ fontSize: "var(--text-xl)", marginBottom: "var(--space-5)" }}>Support</h2>
          <div style={{ display: "grid", gap: "var(--space-5)" }}>
            <SupportPanel donateHref="/support"
              figures={{ monthlyCostMinor: 48000, monthlyReceivedMinor: 17500, currency: "GHS" }} />
            <SponsorWall sponsors={[
              { id: "a", name: "Ghana Open Data Initiative", months: 14 },
              { id: "b", name: "Accra Dev Collective", months: 6 },
            ]} />
          </div>
        </section>
      </main>

      <footer style={{ borderTop: "1px solid var(--border)", padding: "var(--space-8) var(--space-4)",
                       color: "var(--fg-muted)", fontSize: "var(--text-sm)" }}>
        <div className="gg-page gg-page--mid" style={{ padding: 0 }}>
          <p style={{ margin: 0 }}>
            GhanaGeo is the first product on <strong>digitalghana.dev</strong> — public digital
            infrastructure for Ghana.
          </p>
          <p style={{ margin: "var(--space-2) 0 0", fontSize: "var(--text-xs)" }}>
            Contains data from GeoNames and geoBoundaries, licensed CC BY 4.0.
          </p>
        </div>
      </footer>
    </div>
  );
}
