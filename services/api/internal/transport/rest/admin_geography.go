package rest

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	appgeo "github.com/ghanageo/ghanageo/services/api/internal/app/geography"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/account"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/apierr"
)

// Admin geography mutations (story GEO-17.3).
//
// These sit under /v1/admin and require a SESSION, never an API key: editing
// canonical data is a human steward's act, and an audit row naming a key
// rather than a person is not accountability.

// requireSteward authenticates the session and confirms the permission.
//
// The check happens HERE as well as in the use case. That is deliberate
// duplication: the transport refuses early so an unauthorised caller never
// reaches the domain, and the use case refuses independently so a future
// transport that forgets cannot create a hole.
func (h *Handler) requireSteward(
	w http.ResponseWriter, r *http.Request, p account.Permission,
) (appgeo.Actor, bool) {
	_, a, ok := h.requireSession(w, r)
	if !ok {
		return appgeo.Actor{}, false
	}
	if !account.Can(a.Role, p) {
		writeErr(w, r, apierr.New(apierr.PermissionDenied,
			"Your role cannot perform this action.").
			WithDetail("requiredPermission", string(p)))
		return appgeo.Actor{}, false
	}
	return appgeo.Actor{
		ID: a.ID, Email: a.Email, Role: a.Role,
		IP: clientIP(r), RequestID: middleware.GetReqID(r.Context()),
	}, true
}

func (h *Handler) adminUpdateRegion(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.requireSteward(w, r, account.PermEditGeography)
	if !ok {
		return
	}
	var body struct {
		Name               *string `json:"name"`
		Capital            *string `json:"capital"`
		OfficialCode       *string `json:"code"`
		VerificationStatus *string `json:"verificationStatus"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	out, err := h.geo.UpdateRegion(r.Context(), actor, chi.URLParam(r, "id"), appgeo.RegionChanges{
		Name: body.Name, Capital: body.Capital,
		OfficialCode: body.OfficialCode, VerificationStatus: body.VerificationStatus,
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": regionOut(*out)})
}

func (h *Handler) adminUpdateDistrict(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.requireSteward(w, r, account.PermEditGeography)
	if !ok {
		return
	}
	var body struct {
		Name               *string `json:"name"`
		DistrictType       *string `json:"type"`
		OfficialCode       *string `json:"code"`
		Capital            *string `json:"capital"`
		RegionID           *string `json:"regionId"`
		VerificationStatus *string `json:"verificationStatus"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	out, err := h.geo.UpdateDistrict(r.Context(), actor, chi.URLParam(r, "id"), appgeo.DistrictChanges{
		Name: body.Name, DistrictType: body.DistrictType, OfficialCode: body.OfficialCode,
		Capital: body.Capital, RegionID: body.RegionID,
		VerificationStatus: body.VerificationStatus,
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": districtOut(*out)})
}

func (h *Handler) adminUpdatePlace(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.requireSteward(w, r, account.PermEditGeography)
	if !ok {
		return
	}
	var body struct {
		Name               *string `json:"name"`
		Type               *string `json:"type"`
		DistrictID         *string `json:"districtId"`
		RegionID           *string `json:"regionId"`
		VerificationStatus *string `json:"verificationStatus"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	out, err := h.geo.UpdatePlace(r.Context(), actor, chi.URLParam(r, "id"), appgeo.PlaceChanges{
		Name: body.Name, Type: body.Type, DistrictID: body.DistrictID,
		RegionID: body.RegionID, VerificationStatus: body.VerificationStatus,
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": placeOut(*out)})
}

// adminDeprecatePlace retires a place. There is no DELETE, by design: a hard
// delete turns every stored reference into a 404 with no way to find the
// successor, which is what redirects exist to prevent (GEO-3.3).
func (h *Handler) adminDeprecatePlace(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.requireSteward(w, r, account.PermEditGeography)
	if !ok {
		return
	}
	var body struct {
		MergedInto string `json:"mergedInto"`
		Reason     string `json:"reason"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if err := h.geo.DeprecatePlace(r.Context(), actor, chi.URLParam(r, "id"),
		body.MergedInto, body.Reason); err != nil {
		writeErr(w, r, err)
		return
	}
	msg := "Place deprecated. Its id now returns 410."
	if body.MergedInto != "" {
		msg = "Place merged. Its id now returns 410 with mergedInto."
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": msg})
}

// adminPermissions tells the console what the signed-in steward may do, so it
// can hide controls that would be refused. Presentation only — every one of
// these is enforced again at the endpoint.
func (h *Handler) adminPermissions(w http.ResponseWriter, r *http.Request) {
	_, a, ok := h.requireSession(w, r)
	if !ok {
		return
	}
	perms := account.Permissions(a.Role)
	out := make([]string, 0, len(perms))
	for _, p := range perms {
		out = append(out, string(p))
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"role": string(a.Role), "permissions": out,
	}})
}
