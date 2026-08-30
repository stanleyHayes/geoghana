/**
 * The authenticated admin API client.
 *
 * Distinct from lib/api.ts, which reads the PUBLIC endpoints anonymously.
 * These calls carry a session cookie and can change canonical data, so they
 * go through /api — the Next rewrite that keeps the console same-origin with
 * the API. A cross-origin fetch would silently drop the SameSite=Lax cookie
 * and every write would come back 401.
 */

const BASE = "/api/v1";

export class AdminApiError extends Error {
  constructor(
    readonly code: string,
    message: string,
    readonly requestId?: string | undefined,
    readonly details?: Record<string, unknown> | undefined,
  ) {
    super(message);
    this.name = "AdminApiError";
  }
}

export async function adminRequest<T>(path: string, init: RequestInit = {}): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    ...init,
    // Same-origin through the proxy, but explicit so a future move to a
    // different host fails loudly rather than silently unauthenticated.
    credentials: "include",
    headers: { "Content-Type": "application/json", ...(init.headers ?? {}) },
  });
  const body = await res.json().catch(() => null);
  if (!res.ok) {
    const e = body?.error;
    const error = new AdminApiError(
      e?.code ?? "INTERNAL",
      e?.message ?? `Request failed (${res.status})`,
      e?.requestId,
      e?.details,
    );
    if (error.code === "UNAUTHENTICATED" && typeof window !== "undefined") {
      window.dispatchEvent(new Event("ghanageo:session-expired"));
    }
    throw error;
  }
  return body as T;
}

/** Everything a role is allowed to do, for gating the UI. */
export interface SessionInfo {
  accountId: string;
  email: string;
  role: string;
  mfaEnrolled: boolean;
  expiresAt: string;
}

export interface PermissionInfo {
  role: string;
  permissions: string[];
}

export const getSession = () =>
  adminRequest<{ data: SessionInfo }>("/auth/session").then((r) => r.data);

export const getPermissions = () =>
  adminRequest<{ data: PermissionInfo }>("/admin/permissions").then((r) => r.data);

export interface LoginResult {
  mfaRequired: boolean;
  mfaEnrolmentRequired: boolean;
  expiresAt: string;
}

export const login = (email: string, password: string) =>
  adminRequest<LoginResult>(
    "/auth/login",
    { method: "POST", body: JSON.stringify({ email, password }) },
  );

export const completeTotp = (code: string) =>
  adminRequest<{ expiresAt: string }>("/auth/mfa/totp", {
    method: "POST",
    body: JSON.stringify({ code }),
  });

export const completeRecovery = (code: string) =>
  adminRequest<{ expiresAt: string }>("/auth/mfa/recover", {
    method: "POST",
    body: JSON.stringify({ code }),
  });

export interface TotpEnrolment {
  secret: string;
  uri: string;
  recoveryCodes: string[];
  notice: string;
}

export const enrolTotp = () =>
  adminRequest<{ data: TotpEnrolment }>("/auth/mfa/enrol", { method: "POST" }).then((r) => r.data);

export const logout = () => adminRequest<{ message: string }>("/auth/logout", { method: "POST" });

export interface AdminAuditSummary {
  id: string;
  at: string;
  actor: { kind: string; id: string; label: string };
  action: string;
  target: { kind: string; id: string; label: string };
  outcome: "succeeded" | "failed";
}

export interface AdminAuditEntry extends AdminAuditSummary {
  actor: AdminAuditSummary["actor"] & { ip?: string };
  requestId?: string;
  before?: Record<string, unknown>;
  after?: Record<string, unknown>;
  reason?: string;
  error?: string;
  hash: string;
  previousHash?: string;
}

export interface AdminDatasetRelease {
  version: string;
  status: string;
  publishedAt?: string;
  changelog?: string;
  counts?: Record<string, number>;
  downloads: Array<{ entity: string; format: string; sizeBytes: number; sha256: string; recordCount: number; url: string }>;
  license: string;
  attribution: string;
}

export interface AdminReleaseReadiness {
  version: string;
  status: string;
  ready: boolean;
  artifactCount: number;
  totalBytes: number;
  bytesVerified: boolean;
  checks: Array<{ name: string; passed: boolean; detail: string }>;
}

export interface AdminRollbackResult { from: AdminDatasetRelease; restored: AdminDatasetRelease }

export interface AdminDashboard {
  counts: { regions: number; districts: number; places: number; roads: number; pois: number; pendingOutbox: number; deadOutbox: number; auditEntries: number };
  currentRelease: AdminDatasetRelease | null;
  recentActivity: AdminAuditSummary[];
}

export interface AdminSystemHealth {
  status: string;
  dependencies: Array<{ name: string; status: string; detail: string; latencyMs: number; checkedAt: string }>;
  queue: Record<string, number>;
  etlFreshness?: string;
  indexStatus: string;
  metrics: Record<string, { value: number; unit: string; status: string; threshold: string; detail?: string }>;
}

export interface AdminSourceRun {
  id: string;
  sourceId: string;
  status: string;
  payloadHash?: string;
  requestId?: string;
  error?: string;
  startedAt: string;
  queuedAt?: string;
  finishedAt?: string;
  durationMs: number;
  recordsProcessed: number;
  errors?: string[];
  conflicts: number;
  duplicateCandidates: number;
  detailAvailability: "durable" | "historical_summary_only";
}

export interface AdminSourceRecord {
  id: string;
  runId: string;
  sourceId: string;
  externalRef: string;
  payloadHash: string;
  outcome: string;
  reasonCode?: string;
  processedAt: string;
}

export interface AdminSourceRecordPage {
  data: AdminSourceRecord[];
  meta: { nextCursor?: string; detailAvailability: "durable" | "historical_summary_only" };
}

export interface AdminPage<T> { data: T[]; meta: { nextCursor?: string } }

export interface FairUseAllowance { burstUnits: number; refillPerSecond: number; windowSeconds: number }
export type FairUseCostCeilings = Record<"cheap" | "normal" | "spatial" | "geometry", number>;
export interface FairUsePolicy {
  id: string; revision: number; anonymous: FairUseAllowance; authenticated: FairUseAllowance; sandbox: FairUseAllowance;
  costCeilings: FairUseCostCeilings; reason: string; actorId: string; createdAt: string; effectiveFrom: string;
}
export interface FairUseOverrideInput {
  id: string; applicationId: string; allowance: FairUseAllowance; costCeilings: FairUseCostCeilings;
  enabled: boolean; reason: string; expiresAt: string; supersedesId?: string;
}
export interface ChangeEvidence { sourceId?: string; sourceRecordIds?: string[]; duplicateCandidateIds?: string[]; reconciliationConflictIds?: string[]; references?: string[] }
export interface ChangeSnapshot { before: Record<string, unknown>; after: Record<string, unknown>; digest: string }
export interface AdminChangeRequest {
  id: string; target: { kind: string; id: string }; proposedPatch: Record<string, unknown>; snapshot: ChangeSnapshot; evidence: ChangeEvidence;
  submitterId: string; reviewerId?: string; state: string; version: number; createdAt: string; updatedAt: string;
  comments: Array<{ id: string; authorId: string; body: string; createdAt: string }>;
  history: Array<{ id: string; actorId: string; comment?: string; from?: string; to: string; createdAt: string }>;
  revisions: Array<{ number: number; proposedPatch: Record<string, unknown>; snapshot: ChangeSnapshot; evidence: ChangeEvidence; actorId: string; createdAt: string }>;
}
export interface ChangeRequestPage { data: AdminChangeRequest[]; nextCursor?: string }
export interface CreateChangeRequestInput { target: { kind: string; id: string }; proposedPatch: Record<string, unknown>; before: Record<string, unknown>; after: Record<string, unknown>; evidence: ChangeEvidence }
export interface DeveloperOrganization { id: string; name: string; ownerId: string; members: Array<{ accountId: string; email: string; role: string; joinedAt: string }>; createdAt: string }
export interface DeveloperAccount { id: string; email: string; emailVerified: boolean; role: string; disabled: boolean; createdAt: string; updatedAt: string }
export interface DeveloperApplication { id: string; organizationId: string; name: string; description: string; environments: string[] | null; domains: string[] | null; callbackUrl: string | null; createdAt: string }
export interface DeveloperKey { id: string; organizationId: string; applicationId: string; name: string; prefix: string; class: string; environment: string; scopes: string[]; allowedOrigins: string[]; allowedIps: string[]; state: "active" | "suspended" | "revoked"; createdAt: string; expiresAt?: string; lastUsedAt?: string; suspendedAt?: string; suspendedReason?: string; revokedAt?: string; revokedReason?: string }
export interface DeveloperListPage<T> { data: T[]; meta: { nextCursor?: string; total: number } }
export interface DeveloperUsageBreakdown { label: string; requests: number; errors: number; quotaCost: number; avgLatencyMs: number }
export interface DeveloperUsage { since: string; requests: number; errors: number; quotaCost: number; avgLatencyMs: number; byProtocol: DeveloperUsageBreakdown[]; byEndpoint: DeveloperUsageBreakdown[]; byGeography: DeveloperUsageBreakdown[] }
export interface DeveloperRequestEvent { id: string; requestId: string; organizationId: string; applicationId: string; keyId: string; protocol: string; operation: string; status: string; success: boolean; latencyMs: number; quotaCost: number; quotaLimit: number; quotaRemaining: number; geography: string; at: string }

function queryPath(path: string, params: Record<string, string | number | undefined>) {
  const query = new URLSearchParams();
  Object.entries(params).forEach(([key, value]) => { if (value !== undefined && value !== "") query.set(key, String(value)); });
  return `${path}${query.size ? `?${query}` : ""}`;
}

const signalInit = (signal?: AbortSignal): RequestInit => signal ? { signal } : {};

export const getAdminDashboard = (signal?: AbortSignal) => adminRequest<{ data: AdminDashboard }>("/admin/dashboard", signalInit(signal)).then((r) => r.data);
export const getAdminSystemHealth = (signal?: AbortSignal) => adminRequest<{ data: AdminSystemHealth }>("/admin/system-health", signalInit(signal)).then((r) => r.data);
export const listAdminAuditLog = (params: { cursor?: string; limit?: number; actor?: string; action?: string; target?: string; outcome?: string }, signal?: AbortSignal) => adminRequest<AdminPage<AdminAuditEntry>>(queryPath("/admin/audit-log", params), signalInit(signal));
export const listAdminSourceRuns = (params: { cursor?: string; limit?: number }, signal?: AbortSignal) => adminRequest<AdminPage<AdminSourceRun>>(queryPath("/admin/source-runs", params), signalInit(signal));
export const getAdminSourceRun = (id: string, signal?: AbortSignal) => adminRequest<{ data: AdminSourceRun }>(`/admin/source-runs/${encodeURIComponent(id)}`, signalInit(signal)).then((r) => r.data);
export const listAdminSourceRunRecords = (id: string, params: { cursor?: string; limit?: number }, signal?: AbortSignal) => adminRequest<AdminSourceRecordPage>(queryPath(`/admin/source-runs/${encodeURIComponent(id)}/records`, params), signalInit(signal));
export const listAdminSourceRunConflicts = (id: string, params: { cursor?: string; limit?: number }, signal?: AbortSignal) => adminRequest<AdminSourceRecordPage>(queryPath(`/admin/source-runs/${encodeURIComponent(id)}/conflicts`, params), signalInit(signal));
export const listAdminSourceRunDuplicates = (id: string, params: { cursor?: string; limit?: number }, signal?: AbortSignal) => adminRequest<AdminSourceRecordPage>(queryPath(`/admin/source-runs/${encodeURIComponent(id)}/duplicates`, params), signalInit(signal));
export const getAdminSourceRunRecord = (id: string, recordId: string, signal?: AbortSignal) => adminRequest<{ data: AdminSourceRecord }>(`/admin/source-runs/${encodeURIComponent(id)}/records/${encodeURIComponent(recordId)}`, signalInit(signal)).then((r) => r.data);
export const listAdminDatasetReleases = (params: { cursor?: string; limit?: number }, signal?: AbortSignal) => adminRequest<AdminPage<AdminDatasetRelease>>(queryPath("/admin/dataset-releases", params), signalInit(signal));
export const getAdminDatasetReleaseReadiness = (version: string, signal?: AbortSignal) => adminRequest<{ data: AdminReleaseReadiness }>(`/admin/dataset-releases/${encodeURIComponent(version)}/readiness`, signalInit(signal)).then((r) => r.data);
export const publishAdminDatasetRelease = (version: string, confirmVersion: string) => adminRequest<{ data: AdminDatasetRelease }>(`/admin/dataset-releases/${encodeURIComponent(version)}/publish`, { method: "POST", body: JSON.stringify({ confirmVersion }) }).then((r) => r.data);
export const rollbackAdminDatasetRelease = (version: string, confirmVersion: string, reason: string) => adminRequest<{ data: AdminRollbackResult }>(`/admin/dataset-releases/${encodeURIComponent(version)}/rollback`, { method: "POST", body: JSON.stringify({ confirmVersion, reason }) }).then((r) => r.data);
export const advanceAdminDatasetRelease = (version: string, toStatus: "validation" | "review" | "approved", reason: string) => adminRequest<{ data: AdminDatasetRelease }>(`/admin/dataset-releases/${encodeURIComponent(version)}/advance`, { method: "POST", body: JSON.stringify({ toStatus, reason }), cache: "no-store" }).then((r) => r.data);
export const updateAdminDatasetChangelog = (version: string, changelog: string, reason: string) => adminRequest<{ data: AdminDatasetRelease }>(`/admin/dataset-releases/${encodeURIComponent(version)}/changelog`, { method: "PUT", body: JSON.stringify({ changelog, reason }), cache: "no-store" }).then((r) => r.data);
export const getAdminFairUsePolicy = (signal?: AbortSignal) => adminRequest<{ data: FairUsePolicy }>("/admin/fair-use/policy", { ...signalInit(signal), cache: "no-store" }).then((r) => r.data);
export const appendAdminFairUsePolicy = (policy: Omit<FairUsePolicy, "id" | "actorId" | "createdAt">) => adminRequest<{ data: FairUsePolicy }>("/admin/fair-use/policies", { method: "POST", body: JSON.stringify(policy), cache: "no-store" }).then((r) => r.data);
export const appendAdminFairUseOverride = (override: FairUseOverrideInput) => adminRequest<{ data: { id: string; applicationId: string; enabled: boolean; expiresAt: string } }>("/admin/fair-use/overrides", { method: "POST", body: JSON.stringify(override), cache: "no-store" }).then((r) => r.data);
const idempotentPost = <T>(path: string, body: unknown, key: string) => adminRequest<{ data: T }>(path, { method: "POST", headers: { "Idempotency-Key": key }, body: JSON.stringify(body), cache: "no-store" }).then((r) => r.data);
export const listAdminChangeRequests = (params: { cursor?: string; limit?: number; state?: string; targetKind?: string; targetId?: string; submitterId?: string; reviewerId?: string }, signal?: AbortSignal) => adminRequest<ChangeRequestPage>(queryPath("/admin/change-requests", params), { ...signalInit(signal), cache: "no-store" });
export const getAdminChangeRequest = (id: string, signal?: AbortSignal) => adminRequest<{ data: AdminChangeRequest }>(`/admin/change-requests/${encodeURIComponent(id)}`, { ...signalInit(signal), cache: "no-store" }).then((r) => r.data);
export const createAdminChangeRequest = (body: CreateChangeRequestInput, key: string) => idempotentPost<AdminChangeRequest>("/admin/change-requests", body, key);
export const transitionAdminChangeRequest = (id: string, expectedVersion: number, state: string, comment: string, key: string) => idempotentPost<AdminChangeRequest>(`/admin/change-requests/${encodeURIComponent(id)}/transitions`, { expectedVersion, state, comment }, key);
export const reviseAdminChangeRequest = (id: string, expectedVersion: number, proposedPatch: Record<string, unknown>, after: Record<string, unknown>, evidence: ChangeEvidence, comment: string, key: string) => idempotentPost<AdminChangeRequest>(`/admin/change-requests/${encodeURIComponent(id)}/revisions`, { expectedVersion, proposedPatch, after, evidence, comment }, key);
export const commentAdminChangeRequest = (id: string, expectedVersion: number, body: string, key: string) => idempotentPost<AdminChangeRequest>(`/admin/change-requests/${encodeURIComponent(id)}/comments`, { expectedVersion, body }, key);
const developerList = <T>(resource: string, params: Record<string, string | number | undefined>, signal?: AbortSignal) => adminRequest<DeveloperListPage<T>>(queryPath(`/admin/developers/${resource}`, params), { ...signalInit(signal), cache: "no-store" });
export const listDeveloperOrganizations = (params: { q?: string; cursor?: string; limit?: number }, signal?: AbortSignal) => developerList<DeveloperOrganization>("organizations", params, signal);
export const listDeveloperAccounts = (params: { q?: string; cursor?: string; limit?: number }, signal?: AbortSignal) => developerList<DeveloperAccount>("accounts", params, signal);
export const listDeveloperApplications = (params: { q?: string; cursor?: string; limit?: number; organizationId?: string }, signal?: AbortSignal) => developerList<DeveloperApplication>("applications", params, signal);
export const listDeveloperKeys = (params: { q?: string; cursor?: string; limit?: number; organizationId?: string; applicationId?: string; state?: string }, signal?: AbortSignal) => developerList<DeveloperKey>("keys", params, signal);
export const getDeveloperUsage = (organizationId: string, applicationId: string, since?: string, signal?: AbortSignal) => adminRequest<{ data: DeveloperUsage }>(queryPath("/admin/developers/usage", { organizationId, applicationId, since }), { ...signalInit(signal), cache: "no-store" }).then((r) => r.data);
export const listDeveloperRequests = (organizationId: string, applicationId: string, params: { before?: string; limit?: number }, signal?: AbortSignal) => adminRequest<{ data: DeveloperRequestEvent[]; meta: { nextBefore?: string } }>(queryPath("/admin/developers/requests", { organizationId, applicationId, ...params }), { ...signalInit(signal), cache: "no-store" });
export const mutateDeveloperKey = (keyId: string, action: "suspend" | "revoke", reason: string) => adminRequest<void>(`/admin/developers/keys/${encodeURIComponent(keyId)}/${action}`, { method: "POST", body: JSON.stringify({ reason }), cache: "no-store" });

/** Only the fields a steward may edit; the API rejects anything else. */
export interface RegionEdit {
  name?: string;
  capital?: string;
  code?: string;
  verificationStatus?: string;
}
export interface DistrictEdit {
  name?: string;
  type?: string;
  code?: string;
  capital?: string;
  regionId?: string;
  verificationStatus?: string;
}
export interface PlaceEdit {
  name?: string;
  type?: string;
  districtId?: string;
  regionId?: string;
  verificationStatus?: string;
}
export type GeographyKind = "regions" | "districts" | "places" | "roads" | "pois";
export interface GeographyCreateInput {
  id: string; name: string; countryCode?: string; capital?: string; code?: string; regionId?: string; regionName?: string;
  districtId?: string; districtName?: string; districtType?: string; placeType?: string;
  roadClass?: string; ref?: string; poiClass?: string; category?: string; attribution?: string;
  centroid?: { latitude: number; longitude: number };
  geometry?: { type: string; coordinates: unknown };
  provenance: { sourceId: string; externalId: string; sourceUrl?: string; retrievedAt: string; sourcePayloadHash: string; notes?: string };
}
export interface AdminAlias { id: string; placeId: string; value: string; normalizedValue: string; type: string; language: string; isPreferred: boolean; status: "ACTIVE" | "DEPRECATED" }
export interface AdminRedirect { oldId: string; kind: string; newId: string; reason: string; mergedAt: string }
export interface BoundaryGeometry { type: "Polygon" | "MultiPolygon"; coordinates: unknown }

const geographyMutationHeaders = () => ({ "Idempotency-Key": crypto.randomUUID() });

export const updateRegion = (id: string, changes: RegionEdit) =>
  adminRequest<{ data: unknown }>(`/admin/regions/${encodeURIComponent(id)}`, {
    method: "PATCH",
    headers: geographyMutationHeaders(),
    body: JSON.stringify(changes),
  });

export const updateDistrict = (id: string, changes: DistrictEdit) =>
  adminRequest<{ data: unknown }>(`/admin/districts/${encodeURIComponent(id)}`, {
    method: "PATCH",
    headers: geographyMutationHeaders(),
    body: JSON.stringify(changes),
  });

export const updatePlace = (id: string, changes: PlaceEdit) =>
  adminRequest<{ data: unknown }>(`/admin/places/${encodeURIComponent(id)}`, {
    method: "PATCH",
    headers: geographyMutationHeaders(),
    body: JSON.stringify(changes),
  });

/**
 * Deprecate a place. There is no delete — `mergedInto` records the survivor so
 * the old id keeps resolving to a 410 that names it.
 */
export const createAdminGeography = (kind: GeographyKind, body: GeographyCreateInput) => adminRequest<{ data: { id: string; kind: string } }>(`/admin/geography/${kind}`, { method: "POST", headers: geographyMutationHeaders(), body: JSON.stringify(body), cache: "no-store" }).then((r) => r.data);
export const deprecateAdminGeography = (kind: GeographyKind, id: string, mergedInto: string, reason: string) =>
  adminRequest<{ message: string }>(`/admin/geography/${kind}/${encodeURIComponent(id)}/deprecate`, {
    method: "POST",
    headers: geographyMutationHeaders(),
    body: JSON.stringify({ mergedInto, reason }),
  });
export const deprecatePlace = (id: string, mergedInto: string, reason: string) => deprecateAdminGeography("places", id, mergedInto, reason);
export const listAdminRedirects = (params: { cursor?: string; limit?: number }, signal?: AbortSignal) => adminRequest<AdminPage<AdminRedirect>>(queryPath("/admin/redirects", params), { ...signalInit(signal), cache: "no-store" });
export const listAdminAliases = (placeId: string, signal?: AbortSignal) => adminRequest<{ data: AdminAlias[] }>(`/admin/places/${encodeURIComponent(placeId)}/aliases`, { ...signalInit(signal), cache: "no-store" }).then((r) => r.data);
export const createAdminAlias = (placeId: string, body: { id: string; value: string; type?: string; language?: string; isPreferred?: boolean }) => adminRequest<{ data: { id: string } }>(`/admin/places/${encodeURIComponent(placeId)}/aliases`, { method: "POST", headers: geographyMutationHeaders(), body: JSON.stringify(body), cache: "no-store" }).then((r) => r.data);
export const deprecateAdminAlias = (placeId: string, aliasId: string) => adminRequest<{ message: string }>(`/admin/places/${encodeURIComponent(placeId)}/aliases/${encodeURIComponent(aliasId)}/deprecate`, { method: "POST", headers: geographyMutationHeaders(), body: "{}", cache: "no-store" });
export async function getAdminBoundary(kind: "region" | "district", id: string, signal?: AbortSignal): Promise<{ geometry: BoundaryGeometry | null; etag: string }> { const res = await fetch(`${BASE}/admin/boundaries/${kind}/${encodeURIComponent(id)}`, { credentials: "include", cache: "no-store", ...(signal ? { signal } : {}) }); const body = await res.json().catch(() => null); if (!res.ok) throw new AdminApiError(body?.error?.code ?? "INTERNAL", body?.error?.message ?? `Request failed (${res.status})`, body?.error?.requestId); return { geometry: body.data as BoundaryGeometry | null, etag: res.headers.get("etag") ?? "" }; }
export async function updateAdminBoundary(kind: "region" | "district", id: string, geometry: BoundaryGeometry, etag: string): Promise<{ geometry: BoundaryGeometry; etag: string }> { const res = await fetch(`${BASE}/admin/boundaries/${kind}/${encodeURIComponent(id)}`, { method: "PUT", credentials: "include", cache: "no-store", headers: { "Content-Type": "application/json", "If-Match": etag }, body: JSON.stringify(geometry) }); const body = await res.json().catch(() => null); if (!res.ok) throw new AdminApiError(body?.error?.code ?? "INTERNAL", body?.error?.message ?? `Request failed (${res.status})`, body?.error?.requestId, body?.error?.details); return { geometry: body.data, etag: res.headers.get("etag") ?? "" }; }

/* Single-record reads. These go through the same proxy so a steward viewing a
   record and editing it use one origin and one session. */

export interface RegionRecord {
  id: string; name: string; capital?: string; code?: string;
  status: string; verificationStatus: string;
  provenance?: { sourceId?: string; sourceUrl?: string };
}
export interface DistrictRecord {
  id: string; name: string; type?: string; code?: string; capital?: string;
  region?: { id: string; name: string };
  status: string; verificationStatus: string;
  provenance?: { sourceId?: string; sourceUrl?: string };
}
export interface PlaceRecord {
  id: string; name: string; type: string;
  region?: { id: string; name: string };
  district?: { id: string; name: string };
  centroid?: { latitude: number; longitude: number };
  status: string; verificationStatus: string;
  provenance?: { sourceId?: string; sourceUrl?: string };
}

/* A single-record GET returns the record BARE; only collections and the admin
   write responses wrap it in `data`. Unwrapping a key that is not there
   yielded undefined, which the async state rendered as "Nothing to show" —
   a 200 that looked like an empty result. */
export const fetchRegion = (id: string) =>
  adminRequest<RegionRecord>(`/regions/${encodeURIComponent(id)}`);
export const fetchDistrict = (id: string) =>
  adminRequest<DistrictRecord>(`/districts/${encodeURIComponent(id)}`);
export const fetchPlace = (id: string) =>
  adminRequest<PlaceRecord>(`/places/${encodeURIComponent(id)}`);

/** The verification ladder. Seed rows are never promoted automatically (R5). */
export const VERIFICATION_OPTIONS = [
  { value: "REFERENCE", label: "Reference", hint: "From a cited source, not yet reviewed" },
  { value: "SEED_NEEDS_CANONICAL_RECONCILIATION", label: "Needs reconciliation", hint: "Seed row awaiting a steward" },
  { value: "REVIEWED", label: "Reviewed", hint: "A steward has checked it" },
  { value: "CANONICAL", label: "Canonical", hint: "Authoritative" },
];

export const PLACE_TYPE_OPTIONS = [
  "CITY", "TOWN", "VILLAGE", "COMMUNITY", "SUBURB", "NEIGHBOURHOOD",
  "HAMLET", "SETTLEMENT", "LOCALITY", "REGIONAL_CAPITAL",
].map((v) => ({ value: v, label: v.replace(/_/g, " ").toLowerCase() }));
