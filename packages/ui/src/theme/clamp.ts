/**
 * Brand-hue safety: the two rules that stop a user's colour choice from
 * breaking the product.
 *
 *  1. RESERVED ARCS  — the brand hue may not sit where status colours live,
 *     or "published", "approved" and "added" become indistinguishable from
 *     the accent. (DESIGN_SYSTEM.md 6.6)
 *  2. CONTRAST CLAMP — the brand must clear 4.5:1 against its own foreground.
 *     The user's hue is always honoured; only its lightness is corrected.
 *     (DESIGN_SYSTEM.md 6.3)
 */

export type Mode = "light" | "dark";

/** Hue ranges owned by status colours and unavailable to the brand axis. */
export const RESERVED_ARCS: ReadonlyArray<{ from: number; to: number; reason: string }> = [
  { from: 130, to: 165, reason: "success" },
  { from: 10, to: 45, reason: "danger" },
];

export function reservedArcFor(hue: number): { from: number; to: number; reason: string } | null {
  const h = normalizeHue(hue);
  return RESERVED_ARCS.find((a) => h >= a.from && h <= a.to) ?? null;
}

export function normalizeHue(h: number): number {
  const x = h % 360;
  return x < 0 ? x + 360 : x;
}

/**
 * Snaps a hue out of a reserved arc to the nearer edge.
 * Returns the hue unchanged when it is already legal.
 */
export function snapOutOfReservedArc(hue: number): number {
  const h = normalizeHue(hue);
  const arc = reservedArcFor(h);
  if (!arc) return h;
  const distanceToLow = h - arc.from;
  const distanceToHigh = arc.to - h;
  // Step one degree clear of the boundary so the hue is never ON the edge.
  return distanceToLow < distanceToHigh ? normalizeHue(arc.from - 1) : normalizeHue(arc.to + 1);
}

// ---- Contrast -------------------------------------------------------------

/** OKLCH -> sRGB. Follows the CSS Color 4 conversion. */
export function oklchToRgb(L: number, C: number, hDeg: number): [number, number, number] {
  const h = (hDeg * Math.PI) / 180;
  const a = C * Math.cos(h);
  const b = C * Math.sin(h);

  const l_ = L + 0.3963377774 * a + 0.2158037573 * b;
  const m_ = L - 0.1055613458 * a - 0.0638541728 * b;
  const s_ = L - 0.0894841775 * a - 1.291485548 * b;

  const l = l_ ** 3;
  const m = m_ ** 3;
  const s = s_ ** 3;

  const lr = +4.0767416621 * l - 3.3077115913 * m + 0.2309699292 * s;
  const lg = -1.2684380046 * l + 2.6097574011 * m - 0.3413193965 * s;
  const lb = -0.0041960863 * l - 0.7034186147 * m + 1.707614701 * s;

  return [gamma(lr), gamma(lg), gamma(lb)];
}

function gamma(x: number): number {
  const v = x <= 0.0031308 ? 12.92 * x : 1.055 * Math.abs(x) ** (1 / 2.4) - 0.055;
  return Math.min(1, Math.max(0, v));
}

/** WCAG 2.x relative luminance. */
export function relativeLuminance([r, g, b]: [number, number, number]): number {
  const lin = (c: number) => (c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4);
  return 0.2126 * lin(r) + 0.7152 * lin(g) + 0.0722 * lin(b);
}

export function contrastRatio(a: [number, number, number], b: [number, number, number]): number {
  const la = relativeLuminance(a);
  const lb = relativeLuminance(b);
  const [hi, lo] = la > lb ? [la, lb] : [lb, la];
  return (hi + 0.05) / (lo + 0.05);
}

export interface ClampResult {
  /** The lightness to use for --brand. */
  lightness: number;
  /** The hue actually applied — may differ if it was inside a reserved arc. */
  hue: number;
  chroma: number;
  /** Near-white or near-black, whichever passes. */
  foreground: "light" | "dark";
  contrast: number;
  /** True when anything had to be adjusted, so the picker can say so. */
  adjusted: boolean;
  reason?: string;
}

const WHITE: [number, number, number] = [1, 1, 1];
const BLACK: [number, number, number] = [0, 0, 0];
const TARGET = 4.5;

/**
 * Finds a lightness (and, if needed, a foreground) that clears 4.5:1 for the
 * given hue. The hue is honoured; only lightness moves.
 */
export function clampBrand(hue: number, chroma: number, mode: Mode): ClampResult {
  const snapped = snapOutOfReservedArc(hue);
  const arcAdjusted = snapped !== normalizeHue(hue);
  const reason = arcAdjusted
    ? `Hue moved out of the ${reservedArcFor(hue)?.reason} range reserved for status colours.`
    : undefined;

  const startL = mode === "dark" ? 0.68 : 0.545;
  const step = mode === "dark" ? 0.02 : -0.02; // dark mode lightens, light darkens

  for (const fg of ["light", "dark"] as const) {
    const fgRgb = fg === "light" ? WHITE : BLACK;
    for (let i = 0; i <= 12; i++) {
      const L = Math.min(0.98, Math.max(0.08, startL + step * i));
      const ratio = contrastRatio(oklchToRgb(L, chroma, snapped), fgRgb);
      if (ratio >= TARGET) {
        return {
          lightness: round(L),
          hue: snapped,
          chroma,
          foreground: fg,
          contrast: round(ratio, 2),
          adjusted: arcAdjusted || i > 0 || fg === "dark",
          ...(reason ? { reason } : {}),
        };
      }
    }
  }

  // Unreachable in practice: 12 steps in both foregrounds always finds a pass.
  const L = mode === "dark" ? 0.9 : 0.25;
  const fg = mode === "dark" ? "dark" : "light";
  return {
    lightness: L,
    hue: snapped,
    chroma,
    foreground: fg,
    contrast: round(contrastRatio(oklchToRgb(L, chroma, snapped), fg === "light" ? WHITE : BLACK), 2),
    adjusted: true,
    reason: "Adjusted for readability.",
  };
}

function round(n: number, dp = 3): number {
  const f = 10 ** dp;
  return Math.round(n * f) / f;
}
