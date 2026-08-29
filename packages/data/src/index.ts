import corpus from "./data.json";

export type OfflineRegion = { id: string; name: string; capital: string; code: string; status: string; verificationStatus: string; source: string };
export type OfflineDistrict = { id: string; name: string; regionId: string; regionName: string; type: string; code: string; capital: string; status: string; verificationStatus: string; source: string };
export type OfflinePlace = { id: string; name: string; type: string; regionId: string; regionName: string; districtId: string; districtName: string; latitude: number | null; longitude: number | null; population: number | null; status: string; verificationStatus: string; source: string };

export const datasetVersion = corpus.datasetVersion;
const regions = corpus.regions as OfflineRegion[];
const districts = corpus.districts as OfflineDistrict[];
const places = corpus.places as OfflinePlace[];
const placeById = new Map(places.map((place) => [place.id, place]));

export function getRegions(): readonly OfflineRegion[] { return regions; }
export function getDistricts(regionId?: string): readonly OfflineDistrict[] { return regionId ? districts.filter((district) => district.regionId === regionId) : districts; }
export function getPlace(id: string): OfflinePlace | undefined { return placeById.get(id); }
export function search(query: string, options: { limit?: number; regionId?: string; districtId?: string } = {}): OfflinePlace[] {
  const normalized = query.normalize("NFKD").replace(/\p{M}/gu, "").toLocaleLowerCase("en").trim();
  if (normalized.length < 2) return [];
  return places
    .filter((place) => (!options.regionId || place.regionId === options.regionId) && (!options.districtId || place.districtId === options.districtId))
    .map((place) => ({ place, name: place.name.normalize("NFKD").replace(/\p{M}/gu, "").toLocaleLowerCase("en") }))
    .filter(({ name }) => name.includes(normalized))
    .sort((a, b) => Number(b.name.startsWith(normalized)) - Number(a.name.startsWith(normalized)) || a.name.localeCompare(b.name))
    .slice(0, Math.min(Math.max(options.limit ?? 20, 1), 100))
    .map(({ place }) => place);
}
