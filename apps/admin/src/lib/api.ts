/**
 * Typed access to the GhanaGeo REST API for admin screens.
 *
 * Admin reads the SAME public API every developer uses. There is no privileged
 * back channel: if a screen can show it, a developer can fetch it. That keeps
 * us honest about what the public API actually exposes — a gap shows up here
 * first, as a screen we cannot build.
 */

// Public reads use the same-origin Next proxy as protected admin calls. This
// avoids fragile cross-origin browser requests while preserving the exact API.
const BASE = "/api/v1";

export interface Provenance {
  sourceId: string;
  sourceUrl?: string;
  retrievedAt?: string;
}

export interface Region {
  id: string;
  countryCode: string;
  name: string;
  capital?: string;
  status: string;
  verificationStatus: string;
  centroid?: Coordinate;
  provenance?: Provenance;
  datasetVersion?: string;
}

export interface District {
  id: string;
  name: string;
  type: string;
  region?: { id: string; name: string };
  status: string;
  verificationStatus: string;
  centroid?: Coordinate;
  provenance?: Provenance;
}

export interface Place {
  id: string;
  name: string;
  type: string;
  region?: { id: string; name: string };
  district?: { id: string; name: string };
  aliases?: { value: string; type: string }[];
  centroid?: Coordinate;
  population?: number;
  status: string;
  verificationStatus: string;
  provenance?: Provenance;
}

export interface Coordinate {
  latitude: number;
  longitude: number;
}

export interface BoundaryFeature {
  type: "Feature";
  geometry: {
    type:
      | "Point"
      | "LineString"
      | "Polygon"
      | "MultiPoint"
      | "MultiLineString"
      | "MultiPolygon";
    coordinates: unknown[];
  };
  properties: {
    id?: string;
    name?: string;
    kind?: string;
    datasetVersion?: string;
    attribution?: string;
  };
}

export interface Road {
  id: string;
  name: string;
  ref?: string;
  class: string;
  district?: { id: string; name?: string };
  region?: { id: string; name?: string };
}

export interface PointOfInterest {
  id: string;
  name: string;
  class: string;
  category?: string;
  district?: { id: string; name?: string };
  region?: { id: string; name?: string };
  centroid?: { latitude: number; longitude: number };
}

export interface DatasetArtifact {
  entity: string;
  format: string;
  sizeBytes: number;
  sha256: string;
  recordCount: number;
  url: string;
}

export interface DatasetVersion {
  version: string;
  status: string;
  publishedAt?: string;
  changelog?: string;
  counts?: Record<string, number>;
  downloads: DatasetArtifact[];
  license: string;
  attribution: string;
}

export interface OSMPage<T> {
  data: T[];
  attribution: string;
  license: string;
}

export interface Page<T> {
  data: T[];
  nextCursor?: string;
  datasetVersion?: string;
}

export interface ListParams {
  limit?: number | undefined;
  cursor?: string | undefined;
  q?: string | undefined;
  /* An index signature so these can be passed straight to apiGet's query
     builder without a cast at every call site. */
  [key: string]: string | number | undefined;
}

export class ApiError extends Error {
  constructor(
    readonly code: string,
    message: string,
    readonly requestId?: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

/**
 * A failed admin request must say WHY. The API returns a machine `code` and a
 * `requestId`; surfacing both means a steward can quote the id in a bug report
 * instead of describing a spinner that never stopped.
 */
export async function apiGet<T>(
  path: string,
  params: Record<string, string | number | undefined> = {},
  signal?: AbortSignal,
): Promise<T> {
  const query = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined && v !== "") query.set(k, String(v));
  }
  const suffix = query.size ? `?${query.toString()}` : "";
  const res = await fetch(`${BASE}${path}${suffix}`, signal ? { signal } : {});
  const body = await res.json().catch(() => null);
  if (!res.ok) {
    const e = body?.error;
    throw new ApiError(
      e?.code ?? "INTERNAL",
      e?.message ?? `Request failed (${res.status})`,
      e?.requestId,
    );
  }
  return body as T;
}

/** `| undefined` is explicit throughout: exactOptionalPropertyTypes draws a
 *  distinction between "absent" and "present but undefined", and every one of
 *  these params is genuinely optional at the call site. */
/**
 * Follow `nextCursor` until the collection is exhausted.
 *
 * The API caps `limit` at 100 regardless of what is asked for, so a single
 * call for "all 261 districts" silently returns 100 and a cursor. An admin
 * screen that renders that as "100 districts" is worse than one that fails:
 * it reads as a complete count for a country that has 261.
 *
 * `maxPages` is a runaway guard, and reaching it is reported rather than
 * swallowed — see `complete` on the result.
 */
export async function fetchAll<T>(
  fetchPage: (
    cursor: string | undefined,
    signal?: AbortSignal,
  ) => Promise<Page<T>>,
  signal?: AbortSignal,
  maxPages = 25,
): Promise<{
  data: T[];
  datasetVersion?: string | undefined;
  complete: boolean;
}> {
  const data: T[] = [];
  let cursor: string | undefined;
  let datasetVersion: string | undefined;
  for (let i = 0; i < maxPages; i++) {
    const page = await fetchPage(cursor, signal);
    data.push(...page.data);
    datasetVersion ??= page.datasetVersion;
    if (!page.nextCursor) return { data, datasetVersion, complete: true };
    cursor = page.nextCursor;
  }
  return { data, datasetVersion, complete: false };
}

export const listRegions = (p: ListParams, s?: AbortSignal) =>
  apiGet<Page<Region>>("/regions", p, s);

export const listDistricts = (p: ListParams, s?: AbortSignal) =>
  apiGet<Page<District>>("/districts", p, s);

export const listRegionDistricts = (
  id: string,
  p: ListParams,
  s?: AbortSignal,
) =>
  apiGet<Page<District>>(`/regions/${encodeURIComponent(id)}/districts`, p, s);

export const listDistrictPlaces = (
  id: string,
  p: ListParams,
  s?: AbortSignal,
) => apiGet<Page<Place>>(`/districts/${encodeURIComponent(id)}/places`, p, s);

export const listPlaces = (p: ListParams, s?: AbortSignal) =>
  apiGet<Page<Place>>("/places", p, s);

export const getPlace = (id: string, s?: AbortSignal) =>
  apiGet<{ data: Place }>(`/places/${encodeURIComponent(id)}`, {}, s);

export const getBoundary = (id: string, s?: AbortSignal) =>
  apiGet<BoundaryFeature>(`/boundaries/${encodeURIComponent(id)}`, {}, s);

export const searchPlaces = (q: string, p: ListParams, s?: AbortSignal) =>
  apiGet<Page<Place>>("/search", { q, ...p }, s);

export const listRoads = (p: ListParams, s?: AbortSignal) =>
  apiGet<OSMPage<Road>>("/roads", p, s);

export const listPointsOfInterest = (p: ListParams, s?: AbortSignal) =>
  apiGet<OSMPage<PointOfInterest>>("/pois", p, s);

export const listDatasets = (s?: AbortSignal) =>
  apiGet<{ data: DatasetVersion[] }>("/datasets", {}, s);
