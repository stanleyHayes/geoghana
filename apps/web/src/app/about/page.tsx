import { Card, Badge } from "@ghanageo/ui";
import { MarketingFooter, MarketingHeader } from "@/components/site-chrome";
import { ExternalLink } from "lucide-react";
import { VISION, MISSION, CONTEXT, PLATFORM, AUDIENCES, UNVERIFIED } from "@/content/about";

export const metadata = {
  title: "About GhanaGeo — vision, mission and sources",
  description:
    "Why GhanaGeo exists, what it commits to, and the sourced evidence behind every figure we publish.",
};

export default function About() {
  return (
    <div style={{ minHeight: "100dvh", background: "var(--bg)", color: "var(--fg)" }}>
      <MarketingHeader active="/about" />

      <main id="main" className="gg-page gg-page--narrow">
        <section style={{ marginBottom: "var(--space-16)" }}>
          <p style={{ fontSize: "var(--text-2xs)", letterSpacing: ".16em", textTransform: "uppercase",
                      color: "var(--brand)", fontWeight: 800, margin: 0 }}>Vision</p>
<h1 className="gg-hero__title" style={{ maxWidth: "24ch" }}>{VISION.headline}</h1>
          {VISION.body.trim().split("\n\n").map((p, i) => (
            <p key={i} style={{ color: "var(--fg-muted)", fontSize: "var(--text-base)",
                                lineHeight: 1.65, maxWidth: "66ch" }}>{p}</p>
          ))}
        </section>

        <section style={{ marginBottom: "var(--space-16)" }}>
          <p style={{ fontSize: "var(--text-2xs)", letterSpacing: ".16em", textTransform: "uppercase",
                      color: "var(--brand)", fontWeight: 800, margin: 0 }}>Mission</p>
          <h2 style={{ fontSize: "var(--text-2xl)", margin: "var(--space-3) 0 var(--space-6)" }}>
            {MISSION.headline}
          </h2>
          <div style={{ display: "grid", gap: "var(--space-4)" }}>
            {MISSION.pillars.map((p) => (
              <Card key={p.title}>
                <h3 style={{ fontSize: "var(--text-lg)", margin: "0 0 var(--space-2)" }}>{p.title}</h3>
                <p style={{ color: "var(--fg-muted)", margin: 0, lineHeight: 1.6 }}>
                  {p.body.replace(/\n/g, " ")}
                </p>
              </Card>
            ))}
          </div>
        </section>

        <section style={{ marginBottom: "var(--space-16)" }}>
          <h2 style={{ fontSize: "var(--text-2xl)", margin: "0 0 var(--space-2)" }}>Why now</h2>
          <p style={{ color: "var(--fg-muted)", margin: "0 0 var(--space-6)", maxWidth: "66ch" }}>
            Every figure below links to its primary source and states the date it applies to.
            We publish nothing we cannot point at.
          </p>
          <div style={{ display: "grid", gap: "var(--space-3)" }}>
            {CONTEXT.map((f) => (
              <Card key={f.label} data-intensity="restrained">
<div className="gg-stack-row">
                  <strong style={{ fontSize: "var(--text-2xl)", fontVariantNumeric: "tabular-nums",
                                   color: "var(--brand)", minWidth: 110 }}>{f.value}</strong>
                  <div style={{ flex: 1, minWidth: 240 }}>
                    <p style={{ margin: 0, fontWeight: 600 }}>{f.label}</p>
                    <p style={{ margin: "var(--space-1) 0 0", fontSize: "var(--text-xs)",
                                color: "var(--fg-subtle)" }}>
                      <a href={f.url} rel="noopener noreferrer" target="_blank"
                         style={{ color: "var(--fg-muted)" }}>
                        {f.source} <ExternalLink size={11} style={{ display: "inline" }} aria-hidden />
                      </a>
                      {" · "}{f.asOf}
                    </p>
                    {f.caveat ? (
                      <p style={{ margin: "var(--space-2) 0 0", fontSize: "var(--text-xs)",
                                  color: "var(--fg-muted)",
                                  borderInlineStart: "2px solid var(--border-strong)",
                                  paddingInlineStart: "var(--space-3)" }}>{f.caveat}</p>
                    ) : null}
                  </div>
                </div>
              </Card>
            ))}
          </div>
        </section>

        <section style={{ marginBottom: "var(--space-16)" }}>
          <h2 style={{ fontSize: "var(--text-2xl)", margin: "0 0 var(--space-5)" }}>Who this is for</h2>
          <div style={{ display: "grid", gap: "var(--space-2)" }}>
            {AUDIENCES.map((a) => (
              <div key={a.who} style={{ display: "flex", gap: "var(--space-4)", padding: "var(--space-3) 0",
                                        borderBottom: "1px solid var(--border)", flexWrap: "wrap" }}>
                <strong style={{ minWidth: 200 }}>{a.who}</strong>
                <span style={{ color: "var(--fg-muted)", flex: 1, minWidth: 260 }}>{a.breaks}</span>
              </div>
            ))}
          </div>
        </section>

        <section style={{ marginBottom: "var(--space-16)" }}>
          <h2 style={{ fontSize: "var(--text-2xl)", margin: "0 0 var(--space-3)" }}>{PLATFORM.headline}</h2>
          {PLATFORM.body.trim().split("\n\n").map((p, i) => (
            <p key={i} style={{ color: "var(--fg-muted)", lineHeight: 1.65, maxWidth: "66ch" }}>{p}</p>
          ))}
        </section>

        {/* Publishing what we deliberately did not claim is itself the argument
            for trusting what we did. */}
        <section>
          <h2 style={{ fontSize: "var(--text-xl)", margin: "0 0 var(--space-2)" }}>
            What we chose not to claim
          </h2>
          <p style={{ color: "var(--fg-muted)", margin: "0 0 var(--space-5)", maxWidth: "66ch" }}>
            Figures we considered and left out, with the reason. If the pitch is provenance,
            the pitch has to apply to the pitch.
          </p>
          <div style={{ display: "grid", gap: "var(--space-3)" }}>
            {UNVERIFIED.map((u) => (
              <Card key={u.claim} data-intensity="restrained">
                <Badge tone="needsRecon">! not published</Badge>
                <p style={{ fontWeight: 600, margin: "var(--space-2) 0 var(--space-1)" }}>{u.claim}</p>
                <p style={{ color: "var(--fg-muted)", margin: 0, fontSize: "var(--text-sm)",
                            lineHeight: 1.6 }}>{u.why.replace(/\n/g, " ")}</p>
              </Card>
            ))}
          </div>
        </section>
      </main>

      <MarketingFooter />
    </div>
  );
}
