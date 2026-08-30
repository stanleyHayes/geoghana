import { describe, expect, it } from "vitest";
import { TOOL_NAV, resolvePublicOrigin } from "./site";

const fallback = "https://sandbox.geo.digitalghana.dev";

describe("resolvePublicOrigin", () => {
  it("never exports localhost as the non-development tool navigation default", () => {
    expect(TOOL_NAV.map(({ href }) => href)).toEqual([
      "https://sandbox.geo.digitalghana.dev",
      "https://console.geo.digitalghana.dev",
    ]);
  });

  it("keeps secure configured origins while discarding paths", () => {
    expect(resolvePublicOrigin("https://preview.example.com/sandbox?source=nav", fallback)).toBe(
      "https://preview.example.com",
    );
  });

  it("permits loopback HTTP for local development", () => {
    expect(resolvePublicOrigin("http://localhost:4101/workbench", fallback, true)).toBe("http://localhost:4101");
    expect(resolvePublicOrigin("http://127.0.0.1:4101", fallback, true)).toBe("http://127.0.0.1:4101");
  });

  it.each([
    "http://sandbox.example.com",
    "http://localhost:3101",
    "javascript:alert(1)",
    "https://operator:secret@sandbox.example.com",
    "not a URL",
  ])("falls back safely for an invalid public origin: %s", (configured) => {
    expect(resolvePublicOrigin(configured, fallback)).toBe(fallback);
  });
});
