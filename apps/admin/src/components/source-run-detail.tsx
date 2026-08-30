"use client";

import { useState } from "react";
import { Badge, Card, EmptyState } from "@ghanageo/ui";
import {
  Archive,
  CircleAlert,
  Eye,
  FileSearch,
  GitCompareArrows,
} from "lucide-react";
import { AsyncState, useApi } from "@/components/data";
import { PageHeader } from "@/components/screen";
import { TableActionButton, TableActions } from "@/components/table-actions";
import {
  getAdminSourceRun,
  getAdminSourceRunRecord,
  listAdminSourceRunConflicts,
  listAdminSourceRunDuplicates,
  listAdminSourceRunRecords,
  type AdminSourceRecord,
} from "@/lib/admin-api";

export function SourceRunDetail({
  id,
  initialKind = "records",
}: {
  id: string;
  initialKind?: EvidenceKind;
}) {
  const state = useApi((signal) => getAdminSourceRun(id, signal), [id]);
  return (
    <>
      <PageHeader
        eyebrow="Immutable import evidence"
        title="Source run"
        lede="A durable import run or historical audit summary. Per-record controls remain read-only and privacy-safe."
      />
      <AsyncState state={state} empty="No such source run was returned.">
        {(run) => (
          <>
            <Card>
              <div className="admin-run-detail__head">
                <div>
                  <p className="admin-metric-label">Source</p>
                  <h2>{run.sourceId || "Unlabelled source"}</h2>
                  <code>{run.id}</code>
                </div>
                <Badge
                  tone={
                    run.status === "succeeded"
                      ? "canonical"
                      : run.status === "failed"
                        ? "danger"
                        : "reference"
                  }
                >
                  {run.status}
                </Badge>
              </div>
              <dl className="admin-detail-grid">
                <div>
                  <dt>Queued</dt>
                  <dd>
                    {run.queuedAt
                      ? new Date(run.queuedAt).toLocaleString()
                      : "Not recorded"}
                  </dd>
                </div>
                <div>
                  <dt>Started</dt>
                  <dd>{new Date(run.startedAt).toLocaleString()}</dd>
                </div>
                <div>
                  <dt>Finished</dt>
                  <dd>
                    {run.finishedAt
                      ? new Date(run.finishedAt).toLocaleString()
                      : "Not recorded"}
                  </dd>
                </div>
                <div>
                  <dt>Duration</dt>
                  <dd>
                    {run.durationMs
                      ? `${run.durationMs.toLocaleString()} ms`
                      : "Not recorded"}
                  </dd>
                </div>
                <div>
                  <dt>Records processed</dt>
                  <dd>{run.recordsProcessed.toLocaleString()}</dd>
                </div>
                <div>
                  <dt>Conflicts / duplicates</dt>
                  <dd>
                    {run.conflicts.toLocaleString()} /{" "}
                    {run.duplicateCandidates.toLocaleString()}
                  </dd>
                </div>
                <div>
                  <dt>Request ID</dt>
                  <dd>{run.requestId || "Not recorded"}</dd>
                </div>
                <div>
                  <dt>Payload hash</dt>
                  <dd className="admin-break-value">
                    {run.payloadHash || "Not recorded"}
                  </dd>
                </div>
                <div>
                  <dt>Detail availability</dt>
                  <dd>
                    {run.detailAvailability === "durable"
                      ? "Durable record detail"
                      : "Historical summary only"}
                  </dd>
                </div>
              </dl>
              {run.errors?.length ? (
                <div className="admin-error-detail">
                  <strong>Recorded error codes</strong>
                  <p>{run.errors.join(" · ")}</p>
                </div>
              ) : run.error ? (
                <div className="admin-error-detail">
                  <strong>Recorded error</strong>
                  <p>{run.error}</p>
                </div>
              ) : null}
            </Card>
            {run.detailAvailability === "historical_summary_only" ? (
              <Card className="admin-historical-summary">
                <Archive size={20} />
                <div>
                  <strong>Historical summary only</strong>
                  <p>
                    This run predates durable per-record retention. Counts and
                    audit evidence are real, but raw references, conflicts and
                    duplicate candidates were never retained and cannot be
                    reconstructed safely.
                  </p>
                </div>
              </Card>
            ) : (
              <RunEvidence runId={run.id} initialKind={initialKind} />
            )}
          </>
        )}
      </AsyncState>
    </>
  );
}

export type EvidenceKind = "records" | "conflicts" | "duplicates";
function RunEvidence({
  runId,
  initialKind,
}: {
  runId: string;
  initialKind: EvidenceKind;
}) {
  const [kind, setKind] = useState<EvidenceKind>(initialKind);
  const [cursor, setCursor] = useState("");
  const [history, setHistory] = useState<string[]>([]);
  const [selectedRecord, setSelectedRecord] = useState<string | null>(null);
  const state = useApi(
    (signal) =>
      kind === "conflicts"
        ? listAdminSourceRunConflicts(runId, { cursor, limit: 20 }, signal)
        : kind === "duplicates"
          ? listAdminSourceRunDuplicates(runId, { cursor, limit: 20 }, signal)
          : listAdminSourceRunRecords(runId, { cursor, limit: 20 }, signal),
    [runId, kind, cursor],
  );
  function changeKind(next: EvidenceKind) {
    setKind(next);
    setCursor("");
    setHistory([]);
    setSelectedRecord(null);
  }
  return (
    <section
      className="admin-run-evidence"
      aria-labelledby="run-evidence-title"
    >
      <div className="admin-section-heading">
        <FileSearch size={18} />
        <h2 id="run-evidence-title">Per-record evidence</h2>
      </div>
      <div
        className="admin-evidence-filters"
        role="group"
        aria-label="Filter import evidence"
      >
        {(
          [
            ["records", "All records"],
            ["conflicts", "Conflicts"],
            ["duplicates", "Duplicate candidates"],
          ] as const
        ).map(([value, label]) => (
          <button
            key={value}
            type="button"
            className={`gg-button gg-button--sm ${kind === value ? "gg-button--primary" : "gg-button--ghost"}`}
            aria-pressed={kind === value}
            onClick={() => changeKind(value)}
          >
            {label}
          </button>
        ))}
      </div>
      <AsyncState
        state={state}
        empty={`No ${kind === "records" ? "records" : kind} are recorded for this run.`}
      >
        {(page) =>
          page.meta.detailAvailability === "historical_summary_only" ? (
            <Card className="admin-historical-summary">
              <Archive size={20} />
              <div>
                <strong>Historical summary only</strong>
                <p>
                  Per-record data was never retained for this audit-derived run.
                </p>
              </div>
            </Card>
          ) : !page.data.length ? (
            <EmptyState
              icon={<FileSearch />}
              title={`No ${kind === "records" ? "records" : kind} recorded`}
              description="The durable evidence store returned no entries for this filter."
            />
          ) : (
            <>
              <Card style={{ padding: 0, overflow: "hidden" }}>
                <div style={{ overflowX: "auto" }}>
                  <table
                    className="gg-table"
                    style={{ width: "100%", minWidth: 820 }}
                  >
                    <thead>
                      <tr>
                        <th>External reference</th>
                        <th>Outcome</th>
                        <th>Reason</th>
                        <th>Payload SHA-256</th>
                        <th>Processed</th>
                        <th>
                          <span className="sr-only">Detail</span>
                        </th>
                      </tr>
                    </thead>
                    <tbody>
                      {page.data.map((record) => (
                        <tr key={record.id}>
                          <td>
                            <strong>
                              {record.externalRef || "Not recorded"}
                            </strong>
                            <small>{record.id}</small>
                          </td>
                          <td>
                            <Badge
                              tone={
                                record.outcome === "accepted" ||
                                record.outcome === "processed"
                                  ? "canonical"
                                  : "reference"
                              }
                            >
                              {record.outcome}
                            </Badge>
                          </td>
                          <td>{record.reasonCode || "—"}</td>
                          <td>
                            <code title={record.payloadHash}>
                              {record.payloadHash
                                ? `${record.payloadHash.slice(0, 14)}…`
                                : "Not recorded"}
                            </code>
                          </td>
                          <td>
                            {new Date(record.processedAt).toLocaleString()}
                          </td>
                          <td>
                            <TableActions
                              label={`Actions for ${record.externalRef || record.id}`}
                            >
                              <TableActionButton
                                label="Inspect record evidence"
                                onClick={() => setSelectedRecord(record.id)}
                              >
                                <Eye />
                              </TableActionButton>
                            </TableActions>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </Card>
              <nav className="admin-pager" aria-label={`${kind} pagination`}>
                <button
                  className="gg-button gg-button--ghost gg-button--sm"
                  disabled={!history.length}
                  onClick={() => {
                    setCursor(history.at(-1) ?? "");
                    setHistory((items) => items.slice(0, -1));
                    setSelectedRecord(null);
                  }}
                >
                  Previous
                </button>
                <span>Page {history.length + 1}</span>
                <button
                  className="gg-button gg-button--ghost gg-button--sm"
                  disabled={!page.meta.nextCursor}
                  onClick={() => {
                    setHistory((items) => [...items, cursor]);
                    setCursor(page.meta.nextCursor ?? "");
                    setSelectedRecord(null);
                  }}
                >
                  Next
                </button>
              </nav>
            </>
          )
        }
      </AsyncState>
      {selectedRecord ? (
        <RecordDetail runId={runId} recordId={selectedRecord} />
      ) : null}
    </section>
  );
}

function RecordDetail({
  runId,
  recordId,
}: {
  runId: string;
  recordId: string;
}) {
  const state = useApi(
    (signal) => getAdminSourceRunRecord(runId, recordId, signal),
    [runId, recordId],
  );
  return (
    <aside className="admin-record-detail" aria-label="Selected source record">
      <AsyncState state={state} empty="No source-record detail was returned.">
        {(record: AdminSourceRecord) => (
          <Card>
            <div className="admin-section-heading">
              <GitCompareArrows size={18} />
              <h2>Record evidence</h2>
              <Badge tone="reference">Privacy-safe reference</Badge>
            </div>
            <dl className="admin-detail-grid">
              <div>
                <dt>Record ID</dt>
                <dd className="admin-break-value">{record.id}</dd>
              </div>
              <div>
                <dt>External reference</dt>
                <dd>{record.externalRef || "Not recorded"}</dd>
              </div>
              <div>
                <dt>Outcome</dt>
                <dd>{record.outcome}</dd>
              </div>
              <div>
                <dt>Reason</dt>
                <dd>{record.reasonCode || "Not recorded"}</dd>
              </div>
              <div>
                <dt>Processed</dt>
                <dd>{new Date(record.processedAt).toLocaleString()}</dd>
              </div>
              <div>
                <dt>Payload SHA-256</dt>
                <dd className="admin-break-value">
                  {record.payloadHash || "Not recorded"}
                </dd>
              </div>
            </dl>
            <div className="admin-unavailable">
              <CircleAlert size={18} />
              <div>
                <strong>Raw payload intentionally unavailable</strong>
                <p>
                  The API exposes a stable reference and digest, not source
                  contents or personal data.
                </p>
              </div>
            </div>
          </Card>
        )}
      </AsyncState>
    </aside>
  );
}
