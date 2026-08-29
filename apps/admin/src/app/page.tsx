"use client";

import {
  Badge, Card, SponsorWall, SupportPanel, ThemePicker, verificationTone,
} from "@ghanageo/ui";
import { PageHeader } from "@/components/screen";

const COVERAGE = [
  { label: "Regions", value: "16", status: "REFERENCE" },
  { label: "Districts / MMDAs", value: "261", status: "SEED_NEEDS_CANONICAL_RECONCILIATION" },
  { label: "Places", value: "15,925", status: "REFERENCE" },
  { label: "Review queue", value: "12", status: "REVIEWED" },
];

export default function AdminHome() {
  return (
    <>
      <PageHeader
        title="Ghana’s location data, as infrastructure"
        lede={
          <>
            Current dataset <code style={{ fontFamily: "var(--font-mono)" }}>2026.08.3-ulid</code>.
            Press <kbd className="gg-kbd">⌘K</kbd> to search.
          </>
        }
      />

      <div className="gg-auto-grid" style={{ marginBottom: "var(--space-8)" }}>
        {COVERAGE.map((s) => {
          const v = verificationTone(s.status);
          return (
            <Card key={s.label} interactive>
              <p style={{ fontSize: "var(--text-2xs)", textTransform: "uppercase",
                          letterSpacing: "0.1em", color: "var(--fg-subtle)",
                          fontWeight: 700, margin: 0 }}>{s.label}</p>
              <p style={{ fontSize: "var(--text-3xl)", fontWeight: 700,
                          margin: "var(--space-2) 0", fontVariantNumeric: "tabular-nums" }}>{s.value}</p>
              <Badge tone={v.tone}><span aria-hidden>{v.glyph}</span> {v.label}</Badge>
            </Card>
          );
        })}
      </div>

      <h2 style={{ fontSize: "var(--text-xl)", marginBottom: "var(--space-4)" }}>Support</h2>
      <div style={{ display: "grid", gap: "var(--space-5)", marginBottom: "var(--space-8)" }}>
        <SupportPanel
          donateHref="http://localhost:3100/support"
          figures={{ monthlyCostMinor: 48000, monthlyReceivedMinor: 17500, currency: "GHS" }}
        />
        <SponsorWall
          sponsors={[
            { id: "a", name: "Ghana Open Data Initiative", months: 14 },
            { id: "b", name: "Accra Dev Collective", months: 6 },
            { id: "c", name: "Individual supporters", months: 3 },
          ]}
        />
      </div>

      <h2 style={{ fontSize: "var(--text-xl)", marginBottom: "var(--space-4)" }}>Appearance</h2>
      <ThemePicker />
    </>
  );
}
