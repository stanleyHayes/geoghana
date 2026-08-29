export type Coordinate = { latitude: number; longitude: number };

export type Region = {
  id: string;
  code?: string | null;
  name: string;
  capital?: string | null;
  centroid?: Coordinate | null;
  datasetVersion?: string;
};

export type District = {
  id: string;
  code?: string | null;
  name: string;
  regionId: string;
  region?: Region;
  capital?: string | null;
  centroid?: Coordinate | null;
  datasetVersion?: string;
};

export type Place = {
  id: string;
  name: string;
  normalizedName?: string;
  type: string;
  region?: Region;
  district?: District | null;
  centroid?: Coordinate | null;
  aliases?: string[];
  datasetVersion?: string;
};

export type SearchResult = Place & {
  score?: number;
  matchReason?: string;
};

export type Page<T> = {
  data: T[];
  nextCursor?: string | null;
  datasetVersion?: string;
};
