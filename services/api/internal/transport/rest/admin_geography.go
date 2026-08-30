package rest

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	appgeo "github.com/ghanageo/ghanageo/services/api/internal/app/geography"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/account"
	domaingeo "github.com/ghanageo/ghanageo/services/api/internal/domain/geography"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/apierr"
)

func (h *Handler) requireGeographyMutation(w http.ResponseWriter, r *http.Request) (appgeo.Actor, bool) {
	a, ok := h.requireSteward(w, r, account.PermEditGeography)
	if !ok {
		return appgeo.Actor{}, false
	}
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" {
		writeErr(w, r, apierr.New(apierr.InvalidArgument, "Idempotency-Key is required."))
		return appgeo.Actor{}, false
	}
	a.RequestID = idempotencyRequestID(a.ID, key)
	return a, true
}

type geographyCreateBody struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	CountryCode  string `json:"countryCode"`
	Capital      string `json:"capital"`
	Code         string `json:"code"`
	RegionID     string `json:"regionId"`
	RegionName   string `json:"regionName"`
	DistrictID   string `json:"districtId"`
	DistrictName string `json:"districtName"`
	DistrictType string `json:"districtType"`
	PlaceType    string `json:"placeType"`
	RoadClass    string `json:"roadClass"`
	Ref          string `json:"ref"`
	POIClass     string `json:"poiClass"`
	Category     string `json:"category"`
	Attribution  string `json:"attribution"`
	Centroid     *struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	} `json:"centroid"`
	Geometry *struct {
		Type        domaingeo.GeometryType `json:"type"`
		Coordinates json.RawMessage        `json:"coordinates"`
	} `json:"geometry"`
	Provenance struct {
		SourceID          string `json:"sourceId"`
		ExternalID        string `json:"externalId"`
		SourceURL         string `json:"sourceUrl"`
		RetrievedAt       string `json:"retrievedAt"`
		SourcePayloadHash string `json:"sourcePayloadHash"`
		Notes             string `json:"notes"`
	} `json:"provenance"`
}

func geometryFromWire(w *struct {
	Type        domaingeo.GeometryType `json:"type"`
	Coordinates json.RawMessage        `json:"coordinates"`
}) (*domaingeo.Geometry, error) {
	if w == nil {
		return nil, nil
	}
	g := &domaingeo.Geometry{Type: w.Type}
	var target any
	switch w.Type {
	case domaingeo.GeomPoint:
		target = &[]float64{}
	case domaingeo.GeomLineString:
		target = &[][]float64{}
	case domaingeo.GeomPolygon:
		target = &[][][]float64{}
	case domaingeo.GeomMultiPolygon:
		target = &[][][][]float64{}
	case domaingeo.GeomMultiLineString:
		target = &[][][]float64{}
	default:
		return nil, apierr.New(apierr.InvalidArgument, "Unsupported GeoJSON geometry type.")
	}
	if err := json.Unmarshal(w.Coordinates, target); err != nil {
		return nil, apierr.New(apierr.InvalidArgument, "Coordinates must be valid GeoJSON numeric arrays.")
	}
	switch v := target.(type) {
	case *[]float64:
		g.Coordinates = *v
	case *[][]float64:
		g.Coordinates = *v
	case *[][][]float64:
		g.Coordinates = *v
	case *[][][][]float64:
		g.Coordinates = *v
	}
	return g, nil
}

func (h *Handler) adminCreateGeography(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.requireSteward(w, r, account.PermEditGeography)
	if !ok {
		return
	}
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" {
		writeErr(w, r, apierr.New(apierr.InvalidArgument, "Idempotency-Key is required."))
		return
	}
	actor.RequestID = idempotencyRequestID(actor.ID, key)
	var b geographyCreateBody
	if err := decodeJSON(r, &b); err != nil {
		writeErr(w, r, err)
		return
	}
	g, err := geometryFromWire(b.Geometry)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	var c *domaingeo.Coordinate
	if b.Centroid != nil {
		c = &domaingeo.Coordinate{Latitude: b.Centroid.Latitude, Longitude: b.Centroid.Longitude}
	}
	kind := strings.TrimSuffix(chi.URLParam(r, "kind"), "s")
	_, err = h.geo.CreateAdminGeography(r.Context(), actor, kind, appgeo.CreateInput{ID: b.ID, Name: b.Name, CountryCode: b.CountryCode, Capital: b.Capital, OfficialCode: b.Code, RegionID: b.RegionID, RegionName: b.RegionName, DistrictID: b.DistrictID, DistrictName: b.DistrictName, DistrictType: b.DistrictType, PlaceType: b.PlaceType, RoadClass: b.RoadClass, Ref: b.Ref, POIClass: b.POIClass, Category: b.Category, Attribution: b.Attribution, Centroid: c, Geometry: g, Provenance: domaingeo.Provenance{SourceID: b.Provenance.SourceID, ExternalID: b.Provenance.ExternalID, SourceURL: b.Provenance.SourceURL, RetrievedAt: b.Provenance.RetrievedAt, SourcePayloadHash: b.Provenance.SourcePayloadHash, Notes: b.Provenance.Notes}})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": map[string]any{"id": b.ID, "kind": kind}})
}
func (h *Handler) adminDeprecateGeography(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.requireSteward(w, r, account.PermEditGeography)
	if !ok {
		return
	}
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" {
		writeErr(w, r, apierr.New(apierr.InvalidArgument, "Idempotency-Key is required."))
		return
	}
	actor.RequestID = idempotencyRequestID(actor.ID, key)
	var b struct {
		MergedInto string `json:"mergedInto"`
		Reason     string `json:"reason"`
	}
	if err := decodeJSON(r, &b); err != nil {
		writeErr(w, r, err)
		return
	}
	kind := strings.TrimSuffix(chi.URLParam(r, "kind"), "s")
	if err := h.geo.DeprecateAdminGeography(r.Context(), actor, kind, chi.URLParam(r, "id"), b.MergedInto, b.Reason); err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "Record deprecated."})
}
func (h *Handler) adminRedirects(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.requireSteward(w, r, account.PermViewGeography)
	if !ok {
		return
	}
	p, err := h.geo.ListAdminRedirects(r.Context(), actor, listParams(r))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	data := make([]map[string]any, 0, len(p.Data))
	for _, x := range p.Data {
		data = append(data, map[string]any{"oldId": x.OldID, "kind": x.Kind, "newId": x.NewID, "reason": x.Reason, "mergedAt": x.MergedAt})
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data, "meta": map[string]any{"nextCursor": p.NextCursor}})
}
func (h *Handler) adminAliases(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.requireSteward(w, r, account.PermViewGeography)
	if !ok {
		return
	}
	v, err := h.geo.ListAdminAliases(r.Context(), actor, chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	data := make([]map[string]any, 0, len(v))
	for _, x := range v {
		data = append(data, map[string]any{"id": x.ID, "placeId": x.PlaceID, "value": x.Value, "normalizedValue": x.NormalizedValue, "type": x.AliasType, "language": x.Language, "isPreferred": x.IsPreferred, "status": x.Status})
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}
func (h *Handler) adminCreateAlias(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.requireSteward(w, r, account.PermEditGeography)
	if !ok {
		return
	}
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" {
		writeErr(w, r, apierr.New(apierr.InvalidArgument, "Idempotency-Key is required."))
		return
	}
	actor.RequestID = idempotencyRequestID(actor.ID, key)
	var b struct {
		ID          string `json:"id"`
		Value       string `json:"value"`
		Type        string `json:"type"`
		Language    string `json:"language"`
		IsPreferred bool   `json:"isPreferred"`
	}
	if err := decodeJSON(r, &b); err != nil {
		writeErr(w, r, err)
		return
	}
	if err := h.geo.CreateAdminAlias(r.Context(), actor, domaingeo.Alias{ID: b.ID, PlaceID: chi.URLParam(r, "id"), Value: b.Value, AliasType: b.Type, Language: b.Language, IsPreferred: b.IsPreferred}); err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": map[string]any{"id": b.ID}})
}
func (h *Handler) adminDeprecateAlias(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.requireSteward(w, r, account.PermEditGeography)
	if !ok {
		return
	}
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" {
		writeErr(w, r, apierr.New(apierr.InvalidArgument, "Idempotency-Key is required."))
		return
	}
	actor.RequestID = idempotencyRequestID(actor.ID, key)
	if err := h.geo.DeprecateAdminAlias(r.Context(), actor, chi.URLParam(r, "id"), chi.URLParam(r, "aliasId")); err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "Alias deprecated."})
}

func decodeBoundary(r *http.Request) (*domaingeo.Geometry, error) {
	var wire struct {
		Type        domaingeo.GeometryType `json:"type"`
		Coordinates json.RawMessage        `json:"coordinates"`
	}
	if err := decodeJSON(r, &wire); err != nil {
		return nil, err
	}
	g := &domaingeo.Geometry{Type: wire.Type}
	var target any
	switch wire.Type {
	case domaingeo.GeomPolygon:
		target = &[][][]float64{}
	case domaingeo.GeomMultiPolygon:
		target = &[][][][]float64{}
	default:
		return nil, apierr.New(apierr.InvalidArgument, "A Polygon or MultiPolygon geometry is required.")
	}
	if err := json.Unmarshal(wire.Coordinates, target); err != nil {
		return nil, apierr.New(apierr.InvalidArgument, "Coordinates must be valid GeoJSON numeric arrays.")
	}
	switch v := target.(type) {
	case *[][][]float64:
		g.Coordinates = *v
	case *[][][][]float64:
		g.Coordinates = *v
	}
	return g, nil
}

func boundaryJSON(g *domaingeo.Geometry) any {
	if g == nil {
		return nil
	}
	return map[string]any{"type": g.Type, "coordinates": g.Coordinates}
}

func (h *Handler) adminGetBoundary(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.requireSteward(w, r, account.PermViewGeography)
	if !ok {
		return
	}
	out, err := h.geo.GetAdminBoundary(r.Context(), actor, chi.URLParam(r, "kind"), chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	w.Header().Set("ETag", out.ETag)
	writeJSON(w, http.StatusOK, map[string]any{"data": boundaryJSON(out.Geometry)})
}

func (h *Handler) adminUpdateBoundary(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.requireSteward(w, r, account.PermEditGeography)
	if !ok {
		return
	}
	g, err := decodeBoundary(r)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	out, err := h.geo.UpdateAdminBoundary(r.Context(), actor, chi.URLParam(r, "kind"), chi.URLParam(r, "id"), r.Header.Get("If-Match"), g)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	w.Header().Set("ETag", out.ETag)
	writeJSON(w, http.StatusOK, map[string]any{"data": boundaryJSON(out.Geometry)})
}

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
	actor, ok := h.requireGeographyMutation(w, r)
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
	actor, ok := h.requireGeographyMutation(w, r)
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
	actor, ok := h.requireGeographyMutation(w, r)
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
	// This endpoint is also the admin-console admission check. Requiring an
	// admin permission here prevents an ordinary developer account from being
	// treated as an administrator merely because it has a valid session.
	// Mutating endpoints still check their narrower permissions independently.
	actor, ok := h.requireSteward(w, r, account.PermViewGeography)
	if !ok {
		return
	}
	perms := account.Permissions(actor.Role)
	out := make([]string, 0, len(perms))
	for _, p := range perms {
		out = append(out, string(p))
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"role": string(actor.Role), "permissions": out,
	}})
}
