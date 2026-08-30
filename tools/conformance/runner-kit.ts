import { createHash } from "node:crypto";
import { execFileSync } from "node:child_process";
import { fileURLToPath } from "node:url";

export const RUNNER_VERSION = "1.0.0";
export const CANONICAL_DATASET_VERSION = "2026.08.3-ulid";
export type Protocol = "rest" | "grpc" | "graphql";
export type RunnerCase = { id: string; operation: string; input: Record<string, unknown> };
type ExpectedCase = RunnerCase & {
  required: boolean;
  protocols: Protocol[];
  expect: {
    outcome: "success" | "error";
    httpStatus?: number;
    shape: { type: "object" | "array" | "binary"; required: string[]; fieldTypes?: Record<string, "object" | "array" | "string" | "number" | "boolean"> };
    error?: { code: string; requiredFields: string[] };
    quotaCost: Record<string, unknown>;
    semantics?: string[];
  };
};
export type AdapterEvidence = { request: unknown; response: unknown; authorizationSent?: boolean; cancelled?: boolean; pagination?: { firstIds: string[]; secondIds: string[]; firstCursor: string; replayCursor: string } };
export type AdapterResult = { outcome: "success" | "error"; status?: number; value?: unknown; error?: unknown; quotaCost: Record<string, unknown>; evidence: AdapterEvidence };
export interface ConformanceAdapter { supportedProtocols: Protocol[]; execute(testCase: RunnerCase, protocol: Protocol): Promise<AdapterResult> }

function exportedContract(): { contractDigest: string; cases: ExpectedCase[] } {
  const root = fileURLToPath(new URL("../..", import.meta.url));
  return JSON.parse(execFileSync("ruby", ["tools/conformance/export_cases.rb"], { cwd: root, encoding: "utf8" }));
}
function pathValue(value: unknown, path: string): unknown {
  return path.split(".").reduce<unknown>((current, segment) => current && typeof current === "object" ? (current as Record<string, unknown>)[segment] : undefined, value);
}
function typeOf(value: unknown): string { return Array.isArray(value) ? "array" : value === null ? "null" : typeof value; }
function expectedFieldType(path: string): "object" | "array" | "string" | "number" | undefined {
  const leaf = path.split(".").at(-1)!;
  if (["data", "aliases", "downloads", "nearby"].includes(leaf)) return "array";
  if (["error", "provenance", "region", "geometry", "properties", "change", "details"].includes(leaf)) return "object";
  if (["latitude", "longitude", "maxRadiusMeters", "minLength", "retryAfterSeconds", "limit", "cost", "maxCost", "depth", "maxDepth"].includes(leaf)) return "number";
  return "string";
}
function stable(value: unknown): string {
  if (Array.isArray(value)) return `[${value.map(stable).join(",")}]`;
  if (value && typeof value === "object") return `{${Object.entries(value).sort(([a], [b]) => a.localeCompare(b)).map(([key, child]) => `${JSON.stringify(key)}:${stable(child)}`).join(",")}}`;
  return JSON.stringify(value) ?? "undefined";
}
function digest(value: unknown): string { return createHash("sha256").update(stable(value)).digest("hex"); }
function collectDatasetVersions(value: unknown, versions: Set<string>) {
  if (Array.isArray(value)) return value.forEach((child) => collectDatasetVersions(child, versions));
  if (!value || typeof value !== "object") return;
  for (const [key, child] of Object.entries(value)) {
    if (key === "datasetVersion" && typeof child === "string" && child) versions.add(child);
    collectDatasetVersions(child, versions);
  }
}

function validateResult(testCase: ExpectedCase, result: AdapterResult, protocol: Protocol): string[] {
  const failures: string[] = [];
  if (result.outcome !== testCase.expect.outcome) failures.push(`expected ${testCase.expect.outcome}, received ${result.outcome}`);
  if (protocol === "rest" && testCase.expect.httpStatus && result.status !== testCase.expect.httpStatus) failures.push(`expected status ${testCase.expect.httpStatus}, received ${result.status}`);
  if (stable(result.quotaCost) !== stable(testCase.expect.quotaCost)) failures.push("quota cost differs");
  const subject = result.outcome === "error" ? result.error : result.value;
  for (const path of testCase.expect.shape.required) {
    const value = pathValue(subject, path);
    if (value === undefined) failures.push(`missing result field ${path}`);
    else if (typeOf(value) !== expectedFieldType(path)) failures.push(`${path} must be ${expectedFieldType(path)}, received ${typeOf(value)}`);
  }
  for (const [path, expectedType] of Object.entries(testCase.expect.shape.fieldTypes ?? {})) {
    const actualType = typeOf(pathValue(subject, path));
    if (actualType !== expectedType) failures.push(`${path} must be ${expectedType}, received ${actualType}`);
  }
  if (testCase.expect.error) {
    if (pathValue(result.error, "error.code") !== testCase.expect.error.code) failures.push(`expected error ${testCase.expect.error.code}`);
    for (const path of testCase.expect.error.requiredFields) {
      const value = pathValue(result.error, `error.${path}`);
      if (value === undefined) failures.push(`missing error field ${path}`);
      else if (typeOf(value) !== expectedFieldType(path)) failures.push(`error ${path} must be ${expectedFieldType(path)}, received ${typeOf(value)}`);
    }
  }
  return failures;
}

function semanticFailures(testCase: ExpectedCase, executions: Array<{ protocol: Protocol; result: AdapterResult }>): string[] {
  const failures: string[] = [];
  const semantics = testCase.expect.semantics ?? [];
  const values = executions.map(({ result }) => result.value);
  if (semantics.includes("protocolResultsEquivalent") && new Set(values.map(stable)).size !== 1) failures.push("protocol results are not equivalent");
  if (semantics.includes("datasetVersionPresent") && values.some((value) => typeof pathValue(value, "datasetVersion") !== "string")) failures.push("datasetVersion is absent");
  if (semantics.includes("stableCursorPagination")) {
    for (const { result } of executions) {
      const pagination = result.evidence.pagination;
      const duplicate = pagination && pagination.firstIds.some((id) => pagination.secondIds.includes(id));
      const opaque = pagination && pagination.firstCursor.length >= 8 && !/^\d+$/.test(pagination.firstCursor);
      if (!pagination || duplicate || !opaque || pagination.firstCursor !== pagination.replayCursor) failures.push("cursor pagination evidence is invalid");
    }
  }
  if (semantics.includes("preservesGhanaianOrthography") && values.some((value) => !stable(value).includes("Mampɔŋ"))) failures.push("Ghanaian orthography was not preserved");
  if (semantics.includes("anonymousByDefault") && executions.some(({ result }) => result.evidence.authorizationSent !== false)) failures.push("anonymous request sent authorization");
  if (semantics.includes("cancellationPropagates") && executions.some(({ result }) => result.evidence.cancelled !== true)) failures.push("cancellation did not propagate");
  if (semantics.includes("emptyOutsideGhana") && values.some((value) => pathValue(value, "region") !== undefined || pathValue(value, "district") !== undefined || !Array.isArray(pathValue(value, "nearby")) || (pathValue(value, "nearby") as unknown[]).length !== 0)) failures.push("outside-Ghana result was not empty");
  return failures;
}

export async function runConformance(adapter: ConformanceAdapter, sdk: { language: string; name: string; version: string }) {
  const contract = exportedContract();
  const results: Array<Record<string, unknown>> = [];
  const evidence: Array<Record<string, unknown>> = [];
  const datasetVersions = new Set<string>();
  const apiVersions = new Set<string>();
  for (const expected of contract.cases) {
    const protocols = expected.protocols.filter((protocol) => adapter.supportedProtocols.includes(protocol));
    if (!protocols.length) continue;
    const runnerCase: RunnerCase = { id: expected.id, operation: expected.operation, input: expected.input };
    const executions: Array<{ protocol: Protocol; result: AdapterResult }> = [];
    const failures: string[] = [];
    for (const protocol of protocols) {
      try {
        const result = await adapter.execute(runnerCase, protocol);
        executions.push({ protocol, result });
        const observed = new Set<string>();
        collectDatasetVersions(result.value, observed);
        observed.forEach((version) => datasetVersions.add(version));
        if ([...observed].some((version) => version !== CANONICAL_DATASET_VERSION)) failures.push(`${protocol}: dataset version differs from ${CANONICAL_DATASET_VERSION}`);
        const requestText = stable(result.evidence.request);
        for (const match of requestText.matchAll(/\/v(\d+)(?:\/|\\|\?)/g)) apiVersions.add(`v${match[1]}`);
        failures.push(...validateResult(expected, result, protocol).map((failure) => `${protocol}: ${failure}`));
        evidence.push({ caseId: expected.id, protocol, requestDigest: digest(result.evidence.request), responseDigest: digest(result.evidence.response) });
      } catch (error) {
        failures.push(`${protocol}: ${error instanceof Error ? error.message : "adapter failed"}`);
      }
    }
    failures.push(...semanticFailures(expected, executions));
    results.push({ caseId: expected.id, status: failures.length ? "failed" : "passed", protocols, ...(failures.length ? { message: failures.join("; ") } : {}) });
  }
  const count = (status: string) => results.filter((result) => result.status === status).length;
  if (!datasetVersions.has(CANONICAL_DATASET_VERSION)) throw new Error(`runner did not observe canonical dataset ${CANONICAL_DATASET_VERSION}`);
  if (apiVersions.size !== 1) throw new Error(`runner observed ${apiVersions.size} API versions; expected exactly one`);
  return {
    schemaVersion: 2,
    contract: { digest: contract.contractDigest, runnerVersion: RUNNER_VERSION },
    sdk: { ...sdk, supportedProtocols: adapter.supportedProtocols },
    apiVersion: [...apiVersions][0]!, datasetVersion: CANONICAL_DATASET_VERSION,
    summary: { passed: count("passed"), failed: count("failed"), skipped: count("skipped") },
    evidenceDigest: digest(evidence),
    evidence,
    results,
  };
}
