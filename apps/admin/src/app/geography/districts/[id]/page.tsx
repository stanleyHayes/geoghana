"use client";

import { useState } from "react";
import { useParams } from "next/navigation";
import { PageHeader } from "@/components/screen";
import { AsyncState, VerificationBadge, useApi } from "@/components/data";
import { RecordEditor, SavedNotice } from "@/components/record-editor";
import { SignInPanel } from "@/components/session";
import {
  VERIFICATION_OPTIONS, fetchDistrict, updateDistrict, type DistrictRecord,
} from "@/lib/admin-api";

const DISTRICT_TYPES = [
  { value: "METROPOLITAN", label: "Metropolitan" },
  { value: "MUNICIPAL", label: "Municipal" },
  { value: "DISTRICT", label: "District" },
  { value: "UNSPECIFIED", label: "Not recorded", hint: "The seed source did not say" },
];

export default function DistrictDetail() {
  // useParams, not use(params): a client component that awaits a promise
  // SUSPENDS, and the data effect then runs and unmounts around the
  // suspension — its result is discarded as aborted and the record never
  // renders. useParams is synchronous.
  const { id } = useParams<{ id: string }>();
  const [rev, setRev] = useState(0);
  const [savedAt, setSavedAt] = useState(0);
  const state = useApi<DistrictRecord>(() => fetchDistrict(id), [id, rev]);

  return (
    <>
      <PageHeader
        eyebrow="Geography · District"
        title="District / MMDA"
        lede="Edit the canonical record. Every change is audited, and the region a district belongs to is part of its identity."
      />
      <SignInPanel />
      <SavedNotice at={savedAt} />
      <AsyncState state={state}>
        {(d) => (
          <RecordEditor
            title={d.name}
            recordId={d.id}
            onSave={async (changes) => {
              await updateDistrict(d.id, changes);
              setRev((v) => v + 1);
              setSavedAt(Date.now());
            }}
            fields={[
              { key: "name", label: "Name", value: d.name },
              { key: "type", label: "Type", value: d.type ?? "UNSPECIFIED", options: DISTRICT_TYPES },
              { key: "capital", label: "Capital", value: d.capital ?? "" },
              { key: "code", label: "Official code", value: d.code ?? "" },
              {
                key: "regionId", label: "Region", value: d.region?.id ?? "",
                hint: "Moving a district between regions changes what every place inside it reports.",
              },
              {
                key: "verificationStatus", label: "Verification",
                value: d.verificationStatus, options: VERIFICATION_OPTIONS,
              },
            ]}
            footer={
              <div className="gg-stack-row" style={{ marginTop: "var(--space-4)" }}>
                <VerificationBadge status={d.verificationStatus} />
                <span style={{ fontSize: "var(--text-xs)", color: "var(--fg-muted)" }}>
                  {d.region?.name ?? "no region"} · source {d.provenance?.sourceId ?? "not recorded"}
                </span>
              </div>
            }
          />
        )}
      </AsyncState>
    </>
  );
}
