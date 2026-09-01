import assert from "node:assert/strict";
import { describe, it } from "node:test";
// @ts-expect-error Node's TypeScript test runner requires the source extension.
import { resolvePublicOrigin } from "./public-origins.ts";

const fallback = "https://console-geo.digitalghana.dev";

describe("portal resolvePublicOrigin", () => {
  it("accepts only exact credential-free HTTPS public origins in production", () => {
    assert.equal(resolvePublicOrigin("https://preview.example.com", fallback), "https://preview.example.com");
  });

  for (const configured of [
    "http://preview.example.com", "http://localhost:3102", "https://localhost:3102",
    "https://preview.example.com/path", "https://preview.example.com/?query=1",
    "https://preview.example.com/#fragment", "https://user:secret@preview.example.com", "not a URL",
  ]) it(`falls back for unsafe production input: ${configured}`, () => {
    assert.equal(resolvePublicOrigin(configured, fallback), fallback);
  });

  it("allows only loopback HTTP during development", () => {
    assert.equal(resolvePublicOrigin("http://localhost:4102", fallback, true), "http://localhost:4102");
    assert.equal(resolvePublicOrigin("http://127.0.0.1:4102", fallback, true), "http://127.0.0.1:4102");
    assert.equal(resolvePublicOrigin("http://preview.example.com", fallback, true), fallback);
  });
});
