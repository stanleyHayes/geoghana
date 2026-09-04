package rest

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// /metrics is the one admin-adjacent endpoint that would otherwise answer to
// anyone. It publishes traffic volume, latency distribution, error counts and
// queue depth, so it fails closed like the rest of this API.
func TestRequireMetricsTokenAdmitsOnlyTheConfiguredToken(t *testing.T) {
	served := false
	guarded := requireMetricsToken("s3cret-scrape-token", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		served = true
		w.WriteHeader(http.StatusOK)
	}))

	for _, tc := range []struct {
		name       string
		header     string
		wantStatus int
		wantServed bool
	}{
		{"correct token", "Bearer s3cret-scrape-token", http.StatusOK, true},
		{"no header", "", http.StatusNotFound, false},
		{"wrong token", "Bearer nope", http.StatusNotFound, false},
		{"token without scheme", "s3cret-scrape-token", http.StatusOK, true},
		{"prefix of the token", "Bearer s3cret", http.StatusNotFound, false},
		{"token plus suffix", "Bearer s3cret-scrape-token-extra", http.StatusNotFound, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			served = false
			request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
			if tc.header != "" {
				request.Header.Set("Authorization", tc.header)
			}
			recorder := httptest.NewRecorder()
			guarded.ServeHTTP(recorder, request)

			if recorder.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, tc.wantStatus)
			}
			if served != tc.wantServed {
				t.Fatalf("handler served = %v, want %v", served, tc.wantServed)
			}
		})
	}
}

// With no token configured the route is never registered, so an unguarded
// deployment cannot quietly publish metrics.
func TestMetricsNotRegisteredWithoutToken(t *testing.T) {
	handler := (&Handler{}).WithMetricsToken("")
	router := handler.Routes()

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if recorder.Code == http.StatusOK {
		t.Fatal("/metrics answered 200 with no METRICS_TOKEN configured")
	}
}
