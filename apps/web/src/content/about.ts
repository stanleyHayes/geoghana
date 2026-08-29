/**
 * Marketing content for GhanaGeo.
 *
 * EVERY FIGURE HERE CARRIES A SOURCE AND A DATE. Nothing is estimated, rounded
 * for effect, or carried over from memory. A platform whose pitch is
 * "provenance on every record" cannot publish an unsourced number on its own
 * homepage — that is the fastest way to lose the developers we are asking to
 * depend on us.
 *
 * Facts that could not be verified are recorded in UNVERIFIED below rather than
 * quietly dropped, so nobody re-adds them later believing they were checked.
 */

export interface SourcedFact {
  value: string;
  label: string;
  source: string;
  url: string;
  asOf: string;
  /** Caveat that must travel with the figure wherever it is shown. */
  caveat?: string;
}

export const VISION = {
  headline: "Ghana should not have to rebuild its map every time somebody builds an app.",
  body: `Every delivery startup, every logistics team, every field survey and every
checkout form in Ghana solves the same problem from scratch: what is this place,
where is it, and which district is it in. Each rebuilds a partial answer, none
of them agree, and the work is thrown away when the company is.

GhanaGeo makes that answer public infrastructure. One canonical set of regions,
districts, towns and boundaries, with the source of every record attached, free
to use and free to leave.`,
};

export const MISSION = {
  headline: "Free, verifiable, and built to be depended on.",
  pillars: [
    {
      title: "Free, and structurally so",
      body: `No paid tier, no plan upgrade, no card. The code that decides your rate
limit cannot see whether you have donated — a test asserts it. Free products
drift to paid one exception at a time, so the rules are written down and enforced
rather than promised.`,
    },
    {
      title: "Verifiable, not just accurate",
      body: `Every record carries its source, retrieval date and verification status.
You can tell a canonical record from one still awaiting reconciliation against
official figures. District counts are asserted against published government
figures on every dataset run, not checked once and assumed.`,
    },
    {
      title: "Honest about its limits",
      body: `GhanaPostGPS digital addresses are not included: they belong to Ghana
Post and are not ours to give. Where our data is thin, the API says so instead
of guessing. An ambiguous search returns candidates with confidence scores
rather than a confident wrong answer.`,
    },
    {
      title: "Built for how Ghanaians actually write",
      body: `Twi, Ga and Ewe orthography is preserved for display and folded for
matching, so "Kwabɛnya" and "Kwabenya" find each other. Local abbreviations
expand: "tema comm" reaches Tema Community. Typos are tolerated.`,
    },
  ],
};

/** Why now. Sourced, current, and from primary documents where possible. */
export const CONTEXT: SourcedFact[] = [
  {
    value: "17.2%",
    label: "Growth in Ghana's Information & Communication sector, H1 2025",
    source: "Republic of Ghana, 2026 Budget Statement and Economic Policy, para. 62",
    url: "https://mofep.gov.gh/sites/default/files/budget-statements/2026-Budget-Statement-and-Economic-Policy.pdf",
    asOf: "Presented to Parliament, 13 November 2025",
  },
  {
    value: "328,387",
    label: "GitHub developers in Ghana, up 47% year on year",
    source: "GitHub Innovation Graph",
    url: "https://github.com/github/innovationgraph",
    asOf: "Q1 2026",
    caveat:
      "GitHub account holders, not employed software developers. The two are not interchangeable.",
  },
  {
    value: "8th",
    label: "Ghana's rank in Africa by GitHub developers",
    source: "GitHub Innovation Graph",
    url: "https://github.com/github/innovationgraph",
    asOf: "Q1 2026",
  },
  {
    value: "GH¢100m",
    label: "Allocated to the One Million Coders Programme",
    source: "Republic of Ghana, 2026 Budget Statement, Appendix expenditure tables",
    url: "https://mofep.gov.gh/sites/default/files/budget-statements/2026-Budget-Statement-and-Economic-Policy.pdf",
    asOf: "2026 financial year",
  },
  {
    value: "50%",
    label:
      "Share of Ghana's 2025 equity deals with female founders — the highest of any top-10 African market",
    source: "Partech Partners, 2025 Africa Tech Venture Capital Report",
    url: "https://partechpartners.com/",
    asOf: "Published 22 January 2026, covering 2025",
  },
];

/**
 * Facts we chose NOT to publish, and why. Kept in the codebase so nobody
 * re-adds them later assuming they were checked.
 */
export const UNVERIFIED = [
  {
    claim: "Ghana has ~18,000 professional software developers",
    why: `The only source is Google/Accenture's Africa Developer Ecosystem 2021,
published February 2022. No later edition exists. Presenting five-year-old data
as current would be misleading, and there is no current figure from this source.`,
  },
  {
    claim: "Ghana raised $X million in tech funding in 2025",
    why: `Partech reports US$90M across 23 deals; Disrupt Africa reports
US$41.2M across 8 startups. The gap is methodology — debt and undisclosed deals
are counted differently. Neither can be presented as "the" total without naming
the tracker, and Disrupt Africa simultaneously describes Ghana as "in steady
decline as a funding destination since 2022". Quoting the good number while
omitting that assessment would be selective.`,
  },
  {
    claim: "Ghana's current ICT workforce size",
    why: `The only official breakdown found is from 2014 (40,635 people, National
Communications Authority, using 2014 survey data). Nothing more recent could be
verified. We will not extrapolate.`,
  },
  {
    claim: "Chipper Cash as a Ghanaian success story",
    why: `Chipper Cash is San Francisco-headquartered with one Ghanaian
co-founder. Calling it a Ghanaian startup would be inaccurate.`,
  },
];

/** What the platform is for, beyond this one product. */
export const PLATFORM = {
  headline: "GhanaGeo is the first, not the whole.",
  body: `digitalghana.dev is a platform for public digital infrastructure in
Ghana. Location data is the first piece because so much else depends on it —
a delivery cannot be routed, a clinic cannot be found and an address cannot be
verified without it.

Each product that follows takes the same shape: one canonical dataset, open
protocols over it, provenance on every record, and free access. One account for
the platform, not one per product.`,
};

/** Who this is for, and what specifically breaks without it. */
export const AUDIENCES = [
  {
    who: "Delivery and logistics",
    breaks: "Routing to a place name nobody can resolve, and no way to group deliveries by district.",
  },
  {
    who: "E-commerce checkout",
    breaks: "Free-text address fields that produce undeliverable orders and no structured region or district.",
  },
  {
    who: "Fintech and KYC",
    breaks: "Address verification with nothing canonical to verify against.",
  },
  {
    who: "Health and NGO field operations",
    breaks: "Reconciling survey sites against official district boundaries by hand, every round.",
  },
  {
    who: "Researchers and journalists",
    breaks: "No open, citable dataset of Ghanaian geography with sources attached.",
  },
  {
    who: "Government and agencies",
    breaks: "Each department maintaining its own list, none of which agree.",
  },
];
