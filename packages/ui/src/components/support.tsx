"use client";

import { Database, Download, ExternalLink, Heart, Server, ShieldCheck } from "lucide-react";
import { Card } from "./primitives";

/**
 * The support surface for a product that is free and intends to stay that way.
 *
 * Rule F5 in agent_plan.md section 24: the ask is honest and dismissible. No
 * interstitial, no countdown, no guilt, and it never appears on a rate-limit
 * page where it would read as "pay to continue".
 */

export interface SupportFigures {
  /** Monthly running cost, in the smallest currency unit. Omit if unknown —
   *  a fabricated number is worse than no number. */
  monthlyCostMinor?: number;
  /** Received this month, same unit. */
  monthlyReceivedMinor?: number;
  currency?: string;
}

export interface Sponsor {
  id: string;
  name: string;
  url?: string;
  /** Months supported. The wall is ordered by longevity, not by amount, so it
   *  never becomes a leaderboard. */
  months: number;
}

function formatMinor(minor: number, currency: string): string {
  return new Intl.NumberFormat("en-GH", {
    style: "currency",
    currency,
    maximumFractionDigits: 0,
  }).format(minor / 100);
}

export function SupportPanel({
  figures,
  donateHref,
  compact = false,
}: {
  figures?: SupportFigures;
  donateHref: string;
  compact?: boolean;
}) {
  const currency = figures?.currency ?? "GHS";
  const hasFigures =
    figures?.monthlyCostMinor != null && figures?.monthlyReceivedMinor != null;
  const pct = hasFigures
    ? Math.min(100, Math.round((figures.monthlyReceivedMinor! / figures.monthlyCostMinor!) * 100))
    : null;
  const remaining = hasFigures
    ? Math.max(0, figures.monthlyCostMinor! - figures.monthlyReceivedMinor!)
    : null;

  return (
    <Card className="gg-support">
      <header className="gg-support__head">
        <span className="gg-support__icon"><Heart size={20} aria-hidden /></span>
        <div className="gg-support__intro">
          <p className="gg-support__eyebrow">Community funded · public by design</p>
          <h3 className="gg-support__title">Free for everyone. Funded by people who care.</h3>
          <p className="gg-support__lede">
            No paid tier, upgrade path or card required. Contributions keep the
            location layer reliable and openly available.
          </p>
        </div>
      </header>

      {hasFigures ? (
        <div className="gg-support__figures">
          <div className="gg-support__totals">
            <p><span>Raised this month</span><strong>{formatMinor(figures.monthlyReceivedMinor!, currency)}</strong></p>
            <p><span>Monthly operating target</span><strong>{formatMinor(figures.monthlyCostMinor!, currency)}</strong></p>
          </div>
          <div
            className="gg-support__meter"
            role="meter"
            aria-valuenow={pct!}
            aria-valuemin={0}
            aria-valuemax={100}
            aria-label={`Funded ${pct}% of this month's running cost`}
          >
            <span className="gg-support__meter-fill" style={{ width: `${pct}%` }} />
          </div>
          <div className="gg-support__figures-text"><strong>{pct}% funded</strong><span>{formatMinor(remaining!, currency)} still needed</span></div>
        </div>
      ) : null}

      {!compact ? (
        <div className="gg-support__costs" aria-label="What donations fund">
          <div><Server size={17} aria-hidden /><span><strong>Infrastructure</strong><small>API, search and CDN</small></span></div>
          <div><Database size={17} aria-hidden /><span><strong>Data stewardship</strong><small>Licensing and reconciliation</small></span></div>
          <div><Download size={17} aria-hidden /><span><strong>Open downloads</strong><small>Bulk dataset bandwidth</small></span></div>
        </div>
      ) : null}

      <div className="gg-support__actions">
        <a className="gg-button gg-button--primary gg-button--md" href={donateHref}>
          <Heart size={15} aria-hidden /> Support GhanaGeo
        </a>
        <a className="gg-button gg-button--ghost gg-button--md" href="/transparency">
          Where the money goes <ExternalLink size={14} aria-hidden />
        </a>
      </div>

      {/* Rule F3: sponsorship buys recognition, never capability. Saying so
          plainly is the honest thing and it also prevents the expectation. */}
      <p className="gg-support__note"><ShieldCheck size={16} aria-hidden /><span><strong>Support never buys access.</strong> Everyone receives the same rate limits, whether they donate or not.</span></p>
    </Card>
  );
}

export function SponsorWall({ sponsors }: { sponsors: Sponsor[] }) {
  if (sponsors.length === 0) {
    return null;
  }
  // Ordered by longevity rather than amount, so the wall never becomes a
  // leaderboard that implies bought influence.
  const ordered = [...sponsors].sort((a, b) => b.months - a.months);
  return (
    <section className="gg-sponsors" aria-labelledby="sponsors-heading">
      <h3 id="sponsors-heading" className="gg-sponsors__title">
        Supported by
      </h3>
      <ul className="gg-sponsors__list">
        {ordered.map((s) => (
          <li key={s.id} className="gg-sponsors__item">
            {s.url ? (
              <a href={s.url} rel="noopener noreferrer nofollow" target="_blank">
                {s.name}
              </a>
            ) : (
              <span>{s.name}</span>
            )}
            <span className="gg-sponsors__months">
              {s.months} {s.months === 1 ? "month" : "months"}
            </span>
          </li>
        ))}
      </ul>
    </section>
  );
}
