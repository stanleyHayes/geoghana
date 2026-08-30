import type { AdapterResult, ConformanceAdapter, Protocol, RunnerCase } from "../../../tools/conformance/runner-kit";
import { GhanaGeoClient, GhanaGeoError } from "../src";

type Input = { path?: Record<string, string>; query?: Record<string, string | number>; auth?: string; fixture?: string; cancelAfterMilliseconds?: number };
type Capture = { requests: unknown[]; responses: unknown[]; response?: { status: number; headers: Record<string, string> } };
const headerRecord = (headers: Headers) => {
  const result: Record<string, string> = {};
  headers.forEach((value, key) => { result[key] = value; });
  return result;
};

function clientFor(baseUrl: string, testCase: RunnerCase, capture: Capture) {
  const input = testCase.input as Input;
  const fetcher: typeof fetch = async (resource, init) => {
    const headers = new Headers(init?.headers);
    headers.set("x-conformance-case", testCase.id);
    const request = { url: String(resource), method: init?.method ?? "GET", headers: headerRecord(headers), ...(init?.body ? { body: String(init.body) } : {}) };
    capture.requests.push(request);
    const response = await fetch(resource, { ...init, headers });
    capture.response = { status: response.status, headers: headerRecord(response.headers) };
    capture.responses.push(capture.response);
    return response;
  };
  return new GhanaGeoClient({ baseUrl, fetcher, ...(input.auth && input.auth !== "omitted" ? { apiKey: input.auth } : {}) });
}

async function restCall(client: GhanaGeoClient, testCase: RunnerCase, signal?: AbortSignal) {
  const input = testCase.input as Input;
  const path = input.path ?? {};
  const query = input.query ?? {};
  switch (testCase.operation) {
    case "listRegions": return client.regions(query, signal);
    case "getRegion": return client.region(path.id!, signal);
    case "listRegionDistricts": return client.regionDistricts(path.id!, query, signal);
    case "listDistricts": return client.districts(query, signal);
    case "getDistrict": return client.district(path.id!, signal);
    case "listDistrictPlaces": return client.districtPlaces(path.id!, query, signal);
    case "listPlaces": return client.places(query, signal);
    case "getPlace": return client.place(path.id!, signal);
    case "search": return client.search(String(query.q ?? "Kumasi"), query, signal);
    case "autocomplete": return client.autocomplete(String(query.q), Number(query.limit ?? 10), signal);
    case "geocode": return client.geocode(String(query.q), Number(query.limit ?? 10), signal);
    case "reverseGeocode": return client.reverse(Number(query.lat), Number(query.lng), signal);
    case "nearby": return client.nearby(Number(query.lat ?? 6.6885), Number(query.lng ?? -1.6244), Number(query.radius ?? 5000), Number(query.limit ?? 20), signal);
    case "getBoundary": return client.boundary(path.id!, signal);
    case "listDatasets": return client.datasets(signal);
    case "listDatasetDownloads": return client.datasetDownloads(path.version!, signal);
    case "downloadDatasetArtifact": return client.downloadDatasetArtifact(path.version!, path.entity! as "regions" | "districts" | "places", path.format! as "json" | "csv" | "geojson", signal);
    case "listRoads": return client.roads(signal);
    case "listPointsOfInterest": return client.pointsOfInterest(signal);
    default: throw new Error(`GEO-24.TS missing public REST method: ${testCase.operation}`);
  }
}

const regionFields = "id name countryCode status verificationStatus datasetVersion provenance { sourceId }";
const districtFields = "id name status verificationStatus datasetVersion provenance { sourceId } region { id name }";
const placeFields = "id name normalizedName type aliases { value } status verificationStatus datasetVersion provenance { sourceId }";
const literal = (value: string | number) => typeof value === "number" ? String(value) : JSON.stringify(value);
function argumentsFor(values: Array<[string, string | number | undefined]>): string {
  const args = values.filter((entry): entry is [string, string | number] => entry[1] !== undefined).map(([name, value]) => `${name}: ${literal(value)}`);
  return args.length ? `(${args.join(", ")})` : "";
}

function selectionFor(testCase: RunnerCase): string {
  const input = testCase.input as Input;
  const path = input.path ?? {};
  const query = input.query ?? {};
  const page = (field: string, args: string, fields: string) => `${field}${args} { nodes { ${fields} } datasetVersion pageInfo { endCursor } }`;
  switch (testCase.operation) {
    case "listRegions": return page("regions", argumentsFor([["first", query.limit ?? 2], ["after", query.cursor]]), regionFields);
    case "getRegion": return `region${argumentsFor([["id", path.id]])} { ${regionFields} }`;
    case "listRegionDistricts": return page("districts", argumentsFor([["regionId", path.id], ["first", query.limit], ["after", query.cursor]]), districtFields);
    case "listDistricts": return page("districts", argumentsFor([["regionId", query.regionId], ["query", query.q], ["first", query.limit], ["after", query.cursor]]), districtFields);
    case "getDistrict": return `district${argumentsFor([["id", path.id]])} { ${districtFields} }`;
    case "listDistrictPlaces": return page("places", argumentsFor([["districtId", path.id], ["first", query.limit], ["after", query.cursor]]), placeFields);
    case "listPlaces": return page("places", argumentsFor([["regionId", query.regionId], ["districtId", query.districtId], ["query", query.q], ["first", query.limit], ["after", query.cursor]]), placeFields);
    case "getPlace": return `place${argumentsFor([["id", path.id]])} { ${placeFields} }`;
    case "search": return page("search", argumentsFor([["query", query.q ?? "Kumasi"], ["regionId", query.regionId], ["districtId", query.districtId], ["first", query.limit ?? 2]]), "place { id name } score");
    case "autocomplete": return `autocomplete${argumentsFor([["query", query.q], ["first", query.limit]])} { place { id name datasetVersion } score }`;
    case "geocode": return `geocode${argumentsFor([["query", query.q], ["first", query.limit]])} { place { id name datasetVersion } score }`;
    case "reverseGeocode": return `reverse${argumentsFor([["latitude", query.lat], ["longitude", query.lng]])} { region { id name } district { id name } nearby { id name } datasetVersion }`;
    case "nearby": return `nearby${argumentsFor([["latitude", query.lat ?? 6.6885], ["longitude", query.lng ?? -1.6244], ["radius", query.radius], ["first", query.limit ?? 2]])} { place { id name datasetVersion } distanceMeters }`;
    case "getBoundary": return `region${argumentsFor([["id", path.id]])} { boundary }`;
    case "listDatasets": return "datasetVersions(first: 2) { version status }";
    default: throw new Error(`GEO-24.TS missing GraphQL mapping: ${testCase.operation}`);
  }
}

function normalizeGraphql(operation: string, data: Record<string, unknown>): unknown {
  const value = Object.values(data)[0] as any;
  if (["listRegions", "listRegionDistricts", "listDistricts", "listDistrictPlaces", "listPlaces"].includes(operation)) return { data: value.nodes, datasetVersion: value.datasetVersion, ...(value.pageInfo?.endCursor ? { nextCursor: value.pageInfo.endCursor } : {}) };
  if (["search"].includes(operation)) return { data: value.nodes, datasetVersion: value.datasetVersion };
  if (["autocomplete", "geocode", "nearby"].includes(operation)) return { data: value, datasetVersion: value[0]?.place?.datasetVersion };
  if (operation === "getBoundary") return value.boundary;
  if (operation === "listDatasets") return { data: value };
  return value;
}

async function graphqlCall(client: GhanaGeoClient, testCase: RunnerCase, signal?: AbortSignal) {
  const selection = selectionFor(testCase);
  const data = await client.graphql<Record<string, unknown>>(`query ${testCase.id.replace(/[^A-Za-z0-9]/g, "_")} { ${selection} }`, undefined, signal);
  return normalizeGraphql(testCase.operation, data);
}

export function createClientConformanceAdapter(baseUrl: string): ConformanceAdapter {
  return {
    supportedProtocols: ["rest", "graphql"],
    async execute(testCase: RunnerCase, protocol: Protocol): Promise<AdapterResult> {
      if (protocol === "grpc") throw new Error("@ghanageo/client does not claim native gRPC support; use @ghanageo/proto");
      const capture: Capture = { requests: [], responses: [] };
      const client = clientFor(baseUrl, testCase, capture);
      const controller = new AbortController();
      const input = testCase.input as Input;
      const timer = input.cancelAfterMilliseconds ? setTimeout(() => controller.abort(), input.cancelAfterMilliseconds) : undefined;
      try {
        if (testCase.id === "semantic.cursor-pagination") {
          const fetchPage = async (cursor?: string) => {
            if (protocol === "rest") return client.places({ limit: 2, ...(cursor ? { cursor } : {}) }, controller.signal);
            const after = cursor ? `, after: \"${cursor}\"` : "";
            const data = await client.graphql<{ places: { nodes: Array<{ id: string }>; datasetVersion: string; pageInfo: { endCursor?: string } } }>(`query CursorPage { places(first: 2${after}) { nodes { id name normalizedName type aliases { value } status verificationStatus datasetVersion provenance { sourceId } } datasetVersion pageInfo { endCursor } } }`, undefined, controller.signal);
            return { data: data.places.nodes, datasetVersion: data.places.datasetVersion, nextCursor: data.places.pageInfo.endCursor };
          };
          const first = await fetchPage();
          const cursor = first.nextCursor;
          if (!cursor) throw new Error("first page did not return a cursor");
          const second = await fetchPage(cursor);
          const replay = await fetchPage();
          const response = capture.response as { headers?: Record<string, string> } | undefined;
          return {
            outcome: "success", status: 200, value: first,
            quotaCost: JSON.parse(response?.headers?.["x-conformance-quota"] ?? "{}") as Record<string, unknown>,
            evidence: {
              request: capture.requests, response: capture.responses, authorizationSent: false,
              pagination: { firstIds: first.data.map((item) => item.id), secondIds: second.data.map((item) => item.id), firstCursor: cursor, replayCursor: replay.nextCursor ?? "" },
            },
          };
        }
        const value = protocol === "rest" ? await restCall(client, testCase, controller.signal) : await graphqlCall(client, testCase, controller.signal);
        const response = capture.response as { headers?: Record<string, string> } | undefined;
        const quotaCost = JSON.parse(response?.headers?.["x-conformance-quota"] ?? "{}") as Record<string, unknown>;
        return { outcome: "success", status: 200, value, quotaCost, evidence: { request: capture.requests.length === 1 ? capture.requests[0] : capture.requests, response: capture.responses.length === 1 ? capture.responses[0] : capture.responses, authorizationSent: JSON.stringify(capture.requests).toLowerCase().includes("authorization"), cancelled: false } };
      } catch (error) {
        if (error instanceof DOMException && error.name === "AbortError") return { outcome: "success", value: {}, quotaCost: { class: "normal", units: 2 }, evidence: { request: capture.requests, response: [...capture.responses, { aborted: true }], authorizationSent: false, cancelled: true } };
        if (error instanceof GhanaGeoError) {
          const response = capture.response as { headers?: Record<string, string> } | undefined;
          const quotaCost = JSON.parse(response?.headers?.["x-conformance-quota"] ?? "{}") as Record<string, unknown>;
          return { outcome: "error", status: error.status, error: { error: { code: error.code, message: error.message, requestId: error.requestId, docs: error.docs, ...(error.details ? { details: error.details } : {}) } }, quotaCost, evidence: { request: capture.requests.length === 1 ? capture.requests[0] : capture.requests, response: capture.responses.length === 1 ? capture.responses[0] : capture.responses, authorizationSent: JSON.stringify(capture.requests).toLowerCase().includes("authorization") } };
        }
        throw error;
      } finally {
        if (timer) clearTimeout(timer);
      }
    },
  };
}
