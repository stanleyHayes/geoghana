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

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
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
    throw new AdminApiError(
      e?.code ?? "INTERNAL",
      e?.message ?? `Request failed (${res.status})`,
      e?.requestId,
      e?.details,
    );
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
  request<{ data: SessionInfo }>("/auth/session").then((r) => r.data);

export const getPermissions = () =>
  request<{ data: PermissionInfo }>("/admin/permissions").then((r) => r.data);

export const login = (email: string, password: string) =>
  request<{ mfaRequired: boolean; mfaEnrolmentRequired: boolean; expiresAt: string }>(
    "/auth/login",
    { method: "POST", body: JSON.stringify({ email, password }) },
  );

export const completeTotp = (code: string) =>
  request<{ expiresAt: string }>("/auth/mfa/totp", {
    method: "POST",
    body: JSON.stringify({ code }),
  });

export const logout = () => request<{ message: string }>("/auth/logout", { method: "POST" });

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

export const updateRegion = (id: string, changes: RegionEdit) =>
  request<{ data: unknown }>(`/admin/regions/${encodeURIComponent(id)}`, {
    method: "PATCH",
    body: JSON.stringify(changes),
  });

export const updateDistrict = (id: string, changes: DistrictEdit) =>
  request<{ data: unknown }>(`/admin/districts/${encodeURIComponent(id)}`, {
    method: "PATCH",
    body: JSON.stringify(changes),
  });

export const updatePlace = (id: string, changes: PlaceEdit) =>
  request<{ data: unknown }>(`/admin/places/${encodeURIComponent(id)}`, {
    method: "PATCH",
    body: JSON.stringify(changes),
  });

/**
 * Deprecate a place. There is no delete — `mergedInto` records the survivor so
 * the old id keeps resolving to a 410 that names it.
 */
export const deprecatePlace = (id: string, mergedInto: string, reason: string) =>
  request<{ message: string }>(`/admin/places/${encodeURIComponent(id)}/deprecate`, {
    method: "POST",
    body: JSON.stringify({ mergedInto, reason }),
  });

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
  request<RegionRecord>(`/regions/${encodeURIComponent(id)}`);
export const fetchDistrict = (id: string) =>
  request<DistrictRecord>(`/districts/${encodeURIComponent(id)}`);
export const fetchPlace = (id: string) =>
  request<PlaceRecord>(`/places/${encodeURIComponent(id)}`);

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
