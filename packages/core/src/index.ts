export type { components, operations, paths } from "./openapi.gen";

import type { components } from "./openapi.gen";

export type GhanaGeoErrorBody = components["schemas"]["Error"];
export type Region = components["schemas"]["Region"];
export type District = components["schemas"]["District"];
export type Place = components["schemas"]["Place"];
export type SearchResult = components["schemas"]["SearchResult"];
export type RegionPage = components["schemas"]["RegionPage"];
export type DistrictPage = components["schemas"]["DistrictPage"];
export type PlacePage = components["schemas"]["PlacePage"];
export type SearchPage = components["schemas"]["SearchPage"];
export type ReverseResult = components["schemas"]["ReverseResult"];
export type DatasetPage = components["schemas"]["DatasetPage"];
export type DownloadList = components["schemas"]["DownloadList"];
export type BoundaryFeature = components["schemas"]["BoundaryFeature"];
export type DatasetVersion = components["schemas"]["DatasetVersion"];

export const API_VERSION = "v1" as const;
export const TESTED_DATASET_VERSION = "2026.08.3-ulid" as const;
/** @deprecated Prefer TESTED_DATASET_VERSION; this value is compatibility evidence, not a live response pin. */
export const DATASET_VERSION = TESTED_DATASET_VERSION;
export const DEFAULT_API_URL = "https://api.geo.digitalghana.dev/v1" as const;
