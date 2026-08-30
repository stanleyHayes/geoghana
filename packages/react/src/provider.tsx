import { createContext, useContext, useMemo, type PropsWithChildren } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { GhanaGeoClient, type GhanaGeoClientOptions } from "@ghanageo/client";

const ClientContext = createContext<GhanaGeoClient | null>(null);

export type GhanaGeoProviderProps = PropsWithChildren<{
  client?: GhanaGeoClient;
  options?: GhanaGeoClientOptions;
  queryClient?: QueryClient;
}>;

export function GhanaGeoProvider({ children, client, options, queryClient }: GhanaGeoProviderProps) {
  const resolvedClient = useMemo(() => client ?? new GhanaGeoClient(options), [client, options?.baseUrl, options?.apiKey, options?.fetcher, options?.retry, options?.telemetry]);
  const resolvedQueryClient = useMemo(() => queryClient ?? new QueryClient({
    defaultOptions: { queries: { staleTime: 5 * 60_000, retry: false, refetchOnWindowFocus: false } },
  }), [queryClient]);

  return (
    <QueryClientProvider client={resolvedQueryClient}>
      <ClientContext.Provider value={resolvedClient}>{children}</ClientContext.Provider>
    </QueryClientProvider>
  );
}

export function useGhanaGeoClient() {
  const client = useContext(ClientContext);
  if (!client) throw new Error("useGhanaGeoClient must be used inside GhanaGeoProvider");
  return client;
}
