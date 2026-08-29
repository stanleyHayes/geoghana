"use client";

import { Heart, ExternalLink } from "lucide-react";
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

  return (
    <Card className="gg-support">
      <div className="gg-support__head">
        <Heart size={18} aria-hidden className="gg-support__icon" />
        <div>
          <h3 className="gg-support__title">GhanaGeo is free, and stays free</h3>
          <p className="gg-support__lede">
            No paid tier, no plan upgrade, no card. Donations cover hosting, data
            licensing and the bandwidth that bulk downloads consume.
          </p>
        </div>
      </div>

      {hasFigures ? (
        <div className="gg-support__figures">
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
          <p className="gg-support__figures-text">
            <strong>{formatMinor(figures.monthlyReceivedMinor!, currency)}</strong>
            {" of "}
            {formatMinor(figures.monthlyCostMinor!, currency)} this month&rsquo;s running cost
          </p>
        </div>
      ) : null}

      {!compact ? (
        <ul className="gg-support__list">
          <li>Hosting for the API, database, search index and CDN.</li>
          <li>Licensed source data, and the steward time to reconcile it.</li>
          <li>Bandwidth for bulk dataset downloads — the largest variable cost.</li>
        </ul>
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
      <p className="gg-support__note">
        Donating does not change your rate limits. Everyone gets the same
        allowance, whether they give or not.
      </p>
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
