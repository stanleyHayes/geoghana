import { createServer, type IncomingMessage, type ServerResponse } from "node:http";
import { execFileSync } from "node:child_process";
import { fileURLToPath } from "node:url";

type Protocol = "rest" | "grpc" | "graphql";
type RequestShape = { id: string; operation: string; input: Record<string, unknown>; protocol: Protocol };
type WireInput = { path: Record<string, string>; query: Record<string, string> };
const datasetVersion = "2026.08.3-ulid";
const root = fileURLToPath(new URL("../..", import.meta.url));
const exported = JSON.parse(execFileSync("ruby", ["tools/conformance/export_cases.rb"], { cwd: root, encoding: "utf8" })) as { cases: Array<RequestShape & { protocols: Protocol[] }> };
const cases = new Map(exported.cases.map((testCase) => [testCase.id, testCase]));
const errorCodes: Record<string, { code: string; status: number; grpc: string; details?: Record<string, unknown> }> = {
  "error.invalid-argument": { code: "INVALID_ARGUMENT", status: 400, grpc: "INVALID_ARGUMENT" },
  "error.invalid-coordinates": { code: "INVALID_COORDINATES", status: 400, grpc: "INVALID_ARGUMENT", details: { latitude: 91, longitude: 0 } },
  "error.radius-out-of-range": { code: "RADIUS_OUT_OF_RANGE", status: 400, grpc: "INVALID_ARGUMENT", details: { maxRadiusMeters: 50000 } },
  "error.query-too-short": { code: "QUERY_TOO_SHORT", status: 400, grpc: "INVALID_ARGUMENT", details: { minLength: 2 } },
  "error.payload-too-large": { code: "PAYLOAD_TOO_LARGE", status: 413, grpc: "RESOURCE_EXHAUSTED" },
  "error.unauthenticated": { code: "UNAUTHENTICATED", status: 401, grpc: "UNAUTHENTICATED" },
  "error.key-revoked": { code: "KEY_REVOKED", status: 401, grpc: "UNAUTHENTICATED" },
  "error.permission-denied": { code: "PERMISSION_DENIED", status: 403, grpc: "PERMISSION_DENIED", details: { requiredScope: "boundaries:read" } },
  "error.origin-not-allowed": { code: "ORIGIN_NOT_ALLOWED", status: 403, grpc: "PERMISSION_DENIED", details: { origin: "https://denied.invalid" } },
  "error.not-found": { code: "NOT_FOUND", status: 404, grpc: "NOT_FOUND", details: { id: "missing" } },
  "error.resource-gone": { code: "RESOURCE_GONE", status: 410, grpc: "NOT_FOUND", details: { mergedInto: "gh-place-kumasi", reason: "fixture merge" } },
  "error.rate-limit-exceeded": { code: "RATE_LIMIT_EXCEEDED", status: 429, grpc: "RESOURCE_EXHAUSTED", details: { retryAfterSeconds: 1, limit: 1 } },
  "error.quota-exceeded": { code: "QUOTA_EXCEEDED", status: 429, grpc: "RESOURCE_EXHAUSTED", details: { resetsAt: "2026-09-01T00:00:00Z" } },
  "error.query-too-complex": { code: "QUERY_TOO_COMPLEX", status: 400, grpc: "INVALID_ARGUMENT", details: { cost: 101, maxCost: 100, depth: 9, maxDepth: 8 } },
  "error.deadline-exceeded": { code: "DEADLINE_EXCEEDED", status: 504, grpc: "DEADLINE_EXCEEDED" },
  "error.internal": { code: "INTERNAL", status: 500, grpc: "INTERNAL" },
};

function quota(request: RequestShape): Record<string, unknown> {
  if (request.id.startsWith("error.")) {
    if (["error.not-found", "error.resource-gone"].includes(request.id)) return { class: "cheap", units: 1 };
    if (request.id === "error.deadline-exceeded") return { class: "spatial", units: 4 };
    return { class: "none", units: 0 };
  }
  if (["search", "autocomplete", "geocode"].includes(request.operation)) return { class: "normal", units: 2 };
  if (["reverseGeocode", "nearby"].includes(request.operation)) return { class: "spatial", units: 4 };
  if (request.operation === "getBoundary") return { class: "geometry", minimumUnits: 5, maximumUnits: 10 };
  if (["downloadDatasetArtifact", "streamDatasetChanges"].includes(request.operation)) return { class: "dynamic" };
  return { class: "cheap", units: 1 };
}

function entity(operation: string): Record<string, unknown> {
  if (["getRegion", "listRegions"].includes(operation)) return { id: "gh-region-ashanti", countryCode: "GH", name: "Ashanti", status: "ACTIVE", verificationStatus: "REFERENCE", provenance: { sourceId: "fixture" }, datasetVersion };
  if (operation === "listDistrictPlaces") return { id: "gh-place-kumasi", name: "Kumasi", normalizedName: "kumasi", type: "CITY", aliases: [], status: "ACTIVE", verificationStatus: "REFERENCE", provenance: { sourceId: "fixture" }, datasetVersion };
  if (operation.includes("District")) return { id: "gh-district-ahafo-asunafo-north", name: "Asunafo North", region: { id: "gh-region-ahafo", name: "Ahafo" }, status: "ACTIVE", verificationStatus: "REFERENCE", provenance: { sourceId: "fixture" }, datasetVersion };
  return { id: "gh-place-kumasi", name: "Kumasi", normalizedName: "kumasi", type: "CITY", aliases: [], status: "ACTIVE", verificationStatus: "REFERENCE", provenance: { sourceId: "fixture" }, datasetVersion };
}

function successValue(request: RequestShape): unknown {
  const orthography = request.input.fixture === "ghanaian-orthography";
  const cursor = request.input.cursor;
  const cursorData = cursor ? [{ ...entity("getPlace"), id: "gh-place-page-2" }] : [{ ...entity(request.operation), id: "gh-place-page-1" }];
  const page = { data: orthography ? [{ ...entity("getPlace"), name: "Mampɔŋ" }] : request.input.fixture === "two-page-place-sequence" ? cursorData : [entity(request.operation)], datasetVersion, ...(!cursor && request.input.fixture === "two-page-place-sequence" ? { nextCursor: "eyJvZmZzZXQiOjJ9" } : {}) };
  const query = request.input.query as Record<string, unknown> | undefined;
  if (request.operation === "reverseGeocode" && query?.lat === 51.5072) return { nearby: [], datasetVersion };
  if (["listRegions", "listRegionDistricts", "listDistricts", "listDistrictPlaces", "listPlaces", "search", "autocomplete", "geocode", "nearby"].includes(request.operation)) return page;
  if (["getRegion", "getDistrict", "getPlace"].includes(request.operation)) return entity(request.operation);
  if (request.operation === "reverseGeocode") return { region: { id: "gh-region-ashanti", name: "Ashanti" }, district: null, nearby: [], datasetVersion };
  if (request.operation === "getBoundary") return { type: "Feature", geometry: { type: "Polygon", coordinates: [] }, properties: { id: "gh-region-ashanti", datasetVersion } };
  if (request.operation === "listDatasets") return { data: [{ version: datasetVersion, status: "published" }] };
  if (request.operation === "listDatasetDownloads") return { version: datasetVersion, downloads: [] };
  if (request.operation === "downloadDatasetArtifact") return "fixture-bytes";
  if (request.operation === "streamDatasetChanges") return { change: { cursor: "fixture-cursor", datasetVersion } };
  return {};
}

function execute(request: RequestShape, authorizationSent = false) {
  const failure = errorCodes[request.id];
  if (failure) {
    const error = { error: { code: failure.code, message: `Fixture ${failure.code}`, requestId: `req-${request.id}`, docs: `/docs/errors/${failure.code}`, ...(failure.details ? { details: failure.details } : {}) } };
    return { outcome: "error", status: failure.status, error, quotaCost: quota(request), evidence: { request, response: error, authorizationSent } };
  }
  const value = successValue(request);
  const cancelled = request.input.fixture === "delayed-response";
  return { outcome: "success", status: request.operation === "streamDatasetChanges" || cancelled ? undefined : 200, value, quotaCost: quota(request), evidence: { request, response: value, authorizationSent, cancelled } };
}

function send(response: ServerResponse, body: unknown) { response.writeHead(200, { "content-type": "application/json", "x-conformance-fixture": "smoke-only" }).end(JSON.stringify(body)); }
function contractResponse(response: ServerResponse, result: ReturnType<typeof execute>, graphql = false) {
  const headers = { "content-type": "application/json", "x-conformance-fixture": "smoke-only", "x-conformance-quota": JSON.stringify(result.quotaCost) };
  if (graphql) {
    if (result.outcome === "error") {
      const failure = (result.error as { error: Record<string, unknown> }).error;
      response.writeHead(200, headers).end(JSON.stringify({ errors: [{ message: failure.message, extensions: { ...failure, ...(failure.details as object | undefined) } }] }));
      return;
    }
    response.writeHead(200, headers).end(JSON.stringify({ data: graphqlData((result.evidence.request as RequestShape).operation, result.value) }));
    return;
  }
  if ((result.evidence.request as RequestShape).operation === "downloadDatasetArtifact" && result.outcome === "success") {
    response.writeHead(result.status ?? 200, { ...headers, "content-type": "application/octet-stream" }).end(String(result.value));
    return;
  }
  response.writeHead(result.status ?? 200, headers).end(JSON.stringify(result.outcome === "error" ? result.error : result.value));
}
function operationFromPath(path: string): string | undefined {
  if (path === "/v1/regions") return "listRegions";
  if (/^\/v1\/regions\/[^/]+\/districts$/.test(path)) return "listRegionDistricts";
  if (/^\/v1\/regions\/[^/]+$/.test(path)) return "getRegion";
  if (path === "/v1/districts") return "listDistricts";
  if (/^\/v1\/districts\/[^/]+\/places$/.test(path)) return "listDistrictPlaces";
  if (/^\/v1\/districts\/[^/]+$/.test(path)) return "getDistrict";
  if (path === "/v1/places") return "listPlaces";
  if (/^\/v1\/places\/[^/]+$/.test(path)) return "getPlace";
  if (path === "/v1/search") return "search";
  if (path === "/v1/autocomplete") return "autocomplete";
  if (path === "/v1/geocode") return "geocode";
  if (path === "/v1/reverse") return "reverseGeocode";
  if (path === "/v1/nearby") return "nearby";
  if (/^\/v1\/boundaries\/[^/]+$/.test(path)) return "getBoundary";
  if (path === "/v1/datasets") return "listDatasets";
  if (/^\/v1\/datasets\/[^/]+\/downloads$/.test(path)) return "listDatasetDownloads";
  if (/^\/v1\/datasets\/[^/]+\/downloads\/(regions|districts|places)\.(json|csv|geojson)$/.test(path)) return "downloadDatasetArtifact";
  if (path === "/v1/roads") return "listRoads";
  if (path === "/v1/pois") return "listPointsOfInterest";
  return undefined;
}
function operationFromQuery(query: string, caseId: string): string | undefined {
  if (caseId === "operation.list-region-districts") return "listRegionDistricts";
  if (caseId === "operation.list-district-places") return "listDistrictPlaces";
  if (/\bregion\s*\([^)]*\)\s*\{\s*boundary\b/.test(query)) return "getBoundary";
  const mappings: Array<[RegExp, string]> = [
    [/\bdatasetVersions\s*\(/, "listDatasets"], [/\bregions\s*\(/, "listRegions"], [/\bregion\s*\(/, "getRegion"],
    [/\bdistricts\s*\(/, "listDistricts"], [/\bdistrict\s*\(/, "getDistrict"], [/\bplaces\s*\(/, "listPlaces"], [/\bplace\s*\(/, "getPlace"],
    [/\bsearch\s*\(/, "search"], [/\bautocomplete\s*\(/, "autocomplete"], [/\bgeocode\s*\(/, "geocode"], [/\breverse\s*\(/, "reverseGeocode"],
    [/\bnearby\s*\(/, "nearby"],
  ];
  const matches = mappings.filter(([pattern]) => pattern.test(query));
  if (matches.length !== 1) return undefined;
  return matches[0]![1];
}
function graphqlData(operation: string, value: any): Record<string, unknown> {
  if (operation === "listRegions") return { regions: { nodes: value.data, datasetVersion: value.datasetVersion, pageInfo: { endCursor: value.nextCursor ?? null } } };
  if (["listRegionDistricts", "listDistricts"].includes(operation)) return { districts: { nodes: value.data, datasetVersion: value.datasetVersion, pageInfo: { endCursor: value.nextCursor ?? null } } };
  if (["listDistrictPlaces", "listPlaces"].includes(operation)) return { places: { nodes: value.data, datasetVersion: value.datasetVersion, pageInfo: { endCursor: value.nextCursor ?? null } } };
  if (operation === "search") return { search: { nodes: value.data, datasetVersion: value.datasetVersion, pageInfo: { endCursor: null } } };
  if (["autocomplete", "geocode"].includes(operation)) return { [operation]: value.data.map((place: unknown) => ({ place, score: 1 })) };
  if (operation === "nearby") return { nearby: value.data.map((place: unknown) => ({ place, distanceMeters: 1 })) };
  if (operation === "reverseGeocode") return { reverse: value };
  if (operation === "getBoundary") return { region: { boundary: value } };
  if (operation === "listDatasets") return { datasetVersions: value.data };
  const field = operation === "getRegion" ? "region" : operation === "getDistrict" ? "district" : "place";
  return { [field]: value };
}
function fixtureInput(id: string, input: Record<string, unknown>): Record<string, unknown> {
  if (id === "semantic.cursor-pagination") return { ...input, fixture: "two-page-place-sequence" };
  if (id === "semantic.autocomplete-orthography") return { ...input, fixture: "ghanaian-orthography" };
  if (id === "semantic.cancellation-propagates") return { ...input, fixture: "delayed-response" };
  if (id === "semantic.reverse-outside-ghana") return { ...input, query: { lat: 51.5072, lng: -0.1276 } };
  return input;
}
function pathInput(path: string, operation: string): Record<string, string> {
  const parts = path.split("/").filter(Boolean).map(decodeURIComponent);
  if (["getRegion", "getDistrict", "getPlace", "getBoundary"].includes(operation)) return { id: parts.at(-1)! };
  if (operation === "listRegionDistricts" || operation === "listDistrictPlaces") return { id: parts.at(-2)! };
  if (operation === "listDatasetDownloads") return { version: parts.at(-2)! };
  if (operation === "downloadDatasetArtifact") {
    const [entity, format] = parts.at(-1)!.split(".");
    return { version: parts.at(-3)!, entity: entity!, format: format! };
  }
  return {};
}
function graphqlInput(query: string, operation: string): WireInput {
  const argument = (name: string) => query.match(new RegExp(`\\b${name}:\\s*(?:"([^"]*)"|(-?[0-9]+(?:\\.[0-9]+)?))`))?.slice(1).find(Boolean);
  const path: Record<string, string> = {};
  const wireQuery: Record<string, string> = {};
  const pathArgument = operation === "listRegionDistricts" ? "regionId" : operation === "listDistrictPlaces" ? "districtId" : ["getRegion", "getDistrict", "getPlace", "getBoundary"].includes(operation) ? "id" : undefined;
  if (pathArgument) path.id = argument(pathArgument) ?? "";
  const aliases: Record<string, string> = { q: "query", limit: "first", lat: "latitude", lng: "longitude", radius: "radius", regionId: "regionId", districtId: "districtId", cursor: "after" };
  for (const [name, wireName] of Object.entries(aliases)) {
    const value = argument(wireName);
    if (value !== undefined) wireQuery[name] = value;
  }
  return { path, query: wireQuery };
}
function wireMismatch(id: string, operation: string, protocol: Protocol, actual: WireInput, authorization?: string): string | undefined {
  const expected = cases.get(id);
  if (!expected) return `unknown case ${id}`;
  if (!expected.protocols.includes(protocol)) return `${id} does not support ${protocol}`;
  if (expected.operation !== operation) return `operation ${operation} does not match ${expected.operation}`;
  const expectedPath = (expected.input.path ?? {}) as Record<string, unknown>;
  const expectedQuery = (expected.input.query ?? {}) as Record<string, unknown>;
  const same = (actualValue: string | undefined, expectedValue: unknown) => typeof expectedValue === "number" ? actualValue !== undefined && Number(actualValue) === expectedValue : actualValue === String(expectedValue);
  for (const [key, value] of Object.entries(expectedPath)) if (!same(actual.path[key], value)) return `path.${key} differs`;
  for (const [key, value] of Object.entries(expectedQuery)) if (!same(actual.query[key], value)) return `query.${key} differs`;
  const expectedAuth = expected.input.auth;
  if (expectedAuth === "omitted" && authorization) return "authorization must be omitted";
  if (typeof expectedAuth === "string" && expectedAuth !== "omitted" && authorization !== `Bearer ${expectedAuth}`) return "authorization fixture differs";
  if (expectedAuth === undefined && authorization) return "unexpected authorization";
  return undefined;
}
function mismatchResponse(response: ServerResponse, message: string) {
  response.writeHead(422, { "content-type": "application/json", "x-conformance-fixture": "smoke-only" }).end(JSON.stringify({ error: { code: "CONFORMANCE_FIXTURE_MISMATCH", message } }));
}
function fixtureControl(id: string): string | undefined {
  const control = cases.get(id)?.input.fixture;
  return typeof control === "string" ? control : undefined;
}
async function controlledFailure(response: ServerResponse, requestShape: RequestShape, graphql = false): Promise<boolean> {
  const control = fixtureControl(requestShape.id);
  if (control === "oversized-request") {
    const failure = execute(requestShape);
    const payload = JSON.stringify({ ...(failure as any).error, fixtureTransport: { observedBytes: 70_000, padding: "x".repeat(70_000) } });
    response.writeHead(413, { "content-type": "application/json", "x-conformance-fixture": "smoke-only", "x-conformance-quota": JSON.stringify(failure.quotaCost) });
    for (let offset = 0; offset < payload.length; offset += 4096) response.write(payload.slice(offset, offset + 4096));
    response.end();
    return true;
  }
  if (control === "forced-deadline") {
    await new Promise((resolve) => setTimeout(resolve, 150));
    contractResponse(response, execute(requestShape), graphql);
    return true;
  }
  if (control === "forced-internal-error") {
    try { throw new Error("deterministic fixture backend failure"); }
    catch { contractResponse(response, execute(requestShape), graphql); }
    return true;
  }
  return false;
}
async function body(request: IncomingMessage): Promise<Record<string, unknown>> {
  let content = "";
  for await (const chunk of request) content += chunk;
  return JSON.parse(content || "{}") as Record<string, unknown>;
}

const server = createServer(async (request, response) => {
  if (request.method === "GET" && request.url === "/health") return send(response, { ready: true, purpose: "conformance-smoke-only" });
  if (request.method === "GET" && request.url?.startsWith("/v1/__conformance/")) {
    const url = new URL(request.url, "http://fixture");
    const requestShape = JSON.parse(url.searchParams.get("request") ?? "{}") as RequestShape;
    return send(response, execute(requestShape, Boolean(request.headers.authorization)));
  }
  if (request.method === "GET" && request.url?.startsWith("/v1/")) {
    const url = new URL(request.url, "http://fixture");
    const operation = operationFromPath(url.pathname);
    const id = String(request.headers["x-conformance-case"] ?? "");
    if (!operation || !id) return response.writeHead(400).end();
    const input = { path: pathInput(url.pathname, operation), query: Object.fromEntries(url.searchParams) };
    const mismatch = wireMismatch(id, operation, "rest", input, request.headers.authorization);
    if (mismatch) return mismatchResponse(response, mismatch);
    const normalizedInput = { ...input.query, ...(Object.keys(input.path).length ? { path: input.path } : {}), ...(Object.keys(input.query).length ? { query: input.query } : {}) };
    const requestShape: RequestShape = { id, operation, input: fixtureInput(id, normalizedInput), protocol: "rest" };
    if (await controlledFailure(response, requestShape)) return;
    if (id === "semantic.cancellation-propagates") await new Promise((resolve) => setTimeout(resolve, 100));
    return contractResponse(response, execute(requestShape, Boolean(request.headers.authorization)));
  }
  if (request.method === "POST" && request.url === "/graphql") {
    const payload = await body(request);
    if ((payload.variables as { request?: RequestShape } | undefined)?.request) {
      const requestShape = (payload.variables as { request: RequestShape }).request;
      return send(response, { data: { conformance: execute(requestShape, Boolean(request.headers.authorization)) } });
    }
    const id = String(request.headers["x-conformance-case"] ?? "");
    const query = String(payload.query ?? "");
    const operation = operationFromQuery(query, id);
    if (!operation || !id) return response.writeHead(400).end();
    const wireInput = graphqlInput(query, operation);
    const mismatch = wireMismatch(id, operation, "graphql", wireInput, request.headers.authorization);
    if (mismatch) return mismatchResponse(response, mismatch);
    const cursor = query.match(/\bafter:\s*"([^"]+)"/)?.[1];
    const requestShape: RequestShape = { id, operation, input: fixtureInput(id, cursor ? { cursor } : {}), protocol: "graphql" };
    if (await controlledFailure(response, requestShape, true)) return;
    if (id === "semantic.cancellation-propagates") await new Promise((resolve) => setTimeout(resolve, 100));
    return contractResponse(response, execute(requestShape, Boolean(request.headers.authorization)), true);
  }
  response.writeHead(404).end();
});

server.listen(0, "127.0.0.1", () => {
  const address = server.address();
  if (!address || typeof address === "string") throw new Error("fixture server did not bind TCP");
  process.stdout.write(`${address.port}\n`);
});
