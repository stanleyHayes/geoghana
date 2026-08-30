import { spawn } from "node:child_process";
import assert from "node:assert/strict";
import { once } from "node:events";
import { mkdtemp, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { runConformance, type ConformanceAdapter, type Protocol, type RunnerCase } from "./runner-kit.ts";

const root = fileURLToPath(new URL("../..", import.meta.url));
const child = spawn(process.execPath, ["--experimental-strip-types", "tools/conformance/fixture-server.ts"], { cwd: root, stdio: ["ignore", "pipe", "inherit"] });
const [chunk] = await once(child.stdout!, "data") as [Buffer];
const port = Number(chunk.toString().trim());

const adapter: ConformanceAdapter = {
  supportedProtocols: ["rest", "grpc", "graphql"],
  async execute(testCase: RunnerCase, protocol: Protocol) {
    const request = { ...testCase, protocol };
    const response = await fetch(`http://127.0.0.1:${port}/v1/__conformance/${testCase.operation}?request=${encodeURIComponent(JSON.stringify(request))}`);
    if (!response.ok) throw new Error(`fixture server returned ${response.status}`);
    const result = await response.json();
    return {
      ...result,
      evidence: {
        ...result.evidence,
        request: { url: response.url, payload: request },
        ...(testCase.id === "semantic.cursor-pagination" ? { pagination: { firstIds: ["smoke-1"], secondIds: ["smoke-2"], firstCursor: "opaque-smoke-cursor", replayCursor: "opaque-smoke-cursor" } } : {}),
      },
    };
  },
};

try {
  const nestedResponse = await fetch(`http://127.0.0.1:${port}/v1/districts/gh-district-ahafo-asunafo-north/places?limit=2`, {
    headers: { "x-conformance-case": "operation.list-district-places" },
  });
  assert.equal(nestedResponse.status, 200);
  const nested = await nestedResponse.json() as { data: Array<Record<string, unknown>>; datasetVersion: string };
  const place = nested.data[0];
  assert.ok(place, "nested district places fixture must return a place");
  for (const field of ["id", "name", "normalizedName", "type", "aliases", "status", "verificationStatus", "provenance", "datasetVersion"]) {
    assert.ok(field in place, `nested district place is missing required Place field ${field}`);
  }
  assert.equal("region" in place, false, "nested district places must not return a District entity");
  assert.equal(nested.datasetVersion, "2026.08.3-ulid");

  const mismatchedPath = await fetch(`http://127.0.0.1:${port}/v1/districts/wrong/places?limit=2`, { headers: { "x-conformance-case": "operation.list-district-places" } });
  assert.equal(mismatchedPath.status, 422, "case header must not override a mismatched REST path");
  assert.match(await mismatchedPath.text(), /CONFORMANCE_FIXTURE_MISMATCH/);
  const mismatchedQuery = await fetch(`http://127.0.0.1:${port}/v1/search?q=Accra&limit=2`, { headers: { "x-conformance-case": "operation.search" } });
  assert.equal(mismatchedQuery.status, 422, "case header must not override mismatched REST query input");
  const mismatchedAuth = await fetch(`http://127.0.0.1:${port}/v1/regions?limit=2`, { headers: { "x-conformance-case": "semantic.anonymous-default", authorization: "Bearer should-not-be-sent" } });
  assert.equal(mismatchedAuth.status, 422, "anonymous fixture case must reject authorization");
  const mismatchedGraphql = await fetch(`http://127.0.0.1:${port}/graphql`, {
    method: "POST", headers: { "content-type": "application/json", "x-conformance-case": "operation.search" },
    body: JSON.stringify({ query: `query { search(query: "Accra", first: 2) { nodes { place { id } } } }` }),
  });
  assert.equal(mismatchedGraphql.status, 422, "case header must not override mismatched GraphQL arguments");
  const oversized = await fetch(`http://127.0.0.1:${port}/v1/search`, { headers: { "x-conformance-case": "error.payload-too-large" } });
  const oversizedBody = await oversized.text();
  assert.equal(oversized.status, 413);
  assert.ok(oversizedBody.length > 65_536, "oversized fixture must actually stream a body beyond the fixture limit");
  const deadlineStarted = Date.now();
  const deadline = await fetch(`http://127.0.0.1:${port}/v1/nearby`, { headers: { "x-conformance-case": "error.deadline-exceeded" } });
  assert.equal(deadline.status, 504);
  assert.ok(Date.now() - deadlineStarted >= 125, "deadline fixture must observably delay before timing out");
  const internal = await fetch(`http://127.0.0.1:${port}/v1/regions`, { headers: { "x-conformance-case": "error.internal" } });
  assert.equal(internal.status, 500);
  assert.match(await internal.text(), /Fixture INTERNAL/);

  const first = await runConformance(adapter, { language: "typescript", name: "fixture-adapter", version: "0.0.0" });
  const second = await runConformance(adapter, { language: "typescript", name: "fixture-adapter", version: "0.0.0" });
  if (JSON.stringify(first) !== JSON.stringify(second)) throw new Error("fixture reports are not deterministic");
  const directory = await mkdtemp(join(tmpdir(), "ghanageo-conformance-"));
  const reportPath = join(directory, "report.json");
  await writeFile(reportPath, JSON.stringify(first));
  const validator = spawn("ruby", ["tools/conformance/validate.rb", "--report", reportPath], { cwd: root, stdio: "inherit" });
  const [code] = await once(validator, "exit");
  if (code !== 0) process.exitCode = Number(code) || 1;
} finally {
  child.kill("SIGTERM");
  await once(child, "exit");
}
