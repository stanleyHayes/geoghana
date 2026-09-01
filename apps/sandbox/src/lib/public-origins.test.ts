import assert from "node:assert/strict";
import { describe, it } from "node:test";
// @ts-expect-error Node's TypeScript test runner requires the source extension.
import { resolvePublicOrigin } from "./public-origins.ts";

const fallback = "https://sandbox-geo.digitalghana.dev";

describe("sandbox resolvePublicOrigin", () => {
  it("accepts only exact credential-free HTTPS public origins in production", () => {
    assert.equal(resolvePublicOrigin("https://preview.example.com", fallback), "https://preview.example.com");
  });

  for (const configured of [
    "http://preview.example.com", "http://localhost:3101", "https://localhost:3101",
    "https://preview.example.com/path", "https://preview.example.com/?query=1",
    "https://preview.example.com/#fragment", "https://user:secret@preview.example.com", "not a URL",
  ]) it(`falls back for unsafe production input: ${configured}`, () => {
    assert.equal(resolvePublicOrigin(configured, fallback), fallback);
  });

  it("allows only loopback HTTP during development", () => {
    assert.equal(resolvePublicOrigin("http://localhost:4101", fallback, true), "http://localhost:4101");
    assert.equal(resolvePublicOrigin("http://127.0.0.1:4101", fallback, true), "http://127.0.0.1:4101");
    assert.equal(resolvePublicOrigin("http://preview.example.com", fallback, true), fallback);
  });
});
