import { describe, expect, it, vi } from "vitest";
import { GhanaGeoClient, GhanaGeoError } from "../src";

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

describe("GhanaGeoClient", () => {
  it("forwards AbortSignal and only sends an explicitly supplied key", async () => {
    const fetcher = vi.fn<typeof fetch>(async () => jsonResponse({ data: [] }));
    const controller = new AbortController();
    const anonymous = new GhanaGeoClient({ baseUrl: "https://example.test/v1", fetcher });
    await anonymous.regions({}, controller.signal);
    expect(fetcher.mock.calls[0]?.[1]?.signal).toBe(controller.signal);
    expect(new Headers(fetcher.mock.calls[0]?.[1]?.headers).has("Authorization")).toBe(false);

    const keyed = new GhanaGeoClient({ baseUrl: "https://example.test/v1", apiKey: "gh_test_public_fixture", fetcher });
    await keyed.regions();
    expect(new Headers(fetcher.mock.calls[1]?.[1]?.headers).get("Authorization")).toBe("Bearer gh_test_public_fixture");
  });

  it.each([401, 403, 429])("maps HTTP %s errors to GhanaGeoError", async (status) => {
    const fetcher = vi.fn<typeof fetch>(async () => jsonResponse({ error: { code: "DENIED", message: "Nope", requestId: "req_1", docs: "/docs/errors/DENIED", details: { status } } }, status));
    const client = new GhanaGeoClient({ baseUrl: "https://example.test/v1", fetcher });
    const error = await client.regions().catch((cause) => cause);
    expect(error).toBeInstanceOf(GhanaGeoError);
    expect(error).toMatchObject({ status, code: "DENIED", requestId: "req_1", docs: "/docs/errors/DENIED", details: { status } });
  });

  it("maps GraphQL extensions onto the typed error", async () => {
    const fetcher = vi.fn<typeof fetch>(async () => jsonResponse({ errors: [{ message: "Too complex", extensions: { code: "QUERY_TOO_COMPLEX", requestId: "req_gql" } }] }));
    const client = new GhanaGeoClient({ baseUrl: "https://example.test/v1", fetcher });
    const error = await client.graphql("query { regions { id } }").catch((cause) => cause);
    expect(error).toMatchObject({ status: 200, code: "QUERY_TOO_COMPLEX", requestId: "req_gql" });
  });

  it("uses the published REST parameter names", async () => {
    const fetcher = vi.fn<typeof fetch>(async () => jsonResponse({ data: [] }));
    const client = new GhanaGeoClient({ baseUrl: "https://example.test/v1", fetcher });
    await client.nearby(5.56, -0.2, 2500, 12);
    const url = fetcher.mock.calls[0]?.[0] as URL;
    expect(url.searchParams.get("radius")).toBe("2500");
    expect(url.searchParams.get("limit")).toBe("12");
    expect(url.searchParams.has("radiusMeters")).toBe(false);
  });

  it("exposes all nested geography, boundary and dataset public paths", async () => {
    const fetcher = vi.fn<typeof fetch>(async (resource) => String(resource).includes("downloads/regions.json")
      ? new Response("fixture")
      : jsonResponse({ data: [], downloads: [] }));
    const client = new GhanaGeoClient({ baseUrl: "https://example.test/v1", fetcher, retry: false });
    await client.regionDistricts("region/a", { limit: 2 });
    await client.district("district/a");
    await client.districtPlaces("district/a", { type: "city" });
    await client.boundary("region/a");
    await client.datasets();
    await client.datasetDownloads("2026.08.1");
    await client.roads();
    await client.pointsOfInterest();
    expect(new TextDecoder().decode(await client.downloadDatasetArtifact("2026.08.1", "regions", "json"))).toBe("fixture");
    expect(fetcher.mock.calls.map(([resource]) => new URL(String(resource)).pathname)).toEqual([
      "/v1/regions/region%2Fa/districts", "/v1/districts/district%2Fa", "/v1/districts/district%2Fa/places",
      "/v1/boundaries/region%2Fa", "/v1/datasets", "/v1/datasets/2026.08.1/downloads", "/v1/roads", "/v1/pois",
      "/v1/datasets/2026.08.1/downloads/regions.json",
    ]);
  });

  it("retries safe transient reads with bounded backoff and never retries GraphQL posts", async () => {
    const sleep = vi.fn(async () => undefined);
    const fetcher = vi.fn<typeof fetch>()
      .mockResolvedValueOnce(jsonResponse({}, 503))
      .mockResolvedValueOnce(jsonResponse({ data: [], datasetVersion: "v" }))
      .mockResolvedValueOnce(jsonResponse({ errors: [{ message: "unavailable" }] }, 503));
    const client = new GhanaGeoClient({ baseUrl: "https://example.test/v1", fetcher, retry: { maxRetries: 99, baseDelayMs: 25, sleep } });
    await client.regions();
    expect(sleep).toHaveBeenCalledWith(25, undefined);
    expect(fetcher).toHaveBeenCalledTimes(2);
    await expect(client.graphql("query { regions { id } }")).rejects.toBeInstanceOf(GhanaGeoError);
    expect(fetcher).toHaveBeenCalledTimes(3);
  });

  it("iterates stable cursor pages lazily and rejects cursor loops", async () => {
    const client = new GhanaGeoClient({ baseUrl: "https://example.test/v1", fetcher: vi.fn<typeof fetch>(), retry: false });
    const fetchPage = vi.fn(async (cursor?: string) => ({ data: [{ id: cursor ?? "first" }], datasetVersion: "v", nextCursor: "opaque-cursor" }));
    const iterator = client.pages(fetchPage);
    expect(fetchPage).not.toHaveBeenCalled();
    await iterator.next();
    expect(fetchPage).toHaveBeenCalledWith(undefined);
    await expect(iterator.next()).rejects.toMatchObject({ code: "INVALID_CURSOR" });
  });

  it("emits metadata-only telemetry and supports an explicit opt-out", async () => {
    const onEvent = vi.fn();
    const fetcher = vi.fn<typeof fetch>(async () => jsonResponse({ data: [] }));
    await new GhanaGeoClient({ baseUrl: "https://example.test/v1", apiKey: "private", fetcher, telemetry: { onEvent }, retry: false }).regions();
    expect(onEvent).toHaveBeenCalledWith(expect.objectContaining({ path: "/regions", method: "GET", attempt: 0, status: 200 }));
    expect(JSON.stringify(onEvent.mock.calls)).not.toContain("private");
    onEvent.mockClear();
    await new GhanaGeoClient({ baseUrl: "https://example.test/v1", fetcher, telemetry: false, retry: false }).regions();
    expect(onEvent).not.toHaveBeenCalled();
  });

  it("isolates telemetry observer failures from transport and retry behavior", async () => {
    const sleep = vi.fn(async () => undefined);
    const fetcher = vi.fn<typeof fetch>()
      .mockResolvedValueOnce(jsonResponse({}, 503))
      .mockResolvedValueOnce(jsonResponse({ data: [], datasetVersion: "v" }));
    const client = new GhanaGeoClient({
      baseUrl: "https://example.test/v1",
      fetcher,
      retry: { maxRetries: 1, baseDelayMs: 5, sleep },
      telemetry: { onEvent: () => { throw new Error("observer failed"); } },
    });
    await expect(client.regions()).resolves.toMatchObject({ data: [] });
    expect(fetcher).toHaveBeenCalledTimes(2);
    expect(sleep).toHaveBeenCalledTimes(1);
  });

  it.each([
    ["maxRetries", Number.NaN], ["maxRetries", -1], ["maxRetries", 1.5],
    ["baseDelayMs", Number.POSITIVE_INFINITY], ["baseDelayMs", -1], ["baseDelayMs", 0.5],
    ["maxDelayMs", Number.NaN], ["maxDelayMs", -1], ["maxDelayMs", 0.5],
  ] as const)("rejects invalid retry.%s configuration", (field, value) => {
    expect(() => new GhanaGeoClient({ fetcher: vi.fn<typeof fetch>(), retry: { [field]: value } })).toThrow(RangeError);
  });

  it("caps retries and numeric or HTTP-date Retry-After delays", async () => {
    const sleep = vi.fn(async () => undefined);
    const future = new Date(Date.now() + 120_000).toUTCString();
    const fetcher = vi.fn<typeof fetch>()
      .mockResolvedValueOnce(new Response("{}", { status: 429, headers: { "retry-after": "10" } }))
      .mockResolvedValueOnce(new Response("{}", { status: 503, headers: { "retry-after": future } }))
      .mockResolvedValue(jsonResponse({ data: [], datasetVersion: "v" }));
    const client = new GhanaGeoClient({ baseUrl: "https://example.test/v1", fetcher, retry: { maxRetries: 99, baseDelayMs: 99_999, maxDelayMs: 40, sleep } });
    await client.regions();
    expect(sleep.mock.calls.slice(0, 2)).toEqual([[40, undefined], [40, undefined]]);
    expect(fetcher).toHaveBeenCalledTimes(3);
  });

  it("removes the abort listener when the default retry delay resolves", async () => {
    vi.useFakeTimers();
    const controller = new AbortController();
    const add = vi.spyOn(controller.signal, "addEventListener");
    const remove = vi.spyOn(controller.signal, "removeEventListener");
    const fetcher = vi.fn<typeof fetch>()
      .mockResolvedValueOnce(jsonResponse({}, 503))
      .mockResolvedValueOnce(jsonResponse({ data: [], datasetVersion: "v" }));
    const promise = new GhanaGeoClient({ baseUrl: "https://example.test/v1", fetcher, retry: { maxRetries: 1, baseDelayMs: 1 } }).regions({}, controller.signal);
    await vi.runAllTimersAsync();
    await expect(promise).resolves.toMatchObject({ data: [] });
    expect(remove).toHaveBeenCalledTimes(add.mock.calls.length);
    expect(remove).toHaveBeenCalled();
    vi.useRealTimers();
  });

  it("removes the abort listener when the default retry delay is cancelled", async () => {
    const controller = new AbortController();
    const remove = vi.spyOn(controller.signal, "removeEventListener");
    const fetcher = vi.fn<typeof fetch>(async () => jsonResponse({}, 503));
    const promise = new GhanaGeoClient({ baseUrl: "https://example.test/v1", fetcher, retry: { maxRetries: 1, baseDelayMs: 60_000 } }).regions({}, controller.signal);
    await Promise.resolve();
    controller.abort();
    await expect(promise).rejects.toMatchObject({ name: "AbortError" });
    expect(remove).toHaveBeenCalledTimes(1);
  });

  it("returns the raw Place contract instead of wrapping it in data", async () => {
    const place = { id: "gh-place-kumasi", name: "Kumasi", datasetVersion: "2026.08.1" };
    const client = new GhanaGeoClient({ baseUrl: "https://example.test/v1", fetcher: vi.fn(async () => jsonResponse(place)), retry: false });
    await expect(client.place(place.id)).resolves.toEqual(place);
  });

  it.each([
    ["successful JSON", 200, { data: ["too large"] }],
    ["error JSON", 400, { error: { code: "INVALID_ARGUMENT", message: "too large", docs: "/docs/errors/INVALID_ARGUMENT" } }],
  ] as const)("rejects oversized %s bodies before decoding", async (_name, status, body) => {
    const client = new GhanaGeoClient({ baseUrl: "https://example.test/v1", maxJsonBytes: 8, fetcher: vi.fn(async () => jsonResponse(body, status)), retry: false });
    await expect(client.regions()).rejects.toMatchObject({ code: "RESPONSE_TOO_LARGE", status });
  });

  it("caps buffered artifact downloads and verifies optional SHA-256", async () => {
    const payload = new TextEncoder().encode("fixture");
    const expectedSha256 = [...new Uint8Array(await crypto.subtle.digest("SHA-256", payload))].map((byte) => byte.toString(16).padStart(2, "0")).join("");
    const response = () => new Response(payload, { headers: { "content-type": "application/octet-stream" } });
    const verified = new GhanaGeoClient({ baseUrl: "https://example.test/v1", maxArtifactBytes: payload.byteLength, fetcher: vi.fn(async () => response()), retry: false });
    await expect(verified.downloadDatasetArtifact("v", "regions", "json", { expectedSha256 })).resolves.toEqual(payload);
    await expect(verified.downloadDatasetArtifact("v", "regions", "json", { expectedSha256: "0".repeat(64) })).rejects.toMatchObject({ code: "CHECKSUM_MISMATCH" });
    const capped = new GhanaGeoClient({ baseUrl: "https://example.test/v1", maxArtifactBytes: payload.byteLength - 1, fetcher: vi.fn(async () => response()), retry: false });
    await expect(capped.downloadDatasetArtifact("v", "regions", "json")).rejects.toMatchObject({ code: "RESPONSE_TOO_LARGE" });
  });

  it("streams artifacts to a sink and aborts the sink on checksum mismatch", async () => {
    const writes: Uint8Array[] = [];
    const sink = { write: vi.fn(async (chunk: Uint8Array) => { writes.push(chunk.slice()); }), close: vi.fn(), abort: vi.fn() };
    const client = new GhanaGeoClient({ baseUrl: "https://example.test/v1", fetcher: vi.fn(async () => new Response("fixture")), retry: false });
    await expect(client.downloadDatasetArtifactTo("v", "regions", "json", sink, { expectedSha256: "0".repeat(64) })).rejects.toMatchObject({ code: "CHECKSUM_MISMATCH" });
    expect(new TextDecoder().decode(writes[0])).toBe("fixture");
    expect(sink.close).not.toHaveBeenCalled();
    expect(sink.abort).toHaveBeenCalledTimes(1);
  });

  it("propagates cancellation while reading a streamed JSON body", async () => {
    let cancelled = false;
    const body = new ReadableStream<Uint8Array>({ cancel() { cancelled = true; } });
    const controller = new AbortController();
    const client = new GhanaGeoClient({ baseUrl: "https://example.test/v1", fetcher: vi.fn(async () => new Response(body)), retry: false });
    const request = client.regions({}, controller.signal);
    await Promise.resolve();
    controller.abort();
    await expect(request).rejects.toMatchObject({ name: "AbortError" });
    expect(cancelled).toBe(true);
  });
});
