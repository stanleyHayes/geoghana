import { spawn, spawnSync } from "node:child_process";
import { once } from "node:events";
import { mkdtempSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { runConformance, type AdapterResult, type ConformanceAdapter } from "../../../tools/conformance/runner-kit";
import { createClientConformanceAdapter } from "./conformance.runner";

let server: ReturnType<typeof spawn>;
let baseUrl: string;
const repoRoot = fileURLToPath(new URL("../../..", import.meta.url));

beforeAll(async () => {
  server = spawn(process.execPath, ["--experimental-strip-types", "tools/conformance/fixture-server.ts"], { cwd: repoRoot, stdio: ["ignore", "pipe", "inherit"] });
  const [chunk] = await once(server.stdout!, "data") as [Buffer];
  baseUrl = `http://127.0.0.1:${Number(chunk.toString().trim())}/v1`;
});

afterAll(async () => {
  server.kill("SIGTERM");
  await once(server, "exit");
});

describe("public TypeScript client conformance runner", () => {
  it("produces an honest live-fixture audit through actual public client methods", async () => {
    const report = await runConformance(createClientConformanceAdapter(baseUrl), { language: "typescript", name: "@ghanageo/client", version: "0.1.0" });
    const failed = report.results.filter((result) => result.status === "failed").map((result) => result.caseId).sort();
    expect(failed).toEqual([]);
    expect(report.summary).toEqual({ passed: 42, failed: 0, skipped: 0 });
    expect(report.sdk.supportedProtocols).toEqual(["rest", "graphql"]);
    expect(report.datasetVersion).toBe("2026.08.3-ulid");
    expect(report.results.some((result) => result.caseId === "operation.list-datasets" && (result.protocols as string[]).includes("graphql"))).toBe(true);
    const reportPath = process.env.GHANAGEO_CONFORMANCE_REPORT ?? join(mkdtempSync(join(tmpdir(), "ghanageo-client-report-")), "report.json");
    writeFileSync(reportPath, JSON.stringify(report));
    expect(spawnSync("ruby", ["tools/conformance/validate.rb", "--report", reportPath], { cwd: repoRoot }).status).toBe(0);
  }, 30_000);

  it("records actual filtered REST, GraphQL error and multi-page wire requests", async () => {
    const adapter = createClientConformanceAdapter(baseUrl);
    const rest = await adapter.execute({
      id: "operation.list-places", operation: "listPlaces",
      input: { query: { regionId: "gh-region-ashanti", limit: 2 } },
    }, "rest");
    const restUrl = new URL((rest.evidence.request as { url: string }).url);
    expect(Object.fromEntries(restUrl.searchParams)).toMatchObject({ regionId: "gh-region-ashanti", limit: "2" });

    const graphql = await adapter.execute({
      id: "error.invalid-coordinates", operation: "reverseGeocode",
      input: { query: { lat: 91, lng: 0 } },
    }, "graphql");
    const graphqlBody = JSON.parse((graphql.evidence.request as { body: string }).body) as { query: string };
    expect(graphqlBody.query).toContain("latitude: 91");
    expect(graphqlBody.query).toContain("longitude: 0");
    expect(graphql.outcome).toBe("error");

    const pagination = await adapter.execute({
      id: "semantic.cursor-pagination", operation: "listPlaces",
      input: { fixture: "two-page-place-sequence", query: { limit: 2 } },
    }, "rest");
    expect(pagination.evidence.request).toHaveLength(3);
    expect(pagination.evidence.response).toHaveLength(3);
    expect((pagination.evidence.pagination?.firstCursor ?? "").length).toBeGreaterThanOrEqual(8);
  }, 30_000);

  it.each([
    ["semantic.search-protocol-parity", (result: AdapterResult, protocol: string) => protocol === "graphql" ? { ...result, value: { data: [], datasetVersion: "2026.08.3-ulid" } } : result],
    ["semantic.dataset-version-on-pages", (result: AdapterResult) => ({ ...result, value: { data: [] } })],
    ["semantic.cursor-pagination", (result: AdapterResult) => ({ ...result, value: { data: [], datasetVersion: "2026.08.3-ulid" } })],
    ["semantic.autocomplete-orthography", (result: AdapterResult) => ({ ...result, value: { data: [{ name: "Mampon" }], datasetVersion: "2026.08.3-ulid" } })],
    ["semantic.anonymous-default", (result: AdapterResult) => ({ ...result, evidence: { ...result.evidence, authorizationSent: true } })],
    ["semantic.cancellation-propagates", (result: AdapterResult) => ({ ...result, evidence: { ...result.evidence, cancelled: false } })],
    ["semantic.reverse-outside-ghana", (result: AdapterResult) => ({ ...result, value: { region: {}, nearby: [], datasetVersion: "2026.08.3-ulid" } })],
  ] as const)("rejects broken semantic behavior for %s", async (caseId, mutate) => {
    const base = createClientConformanceAdapter(baseUrl);
    const broken: ConformanceAdapter = {
      supportedProtocols: base.supportedProtocols,
      async execute(testCase, protocol) {
        const result = await base.execute(testCase, protocol);
        return testCase.id === caseId ? mutate(result, protocol) : result;
      },
    };
    const report = await runConformance(broken, { language: "typescript", name: "broken", version: "0.0.0" });
    expect(report.results.find((result) => result.caseId === caseId)?.status).toBe("failed");
  }, 30_000);

  it("rejects wrong result field types and incomplete typed errors", async () => {
    const base = createClientConformanceAdapter(baseUrl);
    const broken: ConformanceAdapter = {
      supportedProtocols: base.supportedProtocols,
      async execute(testCase, protocol) {
        const result = await base.execute(testCase, protocol);
        if (testCase.id === "operation.list-regions") return { ...result, value: { data: "not-an-array", datasetVersion: "2026.08.3-ulid" } };
        if (testCase.id === "error.invalid-argument") {
          const error = structuredClone(result.error) as { error: Record<string, unknown> };
          delete error.error.requestId;
          return { ...result, error };
        }
        return result;
      },
    };
    const report = await runConformance(broken, { language: "typescript", name: "broken", version: "0.0.0" });
    expect(report.results.find((result) => result.caseId === "operation.list-regions")?.status).toBe("failed");
    expect(report.results.find((result) => result.caseId === "error.invalid-argument")?.status).toBe("failed");
  }, 30_000);
});
