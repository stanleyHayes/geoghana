"use client";

import { HydrationBoundary, QueryClient, QueryClientProvider, type DehydratedState } from "@tanstack/react-query";
import { useState, type ReactNode } from "react";
import { GhanaGeoClient, GhanaGeoProvider } from "@ghanageo/react";

export function Providers({ children, state }: { children: ReactNode; state: DehydratedState }) {
  const [queryClient] = useState(() => new QueryClient());
  const [geoClient] = useState(() => new GhanaGeoClient());

  return (
    <QueryClientProvider client={queryClient}>
      <HydrationBoundary state={state}>
        <GhanaGeoProvider client={geoClient} queryClient={queryClient}>
          {children}
        </GhanaGeoProvider>
      </HydrationBoundary>
    </QueryClientProvider>
  );
}
