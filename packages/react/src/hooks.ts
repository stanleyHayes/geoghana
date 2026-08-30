import { useQuery, type UseQueryOptions } from "@tanstack/react-query";
import { useGhanaGeoClient } from "./provider";
import type { Place, PlacePage, Region, RegionPage, ReverseResult, SearchPage } from "@ghanageo/client";

export const ghanaGeoKeys = {
  all: ["ghanageo"] as const,
  regions: () => [...ghanaGeoKeys.all, "regions"] as const,
  region: (id: string) => [...ghanaGeoKeys.regions(), id] as const,
  districts: (regionId?: string) => [...ghanaGeoKeys.all, "districts", { regionId }] as const,
  district: (id: string) => [...ghanaGeoKeys.all, "district", id] as const,
  place: (id: string) => [...ghanaGeoKeys.all, "place", id] as const,
  search: (q: string, type?: string) => [...ghanaGeoKeys.all, "search", { q, type }] as const,
  autocomplete: (q: string) => [...ghanaGeoKeys.all, "autocomplete", q] as const,
  reverse: (lat: number, lng: number) => [...ghanaGeoKeys.all, "reverse", lat, lng] as const,
  nearby: (lat: number, lng: number, radiusMeters: number) => [...ghanaGeoKeys.all, "nearby", lat, lng, radiusMeters] as const,
};

export function useRegions() {
  const client = useGhanaGeoClient();
  return useQuery({ queryKey: ghanaGeoKeys.regions(), queryFn: ({ signal }) => client.regions({}, signal) });
}

export function useRegion(id?: string) {
  const client = useGhanaGeoClient();
  return useQuery({ queryKey: ghanaGeoKeys.region(id ?? ""), enabled: !!id, queryFn: ({ signal }) => client.region(id!, signal) });
}

export function useDistricts(params: { regionId?: string; first?: number } = {}) {
  const client = useGhanaGeoClient();
  return useQuery({ queryKey: ghanaGeoKeys.districts(params.regionId), queryFn: ({ signal }) => client.districts({ ...(params.regionId ? { regionId: params.regionId } : {}), ...(params.first ? { limit: params.first } : {}) }, signal) });
}

export function useDistrict(id?: string) {
  const client = useGhanaGeoClient();
  return useQuery({ queryKey: ghanaGeoKeys.district(id ?? ""), enabled: !!id, queryFn: ({ signal }) => client.district(id!, signal) });
}

export function usePlace(id?: string) {
  const client = useGhanaGeoClient();
  return useQuery({ queryKey: ghanaGeoKeys.place(id ?? ""), enabled: !!id, queryFn: ({ signal }) => client.place(id!, signal) });
}

export function useSearch(query: string, options: { type?: string; first?: number; enabled?: boolean } = {}) {
  const client = useGhanaGeoClient();
  const q = query.trim();
  return useQuery({
    queryKey: ghanaGeoKeys.search(q, options.type),
    enabled: (options.enabled ?? true) && q.length >= 2,
    queryFn: ({ signal }) => client.search(q, { ...(options.type ? { type: options.type } : {}), ...(options.first ? { limit: options.first } : {}) }, signal),
  });
}

export function useAutocomplete(query: string, options: { first?: number; enabled?: boolean } = {}) {
  const client = useGhanaGeoClient();
  const q = query.trim();
  return useQuery({
    queryKey: ghanaGeoKeys.autocomplete(q),
    enabled: (options.enabled ?? true) && q.length >= 2,
    staleTime: 30 * 60_000,
    queryFn: ({ signal }) => client.autocomplete(q, options.first ?? 10, signal),
  });
}

export function useGeocode(query: string, options: { first?: number; enabled?: boolean } = {}) {
  const client = useGhanaGeoClient();
  const q = query.trim();
  return useQuery({
    queryKey: [...ghanaGeoKeys.all, "geocode", q, options.first] as const,
    enabled: (options.enabled ?? true) && q.length >= 2,
    queryFn: ({ signal }) => client.geocode(q, options.first ?? 10, signal),
  });
}

export function useReverseGeocode(latitude?: number, longitude?: number, options: { enabled?: boolean } = {}) {
  const client = useGhanaGeoClient();
  const valid = Number.isFinite(latitude) && Number.isFinite(longitude);
  return useQuery({
    queryKey: ghanaGeoKeys.reverse(latitude ?? 0, longitude ?? 0),
    enabled: (options.enabled ?? true) && valid,
    queryFn: ({ signal }) => client.reverse(latitude!, longitude!, signal),
  });
}

export function useNearby(latitude?: number, longitude?: number, radiusMeters = 5000, options: { enabled?: boolean } = {}) {
  const client = useGhanaGeoClient();
  const valid = Number.isFinite(latitude) && Number.isFinite(longitude) && radiusMeters > 0;
  return useQuery({
    queryKey: ghanaGeoKeys.nearby(latitude ?? 0, longitude ?? 0, radiusMeters),
    enabled: (options.enabled ?? true) && valid,
    queryFn: ({ signal }) => client.nearby(latitude!, longitude!, radiusMeters, 20, signal),
  });
}

export function useGhanaGeoGraphQL<TData, TVariables extends Record<string, unknown> = Record<string, unknown>>(
  queryKey: readonly unknown[],
  query: string,
  variables?: TVariables,
  options?: Omit<UseQueryOptions<TData>, "queryKey" | "queryFn">,
) {
  const client = useGhanaGeoClient();
  return useQuery<TData>({
    ...options,
    queryKey: [...ghanaGeoKeys.all, "graphql", ...queryKey],
    queryFn: ({ signal }) => client.graphql<TData, TVariables>(query, variables, signal),
  });
}
