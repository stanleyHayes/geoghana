package rest

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	appaccount "github.com/ghanageo/ghanageo/services/api/internal/app/account"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/account"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/apierr"
)

// Staff administration.
//
// role:manage was defined, granted to SUPER_ADMIN and referenced nowhere, so the
// read-only SECURITY_AUDITOR separation the RBAC table argues for could not be
// granted to anyone, and a suspect account could only be dealt with by revoking
// its API keys one at a time. These two endpoints are the callers that
// AccountRepo.SetRole and SetDisabled were always missing.

func (h *Handler) staffActor(w http.ResponseWriter, r *http.Request) (appaccount.StaffActor, bool) {
	actor, ok := h.requireSteward(w, r, account.PermManageRoles)
	if !ok {
		return appaccount.StaffActor{}, false
	}
	return appaccount.StaffActor{
		ID:        actor.ID,
		Email:     actor.Email,
		IP:        clientIP(r),
		RequestID: middleware.GetReqID(r.Context()),
	}, true
}

func (h *Handler) adminSetStaffRole(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.staffActor(w, r)
	if !ok {
		return
	}
	if h.accounts == nil {
		writeErr(w, r, apierr.New(apierr.Internal, "Account administration is not configured."))
		return
	}
	var body struct {
		Role string `json:"role"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	err := h.accounts.SetStaffRole(r.Context(), actor, chi.URLParam(r, "id"), account.Role(body.Role))
	if !writeStaffErr(w, r, err) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *Handler) adminSetStaffDisabled(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.staffActor(w, r)
	if !ok {
		return
	}
	if h.accounts == nil {
		writeErr(w, r, apierr.New(apierr.Internal, "Account administration is not configured."))
		return
	}
	var body struct {
		Disabled bool `json:"disabled"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	err := h.accounts.SetStaffDisabled(r.Context(), actor, chi.URLParam(r, "id"), body.Disabled)
	if !writeStaffErr(w, r, err) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "disabled": body.Disabled})
}

// writeStaffErr maps the domain refusals onto the wire and reports whether the
// caller should continue. Refusing to act on yourself is a conflict rather than a
// permission denial: the role is sufficient, the target is the problem.
func writeStaffErr(w http.ResponseWriter, r *http.Request, err error) bool {
	switch {
	case err == nil:
		return true
	case errors.Is(err, appaccount.ErrCannotActOnSelf):
		writeErr(w, r, apierr.New(apierr.Conflict,
			"An operator cannot change their own role or access. Ask another administrator."))
	case errors.Is(err, appaccount.ErrInvalidStaffRole):
		writeErr(w, r, apierr.New(apierr.InvalidArgument, "That is not a role this system recognises."))
	case errors.Is(err, appaccount.ErrUnknownStaffTarget):
		writeErr(w, r, apierr.New(apierr.NotFound, "No such account."))
	default:
		writeErr(w, r, apierr.New(apierr.Internal, "That change could not be applied."))
	}
	return false
}
