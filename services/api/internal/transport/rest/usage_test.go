package rest

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/ghanageo/ghanageo/services/api/internal/domain/identity"
	usageDomain "github.com/ghanageo/ghanageo/services/api/internal/domain/usage"
	"github.com/ghanageo/ghanageo/services/api/internal/platform/auth"
)

type usageKeyLookup struct{ key identity.APIKey }

func (f usageKeyLookup) ByPrefix(context.Context, string) (*identity.APIKey, error) {
	return &f.key, nil
}
func (f usageKeyLookup) MarkUsed(context.Context, string, time.Time) error { return nil }

type usageLimiter struct{}

func (usageLimiter) Allow(context.Context, identity.Identity, identity.Allowance, identity.CostClass) (auth.Decision, error) {
	return auth.Decision{Allowed: true, Limit: 100, Remaining: 97}, nil
}

type usageRecorder struct{ events []usageDomain.Event }

func (r *usageRecorder) Record(_ context.Context, event usageDomain.Event) error {
	r.events = append(r.events, event)
	return nil
}
func (*usageRecorder) Summary(context.Context, string, string, time.Time) (usageDomain.Summary, error) {
	return usageDomain.Summary{}, nil
}
func (*usageRecorder) List(context.Context, string, string, *time.Time, int) (usageDomain.Page, error) {
	return usageDomain.Page{}, nil
}

func TestUsageMiddlewareRecordsOnlySafeAttributedMetadata(t *testing.T) {
	generated, err := identity.Generate(identity.EnvLive)
	if err != nil {
		t.Fatal(err)
	}
	recorder := &usageRecorder{}
	authenticator := auth.New(usageKeyLookup{key: identity.APIKey{
		ID: "key_1", OrganizationID: "org_1", ApplicationID: "app_1",
		Prefix: generated.Prefix, SecretHash: generated.SecretHash, Scopes: []identity.Scope{identity.ScopeLocationsRead},
	}}, usageLimiter{}, false)
	handler := &Handler{usage: recorder, log: slog.New(slog.NewTextHandler(io.Discard, nil))}

	router := chi.NewRouter()
	router.Use(chimiddleware.RequestID)
	router.Use(authenticator.Middleware(costOf))
	router.Use(handler.captureUsage("rest"))
	router.Get("/v1/regions/{id}", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) })

	request := httptest.NewRequest(http.MethodGet, "/v1/regions/gh-region-ahafo", nil)
	request.Header.Set("Authorization", "Bearer "+generated.Full)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if len(recorder.events) != 1 {
		t.Fatalf("events = %d, want 1", len(recorder.events))
	}
	event := recorder.events[0]
	if event.OrganizationID != "org_1" || event.ApplicationID != "app_1" || event.KeyID != "key_1" {
		t.Fatalf("tenant attribution = %+v", event)
	}
	if event.Operation != "GET /v1/regions/{id}" || event.Geography != "region:gh-region-ahafo" {
		t.Fatalf("safe operation metadata = %+v", event)
	}
	if event.Status != "404" || event.Success || event.QuotaCost != 1 || event.QuotaRemaining != 97 {
		t.Fatalf("result metadata = %+v", event)
	}
}

func TestUsageMiddlewareSkipsAnonymousRequests(t *testing.T) {
	recorder := &usageRecorder{}
	handler := &Handler{usage: recorder, log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	router := chi.NewRouter()
	router.Use(chimiddleware.RequestID)
	router.Use(handler.captureUsage("rest"))
	router.Get("/v1/regions", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/v1/regions", nil))
	if len(recorder.events) != 0 {
		t.Fatalf("anonymous events persisted = %d, want 0", len(recorder.events))
	}
}
