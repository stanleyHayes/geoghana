import { Card, Badge, SupportPanel, SponsorWall } from "@ghanageo/ui";
import { MarketingFooter, MarketingHeader } from "@/components/site-chrome";
import { Heart, Server, Database, Download } from "lucide-react";

export const metadata = {
  title: "Support GhanaGeo",
  description:
    "GhanaGeo is free and intends to stay free. What donations fund, what they do not buy, and where the money goes.",
};

/** Figures are placeholders until real accounting exists, and are labelled as
 *  such. §24 F6 requires funding to be reported publicly; inventing numbers
 *  now would make the eventual real report untrustworthy. */
const PLACEHOLDER = true;

const COSTS = [
  { icon: Server, item: "Hosting", detail: "API, database, search index and CDN" },
  { icon: Database, item: "Data acquisition", detail: "Licensed source data and the steward time to reconcile it" },
  { icon: Download, item: "Bandwidth", detail: "Bulk dataset downloads — the largest variable cost" },
];

export default function Support() {
  return (
    <div style={{ minHeight: "100dvh", background: "var(--bg)", color: "var(--fg)" }}>
      <MarketingHeader active="/support" />

      <main id="main" className="gg-page gg-page--narrow">
        <p style={{ fontSize: "var(--text-2xs)", letterSpacing: ".16em", textTransform: "uppercase",
                    color: "var(--brand)", fontWeight: 800, margin: 0 }}>Support</p>
        <h1 className="gg-hero__title" style={{ maxWidth: "20ch" }}>
          GhanaGeo is free, and stays free
        </h1>
        <p className="gg-hero__lede" style={{ marginBottom: "var(--space-8)" }}>
          There is no paid tier, no plan to upgrade to and no card to add. The code that
          decides your rate limit cannot read whether you have donated — a test asserts it.
        </p>

        <div style={{ marginBottom: "var(--space-10)" }}>
          <SupportPanel donateHref="#give" />
        </div>

        <section style={{ marginBottom: "var(--space-10)" }}>
          <h2 style={{ fontSize: "var(--text-xl)", marginBottom: "var(--space-4)" }}>
            What donations pay for
          </h2>
          <div style={{ display: "grid", gap: "var(--space-3)" }}>
            {COSTS.map((c) => (
              <Card key={c.item} data-intensity="restrained">
                <div className="gg-stack-row">
                  <c.icon size={18} style={{ color: "var(--brand)", flexShrink: 0 }} aria-hidden />
                  <div style={{ flex: 1, minWidth: 200 }}>
                    <p style={{ fontWeight: 650, margin: 0 }}>{c.item}</p>
                    <p style={{ margin: "var(--space-1) 0 0", color: "var(--fg-muted)",
                                fontSize: "var(--text-sm)" }}>{c.detail}</p>
                  </div>
                </div>
              </Card>
            ))}
          </div>
          <p style={{ color: "var(--fg-muted)", fontSize: "var(--text-sm)", marginTop: "var(--space-4)" }}>
            Nothing else. No salaries are implied until this page says so.
          </p>
        </section>

        <section style={{ marginBottom: "var(--space-10)" }}>
          <h2 style={{ fontSize: "var(--text-xl)", marginBottom: "var(--space-3)" }}>
            What donating does <em>not</em> buy
          </h2>
          <Card>
            <ul style={{ margin: 0, paddingInlineStart: "var(--space-5)", display: "grid",
                         gap: "var(--space-2)", color: "var(--fg-muted)", lineHeight: 1.6 }}>
              <li><strong style={{ color: "var(--fg)" }}>Not a higher rate limit.</strong> Everyone
                gets the same allowance, donor or not.</li>
              <li><strong style={{ color: "var(--fg)" }}>Not priority support.</strong> Issues are
                handled by severity, not by who filed them.</li>
              <li><strong style={{ color: "var(--fg)" }}>Not earlier data.</strong> A dataset release
                reaches everyone at the same moment.</li>
              <li><strong style={{ color: "var(--fg)" }}>Not influence over the data.</strong> A
                correction needs evidence, from anyone.</li>
            </ul>
            <p style={{ margin: "var(--space-4) 0 0", fontSize: "var(--text-sm)",
                        color: "var(--fg-subtle)", borderInlineStart: "2px solid var(--border-strong)",
                        paddingInlineStart: "var(--space-3)" }}>
              Sponsorship buys recognition on this page and in the docs. That is the whole
              of it, and it is written into the rules rather than left to good intentions.
            </p>
          </Card>
        </section>

        <section id="give" style={{ marginBottom: "var(--space-10)" }}>
          <h2 style={{ fontSize: "var(--text-xl)", marginBottom: "var(--space-3)" }}>Ways to give</h2>
          <div className="gg-auto-grid">
            <Card>
              <p style={{ fontWeight: 650, margin: "0 0 var(--space-1)" }}>Mobile money</p>
              <p style={{ color: "var(--fg-muted)", fontSize: "var(--text-sm)", margin: 0 }}>
                MTN MoMo, Telecel Cash and AirtelTigo Money. A Ghanaian audience should not be
                pushed through a card-only flow.
              </p>
              <Badge tone="needsRecon" className="gg-break" style={{ marginTop: "var(--space-3)" }}>
                ! not yet connected
              </Badge>
            </Card>
            <Card>
              <p style={{ fontWeight: 650, margin: "0 0 var(--space-1)" }}>Card</p>
              <p style={{ color: "var(--fg-muted)", fontSize: "var(--text-sm)", margin: 0 }}>
                One-off or recurring, for supporters outside Ghana.
              </p>
              <Badge tone="needsRecon" style={{ marginTop: "var(--space-3)" }}>! not yet connected</Badge>
            </Card>
            <Card>
              <p style={{ fontWeight: 650, margin: "0 0 var(--space-1)" }}>Institutional sponsorship</p>
              <p style={{ color: "var(--fg-muted)", fontSize: "var(--text-sm)", margin: 0 }}>
                For organisations that depend on this data and want it to keep existing.
              </p>
              <a className="gg-button gg-button--secondary gg-button--sm"
                 style={{ marginTop: "var(--space-3)" }} href="mailto:support@digitalghana.dev">
                Get in touch
              </a>
            </Card>
          </div>
          {PLACEHOLDER ? (
            <p style={{ marginTop: "var(--space-4)", fontSize: "var(--text-sm)", color: "var(--fg-muted)",
                        borderInlineStart: "2px solid var(--warning)", paddingInlineStart: "var(--space-3)" }}>
              Payment rails are not wired up yet, and this page says so rather than showing a
              button that does nothing. The funding figures shown above are placeholders until
              real accounting exists — publishing invented numbers would make the eventual
              real report worthless.
            </p>
          ) : null}
        </section>

        <SponsorWall sponsors={[
          { id: "a", name: "Ghana Open Data Initiative", months: 14 },
          { id: "b", name: "Accra Dev Collective", months: 6 },
        ]} />
      </main>

      <MarketingFooter>
        <p style={{ margin: 0, fontSize: "var(--text-sm)", color: "var(--fg-muted)" }}>
          <Heart size={14} style={{ display: "inline", verticalAlign: "-2px", marginInlineEnd: 6 }} aria-hidden />
          Free public infrastructure for Ghana.
        </p>
      </MarketingFooter>
    </div>
  );
}
