"use client";

import { useState, type FormEvent } from "react";
import { useRouter } from "next/navigation";
import { Badge, Card, EmptyState, Field, Select } from "@ghanageo/ui";
import {
  FileDiff,
  Eye,
  MessageSquareText,
  Plus,
  RefreshCw,
  ShieldCheck,
} from "lucide-react";
import { AsyncState, useApi } from "@/components/data";
import { PageHeader } from "@/components/screen";
import { RequirePermission, useSession } from "@/components/session";
import { TableActionLink, TableActions } from "@/components/table-actions";
import {
  AdminApiError,
  commentAdminChangeRequest,
  createAdminChangeRequest,
  getAdminChangeRequest,
  listAdminChangeRequests,
  reviseAdminChangeRequest,
  transitionAdminChangeRequest,
  type AdminChangeRequest,
  type ChangeEvidence,
} from "@/lib/admin-api";

type Mode = "queue" | "submissions" | "drafts" | "evidence";
const copy: Record<Mode, [string, string]> = {
  queue: [
    "Review queue",
    "Versioned proposals awaiting or undergoing independent review.",
  ],
  submissions: [
    "My submissions",
    "Proposals submitted by the signed-in steward, with their live workflow state.",
  ],
  drafts: [
    "Changes requested",
    "Proposals returned to their owner for an evidence-backed revision.",
  ],
  evidence: [
    "Review evidence",
    "Proposal snapshots, digests and durable evidence references available for inspection.",
  ],
};

export function ReviewWorkspace({ mode = "queue" }: { mode?: Mode }) {
  const router = useRouter();
  const { session } = useSession();
  const [cursor, setCursor] = useState("");
  const [history, setHistory] = useState<string[]>([]);
  const [stateFilter, setStateFilter] = useState(
    mode === "drafts" ? "changes_requested" : "all",
  );
  const [refreshKey, setRefreshKey] = useState(0);
  const [creating, setCreating] = useState(false);
  const state = useApi(
    (signal) =>
      listAdminChangeRequests(
        {
          cursor,
          limit: 20,
          state: stateFilter === "all" ? "" : stateFilter,
          ...(mode === "submissions" && session?.accountId
            ? { submitterId: session.accountId }
            : {}),
        },
        signal,
      ),
    [cursor, stateFilter, mode, session?.accountId, refreshKey],
  );
  return (
    <>
      <PageHeader
        eyebrow="Moderated canonical change"
        title={copy[mode][0]}
        lede={copy[mode][1]}
        actions={
          <>
            <button
              className="gg-button gg-button--ghost gg-button--sm"
              onClick={() => setRefreshKey((v) => v + 1)}
            >
              <RefreshCw size={15} /> Refresh
            </button>
            <RequirePermission permission="change:propose">
              <button
                className="gg-button gg-button--primary gg-button--sm"
                onClick={() => setCreating((v) => !v)}
              >
                <Plus size={15} /> New proposal
              </button>
            </RequirePermission>
          </>
        }
      />
      {creating ? (
        <CreateProposal
          onCreated={(id) =>
            router.push(`/review/change-requests/${encodeURIComponent(id)}`)
          }
        />
      ) : null}
      <div className="admin-filter-bar">
        <Field label="Workflow state">
          <Select
            value={stateFilter}
            ariaLabel="Workflow state"
            onValueChange={(value) => {
              setStateFilter(value);
              setCursor("");
              setHistory([]);
            }}
            options={[
              { value: "all", label: "All accessible" },
              { value: "submitted", label: "Submitted" },
              { value: "in_review", label: "In review" },
              { value: "changes_requested", label: "Changes requested" },
              { value: "approved", label: "Approved" },
              { value: "rejected", label: "Rejected" },
            ]}
          />
        </Field>
      </div>
      <AsyncState state={state} empty="No change requests were returned.">
        {(page) =>
          page.data.length ? (
            <>
              <Card style={{ padding: 0, overflow: "hidden" }}>
                <div style={{ overflowX: "auto" }}>
                  <table
                    className="gg-table"
                    style={{ width: "100%", minWidth: 850 }}
                  >
                    <thead>
                      <tr>
                        <th>Target</th>
                        <th>State</th>
                        <th>Owner</th>
                        <th>Reviewer</th>
                        <th>Version</th>
                        <th>Updated</th>
                        <th>
                          <span className="sr-only">Actions</span>
                        </th>
                      </tr>
                    </thead>
                    <tbody>
                      {page.data.map((request) => (
                        <tr key={request.id}>
                          <td>
                            <a
                              href={`/review/change-requests/${encodeURIComponent(request.id)}`}
                            >
                              <strong>
                                {request.target.kind} · {request.target.id}
                              </strong>
                            </a>
                            <small>{request.id}</small>
                          </td>
                          <td>
                            <Badge
                              tone={
                                request.state === "approved"
                                  ? "canonical"
                                  : request.state === "rejected"
                                    ? "danger"
                                    : "reference"
                              }
                            >
                              {request.state}
                            </Badge>
                          </td>
                          <td>{request.submitterId}</td>
                          <td>{request.reviewerId || "Unassigned"}</td>
                          <td>{request.version}</td>
                          <td>
                            {new Date(request.updatedAt).toLocaleString()}
                          </td>
                          <td>
                            <TableActions
                              label={`Actions for request ${request.id}`}
                            >
                              <TableActionLink
                                href={`/review/change-requests/${encodeURIComponent(request.id)}`}
                                label="View change request"
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
              <nav className="admin-pager">
                <button
                  className="gg-button gg-button--ghost gg-button--sm"
                  disabled={!history.length}
                  onClick={() => {
                    setCursor(history.at(-1) ?? "");
                    setHistory((h) => h.slice(0, -1));
                  }}
                >
                  Previous
                </button>
                <span>Page {history.length + 1}</span>
                <button
                  className="gg-button gg-button--ghost gg-button--sm"
                  disabled={!page.nextCursor}
                  onClick={() => {
                    setHistory((h) => [...h, cursor]);
                    setCursor(page.nextCursor ?? "");
                  }}
                >
                  Next
                </button>
              </nav>
            </>
          ) : (
            <EmptyState
              title="No change requests"
              description="No proposals match this workflow view. Change the state filter or create a new proposal."
            />
          )
        }
      </AsyncState>
    </>
  );
}

function CreateProposal({ onCreated }: { onCreated: (id: string) => void }) {
  const [targetKind, setTargetKind] = useState("place");
  const [targetId, setTargetId] = useState("");
  const [patch, setPatch] = useState("{}");
  const [before, setBefore] = useState("{}");
  const [after, setAfter] = useState("{}");
  const [evidence, setEvidence] = useState("{}");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  async function submit(event: FormEvent) {
    event.preventDefault();
    setError(null);
    setBusy(true);
    try {
      const created = await createAdminChangeRequest(
        {
          target: { kind: targetKind.trim(), id: targetId.trim() },
          proposedPatch: parseObject(patch, "patch"),
          before: parseObject(before, "before snapshot"),
          after: parseObject(after, "after snapshot"),
          evidence: parseObject(evidence, "evidence") as ChangeEvidence,
        },
        crypto.randomUUID(),
      );
      onCreated(created.id);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Proposal creation failed.",
      );
    } finally {
      setBusy(false);
    }
  }
  return (
    <RequirePermission permission="change:propose">
      <Card className="admin-release-actions">
        <div>
          <Plus size={20} />
          <div>
            <h3>Create a change request</h3>
            <p>
              Snapshots and the proposed patch must agree. Evidence references
              are stored with the immutable revision.
            </p>
          </div>
        </div>
        <form className="admin-review-form" onSubmit={(e) => void submit(e)}>
          <div className="admin-form-grid">
            <label>
              Target kind
              <input
                value={targetKind}
                onChange={(e) => setTargetKind(e.target.value)}
                required
              />
            </label>
            <label>
              Target ID
              <input
                value={targetId}
                onChange={(e) => setTargetId(e.target.value)}
                required
              />
            </label>
          </div>
          <JsonField
            label="Proposed merge patch"
            value={patch}
            onChange={setPatch}
          />
          <div className="admin-review-diff">
            <JsonField
              label="Before snapshot"
              value={before}
              onChange={setBefore}
            />
            <JsonField
              label="After snapshot"
              value={after}
              onChange={setAfter}
            />
          </div>
          <JsonField
            label="Evidence object"
            value={evidence}
            onChange={setEvidence}
          />
          <button
            className="gg-button gg-button--primary"
            disabled={busy || !targetId.trim()}
          >
            {busy ? "Submitting…" : "Submit proposal"}
          </button>
          {error ? (
            <p
              className="admin-mutation-result admin-mutation-result--error"
              role="alert"
            >
              {error}
            </p>
          ) : null}
        </form>
      </Card>
    </RequirePermission>
  );
}

export function ChangeRequestDetail({ id }: { id: string }) {
  const { session, can } = useSession();
  const [refreshKey, setRefreshKey] = useState(0);
  const state = useApi(
    (signal) => getAdminChangeRequest(id, signal),
    [id, refreshKey],
  );
  return (
    <>
      <PageHeader
        eyebrow="CAS-protected review"
        title="Change request"
        lede="Every mutation uses the displayed version and a unique idempotency key. Conflicts refresh server truth before another attempt."
        actions={
          <button
            className="gg-button gg-button--ghost gg-button--sm"
            onClick={() => setRefreshKey((v) => v + 1)}
          >
            <RefreshCw size={15} /> Refresh
          </button>
        }
      />
      <AsyncState state={state} empty="No such change request was returned.">
        {(request) => (
          <>
            <Card>
              <div className="admin-run-detail__head">
                <div>
                  <p className="admin-metric-label">{request.target.kind}</p>
                  <h2>{request.target.id}</h2>
                  <code>{request.id}</code>
                </div>
                <div>
                  <Badge
                    tone={
                      request.state === "approved"
                        ? "canonical"
                        : request.state === "rejected"
                          ? "danger"
                          : "reference"
                    }
                  >
                    {request.state}
                  </Badge>
                  <p>Version {request.version}</p>
                </div>
              </div>
              <dl className="admin-detail-grid">
                <div>
                  <dt>Proposer</dt>
                  <dd>{request.submitterId}</dd>
                </div>
                <div>
                  <dt>Reviewer</dt>
                  <dd>{request.reviewerId || "Unassigned"}</dd>
                </div>
                <div>
                  <dt>Snapshot digest</dt>
                  <dd className="admin-break-value">
                    {request.snapshot.digest}
                  </dd>
                </div>
              </dl>
            </Card>
            <SnapshotDiff request={request} />
            <EvidencePanel request={request} />
            <ReviewActions
              request={request}
              isOwner={session?.accountId === request.submitterId}
              canReview={can("change:review")}
              onChanged={() => setRefreshKey((v) => v + 1)}
            />
            <ReviewTimeline request={request} />
          </>
        )}
      </AsyncState>
    </>
  );
}

function SnapshotDiff({ request }: { request: AdminChangeRequest }) {
  return (
    <section>
      <div className="admin-section-heading">
        <FileDiff size={18} />
        <h2>Before / after</h2>
      </div>
      <div className="admin-review-diff">
        <Card>
          <p className="admin-metric-label">Before</p>
          <pre>{JSON.stringify(request.snapshot.before, null, 2)}</pre>
        </Card>
        <Card>
          <p className="admin-metric-label">After</p>
          <pre>{JSON.stringify(request.snapshot.after, null, 2)}</pre>
        </Card>
      </div>
      <Card>
        <p className="admin-metric-label">Proposed merge patch</p>
        <pre>{JSON.stringify(request.proposedPatch, null, 2)}</pre>
      </Card>
    </section>
  );
}
function EvidencePanel({ request }: { request: AdminChangeRequest }) {
  const entries = Object.entries(request.evidence).filter(([, value]) =>
    Array.isArray(value) ? value.length : Boolean(value),
  );
  return (
    <Card>
      <div className="admin-section-heading">
        <ShieldCheck size={18} />
        <h2>Evidence</h2>
      </div>
      {entries.length ? (
        <dl className="admin-detail-grid">
          {entries.map(([key, value]) => (
            <div key={key}>
              <dt>{key}</dt>
              <dd>
                {Array.isArray(value) ? value.join(" · ") : String(value)}
              </dd>
            </div>
          ))}
        </dl>
      ) : (
        <p>No evidence references were attached.</p>
      )}
    </Card>
  );
}

function ReviewActions({
  request,
  isOwner,
  canReview,
  onChanged,
}: {
  request: AdminChangeRequest;
  isOwner: boolean;
  canReview: boolean;
  onChanged: () => void;
}) {
  const [comment, setComment] = useState("");
  const [patch, setPatch] = useState(
    JSON.stringify(request.proposedPatch, null, 2),
  );
  const [after, setAfter] = useState(
    JSON.stringify(request.snapshot.after, null, 2),
  );
  const [evidence, setEvidence] = useState(
    JSON.stringify(request.evidence, null, 2),
  );
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState<string | null>(null);
  async function run(
    action: "transition" | "comment" | "revise",
    nextState?: string,
  ) {
    setBusy(true);
    setNotice(null);
    try {
      if (action === "transition")
        await transitionAdminChangeRequest(
          request.id,
          request.version,
          nextState!,
          comment.trim(),
          crypto.randomUUID(),
        );
      else if (action === "comment")
        await commentAdminChangeRequest(
          request.id,
          request.version,
          comment.trim(),
          crypto.randomUUID(),
        );
      else
        await reviseAdminChangeRequest(
          request.id,
          request.version,
          parseObject(patch, "patch"),
          parseObject(after, "after snapshot"),
          parseObject(evidence, "evidence") as ChangeEvidence,
          comment.trim(),
          crypto.randomUUID(),
        );
      setComment("");
      onChanged();
    } catch (error) {
      if (error instanceof AdminApiError && error.code === "CONFLICT") {
        setNotice(
          "This request changed on the server. The latest version is being loaded; review it before trying again.",
        );
        onChanged();
      } else
        setNotice(error instanceof Error ? error.message : "Mutation failed.");
    } finally {
      setBusy(false);
    }
  }
  const terminal = request.state === "approved" || request.state === "rejected";
  return (
    <Card className="admin-release-actions">
      <div>
        <MessageSquareText size={20} />
        <div>
          <h3>Workflow actions</h3>
          <p>
            Ownership, assignment and valid transitions are rechecked by the
            API.
          </p>
        </div>
      </div>
      {isOwner && request.state === "changes_requested" ? (
        <div className="admin-review-form">
          <JsonField
            label="Revised merge patch"
            value={patch}
            onChange={setPatch}
          />
          <JsonField
            label="Revised after snapshot"
            value={after}
            onChange={setAfter}
          />
          <JsonField
            label="Revised evidence"
            value={evidence}
            onChange={setEvidence}
          />
        </div>
      ) : null}
      <label>
        Comment
        <textarea
          value={comment}
          onChange={(e) => setComment(e.target.value)}
          placeholder="Required for decisions and revisions"
        />
      </label>
      <div className="admin-release-actions__buttons">
        {isOwner && request.state === "changes_requested" ? (
          <button
            className="gg-button gg-button--primary gg-button--sm"
            disabled={busy || !comment.trim()}
            onClick={() => void run("revise")}
          >
            Append revision & resubmit
          </button>
        ) : null}
        {canReview && request.state === "submitted" ? (
          <button
            className="gg-button gg-button--primary gg-button--sm"
            disabled={busy}
            onClick={() => void run("transition", "in_review")}
          >
            Start review
          </button>
        ) : null}
        {canReview && request.state === "in_review" ? (
          <>
            {["changes_requested", "approved", "rejected"].map((next) => (
              <button
                key={next}
                className="gg-button gg-button--ghost gg-button--sm"
                disabled={busy || !comment.trim()}
                onClick={() => void run("transition", next)}
              >
                {next.replace("_", " ")}
              </button>
            ))}
          </>
        ) : null}
        {canReview && !terminal ? (
          <button
            className="gg-button gg-button--ghost gg-button--sm"
            disabled={busy || !comment.trim()}
            onClick={() => void run("comment")}
          >
            Add reviewer comment
          </button>
        ) : null}
      </div>
      {notice ? (
        <p
          className="admin-mutation-result admin-mutation-result--error"
          role="alert"
        >
          {notice}
        </p>
      ) : null}
    </Card>
  );
}
function ReviewTimeline({ request }: { request: AdminChangeRequest }) {
  return (
    <div className="admin-review-timeline">
      <Card>
        <h2>State history</h2>
        {request.history.length ? (
          <ol>
            {request.history.map((item) => (
              <li key={item.id}>
                <strong>
                  {item.from ? `${item.from} → ` : ""}
                  {item.to}
                </strong>
                <span>
                  {item.actorId} · {new Date(item.createdAt).toLocaleString()}
                </span>
                {item.comment ? <p>{item.comment}</p> : null}
              </li>
            ))}
          </ol>
        ) : (
          <EmptyState
            title="No state history"
            description="Workflow transitions will appear here as this request moves through review."
          />
        )}
      </Card>
      <Card>
        <h2>Reviewer comments</h2>
        {request.comments.length ? (
          <ol>
            {request.comments.map((item) => (
              <li key={item.id}>
                <strong>{item.authorId}</strong>
                <span>{new Date(item.createdAt).toLocaleString()}</span>
                <p>{item.body}</p>
              </li>
            ))}
          </ol>
        ) : (
          <EmptyState
            title="No reviewer comments"
            description="Comments from reviewers will appear here when the discussion begins."
          />
        )}
      </Card>
    </div>
  );
}
function JsonField({
  label,
  value,
  onChange,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
}) {
  return (
    <label>
      {label}
      <textarea
        className="admin-json-input"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        spellCheck={false}
        required
      />
    </label>
  );
}
function parseObject(value: string, label: string): Record<string, unknown> {
  const parsed: unknown = JSON.parse(value);
  if (!parsed || Array.isArray(parsed) || typeof parsed !== "object")
    throw new Error(`${label} must be a JSON object.`);
  return parsed as Record<string, unknown>;
}
