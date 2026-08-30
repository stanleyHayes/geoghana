import { API_VERSION, DEFAULT_API_URL, TESTED_DATASET_VERSION } from "@ghanageo/core";
import type { BoundaryFeature, DatasetPage, District, DistrictPage, DownloadList, Place, PlacePage, Region, RegionPage, ReverseResult, SearchPage } from "@ghanageo/core";

export * from "@ghanageo/core";

export type GhanaGeoClientOptions = {
  baseUrl?: string;
  apiKey?: string;
  fetcher?: typeof fetch;
  retry?: false | RetryOptions;
  telemetry?: false | TelemetryOptions;
  maxJsonBytes?: number;
  maxArtifactBytes?: number;
};

export type RetryOptions = {
  /** Number of retries after the initial safe request. Capped at five. */
  maxRetries?: number;
  baseDelayMs?: number;
  maxDelayMs?: number;
  sleep?: (milliseconds: number, signal?: AbortSignal) => Promise<void>;
};

export type TelemetryEvent = {
  path: string;
  method: "GET" | "POST";
  attempt: number;
  status?: number;
  durationMs: number;
  errorCode?: string;
};

export type TelemetryOptions = {
  onEvent: (event: TelemetryEvent) => void;
};

export type Page<T> = { data: T[]; datasetVersion?: string; nextCursor?: string };
export type PageRequest = { cursor?: string; limit?: number };
export type DatasetEntity = "regions" | "districts" | "places";
export type DatasetFormat = "json" | "csv" | "geojson";
export type ArtifactDownloadOptions = { expectedSha256?: string };
export type ArtifactSink = WritableStream<Uint8Array> | {
  write(chunk: Uint8Array): void | Promise<void>;
  close?(): void | Promise<void>;
  abort?(reason: unknown): void | Promise<void>;
};

const DEFAULT_RETRY: Required<Omit<RetryOptions, "sleep">> = { maxRetries: 2, baseDelayMs: 100, maxDelayMs: 2_000 };
const MAX_RETRIES = 5;
const MAX_DELAY_MS = 60_000;
const DEFAULT_MAX_JSON_BYTES = 5 * 1024 * 1024;
const MAX_JSON_BYTES = 64 * 1024 * 1024;
const DEFAULT_MAX_ARTIFACT_BYTES = 100 * 1024 * 1024;
const MAX_ARTIFACT_BYTES = 1024 * 1024 * 1024;
const RETRYABLE_STATUS = new Set([429, 502, 503, 504]);

function abortError(): DOMException { return new DOMException("The operation was aborted", "AbortError"); }
function boundedInteger(name: string, value: number | undefined, fallback: number, maximum: number): number {
  const resolved = value ?? fallback;
  if (!Number.isFinite(resolved) || !Number.isInteger(resolved) || resolved < 0) throw new RangeError(`${name} must be a finite non-negative integer.`);
  return Math.min(resolved, maximum);
}
function defaultSleep(milliseconds: number, signal?: AbortSignal): Promise<void> {
  if (signal?.aborted) return Promise.reject(abortError());
  return new Promise((resolve, reject) => {
    const cleanup = () => signal?.removeEventListener("abort", onAbort);
    const timer = setTimeout(() => { cleanup(); resolve(); }, milliseconds);
    const onAbort = () => { clearTimeout(timer); cleanup(); reject(abortError()); };
    signal?.addEventListener("abort", onAbort, { once: true });
  });
}

function retryAfterMilliseconds(value: string | null, now = Date.now()): number | undefined {
  if (value === null) return undefined;
  const seconds = Number(value);
  if (Number.isFinite(seconds) && seconds >= 0) return seconds * 1_000;
  const date = Date.parse(value);
  return Number.isFinite(date) ? Math.max(0, date - now) : undefined;
}

function concat(chunks: Uint8Array[], size: number): Uint8Array {
  const result = new Uint8Array(size);
  let offset = 0;
  for (const chunk of chunks) { result.set(chunk, offset); offset += chunk.byteLength; }
  return result;
}

async function readWithLimit(response: Response, limit: number, signal?: AbortSignal, onChunk?: (chunk: Uint8Array) => void | Promise<void>): Promise<Uint8Array | undefined> {
  const declared = Number(response.headers.get("content-length"));
  if (Number.isFinite(declared) && declared > limit) throw new GhanaGeoError(`GhanaGeo response exceeds the ${limit}-byte limit.`, response.status, "RESPONSE_TOO_LARGE");
  const reader = response.body?.getReader();
  if (!reader) {
    const value = new Uint8Array(await response.arrayBuffer());
    if (value.byteLength > limit) throw new GhanaGeoError(`GhanaGeo response exceeds the ${limit}-byte limit.`, response.status, "RESPONSE_TOO_LARGE");
    await onChunk?.(value);
    return onChunk ? undefined : value;
  }
  const chunks: Uint8Array[] = [];
  let size = 0;
  try {
    while (true) {
      if (signal?.aborted) { await reader.cancel(signal.reason).catch(() => undefined); throw abortError(); }
      const read = reader.read();
      const result = signal ? await new Promise<ReadableStreamReadResult<Uint8Array>>((resolve, reject) => {
        const onAbort = () => { void reader.cancel(signal.reason).catch(() => undefined); reject(abortError()); };
        signal.addEventListener("abort", onAbort, { once: true });
        read.then(
          (value) => { signal.removeEventListener("abort", onAbort); resolve(value); },
          (error) => { signal.removeEventListener("abort", onAbort); reject(error); },
        );
      }) : await read;
      if (result.done) break;
      size += result.value.byteLength;
      if (size > limit) {
        await reader.cancel("response limit exceeded").catch(() => undefined);
        throw new GhanaGeoError(`GhanaGeo response exceeds the ${limit}-byte limit.`, response.status, "RESPONSE_TOO_LARGE");
      }
      if (onChunk) await onChunk(result.value); else chunks.push(result.value);
    }
  } finally {
    reader.releaseLock();
  }
  return onChunk ? undefined : concat(chunks, size);
}

async function sha256Hex(value: Uint8Array): Promise<string> {
  const digest = await globalThis.crypto.subtle.digest("SHA-256", value as Uint8Array<ArrayBuffer>);
  return [...new Uint8Array(digest)].map((byte) => byte.toString(16).padStart(2, "0")).join("");
}

export class GhanaGeoError extends Error {
  constructor(
    message: string,
    public readonly status: number,
    public readonly code?: string,
    public readonly requestId?: string,
    public readonly details?: Record<string, unknown>,
    public readonly docs?: string,
  ) {
    super(message);
    this.name = "GhanaGeoError";
  }
}

export class GhanaGeoClient {
  readonly baseUrl: string;
  private readonly apiKey: string | undefined;
  private readonly fetcher: typeof fetch;
  private readonly retry: (Required<Omit<RetryOptions, "sleep">> & { sleep: NonNullable<RetryOptions["sleep"]> }) | undefined;
  private readonly telemetry: TelemetryOptions | undefined;
  private readonly maxJsonBytes: number;
  private readonly maxArtifactBytes: number;

  static readonly apiVersion = API_VERSION;
  static readonly testedDatasetVersion = TESTED_DATASET_VERSION;
  readonly apiVersion = GhanaGeoClient.apiVersion;
  readonly testedDatasetVersion = GhanaGeoClient.testedDatasetVersion;

  constructor(options: GhanaGeoClientOptions = {}) {
    this.baseUrl = (options.baseUrl ?? DEFAULT_API_URL).replace(/\/$/, "");
    this.apiKey = options.apiKey;
    this.fetcher = options.fetcher ?? globalThis.fetch;
    const configuredRetry = options.retry === false ? undefined : options.retry ?? {};
    if (configuredRetry) {
      const maxDelayMs = boundedInteger("retry.maxDelayMs", configuredRetry.maxDelayMs, DEFAULT_RETRY.maxDelayMs, MAX_DELAY_MS);
      this.retry = {
        maxRetries: boundedInteger("retry.maxRetries", configuredRetry.maxRetries, DEFAULT_RETRY.maxRetries, MAX_RETRIES),
        baseDelayMs: Math.min(boundedInteger("retry.baseDelayMs", configuredRetry.baseDelayMs, DEFAULT_RETRY.baseDelayMs, MAX_DELAY_MS), maxDelayMs),
        maxDelayMs,
        sleep: configuredRetry.sleep ?? defaultSleep,
      };
    }
    this.telemetry = options.telemetry === false ? undefined : options.telemetry;
    this.maxJsonBytes = boundedInteger("maxJsonBytes", options.maxJsonBytes, DEFAULT_MAX_JSON_BYTES, MAX_JSON_BYTES);
    this.maxArtifactBytes = boundedInteger("maxArtifactBytes", options.maxArtifactBytes, DEFAULT_MAX_ARTIFACT_BYTES, MAX_ARTIFACT_BYTES);
    if (!this.fetcher) throw new Error("GhanaGeoClient needs a fetch implementation in this runtime.");
  }

  async requestRaw<T>(path: string, params?: Record<string, string | number | boolean | undefined | null>, signal?: AbortSignal): Promise<{ response: Response; body: T }> {
    const url = new URL(`${this.baseUrl}${path}`);
    for (const [key, value] of Object.entries(params ?? {})) {
      if (value !== undefined && value !== null && value !== "") url.searchParams.set(key, String(value));
    }
    const headers: Record<string, string> = { Accept: "application/json" };
    if (this.apiKey) headers.Authorization = `Bearer ${this.apiKey}`;
    const response = await this.fetchWithRetry(url, { headers, ...(signal ? { signal } : {}) }, path, signal);
    const bytes = await readWithLimit(response, this.maxJsonBytes, signal) ?? new Uint8Array();
    const body = JSON.parse(new TextDecoder().decode(bytes) || "{}") as Record<string, any>;
    if (!response.ok) {
      const failure = body.error ?? {};
      throw new GhanaGeoError(failure.message ?? `GhanaGeo request failed (${response.status})`, response.status, failure.code, failure.requestId, failure.details, failure.docs);
    }
    return { response, body: body as T };
  }

  private async fetchWithRetry(resource: URL | string, init: RequestInit, path: string, signal?: AbortSignal): Promise<Response> {
    const attempts = 1 + (this.retry?.maxRetries ?? 0);
    for (let attempt = 0; attempt < attempts; attempt += 1) {
      if (signal?.aborted) throw abortError();
      const startedAt = Date.now();
      try {
        const response = await this.fetcher(resource, init);
        this.emitTelemetry({ path, method: init.method === "POST" ? "POST" : "GET", attempt, status: response.status, durationMs: Date.now() - startedAt });
        if (!this.retry || !RETRYABLE_STATUS.has(response.status) || attempt === attempts - 1 || init.method === "POST") return response;
        const retryAfter = retryAfterMilliseconds(response.headers.get("retry-after"));
        const delay = Math.min(this.retry.maxDelayMs, retryAfter ?? this.retry.baseDelayMs * 2 ** attempt);
        await this.retry.sleep(delay, signal);
      } catch (error) {
        if (signal?.aborted || (error instanceof DOMException && error.name === "AbortError")) throw error;
        this.emitTelemetry({ path, method: init.method === "POST" ? "POST" : "GET", attempt, durationMs: Date.now() - startedAt });
        if (!this.retry || attempt === attempts - 1 || init.method === "POST") throw error;
        await this.retry.sleep(Math.min(this.retry.maxDelayMs, this.retry.baseDelayMs * 2 ** attempt), signal);
      }
    }
    throw new Error("GhanaGeo retry loop exhausted unexpectedly");
  }

  private emitTelemetry(event: TelemetryEvent): void {
    try { this.telemetry?.onEvent(event); } catch { /* Telemetry observers never affect transport behavior. */ }
  }

  async request<T>(path: string, params?: Record<string, string | number | boolean | undefined | null>, signal?: AbortSignal): Promise<T> {
    return (await this.requestRaw<T>(path, params, signal)).body;
  }

  get<T>(path: string, params?: Record<string, string | number | boolean | undefined | null>, signal?: AbortSignal) {
    return this.request<T>(path, params, signal);
  }

  regions(params: { cursor?: string; limit?: number } = {}, signal?: AbortSignal) { return this.get<RegionPage>("/regions", params, signal); }
  region(id: string, signal?: AbortSignal) { return this.get<Region>(`/regions/${encodeURIComponent(id)}`, undefined, signal); }
  regionDistricts(id: string, params: PageRequest = {}, signal?: AbortSignal) { return this.get<DistrictPage>(`/regions/${encodeURIComponent(id)}/districts`, params, signal); }
  districts(params: { regionId?: string; q?: string; cursor?: string; limit?: number } = {}, signal?: AbortSignal) { return this.get<DistrictPage>("/districts", params, signal); }
  district(id: string, signal?: AbortSignal) { return this.get<District>(`/districts/${encodeURIComponent(id)}`, undefined, signal); }
  districtPlaces(id: string, params: { cursor?: string; limit?: number; type?: string } = {}, signal?: AbortSignal) { return this.get<PlacePage>(`/districts/${encodeURIComponent(id)}/places`, params, signal); }
  places(params: { regionId?: string; districtId?: string; type?: string; q?: string; cursor?: string; limit?: number } = {}, signal?: AbortSignal) { return this.get<PlacePage>("/places", params, signal); }
  place(id: string, signal?: AbortSignal) { return this.get<Place>(`/places/${encodeURIComponent(id)}`, undefined, signal); }
  search(q: string, params: { regionId?: string; districtId?: string; type?: string; limit?: number } = {}, signal?: AbortSignal) { return this.get<SearchPage>("/search", { q, ...params }, signal); }
  autocomplete(q: string, limit = 10, signal?: AbortSignal) { return this.get<SearchPage>("/autocomplete", { q, limit }, signal); }
  geocode(q: string, limit = 10, signal?: AbortSignal) { return this.get<SearchPage>("/geocode", { q, limit }, signal); }
  reverse(latitude: number, longitude: number, signal?: AbortSignal) { return this.get<ReverseResult>("/reverse", { lat: latitude, lng: longitude }, signal); }
  nearby(latitude: number, longitude: number, radius = 5000, limit = 20, signal?: AbortSignal) { return this.get<PlacePage>("/nearby", { lat: latitude, lng: longitude, radius, limit }, signal); }
  boundary(id: string, signal?: AbortSignal) { return this.get<BoundaryFeature>(`/boundaries/${encodeURIComponent(id)}`, undefined, signal); }
  datasets(signal?: AbortSignal) { return this.get<DatasetPage>("/datasets", undefined, signal); }
  datasetDownloads(version: string, signal?: AbortSignal) { return this.get<DownloadList>(`/datasets/${encodeURIComponent(version)}/downloads`, undefined, signal); }
  roads(signal?: AbortSignal) { return this.get<Record<string, unknown>>("/roads", undefined, signal); }
  pointsOfInterest(signal?: AbortSignal) { return this.get<Record<string, unknown>>("/pois", undefined, signal); }

  async downloadDatasetArtifact(version: string, entity: DatasetEntity, format: DatasetFormat, signalOrOptions?: AbortSignal | ArtifactDownloadOptions, signal?: AbortSignal): Promise<Uint8Array> {
    const isSignal = signalOrOptions !== undefined && "aborted" in signalOrOptions;
    const options = isSignal ? undefined : signalOrOptions as ArtifactDownloadOptions | undefined;
    const resolvedSignal = isSignal ? signalOrOptions as AbortSignal : signal;
    const path = `/datasets/${encodeURIComponent(version)}/downloads/${encodeURIComponent(entity)}.${encodeURIComponent(format)}`;
    const headers: Record<string, string> = { Accept: "application/octet-stream" };
    if (this.apiKey) headers.Authorization = `Bearer ${this.apiKey}`;
    const response = await this.fetchWithRetry(new URL(`${this.baseUrl}${path}`), { headers, ...(resolvedSignal ? { signal: resolvedSignal } : {}) }, path, resolvedSignal);
    if (!response.ok) {
      const bytes = await readWithLimit(response, this.maxJsonBytes, resolvedSignal) ?? new Uint8Array();
      const body = JSON.parse(new TextDecoder().decode(bytes) || "{}") as Record<string, any>;
      const failure = body.error ?? {};
      throw new GhanaGeoError(failure.message ?? `GhanaGeo request failed (${response.status})`, response.status, failure.code, failure.requestId, failure.details, failure.docs);
    }
    const value = await readWithLimit(response, this.maxArtifactBytes, resolvedSignal) ?? new Uint8Array();
    if (options?.expectedSha256) await this.verifyArtifactChecksum(value, options.expectedSha256);
    return value;
  }

  async downloadDatasetArtifactTo(version: string, entity: DatasetEntity, format: DatasetFormat, sink: ArtifactSink, options: ArtifactDownloadOptions = {}, signal?: AbortSignal): Promise<void> {
    const path = `/datasets/${encodeURIComponent(version)}/downloads/${encodeURIComponent(entity)}.${encodeURIComponent(format)}`;
    const headers: Record<string, string> = { Accept: "application/octet-stream" };
    if (this.apiKey) headers.Authorization = `Bearer ${this.apiKey}`;
    const response = await this.fetchWithRetry(new URL(`${this.baseUrl}${path}`), { headers, ...(signal ? { signal } : {}) }, path, signal);
    if (!response.ok) {
      const bytes = await readWithLimit(response, this.maxJsonBytes, signal) ?? new Uint8Array();
      const body = JSON.parse(new TextDecoder().decode(bytes) || "{}") as Record<string, any>;
      const failure = body.error ?? {};
      throw new GhanaGeoError(failure.message ?? `GhanaGeo request failed (${response.status})`, response.status, failure.code, failure.requestId, failure.details, failure.docs);
    }
    const writer = "getWriter" in sink ? sink.getWriter() : sink;
    const checksumChunks: Uint8Array[] = [];
    let checksumSize = 0;
    try {
      await readWithLimit(response, this.maxArtifactBytes, signal, async (chunk) => {
        if (options.expectedSha256) { checksumChunks.push(chunk.slice()); checksumSize += chunk.byteLength; }
        await writer.write(chunk);
      });
      if (options.expectedSha256) await this.verifyArtifactChecksum(concat(checksumChunks, checksumSize), options.expectedSha256);
      await writer.close?.();
    } catch (error) {
      await writer.abort?.(error);
      throw error;
    } finally {
      if ("releaseLock" in writer && typeof writer.releaseLock === "function") writer.releaseLock();
    }
  }

  private async verifyArtifactChecksum(value: Uint8Array, expected: string): Promise<void> {
    if (!/^[a-fA-F0-9]{64}$/.test(expected)) throw new RangeError("expectedSha256 must be a 64-character hexadecimal SHA-256 digest.");
    const actual = await sha256Hex(value);
    if (actual !== expected.toLowerCase()) throw new GhanaGeoError("Dataset artifact checksum does not match expected SHA-256.", 422, "CHECKSUM_MISMATCH", undefined, { expected: expected.toLowerCase(), actual });
  }

  async *pages<T>(fetchPage: (cursor?: string) => Promise<Page<T>>): AsyncGenerator<Page<T>, void, undefined> {
    let cursor: string | undefined;
    const seen = new Set<string>();
    do {
      const page = await fetchPage(cursor);
      const nextCursor = page.nextCursor;
      if (nextCursor && seen.has(nextCursor)) throw new GhanaGeoError("GhanaGeo returned a repeated pagination cursor", 500, "INVALID_CURSOR");
      if (nextCursor) seen.add(nextCursor);
      yield page;
      cursor = nextCursor;
    } while (cursor);
  }

  regionPages(params: Omit<PageRequest, "cursor"> = {}, signal?: AbortSignal) { return this.pages((cursor) => this.regions({ ...params, ...(cursor ? { cursor } : {}) }, signal)); }
  districtPages(params: Omit<Parameters<GhanaGeoClient["districts"]>[0], "cursor"> = {}, signal?: AbortSignal) { return this.pages((cursor) => this.districts({ ...params, ...(cursor ? { cursor } : {}) }, signal)); }
  placePages(params: Omit<Parameters<GhanaGeoClient["places"]>[0], "cursor"> = {}, signal?: AbortSignal) { return this.pages((cursor) => this.places({ ...params, ...(cursor ? { cursor } : {}) }, signal)); }

  async graphql<TData, TVariables extends Record<string, unknown> = Record<string, unknown>>(query: string, variables?: TVariables, signal?: AbortSignal): Promise<TData> {
    const endpoint = this.baseUrl.replace(/\/v1$/, "") + "/graphql";
    const headers: Record<string, string> = { "Content-Type": "application/json", Accept: "application/json" };
    if (this.apiKey) headers.Authorization = `Bearer ${this.apiKey}`;
    const response = await this.fetchWithRetry(endpoint, { method: "POST", headers, body: JSON.stringify({ query, variables }), ...(signal ? { signal } : {}) }, "/graphql", signal);
    const body = await response.json() as Record<string, any>;
    if (!response.ok || body.errors?.length) {
      const failure = body.errors?.[0];
      throw new GhanaGeoError(failure?.message ?? `GraphQL request failed (${response.status})`, response.status, failure?.extensions?.code, failure?.extensions?.requestId, failure?.extensions, failure?.extensions?.docs);
    }
    return body.data as TData;
  }
}
