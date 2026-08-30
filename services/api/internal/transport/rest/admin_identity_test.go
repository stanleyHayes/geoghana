package rest

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/identity"
	"github.com/ghanageo/ghanageo/services/api/internal/domain/usage"
	"github.com/go-chi/chi/v5"
)

func TestAdminDeveloperRoutesAreRegisteredAndPrivate(t *testing.T) {
	h := New(nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	want := map[string]bool{
		"GET /v1/admin/developers/organizations":         false,
		"GET /v1/admin/developers/accounts":              false,
		"GET /v1/admin/developers/applications":          false,
		"GET /v1/admin/developers/keys":                  false,
		"GET /v1/admin/developers/usage":                 false,
		"GET /v1/admin/developers/requests":              false,
		"POST /v1/admin/developers/keys/{keyId}/suspend": false,
		"POST /v1/admin/developers/keys/{keyId}/revoke":  false,
	}
	err := chi.Walk(h.Routes().(chi.Routes), func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		key := method + " " + route
		if _, ok := want[key]; ok {
			want[key] = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for route, found := range want {
		if !found {
			t.Errorf("route not registered: %s", route)
		}
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/developers/organizations", nil)
	h.Routes().ServeHTTP(rec, req)
	if rec.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatal("admin developer response is cacheable")
	}
}

func TestAdminDeveloperDTOsDoNotExposeSecrets(t *testing.T) {
	now := time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC)
	key := identity.AdminKey{ID: "key-1", Prefix: "gh_live_public", Name: "Web", CreatedAt: now}
	event := usage.Event{ID: "event-1", RequestID: "req-1", KeyID: "key-1", Operation: "GET /v1/search", At: now}
	b, err := json.Marshal(map[string]any{"key": adminKeyOut(key), "event": adminRequestOut(event), "usage": adminUsageOut(usage.Summary{AvgLatencyMS: 12.5})})
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	for _, forbidden := range []string{"secretHash", "passwordHash", "totpSecret", "recoveryCodes", "sessionEpoch"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("private field %q leaked: %s", forbidden, got)
		}
	}
	for _, required := range []string{`"prefix":"gh_live_public"`, `"state":"active"`, `"operation":"GET /v1/search"`, `"avgLatencyMs":12.5`} {
		if !strings.Contains(got, required) {
			t.Errorf("missing safe field %s: %s", required, got)
		}
	}
}

func TestAdminApplicationDTOUsesEmptyArraysForLegacyMissingLists(t *testing.T) {
	b, err := json.Marshal(adminApplicationOut(identity.Application{ID: "app-legacy", Name: "Legacy"}))
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	for _, required := range []string{`"environments":[]`, `"domains":[]`} {
		if !strings.Contains(got, required) {
			t.Fatalf("missing stable empty list %s: %s", required, got)
		}
	}
}

func TestAdminIdentityQueryValidation(t *testing.T) {
	for _, raw := range []string{"0", "101", "abc"} {
		r := httptest.NewRequest(http.MethodGet, "/?limit="+raw, nil)
		if _, err := adminIdentityFilter(r); err == nil {
			t.Errorf("limit %q accepted", raw)
		}
	}
	if got, err := parseAdminIdentityLimit(""); err != nil || got != 20 {
		t.Fatalf("default limit = %d, %v", got, err)
	}
}
