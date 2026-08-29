"use client";

import { Card } from "@ghanageo/ui";
import { PageHeader } from "@/components/screen";
import { AsyncState, Provenance, VerificationBadge, useApi } from "@/components/data";
import { listRegions, type Page, type Region } from "@/lib/api";

export default function RegionsScreen() {
  const state = useApi<Page<Region>>((s) => listRegions({ limit: 50 }, s), []);

  return (
    <>
      <PageHeader
        eyebrow="Geography"
        title="Regions"
        lede="Ghana's 16 regions, the top level of the administrative hierarchy. Reading the live API — the same endpoint any developer calls."
      />

      <AsyncState state={state} empty="No regions returned.">
        {(page) => (
          <>
            <p style={{ color: "var(--fg-muted)", fontSize: "var(--text-sm)",
                        margin: "0 0 var(--space-4)" }}>
              {page.data.length} regions · dataset{" "}
              <code style={{ fontFamily: "var(--font-mono)" }}>{page.datasetVersion}</code>
            </p>
            <Card style={{ padding: 0, overflow: "hidden" }}>
              <div style={{ overflowX: "auto" }}>
                <table className="gg-table" style={{ width: "100%", minWidth: 640 }}>
                  <thead>
                    <tr>
                      <th scope="col">Region</th>
                      <th scope="col">Capital</th>
                      <th scope="col">Verification</th>
                      <th scope="col">Source</th>
                    </tr>
                  </thead>
                  <tbody>
                    {page.data.map((r) => (
                      <tr key={r.id}>
                        <td>
                          <a href={`/geography/regions/${encodeURIComponent(r.id)}`} style={{ fontWeight: 600, color: "var(--brand)" }}>{r.name}</a>
                          <span style={{ display: "block", fontFamily: "var(--font-mono)",
                                         fontSize: "var(--text-2xs)", color: "var(--fg-subtle)" }}>{r.id}</span>
                        </td>
                        <td>{r.capital ?? "—"}</td>
                        <td><VerificationBadge status={r.verificationStatus} /></td>
                        <td><Provenance source={r.provenance?.sourceId} url={r.provenance?.sourceUrl} /></td>
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
