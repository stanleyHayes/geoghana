import { DEFAULT_API_URL } from "@ghanageo/core";
import type { DistrictPage, DownloadList, Place, PlacePage, Region, RegionPage, ReverseResult, SearchPage } from "@ghanageo/core";

export * from "@ghanageo/core";

export type GhanaGeoClientOptions = {
  baseUrl?: string;
  apiKey?: string;
  fetcher?: typeof fetch;
};

export class GhanaGeoError extends Error {
  constructor(
    message: string,
    public readonly status: number,
    public readonly code?: string,
    public readonly requestId?: string,
    public readonly details?: Record<string, unknown>,
  ) {
    super(message);
    this.name = "GhanaGeoError";
  }
}

export class GhanaGeoClient {
  readonly baseUrl: string;
  private readonly apiKey: string | undefined;
  private readonly fetcher: typeof fetch;

  constructor(options: GhanaGeoClientOptions = {}) {
    this.baseUrl = (options.baseUrl ?? DEFAULT_API_URL).replace(/\/$/, "");
    this.apiKey = options.apiKey;
    this.fetcher = options.fetcher ?? globalThis.fetch;
    if (!this.fetcher) throw new Error("GhanaGeoClient needs a fetch implementation in this runtime.");
  }

  async requestRaw<T>(path: string, params?: Record<string, string | number | boolean | undefined | null>, signal?: AbortSignal): Promise<{ response: Response; body: T }> {
    const url = new URL(`${this.baseUrl}${path}`);
    for (const [key, value] of Object.entries(params ?? {})) {
      if (value !== undefined && value !== null && value !== "") url.searchParams.set(key, String(value));
    }
    const headers: Record<string, string> = { Accept: "application/json" };
    if (this.apiKey) headers.Authorization = `Bearer ${this.apiKey}`;
    const response = await this.fetcher(url, { headers, ...(signal ? { signal } : {}) });
    const body = await response.json().catch(() => ({})) as Record<string, any>;
    if (!response.ok) {
      const failure = body.error ?? {};
      throw new GhanaGeoError(failure.message ?? `GhanaGeo request failed (${response.status})`, response.status, failure.code, failure.requestId, failure.details);
    }
    return { response, body: body as T };
  }

  async request<T>(path: string, params?: Record<string, string | number | boolean | undefined | null>, signal?: AbortSignal): Promise<T> {
    return (await this.requestRaw<T>(path, params, signal)).body;
  }

  get<T>(path: string, params?: Record<string, string | number | boolean | undefined | null>, signal?: AbortSignal) {
    return this.request<T>(path, params, signal);
  }

  regions(params: { cursor?: string; limit?: number } = {}, signal?: AbortSignal) { return this.get<RegionPage>("/regions", params, signal); }
  region(id: string, signal?: AbortSignal) { return this.get<{ data: Region; datasetVersion: string }>(`/regions/${encodeURIComponent(id)}`, undefined, signal); }
  districts(params: { regionId?: string; q?: string; cursor?: string; limit?: number } = {}, signal?: AbortSignal) { return this.get<DistrictPage>("/districts", params, signal); }
  places(params: { regionId?: string; districtId?: string; type?: string; q?: string; cursor?: string; limit?: number } = {}, signal?: AbortSignal) { return this.get<PlacePage>("/places", params, signal); }
  place(id: string, signal?: AbortSignal) { return this.get<{ data: Place; datasetVersion: string }>(`/places/${encodeURIComponent(id)}`, undefined, signal); }
  search(q: string, params: { regionId?: string; districtId?: string; type?: string; limit?: number } = {}, signal?: AbortSignal) { return this.get<SearchPage>("/search", { q, ...params }, signal); }
  autocomplete(q: string, limit = 10, signal?: AbortSignal) { return this.get<SearchPage>("/autocomplete", { q, limit }, signal); }
  geocode(q: string, limit = 10, signal?: AbortSignal) { return this.get<SearchPage>("/geocode", { q, limit }, signal); }
  reverse(latitude: number, longitude: number, signal?: AbortSignal) { return this.get<ReverseResult>("/reverse", { lat: latitude, lng: longitude }, signal); }
  nearby(latitude: number, longitude: number, radius = 5000, limit = 20, signal?: AbortSignal) { return this.get<PlacePage>("/nearby", { lat: latitude, lng: longitude, radius, limit }, signal); }
  datasetDownloads(version: string, signal?: AbortSignal) { return this.get<DownloadList>(`/datasets/${encodeURIComponent(version)}/downloads`, undefined, signal); }

  async graphql<TData, TVariables extends Record<string, unknown> = Record<string, unknown>>(query: string, variables?: TVariables, signal?: AbortSignal): Promise<TData> {
    const endpoint = this.baseUrl.replace(/\/v1$/, "") + "/graphql";
    const headers: Record<string, string> = { "Content-Type": "application/json", Accept: "application/json" };
    if (this.apiKey) headers.Authorization = `Bearer ${this.apiKey}`;
    const response = await this.fetcher(endpoint, { method: "POST", headers, body: JSON.stringify({ query, variables }), ...(signal ? { signal } : {}) });
    const body = await response.json() as Record<string, any>;
    if (!response.ok || body.errors?.length) {
      const failure = body.errors?.[0];
      throw new GhanaGeoError(failure?.message ?? `GraphQL request failed (${response.status})`, response.status, failure?.extensions?.code, failure?.extensions?.requestId, failure?.extensions);
    }
    return body.data as TData;
  }
}
