// @vitest-environment node
import { dehydrate, HydrationBoundary, QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderToString } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { GhanaGeoClient, GhanaGeoProvider, ghanaGeoKeys } from "../src";

describe("Next.js 16 hydration pattern", () => {
  it("renders a dehydrated server cache without browser globals", async () => {
    const queryClient = new QueryClient();
    queryClient.setQueryData(ghanaGeoKeys.regions(), {
      data: [{ id: "01KDVDNA00N6BFFK8VF5K8YXPW", name: "Ahafo" }],
      datasetVersion: "2026.08.3-ulid",
    });

    const html = renderToString(
      <QueryClientProvider client={queryClient}>
        <HydrationBoundary state={dehydrate(queryClient)}>
          <GhanaGeoProvider client={new GhanaGeoClient()} queryClient={queryClient}>
            <span>hydrated</span>
          </GhanaGeoProvider>
        </HydrationBoundary>
      </QueryClientProvider>,
    );

    expect(html).toContain("hydrated");
  });
});
