package rest

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/account"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/identity"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/usage"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/apierr"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func (h *Handler) requireAdminIdentity(w http.ResponseWriter, r *http.Request, permission account.Permission) (account.Account, bool) {
	if h.adminIdentity == nil {
		writeErr(w, r, apierr.New(apierr.Internal, "Developer administration is not configured."))
		return account.Account{}, false
	}
	_, actor, ok := h.requireSession(w, r)
	if !ok {
		return account.Account{}, false
	}
	if !account.Can(actor.Role, permission) {
		writeErr(w, r, apierr.New(apierr.PermissionDenied, "Your role cannot perform this action.").WithDetail("requiredPermission", string(permission)))
		return account.Account{}, false
	}
	return actor, true
}

func adminIdentityFilter(r *http.Request) (identity.AdminListFilter, error) {
	limit, err := parseAdminIdentityLimit(r.URL.Query().Get("limit"))
	if err != nil {
		return identity.AdminListFilter{}, err
	}
	return identity.AdminListFilter{
		Cursor: r.URL.Query().Get("cursor"), Query: r.URL.Query().Get("q"),
		OrganizationID: r.URL.Query().Get("organizationId"), ApplicationID: r.URL.Query().Get("applicationId"),
		State: r.URL.Query().Get("state"), Limit: limit,
	}, nil
}

func writeAdminPage(w http.ResponseWriter, data any, cursor string, total int64) {
	writeJSON(w, http.StatusOK, map[string]any{"data": data, "meta": map[string]any{"nextCursor": cursor, "total": total}})
}

func (h *Handler) adminDeveloperOrganizations(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.requireAdminIdentity(w, r, account.PermViewOrganization)
	if !ok {
		return
	}
	filter, err := adminIdentityFilter(r)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	p, err := h.adminIdentity.Organizations(r.Context(), actor, filter)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	out := make([]map[string]any, 0, len(p.Data))
	for _, v := range p.Data {
		out = append(out, adminOrganizationOut(v))
	}
	writeAdminPage(w, out, p.NextCursor, p.Total)
}

func (h *Handler) adminDeveloperAccounts(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.requireAdminIdentity(w, r, account.PermViewOrganization)
	if !ok {
		return
	}
	filter, err := adminIdentityFilter(r)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	p, err := h.adminIdentity.Accounts(r.Context(), actor, filter)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	out := make([]map[string]any, 0, len(p.Data))
	for _, v := range p.Data {
		out = append(out, adminAccountOut(v))
	}
	writeAdminPage(w, out, p.NextCursor, p.Total)
}

func (h *Handler) adminDeveloperApplications(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.requireAdminIdentity(w, r, account.PermViewOrganization)
	if !ok {
		return
	}
	filter, err := adminIdentityFilter(r)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	p, err := h.adminIdentity.Applications(r.Context(), actor, filter)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	out := make([]map[string]any, 0, len(p.Data))
	for _, v := range p.Data {
		out = append(out, adminApplicationOut(v))
	}
	writeAdminPage(w, out, p.NextCursor, p.Total)
}

func (h *Handler) adminDeveloperKeys(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.requireAdminIdentity(w, r, account.PermViewOrganization)
	if !ok {
		return
	}
	filter, err := adminIdentityFilter(r)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	p, err := h.adminIdentity.Keys(r.Context(), actor, filter)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	out := make([]map[string]any, 0, len(p.Data))
	for _, v := range p.Data {
		out = append(out, adminKeyOut(v))
	}
	writeAdminPage(w, out, p.NextCursor, p.Total)
}

func (h *Handler) adminDeveloperUsage(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.requireAdminIdentity(w, r, account.PermViewOrganization)
	if !ok {
		return
	}
	since := time.Now().UTC().Add(-usage.Retention)
	if raw := strings.TrimSpace(r.URL.Query().Get("since")); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			writeErr(w, r, apierr.New(apierr.InvalidArgument, "Since must be an RFC3339 timestamp."))
			return
		}
		since = parsed
	}
	v, err := h.adminIdentity.Usage(r.Context(), actor, r.URL.Query().Get("organizationId"), r.URL.Query().Get("applicationId"), since)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": adminUsageOut(v)})
}

func (h *Handler) adminDeveloperRequests(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.requireAdminIdentity(w, r, account.PermViewOrganization)
	if !ok {
		return
	}
	var before *time.Time
	if raw := strings.TrimSpace(r.URL.Query().Get("before")); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			writeErr(w, r, apierr.New(apierr.InvalidArgument, "Before must be an RFC3339 timestamp."))
			return
		}
		before = &parsed
	}
	limit, err := parseAdminIdentityLimit(r.URL.Query().Get("limit"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	p, err := h.adminIdentity.Requests(r.Context(), actor, r.URL.Query().Get("organizationId"), r.URL.Query().Get("applicationId"), before, limit)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	out := make([]map[string]any, 0, len(p.Data))
	for _, v := range p.Data {
		out = append(out, adminRequestOut(v))
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out, "meta": map[string]any{"nextBefore": p.NextBefore}})
}

func parseAdminIdentityLimit(raw string) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return 20, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > 100 {
		return 0, apierr.New(apierr.InvalidArgument, "Limit must be between 1 and 100.")
	}
	return n, nil
}

func (h *Handler) adminSuspendDeveloperKey(w http.ResponseWriter, r *http.Request) {
	h.adminMutateDeveloperKey(w, r, false)
}
func (h *Handler) adminRevokeDeveloperKey(w http.ResponseWriter, r *http.Request) {
	h.adminMutateDeveloperKey(w, r, true)
}
func (h *Handler) adminMutateDeveloperKey(w http.ResponseWriter, r *http.Request, revoke bool) {
	actor, ok := h.requireAdminIdentity(w, r, account.PermSuspendKey)
	if !ok {
		return
	}
	var body struct {
		Reason string `json:"reason"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if len(strings.TrimSpace(body.Reason)) > 1000 {
		writeErr(w, r, apierr.New(apierr.InvalidArgument, "Reason must be 1000 characters or fewer."))
		return
	}
	keyID := chi.URLParam(r, "keyId")
	requestID := middleware.GetReqID(r.Context())
	var err error
	if revoke {
		err = h.adminIdentity.RevokeKey(r.Context(), actor, keyID, body.Reason, requestID, clientIP(r))
	} else {
		err = h.adminIdentity.SuspendKey(r.Context(), actor, keyID, body.Reason, requestID, clientIP(r))
	}
	if err != nil {
		writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func adminOrganizationOut(v identity.Organization) map[string]any {
	members := make([]map[string]any, 0, len(v.Members))
	for _, m := range v.Members {
		members = append(members, map[string]any{"accountId": m.AccountID, "email": m.Email, "role": m.Role, "joinedAt": m.JoinedAt})
	}
	return map[string]any{"id": v.ID, "name": v.Name, "ownerId": v.OwnerID, "members": members, "createdAt": v.CreatedAt}
}
func adminAccountOut(v identity.DeveloperAccount) map[string]any {
	return map[string]any{"id": v.ID, "email": v.Email, "emailVerified": v.EmailVerified, "role": v.Role, "disabled": v.Disabled, "createdAt": v.CreatedAt, "updatedAt": v.UpdatedAt}
}
func adminApplicationOut(v identity.Application) map[string]any {
	environments := v.Environments
	if environments == nil {
		environments = []identity.Environment{}
	}
	domains := v.Domains
	if domains == nil {
		domains = []string{}
	}
	return map[string]any{"id": v.ID, "organizationId": v.OrganizationID, "name": v.Name, "description": v.Description, "environments": environments, "domains": domains, "callbackUrl": v.CallbackURL, "createdAt": v.CreatedAt}
}
func adminKeyOut(v identity.AdminKey) map[string]any {
	state := "active"
	if v.RevokedAt != nil {
		state = "revoked"
	} else if v.SuspendedAt != nil {
		state = "suspended"
	}
	return map[string]any{"id": v.ID, "applicationId": v.ApplicationID, "organizationId": v.OrganizationID, "name": v.Name, "class": v.Class, "environment": v.Environment, "prefix": v.Prefix, "scopes": v.Scopes, "allowedOrigins": v.AllowedOrigins, "allowedIps": v.AllowedIPs, "state": state, "createdAt": v.CreatedAt, "expiresAt": v.ExpiresAt, "lastUsedAt": v.LastUsedAt, "revokedAt": v.RevokedAt, "revokedReason": v.RevokedReason, "suspendedAt": v.SuspendedAt, "suspendedReason": v.SuspendedReason}
}
func adminRequestOut(v usage.Event) map[string]any {
	return map[string]any{"id": v.ID, "requestId": v.RequestID, "organizationId": v.OrganizationID, "applicationId": v.ApplicationID, "keyId": v.KeyID, "protocol": v.Protocol, "operation": v.Operation, "status": v.Status, "success": v.Success, "latencyMs": v.LatencyMS, "quotaCost": v.QuotaCost, "quotaLimit": v.QuotaLimit, "quotaRemaining": v.QuotaRemaining, "geography": v.Geography, "at": v.At}
}

func adminUsageOut(v usage.Summary) map[string]any {
	breakdowns := func(values []usage.Breakdown) []map[string]any {
		out := make([]map[string]any, 0, len(values))
		for _, item := range values {
			out = append(out, map[string]any{"label": item.Label, "requests": item.Requests, "errors": item.Errors, "quotaCost": item.QuotaCost, "avgLatencyMs": item.AvgLatencyMS})
		}
		return out
	}
	return map[string]any{
		"since": v.Since, "requests": v.Requests, "errors": v.Errors,
		"quotaCost": v.QuotaCost, "avgLatencyMs": v.AvgLatencyMS,
		"byProtocol": breakdowns(v.ByProtocol), "byEndpoint": breakdowns(v.ByEndpoint), "byGeography": breakdowns(v.ByGeography),
	}
}
