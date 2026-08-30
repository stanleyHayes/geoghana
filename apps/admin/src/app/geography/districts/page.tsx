"use client";

import { useState } from "react";
import { Card, Field, Pagination, Select } from "@ghanageo/ui";
import { PageHeader } from "@/components/screen";
import { AsyncState, VerificationBadge, useApi } from "@/components/data";
import {
  fetchAll,
  listRegionDistricts,
  listDistricts,
  listRegions,
  type District,
  type Page,
  type Region,
} from "@/lib/api";
import { GeographyCreatePanel } from "@/components/geography-admin-controls";
import { TableActionLink, TableActions } from "@/components/table-actions";
import { Archive, Eye, Pencil } from "lucide-react";

const ALL = "__all__";
const PAGE_SIZE = 25;

export default function DistrictsScreen() {
  const [region, setRegion] = useState(ALL);
  const [pageNumber, setPageNumber] = useState(1);

  const regions = useApi<Page<Region>>(
    (s) => listRegions({ limit: 100 }, s),
    [],
  );
  /* Paginated, because the API caps a page at 100 and Ghana has 261 MMDAs.
     Asking for 300 quietly returns 100 — a wrong number rendered confidently. */
  const districts = useApi(
    (s) =>
      fetchAll<District>(
        (cursor, sig) =>
          region === ALL
            ? listDistricts({ limit: 100, cursor }, sig)
            : listRegionDistricts(region, { limit: 100, cursor }, sig),
        s,
      ),
    [region],
  );

  return (
    <>
      <PageHeader
        eyebrow="Geography"
        title="Districts / MMDAs"
        lede="Ghana's 261 Metropolitan, Municipal and District Assemblies. Filter by region to check a single region's count against the published figure."
        actions={
          <div style={{ minWidth: 240 }}>
            <Field label="Region" htmlFor="region-filter">
              <Select
                ariaLabel="Filter by region"
                value={region}
                onValueChange={(value) => {
                  setRegion(value);
                  setPageNumber(1);
                }}
                options={[
                  { value: ALL, label: "All regions", hint: "261 districts" },
                  ...(regions.data?.data ?? []).map((r) => ({
                    value: r.id,
                    label: r.name,
                  })),
                ]}
              />
            </Field>
          </div>
        }
      />
      <GeographyCreatePanel kind="districts" />

      <AsyncState state={districts} empty="No districts for that region.">
        {(page) => {
          const pageCount = Math.max(
            1,
            Math.ceil(page.data.length / PAGE_SIZE),
          );
          const visibleDistricts = page.data.slice(
            (pageNumber - 1) * PAGE_SIZE,
            pageNumber * PAGE_SIZE,
          );
          return (
            <>
              <p
                style={{
                  color: "var(--fg-muted)",
                  fontSize: "var(--text-sm)",
                  margin: "0 0 var(--space-4)",
                }}
              >
                {page.data.length} districts · showing {visibleDistricts.length}{" "}
                on page {pageNumber}
                {region !== ALL ? " in this region" : ""}
                {page.datasetVersion ? (
                  <>
                    {" "}
                    · dataset{" "}
                    <code style={{ fontFamily: "var(--font-mono)" }}>
                      {page.datasetVersion}
                    </code>
                  </>
                ) : null}
                {/* Never let a truncated list read as a complete one. */}
                {!page.complete ? (
                  <strong style={{ color: "var(--warning, var(--danger))" }}>
                    {" "}
                    · truncated, more pages exist
                  </strong>
                ) : null}
              </p>
              <Card style={{ padding: 0, overflow: "hidden" }}>
                <div style={{ overflowX: "auto", maxHeight: "70vh" }}>
                  <table
                    className="gg-table"
                    style={{ width: "100%", minWidth: 640 }}
                  >
                    <thead>
                      <tr>
                        <th scope="col">District</th>
                        <th scope="col">Region</th>
                        <th scope="col">Type</th>
                        <th scope="col">Verification</th>
                        <th scope="col">
                          <span className="sr-only">Actions</span>
                        </th>
                      </tr>
                    </thead>
                    <tbody>
                      {visibleDistricts.map((d) => (
                        <tr key={d.id}>
                          <td>
                            <a
                              href={`/geography/districts/${encodeURIComponent(d.id)}`}
                              style={{ fontWeight: 600, color: "var(--brand)" }}
                            >
                              {d.name}
                            </a>
                            <span
                              style={{
                                display: "block",
                                fontFamily: "var(--font-mono)",
                                fontSize: "var(--text-2xs)",
                                color: "var(--fg-subtle)",
                              }}
                            >
                              {d.id}
                            </span>
                          </td>
                          <td>{d.region?.name ?? "—"}</td>
                          {/* UNSPECIFIED is the honest value: the seed source did
                            not record whether each is Metropolitan, Municipal
                            or District. Showing a guess would be worse. */}
                          <td
                            style={{
                              color:
                                d.type === "UNSPECIFIED"
                                  ? "var(--fg-subtle)"
                                  : undefined,
                            }}
                          >
                            {d.type === "UNSPECIFIED" ? "not recorded" : d.type}
                          </td>
                          <td>
                            <VerificationBadge status={d.verificationStatus} />
                          </td>
                          <td>
                            <TableActions label={`Actions for ${d.name}`}>
                              <TableActionLink
                                href={`/geography/districts/${encodeURIComponent(d.id)}`}
                                label={`View ${d.name}`}
                              >
                                <Eye />
                              </TableActionLink>
                              <TableActionLink
                                href={`/geography/districts/${encodeURIComponent(d.id)}#editor`}
                                label={`Edit ${d.name}`}
                              >
                                <Pencil />
                              </TableActionLink>
                              <TableActionLink
                                href={`/geography/districts/${encodeURIComponent(d.id)}#retire`}
                                label={`Deprecate or merge ${d.name}`}
                                tone="danger"
                              >
                                <Archive />
                              </TableActionLink>
                            </TableActions>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </Card>
              <Pagination
                page={pageNumber}
                pageCount={pageCount}
                onPageChange={setPageNumber}
                label="District pages"
              />
            </>
          );
        }}
      </AsyncState>
    </>
  );
}
