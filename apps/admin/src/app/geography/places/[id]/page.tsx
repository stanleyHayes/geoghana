"use client";

import { useState } from "react";
import { useParams } from "next/navigation";
import { Badge, Card, Field, Input } from "@ghanageo/ui";
import { Merge } from "lucide-react";
import { PageHeader } from "@/components/screen";
import { AsyncState, VerificationBadge, useApi } from "@/components/data";
import { RecordEditor, SavedNotice } from "@/components/record-editor";
import { RequirePermission, SignInPanel } from "@/components/session";
import {
  PLACE_TYPE_OPTIONS, VERIFICATION_OPTIONS, deprecatePlace, fetchPlace,
  updatePlace, type PlaceRecord,
} from "@/lib/admin-api";

export default function PlaceDetail() {
  // useParams, not use(params): a client component that awaits a promise
  // SUSPENDS, and the data effect then runs and unmounts around the
  // suspension — its result is discarded as aborted and the record never
  // renders. useParams is synchronous.
  const { id } = useParams<{ id: string }>();
  const [rev, setRev] = useState(0);
  const [savedAt, setSavedAt] = useState(0);
  const state = useApi<PlaceRecord>(() => fetchPlace(id), [id, rev]);

  return (
    <>
      <PageHeader
        eyebrow="Geography · Place"
        title="Place / Locality"
        lede="Edit the canonical record, or merge a duplicate into the record that survives."
      />
      <SignInPanel />
      <SavedNotice at={savedAt} />
      <AsyncState state={state}>
        {(p) => (
          <div style={{ display: "grid", gap: "var(--space-5)" }}>
            <RecordEditor
              title={p.name}
              recordId={p.id}
              onSave={async (changes) => {
                await updatePlace(p.id, changes);
                setRev((v) => v + 1);
              setSavedAt(Date.now());
                setSavedAt(Date.now());
              }}
              fields={[
                { key: "name", label: "Name", value: p.name },
                { key: "type", label: "Type", value: p.type, options: PLACE_TYPE_OPTIONS },
                { key: "districtId", label: "District", value: p.district?.id ?? "" },
                { key: "regionId", label: "Region", value: p.region?.id ?? "" },
                {
                  key: "verificationStatus", label: "Verification",
                  value: p.verificationStatus, options: VERIFICATION_OPTIONS,
                },
              ]}
              footer={
                <div className="gg-stack-row" style={{ marginTop: "var(--space-4)" }}>
                  <VerificationBadge status={p.verificationStatus} />
                  {p.centroid ? (
                    <span style={{ fontFamily: "var(--font-mono)", fontSize: "var(--text-2xs)",
                                   color: "var(--fg-muted)" }}>
                      {p.centroid.latitude.toFixed(5)}, {p.centroid.longitude.toFixed(5)}
                    </span>
                  ) : null}
                </div>
              }
            />
            <MergePanel place={p} onMerged={() => setRev((v) => v + 1)} />
          </div>
        )}
      </AsyncState>
    </>
  );
}

/**
 * Merging a duplicate.
 *
 * There is no delete. The retired id keeps resolving — to a 410 naming its
 * survivor — so every consumer holding it can follow the redirect and update
 * at their own pace instead of getting a 404 with nowhere to go.
 */
function MergePanel({ place, onMerged }: { place: PlaceRecord; onMerged: () => void }) {
  const [target, setTarget] = useState("");
  const [reason, setReason] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [done, setDone] = useState(false);
  const [busy, setBusy] = useState(false);

  if (place.status !== "ACTIVE") {
    return (
      <Card data-intensity="restrained">
        <div className="gg-stack-row">
          <Badge tone="needsRecon">{place.status}</Badge>
          <span style={{ fontSize: "var(--text-sm)", color: "var(--fg-muted)" }}>
            This record is already retired. Its id still resolves to a 410 so nothing
            that stored it is broken.
          </span>
        </div>
      </Card>
    );
  }

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await deprecatePlace(place.id, target.trim(), reason.trim());
      setDone(true);
      onMerged();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not merge.");
    } finally {
      setBusy(false);
    }
  };

  return (
    <Card>
      <div className="gg-stack-row" style={{ marginBottom: "var(--space-3)" }}>
        <Merge size={18} style={{ color: "var(--brand)" }} aria-hidden />
        <strong>Merge into another record</strong>
      </div>
      <p style={{ margin: "0 0 var(--space-4)", color: "var(--fg-muted)", fontSize: "var(--text-sm)" }}>
        The surviving record keeps its id. This one is retired and its id returns 410
        with <code style={{ fontFamily: "var(--font-mono)" }}>mergedInto</code> — never a
        404, so nothing that already stored it breaks.
      </p>

      <RequirePermission
        permission="geography:edit"
        fallback={<Badge tone="needsRecon">Merging needs geography:edit</Badge>}
      >
        {done ? (
          <p role="status" style={{ margin: 0, fontSize: "var(--text-sm)" }}>
            Merged. <code style={{ fontFamily: "var(--font-mono)" }}>{place.id}</code> now
            redirects to <code style={{ fontFamily: "var(--font-mono)" }}>{target}</code>.
          </p>
        ) : (
          <form onSubmit={submit} style={{ display: "grid", gap: "var(--space-4)" }}>
            <Field label="Surviving record id" htmlFor="merge-target"
                   hint="The record that keeps its identity. It must already exist.">
              <Input id="merge-target" value={target} placeholder="01KDVDNA003BF7FQZ9WWE8VPWE"
                     onChange={(e) => setTarget(e.target.value)} required />
            </Field>
            <Field label="Reason" htmlFor="merge-reason"
                   hint="Recorded in the audit log and returned with the 410.">
              <Input id="merge-reason" value={reason}
                     placeholder="duplicate of the Ashanti regional capital"
                     onChange={(e) => setReason(e.target.value)} />
            </Field>
            {error ? (
              <p role="alert" style={{ margin: 0, color: "var(--danger)", fontSize: "var(--text-sm)" }}>
                {error}
              </p>
            ) : null}
            <button className="gg-button gg-button--danger gg-button--sm" type="submit" disabled={busy}>
              {busy ? "Merging…" : "Merge and retire this record"}
            </button>
          </form>
        )}
      </RequirePermission>
    </Card>
  );
}
