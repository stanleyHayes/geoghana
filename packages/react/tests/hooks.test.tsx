// @vitest-environment jsdom
import { QueryClient } from "@tanstack/react-query";
import { act, render, renderHook, waitFor } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { setupServer } from "msw/node";
import type { PropsWithChildren } from "react";
import { afterAll, afterEach, beforeAll, describe, expect, it, vi } from "vitest";
import { GhanaGeoClient } from "@ghanageo/client";
import { GhanaGeoProvider, ghanaGeoKeys, useGhanaGeoClient, useRegions, useSearch } from "../src";

let regionRequests = 0;
let searchRequests = 0;
const server = setupServer(
  http.get("https://example.test/v1/regions", () => {
    regionRequests += 1;
    return HttpResponse.json({ data: [{ id: "01KDVDNA00N6BFFK8VF5K8YXPW", countryCode: "GH", name: "Ahafo", status: "ACTIVE", verificationStatus: "REFERENCE", provenance: { sourceId: "fixture" }, datasetVersion: "2026.08.3-ulid" }], datasetVersion: "2026.08.3-ulid" });
  }),
  http.get("https://example.test/v1/search", ({ request }) => {
    searchRequests += 1;
    return HttpResponse.json({ data: [], datasetVersion: "2026.08.3-ulid", q: new URL(request.url).searchParams.get("q") });
  }),
);

beforeAll(() => server.listen({ onUnhandledRequest: "error" }));
afterEach(() => { server.resetHandlers(); regionRequests = 0; searchRequests = 0; });
afterAll(() => server.close());

const client = new GhanaGeoClient({ baseUrl: "https://example.test/v1" });
function makeWrapper(resolvedClient = client) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return function Wrapper({ children }: PropsWithChildren) {
    return <GhanaGeoProvider client={resolvedClient} queryClient={queryClient}>{children}</GhanaGeoProvider>;
  };
}

describe("@ghanageo/react", () => {
  it("reuses the provider client across renders", () => {
    const { result, rerender } = renderHook(() => useGhanaGeoClient(), { wrapper: makeWrapper() });
    const first = result.current;
    rerender();
    expect(result.current).toBe(first);
  });

  it("recreates an options-owned client when retry or telemetry configuration changes", () => {
    let observed: GhanaGeoClient | undefined;
    function Consumer() { observed = useGhanaGeoClient(); return null; }
    const fetcher = vi.fn<typeof fetch>();
    const firstRetry = { maxRetries: 1 };
    const secondRetry = { maxRetries: 2 };
    const firstTelemetry = { onEvent: vi.fn() };
    const secondTelemetry = { onEvent: vi.fn() };
    const view = render(<GhanaGeoProvider options={{ fetcher, retry: firstRetry, telemetry: firstTelemetry }}><Consumer /></GhanaGeoProvider>);
    const first = observed;
    view.rerender(<GhanaGeoProvider options={{ fetcher, retry: secondRetry, telemetry: firstTelemetry }}><Consumer /></GhanaGeoProvider>);
    const second = observed;
    view.rerender(<GhanaGeoProvider options={{ fetcher, retry: secondRetry, telemetry: secondTelemetry }}><Consumer /></GhanaGeoProvider>);
    expect(second).not.toBe(first);
    expect(observed).not.toBe(second);
  });

  it("does not amplify client failures with an additional TanStack retry layer", async () => {
    const fetcher = vi.fn<typeof fetch>(async () => HttpResponse.json({ error: { code: "INTERNAL", message: "failed" } }, { status: 500 }));
    function Wrapper({ children }: PropsWithChildren) {
      return <GhanaGeoProvider options={{ baseUrl: "https://example.test/v1", fetcher, retry: false }}>{children}</GhanaGeoProvider>;
    }
    const { result } = renderHook(() => useRegions(), { wrapper: Wrapper });
    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(fetcher).toHaveBeenCalledTimes(1);
  });

  it("deduplicates concurrent region hooks", async () => {
    const fetcher = async () => {
      regionRequests += 1;
      return HttpResponse.json({ data: [], datasetVersion: "2026.08.3-ulid" });
    };
    const { result } = renderHook(() => [useRegions(), useRegions()] as const, {
      wrapper: makeWrapper(new GhanaGeoClient({ baseUrl: "https://example.test/v1", fetcher })),
    });
    await waitFor(() => expect(result.current[0].isSuccess).toBe(true));
    expect(result.current[1].isSuccess).toBe(true);
    expect(regionRequests).toBe(1);
  });

  it("does not search below the minimum query length", async () => {
    const { result } = renderHook(() => useSearch("a"), { wrapper: makeWrapper() });
    await act(async () => { await Promise.resolve(); });
    expect(result.current.fetchStatus).toBe("idle");
    expect(searchRequests).toBe(0);
  });

  it("keeps search query keys isolated", () => {
    expect(ghanaGeoKeys.search("osu")).not.toEqual(ghanaGeoKeys.search("tema"));
    expect(ghanaGeoKeys.search("osu", "TOWN")).not.toEqual(ghanaGeoKeys.search("osu", "CITY"));
  });

  it("cancels stale searches when the query key changes", async () => {
    let aborted = 0;
    const fetcher: typeof fetch = async (input, init) => {
      const url = new URL(String(input));
      if (url.searchParams.get("q") === "osu") {
        return new Promise<Response>((_resolve, reject) => {
          init?.signal?.addEventListener("abort", () => { aborted += 1; reject(new DOMException("Aborted", "AbortError")); });
        });
      }
      return HttpResponse.json({ data: [], datasetVersion: "2026.08.3-ulid" });
    };
    const { result, rerender } = renderHook(({ query }) => useSearch(query), {
      initialProps: { query: "osu" },
      wrapper: makeWrapper(new GhanaGeoClient({ baseUrl: "https://example.test/v1", fetcher })),
    });
    await waitFor(() => expect(result.current.fetchStatus).toBe("fetching"));
    rerender({ query: "tema" });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(aborted).toBe(1);
  });
});
