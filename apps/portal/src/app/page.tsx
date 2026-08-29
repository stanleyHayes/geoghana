"use client";

import { Card, Badge, Logo, SkipLink, SupportPanel } from "@ghanageo/ui";
import { Globe, KeyRound, Activity, BookOpen, ShieldCheck } from "lucide-react";

export default function Portal() {
  return (
    <div style={{ minHeight: "100dvh", background: "var(--bg)", color: "var(--fg)" }}>
      <SkipLink />
      <header className="gg-navbar" data-intensity="balanced">
        <div className="gg-navbar__left">
          <a href="http://localhost:3100" className="gg-logo-link" style={{ textDecoration: "none" }}>
            <Logo size={22} suffix="Console" />
          </a>
        </div>
        <div className="gg-navbar__right">
          <a className="gg-button gg-button--ghost gg-button--sm" href="http://localhost:3101">Sandbox</a>
          <a className="gg-button gg-button--ghost gg-button--sm gg-navbar__hide-xs" href="http://localhost:3100">Home</a>
        </div>
      </header>

      <main id="main" className="gg-page gg-page--narrow">
        <h1 style={{ fontSize: "var(--text-2xl)", margin: "0 0 var(--space-2)" }}>Developer console</h1>
        <p style={{ color: "var(--fg-muted)", margin: "0 0 var(--space-6)", maxWidth: "62ch" }}>
          Keys, usage and logs. GhanaGeo is free, so a key is not required to call the API —
          it identifies heavy use for fair-use accounting and nothing else.
        </p>

<div className="gg-auto-grid" style={{ marginBottom: "var(--space-8)" }}>
          {[
            { icon: KeyRound, t: "API keys", d: "Create, rotate and revoke. Browser keys are origin-restricted; server keys are not for client bundles.", s: "GEO-16.4" },
            { icon: Activity, t: "Usage", d: "Requests, fair-use consumption, latency and errors by protocol and endpoint.", s: "GEO-16.5" },
            { icon: BookOpen, t: "Documentation", d: "OpenAPI reference, GraphQL explorer, protobuf docs and SDK quick starts.", s: "GEO-16.7" },
            { icon: ShieldCheck, t: "Security", d: "Passkeys and MFA, active sessions, audit history.", s: "GEO-16.8" },
          ].map((c) => (
            <Card key={c.t} interactive>
              <c.icon size={18} style={{ color: "var(--brand)" }} aria-hidden />
              <p style={{ fontWeight: 650, margin: "var(--space-2) 0 var(--space-1)" }}>{c.t}</p>
              <p style={{ fontSize: "var(--text-sm)", color: "var(--fg-muted)", margin: "0 0 var(--space-3)" }}>{c.d}</p>
              {/* Honest about what is built and what is not. A console that
                  pretends to have features it lacks wastes a developer's time. */}
              <Badge tone="needsRecon">! planned · {c.s}</Badge>
            </Card>
          ))}
        </div>

        <Card style={{ marginBottom: "var(--space-8)" }}>
          <p style={{ fontWeight: 650, margin: "0 0 var(--space-2)" }}>Start without an account</p>
          <pre style={{ margin: 0, padding: "var(--space-3)", background: "var(--bg-subtle)",
                        border: "1px solid var(--border)", borderRadius: "var(--mat-radius-sm)",
                        fontFamily: "var(--font-mono)", fontSize: "var(--text-xs)", lineHeight: 1.7,
                        overflowX: "auto" }}>
{`curl "https://api.geo.digitalghana.dev/v1/search?q=osu"

npx ghanageo search "tema comm"

npm install @ghanageo/react`}
          </pre>
        </Card>

        <SupportPanel donateHref="/support" compact
          figures={{ monthlyCostMinor: 48000, monthlyReceivedMinor: 17500, currency: "GHS" }} />
      </main>
    </div>
  );
}
