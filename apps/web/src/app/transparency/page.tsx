import { Badge } from "@ghanageo/ui";
import { MarketingFooter, MarketingHeader } from "@/components/site-chrome";
import { ArrowRight, CalendarClock, FileText, ReceiptText, ShieldCheck } from "lucide-react";
import {
  CADENCE, COMMITMENTS, COST_LINES, INCOME_LINES, PERIODS, formatCents,
} from "@/content/transparency";

export const metadata = {
  title: "Transparency — GhanaGeo",
  description:
    "What came in, what it cost to run, and what it paid for. The public funding ledger for GhanaGeo.",
};

type LedgerLines = readonly typeof COST_LINES[number][];

function LedgerTable({ title, lines, index }: { title: string; lines: LedgerLines; index: string }) {
  return (
    <section className="transparency-ledger" aria-labelledby={`ledger-${index}`}>
      <header>
        <span aria-hidden>{index}</span>
        <h3 id={`ledger-${index}`}>{title}</h3>
        <small>{lines.length} named lines</small>
      </header>
      <div className="transparency-ledger__scroll">
        <table>
          <thead>
            <tr><th scope="col">Line item</th><th scope="col">Purpose</th><th scope="col">Amount</th></tr>
          </thead>
          <tbody>
            {lines.map((line) => (
              <tr key={line.id}>
                <th scope="row">{line.label}</th>
                <td>{line.detail}</td>
                <td data-empty={line.amount === null ? "true" : undefined}>{formatCents(line.amount)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <p><strong>—</strong> means not yet recorded. It never means zero.</p>
    </section>
  );
}

export default function Transparency() {
  return (
    <div className="site-page transparency-page">
      <MarketingHeader active="/transparency" />

      <main id="main">
        <section className="transparency-hero" aria-labelledby="transparency-title">
          <div className="transparency-hero__copy">
            <p className="site-eyebrow"><ReceiptText size={14} aria-hidden /> Public funding ledger</p>
            <h1 id="transparency-title">Where the money goes.</h1>
            <p>A public-good project asking for public money shows its books. This page reports what came in, what it cost to run GhanaGeo, and what that paid for.</p>
            <a href="#ledger">Inspect the ledger <ArrowRight size={16} aria-hidden /></a>
          </div>
          <aside className="transparency-hero__status" aria-label="Current reporting status">
            <span className="transparency-index" aria-hidden>Current status / 00</span>
            <strong>No closed period</strong>
            <p>GhanaGeo has not launched publicly and has not accepted donations.</p>
            <dl>
              <div><dt>Cadence</dt><dd>{CADENCE.frequency}</dd></div>
              <div><dt>First period</dt><dd>{CADENCE.firstPeriodOpens}</dd></div>
              <div><dt>Publication</dt><dd>Within 30 days of closing</dd></div>
            </dl>
          </aside>
          <div className="transparency-hero__grid" aria-hidden><i /><i /><i /><i /></div>
        </section>

        <section className="gg-page gg-page--mid transparency-cadence" aria-labelledby="cadence-title">
          <div>
            <p className="site-eyebrow"><CalendarClock size={14} aria-hidden /> Reporting clock</p>
            <span className="transparency-index" aria-hidden>01 / Cadence</span>
          </div>
          <div>
            <h2 id="cadence-title">{CADENCE.frequency}, from the first public request.</h2>
            <p>{CADENCE.note}</p>
          </div>
        </section>

        <section className="transparency-periods" aria-labelledby="periods-title">
          <div className="gg-page gg-page--mid">
            <header>
              <div>
                <p className="site-eyebrow">Reporting periods</p>
                <span className="transparency-index" aria-hidden>02 / Published books</span>
              </div>
              <h2 id="periods-title">{PERIODS.length === 0 ? "Nothing to report yet." : "Closed reporting periods."}</h2>
            </header>

            {PERIODS.length === 0 ? (
              <div className="transparency-empty">
                <span aria-hidden>—</span>
                <div>
                  <Badge>Pre-launch</Badge>
                  <h3>An empty ledger is more honest than a row of zeroes.</h3>
                  <p>GhanaGeo has not launched publicly, so no reporting period has closed and no donations have been accepted. There is nothing to report, and rather than fill this page with zeroes we are telling you that plainly.</p>
                </div>
                <p>The first period opens {CADENCE.firstPeriodOpens}. The line items below are already named, so the first report cannot quietly leave one out.</p>
              </div>
            ) : (
              <div className="transparency-period-list">
                {PERIODS.map((period) => (
                  <article key={period.id}>
                    <header><h3>{period.label}</h3><p>{period.opened} to {period.closed}</p></header>
                    <LedgerTable title="Income" lines={period.income} index={`${period.id}-income`} />
                    <LedgerTable title="Costs" lines={period.costs} index={`${period.id}-costs`} />
                    {period.funded.length > 0 ? (
                      <section><h4>What it paid for</h4><ul>{period.funded.map((item) => <li key={item}>{item}</li>)}</ul></section>
                    ) : null}
                  </article>
                ))}
              </div>
            )}
          </div>
        </section>

        <section id="ledger" className="gg-page gg-page--mid transparency-lines" aria-labelledby="lines-title">
          <header>
            <div>
              <p className="site-eyebrow">Named in advance</p>
              <span className="transparency-index" aria-hidden>03 / Account structure</span>
            </div>
            <div>
              <h2 id="lines-title">The lines we will report.</h2>
              <p>Naming every category before money arrives makes omissions visible later.</p>
            </div>
          </header>
          <LedgerTable title="Income" lines={INCOME_LINES} index="01" />
          <LedgerTable title="Running costs" lines={COST_LINES} index="02" />
        </section>

        <section className="transparency-commitments" aria-labelledby="commitments-title">
          <div className="gg-page gg-page--mid">
            <header>
              <div><p className="site-eyebrow"><ShieldCheck size={14} aria-hidden /> Funding firewall</p><span className="transparency-index" aria-hidden>04 / Commitments</span></div>
              <h2 id="commitments-title">What this page is holding us to.</h2>
            </header>
            <ol>
              {COMMITMENTS.map((commitment) => (
                <li key={commitment.rule}>
                  <span>{commitment.rule}</span>
                  <p>{commitment.text}</p>
                </li>
              ))}
            </ol>
            <p className="transparency-commitments__proof"><ShieldCheck size={16} aria-hidden /> F2 and F3 are enforced by tests, not by good intentions.</p>
          </div>
        </section>

        <section className="gg-page gg-page--mid transparency-corrections" aria-labelledby="corrections-title">
          <div><FileText size={22} aria-hidden /><span className="transparency-index">05 / Corrections</span></div>
          <div>
            <h2 id="corrections-title">The ledger can be corrected. The correction cannot disappear.</h2>
            <p>If a figure here is wrong, say so and it gets corrected in place with the change noted — the same standard we hold the map data to.</p>
          </div>
        </section>
      </main>

      <MarketingFooter />
    </div>
  );
}
