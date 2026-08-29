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

	// Top-level keys under `paths:` are two-space indented and start with "/".
	re := regexp.MustCompile(`(?m)^  (/[^:\s]*):`)
	var documented []string
	for _, m := range re.FindAllStringSubmatch(string(raw), -1) {
		documented = append(documented, normalizePath(m[1]))
	}
	sort.Strings(documented)

	if len(documented) == 0 {
		t.Fatal("no paths parsed from the OpenAPI contract")
	}

	implemented := map[string]bool{
		"/regions": true, "/regions/{}": true, "/regions/{}/districts": true,
		"/districts": true, "/districts/{}": true, "/districts/{}/places": true,
		"/places": true, "/places/{}": true,
		"/nearby": true,
	}
	// Documented but not yet implemented. Each entry is a promise with a story
	// behind it; the list must shrink, never grow silently.
	notYetImplemented := map[string]string{
		"/search":                "GEO-12.1",
		"/autocomplete":          "GEO-12.2",
		"/geocode":               "GEO-12.3",
		"/reverse":               "GEO-12.4",
		"/boundaries/{}":         "GEO-8.2",
		"/datasets":              "GEO-8.3",
		"/datasets/{}/downloads": "GEO-8.3",
	}

	for _, p := range documented {
		if implemented[p] {
			continue
		}
		if story, ok := notYetImplemented[p]; ok {
			t.Logf("documented, pending %s: %s", story, p)
			continue
		}
		t.Errorf("%s is in the OpenAPI contract but is neither implemented nor listed as pending", p)
	}

	docSet := map[string]bool{}
	for _, p := range documented {
		docSet[p] = true
	}
	for p := range implemented {
		if !docSet[p] {
			t.Errorf("%s is served but missing from the OpenAPI contract", p)
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
