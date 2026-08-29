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
    const fetcher = vi.fn<typeof fetch>(async () => jsonResponse({ error: { code: "DENIED", message: "Nope", requestId: "req_1", details: { status } } }, status));
    const client = new GhanaGeoClient({ baseUrl: "https://example.test/v1", fetcher });
    const error = await client.regions().catch((cause) => cause);
    expect(error).toBeInstanceOf(GhanaGeoError);
    expect(error).toMatchObject({ status, code: "DENIED", requestId: "req_1", details: { status } });
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
});
