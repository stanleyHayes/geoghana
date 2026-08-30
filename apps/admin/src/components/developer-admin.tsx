"use client";

import { useState } from "react";
import { Badge, Card, EmptyState, Field, Select } from "@ghanageo/ui";
import {
  Activity,
  Copy,
  Eye,
  KeyRound,
  RefreshCw,
  Search,
  Settings2,
  ShieldAlert,
  X,
} from "lucide-react";
import { AsyncState, useApi } from "@/components/data";
import { PageHeader } from "@/components/screen";
import { RequirePermission } from "@/components/session";
import { TableActionButton, TableActions } from "@/components/table-actions";
import {
  getDeveloperUsage,
  listDeveloperAccounts,
  listDeveloperApplications,
  listDeveloperKeys,
  listDeveloperOrganizations,
  listDeveloperRequests,
  mutateDeveloperKey,
  type DeveloperKey,
  type DeveloperListPage,
} from "@/lib/admin-api";

type Directory = "organizations" | "accounts" | "applications" | "keys";
const titles: Record<Directory, [string, string]> = {
  organizations: [
    "Developer organizations",
    "Support-safe organization ownership and integration counts.",
  ],
  accounts: [
    "Developer users",
    "Redacted developer identities and account state.",
  ],
  applications: [
    "Developer applications",
    "Registered integrations, environments and public origin metadata.",
  ],
  keys: [
    "API keys",
    "Secret-free prefixes, scopes, usage recency and lifecycle state.",
  ],
};

export function DeveloperDirectory({ resource }: { resource: Directory }) {
  const [q, setQ] = useState("");
  const [query, setQuery] = useState("");
  const [organizationId, setOrganizationId] = useState("");
  const [applicationId, setApplicationId] = useState("");
  const [keyState, setKeyState] = useState("all");
  const [cursor, setCursor] = useState("");
  const [history, setHistory] = useState<string[]>([]);
  const [refreshKey, setRefreshKey] = useState(0);
  const [selectedKey, setSelectedKey] = useState<DeveloperKey | null>(null);
  const [selectedRecord, setSelectedRecord] = useState<Record<
    string,
    unknown
  > | null>(null);
  const state = useApi(
    (signal) =>
      loadDirectory(
        resource,
        { q: query, cursor, organizationId, applicationId, state: keyState },
        signal,
      ),
    [
      resource,
      query,
      cursor,
      organizationId,
      applicationId,
      keyState,
      refreshKey,
    ],
  );
  function resetPage() {
    setCursor("");
    setHistory([]);
  }
  return (
    <RequirePermission permission="organization:view">
      <PageHeader
        eyebrow="Developer support"
        title={titles[resource][0]}
        lede={titles[resource][1]}
        actions={
          <button
            className="gg-button gg-button--ghost gg-button--sm"
            onClick={() => setRefreshKey((v) => v + 1)}
          >
            <RefreshCw size={15} /> Refresh
          </button>
        }
      />
      <form
        className="admin-filter-bar"
        onSubmit={(e) => {
          e.preventDefault();
          setQuery(q.trim());
          resetPage();
        }}
      >
        <label>
          Search
          <input
            value={q}
            onChange={(e) => setQ(e.target.value)}
            placeholder="Name or safe identifier"
          />
        </label>
        {resource === "applications" || resource === "keys" ? (
          <label>
            Organization ID
            <input
              value={organizationId}
              onChange={(e) => {
                setOrganizationId(e.target.value);
                resetPage();
              }}
            />
          </label>
        ) : null}
        {resource === "keys" ? (
          <>
            <label>
              Application ID
              <input
                value={applicationId}
                onChange={(e) => {
                  setApplicationId(e.target.value);
                  resetPage();
                }}
              />
            </label>
            <Field label="Key state">
              <Select
                value={keyState}
                ariaLabel="Key state"
                onValueChange={(value) => {
                  setKeyState(value);
                  resetPage();
                }}
                options={[
                  { value: "all", label: "All states" },
                  { value: "active", label: "Active" },
                  { value: "suspended", label: "Suspended" },
                  { value: "revoked", label: "Revoked" },
                ]}
              />
            </Field>
          </>
        ) : null}
        <button className="gg-button gg-button--primary gg-button--sm">
          <Search size={15} /> Search
        </button>
      </form>
      <AsyncState state={state} empty="No developer records were returned.">
        {(page) =>
          page.data.length ? (
            <>
              <Card style={{ padding: 0, overflow: "hidden" }}>
                <div style={{ overflowX: "auto" }}>
                  <DirectoryTable
                    resource={resource}
                    rows={page.data}
                    onKey={setSelectedKey}
                    onInspect={setSelectedRecord}
                  />
                </div>
              </Card>
              <nav className="admin-pager">
                <button
                  className="gg-button gg-button--ghost gg-button--sm"
                  disabled={!history.length}
                  onClick={() => {
                    setCursor(history.at(-1) ?? "");
                    setHistory((v) => v.slice(0, -1));
                  }}
                >
                  Previous
                </button>
                <span>
                  {page.meta.total.toLocaleString()} records · page{" "}
                  {history.length + 1}
                </span>
                <button
                  className="gg-button gg-button--ghost gg-button--sm"
                  disabled={!page.meta.nextCursor}
                  onClick={() => {
                    setHistory((v) => [...v, cursor]);
                    setCursor(page.meta.nextCursor ?? "");
                  }}
                >
                  Next
                </button>
              </nav>
            </>
          ) : (
            <EmptyState
              title="No developer records"
              description="No records match the current search and access filters."
            />
          )
        }
      </AsyncState>
      {selectedRecord ? (
        <DirectoryDetail
          record={selectedRecord}
          onClose={() => setSelectedRecord(null)}
        />
      ) : null}
      {selectedKey ? (
        <KeyAction
          keyRecord={selectedKey}
          onClose={() => setSelectedKey(null)}
          onChanged={() => {
            setSelectedKey(null);
            setRefreshKey((v) => v + 1);
          }}
        />
      ) : null}
    </RequirePermission>
  );
}

async function loadDirectory(
  resource: Directory,
  filters: {
    q: string;
    cursor: string;
    organizationId: string;
    applicationId: string;
    state: string;
  },
  signal: AbortSignal,
): Promise<DeveloperListPage<unknown>> {
  const base = { q: filters.q, cursor: filters.cursor, limit: 20 };
  if (resource === "organizations")
    return listDeveloperOrganizations(base, signal);
  if (resource === "accounts") return listDeveloperAccounts(base, signal);
  if (resource === "applications")
    return listDeveloperApplications(
      { ...base, organizationId: filters.organizationId },
      signal,
    );
  return listDeveloperKeys(
    {
      ...base,
      organizationId: filters.organizationId,
      applicationId: filters.applicationId,
      state: filters.state === "all" ? "" : filters.state,
    },
    signal,
  );
}

function DirectoryTable({
  resource,
  rows,
  onKey,
  onInspect,
}: {
  resource: Directory;
  rows: unknown[];
  onKey: (key: DeveloperKey) => void;
  onInspect: (record: Record<string, unknown>) => void;
}) {
  if (resource === "organizations")
    return (
      <table className="gg-table admin-developer-table">
        <thead>
          <tr>
            <th>Organization</th>
            <th>Owner reference</th>
            <th>Members</th>
            <th>Member roles</th>
            <th>Created</th>
            <th>
              <span className="sr-only">Actions</span>
            </th>
          </tr>
        </thead>
        <tbody>
          {rows.map((raw) => {
            const row = raw as Awaited<
              ReturnType<typeof listDeveloperOrganizations>
            >["data"][number];
            return (
              <tr key={row.id}>
                <td>
                  <strong>{row.name}</strong>
                  <small>{row.id}</small>
                </td>
                <td>{row.ownerId}</td>
                <td>{row.members.length.toLocaleString()}</td>
                <td>
                  {[...new Set(row.members.map((member) => member.role))].join(
                    " · ",
                  ) || "None"}
                </td>
                <td>{formatDate(row.createdAt)}</td>
                <td>
                  <TableActions label={`Actions for ${row.name}`}>
                    <TableActionButton
                      label={`View ${row.name}`}
                      onClick={() =>
                        onInspect(row as unknown as Record<string, unknown>)
                      }
                    >
                      <Eye />
                    </TableActionButton>
                  </TableActions>
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    );
  if (resource === "accounts")
    return (
      <table className="gg-table admin-developer-table">
        <thead>
          <tr>
            <th>Developer</th>
            <th>Role</th>
            <th>Status</th>
            <th>Email</th>
            <th>Updated</th>
            <th>
              <span className="sr-only">Actions</span>
            </th>
          </tr>
        </thead>
        <tbody>
          {rows.map((raw) => {
            const row = raw as Awaited<
              ReturnType<typeof listDeveloperAccounts>
            >["data"][number];
            return (
              <tr key={row.id}>
                <td>
                  <strong>{redactEmail(row.email)}</strong>
                  <small>{row.id}</small>
                </td>
                <td>{row.role}</td>
                <td>
                  <Badge tone={!row.disabled ? "canonical" : "reference"}>
                    {row.disabled ? "disabled" : "active"}
                  </Badge>
                </td>
                <td>{row.emailVerified ? "Verified" : "Unverified"}</td>
                <td>{formatDate(row.updatedAt)}</td>
                <td>
                  <TableActions label={`Actions for ${redactEmail(row.email)}`}>
                    <TableActionButton
                      label="View developer details"
                      onClick={() =>
                        onInspect(row as unknown as Record<string, unknown>)
                      }
                    >
                      <Eye />
                    </TableActionButton>
                  </TableActions>
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    );
  if (resource === "applications")
    return (
      <table className="gg-table admin-developer-table">
        <thead>
          <tr>
            <th>Application</th>
            <th>Organization</th>
            <th>Environments</th>
            <th>Domains</th>
            <th>Callback</th>
            <th>Created</th>
            <th>
              <span className="sr-only">Actions</span>
            </th>
          </tr>
        </thead>
        <tbody>
          {rows.map((raw) => {
            const row = raw as Awaited<
              ReturnType<typeof listDeveloperApplications>
            >["data"][number];
            return (
              <tr key={row.id}>
                <td>
                  <strong>{row.name}</strong>
                  <small>{row.id}</small>
                </td>
                <td>{row.organizationId}</td>
                <td>{row.environments?.join(" · ") || "Not recorded"}</td>
                <td>{row.domains?.join(" · ") || "Not recorded"}</td>
                <td>{row.callbackUrl || "Not recorded"}</td>
                <td>{formatDate(row.createdAt)}</td>
                <td>
                  <TableActions label={`Actions for ${row.name}`}>
                    <TableActionButton
                      label={`View ${row.name}`}
                      onClick={() =>
                        onInspect(row as unknown as Record<string, unknown>)
                      }
                    >
                      <Eye />
                    </TableActionButton>
                  </TableActions>
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    );
  return (
    <table className="gg-table admin-developer-table">
      <thead>
        <tr>
          <th>Key</th>
          <th>Application</th>
          <th>Environment</th>
          <th>State</th>
          <th>Scopes</th>
          <th>Last used</th>
          <th>
            <span className="sr-only">Actions</span>
          </th>
        </tr>
      </thead>
      <tbody>
        {rows.map((raw) => {
          const row = raw as DeveloperKey;
          return (
            <tr key={row.id}>
              <td>
                <strong>{row.name}</strong>
                <small>
                  <code>{row.prefix}</code> · {row.id}
                </small>
              </td>
              <td>
                {row.applicationId}
                <small>{row.organizationId}</small>
              </td>
              <td>
                {row.environment} · {row.class}
              </td>
              <td>
                <Badge
                  tone={
                    row.state === "active"
                      ? "canonical"
                      : row.state === "revoked"
                        ? "danger"
                        : "reference"
                  }
                >
                  {row.state}
                </Badge>
              </td>
              <td>{row.scopes.join(" · ") || "None"}</td>
              <td>{formatDate(row.lastUsedAt)}</td>
              <td>
                <TableActions label={`Actions for ${row.name}`}>
                  <TableActionButton
                    label={`View ${row.name}`}
                    onClick={() =>
                      onInspect(row as unknown as Record<string, unknown>)
                    }
                  >
                    <Eye />
                  </TableActionButton>
                  {row.state !== "revoked" ? (
                    <RequirePermission permission="key:suspend">
                      <TableActionButton
                        label={`Manage ${row.name}`}
                        onClick={() => onKey(row)}
                        tone="danger"
                      >
                        <Settings2 />
                      </TableActionButton>
                    </RequirePermission>
                  ) : null}
                </TableActions>
              </td>
            </tr>
          );
        })}
      </tbody>
    </table>
  );
}

function DirectoryDetail({
  record,
  onClose,
}: {
  record: Record<string, unknown>;
  onClose: () => void;
}) {
  const safeRecord = Object.fromEntries(
    Object.entries(record).filter(
      ([key]) => !/secret|password|token|hash/i.test(key),
    ),
  );
  return (
    <Card className="admin-record-detail">
      <div className="admin-section-heading">
        <Eye size={18} />
        <h2>Record details</h2>
        <TableActions>
          <TableActionButton label="Close details" onClick={onClose}>
            <X />
          </TableActionButton>
        </TableActions>
      </div>
      <pre className="admin-audit-payload">
        {JSON.stringify(safeRecord, null, 2)}
      </pre>
    </Card>
  );
}

function KeyAction({
  keyRecord,
  onClose,
  onChanged,
}: {
  keyRecord: DeveloperKey;
  onClose: () => void;
  onChanged: () => void;
}) {
  const [action, setAction] = useState<"suspend" | "revoke">("suspend");
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  async function submit() {
    setBusy(true);
    setError(null);
    try {
      await mutateDeveloperKey(keyRecord.id, action, reason.trim());
      onChanged();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Key mutation failed.");
    } finally {
      setBusy(false);
    }
  }
  return (
    <RequirePermission permission="key:suspend">
      <Card className="admin-release-actions">
        <div>
          <ShieldAlert size={20} />
          <div>
            <h3>Change key lifecycle state</h3>
            <p>
              Only the public prefix <code>{keyRecord.prefix}</code> is visible.
              Key secrets and raw credentials are never exposed.
            </p>
          </div>
        </div>
        <div className="admin-form-grid">
          <Field label="Action">
            <Select
              value={action}
              ariaLabel="Key lifecycle action"
              onValueChange={(value) =>
                setAction(value as "suspend" | "revoke")
              }
              options={[
                {
                  value: "suspend",
                  label: "Suspend temporarily",
                  hint: "Can be restored later",
                },
                {
                  value: "revoke",
                  label: "Revoke permanently",
                  hint: "Cannot be undone",
                },
              ]}
            />
          </Field>
          <label>
            Exact key ID
            <input value={keyRecord.id} disabled />
          </label>
        </div>
        <label>
          Mandatory support reason
          <textarea
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            maxLength={1000}
            placeholder="Document the incident or support case"
            required
          />
        </label>
        <div className="admin-release-actions__buttons">
          <button
            className="gg-button gg-button--primary gg-button--sm"
            disabled={busy || !reason.trim()}
            onClick={() => void submit()}
          >
            {busy
              ? "Applying…"
              : action === "revoke"
                ? "Revoke key"
                : "Suspend key"}
          </button>
          <button
            className="gg-button gg-button--ghost gg-button--sm"
            disabled={busy}
            onClick={onClose}
          >
            Cancel
          </button>
        </div>
        {error ? (
          <p
            className="admin-mutation-result admin-mutation-result--error"
            role="alert"
          >
            {error}
          </p>
        ) : null}
      </Card>
    </RequirePermission>
  );
}

export function DeveloperLogs() {
  const [organizationId, setOrganizationId] = useState("");
  const [applicationId, setApplicationId] = useState("");
  const [selection, setSelection] = useState<{
    organizationId: string;
    applicationId: string;
  } | null>(null);
  return (
    <RequirePermission permission="organization:view">
      <PageHeader
        eyebrow="Secret-free request telemetry"
        title="Application usage & requests"
        lede="Thirty-day aggregates and bounded request metadata. No request bodies, credentials, IP addresses or raw user data are exposed."
      />
      <form
        className="admin-filter-bar"
        onSubmit={(e) => {
          e.preventDefault();
          if (organizationId.trim() && applicationId.trim())
            setSelection({
              organizationId: organizationId.trim(),
              applicationId: applicationId.trim(),
            });
        }}
      >
        <label>
          Organization ID
          <input
            value={organizationId}
            onChange={(e) => setOrganizationId(e.target.value)}
            required
          />
        </label>
        <label>
          Application ID
          <input
            value={applicationId}
            onChange={(e) => setApplicationId(e.target.value)}
            required
          />
        </label>
        <button className="gg-button gg-button--primary gg-button--sm">
          <Activity size={15} /> Load telemetry
        </button>
      </form>
      {selection ? (
        <Telemetry
          key={`${selection.organizationId}:${selection.applicationId}`}
          {...selection}
        />
      ) : (
        <Card className="admin-unavailable">
          <KeyRound size={18} />
          <div>
            <strong>Select an organization and application</strong>
            <p>
              Telemetry is scoped to an exact owned application; there is no
              global raw-log stream.
            </p>
          </div>
        </Card>
      )}
    </RequirePermission>
  );
}

function Telemetry({
  organizationId,
  applicationId,
}: {
  organizationId: string;
  applicationId: string;
}) {
  const [before, setBefore] = useState("");
  const [history, setHistory] = useState<string[]>([]);
  const usage = useApi(
    (signal) =>
      getDeveloperUsage(organizationId, applicationId, undefined, signal),
    [organizationId, applicationId],
  );
  const requests = useApi(
    (signal) =>
      listDeveloperRequests(
        organizationId,
        applicationId,
        { before, limit: 20 },
        signal,
      ),
    [organizationId, applicationId, before],
  );
  return (
    <>
      <AsyncState state={usage} empty="No usage summary was returned.">
        {(data) => (
          <>
            <div className="gg-auto-grid">
              {[
                ["Requests", data.requests],
                ["Errors", data.errors],
                ["Quota cost", data.quotaCost],
                ["Average latency", `${data.avgLatencyMs.toLocaleString()} ms`],
              ].map(([label, value]) => (
                <Card key={label}>
                  <p className="admin-metric-label">{label}</p>
                  <p className="admin-metric-value">
                    {typeof value === "number" ? value.toLocaleString() : value}
                  </p>
                </Card>
              ))}
            </div>
            <Card>
              <p className="admin-form-hint">
                Measured since {formatDate(data.since)}. Breakdowns are
                aggregated and support-safe.
              </p>
              <div className="admin-usage-breakdowns">
                {[
                  ["Protocol", data.byProtocol],
                  ["Endpoint", data.byEndpoint],
                  ["Geography", data.byGeography],
                ].map(([label, values]) => (
                  <div key={label as string}>
                    <strong>{label as string}</strong>
                    {(values as typeof data.byProtocol).length ? (
                      (values as typeof data.byProtocol).map((item) => (
                        <p key={item.label}>
                          {item.label}: {item.requests.toLocaleString()}{" "}
                          requests · {item.errors.toLocaleString()} errors
                        </p>
                      ))
                    ) : (
                      <p>Unavailable</p>
                    )}
                  </div>
                ))}
              </div>
            </Card>
          </>
        )}
      </AsyncState>
      <AsyncState state={requests} empty="No request metadata was returned.">
        {(page) =>
          page.data.length ? (
            <>
              <Card style={{ padding: 0, overflow: "hidden" }}>
                <div style={{ overflowX: "auto" }}>
                  <table className="gg-table admin-developer-table">
                    <thead>
                      <tr>
                        <th>Request</th>
                        <th>Operation</th>
                        <th>Status</th>
                        <th>Latency</th>
                        <th>Quota</th>
                        <th>At</th>
                        <th>
                          <span className="sr-only">Actions</span>
                        </th>
                      </tr>
                    </thead>
                    <tbody>
                      {page.data.map((item) => (
                        <tr key={item.id}>
                          <td>
                            <code>{item.requestId}</code>
                            <small>{item.protocol}</small>
                          </td>
                          <td>
                            {item.operation}
                            <small>
                              {item.geography || "No geography label"}
                            </small>
                          </td>
                          <td>
                            <Badge tone={item.success ? "canonical" : "danger"}>
                              {item.status}
                            </Badge>
                          </td>
                          <td>{item.latencyMs.toLocaleString()} ms</td>
                          <td>
                            {item.quotaCost} cost · {item.quotaRemaining}/
                            {item.quotaLimit}
                          </td>
                          <td>{formatDate(item.at)}</td>
                          <td>
                            <TableActions
                              label={`Actions for request ${item.requestId}`}
                            >
                              <TableActionButton
                                label={`Copy request ID ${item.requestId}`}
                                onClick={() =>
                                  void navigator.clipboard.writeText(
                                    item.requestId,
                                  )
                                }
                              >
                                <Copy />
                              </TableActionButton>
                            </TableActions>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </Card>
              <nav className="admin-pager">
                <button
                  className="gg-button gg-button--ghost gg-button--sm"
                  disabled={!history.length}
                  onClick={() => {
                    setBefore(history.at(-1) ?? "");
                    setHistory((v) => v.slice(0, -1));
                  }}
                >
                  Previous
                </button>
                <span>Page {history.length + 1}</span>
                <button
                  className="gg-button gg-button--ghost gg-button--sm"
                  disabled={!page.meta.nextBefore}
                  onClick={() => {
                    setHistory((v) => [...v, before]);
                    setBefore(page.meta.nextBefore ?? "");
                  }}
                >
                  Next
                </button>
              </nav>
            </>
          ) : (
            <EmptyState
              title="No request activity"
              description="No API requests were recorded for this application during the selected window."
            />
          )
        }
      </AsyncState>
    </>
  );
}
function redactEmail(email: string) {
  const [name = "", domain = ""] = email.split("@");
  return domain ? `${name.slice(0, 2)}•••@${domain}` : "Redacted developer";
}
function formatDate(value?: string) {
  return value ? new Date(value).toLocaleString() : "Not recorded";
}
