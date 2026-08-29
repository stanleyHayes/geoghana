import { describe, expect, it, vi } from "vitest";
import { datasetVersion, getDistricts, getPlace, getRegions, search } from "../src";

describe("offline data", () => {
  it("exposes the independently versioned published corpus", () => {
    expect(datasetVersion).toBe("2026.08.3-ulid");
    expect(getRegions()).toHaveLength(16);
    expect(getDistricts()).toHaveLength(261);
  });
  it("searches without making a network call", () => {
    const runtime = globalThis as typeof globalThis & { fetch?: (...args: unknown[]) => unknown };
    const originalFetch = runtime.fetch;
    const fetchSpy = vi.fn();
    runtime.fetch = fetchSpy;
    try {
      const results = search("Kumasi");
      expect(results.some((place) => place.name === "Kumasi")).toBe(true);
      expect(getPlace(results[0]!.id)).toEqual(results[0]);
      expect(fetchSpy).not.toHaveBeenCalled();
    } finally {
      if (originalFetch) runtime.fetch = originalFetch;
      else delete runtime.fetch;
    }
  });
  it("keeps same-name places rather than guessing one", () => {
    expect(search("Kumasi", { limit: 100 }).length).toBeGreaterThan(1);
  });
});
