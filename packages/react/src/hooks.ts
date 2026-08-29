import { useQuery, type UseQueryOptions } from "@tanstack/react-query";
import { useGhanaGeoClient } from "./provider";
import type { District, Page, Place, Region, SearchResult } from "./types";

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
  return useQuery({ queryKey: ghanaGeoKeys.regions(), queryFn: ({ signal }) => client.get<Page<Region>>("/regions", undefined, signal) });
}

export function useRegion(id?: string) {
  const client = useGhanaGeoClient();
  return useQuery({ queryKey: ghanaGeoKeys.region(id ?? ""), enabled: !!id, queryFn: ({ signal }) => client.get<Region>(`/regions/${encodeURIComponent(id!)}`, undefined, signal) });
}

export function useDistricts(params: { regionId?: string; first?: number } = {}) {
  const client = useGhanaGeoClient();
  return useQuery({ queryKey: ghanaGeoKeys.districts(params.regionId), queryFn: ({ signal }) => client.get<Page<District>>("/districts", { regionId: params.regionId, first: params.first }, signal) });
}

export function useDistrict(id?: string) {
  const client = useGhanaGeoClient();
  return useQuery({ queryKey: ghanaGeoKeys.district(id ?? ""), enabled: !!id, queryFn: ({ signal }) => client.get<District>(`/districts/${encodeURIComponent(id!)}`, undefined, signal) });
}

export function usePlace(id?: string) {
  const client = useGhanaGeoClient();
  return useQuery({ queryKey: ghanaGeoKeys.place(id ?? ""), enabled: !!id, queryFn: ({ signal }) => client.get<Place>(`/places/${encodeURIComponent(id!)}`, undefined, signal) });
}

export function useSearch(query: string, options: { type?: string; first?: number; enabled?: boolean } = {}) {
  const client = useGhanaGeoClient();
  const q = query.trim();
  return useQuery({
    queryKey: ghanaGeoKeys.search(q, options.type),
    enabled: (options.enabled ?? true) && q.length >= 2,
    queryFn: ({ signal }) => client.get<Page<SearchResult>>("/search", { q, type: options.type, first: options.first }, signal),
  });
}

export function useAutocomplete(query: string, options: { first?: number; enabled?: boolean } = {}) {
  const client = useGhanaGeoClient();
  const q = query.trim();
  return useQuery({
    queryKey: ghanaGeoKeys.autocomplete(q),
    enabled: (options.enabled ?? true) && q.length >= 2,
    staleTime: 30 * 60_000,
    queryFn: ({ signal }) => client.get<SearchResult[]>("/autocomplete", { q, first: options.first ?? 10 }, signal),
  });
}

export function useGeocode(query: string, options: { first?: number; enabled?: boolean } = {}) {
  const client = useGhanaGeoClient();
  const q = query.trim();
  return useQuery({
    queryKey: [...ghanaGeoKeys.all, "geocode", q, options.first] as const,
    enabled: (options.enabled ?? true) && q.length >= 2,
    queryFn: ({ signal }) => client.get<SearchResult[]>("/geocode", { q, first: options.first ?? 10 }, signal),
  });
}

export function useReverseGeocode(latitude?: number, longitude?: number, options: { enabled?: boolean } = {}) {
  const client = useGhanaGeoClient();
  const valid = Number.isFinite(latitude) && Number.isFinite(longitude);
  return useQuery({
    queryKey: ghanaGeoKeys.reverse(latitude ?? 0, longitude ?? 0),
    enabled: (options.enabled ?? true) && valid,
    queryFn: ({ signal }) => client.get<unknown>("/reverse", { lat: latitude!, lng: longitude! }, signal),
  });
}

export function useNearby(latitude?: number, longitude?: number, radiusMeters = 5000, options: { enabled?: boolean } = {}) {
  const client = useGhanaGeoClient();
  const valid = Number.isFinite(latitude) && Number.isFinite(longitude) && radiusMeters > 0;
  return useQuery({
    queryKey: ghanaGeoKeys.nearby(latitude ?? 0, longitude ?? 0, radiusMeters),
    enabled: (options.enabled ?? true) && valid,
    queryFn: ({ signal }) => client.get<SearchResult[]>("/nearby", { lat: latitude!, lng: longitude!, radiusMeters }, signal),
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
