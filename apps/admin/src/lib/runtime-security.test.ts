import assert from "node:assert/strict";
import test from "node:test";

import { adminApiOrigin, publicPortalHref, publicWebHref } from "./runtime-config.ts";
import { safeReturnTo } from "./safe-return-to.ts";

test("return targets stay on the admin origin", () => {
  const origin = "https://admin-geo.digitalghana.dev";
  assert.equal(safeReturnTo("/activity?filter=mine", origin), "/activity?filter=mine");
  for (const target of ["https://evil.test", "//evil.test", "/%5cevil.test", "%252f%252fevil.test"]) {
    assert.equal(safeReturnTo(target, origin), "/");
  }
});

test("production runtime origins reject insecure or credentialed overrides", () => {
  assert.equal(adminApiOrigin({ NODE_ENV: "production", GHANAGEO_API_ORIGIN: "http://api.example.test" }), "https://api-geo.digitalghana.dev");
  assert.equal(adminApiOrigin({ NODE_ENV: "production", GHANAGEO_API_ORIGIN: "https://user:pass@api.example.test" }), "https://api-geo.digitalghana.dev");
  assert.equal(adminApiOrigin({ NODE_ENV: "production", GHANAGEO_API_ORIGIN: "https://api.example.test/path" }), "https://api.example.test");
  assert.equal(publicWebHref("//evil.test", { NODE_ENV: "production" }), "https://geo.digitalghana.dev/");
  assert.equal(publicPortalHref("/forgot-password", { NODE_ENV: "development" }), "http://localhost:3102/forgot-password");
  assert.equal(publicPortalHref("//evil.test", { NODE_ENV: "production" }), "https://console-geo.digitalghana.dev/");
});
