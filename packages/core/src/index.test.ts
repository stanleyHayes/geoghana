import { describe, expect, it } from "vitest";

import { API_VERSION, DATASET_VERSION, DEFAULT_API_URL } from "./index";

describe("published metadata", () => {
  it("exposes the canonical API and dataset versions", () => {
    expect(API_VERSION).toBe("v1");
    expect(DATASET_VERSION).toBe("2026.08.3-ulid");
    expect(DEFAULT_API_URL).toBe("https://api-geo.digitalghana.dev/v1");
  });
});
