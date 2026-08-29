"use client";

import { useState } from "react";
import { Badge, Card, Pagination } from "@ghanageo/ui";
import { MapPin, Search } from "lucide-react";
import { PageHeader } from "@/components/screen";
import { AsyncState, Provenance, VerificationBadge, useApi } from "@/components/data";
import { searchPlaces, type Page, type Place } from "@/lib/api";

export default function ExplorerScreen() {
  const PAGE_SIZE = 8;
  const [q, setQ] = useState("");
  const [selected, setSelected] = useState<Place | null>(null);
  const [pageNumber, setPageNumber] = useState(1);
  const query = q.trim();

  /* The API rejects a query under 2 characters with QUERY_TOO_SHORT, so don't
     send one — an error state the user cannot act on is just noise. */
  const state = useApi<Page<Place> | null>(
    (s) => (query.length >= 2 ? searchPlaces(query, { limit: 25 }, s) : Promise.resolve(null)),
    [query],
  );

  return (
    <>
      <PageHeader
        eyebrow="Explore"
        title="Location Explorer"
        lede="Search the canonical dataset the way a developer's request hits it — same /search endpoint, same ranking, same fuzzy tolerance."
      />

      <label className="gg-searchbar" style={{ cursor: "text", minHeight: 48, maxWidth: 520,
                                               marginBottom: "var(--space-5)" }}>
        <Search size={17} aria-hidden />
        <input
          value={q}
          onChange={(e) => { setQ(e.target.value); setSelected(null); setPageNumber(1); }}
          placeholder="Try “kumsai”, “tema comm”, or “osu”"
          aria-label="Search places"
          style={{ flex: 1, border: 0, background: "transparent", outline: "none",
                   color: "var(--fg)", fontSize: "var(--text-base)", fontFamily: "var(--font-sans)" }}
        />
      </label>

      {query.length < 2 ? (
        <Card data-intensity="restrained">
          <p style={{ margin: 0, color: "var(--fg-muted)", fontSize: "var(--text-sm)" }}>
            Type at least two characters. Typos are tolerated — “kumsai” finds Kumasi.
          </p>
        </Card>
      ) : (
        <div className="gg-split">
          <div>
            <AsyncState state={state} empty={`No match for “${query}”.`}>
              {(page) => {
                const results = page?.data ?? [];
                const pageCount = Math.max(1, Math.ceil(results.length / PAGE_SIZE));
                const visibleResults = results.slice((pageNumber - 1) * PAGE_SIZE, pageNumber * PAGE_SIZE);
                return (
                <div style={{ display: "grid", gap: "var(--space-2)" }}>
                  {visibleResults.map((p) => (
                    <Card
                      key={p.id}
                      interactive
                      data-intensity="restrained"
                      onClick={() => setSelected(p)}
                      style={{ cursor: "pointer",
                               outline: selected?.id === p.id ? "2px solid var(--brand)" : undefined,
                               outlineOffset: 2 }}
                    >
                      <div className="gg-stack-row">
                        <MapPin size={15} style={{ color: "var(--brand)", flexShrink: 0 }} aria-hidden />
                        <div style={{ flex: 1, minWidth: 0 }}>
                          <p style={{ margin: 0, fontWeight: 600 }}>{p.name}</p>
                          <p style={{ margin: 0, fontSize: "var(--text-xs)", color: "var(--fg-muted)" }}>
                            {[p.district?.name, p.region?.name].filter(Boolean).join(" · ") || p.type}
                          </p>
                        </div>
                      </div>
                    </Card>
                  ))}
                  {pageCount > 1 ? <Pagination page={pageNumber} pageCount={pageCount} onPageChange={setPageNumber} label="Search result pages" /> : null}
                </div>
                );
              }}
            </AsyncState>
          </div>

          <Card>
            {selected ? (
              <>
                <h2 style={{ margin: 0, fontSize: "var(--text-xl)" }}>{selected.name}</h2>
                <p style={{ margin: "var(--space-1) 0 var(--space-3)", fontFamily: "var(--font-mono)",
                            fontSize: "var(--text-2xs)", color: "var(--fg-subtle)" }}>{selected.id}</p>
                <div className="gg-stack-row" style={{ marginBottom: "var(--space-4)" }}>
                  <VerificationBadge status={selected.verificationStatus} />
                  <Badge>{selected.type}</Badge>
                </div>

                <dl style={{ margin: 0, display: "grid", gridTemplateColumns: "auto 1fr",
                             gap: "var(--space-2) var(--space-4)", fontSize: "var(--text-sm)" }}>
                  <dt style={{ color: "var(--fg-subtle)" }}>Region</dt>
                  <dd style={{ margin: 0 }}>{selected.region?.name ?? "—"}</dd>
                  <dt style={{ color: "var(--fg-subtle)" }}>District</dt>
                  <dd style={{ margin: 0 }}>{selected.district?.name ?? "unassigned"}</dd>
                  <dt style={{ color: "var(--fg-subtle)" }}>Centroid</dt>
                  <dd style={{ margin: 0, fontFamily: "var(--font-mono)", fontSize: "var(--text-xs)" }}>
                    {selected.centroid
                      ? `${selected.centroid.latitude.toFixed(5)}, ${selected.centroid.longitude.toFixed(5)}`
                      : "—"}
                  </dd>
                  <dt style={{ color: "var(--fg-subtle)" }}>Population</dt>
                  <dd style={{ margin: 0, fontVariantNumeric: "tabular-nums" }}>
                    {selected.population ? selected.population.toLocaleString("en-GH") : "—"}
                  </dd>
                  <dt style={{ color: "var(--fg-subtle)" }}>Source</dt>
                  <dd style={{ margin: 0 }}>
                    <Provenance source={selected.provenance?.sourceId} url={selected.provenance?.sourceUrl} />
                  </dd>
                </dl>

                {selected.aliases?.length ? (
                  <>
                    <h3 style={{ fontSize: "var(--text-sm)", margin: "var(--space-5) 0 var(--space-2)" }}>
                      Aliases ({selected.aliases.length})
                    </h3>
                    <div style={{ display: "flex", flexWrap: "wrap", gap: "var(--space-1)" }}>
                      {selected.aliases.map((a) => (
                        <Badge key={`${a.value}-${a.type}`}>{a.value}</Badge>
                      ))}
                    </div>
                  </>
                ) : null}
              </>
            ) : (
              <p style={{ margin: 0, color: "var(--fg-muted)", fontSize: "var(--text-sm)" }}>
                Select a result to see its full record — aliases, centroid and the source it came from.
              </p>
            )}
          </Card>
        </div>
      )}
    </>
  );
}
