"use client";

import { useState } from "react";
import { Card } from "@ghanageo/ui";
import { Search } from "lucide-react";
import { PageHeader } from "@/components/screen";
import { AsyncState, Provenance, VerificationBadge, useApi } from "@/components/data";
import { listPlaces, type Page, type Place } from "@/lib/api";

export default function PlacesScreen() {
  const [q, setQ] = useState("");
  const state = useApi<Page<Place>>((s) => listPlaces({ limit: 60, q: q.trim() || undefined }, s), [q]);

  return (
    <>
      <PageHeader
        eyebrow="Geography"
        title="Places / Localities"
        lede="Towns, suburbs and villages with coordinates and provenance. 15,925 records from GeoNames, reconciled against the seed set."
      />

      <label className="gg-searchbar" style={{ cursor: "text", minHeight: 44, maxWidth: 460,
                                               marginBottom: "var(--space-5)" }}>
        <Search size={16} aria-hidden />
        <input
          value={q}
          onChange={(e) => setQ(e.target.value)}
          placeholder="Filter by name…"
          aria-label="Filter places by name"
          style={{ flex: 1, border: 0, background: "transparent", outline: "none",
                   color: "var(--fg)", fontSize: "var(--text-sm)", fontFamily: "var(--font-sans)" }}
        />
      </label>

      <AsyncState state={state} empty="No places match that filter.">
        {(page) => (
          <>
            <p style={{ color: "var(--fg-muted)", fontSize: "var(--text-sm)",
                        margin: "0 0 var(--space-4)" }}>
              Showing {page.data.length}{page.nextCursor ? " (more available)" : ""}
            </p>
            <Card style={{ padding: 0, overflow: "hidden" }}>
              <div style={{ overflowX: "auto", maxHeight: "70vh" }}>
                <table className="gg-table" style={{ width: "100%", minWidth: 760,
                                                     padding: "0 var(--space-4)" }}>
                  <thead>
                    <tr>
                      <th scope="col">Place</th>
                      <th scope="col">District</th>
                      <th scope="col">Region</th>
                      <th scope="col" style={{ textAlign: "right" }}>Centroid</th>
                      <th scope="col">Verification</th>
                      <th scope="col">Source</th>
                    </tr>
                  </thead>
                  <tbody>
                    {page.data.map((p) => (
                      <tr key={p.id}>
                        <td>
                          <span style={{ fontWeight: 600 }}>{p.name}</span>
                          <span style={{ display: "block", fontSize: "var(--text-2xs)",
                                         color: "var(--fg-subtle)" }}>
                            {p.type}
                            {p.aliases?.length ? ` · ${p.aliases.length} aliases` : ""}
                          </span>
                        </td>
                        <td>{p.district?.name ?? <span style={{ color: "var(--fg-subtle)" }}>unassigned</span>}</td>
                        <td>{p.region?.name ?? "—"}</td>
                        <td style={{ textAlign: "right", fontFamily: "var(--font-mono)",
                                     fontSize: "var(--text-2xs)", whiteSpace: "nowrap" }}>
                          {p.centroid
                            ? `${p.centroid.latitude.toFixed(4)}, ${p.centroid.longitude.toFixed(4)}`
                            : "—"}
                        </td>
                        <td><VerificationBadge status={p.verificationStatus} /></td>
                        <td><Provenance source={p.provenance?.sourceId} url={p.provenance?.sourceUrl} /></td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </Card>
          </>
        )}
      </AsyncState>
    </>
  );
}
