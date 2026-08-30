"use client";

import { useState, type FormEvent } from "react";
import {
  Badge,
  Card,
  DateTimeInput,
  EmptyState,
  Field,
  Select,
} from "@ghanageo/ui";
import {
  Archive,
  Copy,
  GitMerge,
  MapPinned,
  Plus,
  RefreshCw,
  Tags,
} from "lucide-react";
import { AsyncState, useApi } from "@/components/data";
import { PageHeader } from "@/components/screen";
import { RequirePermission } from "@/components/session";
import { TableActionButton, TableActions } from "@/components/table-actions";
import {
  AdminApiError,
  createAdminAlias,
  createAdminGeography,
  deprecateAdminAlias,
  deprecateAdminGeography,
  getAdminBoundary,
  listAdminAliases,
  listAdminRedirects,
  updateAdminBoundary,
  type BoundaryGeometry,
  type GeographyKind,
} from "@/lib/admin-api";

export function GeographyCreatePanel({
  kind,
  onCreated,
}: {
  kind: GeographyKind;
  onCreated?: () => void;
}) {
  const [open, setOpen] = useState(false);
  const [id, setId] = useState("");
  const [name, setName] = useState("");
  const [parentId, setParentId] = useState("");
  const [sourceId, setSourceId] = useState("");
  const [externalId, setExternalId] = useState("");
  const [retrievedAt, setRetrievedAt] = useState("");
  const [hash, setHash] = useState("");
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState<string | null>(null);
  async function submit(event: FormEvent) {
    event.preventDefault();
    setBusy(true);
    setNotice(null);
    try {
      await createAdminGeography(kind, {
        id: id.trim(),
        name: name.trim(),
        countryCode: "GH",
        ...(kind === "districts"
          ? { regionId: parentId.trim() }
          : kind === "places"
            ? { districtId: parentId.trim() }
            : {}),
        provenance: {
          sourceId: sourceId.trim(),
          externalId: externalId.trim(),
          retrievedAt: new Date(retrievedAt).toISOString(),
          sourcePayloadHash: hash.trim(),
        },
      });
      setNotice(`${name} created and recorded in the audit trail.`);
      setId("");
      setName("");
      onCreated?.();
    } catch (error) {
      setNotice(error instanceof Error ? error.message : "Creation failed.");
    } finally {
      setBusy(false);
    }
  }
  return (
    <RequirePermission permission="geography:edit">
      {!open ? (
        <button
          className="gg-button gg-button--primary gg-button--sm"
          onClick={() => setOpen(true)}
        >
          <Plus size={15} /> Create {kind.slice(0, -1)}
        </button>
      ) : (
        <Card className="admin-release-actions">
          <div>
            <Plus size={20} />
            <div>
              <h3>Create {kind.slice(0, -1)}</h3>
              <p>
                Canonical creation requires durable source provenance. The
                request is idempotent and actor-scoped.
              </p>
            </div>
          </div>
          <form className="admin-form-grid" onSubmit={(e) => void submit(e)}>
            <label>
              ID
              <input
                value={id}
                onChange={(e) => setId(e.target.value)}
                required
              />
            </label>
            <label>
              Name
              <input
                value={name}
                onChange={(e) => setName(e.target.value)}
                required
              />
            </label>
            {kind !== "regions" ? (
              <label>
                {kind === "districts" ? "Region ID" : "District ID"}
                <input
                  value={parentId}
                  onChange={(e) => setParentId(e.target.value)}
                  required
                />
              </label>
            ) : null}
            <label>
              Source ID
              <input
                value={sourceId}
                onChange={(e) => setSourceId(e.target.value)}
                required
              />
            </label>
            <label>
              External source ID
              <input
                value={externalId}
                onChange={(e) => setExternalId(e.target.value)}
                required
              />
            </label>
            <Field label="Retrieved at">
              <DateTimeInput
                name="retrievedAt"
                value={retrievedAt}
                onValueChange={setRetrievedAt}
                required
                ariaLabel="Source retrieved at"
              />
            </Field>
            <label>
              Source payload SHA-256
              <input
                value={hash}
                minLength={64}
                maxLength={64}
                onChange={(e) => setHash(e.target.value)}
                required
              />
            </label>
            <div className="admin-release-actions__buttons">
              <button
                className="gg-button gg-button--primary gg-button--sm"
                disabled={busy || hash.trim().length !== 64}
              >
                {busy ? "Creating…" : "Create canonical record"}
              </button>
              <button
                type="button"
                className="gg-button gg-button--ghost gg-button--sm"
                onClick={() => setOpen(false)}
              >
                Cancel
              </button>
            </div>
          </form>
          {notice ? (
            <p className="admin-mutation-result" role="status">
              {notice}
            </p>
          ) : null}
        </Card>
      )}
    </RequirePermission>
  );
}

export function RetirementPanel({
  kind,
  id,
  status,
  onChanged,
}: {
  kind: GeographyKind;
  id: string;
  status: string;
  onChanged: () => void;
}) {
  const [target, setTarget] = useState("");
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  if (status !== "ACTIVE")
    return (
      <Card>
        <Badge tone="needsRecon">{status}</Badge>
        <p>
          This record is already retired; durable redirects remain discoverable.
        </p>
      </Card>
    );
  async function submit(event: FormEvent) {
    event.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await deprecateAdminGeography(kind, id, target.trim(), reason.trim());
      onChanged();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Retirement failed.");
    } finally {
      setBusy(false);
    }
  }
  return (
    <RequirePermission permission="geography:edit">
      <Card className="admin-release-actions">
        <div>
          <GitMerge size={20} />
          <div>
            <h3>Deprecate or merge</h3>
            <p>
              Leave the successor blank to deprecate. Name a different active{" "}
              {kind.slice(0, -1)} to create a durable merge redirect.
            </p>
          </div>
        </div>
        <form className="admin-review-form" onSubmit={(e) => void submit(e)}>
          <label>
            Successor ID (optional)
            <input value={target} onChange={(e) => setTarget(e.target.value)} />
          </label>
          <label>
            Reason
            <textarea
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              placeholder="Recorded with the retirement and redirect"
            />
          </label>
          <button
            className="gg-button gg-button--danger gg-button--sm"
            disabled={busy || target.trim() === id}
          >
            {busy
              ? "Applying…"
              : target.trim()
                ? "Merge into successor"
                : "Deprecate record"}
          </button>
          {target.trim() === id ? (
            <p className="admin-mutation-result admin-mutation-result--error">
              A record cannot merge into itself.
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
        </form>
      </Card>
    </RequirePermission>
  );
}

export function BoundaryEditor({
  kind,
  id,
}: {
  kind: "region" | "district";
  id: string;
}) {
  const [revision, setRevision] = useState(0);
  const state = useApi(
    (signal) => getAdminBoundary(kind, id, signal),
    [kind, id, revision],
  );
  return (
    <section>
      <div className="admin-section-heading">
        <MapPinned size={18} />
        <h2>Boundary geometry</h2>
      </div>
      <AsyncState state={state} empty="No boundary response was returned.">
        {(current) => (
          <BoundaryForm
            kind={kind}
            id={id}
            current={current}
            onRefresh={() => setRevision((v) => v + 1)}
          />
        )}
      </AsyncState>
    </section>
  );
}

export function GeometryWorkspace() {
  const [kind, setKind] = useState<"region" | "district">("region");
  const [id, setId] = useState("");
  const [selection, setSelection] = useState<{
    kind: "region" | "district";
    id: string;
  } | null>(null);
  return (
    <>
      <PageHeader
        eyebrow="Canonical boundary operations"
        title="Geometry workspace"
        lede="Load one region or district boundary, edit GeoJSON coordinates, and save only against its current ETag. Invalid rings, self-intersections and districts outside their parent region are rejected by the API."
      />
      <Card className="admin-geometry-guide">
        <MapPinned size={20} />
        <div>
          <strong>Coordinate order is longitude, latitude.</strong>
          <p>
            Polygon rings need at least four positions and must close by
            repeating the first position. MultiPolygon wraps one additional
            array level. This editor never silently repairs geometry.
          </p>
        </div>
      </Card>
      <form
        className="admin-geometry-loader"
        onSubmit={(e) => {
          e.preventDefault();
          if (id.trim()) setSelection({ kind, id: id.trim() });
        }}
      >
        <Field label="Boundary kind">
          <Select
            value={kind}
            onValueChange={(value) => setKind(value as "region" | "district")}
            ariaLabel="Boundary kind"
            options={[
              {
                value: "region",
                label: "Region",
                hint: "National administrative boundary",
              },
              {
                value: "district",
                label: "District",
                hint: "MMDA boundary within a region",
              },
            ]}
          />
        </Field>
        <label className="admin-geometry-loader__id">
          <span>Canonical record ID</span>
          <input value={id} onChange={(e) => setId(e.target.value)} required />
        </label>
        <button className="gg-button gg-button--primary admin-geometry-loader__submit">
          <MapPinned size={15} /> Load boundary
        </button>
      </form>
      {selection ? (
        <BoundaryEditor
          key={`${selection.kind}:${selection.id}`}
          {...selection}
        />
      ) : (
        <EmptyState
          title="No boundary loaded"
          description="Choose a boundary kind and enter its canonical record ID to open the geometry editor."
        />
      )}
    </>
  );
}

export function OSMCreatePanel({
  kind,
  onCreated,
}: {
  kind: "roads" | "pois";
  onCreated: () => void;
}) {
  const [open, setOpen] = useState(false);
  const [id, setId] = useState("");
  const [name, setName] = useState("");
  const [districtId, setDistrictId] = useState("");
  const [regionId, setRegionId] = useState("");
  const [classification, setClassification] = useState("");
  const [detail, setDetail] = useState("");
  const [attribution, setAttribution] = useState("");
  const [geometry, setGeometry] = useState(
    kind === "roads"
      ? '{"type":"LineString","coordinates":[]}'
      : '{"type":"Point","coordinates":[]}',
  );
  const [sourceId, setSourceId] = useState("");
  const [externalId, setExternalId] = useState("");
  const [retrievedAt, setRetrievedAt] = useState("");
  const [hash, setHash] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  async function create(event: FormEvent) {
    event.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const parsed = JSON.parse(geometry) as {
        type: string;
        coordinates: unknown;
      };
      await createAdminGeography(kind, {
        id: id.trim(),
        name: name.trim(),
        districtId: districtId.trim(),
        regionId: regionId.trim(),
        ...(kind === "roads"
          ? { roadClass: classification.trim(), ref: detail.trim() }
          : { poiClass: classification.trim(), category: detail.trim() }),
        attribution: attribution.trim(),
        geometry: parsed,
        provenance: {
          sourceId: sourceId.trim(),
          externalId: externalId.trim(),
          retrievedAt: new Date(retrievedAt).toISOString(),
          sourcePayloadHash: hash.trim(),
        },
      });
      setOpen(false);
      onCreated();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Creation failed.");
    } finally {
      setBusy(false);
    }
  }
  return (
    <RequirePermission permission="geography:edit">
      {!open ? (
        <button
          className="gg-button gg-button--primary gg-button--sm"
          onClick={() => setOpen(true)}
        >
          <Plus size={15} /> Create {kind === "roads" ? "road" : "POI"}
        </button>
      ) : (
        <Card className="admin-release-actions">
          <div>
            <Plus size={20} />
            <div>
              <h3>Create {kind === "roads" ? "road" : "point of interest"}</h3>
              <p>
                The V1 contract supports create and deprecate only. Editing an
                existing {kind === "roads" ? "road" : "POI"} is not exposed by
                the backend.
              </p>
            </div>
          </div>
          <form className="admin-review-form" onSubmit={(e) => void create(e)}>
            <div className="admin-form-grid">
              <label>
                ID
                <input
                  value={id}
                  onChange={(e) => setId(e.target.value)}
                  required
                />
              </label>
              <label>
                Name
                <input
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  required
                />
              </label>
              <label>
                District ID
                <input
                  value={districtId}
                  onChange={(e) => setDistrictId(e.target.value)}
                  required
                />
              </label>
              <label>
                Region ID
                <input
                  value={regionId}
                  onChange={(e) => setRegionId(e.target.value)}
                  required
                />
              </label>
              <label>
                {kind === "roads" ? "Road class" : "POI class"}
                <input
                  value={classification}
                  onChange={(e) => setClassification(e.target.value)}
                  required
                />
              </label>
              <label>
                {kind === "roads" ? "Reference" : "Category"}
                <input
                  value={detail}
                  onChange={(e) => setDetail(e.target.value)}
                />
              </label>
              <label>
                Attribution
                <input
                  value={attribution}
                  onChange={(e) => setAttribution(e.target.value)}
                  required
                />
              </label>
              <label>
                Source ID
                <input
                  value={sourceId}
                  onChange={(e) => setSourceId(e.target.value)}
                  required
                />
              </label>
              <label>
                External ID
                <input
                  value={externalId}
                  onChange={(e) => setExternalId(e.target.value)}
                  required
                />
              </label>
              <Field label="Retrieved at">
                <DateTimeInput
                  name="retrievedAt"
                  value={retrievedAt}
                  onValueChange={setRetrievedAt}
                  required
                  ariaLabel="Source retrieved at"
                />
              </Field>
              <label>
                Payload SHA-256
                <input
                  value={hash}
                  minLength={64}
                  maxLength={64}
                  onChange={(e) => setHash(e.target.value)}
                  required
                />
              </label>
            </div>
            <label>
              GeoJSON geometry
              <textarea
                className="admin-json-input"
                value={geometry}
                onChange={(e) => setGeometry(e.target.value)}
              />
            </label>
            <div className="admin-release-actions__buttons">
              <button
                className="gg-button gg-button--primary gg-button--sm"
                disabled={busy || hash.trim().length !== 64}
              >
                {busy ? "Creating…" : "Create record"}
              </button>
              <button
                type="button"
                className="gg-button gg-button--ghost gg-button--sm"
                onClick={() => setOpen(false)}
              >
                Cancel
              </button>
            </div>
            {error ? (
              <p className="admin-mutation-result admin-mutation-result--error">
                {error}
              </p>
            ) : null}
          </form>
        </Card>
      )}
    </RequirePermission>
  );
}

export function OSMDeprecateButton({
  kind,
  id,
  onChanged,
}: {
  kind: "roads" | "pois";
  id: string;
  onChanged: () => void;
}) {
  const [open, setOpen] = useState(false);
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  async function submit() {
    setBusy(true);
    setError(null);
    try {
      await deprecateAdminGeography(kind, id, "", reason.trim());
      onChanged();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Deprecation failed.");
    } finally {
      setBusy(false);
    }
  }
  return (
    <RequirePermission permission="geography:edit">
      {open ? (
        <div className="admin-inline-retire">
          <input
            aria-label="Deprecation reason"
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            placeholder="Reason"
          />
          <button
            className="gg-button gg-button--danger gg-button--sm"
            disabled={busy}
            onClick={() => void submit()}
          >
            {busy ? "Applying…" : "Confirm"}
          </button>
          <button
            className="gg-button gg-button--ghost gg-button--sm"
            onClick={() => setOpen(false)}
          >
            Cancel
          </button>
          {error ? <small role="alert">{error}</small> : null}
        </div>
      ) : (
        <TableActions label={`Actions for ${id}`}>
          <TableActionButton
            label={`Deprecate ${id}`}
            tone="danger"
            onClick={() => setOpen(true)}
          >
            <Archive />
          </TableActionButton>
        </TableActions>
      )}
    </RequirePermission>
  );
}
function BoundaryForm({
  kind,
  id,
  current,
  onRefresh,
}: {
  kind: "region" | "district";
  id: string;
  current: { geometry: BoundaryGeometry | null; etag: string };
  onRefresh: () => void;
}) {
  const [draft, setDraft] = useState(
    current.geometry
      ? JSON.stringify(current.geometry, null, 2)
      : '{\n  "type": "Polygon",\n  "coordinates": []\n}',
  );
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState<string | null>(null);
  async function save() {
    setBusy(true);
    setNotice(null);
    try {
      const geometry = JSON.parse(draft) as BoundaryGeometry;
      validateBoundary(geometry);
      await updateAdminBoundary(kind, id, geometry, current.etag);
      setNotice("Boundary saved against the displayed ETag.");
      onRefresh();
    } catch (error) {
      if (error instanceof AdminApiError && error.code === "CONFLICT") {
        setNotice(
          "Boundary changed on the server. Loading the latest geometry and ETag before another edit.",
        );
        onRefresh();
      } else
        setNotice(
          error instanceof Error ? error.message : "Boundary update failed.",
        );
    } finally {
      setBusy(false);
    }
  }
  return (
    <Card className="admin-release-actions">
      <div>
        <MapPinned size={20} />
        <div>
          <h3>
            {current.geometry
              ? "Edit GeoJSON boundary"
              : "Add GeoJSON boundary"}
          </h3>
          <p>
            CAS token <code>{current.etag || "not returned"}</code>. District
            geometry must remain inside its parent region.
          </p>
        </div>
      </div>
      <pre className="admin-boundary-preview">
        {current.geometry
          ? `${current.geometry.type} · server geometry loaded`
          : "No boundary recorded"}
      </pre>
      <RequirePermission permission="geometry:edit">
        <label>
          Polygon or MultiPolygon
          <textarea
            className="admin-json-input"
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            spellCheck={false}
          />
        </label>
        <div className="admin-release-actions__buttons">
          <button
            className="gg-button gg-button--primary gg-button--sm"
            disabled={busy || !current.etag}
            onClick={() => void save()}
          >
            {busy ? "Validating…" : "Save with If-Match"}
          </button>
          <button
            className="gg-button gg-button--ghost gg-button--sm"
            onClick={onRefresh}
          >
            Discard & refresh
          </button>
        </div>
      </RequirePermission>
      {notice ? (
        <p className="admin-mutation-result" role="status">
          {notice}
        </p>
      ) : null}
    </Card>
  );
}

export function AliasWorkspace({
  initialPlaceId = "",
}: {
  initialPlaceId?: string;
}) {
  const [placeId, setPlaceId] = useState(initialPlaceId);
  const [selected, setSelected] = useState(initialPlaceId);
  const [refreshKey, setRefreshKey] = useState(0);
  return (
    <>
      <PageHeader
        eyebrow="Place naming evidence"
        title="Aliases"
        lede="Inspect and manage alternate place names. Deprecation preserves history instead of deleting evidence."
      />
      <form
        className="admin-filter-bar"
        onSubmit={(e) => {
          e.preventDefault();
          setSelected(placeId.trim());
        }}
      >
        <label>
          Place ID
          <input
            value={placeId}
            onChange={(e) => setPlaceId(e.target.value)}
            required
          />
        </label>
        <button className="gg-button gg-button--primary gg-button--sm">
          <Tags size={15} /> Load aliases
        </button>
      </form>
      {selected ? (
        <AliasList
          key={`${selected}:${refreshKey}`}
          placeId={selected}
          onChanged={() => setRefreshKey((v) => v + 1)}
        />
      ) : (
        <EmptyState
          icon={<Tags />}
          title="No place selected"
          description="Enter a canonical place ID to inspect its alternate names and alias history."
        />
      )}
    </>
  );
}
function AliasList({
  placeId,
  onChanged,
}: {
  placeId: string;
  onChanged: () => void;
}) {
  const state = useApi(
    (signal) => listAdminAliases(placeId, signal),
    [placeId],
  );
  const [id, setId] = useState("");
  const [value, setValue] = useState("");
  const [language, setLanguage] = useState("en");
  const [notice, setNotice] = useState<string | null>(null);
  async function create(event: FormEvent) {
    event.preventDefault();
    try {
      await createAdminAlias(placeId, {
        id: id.trim(),
        value: value.trim(),
        language: language.trim(),
      });
      setId("");
      setValue("");
      onChanged();
    } catch (e) {
      setNotice(e instanceof Error ? e.message : "Alias creation failed.");
    }
  }
  return (
    <>
      <RequirePermission permission="geography:edit">
        <Card>
          <form className="admin-form-grid" onSubmit={(e) => void create(e)}>
            <label>
              Alias ID
              <input
                value={id}
                onChange={(e) => setId(e.target.value)}
                required
              />
            </label>
            <label>
              Alias value
              <input
                value={value}
                onChange={(e) => setValue(e.target.value)}
                required
              />
            </label>
            <label>
              Language
              <input
                value={language}
                onChange={(e) => setLanguage(e.target.value)}
              />
            </label>
            <button className="gg-button gg-button--primary gg-button--sm">
              Create alias
            </button>
          </form>
          {notice ? (
            <p className="admin-mutation-result admin-mutation-result--error">
              {notice}
            </p>
          ) : null}
        </Card>
      </RequirePermission>
      <AsyncState state={state} empty="No aliases were returned.">
        {(aliases) =>
          aliases.length ? (
            <Card style={{ padding: 0, overflow: "hidden" }}>
              <table className="gg-table" style={{ width: "100%" }}>
                <thead>
                  <tr>
                    <th>Alias</th>
                    <th>Type / language</th>
                    <th>Status</th>
                    <th>
                      <span className="sr-only">Action</span>
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {aliases.map((alias) => (
                    <tr key={alias.id}>
                      <td>
                        <strong>{alias.value}</strong>
                        <small>{alias.normalizedValue}</small>
                      </td>
                      <td>
                        {alias.type || "Not typed"} ·{" "}
                        {alias.language || "Not recorded"}
                      </td>
                      <td>
                        <Badge
                          tone={
                            alias.status === "ACTIVE"
                              ? "canonical"
                              : "reference"
                          }
                        >
                          {alias.status}
                        </Badge>
                      </td>
                      <td>
                        {alias.status === "ACTIVE" ? (
                          <RequirePermission permission="geography:edit">
                            <TableActions label={`Actions for ${alias.value}`}>
                              <TableActionButton
                                label={`Deprecate ${alias.value}`}
                                tone="danger"
                                onClick={() =>
                                  void deprecateAdminAlias(placeId, alias.id)
                                    .then(onChanged)
                                    .catch((e: Error) => setNotice(e.message))
                                }
                              >
                                <Archive />
                              </TableActionButton>
                            </TableActions>
                          </RequirePermission>
                        ) : null}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </Card>
          ) : (
            <EmptyState
              icon={<Tags />}
              title="No aliases recorded"
              description="This place has no alternate names yet. Add one when a verified name variant is available."
            />
          )
        }
      </AsyncState>
    </>
  );
}

export function RedirectsWorkspace() {
  const [cursor, setCursor] = useState("");
  const [history, setHistory] = useState<string[]>([]);
  const state = useApi(
    (signal) => listAdminRedirects({ cursor, limit: 20 }, signal),
    [cursor],
  );
  return (
    <>
      <PageHeader
        eyebrow="Durable identity history"
        title="Geography redirects"
        lede="Merged identifiers remain resolvable and name their canonical successor instead of becoming unexplained 404s."
      />
      <AsyncState state={state} empty="No redirects were returned.">
        {(page) =>
          page.data.length ? (
            <>
              <Card style={{ padding: 0, overflow: "hidden" }}>
                <table className="gg-table" style={{ width: "100%" }}>
                  <thead>
                    <tr>
                      <th>Retired ID</th>
                      <th>Kind</th>
                      <th>Successor</th>
                      <th>Reason</th>
                      <th>Merged</th>
                      <th>
                        <span className="sr-only">Actions</span>
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {page.data.map((item) => (
                      <tr key={`${item.kind}:${item.oldId}`}>
                        <td>
                          <code>{item.oldId}</code>
                        </td>
                        <td>{item.kind}</td>
                        <td>
                          <code>{item.newId}</code>
                        </td>
                        <td>{item.reason || "Not recorded"}</td>
                        <td>{new Date(item.mergedAt).toLocaleString()}</td>
                        <td>
                          <TableActions
                            label={`Actions for redirect ${item.oldId}`}
                          >
                            <TableActionButton
                              label={`Copy successor ID ${item.newId}`}
                              onClick={() =>
                                void navigator.clipboard.writeText(item.newId)
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
                  disabled={!page.meta.nextCursor}
                  onClick={() => {
                    setHistory((h) => [...h, cursor]);
                    setCursor(page.meta.nextCursor ?? "");
                  }}
                >
                  Next
                </button>
              </nav>
            </>
          ) : (
            <EmptyState
              icon={<GitMerge />}
              title="No redirects recorded"
              description="Merged and retired identifiers will appear here with their canonical successors."
            />
          )
        }
      </AsyncState>
    </>
  );
}

function validateBoundary(geometry: BoundaryGeometry) {
  if (geometry.type !== "Polygon" && geometry.type !== "MultiPolygon")
    throw new Error("Boundary type must be Polygon or MultiPolygon.");
  const polygons =
    geometry.type === "Polygon" ? [geometry.coordinates] : geometry.coordinates;
  if (!Array.isArray(polygons) || !polygons.length)
    throw new Error("At least one polygon is required.");
  for (const polygon of polygons) {
    if (!Array.isArray(polygon) || !polygon.length)
      throw new Error("Every polygon needs an exterior ring.");
    for (const ring of polygon) {
      if (!Array.isArray(ring) || ring.length < 4)
        throw new Error("Every ring needs at least four positions.");
      for (const position of ring)
        if (
          !Array.isArray(position) ||
          position.length < 2 ||
          position
            .slice(0, 2)
            .some(
              (value) => typeof value !== "number" || !Number.isFinite(value),
            )
        )
          throw new Error(
            "Every position needs finite [longitude, latitude] numbers.",
          );
      const first = ring[0] as number[];
      const last = ring.at(-1) as number[];
      if (first[0] !== last[0] || first[1] !== last[1])
        throw new Error(
          "Every ring must close by repeating its first position.",
        );
    }
  }
}
