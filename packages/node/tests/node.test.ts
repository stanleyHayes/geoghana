import { describe, expect, it } from "vitest";
import { createServerClientFromEnv } from "../src";

describe("Node helpers", () => {
  it("reads server keys only when explicitly provided by server environment", () => {
    const client = createServerClientFromEnv({ GHANAGEO_API_URL: "https://example.test/v1", GHANAGEO_API_KEY: "server-fixture" });
    expect(client.baseUrl).toBe("https://example.test/v1");
  });
  it("supports anonymous server access", () => {
    expect(createServerClientFromEnv({}).baseUrl).toContain("api.geo.digitalghana.dev/v1");
  });
});
