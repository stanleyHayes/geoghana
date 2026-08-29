import type { ReactNode } from "react";
import Link from "next/link";
import { Badge } from "@ghanageo/ui";
import { ArrowUpRight, CheckCircle2, Compass, Layers3 } from "lucide-react";

/** Standard page heading, so every admin screen opens the same way. */
export function PageHeader({
  title, lede, actions, eyebrow,
}: {
  title: string;
  lede?: ReactNode | undefined;
  actions?: ReactNode | undefined;
  eyebrow?: string | undefined;
}) {
  return (
    <header style={{ marginBottom: "var(--space-6)" }}>
      {eyebrow ? (
        <p style={{ fontSize: "var(--text-2xs)", letterSpacing: ".14em", textTransform: "uppercase",
                    color: "var(--brand)", fontWeight: 800, margin: "0 0 var(--space-2)" }}>{eyebrow}</p>
      ) : null}
      <div className="gg-stack-row" style={{ alignItems: "flex-start", gap: "var(--space-4)" }}>
        <div style={{ flex: 1, minWidth: 240 }}>
          <h1 style={{ fontSize: "clamp(1.4rem, 4vw, 1.9rem)", margin: 0, lineHeight: 1.2 }}>{title}</h1>
          {lede ? (
            <p style={{ color: "var(--fg-muted)", margin: "var(--space-2) 0 0", maxWidth: "70ch" }}>{lede}</p>
          ) : null}
        </div>
        {actions ? <div className="gg-stack-row" style={{ flex: "0 0 auto" }}>{actions}</div> : null}
      </div>
    </header>
  );
}

/** An honest operator brief for modules whose live service contract is pending. */
export function PlannedScreen({
  title, story, purpose, capabilities, dependsOn,
}: {
  title: string;
  story?: string | undefined;
  purpose: string;
  capabilities: readonly string[];
  dependsOn?: string | undefined;
}) {
  return (
    <div className="admin-module">
      <PageHeader
        title={title}
        lede={purpose}
        eyebrow="Module brief"
        actions={story ? <Badge>{story}</Badge> : null}
      />

      <section className="admin-module__status" aria-labelledby="module-status-title">
        <div className="admin-module__status-icon"><Layers3 size={22} aria-hidden /></div>
        <div>
          <p className="admin-module__kicker">Operator workspace</p>
          <h2 id="module-status-title">Scope defined. Live data connection pending.</h2>
          <p>
            This destination is ready as an operator brief. Controls and records stay hidden
            until they can be backed by a real service contract and auditable data.
          </p>
        </div>
        <span className="admin-module__signal"><i /> No synthetic data</span>
      </section>

      <div className="admin-module__body">
        <section className="admin-module__capabilities" aria-labelledby="module-capabilities-title">
          <div className="admin-module__section-heading">
            <span>01</span>
            <div>
              <p>Approved scope</p>
              <h2 id="module-capabilities-title">What this workspace supports</h2>
            </div>
          </div>
          <ol>
            {capabilities.map((capability, index) => (
              <li key={capability}>
                <span>{String(index + 1).padStart(2, "0")}</span>
                <p>{capability}</p>
                <CheckCircle2 size={16} aria-hidden />
              </li>
            ))}
          </ol>
        </section>

        <aside className="admin-module__next" aria-labelledby="module-next-title">
          <div className="admin-module__section-heading">
            <span>02</span>
            <div><p>Readiness</p><h2 id="module-next-title">Operator next steps</h2></div>
          </div>
          <dl>
            <div><dt>Data policy</dt><dd>Real records only</dd></div>
            <div><dt>Service dependency</dt><dd>{dependsOn ?? "API contract and audit trail"}</dd></div>
            <div><dt>Tracking</dt><dd>{story ?? "Defined in the admin roadmap"}</dd></div>
          </dl>
          <div className="admin-module__live">
            <Compass size={18} aria-hidden />
            <div><strong>Use a live workspace</strong><p>These destinations already read the API.</p></div>
          </div>
          <nav aria-label="Available live admin workspaces">
            <Link href="/geography/regions">Regions <ArrowUpRight size={14} aria-hidden /></Link>
            <Link href="/geography/districts">Districts <ArrowUpRight size={14} aria-hidden /></Link>
            <Link href="/geography/places">Places <ArrowUpRight size={14} aria-hidden /></Link>
            <Link href="/explorer">Location Explorer <ArrowUpRight size={14} aria-hidden /></Link>
          </nav>
        </aside>
      </div>
    </div>
  );
}
