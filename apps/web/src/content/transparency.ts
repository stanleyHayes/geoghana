/**
 * The public funding ledger (agent_plan.md §24 F6, story GEO-29.4).
 *
 * GEO-29.4 requires this page be "generated from recorded figures, never
 * hand-written prose about them". So the page below renders THIS file and
 * nothing else. If a number is not here, the page says so rather than
 * describing it in words.
 *
 * Every amount is in USD cents to avoid float drift, and `null` means
 * NOT YET RECORDED — which the page renders as an explicit gap, never as
 * zero. A public-good project that rounds its own books loses the right to
 * ask anyone to trust its data.
 */

export type Cents = number;

export interface LedgerLine {
  id: string;
  label: string;
  detail: string;
  /** null until a real invoice or payout statement exists. */
  amount: Cents | null;
}

export interface Period {
  id: string;
  label: string;
  /** ISO dates. A period appears here only once it has closed. */
  opened: string;
  closed: string;
  income: LedgerLine[];
  costs: LedgerLine[];
  /** What the money actually paid for, in plain words. */
  funded: string[];
}

export const CADENCE = {
  /** Stated cadence is part of the commitment — GEO-29.4. */
  frequency: "Quarterly",
  firstPeriodOpens: "the day the API accepts its first public request",
  note: "A period is published within 30 days of closing, whether or not anything came in.",
};

/**
 * Empty by construction: GhanaGeo has not launched, so no reporting period has
 * closed. Publishing a zero-row table would be honest; publishing an invented
 * one would not. This stays empty until the first period closes.
 */
export const PERIODS: readonly Period[] = [];

/**
 * The cost lines that WILL be reported, named now so the eventual report
 * cannot quietly omit one. Amounts stay null until billed.
 */
export const COST_LINES: readonly LedgerLine[] = [
  { id: "api", label: "API hosting", detail: "The Go service and its container host", amount: null },
  { id: "db", label: "Database", detail: "MongoDB cluster and backups", amount: null },
  { id: "search", label: "Search index", detail: "Typesense node", amount: null },
  { id: "cache", label: "Cache", detail: "Redis for rate limiting and hot reads", amount: null },
  { id: "cdn", label: "CDN and bandwidth", detail: "Bulk dataset downloads — the largest variable cost", amount: null },
  { id: "domain", label: "Domain and certificates", detail: "digitalghana.dev and subdomains", amount: null },
  { id: "data", label: "Data acquisition", detail: "Licensed source data where free sources fall short", amount: null },
];

export const INCOME_LINES: readonly LedgerLine[] = [
  { id: "donations", label: "One-off donations", detail: "Mobile money and card", amount: null },
  { id: "recurring", label: "Recurring donations", detail: "Monthly supporters", amount: null },
  { id: "sponsors", label: "Institutional sponsorship", detail: "Organisations funding running costs", amount: null },
  { id: "grants", label: "Grants", detail: "Named in full, including any conditions attached", amount: null },
];

/** The rules this page exists to make checkable. */
export const COMMITMENTS: readonly { rule: string; text: string }[] = [
  { rule: "F1", text: "No code path may charge a developer for access. There is no payment provider in the API." },
  { rule: "F2", text: "Fair-use limits are identical for everyone. Quota resolution cannot read donation state — a test asserts it." },
  { rule: "F3", text: "Sponsorship buys recognition, never capability. No higher limit, no priority support, no earlier data." },
  { rule: "F4", text: "A limit may be raised on documented need, never on payment. The reason is recorded in the audit log." },
  { rule: "F5", text: "Donation is never a dark pattern. No interstitial, no rate-limit page suggesting a donation as the fix." },
  { rule: "F6", text: "Funding is reported publicly. What came in, what it cost to run, and what it paid for." },
];

export function formatCents(c: Cents | null): string {
  if (c === null) return "—";
  return new Intl.NumberFormat("en-GH", {
    style: "currency", currency: "USD", minimumFractionDigits: 2,
  }).format(c / 100);
}
