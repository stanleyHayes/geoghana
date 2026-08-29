import { describe, expect, it } from "vitest";
import {
  clampBrand,
  contrastRatio,
  oklchToRgb,
  reservedArcFor,
  snapOutOfReservedArc,
} from "./clamp";

describe("reserved hue arcs", () => {
  it("rejects hues inside the success arc", () => {
    expect(reservedArcFor(150)?.reason).toBe("success");
  });
  it("rejects hues inside the danger arc", () => {
    expect(reservedArcFor(27)?.reason).toBe("danger");
  });
  it("allows the default meridian hue, 3deg clear of success", () => {
    expect(reservedArcFor(168)).toBeNull();
  });
  it("snaps out to the nearer edge", () => {
    expect(snapOutOfReservedArc(135)).toBe(129); // nearer the low edge of success
    expect(snapOutOfReservedArc(160)).toBe(166); // nearer the high edge
    expect(snapOutOfReservedArc(168)).toBe(168); // already legal, untouched
  });
  it("never returns a hue that is still reserved", () => {
    for (let h = 0; h < 360; h++) {
      expect(reservedArcFor(snapOutOfReservedArc(h))).toBeNull();
    }
  });
});

describe("contrast clamp", () => {
  // The guarantee: NO hue, in EITHER mode, can produce a failing pair.
  it("clears 4.5:1 for every hue in both modes", () => {
    for (const mode of ["light", "dark"] as const) {
      for (let h = 0; h < 360; h += 1) {
        const r = clampBrand(h, 0.115, mode);
        expect(r.contrast, `hue ${h} in ${mode} gave ${r.contrast}:1`).toBeGreaterThanOrEqual(4.5);
      }
    }
  });

  it("honours the hue and only moves lightness", () => {
    const r = clampBrand(200, 0.115, "light");
    expect(r.hue).toBe(200);
  });

  it("flags adjustment when a hue was snapped", () => {
    const r = clampBrand(140, 0.115, "light");
    expect(r.adjusted).toBe(true);
    expect(r.reason).toMatch(/success/);
  });

  it("handles a pale yellow, the classic unreadable brand", () => {
    const r = clampBrand(100, 0.16, "light");
    expect(r.contrast).toBeGreaterThanOrEqual(4.5);
  });
});

describe("colour maths", () => {
  it("converts OKLCH white to sRGB white", () => {
    const [r, g, b] = oklchToRgb(1, 0, 0);
    expect(r).toBeCloseTo(1, 1);
    expect(g).toBeCloseTo(1, 1);
    expect(b).toBeCloseTo(1, 1);
  });
  it("gives 21:1 for black on white", () => {
    expect(contrastRatio([0, 0, 0], [1, 1, 1])).toBeCloseTo(21, 0);
  });
});
