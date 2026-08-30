"use client";

import { useState } from "react";
import { Badge, Card } from "@ghanageo/ui";
import { Copy, Download } from "lucide-react";
import { AsyncState, useApi } from "@/components/data";
import { PageHeader } from "@/components/screen";
import {
  TableActionButton,
  TableActionLink,
  TableActions,
} from "@/components/table-actions";
import {
  OSMCreatePanel,
  OSMDeprecateButton,
} from "@/components/geography-admin-controls";
import {
  listDatasets,
  listPointsOfInterest,
  listRoads,
  type DatasetVersion,
  type OSMPage,
  type PointOfInterest,
  type Road,
} from "@/lib/api";

const PAGE_SIZE = 10;

function Pager({
  page,
  pages,
  onPage,
}: {
  page: number;
  pages: number;
  onPage: (page: number) => void;
}) {
  if (pages <= 1) return null;
  return (
    <nav className="admin-pager" aria-label="Table pagination">
      <button
        className="gg-button gg-button--ghost gg-button--sm"
        disabled={page === 1}
        onClick={() => onPage(page - 1)}
      >
        Previous
      </button>
      <span>
        Page {page} of {pages}
      </span>
      <button
        className="gg-button gg-button--ghost gg-button--sm"
        disabled={page === pages}
        onClick={() => onPage(page + 1)}
      >
        Next
      </button>
    </nav>
  );
}

function OSMCatalogue<
  T extends {
    id: string;
    name: string;
    class: string;
    region?: { id: string; name?: string };
  },
>({
  kind,
  mutationKind,
  load,
  extra,
}: {
  kind: "Roads" | "Points of interest";
  mutationKind: "roads" | "pois";
  load: (signal: AbortSignal) => Promise<OSMPage<T>>;
  extra: (item: T) => string;
}) {
  const [refreshKey, setRefreshKey] = useState(0);
  const state = useApi(load, [kind, refreshKey]);
  const [page, setPage] = useState(1);
  return (
    <>
      <PageHeader
        eyebrow="Live geography catalogue"
        title={kind}
        lede="Live published records with V1 create and deprecate controls. Editing existing records remains unavailable because no update contract is published."
        actions={
          <OSMCreatePanel
            kind={mutationKind}
            onCreated={() => setRefreshKey((v) => v + 1)}
          />
        }
      />
      <AsyncState
        state={state}
        empty={`No ${kind.toLowerCase()} were returned by the API.`}
      >
        {(result) => {
          const pages = Math.max(1, Math.ceil(result.data.length / PAGE_SIZE));
          const current = Math.min(page, pages);
          const rows = result.data.slice(
            (current - 1) * PAGE_SIZE,
            current * PAGE_SIZE,
          );
          return (
            <>
              <div className="admin-live-note">
                <Badge tone="canonical">Live API</Badge>
                <span>
                  {result.data.length} records in this response ·{" "}
                  {result.license}
                </span>
              </div>
              <Card style={{ padding: 0, overflow: "hidden" }}>
                <div style={{ overflowX: "auto" }}>
                  <table
                    className="gg-table"
                    style={{ width: "100%", minWidth: 640 }}
                  >
                    <thead>
                      <tr>
                        <th>Name</th>
                        <th>Class</th>
                        <th>Reference / category</th>
                        <th>Region</th>
                        <th>
                          <span className="sr-only">Lifecycle action</span>
                        </th>
                      </tr>
                    </thead>
                    <tbody>
                      {rows.map((item) => (
                        <tr key={item.id}>
                          <td>
                            <strong>{item.name || "Unnamed"}</strong>
                            <small>{item.id}</small>
                          </td>
                          <td>{item.class || "—"}</td>
                          <td>{extra(item) || "—"}</td>
                          <td>{item.region?.name || item.region?.id || "—"}</td>
                          <td>
                            <OSMDeprecateButton
                              kind={mutationKind}
                              id={item.id}
                              onChanged={() => setRefreshKey((v) => v + 1)}
                            />
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </Card>
              <Pager page={current} pages={pages} onPage={setPage} />
              <p className="admin-attribution">{result.attribution}</p>
            </>
          );
        }}
      </AsyncState>
    </>
  );
}

export function RoadsCatalogue() {
  return (
    <OSMCatalogue<Road>
      kind="Roads"
      mutationKind="roads"
      load={(signal) => listRoads({ limit: 100 }, signal)}
      extra={(road) => road.ref ?? ""}
    />
  );
}
export function POICatalogue() {
  return (
    <OSMCatalogue<PointOfInterest>
      kind="Points of interest"
      mutationKind="pois"
      load={(signal) => listPointsOfInterest({ limit: 100 }, signal)}
      extra={(poi) => poi.category ?? ""}
    />
  );
}

export function DatasetCatalogue({
  downloadsOnly = false,
}: {
  downloadsOnly?: boolean;
}) {
  const state = useApi((signal) => listDatasets(signal), []);
  const [page, setPage] = useState(1);
  const title = downloadsOnly
    ? "Exports & downloads"
    : "Published dataset versions";
  return (
    <>
      <PageHeader
        eyebrow="Live release catalogue"
        title={title}
        lede={
          downloadsOnly
            ? "Immutable published artifacts with recorded checksums, sizes and attribution."
            : "Published API versions and release metadata. Draft and pipeline controls remain hidden because no administration contract exists for them."
        }
      />
      <AsyncState
        state={state}
        empty="No published dataset versions were returned by the API."
      >
        {(result) => {
          const rows = downloadsOnly
            ? result.data.flatMap((dataset) =>
                dataset.downloads.map((artifact) => ({ dataset, artifact })),
              )
            : result.data.map((dataset) => ({ dataset, artifact: null }));
          const pages = Math.max(1, Math.ceil(rows.length / PAGE_SIZE));
          const current = Math.min(page, pages);
          return (
            <>
              <Card style={{ padding: 0, overflow: "hidden" }}>
                <div style={{ overflowX: "auto" }}>
                  <table
                    className="gg-table"
                    style={{ width: "100%", minWidth: 760 }}
                  >
                    <thead>
                      <tr>
                        <th>Version</th>
                        <th>{downloadsOnly ? "Artifact" : "Published"}</th>
                        <th>{downloadsOnly ? "Records" : "Status"}</th>
                        <th>{downloadsOnly ? "Checksum" : "Licence"}</th>
                        <th>
                          <span className="sr-only">Actions</span>
                        </th>
                      </tr>
                    </thead>
                    <tbody>
                      {rows
                        .slice((current - 1) * PAGE_SIZE, current * PAGE_SIZE)
                        .map(({ dataset, artifact }, index) => (
                          <DatasetRow
                            key={`${dataset.version}-${artifact?.entity ?? index}-${artifact?.format ?? "version"}`}
                            dataset={dataset}
                            artifact={artifact}
                          />
                        ))}
                    </tbody>
                  </table>
                </div>
              </Card>
              <Pager page={current} pages={pages} onPage={setPage} />
            </>
          );
        }}
      </AsyncState>
    </>
  );
}

function DatasetRow({
  dataset,
  artifact,
}: {
  dataset: DatasetVersion;
  artifact: DatasetVersion["downloads"][number] | null;
}) {
  return (
    <tr>
      <td>
        <strong>{dataset.version}</strong>
        {dataset.changelog ? <small>{dataset.changelog}</small> : null}
      </td>
      <td>
        {artifact ? (
          <a href={`/api${artifact.url}`}>
            {artifact.entity}.{artifact.format}
          </a>
        ) : dataset.publishedAt ? (
          new Date(dataset.publishedAt).toLocaleString()
        ) : (
          "Not recorded"
        )}
      </td>
      <td>
        {artifact ? (
          artifact.recordCount.toLocaleString()
        ) : (
          <Badge tone="canonical">{dataset.status}</Badge>
        )}
      </td>
      <td>
        {artifact ? (
          <code title={artifact.sha256}>{artifact.sha256.slice(0, 14)}…</code>
        ) : (
          dataset.license
        )}
      </td>
      <td>
        <TableActions label={`Actions for dataset ${dataset.version}`}>
          {artifact ? (
            <TableActionLink
              href={`/api${artifact.url}`}
              label={`Download ${artifact.entity}.${artifact.format}`}
            >
              <Download />
            </TableActionLink>
          ) : (
            <TableActionButton
              label={`Copy version ${dataset.version}`}
              onClick={() =>
                void navigator.clipboard.writeText(dataset.version)
              }
            >
              <Copy />
            </TableActionButton>
          )}
        </TableActions>
      </td>
    </tr>
  );
}
