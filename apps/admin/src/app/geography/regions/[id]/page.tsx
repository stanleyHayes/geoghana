"use client";

import { useState } from "react";
import { useParams } from "next/navigation";
import { PageHeader } from "@/components/screen";
import { AsyncState, VerificationBadge, useApi } from "@/components/data";
import { RecordEditor, SavedNotice } from "@/components/record-editor";
import { SignInPanel } from "@/components/session";
import {
  BoundaryEditor,
  RetirementPanel,
} from "@/components/geography-admin-controls";
import {
  VERIFICATION_OPTIONS,
  fetchRegion,
  updateRegion,
  type RegionRecord,
} from "@/lib/admin-api";

export default function RegionDetail() {
  // useParams, not use(params): a client component that awaits a promise
  // SUSPENDS, and the data effect then runs and unmounts around the
  // suspension — its result is discarded as aborted and the record never
  // renders. useParams is synchronous.
  const { id } = useParams<{ id: string }>();
  // A revision counter re-runs the fetch after a save. useApi has no reload of
  // its own and lives in a file another lane owns, so the refresh is local.
  const [rev, setRev] = useState(0);
  const [savedAt, setSavedAt] = useState(0);
  const state = useApi<RegionRecord>(() => fetchRegion(id), [id, rev]);

  return (
    <>
      <PageHeader
        eyebrow="Geography · Region"
        title="Region"
        lede="Edit the canonical record. Every change is audited."
      />
      <SignInPanel />
      <SavedNotice at={savedAt} />
      <AsyncState state={state}>
        {(r) => (
          <div style={{ display: "grid", gap: "var(--space-5)" }}>
            <div id="editor">
              <RecordEditor
                title={r.name}
                recordId={r.id}
                onSave={async (changes) => {
                  await updateRegion(r.id, changes);
                  setRev((v) => v + 1);
                  setSavedAt(Date.now());
                }}
                fields={[
                  { key: "name", label: "Name", value: r.name },
                  { key: "capital", label: "Capital", value: r.capital ?? "" },
                  { key: "code", label: "Official code", value: r.code ?? "" },
                  {
                    key: "verificationStatus",
                    label: "Verification",
                    value: r.verificationStatus,
                    options: VERIFICATION_OPTIONS,
                    hint: "A seed row is never promoted automatically (R5).",
                  },
                ]}
                footer={
                  <div
                    className="gg-stack-row"
                    style={{ marginTop: "var(--space-4)" }}
                  >
                    <VerificationBadge status={r.verificationStatus} />
                    <span
                      style={{
                        fontSize: "var(--text-xs)",
                        color: "var(--fg-muted)",
                      }}
                    >
                      Source: {r.provenance?.sourceId ?? "not recorded"}
                    </span>
                  </div>
                }
              />
            </div>
            <BoundaryEditor kind="region" id={r.id} />
            <div id="retire">
              <RetirementPanel
                kind="regions"
                id={r.id}
                status={r.status}
                onChanged={() => setRev((v) => v + 1)}
              />
            </div>
          </div>
        )}
      </AsyncState>
    </>
  );
}
