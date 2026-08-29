package rest

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/identity"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/auth"

	app "github.com/ghanageo/ghanageo/services/api/internal/app/geography"
	appsearch "github.com/ghanageo/ghanageo/services/api/internal/app/search"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/apierr"
	"github.com/ghanageo/ghanageo/services/api/internal/ports"
)

type Handler struct {
	geo            *app.Service
	search         *appsearch.Service
	log            *slog.Logger
	allowedOrigins map[string]bool
	auth           *auth.Authenticator
}

// WithAuth attaches the authenticator. When absent — in unit tests — the
// router serves without identity resolution or fair-use limiting.
func (h *Handler) WithAuth(a *auth.Authenticator) *Handler {
	h.auth = a
	return h
}

// costOf maps a request to its fair-use cost class (Appendix D). Spatial and
// geometry work costs more because it costs more to serve, not because it is
// worth more money — GhanaGeo is free.
func costOf(r *http.Request) identity.CostClass {
	p := r.URL.Path
	switch {
	case strings.HasPrefix(p, "/v1/boundaries"):
		return identity.CostGeometry
	case strings.HasPrefix(p, "/v1/reverse"), strings.HasPrefix(p, "/v1/nearby"):
		return identity.CostSpatial
	case strings.HasPrefix(p, "/v1/search"),
		strings.HasPrefix(p, "/v1/autocomplete"),
		strings.HasPrefix(p, "/v1/geocode"):
		return identity.CostNormal
	default:
		return identity.CostCheap
	}
}

func init() {
	// Give the auth middleware the same error envelope every handler uses, so
	// a 401 or 429 is shaped exactly like a 404 (Spec §19).
	auth.SetErrorWriter(writeErr)
}

func New(geo *app.Service, search *appsearch.Service, log *slog.Logger, allowedOrigins []string) *Handler {
	set := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		set[strings.TrimSpace(o)] = true
	}
	return &Handler{geo: geo, search: search, log: log, allowedOrigins: set}
}

// Routes returns the /v1 router. Paths match contracts/openapi/v1.yaml exactly.
func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(h.accessLog)
	r.Use(requestIDHeader)
	r.Use(h.cors)

	r.Get("/health", h.health)

	r.Route("/v1", func(r chi.Router) {
		if h.auth != nil {
			// One middleware resolves the caller and charges the bucket for
			// every endpoint below, so no handler can forget to.
			r.Use(h.auth.Middleware(costOf))
		}
		r.Get("/regions", h.listRegions)
		r.Get("/regions/{id}", h.getRegion)
		r.Get("/regions/{id}/districts", h.listRegionDistricts)

		r.Get("/districts", h.listDistricts)
		r.Get("/districts/{id}", h.getDistrict)
		r.Get("/districts/{id}/places", h.listDistrictPlaces)

		r.Get("/places", h.listPlaces)
		r.Get("/places/{id}", h.getPlace)

		r.Get("/nearby", h.nearby)

		// Search surface (EP-12). Registered only when a SearchPort is wired,
		// so a deployment without one returns 404 rather than a 500.
		if h.search != nil {
			r.Get("/search", h.searchHandler)
			r.Get("/autocomplete", h.autocompleteHandler)
			r.Get("/geocode", h.geocodeHandler)
			r.Get("/reverse", h.reverseHandler)
		}
	})

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		writeErr(w, r, apierr.New(apierr.NotFound, "No such endpoint."))
	})
	return r
}

// ---- middleware ----

func requestIDHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-Id", middleware.GetReqID(r.Context()))
		next.ServeHTTP(w, r)
	})
}

// cors applies an origin ALLOW-LIST, never a wildcard (Spec 12.4). An
// unlisted origin simply gets no CORS headers, so the browser blocks it.
func (h *Handler) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && h.allowedOrigins[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Max-Age", "600")
			// Responses vary by origin, so caches must not share them.
			w.Header().Add("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// accessLog emits one structured line per request. It never logs a secret or an
// authorization header (Spec 20, 21).
func (h *Handler) accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		h.log.Info("request",
			"request_id", middleware.GetReqID(r.Context()),
			"protocol", "rest",
			"operation", r.Method+" "+r.URL.Path,
			"status", ww.Status(),
			"latency_ms", time.Since(start).Milliseconds(),
			// The rate key is a public prefix or an IP — never a secret.
			"caller", auth.FromContext(r.Context()).RateKey,
		)
	})
}

// ---- responses ----

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

type errorBody struct {
	Error struct {
		Code      string         `json:"code"`
		Message   string         `json:"message"`
		RequestID string         `json:"requestId"`
		Details   map[string]any `json:"details,omitempty"`
		Docs      string         `json:"docs"`
	} `json:"error"`
}

// writeErr renders the Spec 19 error envelope for every failure path.
//
// The public body carries only the stable code and a safe message. The CAUSE
// is logged against the request id, because Spec §20 requires an INTERNAL
// error to be traceable — without this a 500 is unactionable.
func writeErr(w http.ResponseWriter, r *http.Request, err error) {
	e := apierr.From(err)
	if e.Code == apierr.Internal {
		slog.Error("request failed",
			"request_id", middleware.GetReqID(r.Context()),
			"operation", r.Method+" "+r.URL.Path,
			"code", string(e.Code),
			// The unwrapped cause, never returned to the caller.
			"cause", err.Error(),
		)
	}
	var body errorBody
	body.Error.Code = string(e.Code)
	body.Error.Message = e.Message
	body.Error.RequestID = middleware.GetReqID(r.Context())
	body.Error.Details = e.Details
	body.Error.Docs = e.Code.DocsURL()
	writeJSON(w, e.Code.HTTPStatus(), body)
}

// ---- request parsing ----

func listParams(r *http.Request) ports.ListParams {
	p := ports.ListParams{Cursor: r.URL.Query().Get("cursor")}
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			p.Limit = n
		}
	}
	return p.Normalize()
}

func floatParam(r *http.Request, key string) (float64, bool) {
	v := r.URL.Query().Get(key)
	if v == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

func intParam(r *http.Request, key string, def int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

// ---- handlers ----

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":         "ok",
		"datasetVersion": h.geo.DatasetVersion(),
	})
}

func (h *Handler) listRegions(w http.ResponseWriter, r *http.Request) {
	page, err := h.geo.ListRegions(r.Context(), listParams(r))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, pageOut(page, regionOut))
}

func (h *Handler) getRegion(w http.ResponseWriter, r *http.Request) {
	out, err := h.geo.GetRegion(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, regionOut(*out))
}

func (h *Handler) listRegionDistricts(w http.ResponseWriter, r *http.Request) {
	page, err := h.geo.ListDistrictsInRegion(r.Context(), chi.URLParam(r, "id"), listParams(r))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, pageOut(page, districtOut))
}

func (h *Handler) listDistricts(w http.ResponseWriter, r *http.Request) {
	page, err := h.geo.ListDistricts(r.Context(), ports.DistrictFilter{
		ListParams: listParams(r),
		RegionID:   r.URL.Query().Get("regionId"),
		Query:      r.URL.Query().Get("q"),
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, pageOut(page, districtOut))
}

func (h *Handler) getDistrict(w http.ResponseWriter, r *http.Request) {
	out, err := h.geo.GetDistrict(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, districtOut(*out))
}

func (h *Handler) listDistrictPlaces(w http.ResponseWriter, r *http.Request) {
	page, err := h.geo.ListPlacesInDistrict(r.Context(), chi.URLParam(r, "id"), ports.PlaceFilter{
		ListParams: listParams(r),
		Type:       r.URL.Query().Get("type"),
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, pageOut(page, placeOut))
}

func (h *Handler) listPlaces(w http.ResponseWriter, r *http.Request) {
	page, err := h.geo.ListPlaces(r.Context(), ports.PlaceFilter{
		ListParams: listParams(r),
		DistrictID: r.URL.Query().Get("districtId"),
		RegionID:   r.URL.Query().Get("regionId"),
		Type:       r.URL.Query().Get("type"),
		Query:      r.URL.Query().Get("q"),
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, pageOut(page, placeOut))
}

func (h *Handler) getPlace(w http.ResponseWriter, r *http.Request) {
	out, err := h.geo.GetPlace(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, placeOut(*out))
}

func (h *Handler) nearby(w http.ResponseWriter, r *http.Request) {
	lat, okLat := floatParam(r, "lat")
	lng, okLng := floatParam(r, "lng")
	if !okLat || !okLng {
		writeErr(w, r, apierr.New(apierr.InvalidArgument, "Both lat and lng are required."))
		return
	}
	places, err := h.geo.Nearby(r.Context(), lat, lng, intParam(r, "radius", 5000), intParam(r, "limit", 20))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	out := make([]placeDTO, 0, len(places))
	for _, p := range places {
		out = append(out, placeOut(p))
	}
	writeJSON(w, http.StatusOK, pageDTO[placeDTO]{Data: out, DatasetVersion: h.geo.DatasetVersion()})
}
