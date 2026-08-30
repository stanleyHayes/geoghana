"use client";

import { useState, type FormEvent } from "react";
import { Badge, Card, EmptyState, Select } from "@ghanageo/ui";
import {
  CircleAlert,
  Download,
  Eye,
  Link2,
  Pencil,
  RefreshCw,
  ServerCog,
  ShieldAlert,
} from "lucide-react";
import { AsyncState, useApi } from "@/components/data";
import { PageHeader } from "@/components/screen";
import { RequirePermission } from "@/components/session";
import {
  TableActionButton,
  TableActionLink,
  TableActions,
} from "@/components/table-actions";
import {
  advanceAdminDatasetRelease,
  getAdminDashboard,
  getAdminDatasetReleaseReadiness,
  getAdminSystemHealth,
  listAdminAuditLog,
  listAdminDatasetReleases,
  listAdminSourceRuns,
  publishAdminDatasetRelease,
  rollbackAdminDatasetRelease,
  updateAdminDatasetChangelog,
  type AdminAuditEntry,
  type AdminDatasetRelease,
} from "@/lib/admin-api";

function CursorPager({
  cursor,
  history,
  next,
  onMove,
}: {
  cursor: string;
  history: string[];
  next?: string | undefined;
  onMove: (cursor: string, history: string[]) => void;
}) {
  return (
    <nav className="admin-pager" aria-label="Result pagination">
      <button
        className="gg-button gg-button--ghost gg-button--sm"
        disabled={!history.length}
        onClick={() => onMove(history.at(-1) ?? "", history.slice(0, -1))}
      >
        Previous
      </button>
      <span>Page {history.length + 1}</span>
      <button
        className="gg-button gg-button--ghost gg-button--sm"
        disabled={!next}
        onClick={() => onMove(next ?? "", [...history, cursor])}
      >
        Next
      </button>
    </nav>
  );
}

export function ActivityFeed() {
  const state = useApi((signal) => getAdminDashboard(signal), []);
  return (
    <>
      <PageHeader
        eyebrow="Protected activity projection"
        title="Activity feed"
        lede="The eight most recent privileged actions, excluding IP addresses and before/after payloads so broader dashboard access cannot bypass audit permissions."
      />
      <AsyncState state={state} empty="No dashboard activity was returned.">
        {(dashboard) =>
          dashboard.recentActivity.length ? (
            <Card style={{ padding: 0 }}>
              <ol className="admin-event-list">
                {dashboard.recentActivity.map((entry) => (
                  <li key={entry.id}>
                    <span
                      className={`admin-status-dot admin-status-dot--${entry.outcome}`}
                    />
                    <div>
                      <strong>{entry.action}</strong>
                      <p>
                        {entry.actor.label ||
                          entry.actor.id ||
                          entry.actor.kind}{" "}
                        ·{" "}
                        {entry.target.label ||
                          entry.target.id ||
                          entry.target.kind}
                      </p>
                    </div>
                    <time dateTime={entry.at}>
                      {new Date(entry.at).toLocaleString()}
                    </time>
                  </li>
                ))}
              </ol>
            </Card>
          ) : (
            <EmptyState
              title="No recent activity"
              description="Privileged operator actions will appear here after they are recorded in the audit trail."
            />
          )
        }
      </AsyncState>
    </>
  );
}

export function SystemHealth({
  view = "health",
}: {
  view?:
    | "health"
    | "queues"
    | "dlq"
    | "freshness"
    | "database"
    | "performance"
    | "slo";
}) {
  const state = useApi((signal) => getAdminSystemHealth(signal), []);
  const titles = {
    health: "Service health",
    queues: "Queues & workers",
    dlq: "Outbox & dead letters",
    freshness: "ETL freshness",
    database: "Database",
    performance: "Measured performance",
    slo: "Operational thresholds",
  };
  return (
    <>
      <PageHeader
        eyebrow="Measured operational state"
        title={titles[view]}
        lede="Only dimensions measured by the protected API are shown. Missing probes remain explicitly unavailable."
      />
      <AsyncState state={state} empty="No health projection was returned.">
        {(health) => {
          if (view === "queues" || view === "dlq")
            return (
              <>
                <div className="gg-auto-grid">
                  {Object.entries(health.queue).map(([status, count]) => (
                    <Card key={status}>
                      <p className="admin-metric-label">{status}</p>
                      <p className="admin-metric-value">
                        {count.toLocaleString()}
                      </p>
                      <Badge
                        tone={status === "dead" && count ? "danger" : "neutral"}
                      >
                        Measured queue state
                      </Badge>
                    </Card>
                  ))}
                  <HealthMetric
                    name="Queue depth"
                    metric={health.metrics.queueDepth}
                  />
                </div>
                <Card className="admin-unavailable">
                  <CircleAlert size={18} />
                  <div>
                    <strong>
                      {view === "dlq"
                        ? "Message details and replay unavailable"
                        : "Worker capacity and throughput unavailable"}
                    </strong>
                    <p>
                      {view === "dlq"
                        ? "The API publishes dead and pending counts, not message bodies, failure reasons, retention settings or replay controls."
                        : "The API publishes queue depth and thresholds, not worker inventory, throughput or retry controls."}
                    </p>
                  </div>
                </Card>
              </>
            );
          if (view === "freshness")
            return (
              <>
                <div className="gg-auto-grid">
                  <Card>
                    <p className="admin-metric-label">
                      Latest recorded source import
                    </p>
                    <p className="admin-metric-value admin-metric-value--version">
                      {health.etlFreshness
                        ? new Date(health.etlFreshness).toLocaleString()
                        : "Not recorded"}
                    </p>
                    <Badge tone={health.etlFreshness ? "reference" : "neutral"}>
                      {health.etlFreshness ? "Measured" : "Unavailable"}
                    </Badge>
                  </Card>
                  <HealthMetric
                    name="ETL age"
                    metric={health.metrics.etlFreshness}
                  />
                </div>
                <Card className="admin-unavailable">
                  <CircleAlert size={18} />
                  <div>
                    <strong>Per-source freshness unavailable</strong>
                    <p>
                      The current contract reports the latest completed import
                      and its threshold, without adapter-specific schedules.
                    </p>
                  </div>
                </Card>
              </>
            );
          const dependencies =
            view === "database"
              ? health.dependencies.filter((item) => item.name === "mongodb")
              : health.dependencies;
          const metricKeys: Array<[string, string]> =
            view === "database"
              ? [["Database latency", "databaseLatency"]]
              : [
                  ["Cache hit rate", "cacheHitRate"],
                  ["Search index parity / lag", "searchIndexLag"],
                  ["Request error rate", "errorRate"],
                ];
          return (
            <>
              <div className="admin-live-note">
                <Badge
                  tone={
                    health.status === "healthy" ? "canonical" : "needsRecon"
                  }
                >
                  {health.status}
                </Badge>
                <span>Index status: {health.indexStatus}</span>
              </div>
              <div className="admin-health-grid">
                {dependencies.map((item) => (
                  <Card key={item.name}>
                    <div className="admin-health-card__head">
                      <ServerCog size={18} />
                      <strong>{item.name}</strong>
                      <Badge
                        tone={
                          item.status === "healthy" ? "canonical" : "neutral"
                        }
                      >
                        {item.status}
                      </Badge>
                    </div>
                    <p>{item.detail || "No detail recorded."}</p>
                    <dl>
                      <div>
                        <dt>Latency</dt>
                        <dd>
                          {item.status === "unavailable"
                            ? "Unavailable"
                            : `${item.latencyMs} ms`}
                        </dd>
                      </div>
                      <div>
                        <dt>Checked</dt>
                        <dd>{new Date(item.checkedAt).toLocaleString()}</dd>
                      </div>
                    </dl>
                  </Card>
                ))}
              </div>
              <div className="gg-auto-grid">
                {metricKeys.map(([label, key]) => (
                  <HealthMetric
                    key={key}
                    name={label}
                    metric={health.metrics[key]}
                  />
                ))}
              </div>
              {view === "database" ? (
                <Card className="admin-unavailable">
                  <CircleAlert size={18} />
                  <div>
                    <strong>Cluster internals unavailable</strong>
                    <p>
                      MongoDB ping latency and its threshold are measured;
                      replication, storage, index efficiency and connection-pool
                      metrics are not published.
                    </p>
                  </div>
                </Card>
              ) : null}
            </>
          );
        }}
      </AsyncState>
    </>
  );
}

function HealthMetric({
  name,
  metric,
}: {
  name: string;
  metric:
    | {
        value: number;
        unit: string;
        status: string;
        threshold: string;
        detail?: string;
      }
    | undefined;
}) {
  if (!metric)
    return (
      <Card>
        <p className="admin-metric-label">{name}</p>
        <p className="admin-metric-value admin-metric-value--version">
          Unavailable
        </p>
        <Badge tone="neutral">Not published</Badge>
      </Card>
    );
  const available = metric.status !== "unavailable";
  return (
    <Card>
      <p className="admin-metric-label">{name}</p>
      <p className="admin-metric-value admin-metric-value--version">
        {available
          ? `${metric.value.toLocaleString()} ${metric.unit}`
          : "Unavailable"}
      </p>
      <Badge
        tone={
          metric.status === "healthy"
            ? "canonical"
            : metric.status === "unhealthy"
              ? "danger"
              : "needsRecon"
        }
      >
        {metric.status}
      </Badge>
      <p className="admin-health-threshold">{metric.threshold}</p>
      {metric.detail ? <small>{metric.detail}</small> : null}
    </Card>
  );
}

export function SourceRuns({ focus }: { focus?: "duplicates" } = {}) {
  const [cursor, setCursor] = useState("");
  const [history, setHistory] = useState<string[]>([]);
  const state = useApi(
    (signal) => listAdminSourceRuns({ cursor, limit: 20 }, signal),
    [cursor],
  );
  return (
    <>
      <PageHeader
        eyebrow="Immutable import evidence"
        title={
          focus === "duplicates"
            ? "Duplicate candidates by import run"
            : "Import runs"
        }
        lede={
          focus === "duplicates"
            ? "Choose a durable run to inspect duplicate-candidate evidence. Older audit-derived runs remain honestly marked as historical summaries."
            : "Recorded source imports with content hashes, durable detail availability and reconciliation counts."
        }
      />
      <AsyncState state={state} empty="No source import runs are recorded.">
        {(page) => (
          <>
            <Card style={{ padding: 0, overflow: "hidden" }}>
              <div style={{ overflowX: "auto" }}>
                <table
                  className="gg-table"
                  style={{ width: "100%", minWidth: 820 }}
                >
                  <thead>
                    <tr>
                      <th>Source / run</th>
                      <th>Outcome</th>
                      <th>Processed</th>
                      <th>Conflicts</th>
                      <th>Detail</th>
                      <th>Started</th>
                      <th>
                        <span className="sr-only">Actions</span>
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {page.data.map((run) => (
                      <tr key={run.id}>
                        <td>
                          <a
                            href={`/ingest/runs/${encodeURIComponent(run.id)}${focus === "duplicates" ? "?evidence=duplicates" : ""}`}
                          >
                            <strong>
                              {run.sourceId || "Unlabelled source"}
                            </strong>
                          </a>
                          <small>{run.id}</small>
                        </td>
                        <td>
                          <Badge
                            tone={
                              run.status === "succeeded"
                                ? "canonical"
                                : "danger"
                            }
                          >
                            {run.status}
                          </Badge>
                        </td>
                        <td>{run.recordsProcessed.toLocaleString()}</td>
                        <td>
                          {run.conflicts.toLocaleString()} ·{" "}
                          {run.duplicateCandidates.toLocaleString()} duplicates
                        </td>
                        <td>
                          {run.detailAvailability === "durable"
                            ? "Durable"
                            : "Summary only"}
                        </td>
                        <td>{new Date(run.startedAt).toLocaleString()}</td>
                        <td>
                          <TableActions label={`Actions for run ${run.id}`}>
                            <TableActionLink
                              href={`/ingest/runs/${encodeURIComponent(run.id)}${focus === "duplicates" ? "?evidence=duplicates" : ""}`}
                              label="View run evidence"
                            >
                              <Eye />
                            </TableActionLink>
                          </TableActions>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </Card>
            <CursorPager
              cursor={cursor}
              history={history}
              next={page.meta.nextCursor}
              onMove={(nextCursor, nextHistory) => {
                setCursor(nextCursor);
                setHistory(nextHistory);
              }}
            />
          </>
        )}
      </AsyncState>
    </>
  );
}

export function DatasetReleases({
  mode = "catalogue",
}: {
  mode?: "catalogue" | "pipeline" | "validation" | "changelog";
}) {
  const [cursor, setCursor] = useState("");
  const [history, setHistory] = useState<string[]>([]);
  const [refreshKey, setRefreshKey] = useState(0);
  const [selected, setSelected] = useState<AdminDatasetRelease | null>(null);
  const state = useApi(
    (signal) => listAdminDatasetReleases({ cursor, limit: 20 }, signal),
    [cursor, refreshKey],
  );
  const copy =
    mode === "validation"
      ? {
          eyebrow: "Artifact validation",
          title: "Release readiness",
          lede: "Byte-level size and SHA-256 evidence reported by the server for a selected release.",
        }
      : mode === "changelog"
        ? {
            eyebrow: "Audited release notes",
            title: "Dataset changelogs",
            lede: "Author public release notes before publication; every edit requires a reason and is recorded atomically with audit evidence.",
          }
        : mode === "pipeline"
          ? {
              eyebrow: "Atomic release control",
              title: "Release pipeline",
              lede: "Advance one legal stage at a time from draft through validation, review and approval, then publish or restore an explicitly named version.",
            }
          : {
              eyebrow: "Protected release catalogue",
              title: "Dataset release lifecycle",
              lede: "Published and non-published release states recorded by the administration API.",
            };
  return (
    <>
      <PageHeader
        eyebrow={copy.eyebrow}
        title={copy.title}
        lede={copy.lede}
        actions={
          <button
            className="gg-button gg-button--ghost gg-button--sm"
            onClick={() => setRefreshKey((value) => value + 1)}
          >
            <RefreshCw size={15} /> Refresh
          </button>
        }
      />
      <AsyncState state={state} empty="No dataset releases are recorded.">
        {(page) => (
          <>
            <Card style={{ padding: 0, overflow: "hidden" }}>
              <div style={{ overflowX: "auto" }}>
                <table
                  className="gg-table"
                  style={{ width: "100%", minWidth: 820 }}
                >
                  <thead>
                    <tr>
                      <th>Version</th>
                      <th>Status</th>
                      <th>Published</th>
                      <th>{mode === "changelog" ? "Changelog" : "Counts"}</th>
                      <th>Artifacts</th>
                      <th>
                        <span className="sr-only">Actions</span>
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {page.data.map((release) => (
                      <tr key={release.version}>
                        <td>
                          <strong>{release.version}</strong>
                        </td>
                        <td>
                          <Badge
                            tone={
                              release.status === "published"
                                ? "canonical"
                                : release.status === "approved"
                                  ? "reviewed"
                                  : "reference"
                            }
                          >
                            {release.status}
                          </Badge>
                        </td>
                        <td>
                          {release.publishedAt
                            ? new Date(release.publishedAt).toLocaleString()
                            : "Not recorded"}
                        </td>
                        <td>
                          {mode === "changelog"
                            ? release.changelog || "Not recorded"
                            : release.counts
                              ? Object.entries(release.counts)
                                  .map(
                                    ([key, value]) =>
                                      `${key}: ${value.toLocaleString()}`,
                                  )
                                  .join(" · ")
                              : "Not recorded"}
                        </td>
                        <td>{release.downloads.length.toLocaleString()}</td>
                        <td>
                          <TableActions
                            label={`Actions for release ${release.version}`}
                          >
                            <TableActionButton
                              label={
                                mode === "changelog"
                                  ? "Edit release notes"
                                  : "View release readiness"
                              }
                              onClick={() => setSelected(release)}
                            >
                              {mode === "changelog" ? <Pencil /> : <Eye />}
                            </TableActionButton>
                          </TableActions>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </Card>
            <CursorPager
              cursor={cursor}
              history={history}
              next={page.meta.nextCursor}
              onMove={(nextCursor, nextHistory) => {
                setCursor(nextCursor);
                setHistory(nextHistory);
                setSelected(null);
              }}
            />
          </>
        )}
      </AsyncState>
      {selected ? (
        mode === "changelog" ? (
          <ChangelogEditor
            release={selected}
            onChanged={() => {
              setSelected(null);
              setRefreshKey((value) => value + 1);
            }}
          />
        ) : (
          <ReleaseReadiness
            release={selected}
            allowActions={mode === "pipeline"}
            onChanged={() => {
              setSelected(null);
              setRefreshKey((value) => value + 1);
            }}
          />
        )
      ) : (
        <Card className="admin-unavailable">
          <CircleAlert size={18} />
          <div>
            <strong>Select a release to inspect server evidence.</strong>
            <p>No readiness or lifecycle state is inferred in the browser.</p>
          </div>
        </Card>
      )}
    </>
  );
}

function ReleaseReadiness({
  release,
  allowActions,
  onChanged,
}: {
  release: AdminDatasetRelease;
  allowActions: boolean;
  onChanged: () => void;
}) {
  const state = useApi(
    (signal) => getAdminDatasetReleaseReadiness(release.version, signal),
    [release.version],
  );
  const nextStage = (
    { draft: "validation", validation: "review", review: "approved" } as const
  )[release.status as "draft" | "validation" | "review"];
  const [confirmVersion, setConfirmVersion] = useState("");
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState<"advance" | "publish" | "rollback" | null>(
    null,
  );
  const [notice, setNotice] = useState("");
  const [error, setError] = useState("");
  const notesRequired = nextStage === "review" || nextStage === "approved";

  async function mutate(action: "advance" | "publish" | "rollback") {
    setBusy(action);
    setNotice("");
    setError("");
    try {
      if (action === "advance" && nextStage) {
        const result = await advanceAdminDatasetRelease(
          release.version,
          nextStage,
          reason,
        );
        setNotice(
          `Advanced ${result.version} to ${result.status}. Refreshing…`,
        );
      } else if (action === "publish") {
        const result = await publishAdminDatasetRelease(
          release.version,
          confirmVersion,
        );
        setNotice(
          `Published ${result.version}. Refreshing the lifecycle view…`,
        );
      } else if (action === "rollback") {
        const result = await rollbackAdminDatasetRelease(
          release.version,
          confirmVersion,
          reason,
        );
        setNotice(
          `Restored ${result.restored.version}; ${result.from.version} is now rolled back. Refreshing…`,
        );
      }
      setConfirmVersion("");
      setReason("");
      window.setTimeout(onChanged, 1200);
    } catch (cause) {
      setError(
        `${cause instanceof Error ? cause.message : "The release operation was refused."} Refresh to reconcile the recorded lifecycle state.`,
      );
      window.setTimeout(onChanged, 1800);
    } finally {
      setBusy(null);
    }
  }

  return (
    <section
      className="admin-release-inspector"
      aria-labelledby="release-readiness-title"
    >
      <AsyncState state={state} empty="No readiness report was returned.">
        {(readiness) => (
          <>
            <Card>
              <div className="admin-release-readiness__head">
                <div>
                  <p className="admin-metric-label">Exact target</p>
                  <h2 id="release-readiness-title">{readiness.version}</h2>
                  <p>Status: {readiness.status}</p>
                </div>
                <Badge tone={readiness.ready ? "canonical" : "danger"}>
                  {readiness.ready ? "Ready" : "Not ready"}
                </Badge>
              </div>
              <div className="admin-release-metrics">
                <div>
                  <span>Artifacts</span>
                  <strong>{readiness.artifactCount.toLocaleString()}</strong>
                </div>
                <div>
                  <span>Recorded bytes</span>
                  <strong>{readiness.totalBytes.toLocaleString()}</strong>
                </div>
                <div>
                  <span>Bytes verified</span>
                  <strong>{readiness.bytesVerified ? "Yes" : "No"}</strong>
                </div>
              </div>
              <ol className="admin-readiness-checks">
                {readiness.checks.map((check) => (
                  <li key={check.name}>
                    <span
                      className={`admin-status-dot admin-status-dot--${check.passed ? "succeeded" : "failed"}`}
                    />
                    <div>
                      <strong>{check.name}</strong>
                      <p>{check.detail}</p>
                    </div>
                    <Badge tone={check.passed ? "canonical" : "danger"}>
                      {check.passed ? "Passed" : "Failed"}
                    </Badge>
                  </li>
                ))}
              </ol>
            </Card>
            {allowActions ? (
              <Card className="admin-release-actions">
                <div>
                  <ShieldAlert size={20} />
                  <div>
                    <h3>Atomic release workflow</h3>
                    <p>
                      Stages advance exactly draft → validation → review →
                      approved. Publication and restoration additionally require
                      the exact version name.
                    </p>
                  </div>
                </div>
                <div
                  className="admin-stage-track"
                  aria-label="Release workflow"
                >
                  {["draft", "validation", "review", "approved"].map(
                    (stage) => (
                      <span
                        className={release.status === stage ? "is-current" : ""}
                        key={stage}
                      >
                        {stage}
                      </span>
                    ),
                  )}
                </div>
                {nextStage || release.status === "rolled_back" ? (
                  <label>
                    Audited reason
                    <textarea
                      value={reason}
                      onChange={(event) => setReason(event.target.value)}
                      placeholder={
                        nextStage
                          ? `Why is ${release.version} ready for ${nextStage}?`
                          : "Why should this historical version become live again?"
                      }
                    />
                  </label>
                ) : null}
                {release.status === "approved" ||
                release.status === "rolled_back" ? (
                  <label>
                    Exact target version
                    <input
                      value={confirmVersion}
                      onChange={(event) =>
                        setConfirmVersion(event.target.value)
                      }
                      placeholder={release.version}
                      autoComplete="off"
                    />
                  </label>
                ) : null}
                {notesRequired && !release.changelog?.trim() ? (
                  <p className="admin-mutation-result admin-mutation-result--error">
                    Release notes must be authored before advancing to{" "}
                    {nextStage}.
                  </p>
                ) : null}
                <div className="admin-release-actions__buttons">
                  {nextStage ? (
                    <RequirePermission permission="release:publish">
                      <button
                        className="gg-button gg-button--primary gg-button--sm"
                        disabled={
                          !reason.trim() ||
                          (notesRequired && !release.changelog?.trim()) ||
                          busy !== null
                        }
                        onClick={() => void mutate("advance")}
                      >
                        {busy === "advance"
                          ? "Advancing…"
                          : `Advance to ${nextStage}`}
                      </button>
                    </RequirePermission>
                  ) : null}
                  {release.status === "approved" ? (
                    <RequirePermission permission="release:publish">
                      <button
                        className="gg-button gg-button--primary gg-button--sm"
                        disabled={
                          !readiness.ready ||
                          confirmVersion !== release.version ||
                          busy !== null
                        }
                        onClick={() => void mutate("publish")}
                      >
                        {busy === "publish"
                          ? "Publishing…"
                          : `Publish ${release.version}`}
                      </button>
                    </RequirePermission>
                  ) : null}
                  {release.status === "rolled_back" ? (
                    <RequirePermission permission="release:rollback">
                      <button
                        className="gg-button gg-button--primary gg-button--sm"
                        disabled={
                          !readiness.ready ||
                          confirmVersion !== release.version ||
                          !reason.trim() ||
                          busy !== null
                        }
                        onClick={() => void mutate("rollback")}
                      >
                        {busy === "rollback"
                          ? "Restoring…"
                          : `Restore ${release.version}`}
                      </button>
                    </RequirePermission>
                  ) : null}
                  {!nextStage &&
                  release.status !== "approved" &&
                  release.status !== "rolled_back" ? (
                    <Badge tone="neutral">
                      No lifecycle action is valid for {release.status}
                    </Badge>
                  ) : null}
                </div>
                {notice ? (
                  <p
                    className="admin-mutation-result admin-mutation-result--success"
                    role="status"
                  >
                    {notice}
                  </p>
                ) : null}
                {error ? (
                  <p
                    className="admin-mutation-result admin-mutation-result--error"
                    role="alert"
                  >
                    {error}
                  </p>
                ) : null}
              </Card>
            ) : null}
          </>
        )}
      </AsyncState>
    </section>
  );
}

function ChangelogEditor({
  release,
  onChanged,
}: {
  release: AdminDatasetRelease;
  onChanged: () => void;
}) {
  const immutable =
    release.status === "published" || release.status === "rolled_back";
  const [changelog, setChangelog] = useState(release.changelog ?? "");
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState("");
  const [error, setError] = useState("");

  async function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setBusy(true);
    setNotice("");
    setError("");
    try {
      const result = await updateAdminDatasetChangelog(
        release.version,
        changelog,
        reason,
      );
      setNotice(
        `Release notes for ${result.version} were recorded with an audit reason. Refreshing…`,
      );
      window.setTimeout(onChanged, 1200);
    } catch (cause) {
      setError(
        `${cause instanceof Error ? cause.message : "The changelog update was refused."} Refresh to reconcile the recorded release state.`,
      );
      window.setTimeout(onChanged, 1800);
    } finally {
      setBusy(false);
    }
  }

  return (
    <Card className="admin-release-actions">
      <div>
        <ShieldAlert size={20} />
        <div>
          <p className="admin-metric-label">{release.version}</p>
          <h3>Audited release notes</h3>
          <p>
            {immutable
              ? "Published and rolled-back release notes are immutable."
              : "Every edit requires an operational reason and is retained in the audit trail."}
          </p>
        </div>
        <Badge tone={immutable ? "neutral" : "reviewed"}>
          {release.status}
        </Badge>
      </div>
      {immutable ? (
        <pre className="admin-audit-payload">
          {release.changelog || "No changelog was recorded."}
        </pre>
      ) : (
        <form onSubmit={(event) => void save(event)}>
          <label>
            Public changelog
            <textarea
              required
              maxLength={20000}
              rows={8}
              value={changelog}
              onChange={(event) => setChangelog(event.target.value)}
              placeholder="Describe verified dataset changes for API consumers."
            />
          </label>
          <p className="admin-field-hint">
            {changelog.length.toLocaleString()} / 20,000 characters
          </p>
          <label>
            Audit reason
            <textarea
              required
              value={reason}
              onChange={(event) => setReason(event.target.value)}
              placeholder="Why are these notes being created or revised?"
            />
          </label>
          <RequirePermission permission="release:publish">
            <button
              className="gg-button gg-button--primary gg-button--sm"
              disabled={busy || !changelog.trim() || !reason.trim()}
            >
              {busy ? "Saving…" : "Save audited notes"}
            </button>
          </RequirePermission>
        </form>
      )}
      {notice ? (
        <p
          className="admin-mutation-result admin-mutation-result--success"
          role="status"
        >
          {notice}
        </p>
      ) : null}
      {error ? (
        <p
          className="admin-mutation-result admin-mutation-result--error"
          role="alert"
        >
          {error}
        </p>
      ) : null}
    </Card>
  );
}

type AuditFilters = {
  actor: string;
  action: string;
  target: string;
  outcome: "__any" | "succeeded" | "failed";
};
export function AuditLog() {
  const [cursor, setCursor] = useState("");
  const [history, setHistory] = useState<string[]>([]);
  const [filters, setFilters] = useState<AuditFilters>({
    actor: "",
    action: "",
    target: "",
    outcome: "__any",
  });
  const [applied, setApplied] = useState(filters);
  const state = useApi(
    (signal) =>
      listAdminAuditLog(
        {
          actor: applied.actor,
          action: applied.action,
          target: applied.target,
          ...(applied.outcome === "__any" ? {} : { outcome: applied.outcome }),
          cursor,
          limit: 20,
        },
        signal,
      ),
    [cursor, applied],
  );
  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setCursor("");
    setHistory([]);
    setApplied(filters);
  }
  return (
    <>
      <PageHeader
        eyebrow="Tamper-evident history"
        title="Audit log"
        lede="Append-only privileged actions with before/after state and chained hashes. Hashes make later tampering detectable; this paged browser does not replace full-chain verification."
      />
      <form className="admin-filter-bar" onSubmit={submit}>
        <label>
          Actor ID
          <input
            value={filters.actor}
            onChange={(event) =>
              setFilters({ ...filters, actor: event.target.value })
            }
          />
        </label>
        <label>
          Action
          <input
            value={filters.action}
            onChange={(event) =>
              setFilters({ ...filters, action: event.target.value })
            }
          />
        </label>
        <label>
          Target ID
          <input
            value={filters.target}
            onChange={(event) =>
              setFilters({ ...filters, target: event.target.value })
            }
          />
        </label>
        <label>
          Outcome
          <Select
            ariaLabel="Outcome"
            value={filters.outcome}
            onValueChange={(outcome) =>
              setFilters({
                ...filters,
                outcome: outcome as AuditFilters["outcome"],
              })
            }
            options={[
              { value: "__any", label: "Any" },
              { value: "succeeded", label: "Succeeded" },
              { value: "failed", label: "Failed" },
            ]}
          />
        </label>
        <button className="gg-button gg-button--primary gg-button--sm">
          Apply filters
        </button>
      </form>
      <Card className="admin-chain-note">
        <Link2 size={18} />
        <div>
          <strong>
            Chain fields are shown, not independently certified here.
          </strong>
          <p>
            Verify the complete ordered log with the server-side audit verifier;
            a single cursor page cannot prove continuity beyond its boundaries.
          </p>
        </div>
      </Card>
      <AsyncState state={state} empty="No audit entries match these filters.">
        {(page) => (
          <>
            <div className="admin-export-bar">
              <span>
                Export this filtered cursor page using support-safe fields only.
              </span>
              <div>
                <button
                  className="gg-button gg-button--ghost gg-button--sm"
                  disabled={!page.data.length}
                  onClick={() => exportAudit(page.data, "csv")}
                >
                  <Download size={14} /> CSV
                </button>
                <button
                  className="gg-button gg-button--ghost gg-button--sm"
                  disabled={!page.data.length}
                  onClick={() => exportAudit(page.data, "json")}
                >
                  <Download size={14} /> JSON
                </button>
              </div>
            </div>
            <div className="admin-audit-list">
              {page.data.map((entry) => (
                <AuditEntryCard entry={entry} key={entry.id} />
              ))}
            </div>
            <CursorPager
              cursor={cursor}
              history={history}
              next={page.meta.nextCursor}
              onMove={(nextCursor, nextHistory) => {
                setCursor(nextCursor);
                setHistory(nextHistory);
              }}
            />
          </>
        )}
      </AsyncState>
    </>
  );
}

function AuditEntryCard({ entry }: { entry: AdminAuditEntry }) {
  const [expanded, setExpanded] = useState(false);
  return (
    <Card>
      <div className="admin-audit-card__head">
        <span
          className={`admin-status-dot admin-status-dot--${entry.outcome}`}
        />
        <div>
          <strong>{entry.action}</strong>
          <p>
            {entry.actor.label || entry.actor.id} →{" "}
            {entry.target.label || entry.target.id}
          </p>
        </div>
        <time dateTime={entry.at}>{new Date(entry.at).toLocaleString()}</time>
        <Badge tone={entry.outcome === "succeeded" ? "canonical" : "danger"}>
          {entry.outcome}
        </Badge>
      </div>
      <dl className="admin-audit-meta">
        <div>
          <dt>Entry</dt>
          <dd>{entry.id}</dd>
        </div>
        <div>
          <dt>Request</dt>
          <dd>{entry.requestId || "Not recorded"}</dd>
        </div>
        <div>
          <dt>Hash</dt>
          <dd title={entry.hash}>{entry.hash || "Not recorded"}</dd>
        </div>
        <div>
          <dt>Previous</dt>
          <dd title={entry.previousHash}>
            {entry.previousHash || "Chain origin / not recorded"}
          </dd>
        </div>
      </dl>
      {entry.before || entry.after || entry.reason || entry.error ? (
        <>
          <button
            className="gg-button gg-button--ghost gg-button--sm"
            onClick={() => setExpanded(!expanded)}
          >
            {expanded ? "Hide recorded change" : "View recorded change"}
          </button>
          {expanded ? (
            <pre className="admin-audit-payload">
              {JSON.stringify(
                {
                  before: entry.before,
                  after: entry.after,
                  reason: entry.reason,
                  error: entry.error,
                },
                null,
                2,
              )}
            </pre>
          ) : null}
        </>
      ) : null}
    </Card>
  );
}

function exportAudit(entries: AdminAuditEntry[], format: "csv" | "json") {
  const safe = entries.map((entry) => ({
    id: entry.id,
    at: entry.at,
    actorKind: entry.actor.kind,
    actorId: entry.actor.id,
    actorLabel: entry.actor.label,
    action: entry.action,
    targetKind: entry.target.kind,
    targetId: entry.target.id,
    targetLabel: entry.target.label,
    outcome: entry.outcome,
    requestId: entry.requestId ?? "",
    hash: entry.hash,
    previousHash: entry.previousHash ?? "",
  }));
  const csv = [
    Object.keys(safe[0] ?? {}).join(","),
    ...safe.map((row) =>
      Object.values(row)
        .map((value) => `"${String(value).replaceAll('"', '""')}"`)
        .join(","),
    ),
  ].join("\n");
  const blob = new Blob(
    [format === "json" ? JSON.stringify(safe, null, 2) : csv],
    { type: format === "json" ? "application/json" : "text/csv" },
  );
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = `ghanageo-audit-page.${format}`;
  anchor.click();
  URL.revokeObjectURL(url);
}
