"use client";

import { useState, type ReactNode } from "react";
import { Badge, Card, Field, Input, Select } from "@ghanageo/ui";
import { PencilLine, X } from "lucide-react";
import { RequirePermission, useSession } from "@/components/session";

/**
 * The shared edit surface for a canonical record.
 *
 * One component for regions, districts and places rather than three: the
 * fields differ but the discipline does not — show what will change, refuse to
 * submit nothing, surface the API's own error text, and never pretend a write
 * succeeded. Three copies would drift.
 */

export interface EditableField {
  key: string;
  label: string;
  value: string;
  /** Present for a closed set; renders a branded select, never a native one. */
  options?: { value: string; label: string; hint?: string }[];
  hint?: string;
  readOnly?: boolean;
}

/** Shown by the page after a successful save, outside the async boundary so a
 *  refetch cannot unmount it. */
export function SavedNotice({ at }: { at: number }) {
  if (!at) return null;
  return (
    <p role="status" style={{ margin: 0, color: "var(--brand)", fontSize: "var(--text-sm)" }}>
      ✓ Saved and recorded in the audit log.
    </p>
  );
}

export function RecordEditor({
  title,
  recordId,
  fields,
  onSave,
  footer,
}: {
  title: string;
  recordId: string;
  fields: EditableField[];
  /** Receives ONLY the fields that actually changed. */
  onSave: (changes: Record<string, string>) => Promise<void>;
  footer?: ReactNode;
}) {
  const { can } = useSession();
  const editable = can("geography:edit");

  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState<Record<string, string>>({});
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const start = () => {
    setDraft(Object.fromEntries(fields.map((f) => [f.key, f.value])));
    setEditing(true);
    setError(null);
  };

  // Only what actually differs is sent. A PATCH carrying unchanged fields
  // would overwrite a concurrent edit with values the steward never touched.
  const changed = () =>
    Object.fromEntries(
      fields
        .filter((f) => !f.readOnly && draft[f.key] !== undefined && draft[f.key] !== f.value)
        .map((f) => [f.key, draft[f.key] as string]),
    );

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    const changes = changed();
    if (Object.keys(changes).length === 0) {
      setError("Nothing has changed.");
      return;
    }
    setBusy(true);
    setError(null);
    try {
      await onSave(changes);
      setEditing(false);
    } catch (err) {
      // The API's own message, not a generic one: it says which field was
      // rejected and why, and replacing it with "Something went wrong" throws
      // that away.
      setError(err instanceof Error ? err.message : "Could not save.");
    } finally {
      setBusy(false);
    }
  };

  const pending = editing ? Object.keys(changed()).length : 0;

  return (
    <Card>
      <div className="gg-stack-row" style={{ marginBottom: "var(--space-4)" }}>
        <div style={{ flex: 1, minWidth: 200 }}>
          <h2 style={{ margin: 0, fontSize: "var(--text-lg)" }}>{title}</h2>
          <p style={{ margin: "var(--space-1) 0 0", fontFamily: "var(--font-mono)",
                      fontSize: "var(--text-2xs)", color: "var(--fg-subtle)" }}>{recordId}</p>
        </div>
        {!editing ? (
          <RequirePermission
            permission="geography:edit"
            fallback={<Badge tone="needsRecon">Read only</Badge>}
          >
            <button className="gg-button gg-button--secondary gg-button--sm" onClick={start}>
              <PencilLine size={14} aria-hidden /> Edit
            </button>
          </RequirePermission>
        ) : null}
      </div>

      {!editing ? (
        <dl style={{ margin: 0, display: "grid", gridTemplateColumns: "auto 1fr",
                     gap: "var(--space-2) var(--space-4)", fontSize: "var(--text-sm)" }}>
          {fields.map((f) => (
            <div key={f.key} style={{ display: "contents" }}>
              <dt style={{ color: "var(--fg-subtle)" }}>{f.label}</dt>
              <dd style={{ margin: 0 }}>
                {f.value || <span style={{ color: "var(--fg-subtle)" }}>not recorded</span>}
              </dd>
            </div>
          ))}
        </dl>
      ) : (
        <form onSubmit={submit} style={{ display: "grid", gap: "var(--space-4)" }}>
          {fields.filter((f) => !f.readOnly).map((f) => (
            <Field key={f.key} label={f.label} htmlFor={`f-${f.key}`} {...(f.hint ? { hint: f.hint } : {})}>
              {f.options ? (
                <Select
                  ariaLabel={f.label}
                  value={draft[f.key] ?? ""}
                  onValueChange={(v) => setDraft((d) => ({ ...d, [f.key]: v }))}
                  options={f.options}
                />
              ) : (
                <Input
                  id={`f-${f.key}`}
                  value={draft[f.key] ?? ""}
                  onChange={(e) => setDraft((d) => ({ ...d, [f.key]: e.target.value }))}
                />
              )}
            </Field>
          ))}

          {error ? (
            <p role="alert" style={{ margin: 0, color: "var(--danger)", fontSize: "var(--text-sm)" }}>
              {error}
            </p>
          ) : null}

          <div className="gg-stack-row">
            <button className="gg-button gg-button--primary gg-button--sm" type="submit" disabled={busy || !editable}>
              {busy ? "Saving…" : pending > 0 ? `Save ${pending} change${pending === 1 ? "" : "s"}` : "Save"}
            </button>
            <button
              className="gg-button gg-button--ghost gg-button--sm"
              type="button"
              onClick={() => { setEditing(false); setError(null); }}
            >
              <X size={14} aria-hidden /> Cancel
            </button>
          </div>
        </form>
      )}

      {footer}
    </Card>
  );
}
