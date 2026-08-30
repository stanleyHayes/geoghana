package rest

import (
	"net/http"
	"strconv"

	app "github.com/ghanageo/ghanageo/services/api/internal/app/adminops"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/account"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/audit"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/apierr"
	"github.com/go-chi/chi/v5"
)

func adminPrivateResponse(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Add("Vary", "Cookie")
		next.ServeHTTP(w, r)
	})
}

func pageQuery(r *http.Request) app.PageQuery {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	return app.PageQuery{Cursor: r.URL.Query().Get("cursor"), Limit: limit}
}

func (h *Handler) requireAdminOps(w http.ResponseWriter, r *http.Request, p account.Permission) bool {
	if h.adminOps == nil {
		writeErr(w, r, apierr.New(apierr.Internal, "Admin operations are not configured."))
		return false
	}
	_, ok := h.requireSteward(w, r, p)
	return ok
}

func (h *Handler) adminDashboard(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdminOps(w, r, account.PermViewOperations) {
		return
	}
	v, err := h.adminOps.Dashboard(r.Context())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	var current any
	if v.CurrentRelease != nil {
		current = toDatasetJSON(*v.CurrentRelease)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"counts": v.Counts, "currentRelease": current, "recentActivity": auditSummaryList(v.RecentActivity)}})
}
func (h *Handler) adminSystemHealth(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdminOps(w, r, account.PermViewOperations) {
		return
	}
	v, err := h.adminOps.Health(r.Context())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": v})
}
func (h *Handler) adminAuditLog(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdminOps(w, r, account.PermViewAudit) {
		return
	}
	q := app.AuditQuery{PageQuery: pageQuery(r), Actor: r.URL.Query().Get("actor"), Action: r.URL.Query().Get("action"), Target: r.URL.Query().Get("target"), Outcome: r.URL.Query().Get("outcome")}
	p, err := h.adminOps.Audit(r.Context(), q)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": auditList(p.Data), "meta": map[string]any{"nextCursor": p.NextCursor}})
}
func (h *Handler) adminSourceRuns(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdminOps(w, r, account.PermViewOperations) {
		return
	}
	p, err := h.adminOps.SourceRuns(r.Context(), pageQuery(r))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": p.Data, "meta": map[string]any{"nextCursor": p.NextCursor}})
}
func (h *Handler) adminSourceRun(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdminOps(w, r, account.PermViewOperations) {
		return
	}
	v, err := h.adminOps.SourceRun(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": v})
}
func (h *Handler) adminSourceRecords(w http.ResponseWriter, r *http.Request) {
	h.adminSourceRecordPage(w, r, "")
}
func (h *Handler) adminSourceConflicts(w http.ResponseWriter, r *http.Request) {
	h.adminSourceRecordPage(w, r, "unresolved_region")
}
func (h *Handler) adminSourceDuplicates(w http.ResponseWriter, r *http.Request) {
	h.adminSourceRecordPage(w, r, "duplicate_candidate")
}
func (h *Handler) adminSourceRecordPage(w http.ResponseWriter, r *http.Request, reasonCode string) {
	if !h.requireAdminOps(w, r, account.PermViewOperations) {
		return
	}
	p, err := h.adminOps.SourceRecords(r.Context(), chi.URLParam(r, "id"), app.SourceRecordQuery{PageQuery: pageQuery(r), ReasonCode: reasonCode})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": p.Data, "meta": map[string]any{"nextCursor": p.NextCursor, "detailAvailability": p.DetailAvailability}})
}
func (h *Handler) adminSourceRecord(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdminOps(w, r, account.PermViewOperations) {
		return
	}
	v, err := h.adminOps.SourceRecord(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "recordId"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": v})
}
func (h *Handler) adminDatasetReleases(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdminOps(w, r, account.PermViewOperations) {
		return
	}
	p, err := h.adminOps.DatasetReleases(r.Context(), pageQuery(r))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	out := make([]datasetJSON, 0, len(p.Data))
	for _, v := range p.Data {
		out = append(out, toDatasetJSON(v))
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out, "meta": map[string]any{"nextCursor": p.NextCursor}})
}
func auditList(entries []audit.Entry) []map[string]any {
	out := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		out = append(out, map[string]any{"id": e.ID, "at": e.At, "actor": map[string]any{"kind": e.Actor.Kind, "id": e.Actor.ID, "label": e.Actor.Label, "ip": e.Actor.IP}, "action": e.Action, "target": map[string]any{"kind": e.Target.Kind, "id": e.Target.ID, "label": e.Target.Label}, "requestId": e.RequestID, "before": e.Before, "after": e.After, "reason": e.Reason, "outcome": e.Outcome, "error": e.Error, "hash": e.Hash, "previousHash": e.PrevHash})
	}
	return out
}

// auditSummaryList excludes IP addresses and before/after payloads. Dashboard
// access is broader than audit-log access, so it cannot be a side door around
// the narrower audit:view permission.
func auditSummaryList(entries []audit.Entry) []map[string]any {
	out := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		out = append(out, map[string]any{"id": e.ID, "at": e.At, "actor": map[string]any{"kind": e.Actor.Kind, "id": e.Actor.ID, "label": e.Actor.Label}, "action": e.Action, "target": map[string]any{"kind": e.Target.Kind, "id": e.Target.ID, "label": e.Target.Label}, "outcome": e.Outcome})
	}
	return out
}
