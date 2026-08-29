import { describe, expect, it } from "vitest";
import { DATASET_VERSION, getRegions, searchOffline } from "../src";

describe("ghanageo umbrella package", () => {
  it("exposes online client types and the offline V1 dataset", () => {
    expect(DATASET_VERSION).toBe("2026.08.3-ulid");
    expect(getRegions()).toHaveLength(16);
    expect(searchOffline("Accra").length).toBeGreaterThan(0);
  });
});
