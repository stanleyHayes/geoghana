"use client";

import { Suspense, useCallback, useEffect, useMemo, useState } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { Badge, EmptyState, Select, Skeleton } from "@ghanageo/ui";
import {
  ArrowLeft,
  ChevronRight,
  Database,
  Filter,
  Layers3,
  ListTree,
  LocateFixed,
  MapPin,
  PanelRight,
  Search,
  SearchX,
  TriangleAlert,
  X,
} from "lucide-react";
import {
  ExplorerMap,
  type ExplorerMapItem,
  type ExplorerViewport,
} from "@/components/explorer-map";
import { Provenance, VerificationBadge, useApi } from "@/components/data";
import {
  fetchAll,
  getBoundary,
  listDistrictPlaces,
  listDistricts,
  listRegions,
  searchPlaces,
  type BoundaryFeature,
  type District,
  type Place,
  type Region,
} from "@/lib/api";

const GHANA_VIEW: ExplorerViewport = {
  latitude: 7.9465,
  longitude: -1.0232,
  zoom: 7,
};
const PLACE_TYPES = [
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
];

type Feature =
  | (Region & { kind: "region" })
  | (District & { kind: "district" })
  | (Place & { kind: "place" });

function numberParam(value: string | null, fallback: number) {
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : fallback;
}

function ExplorerLoading() {
  return (
    <section
      className="admin-explorer admin-explorer--loading"
      aria-label="Loading location explorer"
    >
      <aside className="admin-explorer__pane">
        <Skeleton />
        <Skeleton />
        <Skeleton />
        <Skeleton />
      </aside>
      <div className="admin-explorer__map-skeleton">
        <Skeleton />
      </div>
      <aside className="admin-explorer__pane">
        <Skeleton />
        <Skeleton />
        <Skeleton />
      </aside>
    </section>
  );
}

function StateNotice({
  kind,
  title,
  detail,
}: {
  kind: "error" | "empty";
  title: string;
  detail: string;
}) {
  if (kind === "empty") {
    return <EmptyState icon={<SearchX />} title={title} description={detail} />;
  }
  const Icon = kind === "error" ? TriangleAlert : SearchX;
  return (
    <div
      className={`admin-explorer__notice admin-explorer__notice--${kind}`}
      role={kind === "error" ? "alert" : "status"}
    >
      <Icon size={18} aria-hidden />
      <div>
        <strong>{title}</strong>
        <p>{detail}</p>
      </div>
    </div>
  );
}

function ExplorerContent() {
  const router = useRouter();
  const pathname = usePathname();
  const params = useSearchParams();
  const [regionId, setRegionId] = useState(params.get("region") ?? "");
  const [districtId, setDistrictId] = useState(params.get("district") ?? "");
  const [placeId, setPlaceId] = useState(params.get("place") ?? "");
  const [query, setQuery] = useState(params.get("q") ?? "");
  const [placeType, setPlaceType] = useState(params.get("type") ?? "");
  const [verification, setVerification] = useState(
    params.get("verification") ?? "",
  );
  const [viewport, setViewport] = useState<ExplorerViewport>({
    latitude: numberParam(params.get("lat"), GHANA_VIEW.latitude),
    longitude: numberParam(params.get("lng"), GHANA_VIEW.longitude),
    zoom: Math.min(
      18,
      Math.max(6, numberParam(params.get("z"), GHANA_VIEW.zoom)),
    ),
  });
  const [mobilePane, setMobilePane] = useState<"hierarchy" | "details" | null>(
    null,
  );

  const regions = useApi(
    (signal) =>
      fetchAll<Region>(
        (cursor, inner) => listRegions({ limit: 100, cursor }, inner),
        signal,
        3,
      ),
    [],
  );
  const districts = useApi(
    (signal) =>
      fetchAll<District>(
        (cursor, inner) => listDistricts({ limit: 100, cursor }, inner),
        signal,
        8,
      ),
    [],
  );
  const places = useApi(
    (signal) =>
      districtId
        ? fetchAll<Place>(
            (cursor, inner) =>
              listDistrictPlaces(
                districtId,
                { limit: 100, cursor, type: placeType || undefined },
                inner,
              ),
            signal,
            25,
          )
        : Promise.resolve({ data: [] as Place[], complete: true }),
    [districtId, placeType],
  );
  const search = useApi(
    (signal) =>
      query.trim().length >= 2
        ? searchPlaces(query.trim(), { limit: 50 }, signal)
        : Promise.resolve(null),
    [query],
  );
  const selectedBoundaryId = placeId || districtId || regionId;
  const boundary = useApi<BoundaryFeature | null>(
    (signal) =>
      selectedBoundaryId
        ? getBoundary(selectedBoundaryId, signal)
        : Promise.resolve(null),
    [selectedBoundaryId],
  );

  const selectedRegion = regions.data?.data.find(
    (item) => item.id === regionId,
  );
  const selectedDistrict = districts.data?.data.find(
    (item) => item.id === districtId,
  );
  const selectedPlace =
    places.data?.data.find((item) => item.id === placeId) ??
    search.data?.data.find((item) => item.id === placeId);
  const selected = selectedPlace
    ? ({ ...selectedPlace, kind: "place" } as Feature)
    : selectedDistrict
      ? ({ ...selectedDistrict, kind: "district" } as Feature)
      : selectedRegion
        ? ({ ...selectedRegion, kind: "region" } as Feature)
        : null;

  const visibleDistricts = useMemo(
    () =>
      (districts.data?.data ?? []).filter(
        (item) => !regionId || item.region?.id === regionId,
      ),
    [districts.data, regionId],
  );
  const visiblePlaces = (places.data?.data ?? []).filter(
    (item) => !verification || item.verificationStatus === verification,
  );
  const searchResults = (search.data?.data ?? []).filter(
    (item) =>
      (!placeType || item.type === placeType) &&
      (!verification || item.verificationStatus === verification),
  );
  const mapFeatures: Feature[] =
    placeId || districtId
      ? visiblePlaces.map((item) => ({ ...item, kind: "place" as const }))
      : regionId
        ? visibleDistricts.map((item) => ({
            ...item,
            kind: "district" as const,
          }))
        : (regions.data?.data ?? []).map((item) => ({
            ...item,
            kind: "region" as const,
          }));
  const mapItems: ExplorerMapItem[] = mapFeatures.flatMap((item) =>
    item.centroid
      ? [
          {
            id: item.id,
            name: item.name,
            kind: item.kind,
            centroid: item.centroid,
          },
        ]
      : [],
  );

  const syncUrl = useCallback(
    (next: {
      region?: string;
      district?: string;
      place?: string;
      q?: string;
      type?: string;
      verification?: string;
      viewport?: ExplorerViewport;
    }) => {
      const url = new URLSearchParams();
      const values = {
        region: next.region ?? regionId,
        district: next.district ?? districtId,
        place: next.place ?? placeId,
        q: next.q ?? query,
        type: next.type ?? placeType,
        verification: next.verification ?? verification,
      };
      for (const [key, value] of Object.entries(values))
        if (value) url.set(key, value);
      const view = next.viewport ?? viewport;
      url.set("lat", view.latitude.toFixed(5));
      url.set("lng", view.longitude.toFixed(5));
      url.set("z", String(view.zoom));
      router.replace(`${pathname}?${url}`, { scroll: false });
    },
    [
      districtId,
      pathname,
      placeId,
      placeType,
      query,
      regionId,
      router,
      verification,
      viewport,
    ],
  );

  useEffect(() => {
    const timer = window.setTimeout(() => syncUrl({}), 250);
    return () => window.clearTimeout(timer);
  }, [
    regionId,
    districtId,
    placeId,
    query,
    placeType,
    verification,
    viewport,
    syncUrl,
  ]);

  const selectRegion = (id: string) => {
    setRegionId(id);
    setDistrictId("");
    setPlaceId("");
    setQuery("");
    setPlaceType("");
    setVerification("");
    setMobilePane(null);
  };
  const selectDistrict = (id: string) => {
    const district = districts.data?.data.find((item) => item.id === id);
    if (district?.region?.id) setRegionId(district.region.id);
    setDistrictId(id);
    setPlaceId("");
    setQuery("");
    setMobilePane(null);
  };
  const selectPlace = (item: Place) => {
    if (item.region?.id) setRegionId(item.region.id);
    if (item.district?.id) setDistrictId(item.district.id);
    setPlaceId(item.id);
    setMobilePane("details");
    if (item.centroid)
      setViewport({
        latitude: item.centroid.latitude,
        longitude: item.centroid.longitude,
        zoom: 14,
      });
  };
  const onMapSelect = (id: string) => {
    const feature = mapFeatures.find((item) => item.id === id);
    if (!feature) return;
    if (feature.kind === "region") selectRegion(feature.id);
    else if (feature.kind === "district") selectDistrict(feature.id);
    else selectPlace(feature);
  };

  const loading = regions.loading || districts.loading;
  const hierarchyError = regions.error ?? districts.error;
  const detailsTitle = selected
    ? `${selected.kind[0]!.toUpperCase()}${selected.kind.slice(1)} record`
    : "Feature details";

  return (
    <section className="admin-explorer" aria-label="Location Explorer">
      <header className="admin-explorer__toolbar">
        <div>
          <span className="admin-explorer__eyebrow">Canonical geography</span>
          <h1>Location Explorer</h1>
        </div>
        <div className="admin-explorer__toolbar-actions">
          <button
            className="gg-button gg-button--ghost gg-button--sm admin-explorer__mobile-trigger"
            type="button"
            onClick={() => setMobilePane("hierarchy")}
          >
            <ListTree size={16} aria-hidden /> Browse
          </button>
          <button
            className="gg-button gg-button--ghost gg-button--sm admin-explorer__mobile-trigger"
            type="button"
            onClick={() => setMobilePane("details")}
            disabled={!selected}
          >
            <PanelRight size={16} aria-hidden /> Details
          </button>
          <span className="admin-explorer__live">
            <i aria-hidden /> Public API
          </span>
        </div>
      </header>

      <div className="admin-explorer__mobile-context">
        <span>{selectedRegion?.name ?? "All Ghana"}</span>
        <ChevronRight size={13} aria-hidden />
        <span>{selectedDistrict?.name ?? "All districts"}</span>
      </div>

      <div className="admin-explorer__workspace">
        <aside
          className={`admin-explorer__pane admin-explorer__hierarchy ${mobilePane === "hierarchy" ? "is-open" : ""}`}
          aria-label="Geography hierarchy"
        >
          <div className="admin-explorer__pane-head">
            <div>
              <span>01</span>
              <h2>Browse hierarchy</h2>
            </div>
            <button
              className="admin-explorer__drawer-close"
              type="button"
              onClick={() => setMobilePane(null)}
              aria-label="Close hierarchy"
            >
              <X size={18} />
            </button>
          </div>
          <label className="admin-explorer__search">
            <Search size={16} aria-hidden />
            <input
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder="Search every place…"
              aria-label="Search every place"
            />
          </label>
          <div
            className="admin-explorer__filters"
            aria-label="Explorer filters"
          >
            <div>
              <span>Type</span>
              <Select
                value={placeType || "all"}
                onValueChange={(value) => {
                  setPlaceType(value === "all" ? "" : value);
                  setPlaceId("");
                }}
                aria-label="Filter by place type"
                options={[
                  { value: "all", label: "All types" },
                  ...PLACE_TYPES.map((type) => ({
                    value: type,
                    label: type.replaceAll("_", " ").toLowerCase(),
                  })),
                ]}
              />
            </div>
            <div>
              <span>Verification</span>
              <Select
                value={verification || "all"}
                onValueChange={(value) => {
                  setVerification(value === "all" ? "" : value);
                  setPlaceId("");
                }}
                aria-label="Filter by verification"
                options={[
                  { value: "all", label: "Any status" },
                  { value: "REFERENCE", label: "Reference" },
                  {
                    value: "SEED_NEEDS_CANONICAL_RECONCILIATION",
                    label: "Needs reconciliation",
                  },
                  { value: "REVIEWED", label: "Reviewed" },
                  { value: "CANONICAL", label: "Canonical" },
                ]}
              />
            </div>
          </div>

          {query.trim().length >= 2 ? (
            <div className="admin-explorer__list-block">
              <div className="admin-explorer__list-label">
                <Search size={13} /> Search results{" "}
                <span>{searchResults.length}</span>
              </div>
              {search.loading ? (
                <ExplorerListSkeleton />
              ) : search.error ? (
                <StateNotice
                  kind="error"
                  title="Search unavailable"
                  detail={search.error.message}
                />
              ) : searchResults.length ? (
                <ul className="admin-explorer__list">
                  {searchResults.map((item) => (
                    <li key={item.id}>
                      <button
                        type="button"
                        className={placeId === item.id ? "is-selected" : ""}
                        onClick={() => selectPlace(item)}
                      >
                        <MapPin size={14} />
                        <span>
                          <strong>{item.name}</strong>
                          <small>
                            {[item.district?.name, item.region?.name]
                              .filter(Boolean)
                              .join(" · ") || item.type}
                          </small>
                        </span>
                        <ChevronRight size={14} />
                      </button>
                    </li>
                  ))}
                </ul>
              ) : (
                <StateNotice
                  kind="empty"
                  title="No matching places"
                  detail={`No canonical record matches “${query.trim()}”.`}
                />
              )}
            </div>
          ) : hierarchyError ? (
            <StateNotice
              kind="error"
              title="Hierarchy unavailable"
              detail={hierarchyError.message}
            />
          ) : loading ? (
            <ExplorerListSkeleton />
          ) : (
            <>
              <div className="admin-explorer__list-block">
                <div className="admin-explorer__list-label">
                  <span>Regions</span>
                  <span>{regions.data?.data.length ?? 0}</span>
                </div>
                <ul className="admin-explorer__list">
                  {(regions.data?.data ?? []).map((item) => (
                    <li key={item.id}>
                      <button
                        type="button"
                        className={regionId === item.id ? "is-selected" : ""}
                        onClick={() => selectRegion(item.id)}
                      >
                        <span>
                          <strong>{item.name}</strong>
                          <small>
                            {item.capital ?? "Capital not recorded"}
                          </small>
                        </span>
                        <ChevronRight size={14} />
                      </button>
                    </li>
                  ))}
                </ul>
              </div>
              {regionId ? (
                <div className="admin-explorer__list-block">
                  <div className="admin-explorer__list-label">
                    <span>Districts</span>
                    <span>{visibleDistricts.length}</span>
                  </div>
                  {visibleDistricts.length ? (
                    <ul className="admin-explorer__list">
                      {visibleDistricts.map((item) => (
                        <li key={item.id}>
                          <button
                            type="button"
                            className={
                              districtId === item.id ? "is-selected" : ""
                            }
                            onClick={() => selectDistrict(item.id)}
                          >
                            <span>
                              <strong>{item.name}</strong>
                              <small>
                                {item.type === "UNSPECIFIED"
                                  ? "Type not recorded"
                                  : item.type}
                              </small>
                            </span>
                            <ChevronRight size={14} />
                          </button>
                        </li>
                      ))}
                    </ul>
                  ) : (
                    <StateNotice
                      kind="empty"
                      title="No districts returned"
                      detail="The selected region has no district records in this dataset."
                    />
                  )}
                </div>
              ) : null}
              {districtId ? (
                <div className="admin-explorer__list-block">
                  <div className="admin-explorer__list-label">
                    <span>Places</span>
                    <span>{visiblePlaces.length}</span>
                  </div>
                  <div className="admin-explorer__filter-summary">
                    <Filter size={13} />
                    {[
                      placeType && placeType.toLowerCase(),
                      verification &&
                        verification.replaceAll("_", " ").toLowerCase(),
                    ]
                      .filter(Boolean)
                      .join(" · ") || "All place records"}
                  </div>
                  {places.loading ? (
                    <ExplorerListSkeleton />
                  ) : places.error ? (
                    <StateNotice
                      kind="error"
                      title="Places unavailable"
                      detail={places.error.message}
                    />
                  ) : visiblePlaces.length ? (
                    <ul className="admin-explorer__list">
                      {visiblePlaces.map((item) => (
                        <li key={item.id}>
                          <button
                            type="button"
                            className={placeId === item.id ? "is-selected" : ""}
                            onClick={() => selectPlace(item)}
                          >
                            <MapPin size={13} />
                            <span>
                              <strong>{item.name}</strong>
                              <small>{item.type.toLowerCase()}</small>
                            </span>
                            <ChevronRight size={14} />
                          </button>
                        </li>
                      ))}
                    </ul>
                  ) : (
                    <StateNotice
                      kind="empty"
                      title="No places returned"
                      detail="No canonical places match this district and type filter."
                    />
                  )}
                  {!places.data?.complete ? (
                    <p className="admin-explorer__truncated">
                      The public cursor limit was reached; this branch is
                      incomplete.
                    </p>
                  ) : null}
                </div>
              ) : null}
            </>
          )}
        </aside>

        <div className="admin-explorer__map-pane">
          <ExplorerMap
            items={mapItems}
            boundary={boundary.data}
            selectedId={selected?.id}
            viewport={viewport}
            onSelect={onMapSelect}
            onViewportChange={setViewport}
          />
          <div className="admin-explorer__map-overlay admin-explorer__map-overlay--top">
            <Layers3 size={15} aria-hidden />
            <span>
              {mapItems.length} mapped{" "}
              {mapItems.length === 1 ? "feature" : "features"}
            </span>
            {mapFeatures.length > mapItems.length ? (
              <small>
                {mapFeatures.length - mapItems.length} without centroids
              </small>
            ) : null}
          </div>
          <button
            className="admin-explorer__reset"
            type="button"
            onClick={() => setViewport(GHANA_VIEW)}
          >
            <LocateFixed size={15} /> Ghana view
          </button>
          {boundary.loading ? (
            <div className="admin-explorer__boundary-state">
              <Skeleton />
            </div>
          ) : boundary.error && selected ? (
            <div className="admin-explorer__boundary-state">
              <span>Boundary unavailable</span>
              <small>{boundary.error.message}</small>
            </div>
          ) : null}
          {mapItems.length === 0 && !loading && !places.loading ? (
            <div className="admin-explorer__map-empty">
              <MapPin size={22} />
              <strong>No mapped centroids</strong>
              <span>Choose another level or inspect the record metadata.</span>
            </div>
          ) : null}
        </div>

        <aside
          className={`admin-explorer__pane admin-explorer__details ${mobilePane === "details" ? "is-open" : ""}`}
          aria-label="Selected feature details"
        >
          <div className="admin-explorer__pane-head">
            <div>
              <span>03</span>
              <h2>{detailsTitle}</h2>
            </div>
            <button
              className="admin-explorer__drawer-close"
              type="button"
              onClick={() => setMobilePane(null)}
              aria-label="Close details"
            >
              <X size={18} />
            </button>
          </div>
          {selected ? (
            <FeatureDetails
              feature={selected}
              boundary={boundary.data}
              onBack={() => {
                if (placeId) setPlaceId("");
                else if (districtId) setDistrictId("");
                else setRegionId("");
              }}
            />
          ) : (
            <div className="admin-explorer__detail-empty">
              <Database size={24} />
              <h3>Select a map feature</h3>
              <p>
                Choose a region, district or place to inspect its canonical
                metadata, verification state and source.
              </p>
            </div>
          )}
        </aside>
      </div>
      {mobilePane ? (
        <button
          className="admin-explorer__scrim"
          type="button"
          onClick={() => setMobilePane(null)}
          aria-label="Close open drawer"
        />
      ) : null}
    </section>
  );
}

function ExplorerListSkeleton() {
  return (
    <div
      className="admin-explorer__list-skeleton"
      role="status"
      aria-label="Loading records"
    >
      {[0, 1, 2, 3, 4].map((item) => (
        <div key={item}>
          <Skeleton />
          <Skeleton />
        </div>
      ))}
    </div>
  );
}

function FeatureDetails({
  feature,
  boundary,
  onBack,
}: {
  feature: Feature;
  boundary: BoundaryFeature | null;
  onBack: () => void;
}) {
  const parent =
    feature.kind === "place"
      ? (feature.district?.name ?? feature.region?.name)
      : feature.kind === "district"
        ? feature.region?.name
        : "Ghana";
  return (
    <div className="admin-explorer__detail-body">
      <button className="admin-explorer__back" type="button" onClick={onBack}>
        <ArrowLeft size={14} /> Back one level
      </button>
      <div className="admin-explorer__feature-title">
        <span className="admin-explorer__pin">
          <MapPin size={17} />
        </span>
        <div>
          <span>{feature.kind}</span>
          <h3>{feature.name}</h3>
          <p>{parent}</p>
        </div>
      </div>
      <div className="admin-explorer__badges">
        <VerificationBadge status={feature.verificationStatus} />
        <Badge>{feature.status}</Badge>
      </div>
      <dl className="admin-explorer__metadata">
        <div>
          <dt>Canonical ID</dt>
          <dd>
            <code>{feature.id}</code>
          </dd>
        </div>
        <div>
          <dt>Kind</dt>
          <dd>{feature.kind}</dd>
        </div>
        {"type" in feature ? (
          <div>
            <dt>Type</dt>
            <dd>
              {feature.type === "UNSPECIFIED"
                ? "Not recorded"
                : feature.type.replaceAll("_", " ").toLowerCase()}
            </dd>
          </div>
        ) : null}
        {feature.centroid ? (
          <div>
            <dt>Centroid</dt>
            <dd>
              <code>
                {feature.centroid.latitude.toFixed(5)},{" "}
                {feature.centroid.longitude.toFixed(5)}
              </code>
            </dd>
          </div>
        ) : (
          <div>
            <dt>Centroid</dt>
            <dd>Not recorded</dd>
          </div>
        )}
        {feature.kind === "place" && feature.population ? (
          <div>
            <dt>Population</dt>
            <dd>{feature.population.toLocaleString("en-GH")}</dd>
          </div>
        ) : null}
        <div>
          <dt>Boundary</dt>
          <dd>
            {boundary ? `${boundary.geometry.type} available` : "Not available"}
          </dd>
        </div>
        {boundary?.properties.datasetVersion ? (
          <div>
            <dt>Boundary dataset</dt>
            <dd>
              <code>{boundary.properties.datasetVersion}</code>
            </dd>
          </div>
        ) : null}
        <div>
          <dt>Record source</dt>
          <dd>
            <Provenance
              source={feature.provenance?.sourceId}
              url={feature.provenance?.sourceUrl}
            />
          </dd>
        </div>
        {feature.provenance?.retrievedAt ? (
          <div>
            <dt>Retrieved</dt>
            <dd>
              {new Date(feature.provenance.retrievedAt).toLocaleDateString(
                "en-GH",
                { dateStyle: "medium" },
              )}
            </dd>
          </div>
        ) : null}
      </dl>
      {boundary?.properties.attribution ? (
        <div className="admin-explorer__attribution">
          <strong>Geometry attribution</strong>
          <p>{boundary.properties.attribution}</p>
        </div>
      ) : null}
      {feature.kind === "place" && feature.aliases?.length ? (
        <div className="admin-explorer__aliases">
          <strong>Known aliases</strong>
          <div>
            {feature.aliases.map((alias) => (
              <Badge key={`${alias.type}-${alias.value}`}>{alias.value}</Badge>
            ))}
          </div>
        </div>
      ) : null}
    </div>
  );
}

export default function ExplorerScreen() {
  return (
    <Suspense fallback={<ExplorerLoading />}>
      <ExplorerContent />
    </Suspense>
  );
}
