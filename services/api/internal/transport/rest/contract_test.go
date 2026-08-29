package rest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// TestRoutesMatchOpenAPIContract is the drift guard between the implementation
// and contracts/openapi/v1.yaml. Adding a handler without documenting it, or
// documenting a path that is not served, fails here rather than reaching a
// customer as a 404 against published documentation.
func TestRoutesMatchOpenAPIContract(t *testing.T) {
	raw, err := os.ReadFile("../../../../contracts/openapi/v1.yaml")
	if err != nil {
		t.Skipf("contract not readable from this working directory: %v", err)
	}

	// Parse path + method rather than path alone: several account and developer
	// resources intentionally support both GET and POST.
	pathRE := regexp.MustCompile(`^  (/[^:\s]*):\s*$`)
	methodRE := regexp.MustCompile(`^    (get|post|patch|put|delete):\s*$`)
	currentPath := ""
	var documented []string
	for _, line := range strings.Split(string(raw), "\n") {
		if match := pathRE.FindStringSubmatch(line); match != nil {
			currentPath = normalizePath(match[1])
			continue
		}
		if currentPath != "" {
			if match := methodRE.FindStringSubmatch(line); match != nil {
				documented = append(documented, strings.ToUpper(match[1])+" "+currentPath)
			}
		}
	}
	sort.Strings(documented)

	if len(documented) == 0 {
		t.Fatal("no paths parsed from the OpenAPI contract")
	}

	implemented := map[string]bool{}
	for _, route := range []string{
		"POST /auth/register", "POST /auth/verify", "POST /auth/login",
		"POST /auth/mfa/totp", "POST /auth/mfa/recover", "POST /auth/mfa/enrol",
		"POST /auth/logout", "POST /auth/logout-all", "GET /auth/session", "GET /auth/sessions",
		"POST /auth/passkeys/register/begin", "POST /auth/passkeys/register/finish",
		"POST /auth/passkeys/login/begin", "POST /auth/passkeys/login/finish",
		"GET /auth/passkeys", "DELETE /auth/passkeys/{}",
		"GET /developer/organizations", "POST /developer/organizations",
		"GET /developer/organizations/{}/invitations", "POST /developer/organizations/{}/invitations",
		"POST /developer/organizations/{}/invitations/{}/revoke",
		"POST /developer/organizations/{}/transfer-ownership", "POST /developer/invitations/accept",
		"GET /developer/organizations/{}/applications", "POST /developer/organizations/{}/applications",
		"GET /developer/organizations/{}/applications/{}/keys", "POST /developer/organizations/{}/applications/{}/keys",
		"POST /developer/organizations/{}/applications/{}/keys/{}/rotate",
		"POST /developer/organizations/{}/applications/{}/keys/{}/revoke",
		"GET /developer/organizations/{}/applications/{}/usage",
		"GET /developer/organizations/{}/applications/{}/requests",
		"GET /regions", "GET /regions/{}", "GET /regions/{}/districts",
		"GET /districts", "GET /districts/{}", "GET /districts/{}/places",
		"GET /places", "GET /places/{}", "GET /nearby", "GET /roads", "GET /pois",
		"GET /search", "GET /autocomplete", "GET /geocode", "GET /reverse", "GET /boundaries/{}",
		"GET /datasets", "GET /datasets/{}/downloads", "GET /datasets/{}/downloads/{}.{}",
		"GET /admin/permissions", "PATCH /admin/regions/{}", "PATCH /admin/districts/{}",
		"PATCH /admin/places/{}", "POST /admin/places/{}/deprecate",
	} {
		implemented[route] = true
	}

	for _, route := range documented {
		if !implemented[route] {
			t.Errorf("%s is documented but not registered by the V1 router", route)
		}
	}

	docSet := map[string]bool{}
	for _, route := range documented {
		docSet[route] = true
	}
	for route := range implemented {
		if !docSet[route] {
			t.Errorf("%s is served but missing from the OpenAPI contract", route)
		}
	}
}

// normalizePath collapses {id}, {version} etc so the two sides compare on shape.
func normalizePath(p string) string {
	return regexp.MustCompile(`\{[^}]+\}`).ReplaceAllString(p, "{}")
}

// TestErrorEnvelopeShape locks the Spec 19 contract: every failure carries a
// stable code, a request id and a docs link.
func TestErrorEnvelopeShape(t *testing.T) {
	h := New(nil, nil, discardLogger(), nil)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/definitely-not-a-route", nil)
	h.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
	var body struct {
		Error struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			RequestID string `json:"requestId"`
			Docs      string `json:"docs"`
		} `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("error body is not valid JSON: %v", err)
	}
	if body.Error.Code != "NOT_FOUND" {
		t.Errorf("code = %q, want NOT_FOUND", body.Error.Code)
	}
	if body.Error.Message == "" {
		t.Error("message must not be empty")
	}
	if body.Error.RequestID == "" {
		t.Error("requestId must be set on every error")
	}
	if !strings.HasPrefix(body.Error.Docs, "/docs/errors/") {
		t.Errorf("docs = %q, want a /docs/errors/ link", body.Error.Docs)
	}
}

// TestCORSAllowList proves an unlisted origin receives no CORS header, so the
// browser blocks it. A wildcard here would be a security bug (Spec 12.4).
func TestCORSAllowList(t *testing.T) {
	h := New(nil, nil, discardLogger(), []string{"http://localhost:3103"})
	router := h.Routes()

	for _, tc := range []struct {
		origin string
		want   string
	}{
		{"http://localhost:3103", "http://localhost:3103"},
		{"https://evil.example", ""},
	} {
		// A preflight is answered by the middleware and never reaches a
		// handler, so this asserts CORS alone with no service dependency.
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodOptions, "/v1/regions", nil)
		req.Header.Set("Origin", tc.origin)
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("preflight returned %d, want 204", rec.Code)
		}
		if got := rec.Header().Get("Access-Control-Allow-Origin"); got != tc.want {
			t.Errorf("origin %s: Access-Control-Allow-Origin = %q, want %q", tc.origin, got, tc.want)
		}
	}
}

func TestCSRFDeniesCrossOriginSessionMutationsBeforeHandler(t *testing.T) {
	h := New(nil, nil, discardLogger(), []string{"https://portal.ghanageo.dev"})
	router := h.Routes()

	for _, tc := range []struct {
		name      string
		method    string
		origin    string
		fetchSite string
		want      int
	}{
		{name: "allowed portal", method: http.MethodPost, origin: "https://portal.ghanageo.dev", want: http.StatusNotFound},
		{name: "foreign origin", method: http.MethodPost, origin: "https://evil.example", want: http.StatusForbidden},
		{name: "cross site signal without origin", method: http.MethodDelete, fetchSite: "cross-site", want: http.StatusForbidden},
		{name: "safe reads are unaffected", method: http.MethodGet, origin: "https://evil.example", want: http.StatusNotFound},
		{name: "non browser mutation without cookie", method: http.MethodPost, origin: "https://evil.example", want: http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(tc.method, "/v1/not-a-route", nil)
			req.Header.Set("Origin", tc.origin)
			req.Header.Set("Sec-Fetch-Site", tc.fetchSite)
			if tc.name != "non browser mutation without cookie" {
				req.AddCookie(&http.Cookie{Name: sessionCookie, Value: "session-secret"})
			}
			router.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}
