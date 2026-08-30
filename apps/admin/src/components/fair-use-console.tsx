"use client";

import { useState, type FormEvent } from "react";
import { Badge, Card, DateTimeInput, Field } from "@ghanageo/ui";
import { Gauge, HeartHandshake, RefreshCw, ShieldCheck } from "lucide-react";
import { AsyncState, useApi } from "@/components/data";
import { PageHeader } from "@/components/screen";
import { RequirePermission } from "@/components/session";
import {
  appendAdminFairUseOverride,
  appendAdminFairUsePolicy,
  getAdminFairUsePolicy,
  type FairUseAllowance,
  type FairUseCostCeilings,
  type FairUsePolicy,
} from "@/lib/admin-api";

const COSTS = ["cheap", "normal", "spatial", "geometry"] as const;
const AUDIENCES = ["anonymous", "authenticated", "sandbox"] as const;
type Audience = (typeof AUDIENCES)[number];
type Notice = { tone: "success" | "error"; text: string } | null;

export function FairUseConsole() {
  const [refreshKey, setRefreshKey] = useState(0);
  const state = useApi((signal) => getAdminFairUsePolicy(signal), [refreshKey]);
  return (
    <>
      <PageHeader
        eyebrow="Free public infrastructure"
        title="Fair-use policy"
        lede="GhanaGeo is free forever. Limits protect shared capacity; donations never buy higher allowances and never influence an application’s treatment."
        actions={
          <button
            className="gg-button gg-button--ghost gg-button--sm"
            onClick={() => setRefreshKey((v) => v + 1)}
          >
            <RefreshCw size={15} /> Refresh live policy
          </button>
        }
      />
      <Card className="admin-fair-use-principle">
        <HeartHandshake size={22} />
        <div>
          <strong>No paid tier. No donation linkage.</strong>
          <p>
            Application-specific overrides are temporary operational exceptions,
            documented and audited independently of donations or sponsorship.
          </p>
        </div>
      </Card>
      <AsyncState state={state} empty="No fair-use policy was returned.">
        {(policy) => (
          <>
            <PolicySummary policy={policy} />
            <div className="admin-fair-use-editors">
              <PolicyEditor
                key={`policy-${policy.revision}`}
                policy={policy}
                onChanged={() => setRefreshKey((v) => v + 1)}
              />
              <OverrideEditor
                key={`override-${policy.revision}`}
                policy={policy}
              />
            </div>
          </>
        )}
      </AsyncState>
    </>
  );
}

function PolicySummary({ policy }: { policy: FairUsePolicy }) {
  return (
    <>
      <Card>
        <div className="admin-run-detail__head">
          <div>
            <p className="admin-metric-label">Current immutable revision</p>
            <h2>Revision {policy.revision}</h2>
            <p>{policy.reason || "No rationale recorded"}</p>
          </div>
          <Badge tone="canonical">
            Effective {new Date(policy.effectiveFrom).toLocaleString()}
          </Badge>
        </div>
        <div className="admin-fair-use-allowances">
          {AUDIENCES.map((audience) => (
            <div key={audience}>
              <span>{audience}</span>
              <strong>
                {policy[audience].burstUnits.toLocaleString()} burst units
              </strong>
              <small>
                {policy[audience].refillPerSecond}/s ·{" "}
                {formatWindow(policy[audience].windowSeconds)}
              </small>
            </div>
          ))}
        </div>
      </Card>
      <Card>
        <div className="admin-section-heading">
          <Gauge size={18} />
          <h2>Cost-class ceilings</h2>
        </div>
        <div className="admin-release-metrics">
          {COSTS.map((cost) => (
            <div key={cost}>
              <span>{cost}</span>
              <strong>{policy.costCeilings[cost].toLocaleString()}</strong>
            </div>
          ))}
        </div>
        <p className="admin-form-hint">
          These are request-cost ceilings, not pricing tiers. A disabled or
          constrained class remains free.
        </p>
      </Card>
    </>
  );
}

function PolicyEditor({
  policy,
  onChanged,
}: {
  policy: FairUsePolicy;
  onChanged: () => void;
}) {
  const [revision, setRevision] = useState(policy.revision + 1);
  const [effectiveFrom, setEffectiveFrom] = useState("");
  const [reason, setReason] = useState("");
  const [allowances, setAllowances] = useState<
    Record<Audience, FairUseAllowance>
  >({
    anonymous: { ...policy.anonymous },
    authenticated: { ...policy.authenticated },
    sandbox: { ...policy.sandbox },
  });
  const [ceilings, setCeilings] = useState<FairUseCostCeilings>({
    ...policy.costCeilings,
  });
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState<Notice>(null);
  const invalid =
    revision <= policy.revision ||
    reason.trim().length < 10 ||
    !effectiveFrom ||
    Object.values(allowances).some(
      (a) => a.burstUnits < 1 || a.refillPerSecond <= 0 || a.windowSeconds < 1,
    ) ||
    allowances.sandbox.burstUnits > allowances.authenticated.burstUnits ||
    allowances.sandbox.refillPerSecond >
      allowances.authenticated.refillPerSecond ||
    allowances.sandbox.windowSeconds > allowances.authenticated.windowSeconds ||
    COSTS.some((cost) => ceilings[cost] < 1);
  async function submit(event: FormEvent) {
    event.preventDefault();
    if (invalid) return;
    setBusy(true);
    setNotice(null);
    try {
      const created = await appendAdminFairUsePolicy({
        revision,
        anonymous: allowances.anonymous,
        authenticated: allowances.authenticated,
        sandbox: allowances.sandbox,
        costCeilings: ceilings,
        reason: reason.trim(),
        effectiveFrom: new Date(effectiveFrom).toISOString(),
      });
      setNotice({
        tone: "success",
        text: `Revision ${created.revision} appended. Refreshing live policy.`,
      });
      onChanged();
    } catch (error) {
      setNotice({
        tone: "error",
        text: error instanceof Error ? error.message : "Policy append failed.",
      });
    } finally {
      setBusy(false);
    }
  }
  return (
    <RequirePermission permission="fairuse:manage">
      <Card className="admin-release-actions">
        <div>
          <ShieldCheck size={20} />
          <div>
            <h3>Append a policy revision</h3>
            <p>
              Published revisions are never edited. The server records the actor
              and audit evidence.
            </p>
          </div>
        </div>
        <form
          className="admin-fair-use-form"
          onSubmit={(event) => void submit(event)}
        >
          <div className="admin-form-grid">
            <NumberField
              label="New revision"
              value={revision}
              onChange={setRevision}
              min={policy.revision + 1}
            />
            <Field label="Effective from">
              <DateTimeInput
                name="effectiveFrom"
                value={effectiveFrom}
                onValueChange={setEffectiveFrom}
                required
                ariaLabel="Policy effective from"
              />
            </Field>
          </div>
          <AllowanceFields
            values={allowances}
            onChange={(audience, key, value) =>
              setAllowances((current) => ({
                ...current,
                [audience]: { ...current[audience], [key]: value },
              }))
            }
          />
          <CeilingFields
            values={ceilings}
            onChange={(cost, value) =>
              setCeilings((current) => ({ ...current, [cost]: value }))
            }
          />
          <label>
            Documented rationale
            <textarea
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              minLength={10}
              placeholder="Operational reason for changing the public fair-use policy"
              required
            />
          </label>
          {allowances.sandbox.burstUnits >
            allowances.authenticated.burstUnits ||
          allowances.sandbox.refillPerSecond >
            allowances.authenticated.refillPerSecond ||
          allowances.sandbox.windowSeconds >
            allowances.authenticated.windowSeconds ? (
            <p className="admin-mutation-result admin-mutation-result--error">
              Sandbox allowance cannot exceed authenticated allowance.
            </p>
          ) : null}
          <button
            className="gg-button gg-button--primary"
            disabled={invalid || busy}
          >
            {busy ? "Appending…" : `Append revision ${revision}`}
          </button>
          <MutationNotice notice={notice} />
        </form>
      </Card>
    </RequirePermission>
  );
}

function OverrideEditor({ policy }: { policy: FairUsePolicy }) {
  const [id, setId] = useState("");
  const [applicationId, setApplicationId] = useState("");
  const [reason, setReason] = useState("");
  const [expiresAt, setExpiresAt] = useState("");
  const [supersedesId, setSupersedesId] = useState("");
  const [allowance, setAllowance] = useState<FairUseAllowance>({
    ...policy.authenticated,
    burstUnits: policy.authenticated.burstUnits + 1,
  });
  const [ceilings, setCeilings] = useState<FairUseCostCeilings>({
    ...policy.costCeilings,
  });
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState<Notice>(null);
  const raised =
    allowance.burstUnits >= policy.authenticated.burstUnits &&
    allowance.refillPerSecond >= policy.authenticated.refillPerSecond &&
    allowance.windowSeconds >= policy.authenticated.windowSeconds &&
    COSTS.every((cost) => ceilings[cost] >= policy.costCeilings[cost]) &&
    (allowance.burstUnits > policy.authenticated.burstUnits ||
      allowance.refillPerSecond > policy.authenticated.refillPerSecond ||
      allowance.windowSeconds > policy.authenticated.windowSeconds ||
      COSTS.some((cost) => ceilings[cost] > policy.costCeilings[cost]));
  const invalid =
    !id.trim() ||
    !applicationId.trim() ||
    reason.trim().length < 10 ||
    !expiresAt ||
    !raised;
  async function submit(event: FormEvent) {
    event.preventDefault();
    if (invalid) return;
    setBusy(true);
    setNotice(null);
    try {
      const created = await appendAdminFairUseOverride({
        id: id.trim(),
        applicationId: applicationId.trim(),
        allowance,
        costCeilings: ceilings,
        enabled: true,
        reason: reason.trim(),
        expiresAt: new Date(expiresAt).toISOString(),
        ...(supersedesId.trim() ? { supersedesId: supersedesId.trim() } : {}),
      });
      setNotice({
        tone: "success",
        text: `Temporary override ${created.id} appended for ${created.applicationId}; expires ${new Date(created.expiresAt).toLocaleString()}.`,
      });
      setId("");
      setApplicationId("");
      setReason("");
      setSupersedesId("");
    } catch (error) {
      setNotice({
        tone: "error",
        text:
          error instanceof Error ? error.message : "Override append failed.",
      });
    } finally {
      setBusy(false);
    }
  }
  return (
    <RequirePermission permission="fairuse:manage">
      <Card className="admin-release-actions">
        <div>
          <ShieldCheck size={20} />
          <div>
            <h3>Append a raised application override</h3>
            <p>
              Temporary, named and auditable. It cannot reduce the public
              allowance or become permanent.
            </p>
          </div>
        </div>
        <form
          className="admin-fair-use-form"
          onSubmit={(event) => void submit(event)}
        >
          <div className="admin-form-grid">
            <label>
              Override ID
              <input
                value={id}
                onChange={(e) => setId(e.target.value)}
                placeholder="ovr-census-2026"
                required
              />
            </label>
            <label>
              Application ID
              <input
                value={applicationId}
                onChange={(e) => setApplicationId(e.target.value)}
                placeholder="app_…"
                required
              />
            </label>
            <Field label="Expires at">
              <DateTimeInput
                name="expiresAt"
                value={expiresAt}
                onValueChange={setExpiresAt}
                required
                ariaLabel="Override expires at"
              />
            </Field>
            <label>
              Supersedes override (optional)
              <input
                value={supersedesId}
                onChange={(e) => setSupersedesId(e.target.value)}
              />
            </label>
          </div>
          <div className="admin-form-grid">
            <NumberField
              label="Burst units"
              value={allowance.burstUnits}
              onChange={(value) =>
                setAllowance((v) => ({ ...v, burstUnits: value }))
              }
              min={policy.authenticated.burstUnits}
            />
            <NumberField
              label="Refill per second"
              value={allowance.refillPerSecond}
              onChange={(value) =>
                setAllowance((v) => ({ ...v, refillPerSecond: value }))
              }
              min={policy.authenticated.refillPerSecond}
              step="0.1"
            />
            <NumberField
              label="Window seconds"
              value={allowance.windowSeconds}
              onChange={(value) =>
                setAllowance((v) => ({ ...v, windowSeconds: value }))
              }
              min={policy.authenticated.windowSeconds}
            />
          </div>
          <CeilingFields
            values={ceilings}
            onChange={(cost, value) =>
              setCeilings((current) => ({ ...current, [cost]: value }))
            }
            minimums={policy.costCeilings}
          />
          <label>
            Documented operational reason
            <textarea
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              minLength={10}
              placeholder="Why this application temporarily needs additional shared capacity"
              required
            />
          </label>
          {!raised ? (
            <p className="admin-mutation-result admin-mutation-result--error">
              An override must raise at least one allowance and cannot lower any
              current authenticated value.
            </p>
          ) : null}
          <button
            className="gg-button gg-button--primary"
            disabled={invalid || busy}
          >
            {busy ? "Appending…" : "Append temporary override"}
          </button>
          <MutationNotice notice={notice} />
        </form>
      </Card>
    </RequirePermission>
  );
}

function AllowanceFields({
  values,
  onChange,
}: {
  values: Record<Audience, FairUseAllowance>;
  onChange: (
    audience: Audience,
    key: keyof FairUseAllowance,
    value: number,
  ) => void;
}) {
  return (
    <div className="admin-fair-use-fieldsets">
      {AUDIENCES.map((audience) => (
        <fieldset key={audience}>
          <legend>{audience}</legend>
          <NumberField
            label="Burst units"
            value={values[audience].burstUnits}
            onChange={(v) => onChange(audience, "burstUnits", v)}
            min={1}
          />
          <NumberField
            label="Refill / second"
            value={values[audience].refillPerSecond}
            onChange={(v) => onChange(audience, "refillPerSecond", v)}
            min={0.1}
            step="0.1"
          />
          <NumberField
            label="Window seconds"
            value={values[audience].windowSeconds}
            onChange={(v) => onChange(audience, "windowSeconds", v)}
            min={1}
          />
        </fieldset>
      ))}
    </div>
  );
}
function CeilingFields({
  values,
  onChange,
  minimums,
}: {
  values: FairUseCostCeilings;
  onChange: (cost: keyof FairUseCostCeilings, value: number) => void;
  minimums?: FairUseCostCeilings;
}) {
  return (
    <fieldset>
      <legend>Cost-class ceilings</legend>
      <div className="admin-form-grid">
        {COSTS.map((cost) => (
          <NumberField
            key={cost}
            label={cost}
            value={values[cost]}
            onChange={(v) => onChange(cost, v)}
            min={minimums?.[cost] ?? 1}
          />
        ))}
      </div>
    </fieldset>
  );
}
function NumberField({
  label,
  value,
  onChange,
  min,
  step = "1",
}: {
  label: string;
  value: number;
  onChange: (value: number) => void;
  min: number;
  step?: string;
}) {
  return (
    <label>
      {label}
      <input
        type="number"
        value={value}
        min={min}
        step={step}
        onChange={(e) => onChange(Number(e.target.value))}
        required
      />
    </label>
  );
}
function MutationNotice({ notice }: { notice: Notice }) {
  return notice ? (
    <p
      className={`admin-mutation-result admin-mutation-result--${notice.tone}`}
      role={notice.tone === "error" ? "alert" : "status"}
    >
      {notice.text}
    </p>
  ) : null;
}
function formatWindow(seconds: number) {
  return seconds % 3600 === 0
    ? `${seconds / 3600}h window`
    : `${seconds}s window`;
}
