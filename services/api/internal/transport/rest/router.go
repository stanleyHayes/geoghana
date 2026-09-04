package rest

import (
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	mongoadapter "github.com/ghanageo/ghanageo/services/api/internal/adapters/mongo"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/identity"
	usageDomain "github.com/ghanageo/ghanageo/services/api/internal/domain/usage"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/auth"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/observability"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	appaccount "github.com/ghanageo/ghanageo/services/api/internal/app/account"
	appadminidentity "github.com/ghanageo/ghanageo/services/api/internal/app/adminidentity"
	appadminops "github.com/ghanageo/ghanageo/services/api/internal/app/adminops"
	appchangerequest "github.com/ghanageo/ghanageo/services/api/internal/app/changerequest"
	appdataset "github.com/ghanageo/ghanageo/services/api/internal/app/dataset"
	appdeveloper "github.com/ghanageo/ghanageo/services/api/internal/app/developer"
	appfairuse "github.com/ghanageo/ghanageo/services/api/internal/app/fairuse"
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
	store          *mongoadapter.Store
	graphql        http.Handler
	datasets       *appdataset.Service
	accounts       *appaccount.Service
	developer      *appdeveloper.Service
	usage          usageDomain.Repository
	telemetry      *observability.Telemetry
	metricsToken   string
	adminOps       *appadminops.Service
	adminIdentity  *appadminidentity.Service
	changeRequests *appchangerequest.Service
	fairUse        *appfairuse.Service
}

func (h *Handler) WithAdminOps(a *appadminops.Service) *Handler { h.adminOps = a; return h }

func (h *Handler) WithAdminIdentity(a *appadminidentity.Service) *Handler {
	h.adminIdentity = a
	return h
}

func (h *Handler) WithChangeRequests(a *appchangerequest.Service) *Handler {
	h.changeRequests = a
	return h
}

func (h *Handler) WithFairUse(a *appfairuse.Service) *Handler { h.fairUse = a; return h }

func (h *Handler) WithDeveloper(a *appdeveloper.Service) *Handler { h.developer = a; return h }

func (h *Handler) WithUsage(repository usageDomain.Repository) *Handler {
	h.usage = repository
	return h
}

func (h *Handler) WithTelemetry(telemetry *observability.Telemetry) *Handler {
	h.telemetry = telemetry
	return h
}

// WithMetricsToken supplies the bearer token that guards /metrics. Without it the
// endpoint is not registered.
func (h *Handler) WithMetricsToken(token string) *Handler {
	h.metricsToken = token
	return h
}

// requireMetricsToken admits a scrape that presents the configured bearer token.
//
// The comparison is constant time so a wrong token cannot be narrowed by timing,
// and a refusal says nothing about what is behind it.
func requireMetricsToken(token string, next http.Handler) http.Handler {
	expected := []byte(token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		presented := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
		if subtle.ConstantTimeCompare([]byte(presented), expected) != 1 {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// WithAccounts attaches account authentication. Absent in unit tests, where
// the endpoints report that they are not configured rather than panicking.
func (h *Handler) WithAccounts(a *appaccount.Service) *Handler {
	h.accounts = a
	return h
}

// WithDatasets attaches the dataset catalogue. Absent in unit tests, where the
// endpoints report that they are not configured rather than panicking.
func (h *Handler) WithDatasets(d *appdataset.Service) *Handler {
	h.datasets = d
	return h
}

// WithStore attaches the datastore for endpoints that read across collections.
func (h *Handler) WithStore(s *mongoadapter.Store) *Handler {
	h.store = s
	return h
}

// WithGraphQL mounts the GraphQL endpoint on the same mux, so it inherits the
// request id, CORS, access logging, authentication and fair-use middleware
// rather than reimplementing them.
func (h *Handler) WithGraphQL(g http.Handler) *Handler {
	h.graphql = g
	return h
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
	r.Use(h.csrf)

	r.Get("/health", h.health)
	// Telemetry is exposed only when a token is configured to guard it. Metrics
	// carry no per-caller labels, but they do publish traffic volume, latency
	// distribution, error counts and queue depth, and they are the only
	// admin-adjacent endpoint that would otherwise answer to anyone.
	if h.telemetry != nil && h.metricsToken != "" {
		r.Handle("/metrics", requireMetricsToken(h.metricsToken, h.telemetry.Handler()))
	}

	if h.graphql != nil {
		// Outside /v1: the GraphQL schema carries its own version, and Spec
		// §8.1 places it at /graphql.
		r.Handle("/graphql", h.graphqlRoute())
	}

	r.Route("/v1", func(r chi.Router) {
		if h.auth != nil {
			// One middleware resolves the caller and charges the bucket for
			// every endpoint below, so no handler can forget to.
			r.Use(h.auth.Middleware(costOf))
		}
		if h.usage != nil {
			r.Use(h.captureUsage("rest"))
		}
		// Account authentication (GEO-9.2). Deliberately NOT behind
		// RequireScope: these are humans signing in with a session cookie, not
		// API keys presenting scopes. Cookie-authenticated mutations are guarded
		// by the router-level origin check as well as SameSite=Lax.
		r.Route("/auth", func(r chi.Router) {
			r.Use(adminPrivateResponse)
			r.With(authExpensiveGuard).Post("/register", h.authRegister)
			r.Post("/verify", h.authVerifyEmail)
			r.Post("/password-reset/request", h.authRequestPasswordReset)
			r.With(authExpensiveGuard).Post("/password-reset/confirm", h.authConfirmPasswordReset)
			r.With(authExpensiveGuard).Post("/login", h.authLogin)
			r.Post("/mfa/totp", h.authTOTP)
			r.Post("/mfa/recover", h.authRecover)
			r.Post("/mfa/enrol", h.authEnrolTOTP)
			r.Post("/logout", h.authLogout)
			r.Post("/logout-all", h.authLogoutAll)
			r.Get("/session", h.authSession)
			r.Get("/sessions", h.authSessions)

			// WebAuthn. Login begin/finish are unauthenticated by design —
			// they ARE the authentication.
			r.Post("/passkeys/register/begin", h.passkeyRegisterBegin)
			r.Post("/passkeys/register/finish", h.passkeyRegisterFinish)
			r.Post("/passkeys/login/begin", h.passkeyLoginBegin)
			r.Post("/passkeys/login/finish", h.passkeyLoginFinish)
			r.Get("/passkeys", h.passkeyList)
			r.Delete("/passkeys/{id}", h.passkeyDelete)
		})
		if h.developer != nil {
			r.Route("/developer", func(r chi.Router) {
				r.Use(adminPrivateResponse)
				r.Get("/organizations", h.developerOrganizations)
				r.Post("/organizations", h.developerCreateOrganization)
				r.Get("/organizations/{orgId}/invitations", h.developerInvitations)
				r.Post("/organizations/{orgId}/invitations", h.developerInviteMember)
				r.Post("/organizations/{orgId}/invitations/{inviteId}/revoke", h.developerRevokeInvitation)
				r.Post("/organizations/{orgId}/transfer-ownership", h.developerTransferOwnership)
				r.Post("/invitations/accept", h.developerAcceptInvitation)
				r.Get("/organizations/{orgId}/applications", h.developerApplications)
				r.Post("/organizations/{orgId}/applications", h.developerCreateApplication)
				r.Get("/organizations/{orgId}/applications/{appId}/keys", h.developerKeys)
				r.Post("/organizations/{orgId}/applications/{appId}/keys", h.developerCreateKey)
				r.Post("/organizations/{orgId}/applications/{appId}/keys/{keyId}/rotate", h.developerRotateKey)
				r.Post("/organizations/{orgId}/applications/{appId}/keys/{keyId}/revoke", h.developerRevokeKey)
				r.Get("/organizations/{orgId}/applications/{appId}/usage", h.developerUsageSummary)
				r.Get("/organizations/{orgId}/applications/{appId}/requests", h.developerRequestLogs)
			})
		}

		r.With(auth.RequireScope(identity.ScopeLocationsRead)).Get("/regions", h.listRegions)
		r.With(auth.RequireScope(identity.ScopeLocationsRead)).Get("/regions/{id}", h.getRegion)
		r.With(auth.RequireScope(identity.ScopeLocationsRead)).Get("/regions/{id}/districts", h.listRegionDistricts)

		r.With(auth.RequireScope(identity.ScopeLocationsRead)).Get("/districts", h.listDistricts)
		r.With(auth.RequireScope(identity.ScopeLocationsRead)).Get("/districts/{id}", h.getDistrict)
		r.With(auth.RequireScope(identity.ScopeLocationsRead)).Get("/districts/{id}/places", h.listDistrictPlaces)

		r.With(auth.RequireScope(identity.ScopeLocationsRead)).Get("/places", h.listPlaces)
		r.With(auth.RequireScope(identity.ScopeLocationsRead)).Get("/places/{id}", h.getPlace)

		r.With(auth.RequireScope(identity.ScopeGeocodeRead)).Get("/nearby", h.nearby)

		// OpenStreetMap-derived (GEO-4.7). Both responses carry the ODbL
		// notice: it is share-alike, so attribution travels with the data.
		r.With(auth.RequireScope(identity.ScopeLocationsRead)).Get("/roads", h.listRoads)
		r.With(auth.RequireScope(identity.ScopeLocationsRead)).Get("/pois", h.listPOIs)
		r.With(auth.RequireScope(identity.ScopeBoundariesRead)).Get("/boundaries/{id}", h.boundary)

		// Bulk downloads (GEO-8.3). Cheaper for a consumer than paginating
		// the whole dataset, and cheaper for us to serve.
		// Steward mutations (GEO-17.3). Session-authenticated, permission
		// checked, and audited — including refusals. There is deliberately no
		// DELETE: a record is deprecated into a redirect, never removed.
		r.Route("/admin", func(r chi.Router) {
			r.Use(adminPrivateResponse)
			r.Get("/permissions", h.adminPermissions)
			r.Get("/fair-use/policy", h.adminFairUsePolicy)
			r.Post("/fair-use/policies", h.adminAppendFairUsePolicy)
			r.Post("/fair-use/overrides", h.adminAppendFairUseOverride)
			r.Get("/dashboard", h.adminDashboard)
			r.Get("/system-health", h.adminSystemHealth)
			r.Get("/audit-log", h.adminAuditLog)
			r.Get("/developers/organizations", h.adminDeveloperOrganizations)
			r.Get("/developers/accounts", h.adminDeveloperAccounts)
			r.Get("/developers/applications", h.adminDeveloperApplications)
			r.Get("/developers/keys", h.adminDeveloperKeys)
			r.Get("/developers/usage", h.adminDeveloperUsage)
			r.Get("/developers/requests", h.adminDeveloperRequests)
			r.Post("/developers/keys/{keyId}/suspend", h.adminSuspendDeveloperKey)
			r.Post("/developers/keys/{keyId}/revoke", h.adminRevokeDeveloperKey)
			r.Get("/source-runs", h.adminSourceRuns)
			r.Get("/source-runs/{id}", h.adminSourceRun)
			r.Get("/source-runs/{id}/records", h.adminSourceRecords)
			r.Get("/source-runs/{id}/records/{recordId}", h.adminSourceRecord)
			r.Get("/source-runs/{id}/conflicts", h.adminSourceConflicts)
			r.Get("/source-runs/{id}/duplicates", h.adminSourceDuplicates)
			r.Get("/dataset-releases", h.adminDatasetReleases)
			r.Get("/dataset-releases/{version}/readiness", h.adminDatasetReleaseReadiness)
			r.Post("/dataset-releases/{version}/advance", h.adminAdvanceDatasetRelease)
			r.Put("/dataset-releases/{version}/changelog", h.adminUpdateDatasetChangelog)
			r.Post("/dataset-releases/{version}/publish", h.adminPublishDatasetRelease)
			r.Post("/dataset-releases/{version}/rollback", h.adminRollbackDatasetRelease)
			r.Patch("/regions/{id}", h.adminUpdateRegion)
			r.Patch("/districts/{id}", h.adminUpdateDistrict)
			r.Patch("/places/{id}", h.adminUpdatePlace)
			r.Post("/geography/{kind:regions|districts|places|roads|pois}", h.adminCreateGeography)
			r.Post("/geography/{kind:regions|districts|places|roads|pois}/{id}/deprecate", h.adminDeprecateGeography)
			r.Get("/redirects", h.adminRedirects)
			r.Get("/places/{id}/aliases", h.adminAliases)
			r.Post("/places/{id}/aliases", h.adminCreateAlias)
			r.Post("/places/{id}/aliases/{aliasId}/deprecate", h.adminDeprecateAlias)
			r.Get("/boundaries/{kind}/{id}", h.adminGetBoundary)
			r.Put("/boundaries/{kind}/{id}", h.adminUpdateBoundary)
			r.Get("/change-requests", h.adminListChangeRequests)
			r.Post("/change-requests", h.adminCreateChangeRequest)
			r.Get("/change-requests/{id}", h.adminGetChangeRequest)
			r.Post("/change-requests/{id}/transitions", h.adminTransitionChangeRequest)
			r.Post("/change-requests/{id}/revisions", h.adminReviseChangeRequest)
			r.Post("/change-requests/{id}/comments", h.adminCommentChangeRequest)
		})

		r.With(auth.RequireScope(identity.ScopeDatasetsRead)).Get("/datasets", h.listDatasets)
		r.With(auth.RequireScope(identity.ScopeDatasetsRead)).Get("/datasets/{version}/downloads", h.datasetDownloads)
		r.With(auth.RequireScope(identity.ScopeDatasetsRead)).Get("/datasets/{version}/downloads/{entity}.{format}", h.datasetArtifact)

		// Search surface (EP-12). Registered only when a SearchPort is wired,
		// so a deployment without one returns 404 rather than a 500.
		if h.search != nil {
			r.With(auth.RequireScope(identity.ScopeSearchRead)).Get("/search", h.searchHandler)
			r.With(auth.RequireScope(identity.ScopeSearchRead)).Get("/autocomplete", h.autocompleteHandler)
			r.With(auth.RequireScope(identity.ScopeGeocodeRead)).Get("/geocode", h.geocodeHandler)
			r.With(auth.RequireScope(identity.ScopeGeocodeRead)).Get("/reverse", h.reverseHandler)
		}
	})

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		writeErr(w, r, apierr.New(apierr.NotFound, "No such endpoint."))
	})
	if h.telemetry != nil {
		return otelhttp.NewHandler(r, "http.request")
	}
	return r
}

// graphqlRoute applies the same identity and fair-use accounting REST uses.
// A GraphQL request is charged the NORMAL cost class as a floor; per-field
// cost is enforced separately by the complexity budget, because a single
// GraphQL request can be worth a hundred REST calls.
func (h *Handler) graphqlRoute() http.Handler {
	if h.auth == nil {
		return h.graphql
	}
	handler := auth.RequireScope(identity.ScopeGraphQLAccess)(h.graphql)
	if h.usage != nil {
		handler = h.captureUsage("graphql")(handler)
	}
	return h.auth.Middleware(func(*http.Request) identity.CostClass {
		return identity.CostNormal
	})(handler)
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
			// Keep this list aligned with the registered browser-facing routes.
			// Admin geography edits use PATCH and passkey removal uses DELETE;
			// omitting either here makes the browser reject the request at
			// preflight even though the authenticated endpoint is implemented.
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
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

// csrf prevents a browser from spending a session cookie on a cross-origin
// mutation. CORS alone is not sufficient: it hides a response from an
// untrusted origin but does not stop the request from reaching the handler.
//
// Requests without a session cookie remain usable by API clients. Older
// non-browser clients that do send a cookie may omit Origin; modern browsers
// also send Sec-Fetch-Site, so an explicitly cross-site request is denied even
// when Origin has been stripped by an intermediary.
func (h *Handler) csrf(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isSafeMethod(r.Method) || sessionTokenFrom(r) == "" {
			next.ServeHTTP(w, r)
			return
		}

		origin := strings.TrimSpace(r.Header.Get("Origin"))
		crossSite := strings.EqualFold(strings.TrimSpace(r.Header.Get("Sec-Fetch-Site")), "cross-site")
		if crossSite || (origin != "" && !h.allowedOrigins[origin]) {
			writeErr(w, r, apierr.New(apierr.PermissionDenied, "This browser origin cannot modify the account."))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isSafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return true
	default:
		return false
	}
}

// accessLog emits one structured line per request. It never logs a secret or an
// authorization header (Spec 20, 21).
func (h *Handler) accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		statusCode := ww.Status()
		if statusCode == 0 {
			statusCode = http.StatusOK
		}
		operation := r.Method + " " + chi.RouteContext(r.Context()).RoutePattern()
		if strings.TrimSpace(operation) == r.Method {
			operation = r.Method + " unmatched"
		}
		elapsed := time.Since(start)
		caller := auth.FromContext(r.Context())
		appID, keyPrefix := "", ""
		if caller.Key != nil {
			appID, keyPrefix = caller.Key.ApplicationID, caller.Key.Prefix
		}
		if h.telemetry != nil {
			h.telemetry.ObserveRequest("http", operation, strconv.Itoa(statusCode), elapsed)
		}
		h.log.Info("request",
			"request_id", middleware.GetReqID(r.Context()),
			"protocol", "rest",
			"trace_id", observability.TraceID(r.Context()),
			"app_id", appID,
			"key_prefix", keyPrefix,
			"operation", operation,
			"status", statusCode,
			"latency_ms", elapsed.Milliseconds(),
			// The rate key is a public prefix or an IP — never a secret.
			"caller", caller.RateKey,
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
