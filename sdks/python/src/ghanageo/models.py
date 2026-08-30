"""Hand-mapped Pydantic v2 models for ``contracts/openapi/v1.yaml``."""

from __future__ import annotations

from typing import Any, Literal

from pydantic import BaseModel, ConfigDict

Status = Literal["ACTIVE", "DEPRECATED", "MERGED"]
VerificationStatus = Literal["REFERENCE", "SEED_NEEDS_CANONICAL_RECONCILIATION", "REVIEWED", "CANONICAL"]
PlaceType = Literal[
    "CITY",
    "TOWN",
    "VILLAGE",
    "COMMUNITY",
    "SUBURB",
    "NEIGHBOURHOOD",
    "HAMLET",
    "SETTLEMENT",
    "LOCALITY",
    "REGIONAL_CAPITAL",
]
DatasetStatus = Literal["draft", "validation", "review", "approved", "published", "rolled_back"]


def _camel(value: str) -> str:
    head, *tail = value.split("_")
    return head + "".join(part.capitalize() for part in tail)


class GhanaGeoModel(BaseModel):
    model_config = ConfigDict(alias_generator=_camel, populate_by_name=True, extra="allow")


class Coordinate(GhanaGeoModel):
    latitude: float
    longitude: float


class Ref(GhanaGeoModel):
    id: str
    name: str


class Provenance(GhanaGeoModel):
    source_id: str
    source_url: str | None = None
    retrieved_at: str | None = None


class Alias(GhanaGeoModel):
    value: str
    type: str | None = None
    language: str | None = None
    is_preferred: bool | None = None


class Region(GhanaGeoModel):
    id: str
    country_code: str
    name: str
    capital: str | None = None
    code: str | None = None
    status: Status
    verification_status: VerificationStatus
    centroid: Coordinate | None = None
    provenance: Provenance
    dataset_version: str


class District(GhanaGeoModel):
    id: str
    name: str
    code: str | None = None
    type: str | None = None
    capital: str | None = None
    region: Ref
    status: Status
    verification_status: VerificationStatus
    centroid: Coordinate | None = None
    provenance: Provenance
    dataset_version: str


class Place(GhanaGeoModel):
    id: str
    name: str
    normalized_name: str
    type: PlaceType
    region: Ref | None = None
    district: Ref | None = None
    parent_place_id: str | None = None
    aliases: list[Alias]
    centroid: Coordinate | None = None
    population: int | None = None
    status: Status
    verification_status: VerificationStatus
    provenance: Provenance
    dataset_version: str


class SearchResult(Place):
    score: float | None = None
    match_reason: str | None = None


class Page[T](GhanaGeoModel):
    data: list[T]
    dataset_version: str
    next_cursor: str | None = None


class Geometry(GhanaGeoModel):
    type: Literal["Point", "LineString", "Polygon", "MultiPoint", "MultiLineString", "MultiPolygon"]
    coordinates: list[Any]


class BoundaryProperties(GhanaGeoModel):
    id: str | None = None
    name: str | None = None
    dataset_version: str | None = None
    attribution: str | None = None


class BoundaryFeature(GhanaGeoModel):
    type: Literal["Feature"]
    geometry: Geometry
    properties: BoundaryProperties


class ReverseResult(GhanaGeoModel):
    region: Ref | None = None
    district: Ref | None = None
    nearby: list[Place] | None = None
    dataset_version: str


class Dataset(GhanaGeoModel):
    version: str
    status: DatasetStatus
    published_at: str | None = None
    changelog: str | None = None
    checksum: str | None = None


class Download(GhanaGeoModel):
    format: Literal["json", "csv", "geojson", "parquet"]
    url: str
    checksum: str
    size_bytes: int | None = None
    attribution: str | None = None


class DownloadList(GhanaGeoModel):
    version: str
    downloads: list[Download]


RegionPage = Page[Region]
DistrictPage = Page[District]
PlacePage = Page[Place]
SearchPage = Page[SearchResult]


class DatasetPage(GhanaGeoModel):
    data: list[Dataset]
