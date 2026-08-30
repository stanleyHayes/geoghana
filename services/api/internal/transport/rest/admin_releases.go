package rest

import (
	"net/http"
	"time"

	appdataset "github.com/ghanageo/ghanageo/services/api/internal/app/dataset"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/account"
	domaindataset "github.com/ghanageo/ghanageo/services/api/internal/domain/dataset"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/apierr"
	"github.com/go-chi/chi/v5"
)

func (h *Handler) releaseActor(w http.ResponseWriter, r *http.Request, permission account.Permission) (appdataset.Actor, bool) {
	a, ok := h.requireSteward(w, r, permission)
	if !ok {
		return appdataset.Actor{}, false
	}
	if h.datasets == nil {
		writeErr(w, r, apierr.New(apierr.Internal, "Dataset releases are not configured."))
		return appdataset.Actor{}, false
	}
	return appdataset.Actor{ID: a.ID, Email: a.Email, IP: a.IP, RequestID: a.RequestID}, true
}

func (h *Handler) adminDatasetReleaseReadiness(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdminOps(w, r, account.PermViewOperations) {
		return
	}
	if h.datasets == nil {
		writeErr(w, r, apierr.New(apierr.Internal, "Dataset releases are not configured."))
		return
	}
	report, err := h.datasets.Readiness(r.Context(), chi.URLParam(r, "version"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": report})
}

func (h *Handler) adminPublishDatasetRelease(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.releaseActor(w, r, account.PermPublishRelease)
	if !ok {
		return
	}
	version := chi.URLParam(r, "version")
	var body struct {
		ConfirmVersion string `json:"confirmVersion"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	v, err := h.datasets.PublishAs(r.Context(), actor, version, body.ConfirmVersion, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": toDatasetJSON(v)})
}

func (h *Handler) adminRollbackDatasetRelease(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.releaseActor(w, r, account.PermRollbackRelease)
	if !ok {
		return
	}
	version := chi.URLParam(r, "version")
	var body struct {
		ConfirmVersion string `json:"confirmVersion"`
		Reason         string `json:"reason"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	from, restored, err := h.datasets.RollbackAs(r.Context(), actor, version, body.ConfirmVersion, body.Reason, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"from": toDatasetJSON(from), "restored": toDatasetJSON(restored)}})
}

func (h *Handler) adminAdvanceDatasetRelease(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.releaseActor(w, r, account.PermPublishRelease)
	if !ok {
		return
	}
	var body struct {
		ToStatus string `json:"toStatus"`
		Reason   string `json:"reason"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	v, err := h.datasets.AdvanceAs(r.Context(), actor, chi.URLParam(r, "version"), domaindataset.Status(body.ToStatus), body.Reason)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": toDatasetJSON(v)})
}

func (h *Handler) adminUpdateDatasetChangelog(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.releaseActor(w, r, account.PermPublishRelease)
	if !ok {
		return
	}
	var body struct {
		Changelog string `json:"changelog"`
		Reason    string `json:"reason"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	v, err := h.datasets.UpdateChangelogAs(r.Context(), actor, chi.URLParam(r, "version"), body.Changelog, body.Reason)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": toDatasetJSON(v)})
}
