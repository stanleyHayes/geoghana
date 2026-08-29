import { Card, Badge } from "@ghanageo/ui";
import { MarketingFooter, MarketingHeader } from "@/components/site-chrome";
import { CalendarClock, FileText, ShieldCheck } from "lucide-react";
import {
  CADENCE, COMMITMENTS, COST_LINES, INCOME_LINES, PERIODS, formatCents,
} from "@/content/transparency";

export const metadata = {
  title: "Transparency — GhanaGeo",
  description:
    "What came in, what it cost to run, and what it paid for. The public funding ledger for GhanaGeo.",
};

const h2 = { fontSize: "var(--text-xl)", marginBottom: "var(--space-3)" } as const;
const muted = { color: "var(--fg-muted)", fontSize: "var(--text-sm)" } as const;

function LedgerTable({ title, lines }: { title: string; lines: readonly typeof COST_LINES[number][] }) {
  return (
    <Card data-intensity="restrained">
      <h3 style={{ fontSize: "var(--text-base)", margin: "0 0 var(--space-3)" }}>{title}</h3>
      {/* Wide content scrolls inside its own container so the page body never
          scrolls horizontally on a phone. */}
      <div style={{ overflowX: "auto" }}>
        <table className="gg-table" style={{ width: "100%", minWidth: 320 }}>
          <thead>
            <tr>
              <th scope="col">Line</th>
              <th scope="col" style={{ textAlign: "right", whiteSpace: "nowrap" }}>Amount</th>
            </tr>
          </thead>
          <tbody>
            {lines.map((l) => (
              <tr key={l.id}>
                <td>
                  <span style={{ fontWeight: 600 }}>{l.label}</span>
                  <span style={{ display: "block", ...muted }}>{l.detail}</span>
                </td>
                <td style={{ textAlign: "right", whiteSpace: "nowrap",
                             fontVariantNumeric: "tabular-nums",
                             color: l.amount === null ? "var(--fg-subtle)" : "var(--fg)" }}>
                  {formatCents(l.amount)}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <p style={{ ...muted, margin: "var(--space-3) 0 0" }}>
        <strong>—</strong> means not yet recorded. It never means zero.
      </p>
    </Card>
  );
}

export default function Transparency() {
  return (
    <div style={{ minHeight: "100dvh", background: "var(--bg)", color: "var(--fg)" }}>
      <MarketingHeader active="/transparency" />

      <main id="main" className="gg-page gg-page--narrow">
        <p style={{ fontSize: "var(--text-2xs)", letterSpacing: ".16em", textTransform: "uppercase",
                    color: "var(--brand)", fontWeight: 800, margin: 0 }}>Transparency</p>
        <h1 className="gg-hero__title" style={{ maxWidth: "22ch" }}>
          Where the money goes
        </h1>
        <p className="gg-hero__lede" style={{ marginBottom: "var(--space-8)" }}>
          A public-good project asking for public money shows its books. This page reports
          what came in, what it cost to run GhanaGeo, and what that paid for.
        </p>

        <section style={{ marginBottom: "var(--space-10)" }}>
          <Card>
            <div className="gg-stack-row">
              <CalendarClock size={20} style={{ color: "var(--brand)", flexShrink: 0 }} aria-hidden />
              <div style={{ flex: 1, minWidth: 220 }}>
                <p style={{ margin: 0, fontWeight: 650 }}>
                  {CADENCE.frequency}, starting from {CADENCE.firstPeriodOpens}.
                </p>
                <p style={{ margin: "var(--space-1) 0 0", ...muted }}>{CADENCE.note}</p>
              </div>
            </div>
          </Card>
        </section>

        <section style={{ marginBottom: "var(--space-10)" }}>
          <h2 style={h2}>Reporting periods</h2>
          {PERIODS.length === 0 ? (
            /* The honest empty state. A zero-filled table would imply the
               figures were checked and came to nothing; they have not been
               collected at all, and the difference matters. */
            <Card>
              <Badge>Nothing to report yet</Badge>
              <p style={{ margin: "var(--space-3) 0 0", lineHeight: 1.6 }}>
                GhanaGeo has not launched publicly, so no reporting period has closed and no
                donations have been accepted. There is nothing to report, and rather than fill
                this page with zeroes we are telling you that plainly.
              </p>
              <p style={{ margin: "var(--space-3) 0 0", ...muted }}>
                The first period opens {CADENCE.firstPeriodOpens}. The line items below are
                already named, so the first report cannot quietly leave one out.
              </p>
            </Card>
          ) : (
            <div style={{ display: "grid", gap: "var(--space-4)" }}>
              {PERIODS.map((p) => (
                <Card key={p.id}>
                  <h3 style={{ margin: 0 }}>{p.label}</h3>
                  <p style={{ ...muted, margin: "var(--space-1) 0 var(--space-4)" }}>
                    {p.opened} to {p.closed}
                  </p>
                  <LedgerTable title="Income" lines={p.income} />
                  <div style={{ height: "var(--space-4)" }} />
                  <LedgerTable title="Costs" lines={p.costs} />
                  {p.funded.length > 0 ? (
                    <>
                      <h4 style={{ margin: "var(--space-4) 0 var(--space-2)" }}>What it paid for</h4>
                      <ul style={{ margin: 0, paddingInlineStart: "var(--space-5)",
                                   display: "grid", gap: "var(--space-1)", ...muted }}>
                        {p.funded.map((f) => <li key={f}>{f}</li>)}
                      </ul>
                    </>
                  ) : null}
                </Card>
              ))}
            </div>
          )}
        </section>

        <section style={{ marginBottom: "var(--space-10)", display: "grid", gap: "var(--space-4)" }}>
          <h2 style={{ ...h2, marginBottom: 0 }}>The lines we will report</h2>
          <LedgerTable title="Income" lines={INCOME_LINES} />
          <LedgerTable title="Running costs" lines={COST_LINES} />
        </section>

        <section style={{ marginBottom: "var(--space-10)" }}>
          <h2 style={h2}>What this page is holding us to</h2>
          <Card>
            <ul style={{ margin: 0, padding: 0, listStyle: "none", display: "grid",
                         gap: "var(--space-3)" }}>
              {COMMITMENTS.map((c) => (
                <li key={c.rule} className="gg-stack-row" style={{ alignItems: "flex-start" }}>
                  <Badge>{c.rule}</Badge>
                  <span style={{ flex: 1, minWidth: 200, lineHeight: 1.55 }}>{c.text}</span>
                </li>
              ))}
            </ul>
            <p style={{ margin: "var(--space-4) 0 0", ...muted }}>
              <ShieldCheck size={14} style={{ display: "inline", verticalAlign: "-2px",
                                              marginInlineEnd: 6 }} aria-hidden />
              F2 and F3 are enforced by tests, not by good intentions.
            </p>
          </Card>
        </section>

        <section>
          <h2 style={h2}>Corrections</h2>
          <Card data-intensity="restrained">
            <p style={{ margin: 0, lineHeight: 1.6 }}>
              <FileText size={15} style={{ display: "inline", verticalAlign: "-2px",
                                           marginInlineEnd: 6, color: "var(--brand)" }} aria-hidden />
              If a figure here is wrong, say so and it gets corrected in place with the change
              noted — the same standard we hold the map data to.
            </p>
          </Card>
        </section>
      </main>

      <MarketingFooter />
    </div>
  );
}
